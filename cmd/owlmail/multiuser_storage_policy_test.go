package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/soulteary/owlmail/internal/config"
	"github.com/soulteary/owlmail/internal/mailserver"
	"github.com/soulteary/owlmail/internal/types"
)

func TestLegacyEvictionNeverRunsOnPersonalMailboxes(t *testing.T) {
	for _, isolated := range []bool{false, true} {
		directory := t.TempDir()
		server, err := mailserver.NewMailServerWithOptions(0, "localhost", directory, mailserver.ServerOptions{RequireOwnership: isolated})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = server.Close() })
		for _, id := range []string{"a", "b"} {
			if err = os.WriteFile(filepath.Join(directory, id+".eml"), []byte("Subject: retained\r\n\r\nbody"), 0600); err != nil {
				t.Fatal(err)
			}
			email := &types.Email{Subject: "retained", Text: "body"}
			if isolated {
				email.Ownership = &types.Ownership{MailboxID: uuid.NewString(), TransactionID: uuid.NewString(), IngestAccountID: "gorge", MatchedRecipients: []string{"a@example.com"}, RouteRevisions: []int64{1}}
			}
			if err = server.SaveEmailToStore(id, false, &types.Envelope{To: []string{"a@example.com"}}, email); err != nil {
				t.Fatal(err)
			}
		}
		cfg := config.DefaultConfig()
		cfg.MailMaxMessages = 1
		if isolated {
			cfg.Mode = "multiuser"
		}
		if err = configureStoragePolicyForMode(server, cfg); err != nil {
			t.Fatal(err)
		}
		want := 1
		if isolated {
			want = 2
		}
		if got := len(server.GetAllEmail()); got != want {
			t.Fatalf("isolated=%t messages=%d want=%d", isolated, got, want)
		}
		if isolated {
			cfg.Mode = "standalone"
			if err = configureStoragePolicyForMode(server, cfg); err == nil {
				t.Fatal("mismatched mode could evict personal data")
			}
		}
		if err = server.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
