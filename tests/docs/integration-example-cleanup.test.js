const assert = require("node:assert/strict");
const { test } = require("bun:test");

test("JavaScript integration cleanup preserves the primary error", async () => {
  const { withCleanup } = await import("../../examples/testing/javascript/cleanup.mjs");
  const primaryError = new Error("verification failed");
  const cleanupError = new Error("cleanup failed");

  await assert.rejects(
    withCleanup(
      async () => {
        throw primaryError;
      },
      async () => {
        throw cleanupError;
      },
    ),
    (error) => {
      assert.ok(error instanceof AggregateError);
      assert.equal(error.cause, primaryError);
      assert.deepEqual(error.errors, [primaryError, cleanupError]);
      return true;
    },
  );
});

test("JavaScript integration cleanup still reports a cleanup-only error", async () => {
  const { withCleanup } = await import("../../examples/testing/javascript/cleanup.mjs");
  const cleanupError = new Error("cleanup failed");

  await assert.rejects(
    withCleanup(async () => "verified", async () => {
      throw cleanupError;
    }),
    cleanupError,
  );
});
