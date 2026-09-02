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
    let innerHTML = '';
    let textContent = '';
    const element = {
        listeners,
        attributes,
        classList: createClassList(),
        hidden: true,
        disabled: false,
        title: '',
        addEventListener(name, handler) { listeners.set(name, handler); },
        setAttribute(name, value) { attributes.set(name, value); },
        querySelectorAll() { return []; }
    };
    Object.defineProperties(element, {
        innerHTML: {
            get() { return innerHTML; },
            set(value) { innerHTML = String(value); }
        },
        textContent: {
            get() { return textContent; },
            set(value) {
                textContent = String(value);
                innerHTML = textContent
                    .replaceAll('&', '&amp;')
                    .replaceAll('<', '&lt;')
                    .replaceAll('>', '&gt;');
            }
        }
    });
    return element;
}

function jsonResponse(data) {
    return {
        ok: true,
        status: 200,
        headers: { get: (name) => name.toLowerCase() === 'content-type' ? 'application/json' : null },
        async json() { return data; },
        async text() { return JSON.stringify(data); }
    };
}

function createHarness({ permission = 'default', secure = true, savedPreference = null, serviceWorker = false, fetchImpl = null } = {}) {
    const storage = new Map();
    if (savedPreference !== null) {
        storage.set('owlmail.browserNotifications.enabled', savedPreference);
    }
    const notificationToggle = createElement();
    const notificationStatus = createElement();
    const emailDetail = createElement();
    const emailList = createElement();
    const elements = new Map([
        ['notificationToggle', notificationToggle],
        ['notificationStatus', notificationStatus],
        ['emailDetail', emailDetail],
        ['emailList', emailList]
    ]);
    const notifications = [];
    const fetchRequests = [];
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
        fetch: async (url, options) => {
            fetchRequests.push({ url: String(url), options });
            if (!fetchImpl) throw new Error('unexpected fetch');
            return fetchImpl(url, options);
        },
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
        emailList,
        fetchRequests,
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

test('mailbox lists use the preview endpoint and consume preview results', async () => {
    const harness = createHarness({
        fetchImpl: async () => jsonResponse({
            total: 1,
            limit: 25,
            offset: 50,
            previews: [{
                id: 'mail-42',
                time: '2026-09-01T00:00:00Z',
                read: false,
                subject: 'Quarterly report',
                from: 'sender@example.test',
                preview: 'Compact body preview',
                hasAttachment: true
            }]
        })
    });

    await harness.run(`
        state.currentPage = 2;
        state.pageSize = 25;
        state.searchQuery = 'quarterly report';
        loadEmails();
    `);

    assert.equal(harness.fetchRequests.length, 1);
    const requestURL = new URL(harness.fetchRequests[0].url);
    assert.equal(requestURL.pathname, '/api/v1/emails/preview');
    assert.equal(requestURL.searchParams.get('offset'), '50');
    assert.equal(requestURL.searchParams.get('limit'), '25');
    assert.equal(requestURL.searchParams.get('q'), 'quarterly report');
    assert.equal(harness.run('state.emails[0].id'), 'mail-42');
    assert.equal(harness.run('state.emails[0].read'), false);
    assert.equal(harness.run('state.emails[0].time'), '2026-09-01T00:00:00Z');
    assert.equal(harness.run('state.emails[0].preview'), 'Compact body preview');
    assert.equal(harness.run('state.total'), 1);
    assert.match(harness.emailList.innerHTML, /email-item unread/);
    assert.match(harness.emailList.innerHTML, /sender@example\.test/);
    assert.match(harness.emailList.innerHTML, /Compact body preview/);
    assert.match(harness.emailList.innerHTML, /📎/);
});

test('selected messages still use the single-email detail endpoint', async () => {
    const harness = createHarness({
        fetchImpl: async () => jsonResponse({
            id: 'mail-42',
            subject: 'Quarterly report',
            html: '<p>Full message body</p>'
        })
    });

    await harness.run("loadEmailDetail('mail-42')");

    assert.equal(harness.fetchRequests.length, 1);
    const requestURL = new URL(harness.fetchRequests[0].url);
    assert.equal(requestURL.pathname, '/api/v1/emails/mail-42');
    assert.equal(harness.run('state.currentEmail.html'), '<p>Full message body</p>');
});

test('HTML previews use a zero-permission sandbox and no-referrer policy', () => {
    const harness = createHarness();
    const preview = harness.run(`renderHTML(
        '<form action="https://attacker.test"><a target="_top" href="https://attacker.test">leave</a></form>',
        'mail-42',
        []
    )`);

    assert.match(preview, /sandbox=""/);
    assert.match(preview, /referrerpolicy="no-referrer"/);
    assert.doesNotMatch(preview, /allow-scripts|allow-forms|allow-popups|allow-top-navigation/);
    const csp = harness.run('previewContentSecurityPolicy(false)');
    assert.match(csp, /script-src 'none'/);
    assert.match(csp, /form-action 'none'/);
    assert.match(csp, /frame-src 'none'/);
});

test('remote tracking resources are blocked until the user explicitly loads them', () => {
    const harness = createHarness();
    harness.run(`
        state.currentEmail = {
            id: 'mail-42',
            subject: 'Tracking test',
            from: [],
            to: [],
            attachments: [],
            html: '<img src="https://tracker.example.test/pixel.gif" width="1" height="1"><link rel="stylesheet" href="https://cdn.example.test/mail.css">'
        };
        renderEmailDetail();
    `);

    assert.match(harness.emailDetail.innerHTML, /Load remote content/);
    const blockedCSP = harness.run('previewContentSecurityPolicy(false)');
    assert.match(blockedCSP, /img-src data: blob: http:\/\/owlmail\.test/);
    assert.doesNotMatch(blockedCSP, /img-src http: https:/);
    assert.doesNotMatch(blockedCSP, /style-src[^;]* http: https:/);

    harness.run("loadRemoteContent('mail-42')");

    assert.equal(harness.run('remoteContentAllowedEmailID'), 'mail-42');
    assert.doesNotMatch(harness.emailDetail.innerHTML, /Load remote content/);
    const allowedCSP = harness.run('previewContentSecurityPolicy(true)');
    assert.match(allowedCSP, /img-src http: https:/);
    assert.match(allowedCSP, /font-src http: https:/);
    assert.match(allowedCSP, /style-src[^;]* http: https:/);
});

test('CID images resolve to local attachments without enabling remote content', () => {
    const harness = createHarness();
    const resolved = harness.run(`resolveCIDReferences(
        '<table style="width: 100%"><tr><td><img src="cid:logo@example.test"></td></tr></table>',
        'mail-42',
        [{ contentId: 'logo@example.test', generatedFileName: 'logo image.png' }]
    )`);

    assert.match(resolved, /\/api\/v1\/emails\/mail-42\/attachments\/logo%20image\.png/);
    assert.equal(harness.run(`hasRemoteEmailResources(${JSON.stringify(resolved)})`), false);
    const preview = harness.run(`renderHTML(${JSON.stringify(resolved)}, 'mail-42', [])`);
    assert.doesNotMatch(preview, /Load remote content/);
    assert.match(harness.run('previewContentSecurityPolicy(false)'), /http:\/\/owlmail\.test/);
});

test('remote detection covers every srcset candidate while ignoring local resources', () => {
    const harness = createHarness();
    assert.equal(harness.run(`hasRemoteEmailResources(
        '<img srcset="http://owlmail.test/local.png 1x, https://tracker.example.test/remote.png 2x">'
    )`), true);
    assert.equal(harness.run(`hasRemoteEmailResources(
        '<img src="http://owlmail.test/api/v1/emails/mail-42/attachments/local.png">'
    )`), false);
});

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
