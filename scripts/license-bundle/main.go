// Command license-bundle collects upstream legal texts for OwlMail distributions.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const releasePlatforms = "linux/amd64,linux/arm64,darwin/amd64,darwin/arm64,windows/amd64"

const britishLicenseName = "LICENCE" //nolint:misspell // Preserve the upstream British filename spelling when collecting legal texts.

type module struct {
	Path, Version, Dir string
	Main               bool
	Replace            *module
}

type pkg struct {
	Dir      string
	Standard bool
	Module   *module
}

type legalFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type manifest struct {
	GoVersion string      `json:"go_version"`
	Platforms []string    `json:"platforms"`
	Modules   []string    `json:"modules"`
	Files     []legalFile `json:"files"`
}

func main() {
	output := flag.String("output", "dist/licenses", "new output directory")
	platforms := flag.String("platforms", releasePlatforms, "comma-separated GOOS/GOARCH targets")
	flag.Parse()
	if err := run(*output, *platforms); err != nil {
		fmt.Fprintln(os.Stderr, "license-bundle:", err)
		os.Exit(1)
	}
}

func run(output, platformList string) error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	goRoot, err := goOutput(nil, "env", "GOROOT")
	if err != nil {
		return err
	}
	goVersion, err := goOutput(nil, "env", "GOVERSION")
	if err != nil {
		return err
	}
	platforms := strings.Split(platformList, ",")
	var packages []pkg
	for _, platform := range platforms {
		target := strings.Split(platform, "/")
		if len(target) != 2 || target[0] == "" || target[1] == "" {
			return fmt.Errorf("invalid platform %q", platform)
		}
		data, err := goOutput([]string{"GOOS=" + target[0], "GOARCH=" + target[1], "CGO_ENABLED=0"}, "list", "-deps", "-json", "./cmd/owlmail")
		if err != nil {
			return fmt.Errorf("discover %s dependencies: %w", platform, err)
		}
		decoder := json.NewDecoder(strings.NewReader(data))
		for {
			var p pkg
			if err := decoder.Decode(&p); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				return fmt.Errorf("decode %s dependencies: %w", platform, err)
			}
			packages = append(packages, p)
		}
	}
	files, modules, err := collect(root, strings.TrimSpace(goRoot), packages)
	if err != nil {
		return err
	}
	sort.Strings(platforms)
	m := manifest{GoVersion: strings.TrimSpace(goVersion), Platforms: platforms, Modules: modules}
	paths := sortedKeys(files)
	var notices bytes.Buffer
	notices.WriteString("OwlMail third-party legal texts\n\n")
	notices.WriteString("Collected from exact runtime dependency versions for the targets in manifest.json.\n")
	notices.WriteString("Upstream root notices are preserved in full and may also describe optional or test material.\n")
	notices.WriteString("This bundle covers OwlMail and its Go dependencies; container base-image packages retain their own terms.\n\n")
	for _, path := range paths {
		sum := sha256.Sum256(files[path])
		m.Files = append(m.Files, legalFile{Path: path, SHA256: hex.EncodeToString(sum[:])})
		if path == "LICENSE" {
			continue
		}
		fmt.Fprintf(&notices, "===== %s =====\n\n", path)
		notices.Write(files[path])
		notices.WriteString("\n\n")
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	files["manifest.json"] = append(data, '\n')
	files["THIRD_PARTY_NOTICES.txt"] = notices.Bytes()
	if entries, err := os.ReadDir(output); err == nil && len(entries) != 0 {
		return fmt.Errorf("output directory %s is not empty; remove it before regenerating", output)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, path := range sortedKeys(files) {
		name := filepath.Join(output, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(name, files[path], 0o644); err != nil {
			return err
		}
	}
	fmt.Printf("License bundle: %d runtime modules, %d original legal texts -> %s\n", len(modules), len(paths), output)
	return nil
}

func goOutput(overrides []string, args ...string) (string, error) {
	cmd := exec.Command("go", args...)
	cmd.Env = append(os.Environ(), overrides...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	data, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("go %s: %w\n%s", strings.Join(args, " "), err, stderr.String())
	}
	return string(data), nil
}

func collect(root, goRoot string, packages []pkg) (map[string][]byte, []string, error) {
	files := make(map[string][]byte)
	copyRequired := func(source, name string) error {
		data, err := os.ReadFile(source)
		if err != nil {
			return fmt.Errorf("required legal text %s: %w", source, err)
		}
		if len(bytes.TrimSpace(data)) == 0 {
			return fmt.Errorf("required legal text %s is empty", source)
		}
		files[name] = data
		return nil
	}
	if err := copyRequired(filepath.Join(root, "LICENSE"), "LICENSE"); err != nil {
		return nil, nil, err
	}
	for _, name := range []string{"LICENSE", "PATENTS"} {
		if err := copyRequired(filepath.Join(goRoot, name), "go/"+name); err != nil {
			return nil, nil, err
		}
	}
	modules := make(map[string]*module)
	dirs := make(map[string]map[string]bool)
	packageDirs := make(map[string]map[string]bool)
	standardDirs := make(map[string]bool)
	for _, p := range packages {
		if p.Module == nil {
			if !p.Standard {
				return nil, nil, fmt.Errorf("package %s has no module or standard-library provenance", p.Dir)
			}
			for current := p.Dir; ; current = filepath.Dir(current) {
				rel, err := filepath.Rel(goRoot, current)
				if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
					return nil, nil, fmt.Errorf("standard package %s is outside GOROOT %s", p.Dir, goRoot)
				}
				standardDirs[current] = true
				if current == goRoot {
					break
				}
			}
			continue
		}
		if p.Module.Main {
			continue
		}
		m := p.Module
		if m.Replace != nil {
			m = m.Replace
		}
		if m.Dir == "" || m.Path == "" || m.Version == "" {
			return nil, nil, fmt.Errorf("dependency %s needs a versioned module and source directory", m.Path)
		}
		id := m.Path + "@" + m.Version
		modules[id] = m
		if dirs[id] == nil {
			dirs[id] = make(map[string]bool)
			packageDirs[id] = make(map[string]bool)
		}
		packageDirs[id][p.Dir] = true
		current := p.Dir
		for {
			rel, err := filepath.Rel(m.Dir, current)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return nil, nil, fmt.Errorf("package %s is outside module %s", p.Dir, m.Dir)
			}
			dirs[id][current] = true
			if current == m.Dir {
				break
			}
			current = filepath.Dir(current)
		}
	}
	if len(modules) == 0 {
		return nil, nil, errors.New("no runtime dependency modules discovered")
	}
	for dir := range standardDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, nil, err
		}
		for _, entry := range entries {
			if !entry.Type().IsRegular() || !isLegalText(entry.Name()) {
				continue
			}
			source := filepath.Join(dir, entry.Name())
			rel, _ := filepath.Rel(goRoot, source)
			if err := copyRequired(source, "go/"+filepath.ToSlash(rel)); err != nil {
				return nil, nil, err
			}
		}
	}
	ids := make([]string, 0, len(modules))
	for id, m := range modules {
		ids = append(ids, id)
		licensedDirs := make(map[string]bool)
		for dir := range dirs[id] {
			entries, err := os.ReadDir(dir)
			if err != nil {
				return nil, nil, err
			}
			for _, entry := range entries {
				if !entry.Type().IsRegular() || !isLegalText(entry.Name()) {
					continue
				}
				if isLicense(entry.Name()) {
					licensedDirs[dir] = true
				}
				source := filepath.Join(dir, entry.Name())
				rel, _ := filepath.Rel(m.Dir, source)
				if err := copyRequired(source, "modules/"+id+"/"+filepath.ToSlash(rel)); err != nil {
					return nil, nil, err
				}
			}
		}
		for dir := range packageDirs[id] {
			covered := false
			for current := dir; ; current = filepath.Dir(current) {
				if licensedDirs[current] {
					covered = true
					break
				}
				if current == m.Dir {
					break
				}
			}
			if !covered {
				return nil, nil, fmt.Errorf("no LICENSE or COPYING text found for runtime package %s in %s", dir, id)
			}
		}
	}
	sort.Strings(ids)
	return files, ids, nil
}

func isLicense(name string) bool {
	upper := strings.ToUpper(name)
	return hasLegalPrefix(upper, "LICENSE") || hasLegalPrefix(upper, britishLicenseName) || hasLegalPrefix(upper, "COPYING")
}

func isLegalText(name string) bool {
	upper := strings.ToUpper(name)
	// License-check scripts and copyright tests are code, not legal texts.
	switch strings.ToLower(filepath.Ext(name)) {
	case ".go", ".c", ".h", ".js", ".py", ".sh", ".yml", ".yaml":
		return false
	}
	for _, prefix := range []string{"LICENSE", britishLicenseName, "COPYING", "NOTICE", "COPYRIGHT", "AUTHORS", "PATENTS"} {
		if hasLegalPrefix(upper, prefix) {
			return true
		}
	}
	return false
}

func hasLegalPrefix(name, prefix string) bool {
	return name == prefix || strings.HasPrefix(name, prefix+".") || strings.HasPrefix(name, prefix+"-") || strings.HasPrefix(name, prefix+"_")
}

func sortedKeys(files map[string][]byte) []string {
	keys := make([]string, 0, len(files))
	for path := range files {
		keys = append(keys, path)
	}
	sort.Strings(keys)
	return keys
}
