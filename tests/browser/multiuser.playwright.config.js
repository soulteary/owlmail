const { defineConfig } = require('@playwright/test');
const path = require('node:path');
const fixture = process.env.OWLMAIL_BROWSER_FIXTURE || path.resolve(__dirname, '../../owlmail-browser-fixture');
const strictBackground = process.env.OWLMAIL_REQUIRE_REAL_BACKGROUND === '1';
module.exports = defineConfig({
    testDir: '.',
    testMatch: 'multiuser.spec.js',
    workers: 1,
    retries: 0,
    timeout: 120000,
    reporter: [['list'], ['html', { open: 'never', outputFolder: strictBackground ? 'multiuser-background-report' : 'multiuser-playwright-report' }]],
    outputDir: strictBackground ? 'multiuser-background-results' : 'multiuser-test-results',
    use: {
        baseURL: 'http://127.0.0.1:21180/',
        browserName: 'chromium',
        headless: process.env.OWLMAIL_BROWSER_HEADED !== '1' && process.env.OWLMAIL_REQUIRE_REAL_BACKGROUND !== '1',
        viewport: { width: 1365, height: 900 },
        colorScheme: 'dark',
        trace: 'retain-on-failure',
        screenshot: 'only-on-failure',
        launchOptions: process.env.CHROME_BIN ? { executablePath: process.env.CHROME_BIN } : {}
    },
    webServer: {
        command: `"${fixture}"`,
        url: 'http://127.0.0.1:21180/__fixture__/health',
        reuseExistingServer: false,
        timeout: 30000
    }
});
