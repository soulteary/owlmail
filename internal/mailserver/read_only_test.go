package mailserver

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRefreshReadOnlyMailboxObservesNewMailWithoutMutatingArtifacts(t *testing.T) {
	directory := t.TempDir()
	artifact := filepath.Join(directory, ".owlmail-tmp-active")
	if err := os.WriteFile(artifact, []byte("active"), 0600); err != nil {
		t.Fatal(err)
	}
	server, err := NewMailServerWithOptions(1025, "localhost", directory, ServerOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	if err := server.RefreshReadOnlyMailbox(); err != nil {
		t.Fatal(err)
	}

	notified := make(chan string, 1)
	server.On("new", func(email *Email) { notified <- email.ID })
	raw := "From: sender@example.test\r\nTo: recipient@example.test\r\nSubject: observed\r\nDate: Wed, 02 Sep 2026 12:00:00 +0000\r\n\r\nbody"
	if err := os.WriteFile(filepath.Join(directory, "observed.eml"), []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	if err := server.RefreshReadOnlyMailbox(); err != nil {
		t.Fatal(err)
	}
	if email, err := server.GetEmail("observed"); err != nil || email.Subject != "observed" {
		t.Fatalf("GetEmail() = %#v, %v", email, err)
	}
	select {
	case id := <-notified:
		if id != "observed" {
			t.Fatalf("notification ID = %q", id)
		}
	case <-time.After(time.Second):
		t.Fatal("new mail was not announced")
	}
	if data, err := os.ReadFile(artifact); err != nil || string(data) != "active" {
		t.Fatalf("observer mutated active artifact: %q, %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(directory, metadataDirectoryName)); !os.IsNotExist(err) {
		t.Fatalf("observer created metadata directory: %v", err)
	}
}

func TestReadOnlyConstructorRequiresExistingDirectory(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	if _, err := NewMailServerWithOptions(1025, "localhost", missing, ServerOptions{ReadOnly: true}); err == nil {
		t.Fatal("read-only constructor created a missing mailbox")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("missing mailbox was created: %v", err)
	}
}

func TestRefreshReadOnlyMailboxHidesTransactionFencedMail(t *testing.T) {
	directory := t.TempDir()
	server, err := NewMailServerWithOptions(1025, "localhost", directory, ServerOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	raw := []byte("From: sender@example.test\r\nTo: recipient@example.test\r\nSubject: fenced\r\n\r\nbody")
	for _, id := range []string{"active-message", "deleted-message"} {
		if err := os.WriteFile(filepath.Join(directory, id+".eml"), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(rollbackFencePath(directory, "active-message"), []byte(activeFenceState+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(deletionFencePath(directory, "deleted-message"), []byte(deletionFenceState+"\n"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := server.RefreshReadOnlyMailbox(); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"active-message", "deleted-message"} {
		if _, err := server.GetEmail(id); err == nil {
			t.Fatalf("transaction-fenced email %q became visible", id)
		}
	}

	if err := os.WriteFile(rollbackFencePath(directory, "active-message"), []byte(acceptedFenceState+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := server.RefreshReadOnlyMailbox(); err != nil {
		t.Fatal(err)
	}
	if email, err := server.GetEmail("active-message"); err != nil || email.Subject != "fenced" {
		t.Fatalf("accepted email = %#v, %v", email, err)
	}
	if _, err := server.GetEmail("deleted-message"); err == nil {
		t.Fatal("deletion-fenced email became visible")
	}
}

func TestRefreshReadOnlyMailboxPreservesExistingMailWhenFenceIsUnreadable(t *testing.T) {
	directory := t.TempDir()
	server, err := NewMailServerWithOptions(1025, "localhost", directory, ServerOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	const id = "stable-message"
	raw := []byte("From: sender@example.test\r\nTo: recipient@example.test\r\nSubject: stable\r\n\r\nbody")
	if err := os.WriteFile(filepath.Join(directory, id+".eml"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := server.RefreshReadOnlyMailbox(); err != nil {
		t.Fatal(err)
	}

	notified := make(chan string, 1)
	server.On("new", func(email *Email) { notified <- email.ID })
	fence := rollbackFencePath(directory, id)
	if err := os.Mkdir(fence, 0700); err != nil {
		t.Fatal(err)
	}
	err = server.RefreshReadOnlyMailbox()
	var partial *ReadOnlyRefreshPartialError
	if !errors.As(err, &partial) {
		t.Fatalf("refresh error = %v, want ReadOnlyRefreshPartialError", err)
	}
	if email, getErr := server.GetEmail(id); getErr != nil || email.Subject != "stable" {
		t.Fatalf("existing email was discarded: %#v, %v", email, getErr)
	}
	if err := os.Remove(fence); err != nil {
		t.Fatal(err)
	}
	if err := server.RefreshReadOnlyMailbox(); err != nil {
		t.Fatal(err)
	}
	select {
	case duplicate := <-notified:
		t.Fatalf("existing email emitted a duplicate notification for %q", duplicate)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestRefreshReadOnlyMailboxUsesDeterministicOrderForEqualTimestamps(t *testing.T) {
	directory := t.TempDir()
	server, err := NewMailServerWithOptions(1025, "localhost", directory, ServerOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	timestamp := time.Date(2026, 9, 3, 1, 0, 0, 0, time.UTC)
	for _, id := range []string{"message-b", "message-a"} {
		path := filepath.Join(directory, id+".eml")
		if err := os.WriteFile(path, []byte("From: sender@example.test\r\nTo: recipient@example.test\r\nSubject: "+id+"\r\n\r\nbody"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, timestamp, timestamp); err != nil {
			t.Fatal(err)
		}
	}
	notified := make(chan string, 2)
	server.On("new", func(email *Email) { notified <- email.ID })
	for iteration := 0; iteration < 5; iteration++ {
		if err := server.RefreshReadOnlyMailbox(); err != nil {
			t.Fatal(err)
		}
		emails := server.GetAllEmail()
		if len(emails) != 2 || emails[0].ID != "message-a" || emails[1].ID != "message-b" {
			t.Fatalf("iteration %d order = %#v", iteration, emails)
		}
	}
	if first, second := <-notified, <-notified; first != "message-a" || second != "message-b" {
		t.Fatalf("notification order = %q, %q; want message-a, message-b", first, second)
	}
}

func TestRefreshReadOnlyMailboxReappliesRepairedAttachmentMetadata(t *testing.T) {
	directory := t.TempDir()
	server, err := NewMailServerWithOptions(1025, "localhost", directory, ServerOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	const id = "repaired-metadata"
	raw := "From: sender@example.test\r\nTo: recipient@example.test\r\nSubject: attachment\r\n" +
		"Content-Type: multipart/mixed; boundary=boundary\r\n\r\n" +
		"--boundary\r\nContent-Type: text/plain\r\n\r\nbody\r\n" +
		"--boundary\r\nContent-Type: application/octet-stream\r\nContent-Disposition: attachment; filename=file.bin\r\n\r\ndata\r\n" +
		"--boundary--\r\n"
	if err := os.WriteFile(filepath.Join(directory, id+".eml"), []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(directory, metadataDirectoryName), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(server.metadataPath(id), []byte("not-json"), 0600); err != nil {
		t.Fatal(err)
	}
	var partial *ReadOnlyRefreshPartialError
	if err := server.RefreshReadOnlyMailbox(); !errors.As(err, &partial) {
		t.Fatalf("initial refresh error = %v, want ReadOnlyRefreshPartialError", err)
	}
	before, err := server.GetEmail(id)
	if err != nil || len(before.Attachments) != 1 {
		t.Fatalf("initial email = %#v, %v", before, err)
	}

	metadata := emailMetadata{
		Version:  currentMetadataVersion,
		ID:       id,
		Read:     true,
		Sequence: time.Date(2026, 9, 3, 2, 0, 0, 0, time.UTC),
		Attachments: []attachmentMetadata{{
			GeneratedFileName: "restored.bin",
			Size:              4,
			ContentSHA256:     "0123456789abcdef",
			Storage:           attachmentStorageLocal,
		}},
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(server.metadataPath(id), encoded, 0600); err != nil {
		t.Fatal(err)
	}
	if err := server.RefreshReadOnlyMailbox(); err != nil {
		t.Fatal(err)
	}
	after, err := server.GetEmail(id)
	if err != nil || !after.Read || len(after.Attachments) != 1 {
		t.Fatalf("refreshed email = %#v, %v", after, err)
	}
	attachment := after.Attachments[0]
	if attachment.GeneratedFileName != "restored.bin" || attachment.ContentSHA256 != "0123456789abcdef" || attachment.Storage != attachmentStorageLocal {
		t.Fatalf("repaired attachment metadata = %#v", attachment)
	}
}
