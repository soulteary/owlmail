#!/usr/bin/env node
// Explicit X11 readiness avoids xvfb-run's PID-1/SIGUSR1 startup handshake.
// This test supervisor never modifies document.hidden or production settings.
import { spawn, spawnSync } from 'node:child_process';
import fs from 'node:fs';
import net from 'node:net';
import os from 'node:os';
import path from 'node:path';

const started = Date.now();
const log = (stage, message) => console.log(`[owlmail-browser ${Date.now() - started}ms] ${stage}: ${message}`);
const delay = milliseconds => new Promise(resolve => setTimeout(resolve, milliseconds));
const children = [];
let temporary;
let logFD;
let deadlineTimer;
let receivedSignal;
let notifySignal;
const interrupted = new Promise(resolve => { notifySignal = resolve; });
for (const signal of ['SIGTERM', 'SIGINT']) process.on(signal, () => {
    receivedSignal = signal;
    log('signal', signal);
    notifySignal({ code: signal === 'SIGINT' ? 130 : 143 });
});
function waitChild(child) {
    return new Promise(resolve => {
        child.once('error', error => { log('spawn error', error.message); resolve({ code: 127 }); });
        child.once('exit', (code, signal) => resolve({ code: code ?? (signal === 'SIGTERM' ? 143 : 1), signal }));
    });
}
async function x11Ready(socket) {
    return new Promise(resolve => {
        let settled = false;
        const client = net.createConnection(socket);
        const finish = value => { if (!settled) { settled = true; client.destroy(); resolve(value); } };
        client.setTimeout(750, () => finish(false));
        client.once('error', () => finish(false));
        client.once('connect', () => {
            // X11 little-endian setup: version 11.0, no authentication. The
            // server is private to this test and never listens on TCP.
            const setup = Buffer.alloc(12);
            setup[0] = 0x6c;
            setup.writeUInt16LE(11, 2);
            client.write(setup);
        });
        client.once('data', data => finish(data[0] === 1));
        client.once('end', () => finish(false));
    });
}
function processTable() {
    const table = new Map();
    for (const entry of fs.readdirSync('/proc')) {
        if (!/^\d+$/.test(entry)) continue;
        try {
            const stat = fs.readFileSync(`/proc/${entry}/stat`, 'utf8');
            const fields = stat.slice(stat.lastIndexOf(')') + 2).split(' ');
            table.set(Number(entry), { pid: Number(entry), parent: Number(fields[1]), state: fields[0], start: fields[19] });
        } catch { /* A process may exit while its stat is being read. */ }
    }
    return table;
}
function ownedDescendants() {
    const table = processTable();
    const owned = new Map();
    const parents = new Set([process.pid]);
    let changed = true;
    while (changed) {
        changed = false;
        for (const item of table.values()) {
            if (!parents.has(item.pid) && parents.has(item.parent)) {
                parents.add(item.pid); owned.set(item.pid, item); changed = true;
            }
        }
    }
    return owned;
}
function liveOwned(owned) {
    const current = processTable();
    return [...owned.values()].filter(item => {
        const now = current.get(item.pid);
        return now && now.start === item.start && now.state !== 'Z';
    });
}
function signalOwned(owned, signal) {
    for (const item of liveOwned(owned)) {
        try {
            const stat = fs.readFileSync(`/proc/${item.pid}/stat`, 'utf8');
            const fields = stat.slice(stat.lastIndexOf(')') + 2).split(' ');
            if (fields[19] === item.start && fields[0] !== 'Z') process.kill(item.pid, signal);
        } catch (error) { if (!['ESRCH', 'ENOENT'].includes(error.code)) log('cleanup', error.message); }
    }
}
async function cleanup() {
    clearTimeout(deadlineTimer);
    // Capture before terminating Playwright: its detached fixture otherwise
    // becomes PID 1's child. Match Linux starttime on every later signal.
    const owned = ownedDescendants();
    log('cleanup', `captured ${owned.size} owned descendants before parent termination`);
    signalOwned(owned, 'SIGTERM');
    const graceUntil = Date.now() + 3000;
    while (liveOwned(owned).length && Date.now() < graceUntil) await delay(100);
    signalOwned(owned, 'SIGKILL');
    const killUntil = Date.now() + 1000;
    while (liveOwned(owned).length && Date.now() < killUntil) await delay(50);
    const remaining = liveOwned(owned);
    if (logFD !== undefined) fs.closeSync(logFD);
    if (temporary) fs.rmSync(temporary, { recursive: true, force: true });
    log('cleanup', `owned live descendants remaining=${remaining.length}; exited zombies are left to container init`);
    if (remaining.length) throw new Error(`owned subprocess cleanup failed: ${remaining.map(item => item.pid).join(',')}`);
}
async function main() {
    if (process.platform !== 'linux') throw new Error('this runner requires Linux Xvfb');
    const seconds = Number(process.env.OWLMAIL_BROWSER_TOTAL_SECONDS || '240');
    if (!Number.isInteger(seconds) || seconds < 30 || seconds > 600) throw new Error('OWLMAIL_BROWSER_TOTAL_SECONDS must be 30..600');
    const command = process.argv.slice(2);
    if (!command.length) throw new Error('usage: run-browser-linux.sh <browser-test-command> [args...]');
    log('environment', `Node ${process.version}; uid=${process.getuid?.()}; deadline=${seconds}s`);
    const xvfbVersion = spawnSync('Xvfb', ['-help'], { encoding: 'utf8', timeout: 5000 });
    if (xvfbVersion.error) throw xvfbVersion.error;
    if (process.env.CHROME_BIN) {
        const version = spawnSync(process.env.CHROME_BIN, ['--version'], { encoding: 'utf8', timeout: 5000 });
        if (version.status !== 0) throw new Error(`Chromium version probe failed: ${version.stderr || version.error}`);
        log('chromium', version.stdout.trim());
    }
    temporary = fs.mkdtempSync(path.join(os.tmpdir(), 'owlmail-xvfb-'));
    logFD = fs.openSync(path.join(temporary, 'xvfb.log'), 'w');
    let number = 99;
    while (number < 120 && (fs.existsSync(`/tmp/.X11-unix/X${number}`) || fs.existsSync(`/tmp/.X${number}-lock`))) number += 1;
    if (number === 120) throw new Error('no unused private X display available');
    const display = `:${number}`;
    log('xvfb start', `${display}; Unix socket only`);
    const xvfb = spawn('Xvfb', [display, '-screen', '0', '1365x900x24', '-nolisten', 'tcp', '-ac', '-noreset'], {
        detached: true, stdio: ['ignore', logFD, logFD]
    });
    children.push(xvfb);
    const xvfbExit = waitChild(xvfb);
    let ready = false;
    const readyUntil = Date.now() + 15000;
    while (Date.now() < readyUntil && !receivedSignal) {
        if (xvfb.exitCode !== null || xvfb.signalCode !== null) throw new Error(`Xvfb exited: ${fs.readFileSync(path.join(temporary, 'xvfb.log'), 'utf8')}`);
        if (await x11Ready(`/tmp/.X11-unix/X${number}`)) { ready = true; break; }
        await delay(100);
    }
    if (!ready) throw new Error(`X11 setup handshake failed within 15s: ${fs.readFileSync(path.join(temporary, 'xvfb.log'), 'utf8')}`);
    log('xvfb ready', 'real X11 setup handshake accepted; no SIGUSR1 wait');
    log('test start', JSON.stringify(command));
    const test = spawn(command[0], command.slice(1), {
        detached: true, stdio: 'inherit', env: {
            ...process.env, DISPLAY: display,
            OWLMAIL_BROWSER_HEADED: '1', OWLMAIL_REQUIRE_REAL_BACKGROUND: '1',
            DEBUG: process.env.DEBUG || 'pw:webserver'
        }
    });
    children.push(test);
    const deadline = new Promise(resolve => {
        deadlineTimer = setTimeout(() => { log('deadline', `${seconds}s expired`); resolve({ code: 124 }); }, Math.max(1, seconds * 1000 - (Date.now() - started)));
    });
    const result = await Promise.race([
        waitChild(test), deadline, interrupted,
        xvfbExit.then(value => { log('xvfb exited', JSON.stringify(value)); return { code: 1 }; })
    ]);
    log('test exit', JSON.stringify(result));
    return result.code;
}
let code = 1;
try { code = await main(); } catch (error) { log('failure', error.stack || error.message); }
finally { await cleanup(); }
process.exitCode = code;
