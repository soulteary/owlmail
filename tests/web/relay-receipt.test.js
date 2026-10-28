const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { test } = require('bun:test');
const source = fs.readFileSync(path.join(__dirname, '../../web/app.js'), 'utf8');
function extract(name) {
    const start = source.search(new RegExp(`(?:async )?function ${name}\\(`));
    assert.ok(start >= 0, name);
    const next = source.slice(start + 1).search(/\n(?:async )?function /);
    return source.slice(start, next < 0 ? undefined : start + next + 1);
}
function harness() {
    const context = {
        t: () => 'Accepted downstream:', state: { currentEmail: { id: 'mail' } },
        API: { async getEmail() { return { relayedAt: '2026-10-29T01:00:00Z', relayedTo: ['actual@example.test'] }; } },
        console, setTimeout,
        document: { createElement() { let text = ''; return {
            set textContent(value) { text = String(value); },
            get innerHTML() { return text.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;'); }
        }; } }
    };
    vm.createContext(context);
    vm.runInContext(['escapeHtml', 'escapeHtmlAttribute', 'renderRelayReceiptMetadata', 'refreshRelayReceipt'].map(extract).join('\n'), context);
    return context;
}
test('relay receipt renders actual recipients and acceptance time safely', () => {
    const context = harness();
    const html = context.renderRelayReceiptMetadata({ relayedAt: '2026-10-29T01:00:00Z', relayedTo: ['<img src=x>', 'next@example.test'] });
    assert.match(html, /&lt;img src=x&gt;, next@example.test/);
    assert.match(html, /data-testid="email-relay-receipt"/);
    assert.match(html, /datetime="2026-10-29T01:00:00.000Z"/);
    assert.doesNotMatch(html, /<img/);
});
test('missing or malformed receipt renders no false never-delivered claim', () => {
    const context = harness();
    for (const email of [{}, { relayedAt: 'invalid', relayedTo: ['a'] }, { relayedAt: '2026-10-29', relayedTo: [] }]) {
        assert.equal(context.renderRelayReceiptMetadata(email), '');
    }
});
test('successful task refreshes header state without rerendering the preview', async () => {
    const context = harness();
    await context.refreshRelayReceipt('mail');
    assert.equal(context.state.currentEmail.relayedAt, '2026-10-29T01:00:00Z');
    assert.equal(context.state.currentEmail.relayedTo[0], 'actual@example.test');
    assert.match(source, /completed.status === 'succeeded'\) void refreshRelayReceipt\(id, completed.completedAt\)/);
    context.state.currentEmail = { id: 'other' };
    await context.refreshRelayReceipt('mail');
    assert.equal(context.state.currentEmail.relayedAt, undefined);
});

test('receipt repair waits past missing and stale summaries for the completed acceptance', async () => {
    const context = harness();
    const responses = [{}, { relayedAt: '2026-10-28T01:00:00Z', relayedTo: ['old@example.test'] },
        { relayedAt: '2026-10-29T01:00:00Z', relayedTo: ['new@example.test'] }];
    let calls = 0;
    context.setTimeout = resolve => { resolve(); };
    context.API.getEmail = async () => { calls++; return responses.shift(); };
    await context.refreshRelayReceipt('mail', '2026-10-29T01:00:00Z');
    assert.equal(calls, 3);
    assert.equal(context.state.currentEmail.relayedTo[0], 'new@example.test');
});
test('receipt wait exits after navigation without fetching or changing another detail', async () => {
    const context = harness();
    let calls = 0, waits = 0;
    context.API.getEmail = async () => { calls++; return {}; };
    context.setTimeout = resolve => { waits++; context.state.currentEmail = { id: 'other' }; resolve(); };
    await context.refreshRelayReceipt('mail', '2026-10-29T01:00:00Z');
    assert.equal(calls, 1);
    assert.equal(waits, 1);
    assert.equal(context.state.currentEmail.id, 'other');
    assert.equal(context.state.currentEmail.relayedAt, undefined);
});
test('receipt wait is bounded when repair remains unavailable', async () => {
    const context = harness();
    let calls = 0;
    context.API.getEmail = async () => { calls++; return {}; };
    context.setTimeout = resolve => resolve();
    await context.refreshRelayReceipt('mail', '2026-10-29T01:00:00Z');
    assert.equal(calls, 40);
    assert.equal(context.state.currentEmail.relayedAt, undefined);
});
test('completion polling retains successful tasks until their receipt repair is durable', async () => {
    const context = harness();
    const responses = [
        { status: 'queued' },
        { status: 'succeeded', receiptPending: true },
        { status: 'succeeded', receiptPending: true },
        { status: 'succeeded', receiptPending: false, completedAt: '2026-10-29T01:00:00Z' }
    ];
    let calls = 0;
    context.API.getRelayJob = async () => ({ data: responses[calls++] });
    context.relayWaitOperation = operation => operation();
    context.RELAY_STATUS_POLL_INTERVAL_MS = 0;
    context.setTimeout = resolve => resolve();
    vm.runInContext(extract('waitForRelayJob'), context);
    const completed = await context.waitForRelayJob('/relay-jobs/job');
    assert.equal(calls, 4);
    assert.equal(completed.receiptPending, false);
    assert.equal(completed.completedAt, '2026-10-29T01:00:00Z');
});
