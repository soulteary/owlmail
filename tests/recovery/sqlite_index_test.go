package recovery

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/mailbox"
	"github.com/soulteary/owlmail/internal/mailserver"
)

// This environment variable belongs only to the test binary, enabling the same
// test against a container's mounted volume without adding a production switch.
func sqliteRecoveryRoot(t *testing.T) string {
	t.Helper()
	if base := os.Getenv("OWLMAIL_SQLITE_RECOVERY_ROOT"); base != "" {
		root, err := os.MkdirTemp(base, "sqlite-recovery-") //nolint:usetesting // Explicit mounted-volume placement is required for this test-only runner.
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := os.RemoveAll(root); err != nil {
				t.Error(err)
			}
		})
		return root
	}
	return t.TempDir()
}

func openSQLiteRecovery(t *testing.T, root string, create bool) *backupDeployment {
	t.Helper()
	control, err := mailbox.Open(filepath.Join(root, "control.sqlite"), create)
	if err != nil {
		t.Fatal(err)
	}
	accounts, err := mailbox.LoadAccounts(filepath.Join(root, "accounts.yaml"), "recovery", []string{"example.test"})
	if err != nil {
		t.Fatal(err)
	}
	index, err := mailserver.NewSQLiteMailboxIndex(filepath.Join(root, "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	server, err := mailserver.NewMailServerWithOptions(0, "127.0.0.1", filepath.Join(root, "mail"), mailserver.ServerOptions{
		RequireOwnership: true, AllowEphemeralPort: true, AuthRequireTLS: true, MailboxIndex: index,
		TLSConfig:     &mailserver.TLSConfig{Enabled: true, CertFile: filepath.Join(root, "smtp-cert.pem"), KeyFile: filepath.Join(root, "smtp-key.pem")},
		RoutedIngress: &mailserver.RoutedIngressOptions{Accounts: accounts, Control: control, MaxPending: 20, MaxCreatesPerMinute: 1000, MaxAuthPerMinute: 1000, MaxMessages: 1},
	})
	if err != nil {
		_ = index.Close()
		_ = control.Close()
		t.Fatal(err)
	}
	cert, err := os.ReadFile(filepath.Join(root, "smtp-cert.pem"))
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(cert) {
		t.Fatal("invalid fixture certificate")
	}
	d := &backupDeployment{server: server, control: control, tlsRoots: roots}
	t.Cleanup(func() {
		if d.server != nil {
			d.stop(t)
		}
	})
	return d
}

func assertSQLiteReady(t *testing.T, server *mailserver.MailServer) {
	t.Helper()
	status, ok := server.GetEmailStats()["index"].(map[string]interface{})
	if !ok || status["enabled"] != true || status["ready"] != true || status["backend"] != "sqlite" {
		t.Fatalf("SQLite silently unavailable: %#v", status)
	}
}

func assertDerivedSQLiteRows(t *testing.T, root string, ids map[string]string, received map[string]time.Time) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.Join(root, "index.db")+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	}()
	rows, err := db.Query("SELECT id,is_read,received_at_seconds,received_at_nanoseconds FROM mailbox_index_v2")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			t.Error(err)
		}
	}()
	seen := map[string]bool{}
	for rows.Next() {
		var id string
		var read int
		var sec, nano int64
		if err = rows.Scan(&id, &read, &sec, &nano); err != nil {
			t.Fatal(err)
		}
		expectedRead := 0
		if id == ids["alice"] {
			expectedRead = 1
		}
		at, ok := received[id]
		if !ok || seen[id] || read != expectedRead || !time.Unix(sec, nano).Equal(at) {
			t.Fatalf("wrong derived SQLite row id=%s read=%d received=%d/%d", id, read, sec, nano)
		}
		seen[id] = true
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 {
		t.Fatalf("derived SQLite rows missing: %v", seen)
	}
}

func TestMultiuserSQLiteIndexDeleteAndRebuild(t *testing.T) {
	ctx := context.Background()
	root := sqliteRecoveryRoot(t)
	writeBackupCredentials(t, root)
	d := openSQLiteRecovery(t, root, true)
	d.listen(t)
	owners := map[string]string{}
	ids := map[string]string{}
	received := map[string]time.Time{}
	rawHash := map[string][32]byte{}
	for _, user := range []string{"alice", "bob"} {
		if err := d.deliver(user+"@example.test", user); err != nil {
			t.Fatal(err)
		}
		login, err := d.control.BindLogin(ctx, mailbox.Login{Namespace: "recovery", Subject: user, Email: user + "@example.test", Verified: true}, mailbox.VerifiedEmail)
		if err != nil {
			t.Fatal(err)
		}
		owners[user] = login.MailboxID
	}
	for _, email := range d.server.GetAllEmail() {
		ids[email.Subject] = email.ID
		_, at, err := d.server.GetEmailWithReceivedAt(email.ID)
		if err != nil {
			t.Fatal(err)
		}
		received[email.ID] = at
		raw, err := os.ReadFile(filepath.Join(root, "mail", email.ID+".eml"))
		if err != nil {
			t.Fatal(err)
		}
		rawHash[email.ID] = sha256.Sum256(raw)
	}
	aliceView, err := d.server.MailboxView(owners["alice"])
	if err != nil {
		t.Fatal(err)
	}
	if _, err = aliceView.Read([]string{ids["alice"]}); err != nil {
		t.Fatal(err)
	}
	quotaAt := time.Unix(2_000_000_000, 0)
	if err = d.control.ConsumeAttempt(ctx, "rcpt", "recovery", "sqlite-budget", 1, quotaAt); err != nil {
		t.Fatal(err)
	}
	assertSQLiteReady(t, d.server)
	d.stop(t)
	controlBytes, err := os.ReadFile(filepath.Join(root, "control.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	controlHash := sha256.Sum256(controlBytes)
	for cycle := 1; cycle <= 2; cycle++ {
		// The authority database is deliberately outside this exact derived-artifact list.
		for _, suffix := range []string{"", "-wal", "-shm"} {
			err = os.Remove(filepath.Join(root, "index.db"+suffix))
			if err != nil && (suffix == "" || !errors.Is(err, os.ErrNotExist)) {
				t.Fatal(err)
			}
		}
		authority, readErr := os.ReadFile(filepath.Join(root, "control.sqlite"))
		if readErr != nil || sha256.Sum256(authority) != controlHash {
			t.Fatal("index deletion modified authority database", readErr)
		}
		d = openSQLiteRecovery(t, root, false)
		assertSQLiteReady(t, d.server)
		assertDerivedSQLiteRows(t, root, ids, received)
		if len(d.server.GetAllEmail()) != 2 {
			t.Fatal("rebuild lost or duplicated IDs")
		}
		for _, user := range []string{"alice", "bob"} {
			owner, lookupErr := d.control.PersonalMailbox(ctx, "recovery", user)
			if lookupErr != nil || owner != owners[user] {
				t.Fatal("stable identity binding changed", lookupErr)
			}
			view, viewErr := d.server.MailboxView(owner)
			if viewErr != nil {
				t.Fatal(viewErr)
			}
			list, total := view.List(mailserver.EmailQuery{})
			if total != 1 || len(list) != 1 || list[0].ID != ids[user] || list[0].Read != (user == "alice") {
				t.Fatalf("rebuilt personal list mismatch: %+v total%d", list, total)
			}
			other := "alice"
			if user == "alice" {
				other = "bob"
			}
			if _, viewErr = view.Get(ids[other]); !errors.Is(viewErr, mailserver.ErrEmailNotFound) {
				t.Fatal("cross-mailbox detail exposed", viewErr)
			}
			if _, viewErr = view.Read([]string{ids[user], ids[other]}); !errors.Is(viewErr, mailserver.ErrEmailNotFound) {
				t.Fatal("cross-mailbox bulk read accepted", viewErr)
			}
			email := list[0]
			if len(email.Attachments) != 1 {
				t.Fatal("attachment metadata lost")
			}
			if _, viewErr = view.OpenAttachment(ids[other], email.Attachments[0].GeneratedFileName); !errors.Is(viewErr, mailserver.ErrEmailNotFound) {
				t.Fatal("cross-mailbox attachment exposed", viewErr)
			}
			attachment, attachmentErr := view.OpenAttachment(email.ID, email.Attachments[0].GeneratedFileName)
			if attachmentErr != nil {
				t.Fatal(attachmentErr)
			}
			content, attachmentErr := io.ReadAll(attachment.Body)
			closeErr := attachment.Body.Close()
			if attachmentErr != nil || closeErr != nil || string(content) != "backup attachment "+user {
				t.Fatal("attachment bytes changed", attachmentErr, closeErr)
			}
			_, at, lookupErr := d.server.GetEmailWithReceivedAt(email.ID)
			if lookupErr != nil || !at.Equal(received[email.ID]) {
				t.Fatal("receivedAt changed", lookupErr)
			}
			raw, lookupErr := os.ReadFile(filepath.Join(root, "mail", email.ID+".eml"))
			if lookupErr != nil || sha256.Sum256(raw) != rawHash[email.ID] {
				t.Fatal("raw MIME changed", lookupErr)
			}
		}
		if err = d.control.ConsumeAttempt(ctx, "rcpt", "recovery", "sqlite-budget", 1, quotaAt); !errors.Is(err, mailbox.ErrAttemptQuota) {
			t.Fatal("persistent admission budget reset", err)
		}
		d.listen(t)
		for _, user := range []string{"alice", "bob"} {
			if err = d.deliver(user+"@example.test", "over quota"); err == nil {
				t.Fatal("rebuild bypassed mailbox quota")
			}
		}
		assertSQLiteReady(t, d.server)
		d.stop(t)
		controlBytes, err = os.ReadFile(filepath.Join(root, "control.sqlite"))
		if err != nil {
			t.Fatal(err)
		}
		controlHash = sha256.Sum256(controlBytes)
		t.Logf("SQLITE_REBUILD_EVIDENCE cycle=%d backend=sqlite ready=true messages=2 exact_ids=true attachments=true read=true received_at=true authority_binding=true persistent_quota=true cross_mailbox_denied=true root=%s", cycle, root)
	}
	if len(ids) != 2 {
		t.Fatalf("unexpected initial ID set: %v", ids)
	}
}
