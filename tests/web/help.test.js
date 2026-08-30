const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { test } = require('bun:test');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, '../../web/help.js'), 'utf8');

function createHarness(savedLanguage) {
    const storage = new Map([['language', savedLanguage], ['theme', 'light']]);
    const listeners = new Map();
    const languageSelect = {
        value: '',
        addEventListener(name, handler) { listeners.set(name, handler); }
    };
    const themeButton = {
        textContent: '',
        addEventListener() {},
        setAttribute() {}
    };
    const panels = [
        { dataset: { language: 'en' }, hidden: false },
        { dataset: { language: 'zh-CN' }, hidden: false }
    ];
    const sandbox = {
        document: {
            title: '',
            documentElement: { lang: '', dataset: {} },
            body: { classList: { contains: () => false, toggle() {} } },
            getElementById(id) { return id === 'helpLanguage' ? languageSelect : themeButton; },
            querySelectorAll() { return panels; }
        },
        navigator: { language: 'en-US' },
        localStorage: {
            getItem(key) { return storage.get(key) || null; },
            setItem(key, value) { storage.set(key, value); }
        }
    };
    vm.createContext(sandbox);
    vm.runInContext(source, sandbox, { filename: 'web/help.js' });
    return { languageSelect, listeners, storage };
}

test('help fallback does not overwrite the inbox language preference', () => {
    const harness = createHarness('de');

    assert.equal(harness.languageSelect.value, 'en');
    assert.equal(harness.storage.get('language'), 'de');
});

test('an explicit help language change updates the shared preference', () => {
    const harness = createHarness('de');

    harness.listeners.get('change')({ target: { value: 'zh-CN' } });

    assert.equal(harness.storage.get('language'), 'zh-CN');
});
