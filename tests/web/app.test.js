const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { test } = require('bun:test');
const vm = require('node:vm');

const appSource = fs.readFileSync(path.join(__dirname, '../../web/app.js'), 'utf8');

function createClassList() {
    const values = new Set();
    return {
        contains: (name) => values.has(name),
        toggle(name, enabled) {
            if (enabled) values.add(name);
            else values.delete(name);
        }
    };
}

function createElement() {
    const listeners = new Map();
    const attributes = new Map();
    return {
        listeners,
        attributes,
        classList: createClassList(),
        hidden: true,
        disabled: false,
        textContent: '',
        title: '',
        addEventListener(name, handler) { listeners.set(name, handler); },
        setAttribute(name, value) { attributes.set(name, value); }
    };
}

function createHarness({ permission = 'default', secure = true, savedPreference = null, serviceWorker = false } = {}) {
    const storage = new Map();
    if (savedPreference !== null) {
        storage.set('owlmail.browserNotifications.enabled', savedPreference);
    }
    const notificationToggle = createElement();
    const notificationStatus = createElement();
    const elements = new Map([
        ['notificationToggle', notificationToggle],
        ['notificationStatus', notificationStatus]
    ]);
    const notifications = [];
    const serviceWorkerNotifications = [];
    const windowListeners = new Map();
    const documentListeners = new Map();

    function Notification(title, options) {
        const instance = { title, options, closed: false, close() { this.closed = true; } };
        notifications.push(instance);
        return instance;
    }
    Notification.permission = permission;
    Notification.requestPermission = async () => {
        Notification.permission = 'granted';
        return 'granted';
    };

    const window = {
        Notification,
        isSecureContext: secure,
        location: { origin: 'http://owlmail.test', protocol: 'http:', host: 'owlmail.test' },
        addEventListener(name, handler) { windowListeners.set(name, handler); },
        focus() {}
    };
    const document = {
        title: '',
        documentElement: { lang: '', dataset: {} },
        body: { classList: createClassList() },
        addEventListener(name, handler) { documentListeners.set(name, handler); },
        getElementById(id) { return elements.get(id) || null; },
        querySelector() { return null; },
        querySelectorAll() { return []; },
        createElement() { return createElement(); }
    };
    const navigator = { language: 'en-US' };
    if (serviceWorker) {
        navigator.serviceWorker = {
            addEventListener() {},
            async register(path, options) {
                assert.equal(path, '/service-worker.js');
                assert.equal(options.scope, '/');
                return {
                    async showNotification(title, notificationOptions) {
                        serviceWorkerNotifications.push({ title, options: notificationOptions });
                    }
                };
            }
        };
    }
    const sandbox = {
        window,
        document,
        navigator,
        localStorage: {
            getItem(key) { return storage.has(key) ? storage.get(key) : null; },
            setItem(key, value) { storage.set(key, value); }
        },
        console,
        setTimeout() { return 1; },
        clearTimeout() {},
        fetch: async () => { throw new Error('unexpected fetch'); },
        WebSocket: function WebSocket() {},
        URL,
        Blob,
        confirm: () => true
    };
    vm.createContext(sandbox);
    vm.runInContext(appSource, sandbox, { filename: 'web/app.js' });

    return {
        documentListeners,
        notificationStatus,
        notificationToggle,
        notifications,
        serviceWorkerNotifications,
        storage,
        window,
        windowListeners,
        run(source) { return vm.runInContext(source, sandbox); }
    };
}

test('initialization never prompts and keeps notifications disabled by default', () => {
    const harness = createHarness();
    let permissionRequests = 0;
    harness.window.Notification.requestPermission = async () => {
        permissionRequests++;
        return 'granted';
    };

    harness.run('initializeBrowserNotifications()');

    assert.equal(permissionRequests, 0);
    assert.equal(harness.notificationToggle.attributes.get('aria-pressed'), 'false');
    assert.equal(harness.notificationToggle.listeners.has('click'), true);
});

test('user opt-in requests permission and persists the enabled preference', async () => {
    const harness = createHarness();
    harness.run('initializeBrowserNotifications()');

    await harness.run('toggleBrowserNotifications()');

    assert.equal(harness.window.Notification.permission, 'granted');
    assert.equal(harness.storage.get('owlmail.browserNotifications.enabled'), 'true');
    assert.equal(harness.notificationToggle.attributes.get('aria-pressed'), 'true');
});

test('a revoked permission clears a previously enabled preference', () => {
    const harness = createHarness({ permission: 'granted', savedPreference: 'true' });
    harness.run('initializeBrowserNotifications()');
    harness.window.Notification.permission = 'denied';

    harness.run('synchronizeBrowserNotificationPermission()');

    assert.equal(harness.storage.get('owlmail.browserNotifications.enabled'), 'false');
    assert.equal(harness.notificationToggle.attributes.get('aria-pressed'), 'false');
});

test('new email notification uses bounded text and a stable message tag', () => {
    const harness = createHarness({ permission: 'granted', savedPreference: 'true' });
    harness.run('initializeBrowserNotifications()');
    const longSubject = `  ${'subject '.repeat(30)}  `;

    harness.run(`notifyBrowserForEmail(${JSON.stringify({ id: 'mail-42', subject: longSubject, from: [] })})`);

    assert.equal(harness.notifications.length, 1);
    assert.equal(harness.notifications[0].title.length, 160);
    assert.equal(harness.notifications[0].title.endsWith('…'), true);
    assert.equal(harness.notifications[0].options.tag, 'owlmail-email-mail-42');
    assert.match(harness.notifications[0].options.body, /Unknown/);
});

test('mobile-compatible service worker displays new email notifications', async () => {
    const harness = createHarness({ permission: 'granted', savedPreference: 'true', serviceWorker: true });
    harness.run('initializeBrowserNotifications()');
    await harness.run('browserNotificationRegistrationPromise');

    harness.run(`notifyBrowserForEmail(${JSON.stringify({ id: 'mobile-42', subject: 'Mobile', from: [] })})`);
    await Promise.resolve();

    assert.equal(harness.notifications.length, 0);
    assert.equal(harness.serviceWorkerNotifications.length, 1);
    assert.equal(harness.serviceWorkerNotifications[0].options.data.emailId, 'mobile-42');
});

test('insecure contexts report notifications as unavailable', () => {
    const harness = createHarness({ permission: 'granted', secure: false, savedPreference: 'true' });

    harness.run('initializeBrowserNotifications()');

    assert.equal(harness.notificationToggle.disabled, true);
    assert.equal(harness.notificationToggle.attributes.get('aria-pressed'), 'false');
    assert.equal(harness.storage.get('owlmail.browserNotifications.enabled'), 'true');
});

test('address formatting tolerates null and undefined values', () => {
    const harness = createHarness();

    assert.equal(harness.run('formatAddress(null)'), 'Unknown');
    assert.equal(harness.run('formatAddress(undefined)'), 'Unknown');
    assert.equal(harness.run('formatAddress({ Name: "Sender", Address: "sender@example.test" })'), 'Sender <sender@example.test>');
});
