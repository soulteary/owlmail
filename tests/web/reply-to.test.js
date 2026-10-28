const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { test } = require('bun:test');
const source = fs.readFileSync(path.join(__dirname, '../../web/app.js'), 'utf8');

function extractFunction(name) {
    const start = source.indexOf(`function ${name}(`);
    const next = source.indexOf('\nfunction ', start + 1);
    assert.ok(start >= 0);
    return source.slice(start, next < 0 ? undefined : next);
}

function renderer() {
    const context = {
        t: (key) => key === 'replyTo' ? 'Reply-To:' : key,
        document: {
            createElement() {
                let text = '';
                return {
                    set textContent(value) { text = String(value); },
                    get innerHTML() { return text.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;'); }
                };
            }
        }
    };
    vm.createContext(context);
    vm.runInContext(['formatAddress', 'escapeHtml', 'renderReplyToMetadata'].map(extractFunction).join('\n'), context);
    return (email) => context.renderReplyToMetadata(email);
}

test('Reply-To detail escapes address names and retains ordered destinations', () => {
    const render = renderer();
    const html = render({ replyTo: [
        { Name: '<img src=x onerror=alert(1)>', Address: 'reply@example.test' },
        { name: 'Backup & Support', address: 'backup@example.test' }
    ] });
    assert.match(html, /data-testid="email-reply-to"/);
    assert.match(html, /&lt;img src=x onerror=alert\(1\)&gt; &lt;reply@example.test&gt;, Backup &amp; Support &lt;backup@example.test&gt;/);
    assert.doesNotMatch(html, /<img|<script/);
    assert.match(source, /\$\{renderReplyToMetadata\(email\)\}/);
});

test('Reply-To detail is omitted when destinations are absent', () => {
    const render = renderer();
    for (const email of [{}, { replyTo: [] }, { replyTo: null }, { replyTo: [null] }]) assert.equal(render(email), '');
});
