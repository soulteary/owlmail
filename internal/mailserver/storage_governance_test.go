package mailserver

import (
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
	if err := server.ConfigureStoragePolicy(StoragePolicy{
		MaxDiskBytes: int64(len(governanceTestMessage) + 1), CleanupInterval: time.Hour,
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
	if usage > int64(len(governanceTestMessage)+1) {
		t.Fatalf("disk usage = %d, exceeds limit", usage)
	}
}
