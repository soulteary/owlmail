package mailserver

import (
	"os"
	"path/filepath"
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
	defer index.Close()
	now := time.Now().UTC()
	records := []IndexedEmail{
		{ID: "one", MessageTime: now.Add(-time.Hour), ReceivedAt: now, SubjectSearch: "alpha", TextSearch: "first body", FromSearch: "alice@example.test", RecipientsSearch: "team@example.test", FirstFrom: "alice@example.test", StorePosition: 0},
		{ID: "two", MessageTime: now, ReceivedAt: now, Read: true, SubjectSearch: "beta", TextSearch: "second body", FromSearch: "bob@example.test", RecipientsSearch: "team@example.test", FirstFrom: "bob@example.test", StorePosition: 1},
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
	if err := index.Rebuild(records[1:]); err != nil {
		t.Fatal(err)
	}
	results, total, err = index.Query(EmailQuery{Limit: 10})
	if err != nil || total != 1 || len(results) != 1 || results[0].ID != "two" {
		t.Fatalf("rebuilt query = %#v, total %d, err %v", results, total, err)
	}
	if !index.OwnsPath(path) || !index.OwnsPath(path+"-wal") || index.OwnsPath(filepath.Join(filepath.Dir(path), "other.db")) {
		t.Fatal("SQLite artifact ownership mismatch")
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
	defer server.Close()
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
