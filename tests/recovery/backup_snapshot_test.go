package recovery

// These helpers only copy an explicitly stopped synthetic deployment. They are
// not an online-backup API: callers close listeners, workers and SQLite first.
import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type backupFile struct {
	SHA256 string `json:"sha256"`
	Mode   uint32 `json:"mode"`
	Size   int64  `json:"size"`
}

func backupInventory(root string) (map[string]backupFile, error) {
	result := map[string]backupFile{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == root {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("nonregular backup entry: %s", p)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		result[rel] = backupFile{hex.EncodeToString(sum[:]), uint32(info.Mode().Perm()), info.Size()}
		return nil
	})
	return result, err
}
func copyStoppedDeployment(t *testing.T, source, target string) {
	t.Helper()
	before, err := backupInventory(source)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	// Whole root includes control WAL/SHM if present, ingress and merge journals,
	// account hashes, TLS material and configuration. No migration transforms it.
	for rel, item := range before {
		p := filepath.Join(target, rel)
		if err = os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		b, e := os.ReadFile(filepath.Join(source, rel))
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(p, b, fs.FileMode(item.Mode)); e != nil {
			t.Fatal(e)
		}
	}
	after, err := backupInventory(source)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("source changed while stopped: %v", err)
	}
	copied, err := backupInventory(target)
	if err != nil || !reflect.DeepEqual(before, copied) {
		t.Fatalf("backup verification failed: %v", err)
	}
	manifest, err := json.Marshal(before)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(target+".manifest.json", manifest, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(manifest)
	t.Logf("verified stopped snapshot: %d files, manifest SHA256=%x", len(before), sum)
}
func assertBackupUnchanged(t *testing.T, root string, want map[string]backupFile) {
	t.Helper()
	got, err := backupInventory(root)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("immutable backup/source changed: %v", err)
	}
}

// Manifest lives beside, rather than inside, the hashed deployment root.
func verifyBackupManifest(root string) error {
	data, err := os.ReadFile(root + ".manifest.json")
	if err != nil {
		return err
	}
	var expected map[string]backupFile
	if err = json.Unmarshal(data, &expected); err != nil {
		return err
	}
	actual, err := backupInventory(root)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(expected, actual) {
		return fmt.Errorf("backup manifest mismatch")
	}
	return nil
}
