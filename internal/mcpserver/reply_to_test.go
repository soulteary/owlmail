package mcpserver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emersion/go-message/mail"
	"github.com/soulteary/owlmail/internal/mailserver"
	"github.com/soulteary/owlmail/internal/types"
)

func TestReplyToDetailProjection(t *testing.T) {
	email := &types.Email{ReplyTo: []*mail.Address{{Name: "Support", Address: "reply@example.test"}}}
	detail := makeEmailDetail(email, false)
	if len(detail.ReplyTo) != 1 || detail.ReplyTo[0].Address != "reply@example.test" {
		t.Fatalf("reply_to = %#v", detail.ReplyTo)
	}
	detail.ReplyTo[0].Address = "changed@example.test"
	if email.ReplyTo[0].Address != "reply@example.test" {
		t.Fatal("projection aliases source")
	}
	raw, err := json.Marshal(makeEmailDetail(&types.Email{}, false))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"reply_to"`) {
		t.Fatalf("absent reply_to not omitted: %s", raw)
	}
}

func TestReplyToReadOnlyRecoveredDetail(t *testing.T) {
	dir := t.TempDir()
	raw := []byte("From: sender@example.test\r\nTo: receiver@example.test\r\nReply-To: Support <reply@example.test>\r\n\r\nbody")
	if err := os.WriteFile(filepath.Join(dir, "reply-stdio.eml"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	mailbox, err := mailserver.NewMailServerWithOptions(1025, "localhost", dir, mailserver.ServerOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = mailbox.Close() }()
	// The stdio transport uses this read-only refresh path, not write recovery.
	if err := mailbox.RefreshReadOnlyMailbox(); err != nil {
		t.Fatal(err)
	}
	email, err := mailbox.GetEmail("reply-stdio")
	if err != nil {
		t.Fatal(err)
	}
	detail := makeEmailDetail(email, false)
	if len(detail.ReplyTo) != 1 || detail.ReplyTo[0].Name != "Support" || detail.ReplyTo[0].Address != "reply@example.test" {
		t.Fatalf("recovered reply_to = %#v", detail.ReplyTo)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "reply-stdio.eml" {
		t.Fatalf("read-only refresh wrote metadata: %v", entries)
	}
}
