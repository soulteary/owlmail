package mailserver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/soulteary/owlmail/internal/types"
)

func sampleOwnership() *types.Ownership {
	return &types.Ownership{MailboxID: uuid.NewString(), IngestAccountID: "gorge", TransactionID: uuid.NewString(), MatchedRecipients: []string{"alice@example.com"}, RouteRevisions: []int64{1}}
}
func isolatedServer(t *testing.T, dir string) *MailServer {
	t.Helper()
	s, err := NewMailServerWithOptions(0, "localhost", dir, ServerOptions{RequireOwnership: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
func TestOwnedEmailRoundTripAndStandaloneFence(t *testing.T) {
	dir := t.TempDir()
	s := isolatedServer(t, dir)
	ownership := sampleOwnership()
	session := &Session{mailServer: s, from: "sender@example.com", to: []string{"alice@example.com"}, ownership: ownership}
	const id = "owned-email"
	if err := s.storeIncomingEmail(id, strings.NewReader("From: sender@example.com\r\nTo: alice@example.com\r\nSubject: owned\r\n\r\nhello"), session); err != nil {
		t.Fatal(err)
	}
	ownership.RouteRevisions[0] = 99
	first, err := s.GetEmail(id)
	if err != nil {
		t.Fatal(err)
	}
	if first.Ownership == nil || first.Ownership.RouteRevisions[0] != 1 {
		t.Fatal("ownership aliases caller")
	}
	first.Ownership.MatchedRecipients[0] = "mutated@example.com"
	metadata, err := s.loadEmailMetadata(id)
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Version != ownershipMetadataVersion || metadata.Ownership.MatchedRecipients[0] != "alice@example.com" {
		t.Fatal("missing durable ownership")
	}
	encoded, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "mailboxId") || strings.Contains(string(encoded), "ingestAccountId") {
		t.Fatal("internal ownership leaked into API")
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := isolatedServer(t, dir)
	restored, err := reopened.GetEmail(id)
	if err != nil || restored.Ownership == nil || restored.Ownership.MailboxID != metadata.Ownership.MailboxID {
		t.Fatalf("lost ownership on restart: %+v %v", restored, err)
	}
	if _, err = NewMailServer(0, "localhost", dir); err == nil {
		t.Fatal("standalone opened isolated directory")
	}
	if _, err = NewMailServerWithOptions(0, "localhost", dir, ServerOptions{ReadOnly: true}); err == nil {
		t.Fatal("MCP observer opened isolated directory")
	}
}
func TestIsolatedRecoveryRejectsMissingOrInvalidOwnership(t *testing.T) {
	for _, scenario := range []string{"missing", "invalid", "downgraded"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			s := isolatedServer(t, dir)
			session := &Session{mailServer: s, from: "sender@example.com", to: []string{"alice@example.com"}, ownership: sampleOwnership()}
			if err := s.storeIncomingEmail("owned", strings.NewReader("Subject: owned\r\n\r\nbody"), session); err != nil {
				t.Fatal(err)
			}
			path := s.metadataPath("owned")
			metadata, err := s.loadEmailMetadata("owned")
			if err != nil {
				t.Fatal(err)
			}
			if err = s.Close(); err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "missing":
				err = os.Remove(path)
			case "invalid":
				metadata.Ownership.MailboxID = "invalid"
				var data []byte
				data, err = json.Marshal(metadata)
				if err == nil {
					err = os.WriteFile(path, data, 0600)
				}
			case "downgraded":
				metadata.Ownership = nil
				metadata.Version = currentMetadataVersion
				var data []byte
				data, err = json.Marshal(metadata)
				if err == nil {
					err = os.WriteFile(path, data, 0600)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if reopened, err := NewMailServerWithOptions(0, "localhost", dir, ServerOptions{RequireOwnership: true}); err == nil {
				_ = reopened.Close()
				t.Fatal("accepted unowned email during recovery")
			}
		})
	}
}
func TestIsolatedWriteRejectsMissingOwnershipAndLegacyDirectory(t *testing.T) {
	dir := t.TempDir()
	s := isolatedServer(t, dir)
	if err := s.storeIncomingEmail("missing", strings.NewReader("Subject: test\r\n\r\nbody"), nil); err == nil {
		t.Fatal("accepted missing ownership")
	}
	if emails := s.GetAllEmail(); len(emails) != 0 {
		t.Fatal("rejected email published")
	}
	if _, err := os.Stat(filepath.Join(dir, "missing.eml")); !os.IsNotExist(err) {
		t.Fatal("rejected EML committed")
	}
	legacy := t.TempDir()
	if err := os.WriteFile(filepath.Join(legacy, "legacy.eml"), []byte("Subject: legacy\r\n\r\nbody"), 0600); err != nil {
		t.Fatal(err)
	}
	if upgraded, err := NewMailServerWithOptions(0, "localhost", legacy, ServerOptions{RequireOwnership: true}); err == nil {
		_ = upgraded.Close()
		t.Fatal("implicitly migrated standalone mail")
	}
}
func TestStandaloneMetadataFormatUnchanged(t *testing.T) {
	s, err := NewMailServer(0, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if err = s.storeIncomingEmail("legacy", strings.NewReader("Subject: test\r\n\r\nbody"), nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(s.metadataPath("legacy"))
	if err != nil {
		t.Fatal(err)
	}
	var metadata emailMetadata
	if err = json.Unmarshal(data, &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata.Version != 3 || strings.Contains(string(data), "ownership") {
		t.Fatal("standalone disk format changed")
	}
	if _, err = os.Stat(filepath.Join(s.mailDir, ownershipMarker)); !os.IsNotExist(err) {
		t.Fatal("standalone created mode marker")
	}
}
