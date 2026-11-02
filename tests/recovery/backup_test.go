package recovery

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/smtp"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/mailbox"
	"github.com/soulteary/owlmail/internal/mailserver"
	"golang.org/x/crypto/bcrypt"
)

const backupPassword = "synthetic-backup-system-secret"

func writeBackupCredentials(t *testing.T, root string) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(backupPassword), 10)
	if err != nil {
		t.Fatal(err)
	}
	account := fmt.Sprintf("version: 1\naccounts:\n  - id: gorge\n    username: gorge\n    password-hash: %s\n    namespace: recovery\n    enabled: true\n    allowed-domains: [example.test]\n", hash)
	for name, data := range map[string]string{"accounts.yaml": account, "deployment.yaml": "mode: multiuser\nnamespace: recovery\n# Synthetic offline recovery fixture; production gate unchanged.\n"} {
		if err = os.WriteFile(filepath.Join(root, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "synthetic-backup"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	cert, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"smtp-cert.pem": pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert}), "smtp-key.pem": pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded})} {
		if err = os.WriteFile(filepath.Join(root, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

type backupDeployment struct {
	server   *mailserver.MailServer
	control  *mailbox.Store
	done     chan error
	tlsRoots *x509.CertPool
}

func openBackupDeployment(t *testing.T, root string, create bool) *backupDeployment {
	t.Helper()
	control, err := mailbox.Open(filepath.Join(root, "control.sqlite"), create)
	if err != nil {
		t.Fatal(err)
	}
	accounts, err := mailbox.LoadAccounts(filepath.Join(root, "accounts.yaml"), "recovery", []string{"example.test"})
	if err != nil {
		t.Fatal(err)
	}
	server, err := mailserver.NewMailServerWithOptions(0, "127.0.0.1", filepath.Join(root, "mail"), mailserver.ServerOptions{RequireOwnership: true, AllowEphemeralPort: true, AuthRequireTLS: true, TLSConfig: &mailserver.TLSConfig{Enabled: true, CertFile: filepath.Join(root, "smtp-cert.pem"), KeyFile: filepath.Join(root, "smtp-key.pem")}, RoutedIngress: &mailserver.RoutedIngressOptions{Accounts: accounts, Control: control, MaxPending: 20, MaxCreatesPerMinute: 1000, MaxAuthPerMinute: 1000, MaxMessages: 1}})
	if err != nil {
		_ = control.Close()
		t.Fatal(err)
	}
	certPEM, err := os.ReadFile(filepath.Join(root, "smtp-cert.pem"))
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(certPEM) {
		t.Fatal("invalid synthetic TLS certificate")
	}
	d := &backupDeployment{server: server, control: control, tlsRoots: roots}
	t.Cleanup(func() {
		if d.server != nil {
			d.stop(t)
		}
	})
	return d
}
func (d *backupDeployment) listen(t *testing.T) {
	t.Helper()
	ready := make(chan struct{})
	d.done = make(chan error, 1)
	go func() { d.done <- d.server.ListenWithReady(func() { close(ready) }) }()
	select {
	case <-ready:
	case err := <-d.done:
		t.Fatal(err)
	case <-time.After(10 * time.Second):
		t.Fatal("SMTP startup timeout")
	}
}
func (d *backupDeployment) stop(t *testing.T) {
	t.Helper()
	if err := d.server.Close(); err != nil {
		t.Fatal(err)
	}
	if d.done != nil {
		select {
		case <-d.done:
		case <-time.After(10 * time.Second):
			t.Fatal("SMTP stop timeout")
		}
	}
	if err := d.control.Close(); err != nil {
		t.Fatal(err)
	}
	d.server = nil
}
func (d *backupDeployment) deliver(recipient, subject string) error {
	address := fmt.Sprintf("127.0.0.1:%d", d.server.GetPort())
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	if err = conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	client, err := smtp.NewClient(conn, "127.0.0.1")
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	// Trust only this synthetic deployment certificate, restored with the backup.
	if err = client.StartTLS(&tls.Config{RootCAs: d.tlsRoots, ServerName: "127.0.0.1", MinVersion: tls.VersionTLS12}); err != nil {
		return err
	}
	if err = client.Auth(smtp.PlainAuth("", "gorge", backupPassword, "127.0.0.1")); err != nil {
		return err
	}
	if err = client.Mail("sender@example.test"); err != nil {
		return err
	}
	if err = client.Rcpt(recipient); err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(writer, "From: sender@example.test\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=recovery\r\n\r\n--recovery\r\nContent-Type: text/plain\r\n\r\nprivate %s\r\n--recovery\r\nContent-Type: text/plain\r\nContent-Disposition: attachment; filename=restore.txt\r\n\r\nbackup attachment %s\r\n--recovery--\r\n", recipient, subject, subject, subject)
	if err != nil {
		return err
	}
	return writer.Close()
}
func TestStoppedBackupRestoreWithPendingClaim(t *testing.T) {
	ctx := context.Background()
	source := filepath.Join(t.TempDir(), "source")
	if err := os.Mkdir(source, 0700); err != nil {
		t.Fatal(err)
	}
	writeBackupCredentials(t, source)
	original := openBackupDeployment(t, source, true)
	alice, err := original.control.BindLogin(ctx, mailbox.Login{Namespace: "recovery", Subject: "alice"}, mailbox.VerifiedEmail)
	if err != nil {
		t.Fatal(err)
	}
	original.listen(t)
	for _, user := range []string{"alice", "bob"} {
		if err = original.deliver(user+"@example.test", user); err != nil {
			t.Fatal(err)
		}
	}
	originalReceived := map[string]time.Time{}
	for _, email := range original.server.GetAllEmail() {
		_, captured, e := original.server.GetEmailWithReceivedAt(email.ID)
		if e != nil {
			t.Fatal(e)
		}
		originalReceived[email.ID] = captured
		if email.Subject == "alice" {
			view, e := original.server.MailboxView(email.Ownership.MailboxID)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = view.Read([]string{email.ID}); e != nil {
				t.Fatal(e)
			}
		}
	}
	claim, err := original.control.BindLogin(ctx, mailbox.Login{Namespace: "recovery", Subject: "alice", Email: "alice@example.test", Verified: true}, mailbox.VerifiedEmail)
	if err != nil || claim.MergeID == "" {
		t.Fatalf("expected durable pending merge: %+v %v", claim, err)
	}
	quotaAt := time.Unix(2_000_000_000, 0)
	if err = original.control.ConsumeAttempt(ctx, "rcpt", "recovery", "backup-budget", 1, quotaAt); err != nil {
		t.Fatal(err)
	}
	original.stop(t) // Listener and control DB are closed before the snapshot.
	sourceHash, err := backupInventory(source)
	if err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(t.TempDir(), "immutable-backup")
	copyStoppedDeployment(t, source, backup)
	backupHash, err := backupInventory(backup)
	if err != nil {
		t.Fatal(err)
	}
	restoredPath := filepath.Join(t.TempDir(), "restored")
	if err = verifyBackupManifest(backup); err != nil {
		t.Fatal(err)
	}
	copyStoppedDeployment(t, backup, restoredPath)
	if err = verifyBackupManifest(restoredPath); err != nil {
		t.Fatal(err)
	}
	restored := openBackupDeployment(t, restoredPath, false)
	pending, err := restored.control.PendingMerges(ctx)
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending merge lost: %v %v", pending, err)
	}
	cursor := restored.server.DeliveryCursor()
	if err = restored.control.RunPendingMerges(ctx, func(ctx context.Context, m mailbox.Merge) error {
		return restored.server.MergeOwnership(ctx, m.SourceID, m.TargetID)
	}); err != nil {
		t.Fatal(err)
	}
	if err = restored.control.RunPendingMerges(ctx, func(ctx context.Context, m mailbox.Merge) error {
		return restored.server.MergeOwnership(ctx, m.SourceID, m.TargetID)
	}); err != nil {
		t.Fatal("repeat recovery was not idempotent", err)
	}
	if restored.server.DeliveryCursor() != cursor {
		t.Fatal("restore replayed historical delivery")
	}
	pending, err = restored.control.PendingMerges(ctx)
	if err != nil || len(pending) != 0 {
		t.Fatalf("merge not completed: %v %v", pending, err)
	}
	if err = restored.control.ConsumeAttempt(ctx, "rcpt", "recovery", "backup-budget", 1, quotaAt); !errors.Is(err, mailbox.ErrAttemptQuota) {
		t.Fatalf("persisted quota reset: %v", err)
	}
	bob, err := restored.control.BindLogin(ctx, mailbox.Login{Namespace: "recovery", Subject: "bob", Email: "bob@example.test", Verified: true}, mailbox.VerifiedEmail)
	if err != nil {
		t.Fatal(err)
	}
	aliceView, err := restored.server.MailboxView(alice.MailboxID)
	if err != nil {
		t.Fatal(err)
	}
	bobView, err := restored.server.MailboxView(bob.MailboxID)
	if err != nil {
		t.Fatal(err)
	}
	a, na := aliceView.List(mailserver.EmailQuery{})
	b, nb := bobView.List(mailserver.EmailQuery{})
	if na != 1 || nb != 1 || len(a) != 1 || len(b) != 1 || a[0].Subject != "alice" || b[0].Subject != "bob" {
		t.Fatal("restored identity isolation/count mismatch")
	}
	if !a[0].Read || b[0].Read {
		t.Fatal("restored read state changed")
	}
	for _, email := range append(a, b...) {
		_, captured, e := restored.server.GetEmailWithReceivedAt(email.ID)
		if e != nil || !captured.Equal(originalReceived[email.ID]) {
			t.Fatalf("receivedAt changed: %v", e)
		}
	}
	if _, err = aliceView.Get(b[0].ID); !errors.Is(err, mailserver.ErrEmailNotFound) {
		t.Fatalf("cross-user mail exposed: %v", err)
	}
	if len(a[0].Attachments) != 1 {
		t.Fatal("attachment lost")
	}
	attachment, err := aliceView.OpenAttachment(a[0].ID, a[0].Attachments[0].GeneratedFileName)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(attachment.Body)
	closeErr := attachment.Body.Close()
	if err != nil || closeErr != nil || string(body) != "backup attachment alice" {
		t.Fatalf("attachment bytes changed %q %v %v", body, err, closeErr)
	}
	restored.listen(t)
	if err = restored.deliver("alice@example.test", "must reject full box"); err == nil {
		t.Fatal("restored mailbox quota did not reject second message")
	}
	restored.stop(t)
	// A failed restore is discarded; rollback uses a fresh copy of the immutable
	// whole snapshot, never a downgraded binary opening partially restored state.
	for _, mutation := range []string{"tamper", "missing"} {
		damaged := filepath.Join(t.TempDir(), mutation)
		copyStoppedDeployment(t, backup, damaged)
		path := filepath.Join(damaged, "smtp-key.pem")
		if mutation == "tamper" {
			err = os.WriteFile(path, []byte("corrupted synthetic key"), 0600)
		} else {
			err = os.Remove(path)
		}
		if err != nil {
			t.Fatal(err)
		}
		if err = verifyBackupManifest(damaged); err == nil {
			t.Fatal("damaged snapshot passed pre-start manifest check")
		}
	}
	missing := filepath.Join(t.TempDir(), "missing-control")
	copyStoppedDeployment(t, backup, missing)
	if err = os.Remove(filepath.Join(missing, "control.sqlite")); err != nil {
		t.Fatal(err)
	}
	if control, e := mailbox.Open(filepath.Join(missing, "control.sqlite"), false); e == nil {
		_ = control.Close()
		t.Fatal("missing control opened")
	}
	if _, err = os.Stat(filepath.Join(missing, "control.sqlite")); !os.IsNotExist(err) {
		t.Fatal("missing control recreated")
	}
	rollback := filepath.Join(t.TempDir(), "rollback")
	copyStoppedDeployment(t, backup, rollback)
	if err = verifyBackupManifest(rollback); err != nil {
		t.Fatal(err)
	}
	rolled := openBackupDeployment(t, rollback, false)
	if err = rolled.control.RunPendingMerges(ctx, func(ctx context.Context, m mailbox.Merge) error {
		return rolled.server.MergeOwnership(ctx, m.SourceID, m.TargetID)
	}); err != nil {
		t.Fatal(err)
	}
	rolled.stop(t)
	assertBackupUnchanged(t, source, sourceHash)
	assertBackupUnchanged(t, backup, backupHash)
	t.Log("PASS: stop-write snapshot, new-directory restore, pending merge, claim, isolation, attachment, durable quota, failclosed missing control, immutable rollback")
}
