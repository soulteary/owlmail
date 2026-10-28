package mailserver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func receiptMailbox(t *testing.T) (*MailServer, string) {
	t.Helper()
	directory := t.TempDir()
	server, err := NewMailServer(1025, "127.0.0.1", directory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	raw := "From: sender@example.test\r\nTo: original@example.test\r\nSubject: accepted receipt\r\n\r\nbody"
	if err := server.storeIncomingEmail("receipt-mail", strings.NewReader(raw), nil); err != nil {
		t.Fatal(err)
	}
	return server, directory
}

func TestRelayReceiptSurvivesHistoryPruningReadAndRestart(t *testing.T) {
	server, directory := receiptMailbox(t)
	accepted := time.Date(2026, 10, 29, 2, 3, 4, 0, time.FixedZone("test", 3600))
	recipients := []string{"actual@example.test", "blind@example.test"}
	if err := server.RecordRelaySuccess("receipt-mail", accepted, recipients); err != nil {
		t.Fatal(err)
	}
	recipients[0] = "mutated@example.test"
	assertReceipt := func(email *Email) {
		t.Helper()
		if email.RelayedAt == nil || !email.RelayedAt.Equal(accepted) || len(email.RelayedTo) != 2 || email.RelayedTo[0] != "actual@example.test" {
			t.Fatalf("receipt = %#v, %v", email.RelayedAt, email.RelayedTo)
		}
	}
	email, err := server.GetEmail("receipt-mail")
	if err != nil {
		t.Fatal(err)
	}
	assertReceipt(email)
	*email.RelayedAt = accepted.Add(time.Hour)
	email.RelayedTo[0] = "snapshot-mutation@example.test"
	fresh, err := server.GetEmail("receipt-mail")
	if err != nil {
		t.Fatal(err)
	}
	assertReceipt(fresh)
	if err := server.RecordRelaySuccess("receipt-mail", accepted.Add(-time.Hour), []string{"older@example.test"}); err != nil {
		t.Fatal(err)
	}
	if err := server.ReadEmail("receipt-mail"); err != nil {
		t.Fatal(err)
	}
	// Deleting the bounded task history must not erase per-message evidence.
	if err := os.RemoveAll(filepath.Join(directory, metadataDirectoryName, relayJobMetadataDirectoryName)); err != nil {
		t.Fatal(err)
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	restored, err := NewMailServer(1025, "127.0.0.1", directory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = restored.Close() }()
	email, err = restored.GetEmail("receipt-mail")
	if err != nil {
		t.Fatal(err)
	}
	assertReceipt(email)
	if !email.Read {
		t.Fatal("read-state update lost during receipt persistence")
	}
}

func TestRelayReceiptPersistenceFailureRetainsAcceptedMemoryAndCanRepair(t *testing.T) {
	server, _ := receiptMailbox(t)
	path := server.metadataPath("receipt-mail")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	accepted := time.Now().UTC()
	if err := server.RecordRelaySuccess("receipt-mail", accepted, []string{"actual@example.test"}); err == nil {
		t.Fatal("expected metadata replacement failure")
	}
	email, err := server.GetEmail("receipt-mail")
	if err != nil {
		t.Fatal(err)
	}
	if email.RelayedAt == nil || !email.RelayedAt.Equal(accepted) || email.RelayedTo[0] != "actual@example.test" {
		t.Fatal("accepted delivery was rolled back in memory")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := server.RecordRelaySuccess("receipt-mail", accepted, email.RelayedTo); err != nil {
		t.Fatal(err)
	}
	metadata, err := server.loadEmailMetadata("receipt-mail")
	if err != nil {
		t.Fatal(err)
	}
	if metadata.RelayedAt == nil || !metadata.RelayedAt.Equal(accepted) {
		t.Fatal("repair did not persist acceptance")
	}
}

func TestReadOnlyRelayReceiptRefreshAndMutationRejection(t *testing.T) {
	server, directory := receiptMailbox(t)
	first := time.Now().UTC().Add(-time.Minute)
	if err := server.RecordRelaySuccess("receipt-mail", first, []string{"first@example.test"}); err != nil {
		t.Fatal(err)
	}
	observer, err := NewMailServerWithOptions(1025, "127.0.0.1", directory, ServerOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = observer.Close() }()
	if err := observer.RefreshReadOnlyMailbox(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(server.metadataPath("receipt-mail"))
	if err != nil {
		t.Fatal(err)
	}
	if err := observer.RecordRelaySuccess("receipt-mail", time.Now(), []string{"forbidden@example.test"}); err == nil {
		t.Fatal("observer wrote relay metadata")
	}
	after, err := os.ReadFile(server.metadataPath("receipt-mail"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("read-only observer changed metadata")
	}
	second := first.Add(time.Minute)
	if err := server.RecordRelaySuccess("receipt-mail", second, []string{"latest@example.test"}); err != nil {
		t.Fatal(err)
	}
	if err := observer.RefreshReadOnlyMailbox(); err != nil {
		t.Fatal(err)
	}
	email, err := observer.GetEmail("receipt-mail")
	if err != nil {
		t.Fatal(err)
	}
	if email.RelayedAt == nil || !email.RelayedAt.Equal(second) || email.RelayedTo[0] != "latest@example.test" {
		t.Fatalf("observer receipt stale: %#v", email)
	}
}

func TestRelayReceiptOptionalAndMigrationCompatible(t *testing.T) {
	raw, err := json.Marshal(&Email{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "relayed") {
		t.Fatal("empty receipt serialized")
	}
	directory := t.TempDir()
	at := time.Now().UTC()
	old := emailMetadata{Version: 3, ID: "migrated", Sequence: at}
	if err := persistMigrationMetadata(directory, old); err != nil {
		t.Fatal(err)
	}
	server := &MailServer{mailDir: directory}
	restored, err := server.loadEmailMetadata(old.ID)
	if err != nil {
		t.Fatal(err)
	}
	if restored.RelayedAt != nil {
		t.Fatal("old metadata fabricated a receipt")
	}
	old.RelayedAt = &at
	old.RelayedTo = []string{"actual@example.test"}
	if err := persistMigrationMetadata(directory, old); err != nil {
		t.Fatal(err)
	}
	restored, err = server.loadEmailMetadata(old.ID)
	if err != nil {
		t.Fatal(err)
	}
	if restored.RelayedAt == nil || !restored.RelayedAt.Equal(at) || restored.RelayedTo[0] != "actual@example.test" {
		t.Fatal("attachment migration lost receipt")
	}
}

func TestSMTPTransactionMetadataSurvivesRestartObserverAndMigration(t *testing.T) {
	server, directory := receiptMailbox(t)
	server.storeMutex.Lock()
	email := server.storeByID["receipt-mail"]
	email.Envelope.From = ""
	email.Envelope.SMTPTransaction = true
	err := server.persistEmailMetadataAt(email, server.receivedAtByID[email.ID])
	server.storeMutex.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := server.loadEmailMetadata("receipt-mail")
	if err != nil {
		t.Fatal(err)
	}
	if !metadata.SMTPTransaction {
		t.Fatal("SMTP provenance omitted from metadata")
	}
	if err := persistMigrationMetadata(directory, metadata); err != nil {
		t.Fatal(err)
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	restored, err := NewMailServer(1025, "127.0.0.1", directory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = restored.Close() }()
	message, err := restored.GetEmail("receipt-mail")
	if err != nil {
		t.Fatal(err)
	}
	if message.Envelope == nil || !message.Envelope.SMTPTransaction || message.Envelope.From != "" {
		t.Fatalf("restored null sender = %#v", message.Envelope)
	}
	observer, err := NewMailServerWithOptions(1025, "127.0.0.1", directory, ServerOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = observer.Close() }()
	if err := observer.RefreshReadOnlyMailbox(); err != nil {
		t.Fatal(err)
	}
	message, err = observer.GetEmail("receipt-mail")
	if err != nil {
		t.Fatal(err)
	}
	if message.Envelope == nil || !message.Envelope.SMTPTransaction || message.Envelope.From != "" {
		t.Fatalf("observer null sender = %#v", message.Envelope)
	}
}
