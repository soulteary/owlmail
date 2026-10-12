const { defineConfig } = require('@playwright/test');
const path = require('node:path');
const os = require('node:os');
const fs = require('node:fs');
const binary = process.env.OWLMAIL_BINARY || path.resolve(__dirname, '../../owlmail');
const mailDir = fs.mkdtempSync(path.join(os.tmpdir(), 'owlmail-browser-'));
module.exports = defineConfig({
    testDir: '.',
    testMatch: '*.spec.js',
    workers: 1,
    retries: process.env.CI ? 1 : 0,
    reporter: [['list'], ['html', { open: 'never' }]],
    use: {
        baseURL: 'http://127.0.0.1:21080/preview/',
        browserName: 'chromium',
        colorScheme: 'dark',
        trace: 'retain-on-failure',
        screenshot: 'only-on-failure',
        launchOptions: process.env.CHROME_BIN ? { executablePath: process.env.CHROME_BIN } : {}
    },
    webServer: {
        command: `"${binary}" -smtp 21025 -web 21080 -base-pathname /preview -mail-directory "${mailDir}"`,
        url: 'http://127.0.0.1:21080/preview/healthz',
        reuseExistingServer: false,
        timeout: 30000
    }
});
