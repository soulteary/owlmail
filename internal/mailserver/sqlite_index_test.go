package mailserver

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/types"
)

func TestSQLiteMailboxIndexQueryAndRebuild(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mailbox.db")
	index, err := NewSQLiteMailboxIndex(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := index.Close(); err != nil {
			t.Errorf("close SQLite index: %v", err)
		}
	}()
	now := time.Now().UTC()
	records := []IndexedEmail{
		{ID: "one", MessageTime: now.Add(-time.Hour), ReceivedAt: now, SubjectSearch: "alpha", TextSearch: "first body", FromSearch: "alice@example.test", VisibleRecipientsSearch: "team@example.test", BCCAddressesSearch: "secret@example.test", FirstFrom: "alice@example.test", StorePosition: 0},
		{ID: "two", MessageTime: now, ReceivedAt: now, Read: true, SubjectSearch: "beta", TextSearch: "second body", FromSearch: "bob@example.test", VisibleRecipientsSearch: "team@example.test", FirstFrom: "bob@example.test", StorePosition: 1},
	}
	if err := index.Rebuild(records); err != nil {
		t.Fatal(err)
	}
	unread := false
	results, total, err := index.Query(EmailQuery{Text: "alpha", Read: &unread, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(results) != 1 || results[0].ID != "one" {
		t.Fatalf("query = %#v, total %d", results, total)
	}
	results, total, err = index.Query(EmailQuery{Text: "secret@example.test", SearchAddresses: true, Limit: 10})
	if err != nil || total != 0 || len(results) != 0 {
		t.Fatalf("free-text search exposed BCC = %#v, total %d, err %v", results, total, err)
	}
	results, total, err = index.Query(EmailQuery{To: "secret@example.test", Limit: 10})
	if err != nil || total != 1 || len(results) != 1 || results[0].ID != "one" {
		t.Fatalf("BCC address query = %#v, total %d, err %v", results, total, err)
	}
	results, total, err = index.Query(EmailQuery{To: "hidden person", Limit: 10})
	if err != nil || total != 0 || len(results) != 0 {
		t.Fatalf("BCC display name query = %#v, total %d, err %v", results, total, err)
	}
	if err := index.Rebuild(records[1:]); err != nil {
		t.Fatal(err)
	}
	results, total, err = index.Query(EmailQuery{Limit: 10})
	if err != nil || total != 1 || len(results) != 1 || results[0].ID != "two" {
		t.Fatalf("rebuilt query = %#v, total %d, err %v", results, total, err)
	}
	if !index.OwnsPath(path) || !index.OwnsPath(path+"-wal") || !index.OwnsPath(filepath.Dir(path)) || index.OwnsPath(filepath.Join(filepath.Dir(path), "other.db")) {
		t.Fatal("SQLite artifact ownership mismatch")
	}
	if runtime.GOOS != "windows" {
		for _, artifact := range []string{path, path + "-wal", path + "-shm"} {
			info, err := os.Stat(artifact)
			if err != nil {
				t.Fatalf("stat SQLite artifact %s: %v", artifact, err)
			}
			if got := info.Mode().Perm(); got != 0600 {
				t.Fatalf("SQLite artifact %s permissions = %o, want 600", artifact, got)
			}
		}
	}
	results, total, err = index.Query(EmailQuery{Limit: int(^uint(0) >> 1)})
	if err != nil || total != 1 || len(results) != 1 {
		t.Fatalf("unbounded compatibility query = %#v, total %d, err %v", results, total, err)
	}
}

func TestSQLiteMailboxIndexUsesOneSnapshotForCountAndPage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mailbox.db")
	index, err := NewSQLiteMailboxIndex(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = index.Close() }()
	writer, err := NewSQLiteMailboxIndex(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = writer.Close() }()
	now := time.Now().UTC()
	first := IndexedEmail{ID: "first", MessageTime: now, ReceivedAt: now}
	second := IndexedEmail{ID: "second", MessageTime: now.Add(time.Second), ReceivedAt: now.Add(time.Second)}
	if err := index.Rebuild([]IndexedEmail{first}); err != nil {
		t.Fatal(err)
	}
	index.afterQueryCount = func() {
		if err := writer.Upsert(second); err != nil {
			t.Errorf("concurrent index update: %v", err)
		}
	}

	results, total, err := index.Query(EmailQuery{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(results) != 1 || results[0].ID != "first" {
		t.Fatalf("mixed query snapshot = %#v, total %d", results, total)
	}
	index.afterQueryCount = nil
	results, total, err = index.Query(EmailQuery{Limit: 10})
	if err != nil || total != 2 || len(results) != 2 {
		t.Fatalf("query after concurrent update = %#v, total %d, err %v", results, total, err)
	}
}

func TestMailServerUsesSQLiteIndexAndSynchronizesMutations(t *testing.T) {
	directory := t.TempDir()
	indexPath := filepath.Join(directory, "mailbox.db")
	index, err := NewSQLiteMailboxIndex(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewMailServerWithOptions(1025, "localhost", directory, ServerOptions{MailboxIndex: index})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("close mail server: %v", err)
		}
	}()
	for position, value := range []struct{ id, subject string }{{"mail-one", "Alpha"}, {"mail-two", "Beta"}} {
		if err := os.WriteFile(filepath.Join(directory, value.id+".eml"), []byte("message"), 0600); err != nil {
			t.Fatal(err)
		}
		email := &types.Email{ID: value.id, Subject: value.subject, Text: value.subject + " body", Time: time.Now().Add(time.Duration(position) * time.Second)}
		envelope := &types.Envelope{From: "sender@example.test", To: []string{"recipient@example.test"}}
		if err := server.SaveEmailToStore(value.id, false, envelope, email); err != nil {
			t.Fatal(err)
		}
	}
	results, total := server.QueryEmailPreviews(EmailQuery{Text: "beta", Limit: 10})
	if total != 1 || len(results) != 1 || results[0].ID != "mail-two" {
		t.Fatalf("indexed previews = %#v, total %d", results, total)
	}
	if err := server.ReadEmail("mail-two"); err != nil {
		t.Fatal(err)
	}
	read := true
	results, total = server.QueryEmailPreviews(EmailQuery{Read: &read, Limit: 10})
	if total != 1 || len(results) != 1 || results[0].ID != "mail-two" {
		t.Fatalf("indexed read query = %#v, total %d", results, total)
	}
	if err := server.DeleteEmail("mail-two"); err != nil {
		t.Fatal(err)
	}
	_, total = server.QueryEmailPreviews(EmailQuery{Limit: 10})
	if total != 1 {
		t.Fatalf("indexed total after delete = %d", total)
	}
	stats := server.GetEmailStats()["index"].(map[string]interface{})
	if stats["enabled"] != true || stats["ready"] != true || stats["backend"] != "sqlite" {
		t.Fatalf("index stats = %#v", stats)
	}
}

func TestReloadRebuildsSQLiteStorePositionsAfterSorting(t *testing.T) {
	directory := t.TempDir()
	index, err := NewSQLiteMailboxIndex(filepath.Join(t.TempDir(), "mailbox.db"))
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewMailServerWithOptions(1025, "localhost", directory, ServerOptions{MailboxIndex: index})
	if err != nil {
		_ = index.Close()
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	laterID, earlierID := "Aa11Bb22", "Zz99Yy88"
	later := time.Unix(1_700_000_100, 0)
	earlier := later.Add(-time.Minute)
	for _, item := range []struct {
		id       string
		received time.Time
	}{{laterID, later}, {earlierID, earlier}} {
		path := filepath.Join(directory, item.id+".eml")
		if err := os.WriteFile(path, validMessage(item.id), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, item.received, item.received); err != nil {
			t.Fatal(err)
		}
	}
	if err := server.LoadMailsFromDirectory(); err != nil {
		t.Fatal(err)
	}
	results, total := server.QueryEmailPreviews(EmailQuery{SortBy: "store", Limit: 10})
	if total != 2 || len(results) != 2 || results[0].ID != earlierID || results[1].ID != laterID {
		t.Fatalf("indexed reload order = %#v, total %d", results, total)
	}
}

func TestStartupPreservesSQLiteIndexInsideGeneratedIDDirectory(t *testing.T) {
	directory := t.TempDir()
	indexDirectory := filepath.Join(directory, "Ab12Cd34")
	indexPath := filepath.Join(indexDirectory, "mailbox.db")
	index, err := NewSQLiteMailboxIndex(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewMailServerWithOptions(1025, "localhost", directory, ServerOptions{MailboxIndex: index})
	if err != nil {
		_ = index.Close()
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	if _, err := os.Stat(indexPath); err != nil {
		t.Fatalf("startup moved the configured SQLite index: %v", err)
	}
	if _, err := os.Stat(indexDirectory); err != nil {
		t.Fatalf("startup moved the configured SQLite directory: %v", err)
	}
	status := server.GetEmailStats()["index"].(map[string]interface{})
	if status["ready"] != true {
		t.Fatalf("configured index was not ready after startup: %#v", status)
	}
}

func TestDeleteAllPreservesNestedSQLiteIndex(t *testing.T) {
	directory := t.TempDir()
	indexPath := filepath.Join(directory, ".index", "mailbox.db")
	index, err := NewSQLiteMailboxIndex(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewMailServerWithOptions(1025, "localhost", directory, ServerOptions{MailboxIndex: index})
	if err != nil {
		_ = index.Close()
		t.Fatal(err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("close mail server: %v", err)
		}
	}()

	id := "nested-index-mail"
	if err := os.WriteFile(filepath.Join(directory, id+".eml"), []byte("message"), 0600); err != nil {
		t.Fatal(err)
	}
	email := &types.Email{ID: id, Subject: "Nested index", Time: time.Now()}
	envelope := &types.Envelope{From: "sender@example.test", To: []string{"recipient@example.test"}}
	if err := server.SaveEmailToStore(id, false, envelope, email); err != nil {
		t.Fatal(err)
	}
	if err := server.DeleteAllEmail(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(indexPath); err != nil {
		t.Fatalf("nested SQLite index was removed: %v", err)
	}
	results, total := server.QueryEmailPreviews(EmailQuery{Limit: 10})
	if total != 0 || len(results) != 0 {
		t.Fatalf("query after delete-all = %#v, total %d", results, total)
	}
	status := server.GetEmailStats()["index"].(map[string]interface{})
	if status["ready"] != true {
		t.Fatalf("nested index was disabled: %#v", status)
	}
}

func TestDeleteRejectsSQLiteIndexInsideEmailDirectory(t *testing.T) {
	directory := t.TempDir()
	id := "index-owner"
	indexPath := filepath.Join(directory, id, "mailbox.db")
	index, err := NewSQLiteMailboxIndex(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewMailServerWithOptions(1025, "localhost", directory, ServerOptions{MailboxIndex: index})
	if err != nil {
		_ = index.Close()
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	if err := os.WriteFile(filepath.Join(directory, id+".eml"), []byte("message"), 0600); err != nil {
		t.Fatal(err)
	}
	email := &types.Email{ID: id, Subject: "Index owner", Time: time.Now()}
	envelope := &types.Envelope{From: "sender@example.test", To: []string{"recipient@example.test"}}
	if err := server.SaveEmailToStore(id, false, envelope, email); err != nil {
		t.Fatal(err)
	}

	if err := server.DeleteEmail(id); err == nil || !strings.Contains(err.Error(), "mailbox index") {
		t.Fatalf("DeleteEmail() error = %v, want protected-index error", err)
	}
	if _, err := os.Stat(indexPath); err != nil {
		t.Fatalf("SQLite index was removed: %v", err)
	}
	if _, err := server.GetEmail(id); err != nil {
		t.Fatalf("email was removed despite protected index: %v", err)
	}
	if err := server.DeleteAllEmail(); err == nil || !strings.Contains(err.Error(), "mailbox index") {
		t.Fatalf("DeleteAllEmail() error = %v, want protected-index error", err)
	}
	if _, err := os.Stat(indexPath); err != nil {
		t.Fatalf("SQLite index was removed by delete-all: %v", err)
	}
}
