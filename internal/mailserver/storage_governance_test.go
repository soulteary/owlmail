package mailserver

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const governanceTestMessage = "From: sender@example.test\r\nTo: receiver@example.test\r\nSubject: persisted\r\n\r\nbody\r\n"

func TestReadStatePersistsAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "persisted.eml"), []byte(governanceTestMessage), 0644); err != nil {
		t.Fatal(err)
	}
	server, err := NewMailServer(1025, "localhost", dir)
	if err != nil {
		t.Fatal(err)
	}
	email, err := server.GetEmail("persisted")
	if err != nil || email.Read {
		t.Fatalf("legacy restored email = %#v, %v; want unread", email, err)
	}
	if err := server.ReadEmail("persisted"); err != nil {
		t.Fatal(err)
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}

	restarted, err := NewMailServer(1025, "localhost", dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = restarted.Close() }()
	email, err = restarted.GetEmail("persisted")
	if err != nil || !email.Read {
		t.Fatalf("restarted email = %#v, %v; want read", email, err)
	}
	entries, err := os.ReadDir(filepath.Join(dir, metadataDirectoryName))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".tmp" {
			t.Fatalf("temporary metadata was left behind: %s", entry.Name())
		}
	}
}

func TestStoragePolicyEnforcesOldestMessageCount(t *testing.T) {
	server, err := NewMailServer(1025, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	for i, id := range []string{"oldest", "middle", "newest"} {
		if err := os.WriteFile(filepath.Join(server.mailDir, id+".eml"), []byte(governanceTestMessage), 0644); err != nil {
			t.Fatal(err)
		}
		if err := server.SaveEmailToStore(id, false, &Envelope{To: []string{"receiver@example.test"}}, &Email{Subject: id}); err != nil {
			t.Fatal(err)
		}
		if i < 2 {
			time.Sleep(2 * time.Millisecond)
		}
	}
	if err := server.ConfigureStoragePolicy(StoragePolicy{MaxMessages: 2, CleanupInterval: time.Hour}); err != nil {
		t.Fatal(err)
	}
	if _, err := server.GetEmail("oldest"); err == nil {
		t.Fatal("oldest message was not evicted")
	}
	if got := len(server.GetAllEmail()); got != 2 {
		t.Fatalf("message count = %d, want 2", got)
	}
	stats := server.GetEmailStats()["storage"].(map[string]interface{})
	if stats["deletedMessages"].(uint64) != 1 {
		t.Fatalf("storage metrics = %#v", stats)
	}
}

func TestStoragePolicyUsesArrivalOrderInsteadOfMessageDate(t *testing.T) {
	server, err := NewMailServer(1025, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	for i, test := range []struct {
		id      string
		message time.Time
	}{
		{id: "arrived-first", message: time.Now().Add(24 * time.Hour)},
		{id: "arrived-last", message: time.Now().Add(-24 * time.Hour)},
	} {
		if err := os.WriteFile(filepath.Join(server.mailDir, test.id+".eml"), []byte(governanceTestMessage), 0644); err != nil {
			t.Fatal(err)
		}
		if err := server.SaveEmailToStore(test.id, false, &Envelope{To: []string{"receiver@example.test"}}, &Email{Subject: test.id, Time: test.message}); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			time.Sleep(2 * time.Millisecond)
		}
	}

	if err := server.ConfigureStoragePolicy(StoragePolicy{MaxMessages: 1, CleanupInterval: time.Hour}); err != nil {
		t.Fatal(err)
	}
	if _, err := server.GetEmail("arrived-first"); err == nil {
		t.Fatal("first-arriving message was not evicted")
	}
	if _, err := server.GetEmail("arrived-last"); err != nil {
		t.Fatalf("last-arriving message was evicted: %v", err)
	}
}

func TestStoragePolicyUsesArrivalTimeForAge(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "old.eml")
	if err := os.WriteFile(path, []byte(governanceTestMessage), 0644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-72 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	server, err := NewMailServer(1025, "localhost", dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	if err := server.ConfigureStoragePolicy(StoragePolicy{MaxAge: 24 * time.Hour, CleanupInterval: time.Hour}); err != nil {
		t.Fatal(err)
	}
	if got := len(server.GetAllEmail()); got != 0 {
		t.Fatalf("message count = %d, want 0", got)
	}
}

func TestStoragePolicyEnforcesDiskLimit(t *testing.T) {
	server, err := NewMailServer(1025, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	for _, id := range []string{"disk-old", "disk-new"} {
		if err := os.WriteFile(filepath.Join(server.mailDir, id+".eml"), []byte(governanceTestMessage), 0644); err != nil {
			t.Fatal(err)
		}
		if err := server.SaveEmailToStore(id, false, &Envelope{To: []string{"receiver@example.test"}}, &Email{Subject: id}); err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Millisecond)
	}
	newestUsage, err := server.emailDiskUsage("disk-new")
	if err != nil {
		t.Fatal(err)
	}
	if err := server.ConfigureStoragePolicy(StoragePolicy{
		MaxDiskBytes: newestUsage, CleanupInterval: time.Hour,
	}); err != nil {
		t.Fatal(err)
	}
	if got := len(server.GetAllEmail()); got != 1 {
		t.Fatalf("message count after disk cleanup = %d, want 1", got)
	}
	usage, err := server.mailboxDiskUsage()
	if err != nil {
		t.Fatal(err)
	}
	if usage > newestUsage {
		t.Fatalf("disk usage = %d, exceeds limit", usage)
	}
}

func TestEmailDiskUsageIncludesMetadata(t *testing.T) {
	server, err := NewMailServer(1025, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	id := "with-metadata"
	emlPath := filepath.Join(server.mailDir, id+".eml")
	if err := os.WriteFile(emlPath, []byte(governanceTestMessage), 0644); err != nil {
		t.Fatal(err)
	}
	if err := server.SaveEmailToStore(id, false, &Envelope{To: []string{"receiver@example.test"}}, &Email{Subject: id}); err != nil {
		t.Fatal(err)
	}
	emlStat, err := os.Stat(emlPath)
	if err != nil {
		t.Fatal(err)
	}
	metadataStat, err := os.Stat(server.metadataPath(id))
	if err != nil {
		t.Fatal(err)
	}
	usage, err := server.emailDiskUsage(id)
	if err != nil {
		t.Fatal(err)
	}
	if want := emlStat.Size() + metadataStat.Size(); usage != want {
		t.Fatalf("disk usage = %d, want eml + metadata = %d", usage, want)
	}
}

func TestSaveEmailMetadataFailureDoesNotPublishEmail(t *testing.T) {
	server, err := NewMailServer(1025, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	metadataPath := filepath.Join(server.mailDir, metadataDirectoryName)
	if err := os.WriteFile(metadataPath, []byte("blocks metadata directory"), 0600); err != nil {
		t.Fatal(err)
	}
	err = server.SaveEmailToStore("unpublished", false, &Envelope{To: []string{"receiver@example.test"}}, &Email{Subject: "unpublished"})
	if err == nil {
		t.Fatal("SaveEmailToStore succeeded despite metadata failure")
	}
	if _, err := server.GetEmail("unpublished"); err == nil {
		t.Fatal("email became visible despite metadata failure")
	}
}

func TestRecoveryKeepsValidEmailWhenMetadataCannotBeWritten(t *testing.T) {
	dir := t.TempDir()
	id := "valid-recovery"
	if err := os.WriteFile(filepath.Join(dir, id+".eml"), []byte(governanceTestMessage), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, metadataDirectoryName), []byte("blocks metadata directory"), 0600); err != nil {
		t.Fatal(err)
	}
	server, err := NewMailServer(1025, "localhost", dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	if _, err := server.GetEmail(id); err != nil {
		t.Fatalf("valid email was not restored: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, id+".eml")); err != nil {
		t.Fatalf("valid EML was quarantined: %v", err)
	}
}

func TestCleanupDoesNotAccountForFailedDeletion(t *testing.T) {
	server, err := NewMailServer(1025, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	id := "undeletable"
	if err := os.WriteFile(filepath.Join(server.mailDir, id+".eml"), []byte(governanceTestMessage), 0644); err != nil {
		t.Fatal(err)
	}
	if err := server.SaveEmailToStore(id, false, &Envelope{To: []string{"receiver@example.test"}}, &Email{Subject: id}); err != nil {
		t.Fatal(err)
	}
	server.storeMutex.Lock()
	server.receivedAtByID[id] = time.Now().Add(-time.Hour)
	server.storeMutex.Unlock()
	server.storagePolicy = StoragePolicy{MaxAge: time.Nanosecond}
	server.beforeEmailDelete = func(string) error { return errors.New("injected delete failure") }
	if err := server.CleanupStorage(); err == nil {
		t.Fatal("cleanup succeeded despite deletion failure")
	}
	if _, err := server.GetEmail(id); err != nil {
		t.Fatalf("failed deletion removed email from memory: %v", err)
	}
	stats := server.GetEmailStats()["storage"].(map[string]interface{})
	if stats["deletedMessages"].(uint64) != 0 || stats["reclaimedBytes"].(uint64) != 0 {
		t.Fatalf("failed deletion was counted as reclaimed: %#v", stats)
	}
}

func TestEmailSourceLeaseProtectsExplicitAndRetentionDeletion(t *testing.T) {
	server, err := NewMailServer(1025, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	id := "relay-source"
	if err := os.WriteFile(filepath.Join(server.mailDir, id+".eml"), []byte(governanceTestMessage), 0644); err != nil {
		t.Fatal(err)
	}
	if err := server.SaveEmailToStore(id, false, &Envelope{To: []string{"receiver@example.test"}}, &Email{Subject: id}); err != nil {
		t.Fatal(err)
	}

	email, release, err := server.AcquireEmailSource(id)
	if err != nil || email.ID != id {
		t.Fatalf("AcquireEmailSource() = %#v, %v", email, err)
	}
	if err := server.DeleteEmail(id); !errors.Is(err, ErrEmailSourceInUse) {
		t.Fatalf("DeleteEmail() error = %v, want ErrEmailSourceInUse", err)
	}
	if err := server.DeleteAllEmail(); !errors.Is(err, ErrEmailSourceInUse) {
		t.Fatalf("DeleteAllEmail() error = %v, want ErrEmailSourceInUse", err)
	}
	server.storeMutex.Lock()
	server.receivedAtByID[id] = time.Now().Add(-time.Hour)
	server.storeMutex.Unlock()
	server.storagePolicy = StoragePolicy{MaxAge: time.Nanosecond}
	otherID := "other-expired"
	if err := os.WriteFile(filepath.Join(server.mailDir, otherID+".eml"), []byte(governanceTestMessage), 0644); err != nil {
		t.Fatal(err)
	}
	if err := server.SaveEmailToStore(otherID, false, &Envelope{To: []string{"receiver@example.test"}}, &Email{Subject: otherID}); err != nil {
		t.Fatal(err)
	}
	server.storeMutex.Lock()
	server.receivedAtByID[otherID] = time.Now().Add(-time.Hour)
	server.storeMutex.Unlock()
	if err := server.CleanupStorage(); err != nil {
		t.Fatalf("CleanupStorage() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(server.mailDir, id+".eml")); err != nil {
		t.Fatalf("leased source was removed: %v", err)
	}
	if _, err := server.GetEmail(otherID); err == nil {
		t.Fatal("cleanup stopped before deleting an unrelated expired email")
	}

	release()
	release()
	if err := server.DeleteEmail(id); err != nil {
		t.Fatalf("DeleteEmail() after release: %v", err)
	}
}

func TestReadAllEmailReportsMetadataFailure(t *testing.T) {
	server, err := NewMailServer(1025, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	id := "unread"
	if err := os.WriteFile(filepath.Join(server.mailDir, id+".eml"), []byte(governanceTestMessage), 0644); err != nil {
		t.Fatal(err)
	}
	if err := server.SaveEmailToStore(id, false, &Envelope{To: []string{"receiver@example.test"}}, &Email{Subject: id}); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(server.mailDir, metadataDirectoryName)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(server.mailDir, metadataDirectoryName), []byte("blocks metadata directory"), 0600); err != nil {
		t.Fatal(err)
	}
	count, err := server.ReadAllEmail()
	if err == nil || count != 0 {
		t.Fatalf("ReadAllEmail() = %d, %v; want persisted failure", count, err)
	}
	email, getErr := server.GetEmail(id)
	if getErr != nil || email.Read {
		t.Fatalf("failed read update became visible: %#v, %v", email, getErr)
	}
}
