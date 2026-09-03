import assert from "node:assert/strict";
import net from "node:net";

const smtpHost = process.env.TEST_SMTP_HOST || "127.0.0.1";
const smtpPort = Number(process.env.TEST_SMTP_PORT || "1025");
const apiBase = (process.env.TEST_MAIL_API || "http://127.0.0.1:1080").replace(/\/$/, "");
const runID = `${Date.now()}-${process.pid}`;
const recipient = `signup+${runID}@example.test`;
const subject = `OwlMail integration ${runID}`;
const token = `token-${runID}`;
const ioTimeoutMs = 5_000;

function lineReader(socket) {
  let buffered = "";
  let terminalError;
  const waiting = [];
  socket.setEncoding("utf8");
  socket.on("data", (chunk) => {
    buffered += chunk;
    while (buffered.includes("\n") && waiting.length) {
      const index = buffered.indexOf("\n");
      const line = buffered.slice(0, index + 1);
      buffered = buffered.slice(index + 1);
      waiting.shift().resolve(line);
    }
  });
  function fail(error) {
    terminalError ||= error;
    while (waiting.length) waiting.shift().reject(error);
  }
  socket.on("error", fail);
  socket.on("end", () => fail(new Error("SMTP server ended the connection")));
  socket.on("close", () => fail(new Error("SMTP connection closed")));
  return () => new Promise((resolve, reject) => {
    if (terminalError) {
      reject(terminalError);
    } else if (buffered.includes("\n")) {
      const index = buffered.indexOf("\n");
      const line = buffered.slice(0, index + 1);
      buffered = buffered.slice(index + 1);
      resolve(line);
    } else {
      waiting.push({ resolve, reject });
    }
  });
}

async function requestJSON(path, options = {}) {
  const controller = new AbortController();
  const timer = setTimeout(
    () => controller.abort(new Error(`HTTP request exceeded ${ioTimeoutMs} ms`)),
    ioTimeoutMs,
  );
  try {
    const response = await fetch(`${apiBase}${path}`, { ...options, signal: controller.signal });
    const body = response.ok ? await response.json() : null;
    return { response, body };
  } finally {
    clearTimeout(timer);
  }
}

async function requestStatus(path, options = {}) {
  const controller = new AbortController();
  const timer = setTimeout(
    () => controller.abort(new Error(`HTTP request exceeded ${ioTimeoutMs} ms`)),
    ioTimeoutMs,
  );
  try {
    const response = await fetch(`${apiBase}${path}`, { ...options, signal: controller.signal });
    await response.arrayBuffer();
    return response;
  } finally {
    clearTimeout(timer);
  }
}

async function sendMessage() {
  const socket = net.createConnection({ host: smtpHost, port: smtpPort });
  const abortSMTP = () => {
    socket.destroy(new Error(`SMTP operation exceeded ${ioTimeoutMs} ms`));
  };
  socket.setTimeout(ioTimeoutMs, abortSMTP);
  const deadline = setTimeout(abortSMTP, ioTimeoutMs);
  const readLine = lineReader(socket);
  try {
    await new Promise((resolve, reject) => {
      socket.once("connect", resolve);
      socket.once("error", reject);
    });

    async function reply(expected) {
      let line;
      do {
        line = await readLine();
      } while (/^\d{3}-/.test(line));
      assert.equal(Number(line.slice(0, 3)), expected, line.trim());
    }
    async function command(value, expected) {
      socket.write(`${value}\r\n`);
      await reply(expected);
    }

    await reply(220);
    await command("EHLO integration.test", 250);
    await command("MAIL FROM:<sender@example.test>", 250);
    await command(`RCPT TO:<${recipient}>`, 250);
    await command("DATA", 354);
    socket.write([
      "From: Sender <sender@example.test>",
      `To: ${recipient}`,
      `Subject: ${subject}`,
      "Content-Type: text/plain; charset=utf-8",
      "",
      `Verification token: ${token}`,
      ".",
      "",
    ].join("\r\n"));
    await reply(250);
    await command("QUIT", 221);
  } finally {
    clearTimeout(deadline);
    socket.destroy();
  }
}

async function waitForMessage() {
  const deadline = Date.now() + 15_000;
  while (Date.now() < deadline) {
    const query = new URLSearchParams({ to: recipient, limit: "10" });
    const { response, body: page } = await requestJSON(`/api/v1/emails?${query}`);
    assert.equal(response.ok, true, `list failed: ${response.status}`);
    const match = page.emails.find((email) => email.subject === subject);
    if (match) return match;
    await new Promise((resolve) => setTimeout(resolve, 200));
  }
  throw new Error(`timed out waiting for ${recipient}`);
}

await sendMessage();
const summary = await waitForMessage();
const messagePath = `/api/v1/emails/${encodeURIComponent(summary.id)}`;
try {
  const { response: detailResponse, body: detail } = await requestJSON(messagePath);
  assert.equal(detailResponse.ok, true, `detail failed: ${detailResponse.status}`);
  assert.equal(detail.subject, subject);
  assert.match(detail.text, new RegExp(token));
  console.log(`verified OwlMail message ${summary.id}`);
} finally {
  const deleteResponse = await requestStatus(messagePath, { method: "DELETE" });
  assert.equal(deleteResponse.ok, true, `cleanup failed: ${deleteResponse.status}`);
}
