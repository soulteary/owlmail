const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");

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
        destination.startsWith("#") ||
        destination.startsWith("/") ||
        destination.startsWith("//") ||
        /^[a-z][a-z\d+.-]*:/i.test(destination)
      ) {
        continue;
      }

      destination = destination.split("#", 1)[0].split("?", 1)[0];
      if (destination !== "") destinations.push(decodeURIComponent(destination));
    }
  }

  return destinations;
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
    for (const match of section.matchAll(/(\w+)\s*:=\s*(\w+)\.Group\("([^"]*)"\)/g)) {
      const [, name, parent, segment] = match;
      assert.ok(prefixes.has(parent), `unknown API route group ${parent}`);
      prefixes.set(name, prefixes.get(parent) + segment);
    }
    for (const match of section.matchAll(/(\w+)\.(Get|Post|Put|Patch|Delete)\("([^"]*)"/g)) {
      const [, group, method, route] = match;
      assert.ok(prefixes.has(group), `unknown API route group ${group}`);
      routes.push(`${method.toUpperCase()} ${prefixes.get(group)}${route}`);
    }
  }

  return [...new Set(routes)].sort();
}

test("Markdown has balanced fences and valid local links", () => {
  const failures = [];

  for (const file of walkMarkdown(root)) {
    const relative = path.relative(root, file);
    const markdown = fs.readFileSync(file, "utf8");
    const { text, openFence } = withoutFencedCode(markdown);
    if (openFence !== null) failures.push(`${relative}: unclosed ${openFence} fence`);

    for (const destination of localDestinations(text)) {
      const resolved = path.resolve(path.dirname(file), destination);
      if (!fs.existsSync(resolved)) failures.push(`${relative}: missing ${destination}`);
    }
  }

  assert.deepEqual(failures, []);
});

test("every configuration flag is listed in each root README", () => {
  const configSource = fs.readFileSync(path.join(root, "internal/config/config.go"), "utf8");
  const flags = [...configSource.matchAll(/fs\.(?:Bool|Int|String)\("([^"]+)"/g)].map((match) => match[1]);
  assert.ok(flags.length > 0, "no configuration flags found");

  for (const readme of translatedReadmes) {
    const markdown = fs.readFileSync(path.join(root, readme), "utf8");
    const missing = flags.filter((flag) => !markdown.includes(`\`-${flag}\``));
    assert.deepEqual(missing, [], `${readme} is missing configuration flags`);
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
