package api

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOfflineRelayInspectionPreservesExpiredRecordsAndValidatesSources(t *testing.T) {
	dir := t.TempDir()
	store, err := newPersistentRelayJobStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	job, err := store.create("retained-source", "recipient@example.test")
	if err != nil {
		t.Fatal(err)
	}
	report, err := InspectRelayRecovery(dir)
	if err != nil || report.Pending != 1 || report.MissingSources != 1 {
		t.Fatal(report, err)
	}
	if err = os.WriteFile(filepath.Join(dir, "retained-source.eml"), []byte("Subject: test\r\n\r\nhello"), 0600); err != nil {
		t.Fatal(err)
	}
	report, err = InspectRelayRecovery(dir)
	if err != nil || report.MissingSources != 0 {
		t.Fatal(report, err)
	}
	store.complete(job.ID, nil)
	store.mu.Lock()
	expired := store.jobs[job.ID]
	old := time.Now().Add(-48 * time.Hour)
	expired.CompletedAt = &old
	err = store.persistLocked(expired)
	store.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(store.directory, job.ID+".json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	report, err = InspectRelayRecovery(dir)
	if err != nil || report.Completed != 1 {
		t.Fatal(report, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(before) != string(after) {
		t.Fatal("inspection changed expired record", err)
	}
	if err = os.WriteFile(path, append(before, []byte(` {"extra":true}`)...), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = InspectRelayRecovery(dir); err == nil {
		t.Fatal("accepted trailing JSON")
	}
	if _, err = newPersistentRelayJobStore(dir); err == nil {
		t.Fatal("normal recovery accepted trailing JSON")
	}
}
func TestOfflineRelayInspectionDoesNotCreateMetadata(t *testing.T) {
	dir := t.TempDir()
	report, err := InspectRelayRecovery(dir)
	if err != nil || report.Pending != 0 {
		t.Fatal(report, err)
	}
	if _, err = os.Stat(filepath.Join(dir, ".owlmail-meta")); !os.IsNotExist(err) {
		t.Fatal("created metadata", err)
	}
}
