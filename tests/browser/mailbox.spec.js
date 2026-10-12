const { test, expect } = require('@playwright/test');
const { execFileSync } = require('node:child_process');

test.beforeAll(() => {
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
