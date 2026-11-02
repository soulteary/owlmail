//go:build phorge_integration

package gorgeintegration

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const phorgeCommit = "e2eab00eb21ee9ff04b66b27ea835673f3e6027b"

func TestPinnedPhorgeAdapter(t *testing.T) {
	source := os.Getenv("OWLMAIL_PHORGE_SOURCE")
	if !filepath.IsAbs(source) {
		t.Fatal("OWLMAIL_PHORGE_SOURCE must name the pinned clean Phorge checkout; this integration never skips")
	}
	requirePhorgeSource(t, source)
	t.Cleanup(func() {
		requirePhorgeSource(t, source)
		t.Log("pinned Phorge source remains clean after adapter execution")
	})
	php, err := exec.LookPath("php")
	if err != nil {
		t.Fatal("PHP CLI with curl and mbstring is required: ", err)
	}
	phpFlags := isolatedPHPFlags(t, php)
	script, err := filepath.Abs("phorge-adapter.php")
	if err != nil {
		t.Fatal(err)
	}
	runPinnedGorgeSMTP(t, func(t *testing.T, endpoint string, payload map[string]any, rejected bool) {
		t.Helper()
		input, err := json.Marshal(map[string]any{"uri": endpoint, "token": serviceToken, "message": payload["message"]})
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, php, append(append([]string{}, phpFlags...), "-d", "date.timezone=UTC", script)...)
		cmd.Dir = t.TempDir()
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "TMPDIR=" + cmd.Dir, "OWLMAIL_PHORGE_SOURCE=" + source}
		cmd.Stdin = bytes.NewReader(input)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("real PHP adapter failed: %v\nstdout: %s\nstderr: %s", err, output, stderr.Bytes())
		}
		var result struct {
			Classification, Adapter, PHPVersion string
			Classes                             map[string]string
		}
		decode(t, output, &result)
		expected := "accepted"
		if rejected {
			expected = "permanent"
		}
		if result.Classification != expected || result.Adapter != "PhabricatorMailGorgeAdapter" || len(result.Classes) != 5 || result.PHPVersion == "" {
			t.Fatalf("real PHP adapter receipt mismatch: %s", output)
		}
		t.Logf("unchanged Phorge %s / PHP %s adapter: %s; original class origins %v", phorgeCommit, result.PHPVersion, result.Classification, result.Classes)
	})
}

func requirePhorgeSource(t *testing.T, source string) {
	t.Helper()
	for _, command := range [][]string{{"rev-parse", "HEAD"}, {"status", "--porcelain", "--untracked-files=all", "--ignored=matching"}} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		cmd := exec.CommandContext(ctx, "git", append([]string{"-C", source}, command...)...)
		cmd.Env = []string{"PATH=" + os.Getenv("PATH")}
		output, err := cmd.Output()
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		expected := ""
		if command[0] == "rev-parse" {
			expected = phorgeCommit
		}
		if strings.TrimSpace(string(output)) != expected {
			t.Fatalf("Phorge must be the clean fixed source %s: %s", phorgeCommit, output)
		}
	}
}

// Ignore php.ini entirely. Load only missing required shared extensions from
// PHP's compiled-in extension directory; statically built modules need no flag.
func isolatedPHPFlags(t *testing.T, php string) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	probe := exec.CommandContext(ctx, php, "-n", "-r", `echo json_encode(array("directory" => ini_get("extension_dir"), "loaded" => get_loaded_extensions()), JSON_THROW_ON_ERROR);`)
	probe.Env = []string{"PATH=" + os.Getenv("PATH")}
	raw, err := probe.Output()
	if err != nil {
		t.Fatalf("isolated PHP extension probe: %v", err)
	}
	var result struct {
		Directory string
		Loaded    []string
	}
	decode(t, raw, &result)
	flags := []string{"-n"}
	for _, name := range []string{"curl", "mbstring"} {
		loaded := false
		for _, module := range result.Loaded {
			if module == name {
				loaded = true
				break
			}
		}
		if loaded {
			continue
		}
		path := filepath.Join(result.Directory, name+".so")
		if !filepath.IsAbs(path) {
			t.Fatalf("PHP extension directory must be absolute: %s", path)
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			t.Fatalf("required PHP extension %s unavailable at %s: %v", name, path, err)
		}
		flags = append(flags, "-d", "extension="+path)
	}
	return flags
}
