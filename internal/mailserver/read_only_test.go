package mailserver

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/emersion/go-message/mail"
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
	if err := server.OnWithConcurrency("new", 1, func(email *Email) { notified <- email.ID }); err != nil {
		t.Fatal(err)
	}
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

func TestRefreshReadOnlyMailboxRestoresPersistedEnvelopeRecipients(t *testing.T) {
	directory := t.TempDir()
	writer, err := NewMailServer(1025, "localhost", directory)
	if err != nil {
		t.Fatal(err)
	}
	const id = "persisted-envelope"
	raw := "From: sender@example.test\r\nTo: visible@example.test\r\nSubject: envelope\r\n\r\nbody"
	if err := os.WriteFile(filepath.Join(directory, id+".eml"), []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	envelope := &Envelope{From: "sender@example.test", To: []string{"visible@example.test", "blind@example.test"}}
	email := &Email{To: []*mail.Address{{Address: "visible@example.test"}}, Subject: "envelope"}
	if err := writer.SaveEmailToStore(id, false, envelope, email); err != nil {
		t.Fatal(err)
	}
	_ = writer.Close()

	observer, err := NewMailServerWithOptions(1025, "localhost", directory, ServerOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = observer.Close() }()
	if err := observer.RefreshReadOnlyMailbox(); err != nil {
		t.Fatal(err)
	}
	observed, err := observer.GetEmail(id)
	if err != nil {
		t.Fatal(err)
	}
	if observed.Envelope == nil || len(observed.Envelope.To) != 2 || observed.Envelope.To[1] != "blind@example.test" {
		t.Fatalf("restored envelope = %#v", observed.Envelope)
	}
	if len(observed.CalculatedBCC) != 1 || observed.CalculatedBCC[0].Address != "blind@example.test" {
		t.Fatalf("calculated BCC = %#v", observed.CalculatedBCC)
	}
}

func TestRefreshReadOnlyMailboxDefersNewEventUntilEnvelopeMetadataIsReadable(t *testing.T) {
	directory := t.TempDir()
	const id = "deferred-envelope-metadata"
	raw := "From: sender@example.test\r\nTo: visible@example.test\r\nSubject: deferred envelope\r\n\r\nbody"
	if err := os.WriteFile(filepath.Join(directory, id+".eml"), []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(directory, metadataDirectoryName), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, metadataDirectoryName, id+".json"), []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	observer, err := NewMailServerWithOptions(1025, "localhost", directory, ServerOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = observer.Close() }()
	notified := make(chan *Email, 1)
	observer.On("new", func(email *Email) { notified <- email })
	var partial *ReadOnlyRefreshPartialError
	if err := observer.RefreshReadOnlyMailbox(); !errors.As(err, &partial) {
		t.Fatalf("initial refresh error = %v, want ReadOnlyRefreshPartialError", err)
	}
	if _, err := observer.GetEmail(id); err == nil {
		t.Fatal("email with uncertain envelope metadata was published")
	}

	metadata := emailMetadata{
		Version: currentMetadataVersion, ID: id, Sequence: time.Now().UTC(),
		Envelope: &Envelope{To: []string{"visible@example.test", "blind@example.test"}},
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, metadataDirectoryName, id+".json"), encoded, 0600); err != nil {
		t.Fatal(err)
	}
	if err := observer.RefreshReadOnlyMailbox(); err != nil {
		t.Fatal(err)
	}
	select {
	case email := <-notified:
		if email.Envelope == nil || len(email.Envelope.To) != 2 || len(email.CalculatedBCC) != 1 || email.CalculatedBCC[0].Address != "blind@example.test" {
			t.Fatalf("notified email = %#v", email)
		}
	case <-time.After(time.Second):
		t.Fatal("repaired envelope metadata did not emit a new event")
	}
}

func TestRefreshReadOnlyMailboxRestoresLegacyAttachmentFilenameWithoutWritingMetadata(t *testing.T) {
	directory := t.TempDir()
	const id = "legacy-read-only-attachment"
	const generatedFilename = "legacy-generated.txt"
	if err := os.WriteFile(filepath.Join(directory, id+".eml"), multipartMessage(), 0600); err != nil {
		t.Fatal(err)
	}
	attachmentDirectory := filepath.Join(directory, id)
	if err := os.MkdirAll(attachmentDirectory, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(attachmentDirectory, generatedFilename), []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}

	observer, err := NewMailServerWithOptions(1025, "localhost", directory, ServerOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = observer.Close() }()
	if err := observer.RefreshReadOnlyMailbox(); err != nil {
		t.Fatal(err)
	}
	observed, err := observer.GetEmail(id)
	if err != nil || len(observed.Attachments) != 1 {
		t.Fatalf("observed email = %#v, %v", observed, err)
	}
	if got := observed.Attachments[0].GeneratedFileName; got != generatedFilename {
		t.Fatalf("restored attachment filename = %q, want %q", got, generatedFilename)
	}
	if _, err := os.Stat(filepath.Join(directory, metadataDirectoryName)); !os.IsNotExist(err) {
		t.Fatalf("read-only legacy restoration created metadata: %v", err)
	}
}

func TestRefreshReadOnlyMailboxRetriesLegacyAttachmentRestoration(t *testing.T) {
	directory := t.TempDir()
	const id = "legacy-attachment-retry"
	if err := os.WriteFile(filepath.Join(directory, id+".eml"), multipartMessage(), 0600); err != nil {
		t.Fatal(err)
	}

	observer, err := NewMailServerWithOptions(1025, "localhost", directory, ServerOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = observer.Close() }()
	var partial *ReadOnlyRefreshPartialError
	if err := observer.RefreshReadOnlyMailbox(); !errors.As(err, &partial) {
		t.Fatalf("initial refresh error = %v, want ReadOnlyRefreshPartialError", err)
	}
	if _, err := observer.GetEmail(id); err == nil {
		t.Fatal("email with unresolved legacy attachment metadata was published")
	}

	attachmentDirectory := filepath.Join(directory, id)
	if err := os.MkdirAll(attachmentDirectory, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(attachmentDirectory, "recovered.txt"), []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := observer.RefreshReadOnlyMailbox(); err != nil {
		t.Fatal(err)
	}
	observed, err := observer.GetEmail(id)
	if err != nil || len(observed.Attachments) != 1 || observed.Attachments[0].GeneratedFileName != "recovered.txt" {
		t.Fatalf("recovered email = %#v, %v", observed, err)
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

func TestVisibleRawEmailSourceHidesDeletionFencedMail(t *testing.T) {
	directory := t.TempDir()
	server, err := NewMailServerWithOptions(1025, "localhost", directory, ServerOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	const id = "source-hidden-by-fence"
	raw := []byte("From: sender@example.test\r\nTo: recipient@example.test\r\nSubject: source\r\n\r\nbody")
	emlPath := filepath.Join(directory, id+".eml")
	if err := os.WriteFile(emlPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := server.RefreshReadOnlyMailbox(); err != nil {
		t.Fatal(err)
	}
	content, size, truncated, err := server.GetVisibleRawEmailContentLimit(id, 1024)
	if err != nil || string(content) != string(raw) || size != int64(len(raw)) || truncated {
		t.Fatalf("visible source = %q, %d, %t, %v", content, size, truncated, err)
	}

	if err := os.WriteFile(deletionFencePath(directory, id), []byte(deletionFenceState+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := server.RefreshReadOnlyMailbox(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(emlPath); err != nil {
		t.Fatalf("source file was unexpectedly removed: %v", err)
	}
	if _, _, _, err := server.GetVisibleRawEmailContentLimit(id, 1024); err == nil {
		t.Fatal("deletion-fenced source remained readable")
	}
}

func TestRefreshReadOnlyMailboxRechecksDeletionFenceBeforePublishing(t *testing.T) {
	directory := t.TempDir()
	server, err := NewMailServerWithOptions(1025, "localhost", directory, ServerOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	const id = "deleted-during-refresh"
	raw := []byte("From: sender@example.test\r\nTo: recipient@example.test\r\nSubject: disappearing\r\n\r\nbody")
	if err := os.WriteFile(filepath.Join(directory, id+".eml"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	server.beforeReadOnlyPublish = func(candidateID string) {
		if candidateID != id {
			return
		}
		server.beforeReadOnlyPublish = nil
		if err := os.WriteFile(deletionFencePath(directory, id), []byte(deletionFenceState+"\n"), 0600); err != nil {
			t.Errorf("create deletion fence: %v", err)
		}
	}
	notified := make(chan string, 1)
	server.On("new", func(email *Email) { notified <- email.ID })

	if err := server.RefreshReadOnlyMailbox(); err != nil {
		t.Fatal(err)
	}
	if _, err := server.GetEmail(id); err == nil {
		t.Fatal("deletion-fenced email was published after parsing")
	}
	select {
	case published := <-notified:
		t.Fatalf("deletion-fenced email emitted a new event for %q", published)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestRefreshReadOnlyMailboxRejectsDeletionCompletedBeforePublicationRescan(t *testing.T) {
	directory := t.TempDir()
	server, err := NewMailServerWithOptions(1025, "localhost", directory, ServerOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	const id = "deleted-before-rescan"
	raw := []byte("From: sender@example.test\r\nTo: recipient@example.test\r\nSubject: deleted\r\n\r\nbody")
	if err := os.WriteFile(filepath.Join(directory, id+".eml"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	server.beforeReadOnlyPublish = func(candidateID string) {
		if candidateID != id {
			return
		}
		server.beforeReadOnlyPublish = nil
		if err := server.ensureDeletionFence(id); err != nil {
			t.Errorf("create deletion fence: %v", err)
			return
		}
		if err := server.cleanupDeletionFencedEmail(id); err != nil {
			t.Errorf("complete deletion: %v", err)
		}
	}
	notified := make(chan string, 1)
	server.On("new", func(email *Email) { notified <- email.ID })

	if err := server.RefreshReadOnlyMailbox(); err != nil {
		t.Fatal(err)
	}
	if _, err := server.GetEmail(id); err == nil {
		t.Fatal("completed deletion was published from a stale source observation")
	}
	select {
	case published := <-notified:
		t.Fatalf("completed deletion emitted a new event for %q", published)
	case <-time.After(50 * time.Millisecond):
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
	if err := server.OnWithConcurrency("new", 1, func(email *Email) { notified <- email.ID }); err != nil {
		t.Fatal(err)
	}
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
	if _, err := server.GetEmail(id); err == nil {
		t.Fatal("attachment email with uncertain metadata was published")
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

	if err := os.WriteFile(server.metadataPath(id), []byte("temporarily-unreadable"), 0600); err != nil {
		t.Fatal(err)
	}
	partial = nil
	if err := server.RefreshReadOnlyMailbox(); !errors.As(err, &partial) {
		t.Fatalf("partial refresh error = %v, want ReadOnlyRefreshPartialError", err)
	}
	afterPartial, err := server.GetEmail(id)
	if err != nil || !afterPartial.Read {
		t.Fatalf("partial refresh lost known read state: %#v, %v", afterPartial, err)
	}
	if receivedAt := server.receivedAt(id); !receivedAt.Equal(metadata.Sequence) {
		t.Fatalf("partial refresh sequence = %s, want %s", receivedAt, metadata.Sequence)
	}
}
