package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFixture(t *testing.T, root, name, text string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func licenseFixture(t *testing.T) (string, string, string, []pkg) {
	t.Helper()
	root := t.TempDir()
	goRoot := t.TempDir()
	moduleRoot := t.TempDir()
	writeFixture(t, root, "LICENSE", "Project copyright and full permission notice\n")
	writeFixture(t, goRoot, "LICENSE", "Go copyright and full conditions\n")
	writeFixture(t, goRoot, "PATENTS", "Go patent grant\n")
	writeFixture(t, moduleRoot, "runtime/deep/file.go", "package deep\n")
	m := &module{Path: "example.org/runtime", Version: "v1.2.3", Dir: moduleRoot}
	return root, goRoot, moduleRoot, []pkg{{Dir: filepath.Join(moduleRoot, "runtime/deep"), Module: m}}
}

func TestCollectPreservesUpstreamTextsAndPackageSpecificLicenses(t *testing.T) {
	root, goRoot, moduleRoot, packages := licenseFixture(t)
	texts := map[string]string{
		"LICENSE":                   "MIT plus Apache transition; all copyright notices\n",
		"NOTICE.txt":                "Upstream attribution notice\n",
		"LICENSE-3RD-PARTY.md":      "Inherited third-party notices, unmodified\n",
		"runtime/LICENSE":           "Nested component license\n",
		"runtime/deep/COPYING.txt":  "Additional linked-package conditions\n",
		"testdata/LICENSE":          "Test-only license should not be selected\n",
		"runtime/copyright_test.go": "package runtime\n",
	}
	for name, text := range texts {
		writeFixture(t, moduleRoot, name, text)
	}
	files, modules, err := collect(root, goRoot, packages)
	if err != nil {
		t.Fatal(err)
	}
	if len(modules) != 1 || modules[0] != "example.org/runtime@v1.2.3" {
		t.Fatalf("wrong exact module identity: %v", modules)
	}
	for _, name := range []string{"LICENSE", "NOTICE.txt", "LICENSE-3RD-PARTY.md", "runtime/LICENSE", "runtime/deep/COPYING.txt"} {
		if got := string(files["modules/"+modules[0]+"/"+name]); got != texts[name] {
			t.Errorf("%s was not copied verbatim: %q", name, got)
		}
	}
	for name := range files {
		if strings.Contains(name, "testdata") || strings.HasSuffix(name, ".go") {
			t.Errorf("unlinked or code file included: %s", name)
		}
	}
	if string(files["LICENSE"]) != "Project copyright and full permission notice\n" || string(files["go/PATENTS"]) != "Go patent grant\n" {
		t.Fatal("project or toolchain legal text changed or missing")
	}
}

func TestCollectFailsWithoutDependencyLicense(t *testing.T) {
	root, goRoot, moduleRoot, packages := licenseFixture(t)
	writeFixture(t, moduleRoot, "NOTICE", "Attribution is not a license\n")
	if _, _, err := collect(root, goRoot, packages); err == nil || !strings.Contains(err.Error(), "no LICENSE or COPYING") {
		t.Fatalf("missing dependency license did not fail closed: %v", err)
	}
}

func TestCollectFailsWithoutToolchainPatentGrant(t *testing.T) {
	root, goRoot, moduleRoot, packages := licenseFixture(t)
	writeFixture(t, moduleRoot, "LICENSE", "A complete license\n")
	if err := os.Remove(filepath.Join(goRoot, "PATENTS")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := collect(root, goRoot, packages); err == nil || !strings.Contains(err.Error(), "PATENTS") {
		t.Fatalf("missing toolchain patent grant did not fail closed: %v", err)
	}
}

func TestCollectRejectsEmptyLicense(t *testing.T) {
	root, goRoot, moduleRoot, packages := licenseFixture(t)
	writeFixture(t, moduleRoot, "LICENSE", " \n")
	if _, _, err := collect(root, goRoot, packages); err == nil || !strings.Contains(err.Error(), "is empty") {
		t.Fatalf("empty dependency license did not fail closed: %v", err)
	}
}

func TestCollectRejectsUnversionedReplacement(t *testing.T) {
	root, goRoot, moduleRoot, packages := licenseFixture(t)
	packages[0].Module.Replace = &module{Path: "../local", Dir: moduleRoot}
	if _, _, err := collect(root, goRoot, packages); err == nil || !strings.Contains(err.Error(), "versioned module") {
		t.Fatalf("unversioned replacement did not fail closed: %v", err)
	}
}

func TestCollectDoesNotTreatASiblingLicenseAsCoverage(t *testing.T) {
	root, goRoot, moduleRoot, packages := licenseFixture(t)
	writeFixture(t, moduleRoot, "sibling/LICENSE", "Only covers this sibling\n")
	packages = append(packages, pkg{Dir: filepath.Join(moduleRoot, "sibling"), Module: packages[0].Module})
	if _, _, err := collect(root, goRoot, packages); err == nil || !strings.Contains(err.Error(), "no LICENSE or COPYING") {
		t.Fatalf("unlicensed package was masked by a sibling license: %v", err)
	}
}

func TestCollectPreservesLicensesBundledInsideTheToolchain(t *testing.T) {
	root, goRoot, moduleRoot, packages := licenseFixture(t)
	writeFixture(t, moduleRoot, "LICENSE", "Runtime module license\n")
	writeFixture(t, goRoot, "src/vendor/example.org/crypto/LICENSE", "Vendored crypto license\n")
	writeFixture(t, goRoot, "src/vendor/example.org/crypto/PATENTS", "Vendored crypto patent grant\n")
	writeFixture(t, goRoot, "src/vendor/example.org/crypto/sub/file.go", "package sub\n")
	writeFixture(t, goRoot, "src/crypto/internal/boring/LICENSE", "Boring component license\n")
	writeFixture(t, goRoot, "src/runtime/testdata/LICENSE", "Test-only toolchain license\n")
	packages = append(packages,
		pkg{Dir: filepath.Join(goRoot, "src/vendor/example.org/crypto/sub"), Standard: true},
		pkg{Dir: filepath.Join(goRoot, "src/crypto/internal/boring"), Standard: true},
	)
	files, _, err := collect(root, goRoot, packages)
	if err != nil {
		t.Fatal(err)
	}
	for name, expected := range map[string]string{
		"go/src/vendor/example.org/crypto/LICENSE": "Vendored crypto license\n",
		"go/src/vendor/example.org/crypto/PATENTS": "Vendored crypto patent grant\n",
		"go/src/crypto/internal/boring/LICENSE":    "Boring component license\n",
	} {
		if string(files[name]) != expected {
			t.Errorf("toolchain package legal text %s is missing or changed", name)
		}
	}
	if _, exists := files["go/src/runtime/testdata/LICENSE"]; exists {
		t.Fatal("unlinked toolchain testdata was collected")
	}
}
