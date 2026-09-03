import assert from "node:assert/strict";
import net from "node:net";

const smtpHost = process.env.TEST_SMTP_HOST || "127.0.0.1";
const smtpPort = Number(process.env.TEST_SMTP_PORT || "1025");
const apiBase = (process.env.TEST_MAIL_API || "http://127.0.0.1:1080").replace(/\/$/, "");
const runID = `${Date.now()}-${process.pid}`;
const recipient = `signup+${runID}@example.test`;
const subject = `OwlMail integration ${runID}`;
const token = `token-${runID}`;

function lineReader(socket) {
  let buffered = "";
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
  socket.on("error", (error) => {
    while (waiting.length) waiting.shift().reject(error);
  });
  return () => new Promise((resolve, reject) => {
    if (buffered.includes("\n")) {
      const index = buffered.indexOf("\n");
      const line = buffered.slice(0, index + 1);
      buffered = buffered.slice(index + 1);
      resolve(line);
    } else {
      waiting.push({ resolve, reject });
    }
  });
}

async function sendMessage() {
  const socket = net.createConnection({ host: smtpHost, port: smtpPort });
  const readLine = lineReader(socket);
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
  socket.end();
}

async function waitForMessage() {
  const deadline = Date.now() + 15_000;
  while (Date.now() < deadline) {
    const query = new URLSearchParams({ to: recipient, limit: "10" });
    const response = await fetch(`${apiBase}/api/v1/emails?${query}`);
    assert.equal(response.ok, true, `list failed: ${response.status}`);
    const page = await response.json();
    const match = page.emails.find((email) => email.subject === subject);
    if (match) return match;
    await new Promise((resolve) => setTimeout(resolve, 200));
  }
  throw new Error(`timed out waiting for ${recipient}`);
}

await sendMessage();
const summary = await waitForMessage();
const detailResponse = await fetch(`${apiBase}/api/v1/emails/${encodeURIComponent(summary.id)}`);
assert.equal(detailResponse.ok, true, `detail failed: ${detailResponse.status}`);
const detail = await detailResponse.json();
assert.equal(detail.subject, subject);
assert.match(detail.text, new RegExp(token));

const deleteResponse = await fetch(`${apiBase}/api/v1/emails/${encodeURIComponent(summary.id)}`, {
  method: "DELETE",
});
assert.equal(deleteResponse.ok, true, `cleanup failed: ${deleteResponse.status}`);
console.log(`verified OwlMail message ${summary.id}`);
