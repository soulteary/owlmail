const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, '../../web/webhooks.js'), 'utf8');
const page = fs.readFileSync(path.join(__dirname, '../../web/webhooks.html'), 'utf8');
const styles = fs.readFileSync(path.join(__dirname, '../../web/webhooks.css'), 'utf8');

function loadConfigurator() {
    const window = {};
    const document = { addEventListener() {} };
    const sandbox = {
        URL,
        clearTimeout,
        console,
        document,
        navigator: { language: 'en-US' },
        setTimeout,
        window
    };
    vm.createContext(sandbox);
    vm.runInContext(source, sandbox, { filename: 'web/webhooks.js' });
    return window.OwlMailWebhookConfigurator;
}

const configurator = loadConfigurator();

function codes(result, type = 'errors') {
    return Array.from(result[type], (item) => item.code);
}

test('page exposes builder, import, output, and local-only controls', () => {
    for (const marker of [
        'id="targetList"',
        'id="targetTemplate"',
        'id="importInput"',
        'id="dropZone"',
        'id="configOutput"',
        'id="copyConfig"',
        'id="downloadConfig"',
        'Local-only editor.'
    ]) {
        assert.ok(page.includes(marker), 'webhooks.html is missing ' + marker);
    }
    assert.ok(page.includes('href="/style.css"'));
    assert.ok(styles.includes('.webhook-workspace'));
    assert.ok(styles.includes('body.dark-theme'));
    assert.ok(!page.includes('http://') && !page.includes('https://cdn.'), 'page must not load remote assets');
});

test('minimal configuration imports and normalizes defaults', () => {
    const result = configurator.parseConfigText(JSON.stringify({
        version: 1,
        targets: [{
            name: ' primary ',
            url: 'https://example.com/hooks/owlmail',
            method: 'post',
            contentType: 'application/json',
            timeout: '5s',
            retries: 0
        }]
    }));

    assert.deepEqual(codes(result), []);
    assert.deepEqual(JSON.parse(JSON.stringify(result.config)), {
        version: 1,
        targets: [{
            name: 'primary',
            url: 'https://example.com/hooks/owlmail'
        }]
    });
});

test('full configuration survives import parsing', () => {
    const sourceConfig = {
        version: 1,
        targets: [{
            name: 'alerts',
            url: 'https://example.com/owlmail',
            method: 'PATCH',
            headers: { Authorization: 'Bearer token' },
            contentType: 'application/vnd.owlmail+json',
            bodyTemplate: '{"subject":{{ json .Subject }}}',
            secret: 'test-secret',
            timeout: '10s',
            retries: 3,
            match: {
                from: ['*@example.com'],
                to: ['alerts@example.com'],
                subject: ['[staging]*'],
                text: ['*failed*']
            }
        }]
    };

    const result = configurator.parseConfigText(JSON.stringify(sourceConfig));

    assert.deepEqual(codes(result), []);
    assert.deepEqual(JSON.parse(JSON.stringify(result.config)), sourceConfig);
    assert.ok(codes(result, 'warnings').includes('templateRuntime'));
});

test('import parser rejects malformed JSON and unknown fields', () => {
    const malformed = configurator.parseConfigText('{"targets":[]} trailing');
    assert.deepEqual(codes(malformed), ['invalidJSON']);

    const unknown = configurator.parseConfigText(JSON.stringify({
        version: 1,
        targetz: [],
        targets: [{ name: 'one', url: 'https://example.com', unexpected: true }]
    }));
    assert.ok(codes(unknown).filter((code) => code === 'unknownField').length >= 2);
    assert.equal(unknown.config, null);
});

test('validation follows OwlMail target limits and safety rules', () => {
    const result = configurator.validateConfig({
        version: 1,
        targets: [
            {
                name: 'duplicate',
                url: 'https://user:pass@example.com/hook#fragment',
                method: 'GET',
                headers: {
                    Host: 'example.com',
                    'Bad Header': 'value',
                    'X-Newline': 'first\nsecond'
                },
                timeout: '61s',
                retries: 6,
                match: { subject: ['['] }
            },
            { name: 'duplicate', url: 'file:///tmp/hook' }
        ]
    });
    const resultCodes = codes(result);

    [
        'duplicateName',
        'urlCredentials',
        'urlFragment',
        'unsupportedMethod',
        'managedHeader',
        'headerNameInvalid',
        'headerNewline',
        'timeoutRange',
        'retriesInvalid',
        'matchGlob',
        'urlScheme',
        'urlHost'
    ].forEach((code) => assert.ok(resultCodes.includes(code), 'missing validation code ' + code));
});

test('environment placeholders remain editable and produce runtime notes', () => {
    const hostPlaceholder = '$' + '{OWLMAIL_WEBHOOK_HOST}';
    const tokenPlaceholder = '$' + '{OWLMAIL_WEBHOOK_TOKEN}';
    const secretPlaceholder = '$' + '{OWLMAIL_WEBHOOK_SECRET}';
    const result = configurator.parseConfigText(JSON.stringify({
        version: 1,
        targets: [{
            name: 'secure',
            url: 'https://' + hostPlaceholder + '/hook',
            headers: { Authorization: 'Bearer ' + tokenPlaceholder },
            secret: secretPlaceholder
        }]
    }));

    assert.deepEqual(codes(result), []);
    assert.ok(codes(result, 'warnings').filter((code) => code === 'envRuntime').length >= 3);
    assert.equal(result.config.targets[0].url, 'https://' + hostPlaceholder + '/hook');
});

test('target count and duration helpers enforce backend bounds', () => {
    const targets = Array.from({ length: configurator.MAX_TARGETS + 1 }, (_, index) => ({
        name: 'target-' + index,
        url: 'https://example.com/' + index
    }));
    const result = configurator.validateConfig({ version: 1, targets });

    assert.ok(codes(result).includes('tooManyTargets'));
    assert.equal(configurator.parseDurationMilliseconds('500ms'), 500);
    assert.equal(configurator.parseDurationMilliseconds('1m'), 60000);
    assert.equal(configurator.parseDurationMilliseconds('1m1s'), 61000);
    assert.equal(configurator.parseDurationMilliseconds('+5s'), 5000);
    assert.equal(configurator.parseDurationMilliseconds('-5s'), -5000);
    assert.equal(Number.isNaN(configurator.parseDurationMilliseconds('soon')), true);
});

test('pasted configurations use the same 1 MiB limit as OwlMail', () => {
    const oversized = 'x'.repeat(configurator.MAX_CONFIG_BYTES + 1);
    assert.deepEqual(codes(configurator.parseConfigText(oversized)), ['configTooLarge']);
});
