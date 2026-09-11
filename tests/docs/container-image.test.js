const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const { test } = require("bun:test");

const root = path.resolve(__dirname, "../..");
const dockerfile = fs.readFileSync(path.join(root, "Dockerfile"), "utf8");
const goMod = fs.readFileSync(path.join(root, "go.mod"), "utf8");
const workflow = fs.readFileSync(path.join(root, ".github/workflows/docker.yml"), "utf8");

function builderStage() {
  const pin = dockerfile.match(/^FROM golang:(\d+\.\d+\.\d+)-alpine(\d+\.\d+) AS builder$/m);
  assert.ok(pin, "the Dockerfile builder stage is not pinned to golang:<x.y.z>-alpine<x.y>");
  return { go: pin[1], alpine: pin[2] };
}

function runtimeAlpine() {
  const pin = dockerfile.match(/^FROM alpine:(\d+\.\d+)\.\d+$/m);
  assert.ok(pin, "the Dockerfile runtime stage is not pinned to alpine:<x.y.z>");
  return pin[1];
}

// `actions/setup-go` resolves `go-version-file: go.mod` from the toolchain
// directive when one is present and from the go directive otherwise. Every
// workflow installs Go that way, so this is the toolchain behind the released
// binaries, the race tests, and govulncheck.
function declaredToolchain() {
  const toolchain = goMod.match(/^toolchain go(\d+\.\d+\.\d+)$/m);
  if (toolchain) {
    return toolchain[1];
  }
  const directive = goMod.match(/^go (\d+(?:\.\d+)*)$/m);
  assert.ok(directive, "go.mod does not declare a go directive");
  assert.match(
    directive[1],
    /^\d+\.\d+\.\d+$/,
    "go.mod must pin a patch version, or add a toolchain directive, so the image can be held to it",
  );
  return directive[1];
}

test("the image and the release binaries are built with one Go version", () => {
  const builder = builderStage();
  const toolchain = declaredToolchain();
  assert.equal(
    builder.go,
    toolchain,
    `Dockerfile builds with Go ${builder.go} while go.mod puts CI and the release binaries on Go ${toolchain}; ` +
      "a base image bump has to carry the matching go.mod toolchain line, otherwise one tag ships two toolchains",
  );
});

test("both Dockerfile stages track the same Alpine release", () => {
  const builder = builderStage();
  const runtime = runtimeAlpine();
  assert.equal(
    builder.alpine,
    runtime,
    `the builder stage is on Alpine ${builder.alpine} and the runtime stage on Alpine ${runtime}; ` +
      "Dependabot never moves the alpine suffix of a golang tag, so the two stages have to be moved together by hand",
  );
});

test("pull requests build and run the image instead of publishing it", () => {
  assert.match(workflow, /^ {2}pull_request:$/m, "docker.yml does not run on pull requests");
  assert.ok(
    workflow.includes("if: github.event_name == 'pull_request'"),
    "the verification job is not restricted to pull requests",
  );
  assert.ok(
    workflow.includes("if: github.event_name != 'pull_request'"),
    "the publishing job is not held back on pull requests",
  );
  assert.ok(workflow.includes("load: true"), "the verification job does not load the image locally");
  assert.ok(workflow.includes("push: false"), "the verification job does not opt out of pushing");
  assert.ok(workflow.includes("/readyz"), "the verification job does not wait for the container");
  assert.ok(
    workflow.includes('\\"go_version\\":\\"go${expected_go}\\"'),
    "the verification job does not confirm the container was built with the Go version the Dockerfile pins",
  );
  assert.ok(
    workflow.includes("python3 examples/testing/python/email_test.py"),
    "the verification job does not capture mail through the running container",
  );
  assert.ok(workflow.includes("trap cleanup EXIT"), "the verification job leaks the container on failure");
});
