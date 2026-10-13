const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const { test } = require("bun:test");

const root = path.resolve(__dirname, "../..");
const read = (name) => fs.readFileSync(path.join(root, name), "utf8");

test("the runtime image includes legal texts rather than only a license label", () => {
  const ignored = read(".dockerignore").split(/\r?\n/).map((line) => line.trim());
  assert.equal(ignored.includes("LICENSE"), false, "Docker context drops the project license");
  const dockerfile = read("Dockerfile");
  assert.match(dockerfile, /go run \.\/scripts\/license-bundle -output \/build\/licenses -platforms "\$\{TARGETOS\}\/\$\{TARGETARCH\}"/);
  assert.match(dockerfile, /^COPY --from=builder \/build\/licenses \/usr\/share\/licenses\/owlmail$/m);
});

test("release and snapshot downloads carry the original legal texts", () => {
  const build = read(".github/workflows/build.yml");
  assert.ok(build.includes("go run ./scripts/license-bundle -output dist/licenses"));
  assert.match(build, /name: owlmail-\$\{\{ github\.sha \}\}\s+path: dist\//);

  const release = read(".github/workflows/release.yml");
  const bundle = release.indexOf("name: Bundle release licenses and upstream notices");
  const checksums = release.indexOf("name: Create checksums");
  assert.ok(bundle >= 0 && checksums > bundle, "license archive must exist before signed checksums");
  assert.ok(release.includes("tar -czf owlmail-licenses.tar.gz"));
  assert.ok(release.includes("sha256sum owlmail-* > checksums.txt"), "license archive is omitted from checksum inputs");
  assert.match(release, /files: \|\s+owlmail-\*/);
});
