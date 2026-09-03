const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const { test } = require("bun:test");

function triggerPaths(workflow, event, nextEvent) {
  const start = workflow.indexOf(`  ${event}:`);
  assert.notEqual(start, -1, `missing ${event} trigger`);
  const end = workflow.indexOf(`  ${nextEvent}:`, start + 1);
  const block = workflow.slice(start, end === -1 ? workflow.length : end);
  const pathsStart = block.indexOf("    paths:");
  assert.notEqual(pathsStart, -1, `${event} trigger is missing paths`);
  return [...block.slice(pathsStart).matchAll(/^      - '([^']+)'$/gm)].map((match) => match[1]);
}

test("CI runs all three integration examples against a live OwlMail binary", () => {
  const workflow = fs.readFileSync(
    path.resolve(__dirname, "../../.github/workflows/integration-examples.yml"),
    "utf8",
  );
  assert.ok(workflow.includes('go build -o "${RUNNER_TEMP}/owlmail" ./cmd/owlmail'));
  assert.ok(workflow.includes("http://127.0.0.1:1080/readyz"));
  assert.ok(workflow.includes("node examples/testing/javascript/email-test.mjs"));
  assert.ok(workflow.includes("python3 examples/testing/python/email_test.py"));
  assert.ok(workflow.includes("OWLMAIL_RUN_INTEGRATION_TEST=1 go test -count=1 ./examples/testing/go -v"));
  assert.ok(workflow.includes("trap cleanup EXIT"));
  assert.ok(workflow.includes("if: failure()"));

  const expectedPaths = [
    "**/*.go",
    "go.mod",
    "go.sum",
    "examples/testing/**",
    ".github/workflows/integration-examples.yml",
  ];
  assert.deepEqual(triggerPaths(workflow, "push", "pull_request"), expectedPaths);
  assert.deepEqual(triggerPaths(workflow, "pull_request", "workflow_dispatch"), expectedPaths);
  assert.match(workflow, /^  workflow_dispatch:\s*$/m);
});
