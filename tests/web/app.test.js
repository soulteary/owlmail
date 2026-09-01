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

function createHarness({ permission = 'default', secure = true, savedPreference = null, serviceWorker = false, fetchImpl } = {}) {
    const storage = new Map();
    if (savedPreference !== null) {
        storage.set('owlmail.browserNotifications.enabled', savedPreference);
    }
    const notificationToggle = createElement();
    const notificationStatus = createElement();
    const emailDetail = createElement();
    const elements = new Map([
        ['notificationToggle', notificationToggle],
        ['notificationStatus', notificationStatus],
        ['emailDetail', emailDetail]
    ]);
    const notifications = [];
    const windowListeners = new Map();
    const documentListeners = new Map();
    const serviceNotifications = [];

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
        location: { origin: 'http://owlmail.test', protocol: 'http:', host: 'owlmail.test', search: '' },
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
		const activeRegistration = {
			async showNotification(title, options) {
				serviceNotifications.push({ title, options });
			}
		};
        navigator.serviceWorker = {
            addEventListener() {},
            async register() {
				return { installing: {} };
			},
			ready: Promise.resolve(activeRegistration)
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
        fetch: fetchImpl || (async () => { throw new Error('unexpected fetch'); }),
        WebSocket: function WebSocket() {},
        URL,
        URLSearchParams,
        Blob,
        confirm: () => true
    };
    vm.createContext(sandbox);
    vm.runInContext(appSource, sandbox, { filename: 'web/app.js' });

    return {
        documentListeners,
        emailDetail,
        notificationStatus,
        notificationToggle,
        notifications,
        serviceNotifications,
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

test('new email notification uses bounded text and a stable message tag', async () => {
    const harness = createHarness({ permission: 'granted', savedPreference: 'true' });
    harness.run('initializeBrowserNotifications()');
    const longSubject = `  ${'subject '.repeat(30)}  `;

    await harness.run(`notifyBrowserForEmail(${JSON.stringify({ id: 'mail-42', subject: longSubject, from: [] })})`);

    assert.equal(harness.notifications.length, 1);
    assert.equal(harness.notifications[0].title.length, 160);
    assert.equal(harness.notifications[0].title.endsWith('…'), true);
    assert.equal(harness.notifications[0].options.tag, 'owlmail-email-mail-42');
    assert.match(harness.notifications[0].options.body, /Unknown/);
});

test('mobile-compatible notifications use the service worker registration', async () => {
    const harness = createHarness({ permission: 'granted', savedPreference: 'true', serviceWorker: true });
    harness.run('initializeBrowserNotifications()');

    await harness.run(`notifyBrowserForEmail({
        id: 'mobile-message',
        subject: 'Mobile message',
        from: [{ address: 'sender@example.com' }]
    })`);

    assert.equal(harness.serviceNotifications.length, 1);
    assert.equal(harness.serviceNotifications[0].title, 'Mobile message');
    assert.equal(harness.serviceNotifications[0].options.data.emailID, 'mobile-message');
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

test('English translations use singular forms', () => {
    const harness = createHarness();

    assert.equal(harness.run("t('emailCount', { count: 1 })"), '1 email');
    assert.equal(harness.run("t('attachments', { count: 1 })"), '1 attachment');
    assert.equal(harness.run("t('minutesAgo', { minutes: 1 })"), '1 minute ago');
    assert.equal(harness.run("t('emailCount', { count: 2 })"), '2 emails');
});

test('Traditional Chinese locales do not silently select Simplified Chinese', () => {
    const harness = createHarness();
    harness.run("navigator.language = 'zh-TW'");

    assert.equal(harness.run('detectLanguage()'), 'en');
});

test('Simplified Chinese Singapore locale remains supported', () => {
    const harness = createHarness();
    harness.run("navigator.language = 'zh-SG'");

    assert.equal(harness.run('detectLanguage()'), 'zh-CN');
});

test('initial language setup translates the empty email detail', () => {
    const harness = createHarness();

    harness.run("setLanguage('fr', false)");

    assert.match(harness.emailDetail.innerHTML, /Sélectionnez un email/);
});

test('mailbox lists use the preview endpoint and preserve query parameters', async () => {
    const requests = [];
    const harness = createHarness({
        fetchImpl: async (url) => {
            requests.push(url);
            return {
                ok: true,
                headers: { get: () => 'application/json' },
                json: async () => ({ emails: [], total: 0 })
            };
        }
    });

    await harness.run("API.getEmails(50, 25, 'release notes')");

    assert.equal(requests.length, 1);
    const requestURL = new URL(requests[0]);
    assert.equal(requestURL.pathname, '/api/v1/emails/preview');
    assert.equal(requestURL.searchParams.get('offset'), '50');
    assert.equal(requestURL.searchParams.get('limit'), '25');
    assert.equal(requestURL.searchParams.get('q'), 'release notes');
});

test('email details continue to use the single-email endpoint', async () => {
    const requests = [];
    const harness = createHarness({
        fetchImpl: async (url) => {
            requests.push(url);
            return {
                ok: true,
                headers: { get: () => 'application/json' },
                json: async () => ({ id: 'mail-42', html: '<p>full body</p>' })
            };
        }
    });

    await harness.run("API.getEmail('mail-42')");

    assert.equal(requests.length, 1);
    assert.equal(requests[0], 'http://owlmail.test/api/v1/emails/mail-42');
});
