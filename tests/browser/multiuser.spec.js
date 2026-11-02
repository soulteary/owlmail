const { test, expect } = require('@playwright/test');
const fs = require('node:fs');
const path = require('node:path');
const os = require('node:os');
const { spawn } = require('node:child_process');
const { removeBrowserProfile } = require('./profile-cleanup');

// This suite uses the explicitly tagged loopback proxy fixture. Assertions are
// synthetic verified-email evidence, not a claim about deployed Stargate.
async function identity(context, user, verified = true) {
    await context.addCookies([
        { name: 'fixture-user', value: user, url: 'http://127.0.0.1:21180/' },
        { name: 'fixture-verified', value: String(verified), url: 'http://127.0.0.1:21180/' }
    ]);
}
async function deliver(request, user, subject, attachment = '') {
    const response = await request.post('/__fixture__/deliver', { data: {
        Recipient: `${user}@example.test`, Subject: subject,
        Body: `${user} private body for ${subject}`, Attachment: attachment
    } });
    expect(response.ok(), await response.text()).toBe(true);
}
async function stats(request) { return (await request.get('/__fixture__/stats')).json(); }

test('real SMTP, synthetic trusted identity and browser mailbox isolation', async ({ page, context, request }, testInfo) => {
    request = context.request;
    const sockets = [];
    page.on('websocket', socket => sockets.push(socket.url()));
    await deliver(request, 'alice', 'Alice private notice', 'alice-only attachment');
    await deliver(request, 'bob', 'Bob private notice', 'bob-only attachment');
    await identity(context, 'alice', false);
    await page.goto('/');
    await expect(page.locator('.email-item')).toHaveCount(0);
    await expect(page.locator('#webhookBtn')).not.toBeVisible();
    await identity(context, 'alice', true);
    await page.getByRole('button', { name: 'Refresh', exact: true }).click();
    await expect(page.locator('.email-item')).toHaveCount(1);
    await expect(page.locator('.email-item')).toContainText('Alice private notice');
    await expect(page.locator('.email-item')).not.toContainText('Bob private notice');
    await page.locator('.email-item-open').click();
    await expect(page.locator('#emailDetail')).toContainText('Alice private notice');
    const aliceEmails = await (await request.get('/api/v1/emails')).json();
    expect(aliceEmails.total).toBe(1);
    const alice = aliceEmails.emails[0];
    const attachment = alice.attachments[0];
    expect(attachment.fileName).toBe('fixture.txt');
    const ownAttachment = await request.get(`/api/v1/emails/${alice.id}/attachments/${attachment.generatedFileName}`);
    expect(await ownAttachment.text()).toBe('alice-only attachment');
    const evidence = path.resolve(__dirname, 'multiuser-evidence');
    fs.mkdirSync(evidence, { recursive: true });
    await page.screenshot({ path: path.join(evidence, 'alice_personal_inbox.png'), fullPage: true });
    await identity(context, 'bob', true);
    await page.getByRole('button', { name: 'Refresh', exact: true }).click();
    await expect(page.locator('.email-item')).toHaveCount(1);
    await expect(page.locator('.email-item')).toContainText('Bob private notice');
    await expect(page.locator('#emailDetail')).not.toContainText('Alice private notice');
    await page.locator('.email-item-open').click();
    await expect(page.locator('#emailDetail')).toContainText('Bob private notice');
    for (const suffix of ['', '/html', '/source', '/raw', `/attachments/${attachment.generatedFileName}`]) {
        expect((await request.get(`/api/v1/emails/${alice.id}${suffix}`)).status()).toBe(404);
    }
    expect((await request.get(`/api/v1/emails/export?ids=${alice.id}`)).status()).toBe(404);
    const bobEmails = await (await request.get('/api/v1/emails')).json();
    const bob = bobEmails.emails[0];
    expect((await request.delete('/api/v1/emails/batch', { data: { ids: [bob.id, alice.id] } })).status()).toBe(404);
    expect((await (await request.get('/api/v1/emails')).json()).total).toBe(1);
    for (const route of ['/mcp', '/api/v1/ws', '/socket.io', '/email', '/config', '/reloadMailsFromDirectory']) {
        expect((await request.get(route)).status()).toBe(404);
    }
    expect(sockets).toEqual([]);
    await page.screenshot({ path: path.join(evidence, 'bob_personal_inbox.png'), fullPage: true });
    await testInfo.attach('alice personal inbox', { path: path.join(evidence, 'alice_personal_inbox.png'), contentType: 'image/png' });
    await testInfo.attach('bob personal inbox', { path: path.join(evidence, 'bob_personal_inbox.png'), contentType: 'image/png' });
    page.once('dialog', dialog => dialog.accept());
    await page.getByRole('button', { name: 'Delete All', exact: true }).click();
    await expect(page.locator('.email-item')).toHaveCount(0);
    await identity(context, 'alice', true);
    expect((await (await request.get('/api/v1/emails')).json()).total).toBe(1);
});

test('manual and timed refresh preserve selection and clear on 401', async ({ page, context, request }) => {
    request = context.request;
    await identity(context, 'alice', true);
    await request.delete('/api/v1/emails');
    await deliver(request, 'alice', 'Polling selection stays');
    await page.goto('/');
    await expect(page.locator('.email-item')).toHaveCount(1);
    await page.locator('.email-item-open').click();
    await expect(page.locator('#emailDetail')).toContainText('Polling selection stays');
    await page.getByRole('button', { name: 'Auto refresh: on' }).click();
    const disabledBefore = (await stats(request)).previews;
    await deliver(request, 'alice', 'Manual refresh arrived');
    await page.waitForTimeout(16000);
    expect((await stats(request)).previews).toBe(disabledBefore);
    await expect(page.locator('.email-item')).toHaveCount(1);
    await page.getByRole('button', { name: 'Refresh', exact: true }).click();
    await expect(page.locator('.email-item')).toHaveCount(2);
    await expect(page.locator('#emailDetail')).toContainText('Polling selection stays');
    await page.getByRole('button', { name: 'Auto refresh: off' }).click();
    await deliver(request, 'alice', 'Timed refresh arrived');
    await expect(page.locator('.email-item')).toHaveCount(3, { timeout: 20000 });
    await expect(page.locator('#emailDetail')).toContainText('Polling selection stays');
    await context.addCookies([{ name: 'fixture-delay-ms', value: '300', url: 'http://127.0.0.1:21180/' }]);
    let activeRefreshes = 0;
    let peakRefreshes = 0;
    const refreshRequests = new Set();
    page.on('request', r => {
        if (!r.url().includes('/emails/preview')) return;
        refreshRequests.add(r);
        activeRefreshes += 1;
        peakRefreshes = Math.max(peakRefreshes, activeRefreshes);
    });
    const finished = r => { if (refreshRequests.delete(r)) activeRefreshes -= 1; };
    page.on('requestfinished', finished);
    page.on('requestfailed', finished);
    await page.getByRole('button', { name: 'Refresh', exact: true }).click();
    await page.getByRole('button', { name: 'Refresh', exact: true }).click();
    await expect(page.locator('#emailDetail')).toContainText('Polling selection stays');
    await page.waitForTimeout(700);
    expect(peakRefreshes).toBeLessThanOrEqual(1);
    await identity(context, 'anonymous');
    await page.getByRole('button', { name: 'Refresh', exact: true }).click();
    await expect(page.locator('.email-item')).toHaveCount(0);
    await expect(page.locator('#emailDetail')).not.toContainText('Polling selection stays');
    const expiredBefore = (await stats(request)).previews;
    await page.waitForTimeout(16000);
    expect((await stats(request)).previews).toBe(expiredBefore);
});


// Playwright normally enables focus emulation on every managed page. That
// makes all pages report visible even after a real tab switch. This helper
// starts only a private temporary browser and uses the documented noDefaults
// CDP option on its default context; it never edits document visibility.
async function nativeBackgroundBrowser(playwright) {
    const profile = fs.mkdtempSync(path.join(os.tmpdir(), 'owlmail-real-background-'));
    const executable = process.env.CHROME_BIN || playwright.chromium.executablePath();
    const child = spawn(executable, [
        `--user-data-dir=${profile}`, '--remote-debugging-address=127.0.0.1',
        '--remote-debugging-port=0', '--no-first-run', '--no-default-browser-check',
        '--no-sandbox', '--disable-dev-shm-usage', '--disable-background-timer-throttling',
        '--disable-renderer-backgrounding', '--window-size=1365,900', 'about:blank'
    ], { stdio: ['ignore', 'ignore', 'pipe'] });
    let stderr = '';
    let exited = false;
    child.stderr.on('data', chunk => { stderr = (stderr + chunk).slice(-16384); });
    const exit = new Promise(resolve => {
        child.once('error', error => { stderr = `Native browser spawn failed: ${error.message}`; exited = true; resolve(); });
        child.once('exit', () => { exited = true; resolve(); });
    });
    let browser;
    const close = async () => {
        if (browser) await Promise.race([browser.close().catch(() => {}), new Promise(resolve => setTimeout(resolve, 3000))]);
        if (!exited) child.kill('SIGTERM');
        await Promise.race([exit, new Promise(resolve => setTimeout(resolve, 3000))]);
        if (!exited) {
            child.kill('SIGKILL');
            await Promise.race([exit, new Promise(resolve => setTimeout(resolve, 3000))]);
        }
        if (!exited) throw new Error(`Native Chromium did not exit after SIGKILL; test profile retained at ${profile}`);
        await removeBrowserProfile(profile);
    };
    try {
        const activePort = path.join(profile, 'DevToolsActivePort');
        for (let attempt = 0; attempt < 150 && !fs.existsSync(activePort); attempt++) {
            if (exited) throw new Error(`Native Chromium exited before CDP startup: ${stderr}`);
            await new Promise(resolve => setTimeout(resolve, 100));
        }
        if (!fs.existsSync(activePort)) throw new Error(`Native Chromium CDP startup timeout: ${stderr}`);
        const port = fs.readFileSync(activePort, 'utf8').trim().split('\n')[0];
        browser = await playwright.chromium.connectOverCDP(`http://127.0.0.1:${port}`, { noDefaults: true, timeout: 10000 });
        const context = browser.contexts()[0];
        const page = context.pages()[0] || await context.newPage();
        return { context, page, close };
    } catch (error) { await close(); throw error; }
}

test('real background visibility pauses polling and immediately refreshes on return', async ({ playwright }, testInfo) => {
    // Do not request Playwright's page/context/browser fixtures here: their
    // default focus override would invalidate the real-visibility assertion.
    const native = await nativeBackgroundBrowser(playwright);
    const { page, context } = native;
    let request;
    try {
        await identity(context, 'alice', true);
        request = await playwright.request.newContext({
            baseURL: 'http://127.0.0.1:21180/', storageState: await context.storageState()
        });
        await request.delete('/api/v1/emails');
        await deliver(request, 'alice', 'Background selection stays');
        await page.goto('http://127.0.0.1:21180/');
        await expect(page.locator('.email-item')).toHaveCount(1);
        await page.locator('.email-item-open').click();
        await expect(page.locator('#emailDetail')).toContainText('Background selection stays');
        const foreground = await context.newPage();
        await foreground.goto('about:blank');
        await foreground.bringToFront();
        const strict = process.env.OWLMAIL_REQUIRE_REAL_BACKGROUND === '1';
        if (strict) {
            await expect.poll(() => page.evaluate(() => document.visibilityState), {
                message: 'Strict background regression requires actual Chromium hidden visibility; skip is forbidden.'
            }).toBe('hidden');
        } else {
            await page.waitForTimeout(500);
            const visibility = await page.evaluate(() => document.visibilityState);
            test.skip(visibility !== 'hidden', `Native browser reports ${visibility} for a real background tab; no synthetic visibility override is used.`);
        }
        let activeRefreshes = 0;
        let peakRefreshes = 0;
        const refreshRequests = new Set();
        page.on('request', r => {
            if (!r.url().includes('/emails/preview')) return;
            refreshRequests.add(r); activeRefreshes += 1;
            peakRefreshes = Math.max(peakRefreshes, activeRefreshes);
        });
        const finished = r => { if (refreshRequests.delete(r)) activeRefreshes -= 1; };
        page.on('requestfinished', finished);
        page.on('requestfailed', finished);
        // Observe a separate real JS timer while hidden. It neither overrides
        // visibility nor touches the application timer, proving no freeze is
        // being mistaken for application-level polling suspension.
        await page.evaluate(() => {
            window.__owlmailBackgroundHeartbeats = 0;
            window.__owlmailBackgroundHeartbeat = setInterval(() => { window.__owlmailBackgroundHeartbeats += 1; }, 1000);
        });
        const before = (await stats(request)).previews;
        await deliver(request, 'alice', 'Delivered while truly hidden');
        await page.waitForTimeout(16000);
        const hidden = await page.evaluate(() => ({ visibility: document.visibilityState, hidden: document.hidden }));
        expect(hidden).toEqual({ visibility: 'hidden', hidden: true });
        const backgroundHeartbeats = await page.evaluate(() => window.__owlmailBackgroundHeartbeats);
        expect(backgroundHeartbeats).toBeGreaterThan(0);
        const after = (await stats(request)).previews;
        expect(after).toBe(before);
        await expect(page.locator('.email-item')).toHaveCount(1);
        const foregroundAt = Date.now();
        await page.bringToFront();
        await expect.poll(() => page.evaluate(() => document.visibilityState)).toBe('visible');
        // Normal refresh is 15 seconds. Five seconds proves immediate resume.
        await expect(page.locator('.email-item')).toHaveCount(2, { timeout: 5000 });
        const foregroundRefreshMilliseconds = Date.now() - foregroundAt;
        await expect(page.locator('.email-item')).toContainText(['Delivered while truly hidden', 'Background selection stays']);
        await expect(page.locator('#emailDetail')).toContainText('Background selection stays');
        expect(peakRefreshes).toBeGreaterThanOrEqual(1);
        expect(peakRefreshes).toBeLessThanOrEqual(1);
        await page.evaluate(() => clearInterval(window.__owlmailBackgroundHeartbeat));
        const evidence = {
            syntheticIdentity: true, smtpTransport: 'TLS SMTP',
            visibility: { background: hidden, foreground: await page.evaluate(() => document.visibilityState) },
            backgroundMilliseconds: 16000, backgroundHeartbeats, backgroundPreviewRequestsBefore: before,
            backgroundPreviewRequestsAfter: after, foregroundRefreshMilliseconds,
            selectedDetailPreserved: true, newMessageVisible: true, maximumInFlightRefreshes: peakRefreshes
        };
        console.log('REAL_BACKGROUND_EVIDENCE ' + JSON.stringify(evidence));
        const evidencePath = testInfo.outputPath('real_background_evidence.json');
        fs.writeFileSync(evidencePath, JSON.stringify(evidence, null, 2));
        await testInfo.attach('real background evidence', { path: evidencePath, contentType: 'application/json' });
        const screenshot = testInfo.outputPath('background_resume_selection.png');
        await page.screenshot({ path: screenshot, fullPage: true });
        await testInfo.attach('background resume preserves selection', { path: screenshot, contentType: 'image/png' });
        await foreground.close();
    } finally {
        try { if (request) await request.dispose(); }
        finally { await native.close(); }
    }
});
