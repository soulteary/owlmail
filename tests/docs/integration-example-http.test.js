const assert = require("node:assert/strict");
const { test } = require("bun:test");

test("JavaScript integration example removes every trailing API slash", async () => {
  const { normalizeAPIBase } = await import("../../examples/testing/javascript/api.mjs");
  assert.equal(normalizeAPIBase("http://owlmail.test///"), "http://owlmail.test");
});

test("JavaScript cleanup rejects redirects without buffering the response", async () => {
  const { cleanupCapturedEmail } = await import("../../examples/testing/javascript/api.mjs");
  let cancelled = false;
  let options;
  const fetchImpl = async (_url, requestOptions) => {
    options = requestOptions;
    return {
      body: { cancel: async () => { cancelled = true; } },
      ok: false,
      status: 302,
    };
  };

  await assert.rejects(
    cleanupCapturedEmail(
      "http://owlmail.test",
      "recipient@example.test",
      "subject",
      "captured-id",
      { fetchImpl },
    ),
    /cleanup failed: 302/,
  );
  assert.equal(options.method, "DELETE");
  assert.equal(options.redirect, "manual");
  assert.equal(cancelled, true);
});

test("JavaScript cleanup discovers an accepted message when its ID was not returned", async () => {
  const { cleanupCapturedEmail } = await import("../../examples/testing/javascript/api.mjs");
  const requests = [];
  const fetchImpl = async (url, options) => {
    requests.push({ url, options });
    if (!options.method) {
      return {
        ok: true,
        status: 200,
        json: async () => ({
          emails: [
            { id: "other-id", subject: "other" },
            { id: "captured-id", subject: "subject" },
          ],
        }),
      };
    }
    return { body: null, ok: true, status: 204 };
  };

  await cleanupCapturedEmail(
    "http://owlmail.test",
    "recipient@example.test",
    "subject",
    "",
    { fetchImpl },
  );
  assert.equal(requests.length, 2);
  assert.match(requests[0].url, /to=recipient%40example\.test/);
  assert.equal(requests[1].url, "http://owlmail.test/api/v1/emails/captured-id");
  assert.equal(requests[1].options.method, "DELETE");
});
