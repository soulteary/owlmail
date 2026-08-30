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

test('whitespace-only methods normalize to the backend default', () => {
    const result = configurator.parseConfigText(JSON.stringify({
        version: 1,
        targets: [{
            name: 'primary',
            url: 'https://example.com/hooks/owlmail',
            method: ' \u0085 '
        }]
    }));

    assert.deepEqual(codes(result), []);
    assert.equal(Object.hasOwn(result.config.targets[0], 'method'), false);
});

test('content types apply backend whitespace normalization before validation', () => {
    const result = configurator.parseConfigText(JSON.stringify({
        version: 1,
        targets: [{
            name: 'primary',
            url: 'https://example.com/hooks/owlmail',
            contentType: ' text/plain\r\n'
        }]
    }));

    assert.deepEqual(codes(result), []);
    assert.equal(result.config.targets[0].contentType, 'text/plain');
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
    assert.equal(configurator.parseDurationMilliseconds('1μs'), 0.001);
    assert.equal(configurator.parseDurationMilliseconds('0.000000001s'), 0.000001);
    assert.equal(configurator.parseDurationMilliseconds('0.0000000001s'), 0);
    assert.equal(Number.isNaN(configurator.parseDurationMilliseconds('soon')), true);
});

test('pasted configurations use the same 1 MiB limit as OwlMail', () => {
    const oversized = 'x'.repeat(configurator.MAX_CONFIG_BYTES + 1);
    assert.deepEqual(codes(configurator.parseConfigText(oversized)), ['configTooLarge']);
});

test('generated configurations use the same 1 MiB limit as OwlMail', () => {
    const result = configurator.validateGeneratedConfig({
        version: 1,
        targets: [{
            name: 'large',
            url: 'https://example.com/hook',
            bodyTemplate: 'x'.repeat(configurator.MAX_CONFIG_BYTES)
        }]
    });

    assert.ok(codes(result).includes('configTooLarge'));
});

test('URL validation defers environment-backed schemes and ports to runtime', () => {
    const schemePlaceholder = '$' + '{SCHEME}';
    const schemePrefixPlaceholder = '$' + '{SCHEME_PREFIX}';
    const separatorPlaceholder = '$' + '{URL_SEPARATOR}';
    const hostPlaceholder = '$' + '{HOST}';
    const ipv6PrefixPlaceholder = '$' + '{IPV6_PREFIX}';
    const portPlaceholder = '$' + '{PORT}';
    const basePlaceholder = '$' + '{BASE_URL}';
    const result = configurator.validateConfig({
        version: 1,
        targets: [
            { name: 'scheme', url: schemePlaceholder + '://example.com/hook' },
            { name: 'scheme-composed', url: schemePrefixPlaceholder + 'ps://example.com/hook' },
            { name: 'separator', url: 'https' + separatorPlaceholder + 'example.com/hook' },
            { name: 'port', url: 'https://example.com:' + portPlaceholder + '/hook' },
            { name: 'port-composed', url: 'https://example.com:' + portPlaceholder + '000/hook' },
            { name: 'ipv6', url: 'https://[' + hostPlaceholder + ']:' + portPlaceholder + '/hook' },
            { name: 'ipv6-composed', url: 'https://[' + ipv6PrefixPlaceholder + '1]/hook' },
            { name: 'base', url: basePlaceholder + '/hook' }
        ]
    });

    assert.deepEqual(codes(result), []);
    assert.equal(codes(result, 'warnings').filter((code) => code === 'envRuntime').length, 8);
});

test('URL validation still checks static authority with a placeholder scheme', () => {
    const schemePlaceholder = '$' + '{SCHEME}';
    const result = configurator.validateConfig({
        version: 1,
        targets: [{ name: 'missing-host', url: schemePlaceholder + ':///hook' }]
    });

    assert.ok(codes(result).includes('urlHost'));
});

test('base URL placeholders still reject static fragments', () => {
    const basePlaceholder = '$' + '{BASE_URL}';
    const result = configurator.validateConfig({
        version: 1,
        targets: [{ name: 'fragment', url: basePlaceholder + '/hook#secret' }]
    });

    assert.ok(codes(result).includes('urlFragment'));
});

test('URL validation rejects escapes that Go net/url cannot parse', () => {
    const result = configurator.validateConfig({
        version: 1,
        targets: [
            { name: 'bad-escape', url: 'https://example.com/%zz' },
            { name: 'escaped-host', url: 'https://%65xample.com/hook' }
        ]
    });

    assert.equal(codes(result).filter((code) => code === 'urlInvalid').length, 2);
});

test('URL validation preserves opaque raw-query percent values', () => {
    const result = configurator.validateConfig({
        version: 1,
        targets: [{ name: 'raw-query', url: 'https://example.com/hook?token=%zz' }]
    });

    assert.deepEqual(codes(result), []);
});

test('URL validation rejects normalized whitespace and opaque HTTP URLs', () => {
    const result = configurator.validateConfig({
        version: 1,
        targets: [
            { name: 'leading-space', url: ' https://example.com/hook' },
            { name: 'control', url: 'https://exa\nmple.com/hook' },
            { name: 'opaque', url: 'https:example.com' }
        ]
    });

    assert.equal(codes(result).filter((code) => code === 'urlInvalid').length, 2);
    assert.ok(codes(result).includes('urlHost'));
});

test('URL validation accepts and preserves trailing spaces in paths', () => {
    const url = 'https://example.com/hook ';
    const result = configurator.parseConfigText(JSON.stringify({
        version: 1,
        targets: [{ name: 'trailing-space', url }]
    }));

    assert.deepEqual(codes(result), []);
    assert.equal(result.config.targets[0].url, url);
    assert.ok(source.includes("url: value('[data-field=\"url\"]')"));
});

test('URL validation rejects Go-incompatible authority syntax', () => {
    const result = configurator.validateConfig({
        version: 1,
        targets: [
            { name: 'backslash', url: 'https://example.com\\hook' },
            { name: 'empty-user', url: 'https://@example.com/hook' },
            { name: 'empty-user-password', url: 'https://:@example.com/hook' },
            { name: 'opening-brace', url: 'https://example{.com/hook' },
            { name: 'closing-brace', url: 'https://example}.com/hook' },
            { name: 'backtick', url: 'https://example`.com/hook' }
        ]
    });

    assert.equal(codes(result).filter((code) => code === 'urlInvalid').length, 4);
    assert.equal(codes(result).filter((code) => code === 'urlCredentials').length, 2);
});

test('URL validation accepts Go-compatible IPv6 zone identifiers', () => {
    const result = configurator.validateConfig({
        version: 1,
        targets: [{ name: 'scoped-ipv6', url: 'https://[fe80::1%25eth0]/hook' }]
    });

    assert.deepEqual(codes(result), []);
});

test('IPv6 zone normalization stays inside a balanced URL authority', () => {
    const malformedAuthority = configurator.validateConfig({
        version: 1,
        targets: [{ name: 'unbalanced-zone', url: 'https://[::1%25x#]' }]
    });
    const hostPlaceholder = '$' + '{HOST}';
    const pathFragment = configurator.validateConfig({
        version: 1,
        targets: [{
            name: 'path-fragment',
            url: 'https://[' + hostPlaceholder + '%25zone]/[path%25x#]'
        }]
    });

    assert.ok(codes(malformedAuthority).includes('urlInvalid'));
    assert.ok(codes(pathFragment).includes('urlFragment'));
});

test('glob validation follows Go path.Match character-class grammar', () => {
    for (const pattern of ['[a-]', '[-a]', '[a-b-c]']) {
        assert.equal(configurator.validGlobPattern(pattern), false, pattern + ' should be rejected');
    }
    for (const pattern of ['[a-z]', '[\\-]']) {
        assert.equal(configurator.validGlobPattern(pattern), true, pattern + ' should be accepted');
    }
});

test('target names use the backend UTF-8 byte limit', () => {
    const valid = configurator.validateConfig({
        version: 1,
        targets: [{ name: '猫'.repeat(33), url: 'https://example.com/hook' }]
    });
    const oversized = configurator.validateConfig({
        version: 1,
        targets: [{ name: '猫'.repeat(40), url: 'https://example.com/hook' }]
    });

    assert.deepEqual(codes(valid), []);
    assert.ok(codes(oversized).includes('nameInvalid'));
    assert.equal(configurator.utf8ByteLength('猫'.repeat(33)), 99);
});

test('name and pattern whitespace normalization follows Go strings.TrimSpace', () => {
    const nextLine = '\u0085';
    const invalid = configurator.validateConfig({
        version: 1,
        targets: [{
            name: nextLine,
            url: 'https://example.com/hook',
            match: { subject: [nextLine] }
        }]
    });
    const parsed = configurator.parseConfigText(JSON.stringify({
        version: 1,
        targets: [{
            name: nextLine + 'primary' + nextLine,
            url: 'https://example.com/hook',
            method: nextLine + 'post' + nextLine,
            contentType: nextLine + 'application/json' + nextLine
        }]
    }));

    assert.ok(codes(invalid).includes('nameRequired'));
    assert.ok(codes(invalid).includes('matchPattern'));
    assert.deepEqual(codes(parsed), []);
    assert.equal(parsed.config.targets[0].name, 'primary');
    assert.equal(Object.hasOwn(parsed.config.targets[0], 'method'), false);
    assert.equal(Object.hasOwn(parsed.config.targets[0], 'contentType'), false);
    assert.equal(configurator.goTrimSpace('\uFEFF'), '\uFEFF');
});

test('imported match patterns retain significant whitespace and embedded newlines', () => {
    const patterns = [' leading*', 'trailing* ', 'line\nbreak'];
    const parsed = configurator.parseConfigText(JSON.stringify({
        version: 1,
        targets: [{
            name: 'preserved',
            url: 'https://example.com/hook',
            match: { subject: patterns }
        }]
    }));
    const displayValue = patterns.join('\n');

    assert.deepEqual(Array.from(parsed.config.targets[0].match.subject), patterns);
    assert.deepEqual(
        configurator.patternsFromEditorValue(displayValue, { displayValue, patterns }),
        patterns
    );
    assert.deepEqual(
        Array.from(configurator.patternsFromEditorValue(' leading*\ntrailing* ', null)),
        [' leading*', 'trailing* ']
    );
});

test('unmodified secret and template values survive control normalization', () => {
    const normalizedDisplay = 'line\nvalue';
    const originalValue = 'line\r\nvalue';
    const preserved = { displayValue: normalizedDisplay, value: originalValue };

    assert.equal(configurator.editorValueFromPreserved(normalizedDisplay, preserved), originalValue);
    assert.equal(configurator.editorValueFromPreserved('edited', preserved), 'edited');
});

test('sub-nanosecond timeouts are rejected after Go-compatible quantization', () => {
    const result = configurator.validateConfig({
        version: 1,
        targets: [{
            name: 'tiny-timeout',
            url: 'https://example.com/hook',
            timeout: '0.0000000001s'
        }]
    });

    assert.ok(codes(result).includes('timeoutRange'));
});

test('generated header maps preserve Object prototype property names', () => {
    const headers = configurator.createHeaderMap();
    headers.__proto__ = 'kept';

    assert.equal(Object.prototype.hasOwnProperty.call(headers, '__proto__'), true);
    assert.equal(JSON.parse(JSON.stringify(headers)).__proto__, 'kept');
});

test('HTTP field values reject control bytes while allowing horizontal tabs', () => {
    const result = configurator.validateConfig({
        version: 1,
        targets: [{
            name: 'header-controls',
            url: 'https://example.com/hook',
            headers: {
                'X-Nul': 'a\u0000b',
                'X-Del': 'a\u007Fb',
                'X-Tab': 'a\tb'
            },
            contentType: 'application/json\u0000'
        }]
    });

    assert.equal(codes(result).filter((code) => code === 'headerControl').length, 2);
    assert.equal(codes(result).filter((code) => code === 'contentTypeControl').length, 1);
    assert.equal(configurator.hasInvalidHTTPFieldValue('a\tb'), false);
});
