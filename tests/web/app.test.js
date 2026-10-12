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
        add(...names) { names.forEach((name) => values.add(name)); },
        contains: (name) => values.has(name),
        remove(...names) { names.forEach((name) => values.delete(name)); },
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
        focused: false,
        title: '',
        addEventListener(name, handler) { listeners.set(name, handler); },
        focus() { this.focused = true; },
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
    const deleteAllBtn = createElement();
    const emailDetail = createElement();
    const emailList = createElement();
    const emailViewportFrame = createElement();
    const emailViewportStage = createElement();
    const emailSourceTab = createElement();
    const emailViewportButtons = ['100%', '1440', '1024', '768', '425', '375', '320']
        .map((width) => createElement({ dataset: { viewportWidth: width } }));
    const elements = new Map([
        ['notificationToggle', notificationToggle],
        ['notificationStatus', notificationStatus],
        ['deleteAllBtn', deleteAllBtn],
        ['emailDetail', emailDetail],
        ['emailList', emailList],
        ['emailViewportFrame', emailViewportFrame],
        ['emailViewportStage', emailViewportStage],
        ['email-content-tab-source', emailSourceTab],
        ['emailInspectionRun', createElement()],
        ['emailInspectionStatus', createElement()],
        ['emailDeliveriesRefresh', createElement()],
        ['emailDeliveriesPanel', createElement()],
        ['emailRelayActions', createElement()],
        ['emailDeleteAction', createElement()]
    ]);
    const notifications = [];
    const fetchRequests = [];
    const windowListeners = new Map();
    const documentListeners = new Map();
    const serviceNotifications = [];
    const serviceWorkerRegistrations = [];
    const webSocketURLs = [];
    const historyCalls = [];
    const alerts = [];
    const confirmations = [];

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
		activeElement: null,
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
	for (const [id, element] of elements) {
        element.id = id;
		element.focus = function focus() {
			this.focused = true;
			document.activeElement = this;
		};
	}
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
        setTimeout(callback, delay = 0) { if (delay <= 1000) Promise.resolve().then(callback); return 1; },
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
        AbortController,
        TextEncoder,
        alert(message) { alerts.push(String(message)); },
        confirm(message) { confirmations.push(String(message)); return true; }
    };
    vm.createContext(sandbox);
    vm.runInContext(appSource, sandbox, { filename: 'web/app.js' });

    return {
        alerts,
        confirmations,
        deleteAllBtn,
        documentListeners,
        emailDetail,
        emailList,
        emailViewportButtons,
        emailViewportFrame,
        emailViewportStage,
        emailSourceTab,
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
            total: 51,
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
    assert.equal(harness.run('state.total'), 51);
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

test('manual relay controls render only when outgoing mail is enabled', () => {
    const harness = createHarness();
    harness.run(`state.currentEmail = { id: 'mail-1', subject: 'hello', from: [], to: [], cc: [], attachments: [], text: 'body' }; relayEnabled = false; renderEmailDetail();`);
    assert.equal(harness.emailDetail.innerHTML.includes('Relay to original recipients'), false);

    harness.run(`relayEnabled = true; renderEmailDetail();`);
    assert.equal(harness.emailDetail.innerHTML.includes('Relay to original recipients'), true);
    assert.equal(harness.emailDetail.innerHTML.includes('Relay to…'), true);
});

test('manual relay confirms envelope recipients and stays pending until completion', async () => {
    let resolveStatus;
    const statusResponse = new Promise((resolve) => { resolveStatus = resolve; });
    const harness = createHarness({
        fetchImpl: async (url, options) => {
            if (String(url).endsWith('/actions/relay/preflight')) {
                return jsonResponse({ data: { recipients: ['hidden@example.test', 'redirect@example.test'] } });
            }
            if (options && options.method === 'POST') {
                return jsonResponse({ data: { job: { id: 'job-1', status: 'queued' }, statusUrl: '/api/v1/relay-jobs/job-1' } });
            }
            return statusResponse;
        }
    });
    harness.run(`
        relayEnabled = true;
        state.currentEmail = {
            id: 'mail-1', subject: 'hello', from: [], to: [], cc: [], attachments: [], text: 'body',
            envelope: { to: ['hidden@example.test', 'redirect@example.test'] }
        };
    `);

    const relay = harness.run(`relayCurrentEmail('mail-1')`);
    await new Promise((resolve) => setTimeout(resolve, 0));
    assert.match(harness.confirmations[0], /hidden@example\.test, redirect@example\.test/);
    assert.equal(harness.run(`relayPending.has('mail-1')`), true);
    assert.equal(harness.fetchRequests.length, 3);
    await harness.run(`relayCurrentEmail('mail-1')`);
    assert.equal(harness.fetchRequests.length, 3);

    resolveStatus(jsonResponse({ data: { id: 'job-1', status: 'succeeded' } }));
    await relay;
    assert.equal(harness.run(`relayPending.has('mail-1')`), false);
});

test('manual relay aborts when the selection changes during preflight', async () => {
    let resolvePreflight;
    const preflightResponse = new Promise((resolve) => { resolvePreflight = resolve; });
    const harness = createHarness({
        fetchImpl: async (url) => {
            if (String(url).endsWith('/actions/relay/preflight')) return preflightResponse;
            throw new Error('relay submission must not run after the selection changes');
        }
    });
    harness.run(`
        relayEnabled = true;
        state.currentEmail = { id: 'mail-1', envelope: { to: ['recipient@example.test'] } };
    `);

    const relay = harness.run(`relayCurrentEmail('mail-1')`);
    await new Promise((resolve) => setTimeout(resolve, 0));
    harness.run(`state.currentEmail = { id: 'mail-2', envelope: { to: ['other@example.test'] } };`);
    resolvePreflight(jsonResponse({ data: { recipients: ['recipient@example.test'] } }));
    await relay;

    assert.equal(harness.confirmations.length, 0);
    assert.equal(harness.fetchRequests.length, 1);
    assert.equal(harness.run(`relayPending.has('mail-1')`), false);
});

test('manual relay refuses an unknown original envelope', async () => {
    const harness = createHarness();
    harness.run(`relayEnabled = true; state.currentEmail = { id: 'mail-1', envelope: null }`);

    await harness.run(`relayCurrentEmail('mail-1')`);

    assert.equal(harness.fetchRequests.length, 0);
    assert.match(harness.alerts[0], /no SMTP envelope recipients/);
});

test('manual relay confirms only recipients allowed by outgoing rules', async () => {
    let preflightAttempts = 0;
    const harness = createHarness({
        fetchImpl: async (url, options) => {
            if (String(url).endsWith('/actions/relay/preflight')) {
                preflightAttempts++;
                return jsonResponse({ data: { recipients: preflightAttempts === 1 ? ['allowed@example.test'] : [] } });
            }
            return options && options.method === 'POST'
                ? jsonResponse({ data: { job: { id: 'job-1', status: 'queued' }, statusUrl: '/api/v1/relay-jobs/job-1' } })
                : jsonResponse({ data: { id: 'job-1', status: 'succeeded' } });
        }
    });
    harness.run(`
        relayEnabled = true;
        state.currentEmail = {
            id: 'mail-1',
            envelope: { to: ['allowed@example.test', 'blocked@example.test'] }
        };
    `);

    await harness.run(`relayCurrentEmail('mail-1')`);

    assert.match(harness.confirmations[0], /allowed@example\.test/);
    assert.doesNotMatch(harness.confirmations[0], /blocked@example\.test/);
    assert.equal(harness.fetchRequests.length, 3);
    assert.deepEqual(JSON.parse(harness.fetchRequests[1].options.body).confirmedRecipients, ['allowed@example.test']);

    await harness.run(`relayCurrentEmail('mail-1')`);
    assert.equal(harness.fetchRequests.length, 4);
    assert.match(harness.alerts.at(-1), /relay rules exclude every/);
});

test('manual relay retries transient status failures', async () => {
    let statusAttempts = 0;
    const harness = createHarness({
        fetchImpl: async (url, options) => {
            if (String(url).endsWith('/actions/relay/preflight')) {
                return jsonResponse({ data: { recipients: ['recipient@example.test'] } });
            }
            if (options && options.method === 'POST') {
                return jsonResponse({ data: { job: { id: 'job-1', status: 'queued' }, statusUrl: '/api/v1/relay-jobs/job-1' } });
            }
            statusAttempts++;
            if (statusAttempts === 1) {
                return {
                    ...jsonResponse({ message: 'temporarily unavailable' }),
                    ok: false,
                    status: 503
                };
            }
            return jsonResponse({ data: { id: 'job-1', status: 'succeeded' } });
        }
    });
    harness.run(`
        relayEnabled = true;
        state.currentEmail = { id: 'mail-1', envelope: { to: ['recipient@example.test'] } };
    `);

    await harness.run(`relayCurrentEmail('mail-1')`);

    assert.equal(statusAttempts, 2);
    assert.equal(harness.run(`relayPending.has('mail-1')`), false);
    assert.equal(harness.alerts.length, 1);
});

test('delete controls stay disabled while relay delivery is pending', async () => {
    let resolveStatus;
    const statusResponse = new Promise((resolve) => { resolveStatus = resolve; });
    const harness = createHarness({
        fetchImpl: async (url, options) => {
            if (String(url).endsWith('/actions/relay/preflight')) {
                return jsonResponse({ data: { recipients: ['recipient@example.test'] } });
            }
            if (options && options.method === 'POST') {
                return jsonResponse({ data: { job: { id: 'job-1', status: 'queued' }, statusUrl: '/api/v1/relay-jobs/job-1' } });
            }
            return statusResponse;
        }
    });
    harness.run(`
        relayEnabled = true;
        state.currentEmail = {
            id: 'mail-1', subject: 'hello', from: [], to: [], cc: [], attachments: [], text: 'body',
            envelope: { to: ['recipient@example.test'] }
        };
    `);

    const relay = harness.run(`relayCurrentEmail('mail-1')`);
    await new Promise((resolve) => setTimeout(resolve, 0));
    assert.equal(harness.deleteAllBtn.disabled, true);
    assert.equal(harness.run("document.getElementById('emailDeleteAction').disabled"), true);
    harness.run("emailContentTab = 'inspect'; document.activeElement = { id: 'verification-input', selectionStart: 3 }; const original = renderEmailDetail; renderEmailDetail = () => { original(); document.activeElement = null; };");
    const requestCount = harness.fetchRequests.length;
    await harness.run(`deleteEmail('mail-1')`);
    await harness.run(`deleteAllEmails()`);
    assert.equal(harness.fetchRequests.length, requestCount);

    resolveStatus(jsonResponse({ data: { id: 'job-1', status: 'succeeded' } }));
    await relay;
    assert.equal(harness.deleteAllBtn.disabled, false);
    assert.equal(harness.run("document.activeElement.selectionStart"), 3);
});

test('email detail offers HTML, text, headers, and source tabs', () => {
    const harness = createHarness();
    const markup = harness.run(`renderEmailContentTabs({ id: 'mail-1', html: '<p>hello</p>', text: 'hello', headers: { subject: 'hello' }, attachments: [] })`);
    for (const tab of ['HTML', 'Plain text', 'Headers', 'Source']) {
        assert.equal(markup.includes(tab), true);
    }
    assert.match(markup, /role="tablist" aria-label="Email content views"/);
    assert.match(markup, /role="tabpanel"/);
    assert.match(markup, /tabindex="0"/);
    assert.match(markup, /onclick="setEmailContentTab\('source', true\)"/);
    for (const locale of ['zh-CN', 'en', 'de', 'it', 'fr', 'ko', 'ja']) {
        assert.equal(harness.run(`Boolean(i18n[${JSON.stringify(locale)}].contentSource)`), true);
        assert.equal(harness.run(`Boolean(i18n[${JSON.stringify(locale)}].emailContentViews)`), true);
    }
});

test('email content tabs support arrow-key navigation', () => {
    const harness = createHarness();
    harness.run(`state.currentEmail = { id: 'mail-1', html: '', text: 'hello', headers: {}, attachments: [] }; emailContentTab = 'text'`);
    let prevented = false;
    harness.run(`handleEmailContentTabKeydown({ key: 'ArrowRight', preventDefault() { globalThis.prevented = true; } }, 'text')`);
    prevented = harness.run('globalThis.prevented');
    assert.equal(prevented, true);
    assert.equal(harness.run('emailContentTab'), 'headers');
});

test('source load failures replace the loading placeholder', async () => {
    const harness = createHarness({
        fetchImpl: async () => ({
            ok: false,
            status: 500,
            headers: { get: () => 'application/json' },
            async json() { return { message: 'source unavailable' }; },
            async text() { return '{"message":"source unavailable"}'; }
        })
    });
    harness.run(`state.currentEmail = { id: 'mail-1', html: '', text: 'hello', headers: {}, attachments: [] }`);
    await harness.run(`setEmailContentTab('source')`);
    assert.equal(harness.emailDetail.innerHTML.includes('Loading source'), false);
    assert.equal(harness.emailDetail.innerHTML.includes('source unavailable'), true);
});

test('source retry clears the prior error before rendering loading state', async () => {
    let attempt = 0;
    let resolveRetry;
    const retryResponse = new Promise((resolve) => { resolveRetry = resolve; });
    const harness = createHarness({
        fetchImpl: async () => {
            attempt++;
            if (attempt === 1) {
                return {
                    ok: false,
                    status: 500,
                    headers: { get: () => 'application/json' },
                    async json() { return { message: 'source unavailable' }; },
                    async text() { return '{"message":"source unavailable"}'; }
                };
            }
            return retryResponse;
        }
    });
    harness.run(`state.currentEmail = { id: 'mail-1', size: 128, html: '', text: 'hello', headers: {}, attachments: [] }`);
    await harness.run(`setEmailContentTab('source')`);

    const retry = harness.run(`setEmailContentTab('source')`);
    assert.equal(harness.emailDetail.innerHTML.includes('source unavailable'), false);
    assert.equal(harness.emailDetail.innerHTML.includes('Loading source'), true);
    resolveRetry({ ok: true, status: 200, body: null, headers: { get: () => null }, async text() { return 'source'; } });
    await retry;
});

test('source requests are deduplicated and keyboard focus survives completion', async () => {
    let resolveFetch;
    const response = new Promise((resolve) => { resolveFetch = resolve; });
    const harness = createHarness({ fetchImpl: async () => response });
    harness.run(`state.currentEmail = { id: 'mail-1', size: 128, html: '', text: 'hello', headers: {}, attachments: [] }`);

    const first = harness.run(`setEmailContentTab('source', true)`);
    const second = harness.run(`setEmailContentTab('source', true)`);
    assert.equal(harness.fetchRequests.length, 1);
    resolveFetch({
        ok: true,
        status: 200,
        body: null,
        headers: { get: () => null },
        async text() { return 'Subject: hello\r\n\r\nbody'; }
    });
    await Promise.all([first, second]);

    assert.equal(harness.run(`emailSourceCache.get('mail-1')`), 'Subject: hello\r\n\r\nbody');
    assert.equal(harness.run('emailSourceRequests.size'), 0);
    assert.equal(harness.emailSourceTab.focused, true);
});

test('source completion does not steal focus moved to another control', async () => {
    let resolveFetch;
    const response = new Promise((resolve) => { resolveFetch = resolve; });
    const harness = createHarness({ fetchImpl: async () => response });
    harness.run(`state.currentEmail = { id: 'mail-1', size: 128, html: '', text: 'hello', headers: {}, attachments: [] }`);

    const loading = harness.run(`setEmailContentTab('source', true)`);
    harness.emailSourceTab.focused = false;
    harness.run(`document.activeElement = { id: 'another-control' }`);
    resolveFetch({
        ok: true,
        status: 200,
        body: null,
        headers: { get: () => null },
        async text() { return 'Subject: hello\r\n\r\nbody'; }
    });
    await loading;

    assert.equal(harness.emailSourceTab.focused, false);
    assert.equal(harness.run(`document.activeElement.id`), 'another-control');
});

test('large sources use the standalone view without an inline fetch', async () => {
    const harness = createHarness();
    harness.run(`state.currentEmail = { id: 'large-mail', size: EMAIL_SOURCE_INLINE_MAX_BYTES + 1, html: '', text: '', headers: {}, attachments: [] }`);

    await harness.run(`setEmailContentTab('source')`);

    assert.equal(harness.fetchRequests.length, 0);
    assert.equal(harness.run(`emailSourceOversized.has('large-mail')`), true);
    assert.match(harness.emailDetail.innerHTML, /too large to display safely inline/);
    assert.match(harness.emailDetail.innerHTML, /View Source/);
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

test('delayed initial deep link cannot supersede newer history navigation', async () => {
    let resolvePreview;
    const detailRequests = [];
    const harness = createHarness({
        fetchImpl: async (url) => {
            const requestURL = new URL(url);
            if (requestURL.pathname.endsWith('/settings/outgoing')) {
                return jsonResponse({ enabled: false });
            }
            if (requestURL.pathname.endsWith('/emails/preview')) {
                return new Promise((resolve) => {
                    resolvePreview = () => resolve(jsonResponse({ previews: [], total: 0 }));
                });
            }
            const id = requestURL.pathname.split('/').pop();
            detailRequests.push(id);
            return jsonResponse({ id, subject: id, from: [], to: [], attachments: [] });
        }
    });
    harness.window.location.href = 'http://owlmail.test/?email=mail-a';
    harness.window.location.search = '?email=mail-a';

    harness.documentListeners.get('DOMContentLoaded')();
    harness.window.location.href = 'http://owlmail.test/?email=mail-b';
    harness.window.location.search = '?email=mail-b';
    harness.run('handleHistoryNavigation()');
    await new Promise((resolve) => setTimeout(resolve, 0));
    assert.equal(harness.run('state.currentEmail.id'), 'mail-b');

    resolvePreview();
    await new Promise((resolve) => setTimeout(resolve, 0));
    assert.deepEqual(detailRequests, ['mail-b']);
    assert.equal(harness.run('state.currentEmail.id'), 'mail-b');
});

test('failed history detail load clears the previously rendered selection', async () => {
    const harness = createHarness({
        fetchImpl: async (url) => {
            const id = new URL(url).pathname.split('/').pop();
            if (id === 'mail-missing') {
                return {
                    ok: false,
                    status: 404,
                    headers: { get: () => 'application/json' },
                    async json() { return { message: 'not found' }; },
                    async text() { return '{"message":"not found"}'; }
                };
            }
            return jsonResponse({ id, subject: id, from: [], to: [], attachments: [] });
        }
    });
    await harness.run("loadEmailDetail('mail-a')");
    harness.window.location.href = 'http://owlmail.test/?email=mail-missing';
    harness.window.location.search = '?email=mail-missing';

    harness.run('handleHistoryNavigation()');
    await new Promise((resolve) => setTimeout(resolve, 0));

    assert.equal(harness.run('state.currentEmail'), null);
    assert.equal(harness.window.location.search, '?email=mail-missing');
    assert.equal(harness.alerts.length, 1);
});

test('verification submits exact assertions and safely renders extracted mail content', async () => {
    const harness = createHarness({ basePathname: '/owlmail', fetchImpl: async () => jsonResponse({ passed: false, checks: [{ name: 'text', passed: false, message: '<script>bad</script>' }], links: [], extractions: { code: ['123456'] }, links_truncated: false, extractions_truncated: false }) });
    harness.run(`state.currentEmail = { id: 'mail-1', text: 'hello', headers: {}, attachments: [], run_id: 'run-a' }; emailContentTab = 'inspect'; renderEmailDetail(); updateEmailInspectionField('subject', 'Verify'); updateEmailInspectionField('text', 'Expected'); updateEmailInspectionField('hosts', 'app.example.test, login.example.test'); updateEmailInspectionField('codes', true)`);
    await harness.run('runEmailInspection()');
    assert.equal(harness.fetchRequests[0].url, 'http://owlmail.test/owlmail/api/v1/emails/mail-1/check');
    const body = JSON.parse(harness.fetchRequests[0].options.body);
    assert.deepEqual(body.allowed_link_hosts, ['app.example.test', 'login.example.test']);
    assert.deepEqual(body.text_contains, ['Expected']);
    assert.deepEqual('1234567 123456 912345678'.match(new RegExp(body.extract[0].pattern, 'g')), ['123456']);
    assert.match(harness.run("document.getElementById('emailInspectionStatus').innerHTML"), /Checks failed/);
    assert.match(harness.run("document.getElementById('emailInspectionStatus').innerHTML"), /&lt;script&gt;bad/);
    assert.equal(harness.emailDetail.innerHTML.includes('<script>bad'), false);
    assert.match(harness.emailDetail.innerHTML, /run-a/);
});

test('late verification response cannot overwrite another selected message', async () => {
    let finish;
    const waiting = new Promise(resolve => { finish = resolve; });
    const harness = createHarness({ fetchImpl: async () => waiting });
    harness.run(`state.currentEmail = { id: 'first', text: 'first', attachments: [] }; emailContentTab = 'inspect'`);
    const pending = harness.run('runEmailInspection()');
    harness.run(`state.currentEmail = { id: 'second', text: 'second', attachments: [] }; renderEmailDetail()`);
    finish(jsonResponse({ passed: true, checks: [], links: [], extractions: {} }));
    await pending;
    assert.equal(harness.run('emailTools.id'), 'second');
    assert.equal(harness.run('emailTools.report'), null);
    assert.equal(harness.emailDetail.innerHTML.includes('Checks passed'), false);
});

test('delivery panels keep partial errors visible and require explicit retry confirmation', async () => {
    const harness = createHarness({ fetchImpl: async (url, options) => {
        if (options?.method === 'POST') return jsonResponse({ data: { job: { id: 'new-job', status: 'queued' }, statusUrl: '/api/v1/relay-jobs/new-job' } });
        if (String(url).endsWith('/relay-jobs/new-job')) return jsonResponse({ data: { id: 'new-job', status: 'succeeded' } });
        if (String(url).includes('/relay-jobs')) return jsonResponse({ jobs: [{ id: 'job-1', status: 'failed', attempts: 3, errorCategory: 'temporary' }], persistence_healthy: false });
        return { ok: false, async text() { return 'Webhook history unavailable'; } };
    } });
    harness.run(`state.currentEmail = { id: 'mail-1', text: 'hello', attachments: [] }; emailContentTab = 'deliveries'`);
    await harness.run('refreshEmailDeliveries()');
    assert.match(harness.run("document.getElementById('emailDeliveriesPanel').outerHTML"), /could not be persisted/);
    assert.match(harness.run("document.getElementById('emailDeliveriesPanel').outerHTML"), /Webhook history unavailable/);
    assert.match(harness.run("document.getElementById('emailDeliveriesPanel').outerHTML"), /job-1/);
    await harness.run(`retryEmailWorkflow('relay', 0)`);
    assert.equal(harness.confirmations.length, 1);
    assert.equal(harness.fetchRequests[2].url, 'http://owlmail.test/api/v1/relay-jobs/job-1/retry');
    assert.equal(harness.run('emailTools.notice'), '');
    await harness.run(`retryEmailWorkflow('relay', 0)`);
    assert.equal(harness.fetchRequests.length, 4);
    await harness.run('refreshEmailDeliveries()');
    assert.equal(harness.run('emailTools.notice'), '');
    assert.equal(harness.run("document.getElementById('emailDeliveriesPanel').outerHTML").includes('Retry accepted'), false);
});


test('edited verification criteria clear completed results and supersede pending responses', async () => {
    let finish;
    const harness = createHarness({ fetchImpl: async () => new Promise(resolve => { finish = resolve; }) });
    harness.run(`state.currentEmail = { id: 'mail-1', text: 'hello', attachments: [] }; emailContentTab = 'inspect'; emailToolsFor(state.currentEmail).report = { passed: true, checks: [], links: [], extractions: {} }; updateEmailInspectionField('subject', 'new criteria')`);
    assert.equal(harness.run('emailTools.report'), null);
    const pending = harness.run('runEmailInspection()');
    harness.run(`updateEmailInspectionField('subject', 'even newer criteria')`);
    finish(jsonResponse({ passed: true, checks: [], links: [], extractions: {} }));
    await pending;
    assert.equal(harness.run('emailTools.report'), null);
    assert.equal(harness.emailDetail.innerHTML.includes('Checks passed'), false);
    assert.equal(harness.run('emailTools.checking'), false);
});


test('delivery loading preserves tab focus and completion does not steal moved focus', async () => {
    let finish;
    const waiting = new Promise(resolve => { finish = resolve; });
    const harness = createHarness({ fetchImpl: async () => waiting });
    harness.run(`state.currentEmail = { id: 'mail-1', text: 'hello', attachments: [] }; emailContentTab = 'deliveries';
        const deliveryTab = { focus() { document.activeElement = this; } };
        const lookup = document.getElementById;
        document.getElementById = id => id === 'email-content-tab-deliveries' ? deliveryTab : lookup(id);
        const originalRender = renderEmailDetail;
        renderEmailDetail = () => { originalRender(); document.activeElement = null; };
        document.activeElement = deliveryTab;`);
    const pending = harness.run('refreshEmailDeliveries()');
    assert.equal(harness.run('document.activeElement === deliveryTab'), true);
    harness.run(`document.activeElement = { id: 'another-control' }`);
    finish(jsonResponse({ jobs: [], deliveries: [] }));
    await pending;
    assert.equal(harness.run('document.activeElement === deliveryTab'), false);
});

test('queue-only webhook failures do not offer or submit manual retries', async () => {
    const harness = createHarness();
    harness.run(`state.currentEmail = { id: 'mail-1', text: 'hello', attachments: [] }; emailContentTab = 'deliveries'; emailToolsFor(state.currentEmail).webhook = { deliveries: [{ id: 'queue-failure', status: 'failed', outcomes: [{ target: 'delivered', error_category: '' }, { target: '', error_category: 'queue' }] }] }; renderEmailDetail()`);
    assert.equal(harness.emailDetail.innerHTML.includes("retryEmailWorkflow('webhook'"), false);
    await harness.run("retryEmailWorkflow('webhook', 0)");
    assert.equal(harness.confirmations.length, 0);
    assert.equal(harness.fetchRequests.length, 0);
    harness.run(`emailTools.webhook.deliveries[0].outcomes.push({ target: 'failed-target', error_category: 'http_status' }); renderEmailDetail()`);
    assert.equal(harness.emailDetail.innerHTML.includes("retryEmailWorkflow('webhook'"), true);
});


test('superseded checks and inactive delivery completions preserve panel controls', async () => {
    let finish;
    const harness = createHarness({ fetchImpl: async () => new Promise(resolve => { finish = resolve; }) });
    harness.run(`state.currentEmail = { id: 'mail-1', text: 'hello', attachments: [] }; emailContentTab = 'inspect'`);
    const check = harness.run('runEmailInspection()');
    harness.run(`updateEmailInspectionField('text', 'new criteria'); document.activeElement = { id: 'verification-input' }; const original = renderEmailDetail; renderEmailDetail = () => { original(); document.activeElement = null; };`);
    finish(jsonResponse({ passed: true, checks: [], links: [], extractions: {} }));
    await check;
    assert.equal(harness.run('document.activeElement.id'), 'verification-input');
});


test('delivery completion does not rerender an inactive Verify panel', async () => {
    let finish;
    const waiting = new Promise(resolve => { finish = resolve; });
    const harness = createHarness({ fetchImpl: async () => waiting });
    harness.run(`state.currentEmail = { id: 'mail-1', text: 'hello', attachments: [] }; emailContentTab = 'deliveries'`);
    const refresh = harness.run('refreshEmailDeliveries()');
    harness.run(`emailContentTab = 'inspect'; document.activeElement = { id: 'verification-input' }; const original = renderEmailDetail; renderEmailDetail = () => { original(); document.activeElement = null; };`);
    finish(jsonResponse({ deliveries: [], jobs: [] }));
    await refresh;
    assert.equal(harness.run('document.activeElement.id'), 'verification-input');
    assert.equal(harness.run('emailTools.loaded'), true);
});


test('verification updates status in place and retains the keyboard run button', async () => {
    let finish;
    const waiting = new Promise(resolve => { finish = resolve; });
    const harness = createHarness({ fetchImpl: async () => waiting });
    harness.run(`state.currentEmail = { id: 'mail-1', text: 'hello', attachments: [] }; emailContentTab = 'inspect'; document.getElementById('emailInspectionRun').focus(); const original = renderEmailDetail; renderEmailDetail = () => { original(); document.activeElement = null; };`);
    const pending = harness.run('runEmailInspection()');
    assert.equal(harness.run("document.activeElement === document.getElementById('emailInspectionRun')"), true);
    finish(jsonResponse({ passed: true, checks: [], links: [], extractions: {} }));
    await pending;
    assert.equal(harness.run("document.activeElement === document.getElementById('emailInspectionRun')"), true);
    assert.match(harness.run("document.getElementById('emailInspectionStatus').innerHTML"), /Checks passed/);
});

test('retry completion retains an inactive panel and its focused input', async () => {
    let finish;
    const waiting = new Promise(resolve => { finish = resolve; });
    const harness = createHarness({ fetchImpl: async () => waiting });
    harness.run(`state.currentEmail = { id: 'mail-1', text: 'hello', attachments: [] }; emailContentTab = 'deliveries'; emailToolsFor(state.currentEmail).webhook = { deliveries: [{ id: 'job-1', status: 'failed', outcomes: [{ target: 'target', error_category: 'transport' }] }] };`);
    const pending = harness.run("retryEmailWorkflow('webhook', 0)");
    harness.run(`emailContentTab = 'inspect'; document.activeElement = { id: 'verification-input' }; const original = renderEmailDetail; renderEmailDetail = () => { original(); document.activeElement = null; };`);
    finish(jsonResponse({ success: true }));
    await pending;
    assert.equal(harness.run('document.activeElement.id'), 'verification-input');
    assert.equal(harness.run('emailTools.webhook.deliveries[0].status'), 'queued');
});


test('delivery refresh preserves its keyboard action across loading and completion', async () => {
    let finish;
    const waiting = new Promise(resolve => { finish = resolve; });
    const harness = createHarness({ fetchImpl: async () => waiting });
    harness.run(`state.currentEmail = { id: 'mail-1', text: 'hello', attachments: [] }; emailContentTab = 'deliveries'; document.getElementById('emailDeliveriesRefresh').focus(); const original = renderEmailDetail; renderEmailDetail = () => { original(); document.activeElement = null; };`);
    const pending = harness.run('refreshEmailDeliveries()');
    assert.equal(harness.run("document.activeElement.id"), 'emailDeliveriesRefresh');
    finish(jsonResponse({ deliveries: [], jobs: [] }));
    await pending;
    assert.equal(harness.run("document.activeElement.id"), 'emailDeliveriesRefresh');
});


test('HTML-only mail clearly limits plain-text controls and retains subject/link checks', async () => {
    const harness = createHarness({ fetchImpl: async () => jsonResponse({ passed: true, checks: [], links: [], extractions: {} }) });
    harness.run(`state.currentEmail = { id: 'html-only', text: '', html: '<p>Code 123456</p>', attachments: [] }; emailContentTab = 'inspect'; updateEmailInspectionField('subject', 'Verify'); updateEmailInspectionField('text', 'Code'); updateEmailInspectionField('codes', true); renderEmailDetail()`);
    assert.match(harness.emailDetail.innerHTML, /no plain-text part/);
    assert.match(harness.emailDetail.innerHTML, /input disabled maxlength/);
    assert.match(harness.emailDetail.innerHTML, /type="checkbox" disabled/);
    await harness.run('runEmailInspection()');
    const request = JSON.parse(harness.fetchRequests[0].options.body);
    assert.deepEqual(request.text_contains, []);
    assert.equal(request.extract, undefined);
    assert.deepEqual(request.subject_contains, ['Verify']);
});


test('verification enforces UTF-8 byte limits before submitting non-ASCII assertions', async () => {
    const harness = createHarness({ fetchImpl: async () => jsonResponse({ passed: true, checks: [], links: [], extractions: {} }) });
    harness.run(`state.currentEmail = { id: 'mail-1', text: 'hello', attachments: [] }; emailContentTab = 'inspect'; updateEmailInspectionField('subject', '汉'.repeat(512));`);
    await harness.run('runEmailInspection()');
    assert.equal(harness.fetchRequests.length, 0);
    assert.match(harness.run("document.getElementById('emailInspectionStatus').innerHTML"), /1024 UTF-8 bytes/);
    harness.run(`updateEmailInspectionField('subject', ''); updateEmailInspectionField('text', '汉'.repeat(512));`);
    await harness.run('runEmailInspection()');
    assert.equal(harness.fetchRequests.length, 0);
    harness.run(`updateEmailInspectionField('text', '汉'.repeat(341) + 'x');`);
    await harness.run('runEmailInspection()');
    assert.equal(harness.fetchRequests.length, 1);
});


test('relay availability completion preserves Verify input and composition state', async () => {
    const harness = createHarness();
    harness.run(`state.currentEmail = { id: 'mail-1', text: 'hello' }; emailContentTab = 'inspect'; API.getOutgoingConfig = () => new Promise(resolve => { globalThis.finishAvailability = resolve; }); const original = renderEmailDetail; renderEmailDetail = () => { original(); document.activeElement = null; }; document.activeElement = { id: 'verification-input', selectionStart: 3 };`);
    const pending = harness.run('loadRelayAvailability()');
    harness.run('finishAvailability({ enabled: true })');
    await pending;
    assert.equal(harness.run('document.activeElement.selectionStart'), 3);
    assert.match(harness.run("document.getElementById('emailRelayActions').innerHTML"), /relayCurrentEmail/);
});

test('delivery refresh completion preserves a focused header action', async () => {
    let finish;
    const waiting = new Promise(resolve => { finish = resolve; });
    const harness = createHarness({ fetchImpl: async () => waiting });
    harness.run(`state.currentEmail = { id: 'mail-1', text: 'hello' }; emailContentTab = 'deliveries'; const original = renderEmailDetail; renderEmailDetail = () => { original(); document.activeElement = null; };`);
    const pending = harness.run('refreshEmailDeliveries()');
    harness.run("document.activeElement = { id: 'download-action' }");
    finish(jsonResponse({ jobs: [], deliveries: [] }));
    await pending;
    assert.equal(harness.run('document.activeElement.id'), 'download-action');
    assert.match(harness.run("document.getElementById('emailDeliveriesPanel').outerHTML"), /emailDeliveriesRefresh/);
});


test('relay retry tracks the new job and blocks mutation until completion', async () => {
    let finish;
    const waiting = new Promise(resolve => { finish = resolve; });
    const harness = createHarness({ fetchImpl: async (url, options) => options?.method === 'POST' ? jsonResponse({ data: { job: { id: 'new-job', status: 'queued' }, statusUrl: '/api/v1/relay-jobs/new-job' } }) : waiting });
    harness.run(`state.currentEmail = { id: 'mail-1', text: 'hello' }; emailContentTab = 'deliveries'; emailToolsFor(state.currentEmail).relay = { jobs: [{ id: 'old-job', status: 'failed' }] };`);
    const pending = harness.run("retryEmailWorkflow('relay', 0)");
    await new Promise(resolve => setTimeout(resolve, 0));
    assert.equal(harness.deleteAllBtn.disabled, true);
    assert.equal(harness.run("document.getElementById('emailDeleteAction').disabled"), true);
    assert.equal(harness.run('emailTools.relay.jobs[0].id'), 'new-job');
    assert.equal(harness.run('emailTools.relay.jobs[1].status'), 'failed');
    finish(jsonResponse({ data: { id: 'new-job', status: 'succeeded' } }));
    await pending;
    assert.equal(harness.deleteAllBtn.disabled, false);
    assert.equal(harness.run('emailTools.relay.jobs[0].status'), 'succeeded');
    assert.equal(harness.run('emailTools.notice'), '');
});


test('relay retry admission blocks mutation before POST resolves and releases on rejection', async () => {
    let finish;
    const waiting = new Promise(resolve => { finish = resolve; });
    const harness = createHarness({ fetchImpl: async () => waiting });
    harness.run(`state.currentEmail = { id: 'mail-1', text: 'hello' }; emailContentTab = 'deliveries'; emailToolsFor(state.currentEmail).relay = { jobs: [{ id: 'old-job', status: 'failed' }] };`);
    const pending = harness.run("retryEmailWorkflow('relay', 0)");
    assert.equal(harness.deleteAllBtn.disabled, true);
    assert.equal(harness.run("document.getElementById('emailDeleteAction').disabled"), true);
    await harness.run("deleteEmail('mail-1'); deleteAllEmails(); relayCurrentEmail('mail-1'); retryEmailWorkflow('relay', 0)");
    assert.equal(harness.fetchRequests.length, 1);
    finish({ ok: false, status: 503, async text() { return 'Unavailable'; } });
    await pending;
    assert.equal(harness.deleteAllBtn.disabled, false);
    assert.equal(harness.run('relayPending.size'), 0);
    assert.equal(harness.run('emailTools.relay.jobs[0].status'), 'failed');
});


test('delivery retry focus follows job identity when refresh reorders records', async () => {
    let finish;
    const waiting = new Promise(resolve => { finish = resolve; });
    const harness = createHarness({ fetchImpl: async () => waiting });
    harness.run(`state.currentEmail = { id: 'mail-1', text: 'hello' }; emailContentTab = 'deliveries'; emailToolsFor(state.currentEmail).relay = { jobs: [{ id: 'first', status: 'failed' }, { id: 'selected', status: 'failed' }] };
        const lookup = document.getElementById;
        document.getElementById = id => {
            if (!id.startsWith('email-workflow-retry-relay-')) return lookup(id);
            const job = emailTools.relay.jobs.find(job => emailWorkflowActionID('relay', job.id) === id);
            return job ? { id, jobID: job.id, focus() { document.activeElement = this; } } : null;
        };`);
    const pending = harness.run('refreshEmailDeliveries()');
    harness.run("document.getElementById(emailWorkflowActionID('relay', 'selected')).focus()");
    finish(jsonResponse({ jobs: [{ id: 'selected', status: 'failed' }, { id: 'replacement', status: 'failed' }], deliveries: [] }));
    await pending;
    assert.equal(harness.run('document.activeElement.jobID'), 'selected');
    assert.match(harness.run("document.getElementById('emailDeliveriesPanel').outerHTML"), /id="email-workflow-retry-relay-selected"/);
});

 test('theme follows the system without saving an implicit preference', () => {
    const harness = createHarness();
    const media = { matches: true, addEventListener(name, handler) { this.change = handler; } };
    harness.window.matchMedia = () => media;
    harness.run('initTheme()');
    assert.equal(harness.run("document.body.classList.contains('dark-theme')"), true);
    assert.equal(harness.storage.has('theme'), false);
    media.matches = false;
    media.change();
    assert.equal(harness.run("document.body.classList.contains('light-theme')"), true);
    harness.run('toggleTheme()');
    assert.equal(harness.storage.get('theme'), 'dark');
    media.change();
    assert.equal(harness.run("document.body.classList.contains('dark-theme')"), true);
});

test('theme, language and viewport remain usable with blocked browser storage', () => {
    const harness = createHarness();
    harness.run("localStorage.getItem = localStorage.setItem = () => { throw new Error('blocked'); }");
    harness.run("initTheme(); toggleTheme(); setEmailViewport('375'); detectLanguage()");
    assert.equal(harness.run('currentTheme'), 'dark');
    assert.equal(harness.emailViewportFrame.style.width, '375px');
    harness.run('toggleTheme()');
    assert.equal(harness.run('currentTheme'), 'light');
});

test('preview width persists only valid preset choices', () => {
    const harness = createHarness();
    harness.run("setEmailViewport('425')");
    assert.equal(harness.storage.get('owlmail.previewWidth'), '425');
    harness.run("setEmailViewport('bad')");
    assert.equal(harness.storage.get('owlmail.previewWidth'), '425');
});

test('search shortcut focuses search and ignores composition and interactive targets', () => {
    const harness = createHarness();
    harness.run("const search = document.createElement('input'); document.getElementById = id => id === 'searchInput' ? search : null;");
    harness.run("handleMailboxKeydown({key:'/', target:{tagName:'BODY'}, preventDefault(){}})");
    assert.equal(harness.run('search.focused'), true);
    harness.run('search.focused = false');
    harness.run("handleMailboxKeydown({key:'/', isComposing:true, preventDefault(){}})");
    harness.run("handleMailboxKeydown({key:'/', target:{closest(){return {}; }}, preventDefault(){}})");
    assert.equal(harness.run('search.focused'), false);
});

test('Delete shortcut retains confirmation and ignores held keys', async () => {
    const harness = createHarness();
    harness.run("state.currentEmail = {id:'mail-1'}; confirm = () => false;");
    await harness.run("handleMailboxKeydown({key:'Delete', preventDefault(){}})");
    assert.equal(harness.fetchRequests.length, 0);
    assert.equal(harness.run('state.currentEmail.id'), 'mail-1');
    harness.run("confirm = () => { throw new Error('repeat must not confirm'); }");
    await harness.run("handleMailboxKeydown({key:'Delete', repeat:true, preventDefault(){}})");
});

test('HTML previews keep explicit light color rendering inside the sandbox', () => {
    const harness = createHarness();
    const html = harness.run("renderHTML('<p>Message colors</p>', 'mail-1', [])");
    assert.match(html, /color-scheme/);
    assert.match(html, /sandbox=""/);
    assert.match(styleSource, /height: 70vh/);
});


test('role buttons suppress global shortcuts for a different selected message', async () => {
    const harness = createHarness();
    harness.run("state.currentEmail = {id:'selected'}; confirm = () => { throw new Error('must not delete selected message'); }");
    await harness.run(`handleMailboxKeydown({key:'Delete', target: {closest(selector) {
        return selector.includes('[role=button]') ? {role:'button'} : null;
    }}, preventDefault() {throw new Error('must not intercept role button');}})`);
    assert.equal(harness.run('state.currentEmail.id'), 'selected');
    assert.equal(harness.fetchRequests.length, 0);
});


test('inbox preserves an explicit session theme when storage is blocked', () => {
    const harness = createHarness();
    const media = {matches:true, addEventListener(name, handler) {this.change = handler;}};
    harness.window.matchMedia = () => media;
    harness.run("localStorage.getItem = localStorage.setItem = () => {throw new Error('blocked');}; initTheme(); toggleTheme();");
    assert.equal(harness.run('currentTheme'), 'light');
    media.change();
    assert.equal(harness.run('currentTheme'), 'light');
});


test('latest list query wins even when the old response arrives last', async () => {
 const pending=[];
 const h=createHarness({fetchImpl:()=>new Promise(resolve=>pending.push(resolve))});
 const old=h.run('loadEmails()');
 h.run("state.searchQuery='invoice'");
 const fresh=h.run('loadEmails()');
 pending[1](jsonResponse({previews:[{id:'new'}],total:1})); await fresh;
 pending[0](jsonResponse({previews:[{id:'old'}],total:10})); await old;
 assert.equal(h.run('state.emails[0].id'),'new');
 assert.equal(h.run('state.total'),1);
});
test('realtime bursts refetch the active filtered page without inserting duplicate messages', async () => {
 const h=createHarness({fetchImpl:async()=>jsonResponse({previews:[{id:'filtered'}],total:101})});
 h.run("state.searchQuery='invoice'; state.currentPage=2;");
 h.run("handleWebSocketMessage({type:'new',email:{id:'unrelated'}}); handleWebSocketMessage({type:'new',email:{id:'unrelated'}})");
 await new Promise(resolve=>setTimeout(resolve,0));
 assert.equal(h.fetchRequests.length,1);
 assert.equal(h.run('state.emails.length'),1);
 assert.equal(h.run('state.emails[0].id'),'filtered');
 const url=new URL(h.fetchRequests[0].url);
 assert.equal(url.searchParams.get('q'),'invoice');
 assert.equal(url.searchParams.get('offset'),'100');
});
test('removing the last result on a page falls back to the last valid page', async () => {
 const h=createHarness({fetchImpl:async url=>jsonResponse({previews:new URL(url).searchParams.get('offset')==='0'?[{id:'remaining'}]:[],total:1})});
 h.run('state.currentPage=1');
 await h.run('loadEmails()');
 assert.equal(h.run('state.currentPage'),0);
 assert.equal(h.run('state.emails[0].id'),'remaining');
 assert.equal(h.fetchRequests.length,2);
});
test('websocket reconnect invalidates and refreshes missed changes', async () => {
 const h=createHarness({fetchImpl:async()=>jsonResponse({previews:[],total:0})});
 h.run('connectWebSocket(); state.ws.onopen(); connectWebSocket(); state.ws.onopen()');
 await new Promise(resolve=>setTimeout(resolve,0));
 assert.equal(h.fetchRequests.length,1);
});

test('overlapping local loads keep aria-busy until both finish', () => {
 const h=createHarness();
 h.run("showLoading('emailList'); showLoading('emailList'); hideLoading('emailList')");
 assert.equal(h.emailList.attributes.get('aria-busy'),'true');
 h.run("hideLoading('emailList')");
 assert.equal(h.emailList.attributes.get('aria-busy'),'false');
 assert.equal(h.run('loadingCounts.size'),0);
});
test('stopping a stalled relay wait keeps the accepted job and blocks resubmission', async () => {
 const h=createHarness({fetchImpl:()=>new Promise(()=>{})});
 h.run("relayEnabled=true; relayPending.add('mail-1'); relayTracking.set('mail-1',{statusUrl:'/api/v1/relay-jobs/job-1'});");
 const waiting=h.run("resumeRelayWaiting('mail-1')");
 await new Promise(resolve=>setTimeout(resolve,0));
 h.run("stopRelayWaiting('mail-1')");
 await waiting;
 assert.equal(h.run("relayTracking.get('mail-1').paused"),true);
 assert.equal(h.run("relayPending.has('mail-1')"),true);
 assert.match(h.alerts.at(-1),/queued delivery continues/);
 assert.equal(h.fetchRequests.filter(r=>r.options?.method==='POST').length,0);
});
test('bounded relay waiting times out without cancelling server delivery and can resume', async () => {
 let done=false;
 const h=createHarness({fetchImpl:()=>done?Promise.resolve(jsonResponse({data:{status:'succeeded'}})):new Promise(()=>{})});
 h.run("relayPending.add('mail-1'); relayTracking.set('mail-1',{statusUrl:'/api/v1/relay-jobs/job-1'});");
 await h.run("resumeRelayWaiting('mail-1',0)");
 assert.equal(h.run("relayTracking.get('mail-1').paused"),true);
 assert.match(h.alerts.at(-1),/timed out/);
 done=true;
 await h.run("resumeRelayWaiting('mail-1')");
 assert.equal(h.run("relayPending.has('mail-1')"),false);
 assert.equal(h.run("relayTracking.has('mail-1')"),false);
});

test('advanced filters are encoded on preview and filtered export queries', async () => {
 const h=createHarness({fetchImpl:async()=>jsonResponse({previews:[],total:0})});
 h.run("state.searchQuery='invoice & receipt'; mailboxFilters={from:'Sender <s@example.test>',to:'r@example.test',read:'false',dateFrom:'2026-10-01',dateTo:'2026-10-12',sortBy:'size',sortOrder:'asc'}");
 await h.run('loadEmails()');
 const url=new URL(h.fetchRequests[0].url);
 assert.equal(url.searchParams.get('from'),'Sender <s@example.test>');
 assert.equal(url.searchParams.get('read'),'false');
 assert.equal(url.searchParams.get('sortBy'),'size');
 const exported=new URL(h.run('mailboxExportURL(false)'));
 assert.equal(exported.pathname,'/api/v1/emails/export');
 assert.equal(exported.searchParams.get('q'),'invoice & receipt');
 assert.equal(exported.searchParams.get('dateTo'),'2026-10-12');
 assert.equal(exported.searchParams.has('sortBy'),false);
});
test('selection is limited to the visible page and export uses explicit IDs', async () => {
 const h=createHarness({fetchImpl:async()=>jsonResponse({previews:[{id:'next-page'}],total:1})});
 h.run("state.emails=[{id:'mail-1'},{id:'mail-2'}]; selectMailboxPage(true); toggleMailSelection('unknown',true)");
 assert.equal(h.run('selectedMailIDs.size'),2);
 assert.equal(new URL(h.run('mailboxExportURL(true)')).searchParams.get('ids'),'mail-1,mail-2');
 await h.run('loadEmails()');
 assert.equal(h.run('selectedMailIDs.size'),0);
 assert.equal(h.run('mailboxExportURL(true)'),'');
});
test('batch deletion keeps failures selected and clears successfully deleted details', async () => {
 const h=createHarness({fetchImpl:async(url,options)=>options?.method==='DELETE'
  ?jsonResponse({success:1,failed:1,failedIDs:['mail-2']})
  :jsonResponse({previews:[{id:'mail-2'}],total:1})});
 h.run("state.emails=[{id:'mail-1'},{id:'mail-2'}]; state.currentEmail={id:'mail-1'}; selectMailboxPage(true)");
 await h.run("runMailboxBatch('delete')");
 assert.deepEqual(JSON.parse(h.fetchRequests[0].options.body).ids,['mail-1','mail-2']);
 assert.equal(h.run('state.currentEmail'),null);
 assert.equal(h.run("selectedMailIDs.has('mail-2')"),true);
 assert.equal(h.run('mailboxBatchBusy'),false);
 assert.match(h.confirmations[0],/2 selected/);
});
test('batch deletion is blocked for selected pending relay sources', async () => {
 const h=createHarness();
 h.run("state.emails=[{id:'mail-1'}]; selectMailboxPage(true); relayPending.add('mail-1')");
 await h.run("runMailboxBatch('delete')");
 assert.equal(h.fetchRequests.length,0);
 assert.equal(h.confirmations.length,0);
});
test('batch read uses the existing PATCH contract', async () => {
 const h=createHarness({fetchImpl:async(url,options)=>options?.method==='PATCH'
  ?jsonResponse({success:1,failed:0,failedIDs:[]})
  :jsonResponse({previews:[{id:'mail-1',read:true}],total:1})});
 h.run("state.emails=[{id:'mail-1'}]; selectMailboxPage(true)");
 await h.run("runMailboxBatch('read')");
 assert.equal(new URL(h.fetchRequests[0].url).pathname,'/api/v1/emails/batch/read');
 assert.equal(h.fetchRequests[0].options.method,'PATCH');
 assert.equal(h.run('state.emails[0].read'),true);
});

test('arrival-order filter sends the explicit store sort to the API', async () => {
 const h=createHarness({fetchImpl:async()=>jsonResponse({previews:[],total:0})});
 h.run("const sortControl={value:'store'}; const originalGetElement=document.getElementById; document.getElementById=id=>id==='mail-filter-sortBy'?sortControl:originalGetElement(id)");
 await h.run('applyMailboxFilters()');
 assert.equal(new URL(h.fetchRequests.at(-1).url).searchParams.get('sortBy'),'store');
 assert.match(appSource, /\['store','sortStore'\]/);
});

test('resumed retry completion clears paused notice and refreshes deliveries', async () => {
 let completed=false;
 const h=createHarness({fetchImpl:async(url,options)=>options?.method==='POST'?jsonResponse({data:{job:{id:'new-job',status:'queued'},statusUrl:'/api/v1/relay-jobs/new-job'}}):completed?jsonResponse({data:{id:'new-job',status:'succeeded'}}):new Promise(()=>{})});
 h.run("state.currentEmail={id:'mail-1',text:'hello'}; emailContentTab='deliveries'; emailToolsFor(state.currentEmail).relay={jobs:[{id:'old-job',status:'failed'}]}");
 const retry=h.run("retryEmailWorkflow('relay',0)");
 await new Promise(resolve=>setTimeout(resolve,0));
 h.run("stopRelayWaiting('mail-1')"); await retry;
 assert.equal(h.run('emailTools.notice'),h.run("t('relayPaused')"));
 completed=true;
 h.run("emailToolsFor({id:'other-mail'}); const replacement=emailToolsFor(state.currentEmail); replacement.loaded=true; replacement.notice=t('relayPaused'); replacement.relay={jobs:[{id:'new-job',status:'queued'}]}");
 h.run('let resumedPanelRenders=0; const renderBeforeResume=renderEmailDetailKeepingTabFocus; renderEmailDetailKeepingTabFocus=()=>{resumedPanelRenders++; renderBeforeResume();}');
 await h.run("resumeRelayWaiting('mail-1')");
 assert.equal(h.run('emailTools.notice'),'');
 assert.equal(h.run('emailTools.relay.jobs[0].status'),'succeeded');
 assert.equal(h.run('resumedPanelRenders'),1);
});

test('first socket open preserves the successful initial mailbox request', async () => {
 let complete;
 let calls=0;
 const h=createHarness({fetchImpl:()=>++calls===1?new Promise(resolve=>{complete=resolve;}):Promise.reject(new Error('sync temporarily unavailable'))});
 const initial=h.run('loadEmails()');
 h.run('connectWebSocket(); state.ws.onopen()');
 complete(jsonResponse({previews:[{id:'initial'}],total:1}));
 await initial;
 await new Promise(resolve=>setTimeout(resolve,0));
 assert.equal(h.run('state.emails[0].id'),'initial');
 assert.equal(h.fetchRequests.length,2);
 assert.equal(h.alerts.length,0);
});

test('first socket open after the initial list settles synchronizes missed changes', async () => {
 let calls=0;
 const h=createHarness({fetchImpl:async()=>jsonResponse({previews:[{id:++calls===1?'snapshot':'fresh'}],total:1})});
 await h.run('loadEmails()');
 h.run('connectWebSocket(); state.ws.onopen()');
 await new Promise(resolve=>setTimeout(resolve,0));
 assert.equal(h.run('state.emails[0].id'),'fresh');
 assert.equal(h.fetchRequests.length,2);
});

test('mail selection labels escape attacker-controlled subject quotes', () => {
 const h=createHarness();
 h.run(`state.emails=[{id:'mail-1',subject:'" onclick="alert(1)',from:[],to:[]}]; renderEmailList()`);
 assert.match(h.emailList.innerHTML,/aria-label="Select message: &quot; onclick=&quot;alert\(1\)"/);
});
test('relay mutation changes refresh selected-message batch controls', () => {
 const h=createHarness();
 h.run("let batchControlRefreshes=0; updateMailboxSelectionControls=()=>batchControlRefreshes++; relayPending.add('mail-1'); syncRelayMutationControls(); relayPending.delete('mail-1'); syncRelayMutationControls()");
 assert.equal(h.run('batchControlRefreshes'),2);
});

test('detail mutations stay inert until all overlapping detail requests finish', () => {
 const h=createHarness();
 h.run("showLoading('emailDetail'); showLoading('emailDetail'); hideLoading('emailDetail')");
 assert.equal(h.emailDetail.inert,true);
 assert.notEqual(h.emailList.inert,true);
 h.run("hideLoading('emailDetail')");
 assert.equal(h.emailDetail.inert,false);
});

test('deleting the whole mailbox clears selected-message actions', async () => {
 const h=createHarness({fetchImpl:async()=>jsonResponse({})});
 h.run("state.emails=[{id:'mail-1'}]; selectMailboxPage(true)");
 await h.run('deleteAllEmails()');
 assert.equal(h.run('selectedMailIDs.size'),0);
 assert.equal(h.run('mailboxExportURL(true)'),'');
 assert.equal(h.run("t('sortDesc')"),'Descending');
 assert.equal(h.run("t('sortAsc')"),'Ascending');
});

test('a newer foreground query cancels a delayed background refresh', async () => {
 let complete;
 const h=createHarness({fetchImpl:()=>new Promise(resolve=>{complete=resolve;})});
 h.run("scheduleMailboxRefresh(); state.searchQuery='new query'");
 const foreground=h.run('loadEmails()');
 await new Promise(resolve=>setTimeout(resolve,0));
 assert.equal(h.fetchRequests.length,1);
 complete(jsonResponse({previews:[{id:'new-query'}],total:1})); await foreground;
 assert.equal(h.run('state.emails[0].id'),'new-query');
});

test('manual relay completion updates an already loaded delivery panel', async () => {
 const h=createHarness({fetchImpl:async()=>jsonResponse({data:{id:'job-1',status:'succeeded'}})});
 h.run("state.currentEmail={id:'mail-1',text:'hello'}; emailContentTab='deliveries'; const tools=emailToolsFor(state.currentEmail); tools.loaded=true; tools.relay={jobs:[{id:'job-1',status:'queued'}]}; relayPending.add('mail-1'); relayTracking.set('mail-1',{statusUrl:'/api/v1/relay-jobs/job-1'}); let deliveryRenders=0; renderEmailDetailKeepingTabFocus=()=>deliveryRenders++");
 await h.run("resumeRelayWaiting('mail-1')");
 assert.equal(h.run('emailTools.relay.jobs[0].status'),'succeeded');
 assert.equal(h.run('deliveryRenders'),1);
});

test('restored filter values escape quoted display names', () => {
 const h=createHarness();
 assert.match(h.run(`escapeHtmlAttribute('John "Boss"')`), /John &quot;Boss&quot;/);
 assert.match(appSource,/escapeHtmlAttribute\(mailboxFilters\[field\] \|\| ''\)/);
});

test('failed fallback loading clears rows from a deleted last page', async () => {
 let calls=0;
 const h=createHarness({fetchImpl:async()=>++calls===1?jsonResponse({previews:[],total:1}):Promise.reject(new Error('unavailable'))});
 h.run("state.currentPage=1; state.emails=[{id:'deleted'}]");
 await h.run('loadEmails({background:true})');
 assert.equal(h.run('state.emails.length'),0);
 assert.equal(h.run('state.currentPage'),0);
 assert.equal(h.run('state.total'),1);
});

test('failed filter application immediately clears selection checkboxes', async () => {
 const h=createHarness({fetchImpl:async()=>{throw new Error('offline');}});
 h.run("state.emails=[{id:'mail-1',from:[],to:[]}]; selectMailboxPage(true)");
 assert.match(h.emailList.innerHTML,/checked/);
 await h.run('applyMailboxFilters()');
 assert.equal(h.run('selectedMailIDs.size'),0);
 assert.doesNotMatch(h.emailList.innerHTML,/ checked /);
});

test('a delayed delivery snapshot cannot overwrite a terminal relay result', async () => {
 let finishHistory;
 const h=createHarness({fetchImpl:async url=>url.endsWith('/job-1')?jsonResponse({data:{id:'job-1',status:'succeeded'}}):url.includes('/relay-jobs?')?new Promise(resolve=>{finishHistory=resolve;}):jsonResponse({deliveries:[]})});
 h.run("state.currentEmail={id:'mail-1',text:'hello'}; emailContentTab='deliveries'; emailToolsFor(state.currentEmail).relay={jobs:[{id:'job-1',status:'queued'}]}; relayTracking.set('mail-1',{statusUrl:'/api/v1/relay-jobs/job-1'}); relayPending.add('mail-1')");
 const history=h.run('refreshEmailDeliveries()');
 await h.run("resumeRelayWaiting('mail-1')");
 finishHistory(jsonResponse({jobs:[{id:'job-1',status:'queued'}]})); await history;
 assert.equal(h.run('emailTools.relay.jobs[0].status'),'succeeded');
});

test('delete all resets the pagination controls with page state', async () => {
 const h=createHarness({fetchImpl:async()=>jsonResponse({})});
 h.run('state.currentPage=3; state.total=200; let paginationUpdates=0; updatePagination=()=>paginationUpdates++');
 await h.run('deleteAllEmails()');
 assert.equal(h.run('state.currentPage'),0);
 assert.equal(h.run('paginationUpdates'),1);
});

test('successful deletion removes its row even if preview refresh fails', async () => {
 const h=createHarness({fetchImpl:async(url,options)=>options?.method==='DELETE'?jsonResponse({}):Promise.reject(new Error('offline'))});
 h.run("state.emails=[{id:'deleted'}]; state.total=1");
 await h.run("deleteEmail('deleted')");
 assert.equal(h.run('state.emails.length'),0);
 assert.equal(h.run('state.total'),0);
});
