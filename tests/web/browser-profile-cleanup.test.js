const assert = require('node:assert/strict');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const { test } = require('bun:test');

// Run each case in a fresh Node process: Node's recursive-removal implementation
// captures fs.rmdir on first use. Inject a real late file immediately before the
// second directory-removal syscall, after its children have been enumerated.
// This deterministically reproduces ENOTEMPTY without timing or a live browser.
for (const scenario of ['baseline', 'transient', 'persistent', 'permanent']) {
    test(`private browser profile cleanup: ${scenario}`, () => {
        const program = `
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const scenario = ${JSON.stringify(scenario)};
const { removeBrowserProfile } = require(${JSON.stringify(path.resolve(__dirname, '../browser/profile-cleanup.js'))});
const profile = fs.mkdtempSync(path.join(os.tmpdir(), 'owlmail-cleanup-check-'));
const directory = path.join(profile, 'Default');
fs.mkdirSync(directory);
fs.writeFileSync(path.join(directory, 'initial'), 'fixture');
const original = fs.rmdir;
let calls = 0;
fs.rmdir = function(target, ...args) {
    if (target.toString() === directory) {
        calls++;
        if (scenario === 'permanent') {
            const error = Object.assign(new Error('denied'), { code: 'EACCES' });
            return process.nextTick(() => args[args.length - 1](error));
        }
        if (calls === 2 || (scenario === 'persistent' && calls > 2)) {
            fs.writeFileSync(path.join(directory, 'late'), 'late writer');
        }
    }
    return original.call(this, target, ...args);
};
(async () => {
    const started = Date.now();
    try {
        if (scenario === 'baseline') {
            await assert.rejects(fs.promises.rm(profile, { recursive: true, force: true }), { code: 'ENOTEMPTY' });
        } else if (scenario === 'transient') {
            await removeBrowserProfile(profile);
            assert.equal(fs.existsSync(profile), false);
        } else {
            await assert.rejects(removeBrowserProfile(profile), error => {
                assert.equal(error.cause.code, scenario === 'permanent' ? 'EACCES' : 'ENOTEMPTY');
                assert.ok(error.message.includes(profile));
                return true;
            });
            assert.ok(fs.existsSync(profile));
            if (scenario === 'permanent') assert.equal(calls, 1, 'permanent errors must not retry');
        }
        assert.ok(Date.now() - started < 5000, 'cleanup exceeded finite retry budget');
        console.log(scenario, 'passed; rmdir calls=' + calls);
    } finally {
        fs.rmdir = original;
        // Do not use the injected asynchronous remover for test cleanup.
        fs.rmSync(profile, { recursive: true, force: true });
    }
})().catch(error => { console.error(error); process.exitCode = 1; });
`;
        const result = spawnSync('node', ['-e', program], { encoding: 'utf8', timeout: 8000 });
        assert.equal(result.status, 0, result.stdout + result.stderr);
    });
}
