const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { test } = require('bun:test');
const vm = require('node:vm');

const appSource = fs.readFileSync(path.join(__dirname, '../../web/app.js'), 'utf8');
const styleSource = fs.readFileSync(path.join(__dirname, '../../web/style.css'), 'utf8');

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

function createElement({ dataset = {} } = {}) {
    const listeners = new Map();
    const attributes = new Map();
    let innerHTML = '';
    let textContent = '';
    const element = {
        listeners,
        attributes,
        classList: createClassList(),
        dataset,
        style: {},
        scrollLeft: 0,
        scrollTop: 0,
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

function createHarness({ permission = 'default', secure = true, savedPreference = null, serviceWorker = false, fetchImpl = null, basePathname = '' } = {}) {
    const storage = new Map();
    if (savedPreference !== null) {
        storage.set('owlmail.browserNotifications.enabled', savedPreference);
    }
    const notificationToggle = createElement();
    const notificationStatus = createElement();
    const emailDetail = createElement();
    const emailList = createElement();
    const emailViewportFrame = createElement();
    const emailViewportStage = createElement();
    const emailViewportButtons = ['100%', '1440', '1024', '768', '425', '375', '320']
        .map((width) => createElement({ dataset: { viewportWidth: width } }));
    const elements = new Map([
        ['notificationToggle', notificationToggle],
        ['notificationStatus', notificationStatus],
        ['emailDetail', emailDetail],
        ['emailList', emailList],
        ['emailViewportFrame', emailViewportFrame],
        ['emailViewportStage', emailViewportStage]
    ]);
    const notifications = [];
    const fetchRequests = [];
    const windowListeners = new Map();
    const documentListeners = new Map();
    const serviceNotifications = [];
    const serviceWorkerRegistrations = [];
    const webSocketURLs = [];
    const historyCalls = [];

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
        location: {
            origin: 'http://owlmail.test',
            protocol: 'http:',
            host: 'owlmail.test',
            pathname: '/',
            search: '',
            hash: '',
            href: 'http://owlmail.test/'
        },
        history: {
            pushState(state, title, url) {
                historyCalls.push({ method: 'pushState', state, title, url: String(url) });
                const next = new URL(String(url), window.location.href);
                window.location.href = next.href;
                window.location.pathname = next.pathname;
                window.location.search = next.search;
                window.location.hash = next.hash;
            },
            replaceState(state, title, url) {
                historyCalls.push({ method: 'replaceState', state, title, url: String(url) });
                const next = new URL(String(url), window.location.href);
                window.location.href = next.href;
                window.location.pathname = next.pathname;
                window.location.search = next.search;
                window.location.hash = next.hash;
            }
        },
        addEventListener(name, handler) { windowListeners.set(name, handler); },
        focus() {}
    };
    const document = {
        title: '',
        documentElement: { lang: '', dataset: {} },
        body: { classList: createClassList() },
        addEventListener(name, handler) { documentListeners.set(name, handler); },
        getElementById(id) { return elements.get(id) || null; },
        querySelector(selector) {
            return selector === 'meta[name="owlmail-base-pathname"]' ? { content: basePathname } : null;
        },
        querySelectorAll(selector) {
            return selector === '.email-viewport-preset' ? emailViewportButtons : [];
        },
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
			async register(url, options) {
				serviceWorkerRegistrations.push({ url, options });
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
        WebSocket: function WebSocket(url) { webSocketURLs.push(url); },
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
        emailViewportButtons,
        emailViewportFrame,
        emailViewportStage,
        fetchRequests,
        historyCalls,
        notificationStatus,
        notificationToggle,
        notifications,
        serviceNotifications,
        serviceWorkerRegistrations,
        storage,
        window,
        windowListeners,
        webSocketURLs,
        run(source) { return vm.runInContext(source, sandbox); }
    };
}

test('browser routes use the configured base pathname', async () => {
    const harness = createHarness({
        basePathname: '/owlmail',
        serviceWorker: true,
        fetchImpl: async () => jsonResponse({ total: 0, limit: 50, offset: 0, previews: [] })
    });

    await harness.run('API.getEmailPreviews()');
    harness.run('connectWebSocket()');
    await harness.run('getNotificationServiceWorker()');

    assert.equal(new URL(harness.fetchRequests[0].url).pathname, '/owlmail/api/v1/emails/preview');
    assert.equal(harness.webSocketURLs[0], 'ws://owlmail.test/owlmail/api/v1/ws');
    assert.deepEqual(JSON.parse(JSON.stringify(harness.serviceWorkerRegistrations[0])), {
        url: '/owlmail/service-worker.js',
        options: { scope: '/owlmail/' }
    });
});

test('browser routes remain root-relative by default', () => {
    const harness = createHarness();
    harness.run('connectWebSocket()');
    assert.equal(harness.run('API_BASE'), 'http://owlmail.test/api/v1');
    assert.equal(harness.webSocketURLs[0], 'ws://owlmail.test/api/v1/ws');
});

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

test('HTML previews survive attribute parsing with a zero-permission sandbox', async () => {
    const harness = createHarness();
    const preview = harness.run(`renderHTML(
        '<form action="https://attacker.test"><a target="_top" href="https://attacker.test">leave</a></form>',
        'mail-42',
        []
    )`);

    assert.match(preview, /sandbox=""/);
    assert.match(preview, /referrerpolicy="no-referrer"/);
    assert.doesNotMatch(preview, /allow-scripts|allow-forms|allow-popups|allow-top-navigation/);

    let parsedSrcdoc = null;
    await new HTMLRewriter()
        .on('iframe', {
            element(element) {
                parsedSrcdoc = element.getAttribute('srcdoc');
            }
        })
        .transform(new Response(preview))
        .text();
    assert.match(parsedSrcdoc, /&lt;meta http-equiv=&quot;Content-Security-Policy&quot;/);
    assert.match(parsedSrcdoc, /&lt;meta name=&quot;referrer&quot; content=&quot;no-referrer&quot;&gt;/);
    assert.match(parsedSrcdoc, /&lt;form action=&quot;https:\/\/attacker\.test&quot;&gt;/);
    assert.match(parsedSrcdoc, /&lt;\/form&gt;$/);

    const csp = harness.run('previewContentSecurityPolicy(false)');
    assert.match(csp, /script-src 'none'/);
    assert.match(csp, /form-action 'none'/);
    assert.match(csp, /frame-src 'none'/);
});

test('HTML previews expose every responsive viewport preset', () => {
    const harness = createHarness();
    const preview = harness.run(`renderHTML('<p>Responsive message</p>', 'mail-42', [])`);

    assert.match(preview, /role="group" aria-label="Preview width"/);
    for (const width of ['100%', '1440', '1024', '768', '425', '375', '320']) {
        assert.equal(preview.includes(`data-viewport-width="${width}"`), true);
    }
    assert.match(preview, /style="width: 100%;"/);
    assert.match(preview, /sandbox=""/);
    assert.match(preview, /referrerpolicy="no-referrer"/);
});

test('changing the viewport resizes the existing frame without reloading or losing stage scroll', () => {
    const harness = createHarness();
    const preview = harness.run(`renderHTML('<p>Keep me</p>', 'mail-42', [])`);
    harness.emailDetail.innerHTML = preview;
    harness.emailViewportFrame.style.width = '100%';
    harness.emailViewportStage.scrollLeft = 47;
    harness.emailViewportStage.scrollTop = 91;

    harness.run("setEmailViewport('375')");

    assert.equal(harness.emailViewportFrame.style.width, '375px');
    assert.equal(harness.emailViewportStage.scrollLeft, 47);
    assert.equal(harness.emailViewportStage.scrollTop, 91);
    assert.equal(harness.emailDetail.innerHTML, preview);
    assert.equal(harness.fetchRequests.length, 0);
    assert.equal(harness.emailViewportButtons[5].attributes.get('aria-pressed'), 'true');
    assert.equal(harness.emailViewportButtons[0].attributes.get('aria-pressed'), 'false');
    assert.match(harness.emailDetail.innerHTML, /sandbox=""/);
    assert.match(harness.emailDetail.innerHTML, /referrerpolicy="no-referrer"/);

    harness.run("setEmailViewport('not-a-preset')");
    assert.equal(harness.emailViewportFrame.style.width, '375px');
});

test('viewport controls wrap into touch-sized rows on narrow screens', () => {
    const mobileStyles = styleSource.match(/@media \(max-width: 768px\)[\s\S]*?\/\* Scrollbar \*\//)?.[0] || '';

    assert.match(mobileStyles, /\.email-viewport-toolbar\s*\{[\s\S]*?flex-direction:\s*column/);
    assert.match(mobileStyles, /\.email-viewport-presets\s*\{[\s\S]*?width:\s*100%/);
    assert.match(mobileStyles, /\.email-viewport-preset\s*\{[\s\S]*?min-height:\s*38px/);
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

test('every supported language translates the viewport controls', () => {
    const harness = createHarness();

    assert.equal(harness.run(`Object.values(i18n).every((translations) =>
        Object.hasOwn(translations, 'emailViewportPresets')
        && Object.hasOwn(translations, 'emailViewportWidth')
    )`), true);
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


test('email selection updates browser history and popstate restores the message', async () => {
    const harness = createHarness({
        fetchImpl: async (url) => {
            const id = new URL(url).pathname.split('/').pop();
            return jsonResponse({ id, subject: id, from: [], to: [], attachments: [] });
        }
    });

    await harness.run("loadEmailDetail('mail-42')");
    assert.equal(harness.historyCalls.length, 1);
    assert.equal(harness.historyCalls[0].method, 'pushState');
    assert.equal(new URL(harness.historyCalls[0].url, 'http://owlmail.test').searchParams.get('email'), 'mail-42');

    harness.window.location.href = 'http://owlmail.test/?email=mail-17';
    harness.window.location.search = '?email=mail-17';
    harness.run('handleHistoryNavigation()');
    await new Promise((resolve) => setTimeout(resolve, 0));
    assert.equal(harness.run('state.currentEmail.id'), 'mail-17');

    harness.window.location.href = 'http://owlmail.test/';
    harness.window.location.search = '';
    harness.run('handleHistoryNavigation()');
    assert.equal(harness.run('state.currentEmail'), null);
});

test('mailbox keyboard navigation supports arrows, j/k, escape, and ignores inputs', async () => {
    const harness = createHarness({
        fetchImpl: async (url) => {
            const id = new URL(url).pathname.split('/').pop();
            return jsonResponse({ id, subject: id, from: [], to: [], attachments: [] });
        }
    });
    harness.run("state.emails = [{ id: 'mail-1' }, { id: 'mail-2' }]");

    await harness.run("handleMailboxKeydown({ key: 'j', target: { tagName: 'BODY' }, preventDefault() {} })");
    assert.equal(harness.run('state.currentEmail.id'), 'mail-1');

    await harness.run("handleMailboxKeydown({ key: 'ArrowDown', target: { tagName: 'BODY' }, preventDefault() {} })");
    assert.equal(harness.run('state.currentEmail.id'), 'mail-2');

    const requestsBeforeInput = harness.fetchRequests.length;
    harness.run("handleMailboxKeydown({ key: 'k', target: { tagName: 'INPUT' }, preventDefault() {} })");
    assert.equal(harness.fetchRequests.length, requestsBeforeInput);

    harness.run("handleMailboxKeydown({ key: 'Escape', target: { tagName: 'BODY' }, preventDefault() {} })");
    assert.equal(harness.run('state.currentEmail'), null);
    assert.equal(harness.window.location.search, '');
});


test('slower email detail responses cannot overwrite newer navigation', async () => {
    const pending = new Map();
    const harness = createHarness({
        fetchImpl: (url) => new Promise((resolve) => {
            const id = new URL(url).pathname.split('/').pop();
            pending.set(id, () => resolve(jsonResponse({ id, subject: id, from: [], to: [], attachments: [] })));
        })
    });

    const first = harness.run("loadEmailDetail('mail-slow')");
    const second = harness.run("loadEmailDetail('mail-fast')");
    pending.get('mail-fast')();
    await second;
    pending.get('mail-slow')();
    await first;

    assert.equal(harness.run('state.currentEmail.id'), 'mail-fast');
    assert.equal(new URL(harness.window.location.href).searchParams.get('email'), 'mail-fast');
});

test('reselecting the current email does not add a duplicate history entry', async () => {
    const harness = createHarness({
        fetchImpl: async (url) => {
            const id = new URL(url).pathname.split('/').pop();
            return jsonResponse({ id, subject: id, from: [], to: [], attachments: [] });
        }
    });

    await harness.run("loadEmailDetail('mail-42')");
    await harness.run("loadEmailDetail('mail-42')");

    assert.equal(harness.historyCalls.length, 1);
    assert.equal(harness.historyCalls[0].method, 'pushState');
});
