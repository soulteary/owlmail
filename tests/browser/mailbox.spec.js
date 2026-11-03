const { test, expect } = require('@playwright/test');
const { execFileSync } = require('node:child_process');

test.beforeAll(async ({ request, baseURL }) => {
    // A retried worker shares the server, so replace the previous fixture.
    const cleared = await request.delete(new URL('api/v1/emails', baseURL).href);
    expect(cleared.ok()).toBe(true);
    execFileSync('python3', ['-c', `
import smtplib
import base64
from email.message import EmailMessage
m = EmailMessage()
m['From'] = 'sender@example.test'
m['To'] = 'reader@example.test'
m['Subject'] = 'Browser regression fixture'
m['Reply-To'] = 'Support <replies@example.test>, Backup <backup@example.test>'
m.set_content('Plain-text fallback')
m.add_alternative('<html><body><h1>Long email</h1><img alt="Inline logo" src="cid:logo&commat;example.test"><a href="cid:logo@example.test">Download logo</a><img src="https://tracking.invalid/pixel">' + ''.join('<p>Row %d</p>' % i for i in range(100)) + '<script>document.body.dataset.executed="yes"</script></body></html>', subtype='html')
m.get_payload()[-1].add_related(base64.b64decode('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII='), maintype='image', subtype='png', cid='<logo@example.test>', filename='logo.png')
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
    await expect(page.getByTestId('email-reply-to')).toContainText('replies@example.test');
    await expect(page.getByTestId('email-reply-to')).toContainText('backup@example.test');
    const inlineLogo = preview.locator('img[alt="Inline logo"]');
    await expect(inlineLogo).toHaveAttribute('src', /^http:\/\/127\.0\.0\.1:21080\/preview\/api\/v1\/emails\/[^/]+\/attachments\//);
    await expect.poll(() => inlineLogo.evaluate(node => node.complete && node.naturalWidth)).toBe(1);
    const attachmentLink = preview.getByText('Download logo');
    await expect(attachmentLink).toHaveAttribute('href', await inlineLogo.getAttribute('src'));
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

test('direct HTML view blocks remote resources while the preview keeps explicit opt-in', async ({ page, request, baseURL }) => {
    const http = require('node:http');
    const remoteHits = [];
    let destinationReferrer;
    const svg = '<svg xmlns="http://www.w3.org/2000/svg"><text>download fixture</text></svg>';
    const pixel = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=', 'base64');
    const tracker = http.createServer((req, res) => {
        remoteHits.push(req.url);
        if (req.url === '/destination') {
            destinationReferrer = req.headers.referer || null;
            res.writeHead(200, { 'Content-Type': 'text/html' });
            res.end('<html><head><link rel="icon" href="data:,"></head><body><h1>Verification destination</h1><script>document.body.dataset.functional="yes"</script></body></html>');
            return;
        }
        res.writeHead(200, { 'Content-Type': req.url === '/mail.css' ? 'text/css' : 'image/png' });
        res.end(req.url === '/mail.css' ? 'h1 { color: blue; }' : pixel);
    });
    await new Promise(resolve => tracker.listen(0, '127.0.0.1', resolve));
    let email;
    try {
        const remote = `http://127.0.0.1:${tracker.address().port}`;
        execFileSync('python3', ['-c', `
import smtplib, sys
from email.message import EmailMessage
m = EmailMessage()
m['From'] = 'sender@example.test'
m['To'] = 'reader@example.test'
m['Subject'] = 'Direct HTML resource boundary'
m.set_content('Plain fallback')
m.add_alternative('<h1>HTML boundary</h1><a href="cid:download@example.test">Download SVG</a><a href="'+sys.argv[1]+'/destination">Verification link</a><link rel="stylesheet" href="'+sys.argv[1]+'/mail.css"><img alt="Remote pixel" src="'+sys.argv[1]+'/pixel"><img alt="Local asset" src="/preview/favicon.svg">', subtype='html')
m.get_payload()[-1].add_related(sys.argv[2].encode('utf-8'), maintype='image', subtype='svg+xml', cid='<download@example.test>', filename='download.svg')
with smtplib.SMTP('127.0.0.1', 21025) as client:
    client.send_message(m)
`, remote, svg]);
        const listing = await (await request.get(new URL('api/v1/emails', baseURL).href)).json();
        email = listing.emails.find(item => item.subject === 'Direct HTML resource boundary');
        expect(email).toBeTruthy();
        const response = await page.goto(new URL(`api/v1/emails/${email.id}/html`, baseURL).href);
        expect(response.headers()['content-security-policy']).toContain("default-src 'none'");
        await expect(page.locator('h1')).toHaveText('HTML boundary');
        await expect.poll(() => page.getByAltText('Remote pixel').evaluate(node => node.complete && node.naturalWidth === 0)).toBe(true);
        await expect.poll(() => page.getByAltText('Local asset').evaluate(node => node.naturalWidth)).toBeGreaterThan(0);
        expect(remoteHits).toEqual([]);

        const destinationOpened = page.context().waitForEvent('page');
        await page.getByRole('link', { name: 'Verification link', exact: true }).click();
        const destination = await destinationOpened;
        try {
            await expect(destination.locator('h1')).toHaveText('Verification destination');
            await expect(destination.locator('body')).toHaveAttribute('data-functional', 'yes');
            expect(await destination.evaluate(() => window.opener)).toBeNull();
            expect(destinationReferrer).toBeNull();
            expect(remoteHits).toContain('/destination');
            expect(remoteHits).not.toContain('/pixel');
            expect(remoteHits).not.toContain('/mail.css');
        } finally {
            await destination.close();
        }
        remoteHits.length = 0;

        for (const path of [`api/email/${email.id}/html`, `messages/${email.id}.html`]) {
            await page.goto(new URL(path, baseURL).href);
            const downloadStarted = page.waitForEvent('download');
            await page.getByRole('link', { name: 'Download SVG', exact: true }).click();
            const download = await downloadStarted;
            const stream = await download.createReadStream();
            const chunks = [];
            for await (const chunk of stream) chunks.push(chunk);
            expect(Buffer.concat(chunks).toString()).toBe(svg);
            expect(await download.failure()).toBeNull();
            expect(remoteHits).toEqual([]);
        }

        await page.goto('./');
        await page.locator('.email-item').filter({ hasText: 'Direct HTML resource boundary' }).click();
        await expect(page.getByRole('button', { name: 'Load remote content', exact: true })).toBeVisible();
        expect(remoteHits).toEqual([]);
        await page.getByRole('button', { name: 'Load remote content', exact: true }).click();
        await expect.poll(() => remoteHits).toContain('/pixel');
        await expect.poll(() => remoteHits).toContain('/mail.css');
    } finally {
        if (email) await request.delete(new URL(`api/v1/emails/${email.id}`, baseURL).href);
        await page.goto('about:blank');
        await new Promise(resolve => tracker.close(resolve));
    }
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

test('relay receipt detail and successful refresh preserve the sandboxed preview', async ({ page }) => {
    let receipt = null;
    let detailRequests = 0;
    let repairDetailRequests = 0;
    await page.route('**/api/v1/emails/**', async route => {
        const url = new URL(route.request().url());
        // Only decorate the singular native detail response; list, preview,
        // attachment and action routes must continue to use real server data.
        if (route.request().method() !== 'GET' || !/\/api\/v1\/emails\/[^/]+$/.test(url.pathname)) {
            await route.continue();
            return;
        }
        const response = await route.fetch();
        const email = await response.json();
        if (!email || typeof email.id !== 'string') {
            await route.fulfill({ response });
            return;
        }
        detailRequests++;
        const repaired = receipt && ++repairDetailRequests >= 3;
        const projected = !receipt || repairDetailRequests === 1 ? email
            : { ...email, ...(repaired ? receipt : { relayedAt: '2026-10-28T01:02:03Z', relayedTo: ['old@example.test'] }) };
        await route.fulfill({ response, json: projected });
    });
    await page.goto('./');
    await expect(page.locator('.email-item')).toHaveCount(1);
    await page.locator('.email-item').filter({ hasText: 'Browser regression fixture' }).click();
    await expect(page.getByTestId('email-relay-receipt')).toHaveCount(0);
    const iframe = page.locator('.email-detail-html iframe');
    const preview = page.frameLocator('.email-detail-html iframe');
    await expect(iframe).toHaveAttribute('sandbox', '');
    await expect(iframe).toHaveAttribute('referrerpolicy', 'no-referrer');
    await expect.poll(() => preview.locator('html').evaluate(node => node.scrollHeight)).toBeGreaterThan(1000);
    await iframe.evaluate(node => { node.dataset.receiptIdentity = 'original-preview'; });
    await preview.locator('body').evaluate(() => {
        document.body.dataset.receiptIdentity = 'original-document';
        window.scrollTo(0, 300);
    });
    const scrollBefore = await preview.locator('body').evaluate(() => window.scrollY);
    receipt = { relayedAt: '2026-10-29T01:02:03Z', relayedTo: ['actual@example.test', 'blind@example.test'] };
    // This is the same completion hook used after a successful Relay status poll.
    await page.evaluate(() => cacheRelayCompletion(state.currentEmail.id, { status: 'succeeded', receiptPending: false, completedAt: '2026-10-29T01:02:03Z' }));
    const summary = page.getByTestId('email-relay-receipt');
    await expect(summary).toContainText('actual@example.test, blind@example.test');
    await expect(summary.locator('time')).toHaveAttribute('datetime', '2026-10-29T01:02:03.000Z');
    await expect(summary.locator('time')).not.toHaveText('');
    expect(detailRequests).toBe(4);
    await expect(iframe).toHaveAttribute('data-receipt-identity', 'original-preview');
    await expect(iframe).toHaveAttribute('sandbox', '');
    await expect(iframe).toHaveAttribute('referrerpolicy', 'no-referrer');
    await expect(preview.locator('body')).toHaveAttribute('data-receipt-identity', 'original-document');
    expect(await preview.locator('body').evaluate(() => window.scrollY)).toBe(scrollBefore);
});
