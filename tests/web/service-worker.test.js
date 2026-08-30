const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { test } = require('bun:test');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, '../../web/service-worker.js'), 'utf8');

function loadWorker(windows) {
  let clickHandler;
  let opened;
  const self = {
    location: { origin: 'https://owlmail.test' },
    addEventListener(name, handler) {
      if (name === 'notificationclick') clickHandler = handler;
    },
    clients: {
      async matchAll() { return windows; },
      async openWindow(url) { opened = url; return null; }
    }
  };
  vm.runInNewContext(source, { self, URL, encodeURIComponent });
  return { clickHandler, opened: () => opened };
}

async function click(worker, emailID) {
  let completion;
  worker.clickHandler({
    notification: { data: { emailID }, close() {} },
    waitUntil(promise) { completion = promise; }
  });
  await completion;
}

test('notification clicks focus only an inbox client', async () => {
  const messages = [];
  const help = { url: 'https://owlmail.test/help', async focus() { throw new Error('focused help'); } };
  const inbox = {
    url: 'https://owlmail.test/',
    async focus() {},
    postMessage(message) { messages.push(message); }
  };
  const worker = loadWorker([help, inbox]);
  await click(worker, 'mail-42');
  assert.equal(JSON.stringify(messages), JSON.stringify([{ type: 'owlmail-notification-click', emailID: 'mail-42' }]));
  assert.equal(worker.opened(), undefined);
});

test('notification clicks open a durable email deep link without an inbox client', async () => {
  const worker = loadWorker([{ url: 'https://owlmail.test/webhooks', async focus() {} }]);
  await click(worker, 'mail/42');
  assert.equal(worker.opened(), '/?email=mail%2F42');
});
