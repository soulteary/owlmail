const { test, expect } = require('@playwright/test');
const { execFileSync } = require('node:child_process');

test.beforeAll(async ({ request, baseURL }) => {
    // A retried worker shares the server, so replace the previous fixture.
    const cleared = await request.delete(new URL('api/v1/emails', baseURL).href);
    expect(cleared.ok()).toBe(true);
    execFileSync('python3', ['-c', `
import smtplib
from email.message import EmailMessage
m = EmailMessage()
m['From'] = 'sender@example.test'
m['To'] = 'reader@example.test'
m['Subject'] = 'Browser regression fixture'
m.set_content('Plain-text fallback')
m.add_alternative('<html><body><h1>Long email</h1><img src="https://tracking.invalid/pixel">' + ''.join('<p>Row %d</p>' % i for i in range(100)) + '<script>document.body.dataset.executed="yes"</script></body></html>', subtype='html')
with smtplib.SMTP('127.0.0.1', 21025) as s:
    s.send_message(m)
`]);
});

test('sandboxed long HTML preview supports scrolling, widths and persisted preferences', async ({ page }) => {
    const remoteRequests = [];
    page.on('request', request => { if (request.url().includes('tracking.invalid')) remoteRequests.push(request.url()); });
    await page.goto('./');
    await expect(page.locator('body')).toHaveClass('dark-theme');
    await page.locator('.email-item').filter({ hasText: 'Browser regression fixture' }).click();
    const iframe = page.locator('.email-detail-html iframe');
    await expect(iframe).toHaveAttribute('sandbox', '');
    await expect(iframe).toHaveAttribute('referrerpolicy', 'no-referrer');
    await expect.poll(() => iframe.evaluate(node => node.getBoundingClientRect().height)).toBeGreaterThanOrEqual(400);
    const preview = page.frameLocator('.email-detail-html iframe');
    await expect(preview.locator('body')).not.toHaveAttribute('data-executed', 'yes');
    await expect.poll(() => preview.locator('html').evaluate(node => node.scrollHeight)).toBeGreaterThan(1000);
    await preview.locator('body').evaluate(() => window.scrollTo(0, 300));
    const before = await preview.locator('body').evaluate(() => window.scrollY);
    await page.locator('[data-viewport-width="375"]').click();
    await expect.poll(() => page.locator('#emailViewportFrame').evaluate(node => node.getBoundingClientRect().width)).toBe(375);
    await expect.poll(() => preview.locator('body').evaluate(() => window.scrollY)).toBe(before);
    await page.locator('[data-viewport-width="1440"]').click();
    await expect.poll(() => page.locator('#emailViewportStage').evaluate(node => node.scrollWidth > node.clientWidth)).toBe(true);
    await page.locator('[data-viewport-width="375"]').click();
    await page.locator('#themeToggle').click();
    await expect(page.locator('body')).toHaveClass('light-theme');
    await page.reload({ waitUntil: 'networkidle' });
    await expect(page.locator('body')).toHaveClass('light-theme');
    await expect.poll(() => page.locator('#emailViewportFrame').evaluate(node => node.getBoundingClientRect().width)).toBe(375);
    expect(remoteRequests).toEqual([]);
});

test('shortcuts respect text input and focused buttons', async ({ page }) => {
    await page.goto('./');
    await expect(page.locator('.email-item')).toHaveCount(1);
    await page.locator('h1').click();
    await page.keyboard.press('/');
    await expect(page.locator('#searchInput')).toBeFocused();
    await page.keyboard.type('t');
    await expect(page.locator('body')).toHaveClass('dark-theme');
    await page.locator('#refreshBtn').focus();
    await page.keyboard.press('t');
    await expect(page.locator('body')).toHaveClass('dark-theme');
    await page.locator('h1').click();
    await page.keyboard.press('t');
    await expect(page.locator('body')).toHaveClass('light-theme');
});

test('blocked preference storage does not prevent mailbox startup', async ({ page }) => {
    await page.addInitScript(() => {
        Storage.prototype.getItem = Storage.prototype.setItem = () => { throw new DOMException('Blocked', 'SecurityError'); };
    });
    await page.goto('./');
    await expect(page.locator('.email-item')).toHaveCount(1);
    await page.locator('#themeToggle').click();
    await expect(page.locator('body')).toHaveClass('light-theme');
});

test('command palette supports filtering, keyboard execution and focus restoration', async ({ page }) => {
    await page.goto('./');
    const opener = page.locator('#commandPaletteButton');
    const dialog = page.locator('#commandPalette');
    const input = page.locator('#commandSearch');
    await opener.click();
    await expect(dialog).toBeVisible();
    await expect(input).toBeFocused();
    await expect(page.locator('[data-command="delete-email"]')).toBeDisabled();
    await input.fill('no-such-command');
    await expect(page.locator('#commandEmpty')).toBeVisible();
    await expect(page.locator('.command-option')).toHaveCount(0);
    await input.fill('refresh');
    await expect(page.locator('.command-option')).toHaveCount(1);
    await page.keyboard.press('Enter');
    await expect(dialog).not.toBeVisible();
    await expect(opener).toBeFocused();
    await page.keyboard.press('Control+k');
    await expect(input).toBeFocused();
    await page.keyboard.press('Escape');
    await expect(opener).toBeFocused();
    await opener.click();
    await input.fill('Focus mailbox search');
    await page.keyboard.press('ArrowDown');
    await expect(page.locator('[data-command="search"]')).toBeFocused();
    await page.keyboard.press('Enter');
    await expect(page.locator('#searchInput')).toBeFocused();
    await page.keyboard.type('?');
    await expect(dialog).not.toBeVisible();
});

test('palette is localized, fits mobile screens and keeps delete confirmation', async ({ page }) => {
    await page.goto('./');
    await page.locator('.email-item').filter({ hasText: 'Browser regression fixture' }).click();
    await page.locator('#commandPaletteButton').click();
    await page.locator('#commandSearch').fill('Delete selected message');
    const confirmation = page.waitForEvent('dialog');
    const activation = page.keyboard.press('Enter');
    const prompt = await confirmation;
    expect(prompt.type()).toBe('confirm');
    await prompt.dismiss();
    await activation;
    await expect(page.locator('.email-item')).toHaveCount(1);
    await page.locator('#langSelect').selectOption('zh-CN');
    await page.setViewportSize({ width: 375, height: 700 });
    await page.locator('#commandPaletteButton').click();
    await expect(page.locator('#commandTitle')).toHaveText('命令与快捷键');
    await expect.poll(() => page.locator('#commandPalette').evaluate(node => node.getBoundingClientRect().width)).toBeLessThanOrEqual(375);
    await page.keyboard.press('Escape');
});
