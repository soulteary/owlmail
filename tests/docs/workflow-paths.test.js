const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const { test } = require("bun:test");

const root = path.resolve(__dirname, "../..");

function triggerBlock(workflow, event, nextEvent) {
  const start = workflow.indexOf(`  ${event}:`);
  assert.notEqual(start, -1, `missing ${event} trigger`);
  const end = nextEvent === undefined ? workflow.length : workflow.indexOf(`  ${nextEvent}:`, start + 1);
  return workflow.slice(start, end === -1 ? workflow.length : end);
}

test("badge-only commits do not rebuild binaries or container images", () => {
  const build = fs.readFileSync(path.join(root, ".github/workflows/build.yml"), "utf8");
  const docker = fs.readFileSync(path.join(root, ".github/workflows/docker.yml"), "utf8");

  for (const [name, workflow] of [["Build", build], ["Docker", docker]]) {
    const push = triggerBlock(workflow, "push", "pull_request");
    assert.match(
      push,
      /^\s+- ['"]?\.github\/goreportcard\.svg['"]?$/m,
      `${name} push trigger does not ignore the generated badge`,
    );

    const pullRequest = triggerBlock(workflow, "pull_request", "workflow_dispatch");
    assert.match(
      pullRequest,
      /^\s+- ['"]?\.github\/goreportcard\.svg['"]?$/m,
      `${name} pull request trigger does not ignore the generated badge`,
    );
  }
});


test('stacked pull requests run CI and analyzer details stay in artifacts', () => {
    const ci = fs.readFileSync(path.join(root, '.github/workflows/ci.yml'), 'utf8');
    assert.match(ci, /pull_request:\s*\n\nconcurrency:/);
    const reports = fs.readFileSync(path.join(root, '.github/workflows/go-quality-reports.yml'), 'utf8');
    assert.ok(reports.includes('.github/goreportcard-report.md'));
    assert.equal(reports.includes('cp "$REPORT_DIR/.github/goreportcard-report.md"'), false);
    assert.equal(reports.match(/git add (?:-f )?-- ([^\n]+)/)[1].includes('goreportcard-report.md'), false);
});

test("development image publication cannot write stable release aliases", () => {
  const workflow = Bun.YAML.parse(fs.readFileSync(path.join(root, ".github/workflows/docker.yml"), "utf8"));
  const publish = workflow.jobs["build-and-push"];
  assert.equal(publish.if, "github.event_name != 'pull_request' && github.ref == 'refs/heads/main'",
    "push and manual publication must both be restricted to main");
  const metadata = publish.steps.find(step => step.id === "meta");
  assert.equal(metadata.with.flavor, "latest=false", "metadata must not create an automatic latest alias");
  const tags = metadata.with.tags.trim().split(/\r?\n/);
  assert.deepEqual(tags, ["type=raw,value=main", "type=sha,prefix=sha-"],
    "development builds may only publish main and commit snapshots");
  const pushes = publish.steps.filter(step => step.uses?.startsWith("docker/build-push-action@"));
  assert.equal(pushes.length, 1);
  assert.equal(pushes[0].with.tags, "${{ steps.meta.outputs.tags }}",
    "image publication must use only the restricted development metadata");

  const release = Bun.YAML.parse(fs.readFileSync(path.join(root, ".github/workflows/release.yml"), "utf8"));
  const releaseMetadata = release.jobs.release.steps.find(step => step.id === "image-meta");
  assert.equal(releaseMetadata.with.flavor, "latest=false");
  assert.match(releaseMetadata.with.tags, /type=raw,value=latest,enable=\$\{\{ steps\.moving-tags\.outputs\.enabled == 'true' \}\}/,
    "formal releases must retain the explicit newest-stable guard for latest");
});
