const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { test } = require('bun:test');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, '../../web/help.js'), 'utf8');

function createHarness(savedLanguage, savedTheme = 'light', dark = false) {
    const storage = new Map([['language', savedLanguage], ['theme', savedTheme]]);
    const listeners = new Map();
    const languageSelect = {
        value: '',
        addEventListener(name, handler) { listeners.set(name, handler); }
    };
    const themeListeners = new Map();
    const themeButton = {
        textContent: '',
        addEventListener(name, handler) {themeListeners.set(name, handler);},
        setAttribute() {}
    };
    const panels = [
        { dataset: { language: 'en' }, hidden: false },
        { dataset: { language: 'zh-CN' }, hidden: false }
    ];
    const classes = new Set();
    const media = {matches: dark, addEventListener(name, handler) { this.change = handler; }};
    const windowListeners = new Map();
    const sandbox = {
        window: {matchMedia: () => media, addEventListener(name, handler) {windowListeners.set(name, handler);}},
        document: {
            title: '',
            documentElement: { lang: '', dataset: {} },
            body: { classList: { contains: name => classes.has(name), toggle(name, enabled) {if (enabled) classes.add(name); else classes.delete(name);} } },
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
    return { languageSelect, listeners, storage, classes, media, windowListeners, themeListeners };
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


test('help uses the system theme without saving an implicit preference', () => {
    const harness = createHarness('en', null, true);
    assert.equal(harness.classes.has('dark-theme'), true);
    assert.equal(harness.storage.get('theme'), null);
    harness.media.matches = false;
    harness.media.change();
    assert.equal(harness.classes.has('light-theme'), true);
    harness.storage.set('theme', 'dark');
    harness.windowListeners.get('storage')({key:'theme'});
    assert.equal(harness.classes.has('dark-theme'), true);
});


test('help preserves an explicit session theme when storage is blocked', () => {
    const harness = createHarness('en', null, true);
    harness.storage.get = harness.storage.set = () => {throw new Error('blocked');};
    harness.themeListeners.get('click')();
    assert.equal(harness.classes.has('light-theme'), true);
    harness.media.change();
    assert.equal(harness.classes.has('light-theme'), true);
});
