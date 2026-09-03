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
  assert.ok(compose.includes("image: ghcr.io/soulteary/owlmail:0.8.0"));
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

function fencedBlocks(markdown, acceptedLanguages) {
  const blocks = [];
  const expression = /^```([^\r\n]*)\r?\n([\s\S]*?)^```\s*$/gm;
  for (const match of markdown.matchAll(expression)) {
    const language = match[1].trim().toLowerCase();
    if (acceptedLanguages === undefined || acceptedLanguages.includes(language)) {
      blocks.push({ language, body: match[2] });
    }
  }
  return blocks;
}

function shellCommands(markdown) {
  const commands = [];
  for (const { body } of fencedBlocks(markdown, ["bash", "sh", "shell"])) {
    let command = "";
    for (const sourceLine of body.split(/\r?\n/)) {
      const line = sourceLine.trim();
      if (line === "" || line.startsWith("#")) continue;
      command += `${command === "" ? "" : " "}${line.replace(/\\$/, "").trim()}`;
      if (!line.endsWith("\\")) {
        commands.push(command);
        command = "";
      }
    }
    if (command !== "") commands.push(command);
  }
  return commands;
}

function assertPrivateDockerPorts(command, label) {
  const published = [...command.matchAll(/(?:^|\s)-p\s+(\S+)/g)].map((match) => match[1]);
  assert.ok(published.length > 0, `${label} does not publish any ports`);
  for (const port of published) {
    assert.match(port, /^127\.0\.0\.1:\d+:\d+$/, `${label} publishes ${port} beyond loopback`);
  }
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
  const mailDevCompatSource = fs.readFileSync(
    path.join(root, "internal/api/api_maildev_compat.go"),
    "utf8",
  );
  const sections = [
    source.slice(source.indexOf("func (api *API) setupImprovedAPIRoutes"), source.indexOf("// Start starts the API server")),
    source.slice(source.indexOf("func (api *API) setupMailDevCompatibleRoutes")),
    mailDevCompatSource.slice(mailDevCompatSource.indexOf("func (api *API) setupMailDevRESTCompatRoutes")),
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

test("sendmail CLI documentation covers every locale and its stable contract", () => {
  const localeDocs = ["en", "zh-CN", "de", "fr", "it", "ja", "ko"].map(
    (locale) => `docs/${locale}/Sendmail.md`,
  );
  for (const document of localeDocs) {
    const markdown = fs.readFileSync(path.join(root, document), "utf8");
    for (const marker of [
      "owlmail sendmail -t -i",
      "sendmail_path",
      "OWLMAIL_SENDMAIL_HOST",
      "OWLMAIL_SENDMAIL_PORT",
      "OWLMAIL_SENDMAIL_STARTTLS",
      "OWLMAIL_SENDMAIL_SMTPS",
      "OWLMAIL_SENDMAIL_USERNAME",
      "OWLMAIL_SENDMAIL_PASSWORD",
      "OWLMAIL_SENDMAIL_TIMEOUT",
      "`64`",
      "`65`",
      "`69`",
      "`74`",
      "`75`",
    ]) {
      assert.ok(markdown.includes(marker), `${document} is missing ${marker}`);
    }
  }

  for (const readme of translatedReadmes) {
    const markdown = fs.readFileSync(path.join(root, readme), "utf8");
    assert.ok(markdown.includes("Sendmail.md"), `${readme} does not link the sendmail guide`);
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
    "smtp-read-timeout",
    "OWLMAIL_SMTP_READ_TIMEOUT",
    "smtp-write-timeout",
    "OWLMAIL_SMTP_WRITE_TIMEOUT",
    "smtp-max-recipients",
    "OWLMAIL_SMTP_MAX_RECIPIENTS",
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

test("three-way comparison stays source-pinned and reflects the 0.8.0 contract", () => {
  for (const document of [
    "docs/en/Comparison-and-Migration.md",
    "docs/zh-CN/Comparison-and-Migration.md",
  ]) {
    const markdown = fs.readFileSync(path.join(root, document), "utf8");
    for (const marker of [
      "OwlMail × MailDev × MailCatcher",
      "2026-09-03",
      "e3d2cfcaf5580a7d914d1d27142a9edf43eaf8e9",
      "0.8.0",
      "9d4141f42b0acedfa544a306f96a5373ded8c8a3",
      "43e488e2a5692532c131a87d5bd16a973ee8db56",
      "0.11.0",
      "MCP",
      "MailCatcher",
      "Prometheus",
      "SQLite",
    ]) {
      assert.ok(markdown.includes(marker), `${document} is missing ${marker}`);
    }
    assert.ok(!markdown.includes("| MCP server | Current MailDev provides one | No |"));
    assert.ok(!markdown.includes("| MCP 服务 | 当前 MailDev 提供 | 不提供 |"));
    assert.ok(!markdown.includes("with five closed-world"));
    assert.ok(!markdown.includes("只包含五个封闭只读能力"));
    for (const obsolete of [
      "release 0.6.0",
      "正式版 0.6.0",
      "general application config file",
      "通用应用配置文件",
      "SMTP read/write timeouts and the recipient count are still fixed defaults",
      "SMTP 读写超时和收件人数仍使用固定默认值",
      "does not yet provide complete browser-history",
      "尚未提供完整浏览器历史",
      "The mailbox index remains in memory",
      "邮箱索引仍主要位于内存",
      "There is no Prometheus metrics endpoint",
      "没有 Prometheus 指标端点",
    ]) {
      assert.ok(!markdown.includes(obsolete), `${document} retains obsolete 0.6.0 claim ${obsolete}`);
    }
    assert.ok(markdown.includes("<base-pathname>/mcp"));
  }

  for (const locale of ["de", "fr", "it", "ja", "ko"]) {
    const stub = fs.readFileSync(
      path.join(root, `docs/${locale}/Comparison-and-Migration.md`),
      "utf8",
    );
    assert.ok(stub.includes("0.8.0"), `${locale} comparison does not identify the stable release`);
    assert.ok(stub.includes("stdio"), `${locale} comparison does not identify both MCP transports`);
    assert.ok(!stub.includes("v0.6.0"), `${locale} comparison still presents MCP as a post-0.6.0 main-only feature`);
  }
});

test("0.8.0 release documentation and workflow stay connected", () => {
  const changelog = fs.readFileSync(path.join(root, "CHANGELOG.md"), "utf8");
  const releaseStart = changelog.indexOf("## [0.8.0]");
  const releaseEnd = changelog.indexOf("## [0.7.0]", releaseStart);
  assert.ok(releaseStart >= 0 && releaseEnd > releaseStart, "CHANGELOG.md is missing the 0.8.0 release section");
  const releaseSection = changelog.slice(releaseStart, releaseEnd);
  for (const marker of [
    "Prometheus metrics",
    "layered YAML and JSON configuration",
    "MailCatcher REST facade",
    "read-only MCP stdio bridge",
  ]) {
    assert.ok(releaseSection.includes(marker), `CHANGELOG.md 0.8.0 section is missing ${marker}`);
  }
  assert.ok(changelog.includes("[0.8.0]:"), "CHANGELOG.md is missing the 0.8.0 comparison link");
  assert.ok(
    !changelog.slice(changelog.indexOf("## [Unreleased]"), releaseStart).includes("Prometheus metrics"),
    "CHANGELOG.md still classifies 0.8.0 operational work as unreleased",
  );

  const releaseNotes = [
    [
      "docs/en/Release-0.8.0.md",
      ["Persistent relay jobs", "Layered configuration and indexing", "MCP stdio bridge", "Known limitations", "owlmail-linux-amd64"],
    ],
    [
      "docs/zh-CN/Release-0.8.0.md",
      ["持久化中继任务", "分层配置与索引", "MCP stdio 桥接", "已知限制", "owlmail-linux-amd64"],
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
    assert.ok(markdown.includes("ghcr.io/soulteary/owlmail:0.8.0"), `${readme} does not pin the release image`);
    assert.ok(markdown.includes("Release-0.8.0.md"), `${readme} does not link the release notes`);
    assert.ok(markdown.includes("`/webhooks`"), `${readme} does not document the webhook configurator`);
    assert.ok(markdown.includes("Bun"), `${readme} does not distinguish the Bun build tool from runtime requirements`);
  }

  for (const operations of ["docs/en/Operations.md", "docs/zh-CN/Operations.md"]) {
    const markdown = fs.readFileSync(path.join(root, operations), "utf8");
    assert.ok(markdown.includes("ghcr.io/soulteary/owlmail:0.8.0"), `${operations} does not pin 0.8.0`);
    assert.ok(!markdown.includes("ghcr.io/soulteary/owlmail:latest"), `${operations} uses a moving image`);
  }

  for (const reference of ["docs/en/API-Reference.md", "docs/zh-CN/API-Reference.md"]) {
    const markdown = fs.readFileSync(path.join(root, reference), "utf8");
    for (const field of ["version", "commit", "build_date", "branch", "go_version", "platform", "compiler"]) {
      assert.ok(markdown.includes(`"${field}"`), `${reference} is missing version field ${field}`);
    }
    assert.ok(markdown.includes('"version": "0.8.0"'), `${reference} does not show the 0.8.0 version`);
    assert.ok(markdown.includes('"branch": "v0.8.0"'), `${reference} does not show the 0.8.0 tag`);
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
    "docs/en/Release-0.8.0.md",
    "docs/zh-CN/Release-0.8.0.md",
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
      "--ref v0.8.0",
    ]) {
      assert.ok(markdown.includes(marker), `${guide} is missing supply-chain marker ${marker}`);
    }
  }
});

test("integration and AI-first guides are complete, bilingual, and runnable", () => {
  const guideNames = [
    "Integration-Testing.md",
    "CI-Quickstart.md",
    "AI-Agent-Testing.md",
    "MCP-Reference.md",
    "Testing-Recipes.md",
    "Architecture.md",
    "Security-Model.md",
  ];
  for (const locale of ["en", "zh-CN"]) {
    const index = fs.readFileSync(path.join(root, `docs/${locale}/README.md`), "utf8");
    for (const name of guideNames) {
      const document = `docs/${locale}/${name}`;
      assert.ok(fs.existsSync(path.join(root, document)), `missing ${document}`);
      assert.ok(index.includes(name), `docs/${locale}/README.md does not link ${name}`);
    }
  }

  for (const readme of translatedReadmes) {
    const markdown = fs.readFileSync(path.join(root, readme), "utf8");
    for (const marker of ["Integration-Testing.md", "MCP-Reference.md", "Security-Model.md"]) {
      assert.ok(markdown.includes(marker), `${readme} does not link ${marker}`);
    }
  }

  for (const example of [
    "examples/testing/compose.yaml",
    "examples/testing/javascript/email-test.mjs",
    "examples/testing/go/email_test.go",
    "examples/testing/python/email_test.py",
  ]) {
    assert.ok(fs.existsSync(path.join(root, example)), `missing ${example}`);
  }
  const compose = fs.readFileSync(path.join(root, "examples/testing/compose.yaml"), "utf8");
  assert.ok(compose.includes("ghcr.io/soulteary/owlmail:0.8.0"));
  assert.match(compose, /127\.0\.0\.1:1025:1025/);
  assert.match(compose, /127\.0\.0\.1:1080:1080/);

  const goExample = fs.readFileSync(path.join(root, "examples/testing/go/email_test.go"), "utf8");
  assert.ok(goExample.includes('os.Getenv("OWLMAIL_RUN_INTEGRATION_TEST") != "1"'));
  for (const readme of ["examples/testing/README.md", "examples/testing/README.zh-CN.md"]) {
    const markdown = fs.readFileSync(path.join(root, readme), "utf8");
    assert.ok(markdown.includes("OWLMAIL_RUN_INTEGRATION_TEST=1 go test"));
  }

  const mcp = fs.readFileSync(path.join(root, "docs/en/MCP-Reference.md"), "utf8");
  for (const marker of [
    "list_emails",
    "search_emails",
    "get_email",
    "get_email_source",
    "list_attachments",
    "get_latest_email",
    "wait_for_email",
    "owlmail://inbox",
    "owlmail://stats",
    "owlmail://email/{id}",
  ]) {
    assert.ok(mcp.includes(marker), `MCP reference is missing ${marker}`);
  }
});

test("GitHub community files match repository features and supported conventions", () => {
  assert.ok(!fs.existsSync(path.join(root, ".github/ISSUE_TEMPLATE/config.zh-CN.yml")));
  assert.ok(!fs.existsSync(path.join(root, ".github/pull_request_template.zh-CN.md")));

  const issueConfig = fs.readFileSync(path.join(root, ".github/ISSUE_TEMPLATE/config.yml"), "utf8");
  assert.ok(!issueConfig.includes("/discussions"));
  assert.ok(issueConfig.includes("Question / 使用问题"));

  for (const template of ["bug_report.yml", "bug_report.zh-CN.yml"]) {
    const source = fs.readFileSync(path.join(root, `.github/ISSUE_TEMPLATE/${template}`), "utf8");
    for (const marker of ["v0.8.0", "deployment", "component", "S3", "Webhook", "MCP"]) {
      assert.ok(source.includes(marker), `${template} is missing ${marker}`);
    }
    assert.ok(!source.includes("v1.0.0"), `${template} retains a future example version`);
    assert.ok(!source.includes("go1.24.0"), `${template} retains an obsolete Go example`);
  }

  for (const template of ["feature_request.yml", "feature_request.zh-CN.yml"]) {
    const source = fs.readFileSync(path.join(root, `.github/ISSUE_TEMPLATE/${template}`), "utf8");
    for (const marker of ["MCP / AI Agent", "Webhook", "Security", "Compatibility", "Migration"]) {
      assert.ok(source.includes(marker) || source.includes({ Security: "安全", Compatibility: "兼容性", Migration: "迁移" }[marker]),
        `${template} is missing ${marker}`);
    }
  }

  const pullRequest = fs.readFileSync(path.join(root, ".github/pull_request_template.md"), "utf8");
  for (const command of [
    "go test -race ./...",
    "go vet ./...",
    "bun build ./web/*.js --target=browser --outdir=./.bun-check",
    "bun test ./tests/web ./tests/docs",
  ]) {
    assert.ok(pullRequest.includes(command), `pull request template is missing ${command}`);
  }

  for (const locale of ["de", "fr", "it", "ja", "ko"]) {
    const contributing = fs.readFileSync(path.join(root, `.github/CONTRIBUTING.${locale}.md`), "utf8");
    const conduct = fs.readFileSync(path.join(root, `.github/CODE_OF_CONDUCT.${locale}.md`), "utf8");
    assert.notEqual(contributing, fs.readFileSync(path.join(root, ".github/CONTRIBUTING.md"), "utf8"));
    assert.notEqual(conduct, fs.readFileSync(path.join(root, ".github/CODE_OF_CONDUCT.md"), "utf8"));
    assert.ok(contributing.includes("CONTRIBUTING.md"), `${locale} contribution summary lacks canonical link`);
    assert.ok(conduct.includes("CODE_OF_CONDUCT.md"), `${locale} conduct summary lacks canonical link`);

    const security = fs.readFileSync(path.join(root, `SECURITY.${locale}.md`), "utf8");
    for (const marker of ["0.8.x", "0.7.x", "0.6.x", "srcdoc", 'referrerpolicy="no-referrer"', "CSP", "CID"]) {
      assert.ok(security.includes(marker), `SECURITY.${locale}.md is missing ${marker}`);
    }
  }
  for (const security of ["SECURITY.md", "SECURITY.zh-CN.md"]) {
    const markdown = fs.readFileSync(path.join(root, security), "utf8");
    for (const marker of ["0.8.x", "0.7.x", "0.6.x"]) {
      assert.ok(markdown.includes(marker), `${security} is missing ${marker}`);
    }
  }
});

test("release history and generated reports avoid stale documentation", () => {
  const changelog = fs.readFileSync(path.join(root, "CHANGELOG.md"), "utf8");
  assert.ok(changelog.includes("## [0.6.0] - 2026-09-01"));
  assert.ok(changelog.includes("## [0.5.0] - 2026-08-30"));
  for (const locale of ["en", "zh-CN"]) {
    const release = `docs/${locale}/Release-0.7.0.md`;
    assert.ok(fs.existsSync(path.join(root, release)), `missing ${release}`);
    assert.ok(fs.readFileSync(path.join(root, `docs/${locale}/README.md`), "utf8").includes("Release-0.7.0.md"));
  }

  for (const locale of ["en", "zh-CN", "de", "fr", "it", "ja", "ko"]) {
    assert.ok(fs.existsSync(path.join(root, `docs/${locale}/Comparison-and-Migration.md`)));
  }
  assert.deepEqual(
    walkMarkdown(path.join(root, "docs")).filter((file) => path.basename(file).includes("Full Feature & API")),
    [],
  );

  const report = fs.readFileSync(path.join(root, ".github/goreportcard-report.md"), "utf8");
  assert.doesNotMatch(report, /\bLine \d+:/, "Go Report Card keeps stale source line numbers");
  assert.ok(report.includes("exact analyzed commit"));
  const workflow = fs.readFileSync(path.join(root, ".github/workflows/go-reportcard.yml"), "utf8");
  assert.ok(workflow.includes("branches:"));
  assert.ok(workflow.includes("- main"));
  assert.ok(workflow.includes("'**/*.go'"));
  assert.ok(workflow.includes('report: "false"'));
  assert.ok(workflow.includes('commit: "false"'));
  assert.ok(workflow.includes('version: "v1.0.0"'));
  assert.ok(workflow.includes("git add .github/goreportcard.svg"));
  assert.ok(!workflow.includes("[skip ci]"));
});

test("0.8.0 examples are pinned, private by default, persistent, and bounded", () => {
  for (const readme of translatedReadmes) {
    const markdown = fs.readFileSync(path.join(root, readme), "utf8");
    const commands = shellCommands(markdown);
    const clone = commands.find((command) => command.startsWith("git clone "));
    assert.ok(clone, `${readme} has no source clone command`);
    assert.match(clone, /(?:^|\s)--branch\s+v0\.8\.0(?:\s|$)/, `${readme} does not pin the source tag`);
    assert.match(clone, /(?:^|\s)--depth\s+1(?:\s|$)/, `${readme} does not use a shallow release clone`);

    const installs = commands.filter((command) => command.startsWith("go install "));
    assert.deepEqual(
      installs,
      ["go install github.com/soulteary/owlmail/cmd/owlmail@v0.8.0"],
      `${readme} must contain exactly one release-pinned Go install`,
    );

    const dockerRuns = commands.filter((command) => command.startsWith("docker run "));
    assert.equal(dockerRuns.length, 2, `${readme} has an unexpected number of Docker run examples`);
    for (const [index, command] of dockerRuns.entries()) {
      assertPrivateDockerPorts(command, `${readme} docker run #${index + 1}`);
      assert.match(
        command,
        /(?:^|\s)-v\s+owlmail-data:\/app\/mail(?:\s|$)/,
        `${readme} docker run #${index + 1} omits mailbox persistence`,
      );
    }
  }

  for (const locale of ["en", "zh-CN"]) {
    const release = fs.readFileSync(path.join(root, `docs/${locale}/Release-0.8.0.md`), "utf8");
    const releaseCommands = shellCommands(release);
    const downloads = releaseCommands
      .filter((command) => command.startsWith("curl "))
      .map((command) => command.match(/\/([^/\s]+)$/)?.[1]);
    assert.deepEqual(downloads, [
      "owlmail-linux-amd64",
      "checksums.txt",
      "checksums.txt.sigstore.json",
    ]);
    const checksum = releaseCommands.find((command) => command.includes("sha256sum -c -"));
    assert.match(checksum || "", /grep ' owlmail-linux-amd64\$' checksums\.txt \| sha256sum -c -/);
    assert.ok(releaseCommands.includes("chmod +x owlmail-linux-amd64"));
    assert.ok(releaseCommands.includes("./owlmail-linux-amd64"));

    const operations = fs.readFileSync(path.join(root, `docs/${locale}/Operations.md`), "utf8");
    for (const [index, command] of shellCommands(operations).filter((item) => item.startsWith("docker run ")).entries()) {
      assertPrivateDockerPorts(command, `docs/${locale}/Operations.md docker run #${index + 1}`);
    }

    const ci = fs.readFileSync(path.join(root, `docs/${locale}/CI-Quickstart.md`), "utf8");
    const workflow = fencedBlocks(ci, ["yaml", "yml"])[0]?.body || "";
    const actionRefs = [...workflow.matchAll(/uses:\s+actions\/(checkout|setup-go|upload-artifact)@(\S+)/g)];
    assert.deepEqual(actionRefs.map((match) => match[1]).sort(), ["checkout", "setup-go", "upload-artifact"]);
    for (const [, action, ref] of actionRefs) {
      assert.match(ref, /^[0-9a-f]{40}$/, `actions/${action} is not pinned to a full commit SHA`);
    }
    assert.match(workflow, /for attempt in \$\(seq 1 30\); do/);
    assert.match(workflow, /curl .*--connect-timeout 2 --max-time 3/);
    assert.match(workflow, /OWLMAIL_RUN_INTEGRATION_TEST=1 go test \.\/examples\/testing\/go -v/);
    assert.doesNotMatch(workflow, /\.\/scripts\/test-integration/);

    const ai = fs.readFileSync(path.join(root, `docs/${locale}/AI-Agent-Testing.md`), "utf8");
    assert.ok(ai.includes("get_latest_email"));
  }

  const javascript = fs.readFileSync(path.join(root, "examples/testing/javascript/email-test.mjs"), "utf8");
  assert.ok(javascript.includes("socket.setTimeout"));
  assert.ok(javascript.includes("AbortController"));
  const goExample = fs.readFileSync(path.join(root, "examples/testing/go/email_test.go"), "utf8");
  assert.ok(goExample.includes("net.Dialer{Timeout:"));
  assert.ok(goExample.includes("SetDeadline"));
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
