const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { test } = require('bun:test');

const source = fs.readFileSync(path.join(__dirname, '../../web/app.js'), 'utf8');
const start = source.indexOf('function resolveCIDReferences(');
const end = source.indexOf('\nfunction previewContentSecurityPolicy', start);
const fixtures = JSON.parse(fs.readFileSync(path.join(__dirname, '../../testdata/cid/references.json'), 'utf8'));
for (const fixture of fixtures) {
    test(`CID ${fixture.name}`, async () => {
        // DOMParser is supplied by the browser. This focused VM adapter
        // handles the fixture entities; real-browser coverage verifies decoding.
        const documents = new Map();
        for (const match of fixture.html.matchAll(/(?:src|href)\s*=\s*("([^"]*)"|'([^']*)'|([^\s>]+))/gi)) {
            const value = match[2] ?? match[3] ?? match[4];
            const markup = `<a href="${value.replace(/"/g, '&#34;')}"></a>`;
            const decoded = value.replace(/&#64;|&commat;/g, '@');
            documents.set(markup, decoded);
        }
        const sandbox = {
            API_BASE: '/mail/api/v1',
            DOMParser: class {
                parseFromString(markup) {
                    assert.ok(documents.has(markup), `unexpected controlled document: ${markup}`);
                    return { querySelector() { return { getAttribute() { return documents.get(markup); } }; } };
                }
            }
        };
        vm.createContext(sandbox);
        vm.runInContext(source.slice(start, end), sandbox);
        const result = sandbox.resolveCIDReferences(fixture.html, 'mail id', fixture.attachments);
        const targets = [];
        await new HTMLRewriter().on('*', { element(node) {
            for (const key of ['src', 'href']) {
                const value = node.getAttribute(key);
                const prefix = '/mail/api/v1/emails/mail%20id/attachments/';
                if (value && value.startsWith(prefix)) targets.push(decodeURIComponent(value.slice(prefix.length)));
            }
        } }).transform(new Response(result)).text();
        assert.deepEqual(targets, fixture.targets || [], result);
        if (!fixture.rewritten) assert.equal(result, fixture.html);
        if (fixture.preserved) assert.ok(result.includes(fixture.preserved), result);
    });
}
