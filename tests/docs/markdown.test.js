const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const { test } = require("bun:test");

const root = path.resolve(__dirname, "../..");
const translatedReadmes = [
  "README.md",
  "README.zh-CN.md",
  "README.de.md",
  "README.fr.md",
  "README.it.md",
  "README.ja.md",
  "README.ko.md",
];

test("Webhook demo publishes host ports on loopback only", () => {
  const compose = fs.readFileSync(
    path.join(root, "examples/webhooks/soulteary-webhook/compose.yaml"),
    "utf8",
  );
  for (const port of ["9000", "1025", "1080"]) {
    assert.match(compose, new RegExp(`127\\.0\\.0\\.1:${port}:${port}`));
  }
  assert.doesNotMatch(compose, /^\s*-\s*["']?(?:9000|1025|1080):/m);
  assert.ok(compose.includes("image: ghcr.io/soulteary/owlmail:0.6.0"));
  assert.doesNotMatch(compose, /^\s*image:\s*soulteary\/owlmail/m);
});

test("Webhook demo distinguishes HTTP proxying from SMTP network controls", () => {
  const english = fs.readFileSync(
    path.join(root, "examples/webhooks/soulteary-webhook/README.md"),
    "utf8",
  );
  assert.match(english, /This does not\s+protect SMTP/);
  assert.ok(english.includes("network policy, a firewall, or a private tunnel"));

  const chinese = fs.readFileSync(
    path.join(root, "examples/webhooks/soulteary-webhook/README.zh-CN.md"),
    "utf8",
  );
  assert.ok(chinese.includes("不能保护 SMTP"));
  assert.ok(chinese.includes("网络策略、防火墙"));
  assert.ok(chinese.includes("私有隧道"));
});

function walkMarkdown(directory) {
  const files = [];
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    if (entry.name === ".git" || entry.name === "node_modules") continue;
    const fullPath = path.join(directory, entry.name);
    if (entry.isDirectory()) files.push(...walkMarkdown(fullPath));
    if (entry.isFile() && entry.name.endsWith(".md")) files.push(fullPath);
  }
  return files;
}

function withoutFencedCode(markdown) {
  const output = [];
  let fence = null;

  for (const line of markdown.split(/\r?\n/)) {
    const match = line.match(/^\s*(`{3,}|~{3,})/);
    if (match) {
      const marker = match[1][0];
      if (fence === null) fence = marker;
      else if (fence === marker) fence = null;
      output.push("");
      continue;
    }
    output.push(fence === null ? line : "");
  }

  return { text: output.join("\n"), openFence: fence };
}

function localDestinations(markdown) {
  const destinations = [];
  const inlineLink = /!?\[[^\]]*\]\(([^)]+)\)/g;
  const referenceLink = /^\s*\[[^\]]+\]:\s*(\S+)/gm;
  const htmlLink = /\b(?:href|src)=["']([^"']+)["']/gi;

  for (const expression of [inlineLink, referenceLink, htmlLink]) {
    for (const match of markdown.matchAll(expression)) {
      let destination = match[1].trim();
      if (destination.startsWith("<") && destination.endsWith(">")) {
        destination = destination.slice(1, -1);
      } else {
        destination = destination.split(/\s+["']/)[0];
      }

      if (
        destination === "" ||
        destination.startsWith("/") ||
        destination.startsWith("//") ||
        /^[a-z][a-z\d+.-]*:/i.test(destination)
      ) {
        continue;
      }

      destinations.push(destination);
    }
  }

  return destinations;
}

function githubHeadingAnchors(markdown) {
  const { text } = withoutFencedCode(markdown);
  const anchors = new Set();
  const counts = new Map();

  for (const line of text.split(/\r?\n/)) {
    const heading = line.match(/^\s{0,3}#{1,6}\s+(.+?)\s*#*\s*$/);
    if (heading) {
      const label = heading[1]
        .replace(/!\[([^\]]*)\]\([^)]+\)/g, "$1")
        .replace(/\[([^\]]+)\]\([^)]+\)/g, "$1")
        .replace(/<[^>]+>/g, "")
        .replace(/[`*_~]/g, "")
        .trim()
        .toLowerCase();
      const base = label
        .replace(/[^\p{L}\p{N}\s_-]/gu, "")
        .replace(/\s+/g, "-");
      const count = counts.get(base) || 0;
      anchors.add(count === 0 ? base : `${base}-${count}`);
      counts.set(base, count + 1);
    }

    for (const id of line.matchAll(/\bid=["']([^"']+)["']/gi)) {
      anchors.add(id[1]);
    }
  }

  return anchors;
}

function extractConfigContract() {
  const source = fs.readFileSync(path.join(root, "internal/config/config.go"), "utf8");
  const constants = new Map(
    [...source.matchAll(/^const\s+(\w+)\s*=\s*([^\n]+)$/gm)].map((match) => [match[1], match[2].trim()]),
  );
  const defaultStart = source.indexOf("func DefaultConfig()");
  const defaultEnd = source.indexOf("// FlagRefs holds", defaultStart);
  const defaultBlock = source.slice(defaultStart, defaultEnd);
  const defaults = new Map(
    [...defaultBlock.matchAll(/^\s*(\w+):\s*([^,\n]+),/gm)].map((match) => [match[1], match[2].trim()]),
  );
  const flags = [...source.matchAll(/fs\.(?:Bool|Int|String)\("([^"]+)",\s*cfg\.(\w+),/g)].map(
    (match) => ({ name: match[1], field: match[2] }),
  );
  const environment = [
    ...new Set([...source.matchAll(/\b(?:OWLMAIL|MAILDEV)_[A-Z0-9_]+\b/g)].map((match) => match[0])),
  ].sort();

  function resolveLiteral(raw) {
    if (constants.has(raw)) return resolveLiteral(constants.get(raw));
    if (raw === "true" || raw === "false" || /^-?\d+$/.test(raw)) return raw;
    if (raw.startsWith('"')) return JSON.parse(raw);
    throw new Error(`unsupported default literal ${raw}`);
  }

  return {
    environment,
    flags: flags.map((flag) => {
      assert.ok(defaults.has(flag.field), `missing default for config field ${flag.field}`);
      const value = resolveLiteral(defaults.get(flag.field));
      return { ...flag, defaultValue: value === "" ? "-" : value };
    }),
  };
}

function configRows(markdown) {
  const rows = new Map();
  for (const line of markdown.split(/\r?\n/)) {
    const match = line.match(/^\|\s*`-([^`]+)`\s*\|\s*([^|]*)\|\s*([^|]*)\|/);
    if (match) rows.set(match[1], { environment: match[2].trim(), defaultValue: match[3].trim() });
  }
  return rows;
}

function extractAPIRoutes() {
  const source = fs.readFileSync(path.join(root, "internal/api/api.go"), "utf8");
  const sections = [
    source.slice(source.indexOf("func (api *API) setupImprovedAPIRoutes"), source.indexOf("// Start starts the API server")),
    source.slice(source.indexOf("func (api *API) setupMailDevCompatibleRoutes")),
  ];
  const routes = [];

  for (const section of sections) {
    const prefixes = new Map([["app", ""]]);
    for (const match of section.matchAll(/(\w+)\s*:=\s*(\w+)\.Group\((?:api\.route\()?"([^"]*)"\)?\)/g)) {
      const [, name, parent, segment] = match;
      assert.ok(prefixes.has(parent), `unknown API route group ${parent}`);
      prefixes.set(name, prefixes.get(parent) + segment);
    }
    for (const match of section.matchAll(/(\w+)\.(Get|Post|Put|Patch|Delete)\((?:api\.route\()?"([^"]*)"/g)) {
      const [, group, method, route] = match;
      assert.ok(prefixes.has(group), `unknown API route group ${group}`);
      routes.push(`${method.toUpperCase()} ${prefixes.get(group)}${route}`);
    }
  }

  return [...new Set(routes)].sort();
}

test("Markdown has balanced fences and valid local links", () => {
  const failures = [];
  const anchorCache = new Map();

  for (const file of walkMarkdown(root)) {
    const relative = path.relative(root, file);
    const markdown = fs.readFileSync(file, "utf8");
    const { text, openFence } = withoutFencedCode(markdown);
    if (openFence !== null) failures.push(`${relative}: unclosed ${openFence} fence`);

    for (const destination of localDestinations(text)) {
      const [beforeFragment, fragment = ""] = destination.split("#", 2);
      const filePart = beforeFragment.split("?", 1)[0];
      const decodedPath = decodeURIComponent(filePart);
      const resolved = decodedPath === "" ? file : path.resolve(path.dirname(file), decodedPath);
      if (!fs.existsSync(resolved)) {
        failures.push(`${relative}: missing ${decodedPath}`);
        continue;
      }

      if (fragment !== "" && path.extname(resolved).toLowerCase() === ".md") {
        if (!anchorCache.has(resolved)) {
          anchorCache.set(resolved, githubHeadingAnchors(fs.readFileSync(resolved, "utf8")));
        }
        const decodedFragment = decodeURIComponent(fragment);
        if (!anchorCache.get(resolved).has(decodedFragment)) {
          failures.push(`${relative}: missing anchor ${destination}`);
        }
      }
    }
  }

  assert.deepEqual(failures, []);
});

test("root README configuration tables match flags, defaults, and environment aliases", () => {
  const contract = extractConfigContract();
  assert.ok(contract.flags.length > 0, "no configuration flags found");

  for (const readme of translatedReadmes) {
    const markdown = fs.readFileSync(path.join(root, readme), "utf8");
    const rows = configRows(markdown);

    for (const flag of contract.flags) {
      assert.ok(rows.has(flag.name), `${readme} is missing -${flag.name}`);
      assert.equal(
        rows.get(flag.name).defaultValue,
        flag.defaultValue,
        `${readme} has the wrong default for -${flag.name}`,
      );
    }

    const missingEnvironment = contract.environment.filter((name) => !markdown.includes(`\`${name}\``));
    assert.deepEqual(missingEnvironment, [], `${readme} is missing environment variables`);
  }
});

test("security-sensitive SMTP authentication modes remain explicit", () => {
  const smtpWarnings = new Map([
    ["README.md", "NO AUTH"],
    ["README.zh-CN.md", "NO AUTH"],
    ["README.de.md", "NO AUTH"],
    ["README.fr.md", "NO AUTH"],
    ["README.it.md", "NO AUTH"],
    ["README.ja.md", "NO AUTH"],
    ["README.ko.md", "NO AUTH"],
  ]);

  for (const [readme, warning] of smtpWarnings) {
    const markdown = fs.readFileSync(path.join(root, readme), "utf8");
    assert.ok(markdown.includes(warning), `${readme} obscures the SMTP authentication modes`);
  }

  const references = [
    ["docs/en/API-Reference.md", ["OWLMAIL_WEB_USER", "OWLMAIL_WEB_PASSWORD", "Startup fails", "PLAIN/LOGIN", "NO AUTH"]],
    ["docs/zh-CN/API-Reference.md", ["OWLMAIL_WEB_USER", "OWLMAIL_WEB_PASSWORD", "启动失败", "PLAIN/LOGIN", "NO AUTH"]],
  ];
  for (const [reference, markers] of references) {
    const markdown = fs.readFileSync(path.join(root, reference), "utf8");
    for (const marker of markers) {
      assert.ok(markdown.includes(marker), `${reference} is missing security contract marker ${marker}`);
    }
  }

  const help = fs.readFileSync(path.join(root, "web/help.html"), "utf8");
  for (const marker of [
    "permits unauthenticated delivery",
    "允许不认证投递",
    "PLAIN/LOGIN",
    "NO AUTH",
    "100 MiB",
    "smtp-max-message-mb",
    "50 recipients",
    "50 个收件人",
    "OWLMAIL_SMTP_MAX_CONCURRENCY",
    "451 4.3.2",
  ]) {
    assert.ok(help.includes(marker), `web/help.html is missing SMTP ingress marker ${marker}`);
  }

  const capacityReferences = [
    ["docs/en/Webhook-Forwarding.md", ["SMTP `DATA` completion", ".owlmail-webhook-outbox", "drains the outbox", "100 UTF-8 bytes"]],
    ["docs/zh-CN/Webhook-Forwarding.md", ["SMTP `DATA` 命令", ".owlmail-webhook-outbox", "outbox、排队任务", "100 个 UTF-8 字节"]],
  ];
  for (const [reference, markers] of capacityReferences) {
    const markdown = fs.readFileSync(path.join(root, reference), "utf8");
    for (const marker of markers) {
      assert.ok(markdown.includes(marker), `${reference} is missing delivery boundary marker ${marker}`);
    }
  }
});

test("English and Chinese API references cover every registered API route", () => {
  const routes = extractAPIRoutes();
  assert.ok(routes.length > 0, "no API routes found");

  for (const reference of ["docs/en/API-Reference.md", "docs/zh-CN/API-Reference.md"]) {
    const markdown = fs.readFileSync(path.join(root, reference), "utf8");
    const missing = routes.filter((route) => !markdown.includes(`\`${route}\``));
    assert.deepEqual(missing, [], `${reference} is missing API routes`);
  }
});

test("OpenAPI contract is linked from every translated README", () => {
  const document = JSON.parse(fs.readFileSync(path.join(root, "openapi/openapi.json"), "utf8"));
  assert.equal(document.openapi, "3.1.0");
  assert.equal(document.jsonSchemaDialect, "https://json-schema.org/draft/2020-12/schema");

  const yaml = fs.readFileSync(path.join(root, "openapi/openapi.yaml"), "utf8");
  assert.ok(yaml.startsWith("openapi: 3.1.0\n"));
  assert.ok(yaml.includes("/emails/{id}/actions/relay:"));
  assert.ok(yaml.includes("/ws:"));

  for (const readme of translatedReadmes) {
    const markdown = fs.readFileSync(path.join(root, readme), "utf8");
    for (const marker of [
      "/api/v1/openapi.json",
      "/api/v1/openapi.yaml",
      "openapi/openapi.yaml",
    ]) {
      assert.ok(markdown.includes(marker), `${readme} is missing OpenAPI marker ${marker}`);
    }
  }
});

test("0.6.0 release documentation and workflow stay connected", () => {
  const changelog = fs.readFileSync(path.join(root, "CHANGELOG.md"), "utf8");
  const releaseStart = changelog.indexOf("## [0.6.0]");
  const releaseEnd = changelog.indexOf("## [0.5.0]", releaseStart);
  assert.ok(releaseStart >= 0 && releaseEnd > releaseStart, "CHANGELOG.md is missing the 0.6.0 release section");
  const releaseSection = changelog.slice(releaseStart, releaseEnd);
  for (const marker of [
    "Mailbox governance",
    "Redis Streams-backed webhook delivery",
    "local webhook outbox",
    "Service-worker browser notifications",
  ]) {
    assert.ok(releaseSection.includes(marker), `CHANGELOG.md 0.6.0 section is missing ${marker}`);
  }
  assert.ok(changelog.includes("[0.6.0]:"), "CHANGELOG.md is missing the 0.6.0 comparison link");
  assert.ok(
    !changelog.slice(changelog.indexOf("## [Unreleased]"), releaseStart).includes("Mailbox governance"),
    "CHANGELOG.md still classifies the 0.6.0 storage work as unreleased",
  );

  const releaseNotes = [
    [
      "docs/en/Release-0.6.0.md",
      ["Atomic mailbox persistence and recovery", "Storage governance", "Durable webhook handoff and Redis delivery", "Known limitations", "owlmail-linux-amd64"],
    ],
    [
      "docs/zh-CN/Release-0.6.0.md",
      ["原子邮件持久化与恢复", "存储治理", "持久化 Webhook 交接与 Redis 投递", "已知限制", "owlmail-linux-amd64"],
    ],
  ];
  for (const [releaseNote, markers] of releaseNotes) {
    const markdown = fs.readFileSync(path.join(root, releaseNote), "utf8");
    for (const marker of markers) {
      assert.ok(markdown.includes(marker), `${releaseNote} is missing ${marker}`);
    }
  }

  for (const readme of translatedReadmes) {
    const markdown = fs.readFileSync(path.join(root, readme), "utf8");
    assert.ok(markdown.includes("ghcr.io/soulteary/owlmail:0.6.0"), `${readme} does not pin the release image`);
    assert.ok(markdown.includes("Release-0.6.0.md"), `${readme} does not link the release notes`);
    assert.ok(markdown.includes("`/webhooks`"), `${readme} does not document the webhook configurator`);
    assert.ok(markdown.includes("Bun"), `${readme} does not distinguish the Bun build tool from runtime requirements`);
  }

  for (const operations of ["docs/en/Operations.md", "docs/zh-CN/Operations.md"]) {
    const markdown = fs.readFileSync(path.join(root, operations), "utf8");
    assert.ok(markdown.includes("ghcr.io/soulteary/owlmail:0.6.0"), `${operations} does not pin 0.6.0`);
    assert.ok(!markdown.includes("ghcr.io/soulteary/owlmail:latest"), `${operations} uses a moving image`);
  }

  for (const reference of ["docs/en/API-Reference.md", "docs/zh-CN/API-Reference.md"]) {
    const markdown = fs.readFileSync(path.join(root, reference), "utf8");
    for (const field of ["version", "commit", "build_date", "branch", "go_version", "platform", "compiler"]) {
      assert.ok(markdown.includes(`"${field}"`), `${reference} is missing version field ${field}`);
    }
    assert.ok(markdown.includes('"version": "0.6.0"'), `${reference} does not show the 0.6.0 version`);
    assert.ok(markdown.includes('"branch": "v0.6.0"'), `${reference} does not show the 0.6.0 tag`);
  }

  const workflow = fs.readFileSync(path.join(root, ".github/workflows/release.yml"), "utf8");
  for (const marker of [
    "git rev-parse --verify",
    "git checkout --detach",
    'NOTES="docs/en/Release-${VERSION#v}.md"',
    "github.com/soulteary/version-kit/v2.Version",
    "Verify embedded release metadata",
    "Run release preflight checks",
    "Scan for reachable Go vulnerabilities",
    "Verify browser assets and documentation",
    "govulncheck@${GOVULNCHECK_VERSION}",
    "body_path: ${{ steps.release.outputs.notes }}",
    "fail_on_unmatched_files: true",
    "git fetch --force --tags origin",
    "enable=${{ steps.moving-tags.outputs.enabled == 'true' }}",
    "group: release-${{ github.ref }}",
    "flavor: latest=false",
    "Refuse to overwrite exact release image tag",
    'scope=repository:${IMAGE_NAME}:pull',
    'Authorization: Bearer ${REGISTRY_TOKEN}',
    "Could not exchange the GitHub credential for a GHCR registry token",
    "Refusing to overwrite published release image",
    "Registry returned HTTP ${HTTP_STATUS}",
    "failing closed",
  ]) {
    assert.ok(workflow.includes(marker), `.github/workflows/release.yml is missing ${marker}`);
  }
  const movingTagCheck = workflow.indexOf("- name: Revalidate moving image tags");
  const imageMetadata = workflow.indexOf("- name: Extract release image metadata");
  const imagePush = workflow.indexOf("- name: Build and push release image");
  assert.ok(movingTagCheck < imageMetadata && imageMetadata < imagePush,
    "moving image tags must be revalidated immediately before metadata generation and image publication");
  assert.ok(!workflow.includes("group: release-publishing"),
    "workflow-wide release serialization can cancel an unrelated pending release");
  const exactTagGuard = workflow.indexOf("- name: Refuse to overwrite exact release image tag");
  const goSetup = workflow.indexOf("- name: Set up Go");
  assert.ok(exactTagGuard >= 0 && exactTagGuard < goSetup,
    "exact release tags must be checked before expensive release builds");
  assert.ok(!workflow.includes("type=sha,prefix=sha-"),
    "the release workflow must not overwrite default-branch sha aliases");

  for (const reference of [
    "docs/en/Release-0.6.0.md",
    "docs/zh-CN/Release-0.6.0.md",
    "docs/en/Operations.md",
    "docs/zh-CN/Operations.md",
  ]) {
    const markdown = fs.readFileSync(path.join(root, reference), "utf8");
    assert.ok(markdown.includes("ghcr.io/soulteary/owlmail@sha256:<digest>"),
      `${reference} does not recommend an immutable image digest`);
  }

  const dockerfile = fs.readFileSync(path.join(root, "Dockerfile"), "utf8");
  for (const marker of ["ARG VERSION=dev", "version-kit/v2.Version=${VERSION}", "version-kit/v2.Commit=${COMMIT}"]) {
    assert.ok(dockerfile.includes(marker), `Dockerfile is missing release metadata marker ${marker}`);
  }

  const dockerWorkflow = fs.readFileSync(path.join(root, ".github/workflows/docker.yml"), "utf8");
  for (const marker of [
    "build-args:",
    "VERSION=${{ steps.meta.outputs.version }}",
    "COMMIT=${{ github.sha }}",
    "BUILD_DATE=${{ steps.build-meta.outputs.build-date }}",
    "BRANCH=${{ github.ref_name }}",
  ]) {
    assert.ok(dockerWorkflow.includes(marker), `.github/workflows/docker.yml is missing ${marker}`);
  }
});

test("release workflow preserves supply-chain evidence", () => {
  const workflow = fs.readFileSync(path.join(root, ".github/workflows/release.yml"), "utf8");
  for (const marker of [
    "id-token: write",
    "attestations: write",
    "anchore/sbom-action@3ad7283483fc7af8ff2b4ea19663c2d5ca935e26",
    "actions/attest-build-provenance@4d101475d8b20a2381f78447822ac1eab6504dd8",
    "sigstore/cosign-installer@6f9f17788090df1f26f669e9d70d6ae9567deba6",
    "subject-checksums: checksums.txt",
    "checksums.txt.sigstore.json",
    "sbom: true",
    "provenance: mode=max",
    "push-to-registry: true",
    "org.opencontainers.image.source",
    "org.opencontainers.image.revision",
    "org.opencontainers.image.version",
    "org.opencontainers.image.licenses=MIT",
    'cosign sign --yes "${REGISTRY}/${IMAGE_NAME}@${IMAGE_DIGEST}"',
    '"${GITHUB_REF}" != "refs/tags/${VERSION}"',
  ]) {
    assert.ok(workflow.includes(marker), `.github/workflows/release.yml is missing ${marker}`);
  }

  for (const guide of ["docs/en/Releasing.md", "docs/zh-CN/Releasing.md"]) {
    const markdown = fs.readFileSync(path.join(root, guide), "utf8");
    for (const marker of [
      "checksums.txt.sigstore.json",
      "gh attestation verify",
      "cosign verify-blob",
      "cosign verify",
      "--ref v0.6.0",
    ]) {
      assert.ok(markdown.includes(marker), `${guide} is missing supply-chain marker ${marker}`);
    }
  }
});

test("browser and documentation tests use the pinned Bun runner", () => {
  const legacyRunner = `node:${"test"}`;
  for (const testFile of [
    "tests/web/app.test.js",
    "tests/web/webhooks.test.js",
    "tests/docs/markdown.test.js",
  ]) {
    const source = fs.readFileSync(path.join(root, testFile), "utf8");
    assert.ok(source.includes('require("bun:test")') || source.includes("require('bun:test')"), `${testFile} does not use bun:test`);
    assert.ok(
      !source.includes(`require("${legacyRunner}")`) && !source.includes(`require('${legacyRunner}')`),
      `${testFile} still uses ${legacyRunner}`,
    );
  }

  assert.equal(fs.readFileSync(path.join(root, ".bun-version"), "utf8").trim(), "1.4.0");
});
