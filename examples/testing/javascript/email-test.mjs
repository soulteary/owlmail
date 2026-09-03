import assert from "node:assert/strict";
import net from "node:net";
import {
  cleanupCapturedEmail,
  findMatchingMessages,
  normalizeAPIBase,
  requestJSON,
} from "./api.mjs";
import { withCleanup } from "./cleanup.mjs";

const smtpHost = process.env.TEST_SMTP_HOST || "127.0.0.1";
const smtpPort = Number(process.env.TEST_SMTP_PORT || "1025");
const apiBase = normalizeAPIBase(process.env.TEST_MAIL_API || "http://127.0.0.1:1080");
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

async function sendMessage(onAccepted) {
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
    onAccepted();
    await command("QUIT", 221);
  } finally {
    clearTimeout(deadline);
    socket.destroy();
  }
}

async function waitForMessage() {
  const deadline = Date.now() + 15_000;
  while (Date.now() < deadline) {
    const match = (await findMatchingMessages(apiBase, recipient, subject))[0];
    if (match) return match;
    await new Promise((resolve) => setTimeout(resolve, 200));
  }
  throw new Error(`timed out waiting for ${recipient}`);
}

let accepted = false;
let messageID = "";
await withCleanup(
  async () => {
    await sendMessage(() => {
      accepted = true;
    });
    const summary = await waitForMessage();
    messageID = summary.id;
    const messagePath = `/api/v1/emails/${encodeURIComponent(messageID)}`;
    const { response: detailResponse, body: detail } = await requestJSON(apiBase, messagePath);
    assert.equal(detailResponse.ok, true, `detail failed: ${detailResponse.status}`);
    assert.equal(detail.subject, subject);
    assert.match(detail.text, new RegExp(token));
    console.log(`verified OwlMail message ${messageID}`);
  },
  async () => {
    if (accepted) {
      await cleanupCapturedEmail(apiBase, recipient, subject, messageID);
    }
  },
);
