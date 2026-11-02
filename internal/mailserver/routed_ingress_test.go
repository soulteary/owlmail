package mailserver

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
	"github.com/google/uuid"
	"github.com/soulteary/owlmail/internal/mailbox"
	"golang.org/x/crypto/bcrypt"
)

func routedOptions(t *testing.T) (*RoutedIngressOptions, *mailbox.Store) {
	t.Helper()
	control, err := mailbox.Open(filepath.Join(t.TempDir(), "control.sqlite"), true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = control.Close() })
	hash, err := bcrypt.GenerateFromPassword([]byte("system-secret"), 10)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "accounts.yaml")
	data := fmt.Sprintf("version: 1\naccounts:\n  - id: gorge\n    username: gorge\n    password-hash: %s\n    namespace: main\n    enabled: true\n    allowed-domains: [example.com]\n", hash)
	if err = os.WriteFile(file, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	accounts, err := mailbox.LoadAccounts(file, "main", []string{"example.com"})
	if err != nil {
		t.Fatal(err)
	}
	return &RoutedIngressOptions{Accounts: accounts, Control: control, MaxPending: 20, MaxCreatesPerMinute: 100}, control
}
func newRoutedTestServer(t *testing.T, dir string, options *RoutedIngressOptions) *MailServer {
	t.Helper()
	server, err := NewMailServerWithOptions(0, "127.0.0.1", dir, ServerOptions{RequireOwnership: true, AuthRequireTLS: true, TLSConfig: &TLSConfig{Enabled: true}, SMTPSPort: DefaultSMTPSPort, RoutedIngress: options})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	return server
}
func routedTestSession(t *testing.T, server *MailServer, recipients ...string) *Session {
	t.Helper()
	session := &Session{mailServer: server}
	if err := session.authenticate("gorge", "system-secret"); err != nil {
		t.Fatal(err)
	}
	if err := session.Mail("sender@example.com", nil); err != nil {
		t.Fatal(err)
	}
	for _, recipient := range recipients {
		if err := session.Rcpt(recipient, nil); err != nil {
			t.Fatal(err)
		}
	}
	return session
}

const routedMessage = "From: sender@example.com\r\nTo: alice@example.com, bob@example.com\r\nSubject: routed\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=part\r\n\r\n--part\r\nContent-Type: text/plain\r\n\r\nhello\r\n--part\r\nContent-Type: text/plain\r\nContent-Disposition: attachment; filename=test.txt\r\n\r\nattachment\r\n--part--\r\n"

func TestRoutedSMTPImplicitTLSAndRecoveryAfterDeletion(t *testing.T) {
	options, _ := routedOptions(t)
	dir := t.TempDir()
	server := newRoutedTestServer(t, dir, options)
	raw, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	listener := tls.NewListener(raw, server.smtpsServer.TLSConfig)
	done := make(chan error, 1)
	go func() { done <- server.smtpsServer.Serve(listener) }()
	t.Cleanup(func() { _ = listener.Close() })
	plain := dialSMTPTestClientOn(t, server)
	if ok, _ := plain.Extension("AUTH"); ok {
		t.Fatal("plaintext advertised credentials")
	}
	_ = plain.Close()
	client, err := smtp.DialTLS(raw.Addr().String(), insecureSMTPTestTLSConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()
	if err = client.Auth(sasl.NewPlainClient("", "gorge", "system-secret")); err != nil {
		t.Fatal(err)
	}
	if err = client.Mail("sender@example.com", nil); err != nil {
		t.Fatal(err)
	}
	for _, recipient := range []string{"alice@example.com", "bob@example.com"} {
		if err = client.Rcpt(recipient, nil); err != nil {
			t.Fatal(err)
		}
	}
	// Keep the committed journal to model a process stopping after acceptance.
	server.routedIngress.fault = func(stage string, _ int) error {
		if stage == "after-commit" {
			return errors.New("crash")
		}
		return nil
	}
	writer, err := client.Data()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = writer.Write([]byte(routedMessage)); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	emails := server.GetAllEmail()
	if len(emails) != 2 {
		t.Fatalf("got %d replicas", len(emails))
	}
	if emails[0].Ownership.MailboxID == emails[1].Ownership.MailboxID {
		t.Fatal("recipient boxes were shared")
	}
	for _, email := range emails {
		if len(email.Envelope.To) != 1 || len(email.Ownership.MatchedRecipients) != 1 || len(email.Attachments) != 1 {
			t.Fatal("wrong per-recipient projection")
		}
	}
	if err = server.DeleteEmail(emails[0].ID); err != nil {
		t.Fatal(err)
	}
	_ = client.Quit()
	_ = listener.Close()
	_ = server.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SMTP did not close")
	}
	reopened := newRoutedTestServer(t, dir, options)
	recovered := reopened.GetAllEmail()
	if len(recovered) != 1 || recovered[0].ID != emails[1].ID {
		t.Fatal("deleted replica resurrected or accepted replica lost")
	}
}
func dialSMTPTestClientOn(t *testing.T, server *MailServer) *smtp.Client {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = server.smtpServer.Serve(listener) }()
	t.Cleanup(func() { _ = listener.Close() })
	return dialSMTPTestClient(t, listener.Addr().String())
}
func TestRoutedAtomicFailureAndCrashRecovery(t *testing.T) {
	for _, point := range []string{"after-install", "before-commit"} {
		t.Run(point, func(t *testing.T) {
			options, _ := routedOptions(t)
			dir := t.TempDir()
			server := newRoutedTestServer(t, dir, options)
			session := routedTestSession(t, server, "alice@example.com", "bob@example.com")
			server.routedIngress.fault = func(stage string, index int) error {
				if stage == point && (point != "after-install" || index == 0) {
					if got := len(server.GetAllEmail()); got != 0 {
						t.Error("partially installed replicas visible")
					}
					return errors.New("injected failure")
				}
				return nil
			}
			if err := session.Data(strings.NewReader(routedMessage)); err == nil {
				t.Fatal("failure accepted")
			}
			if got := len(server.GetAllEmail()); got != 0 {
				t.Fatalf("published %d rejected replicas", got)
			}
			_ = server.Close()
			reopened := newRoutedTestServer(t, dir, options)
			if len(reopened.GetAllEmail()) != 0 {
				t.Fatal("rejected group recovered as accepted")
			}
		})
	}
	// Simulate an actual process stop: leave a valid prepared journal and one
	// installed EML, rather than letting the normal error path roll it back.
	options, _ := routedOptions(t)
	dir := t.TempDir()
	server := newRoutedTestServer(t, dir, options)
	txID := uuid.NewString()
	id := uuid.NewString()
	stage := filepath.Join(dir, ingressDirectory, txID)
	if err := os.MkdirAll(stage, 0700); err != nil {
		t.Fatal(err)
	}
	if err := writeIngressJournal(stage, routedJournal{Version: 1, TransactionID: txID, IDs: []string{id}}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".eml"), []byte("Subject: partial\r\n\r\nbody"), 0600); err != nil {
		t.Fatal(err)
	}
	_ = server.Close()
	reopened := newRoutedTestServer(t, dir, options)
	if len(reopened.GetAllEmail()) != 0 {
		t.Fatal("prepared transaction leaked on restart")
	}
	if _, err := os.Stat(filepath.Join(dir, id+".eml")); !os.IsNotExist(err) {
		t.Fatal("partial EML not removed")
	}
}
func TestRoutedAliasDedupClaimDuringTransactionAndAuthReset(t *testing.T) {
	options, control := routedOptions(t)
	server := newRoutedTestServer(t, t.TempDir(), options)
	session := routedTestSession(t, server, "alice@example.com", "alias@example.com")
	first, err := control.BindLogin(context.Background(), mailbox.Login{Namespace: "main", Subject: "alice", Email: "alice@example.com", Verified: true}, mailbox.VerifiedEmail)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = control.BindLogin(context.Background(), mailbox.Login{Namespace: "main", Subject: "alice", Email: "alias@example.com", Verified: true}, mailbox.VerifiedEmail); err != nil {
		t.Fatal(err)
	}
	if err = session.Data(strings.NewReader(routedMessage)); err != nil {
		t.Fatal(err)
	}
	emails := server.GetAllEmail()
	if len(emails) != 1 || emails[0].Ownership.MailboxID != first.MailboxID || len(emails[0].Ownership.MatchedRecipients) != 2 {
		t.Fatal("claim during SMTP was not followed or aliases duplicated")
	}
	session.Reset()
	if session.ingestAccount == nil || !session.authenticated || len(session.to) != 0 || session.rcptRoutes != nil {
		t.Fatal("RSET lost authentication or retained transaction")
	}
	if err = session.authenticate("gorge", "wrong"); err == nil || session.ingestAccount != nil || session.authenticated {
		t.Fatal("failed authentication retained prior identity")
	}
	if err = session.Rcpt("alice@example.com", nil); err == nil {
		t.Fatal("unauthenticated RCPT accepted")
	}
	session = routedTestSession(t, server)
	if err = session.Rcpt("other@unallowed.test", nil); err == nil {
		t.Fatal("unallowed recipient accepted")
	}
}

func TestRoutedMessageQuotaIsAtomicAndDeletionReleasesCapacity(t *testing.T) {
	options, _ := routedOptions(t)
	options.MaxMessages = 1
	dir := t.TempDir()
	server := newRoutedTestServer(t, dir, options)
	if err := routedTestSession(t, server, "alice@example.com").Data(strings.NewReader(routedMessage)); err != nil {
		t.Fatal(err)
	}
	first := server.GetAllEmail()[0]
	// Bob is deliberately first: a later full target must not accept or charge
	// the earlier target in the same SMTP transaction.
	if err := routedTestSession(t, server, "bob@example.com", "alice@example.com").Data(strings.NewReader(routedMessage)); err == nil {
		t.Fatal("multi-recipient quota overflow accepted")
	}
	if len(server.GetAllEmail()) != 1 {
		t.Fatal("part of rejected transaction published")
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := newRoutedTestServer(t, dir, options)
	if len(reopened.GetAllEmail()) != 1 {
		t.Fatal("rejected transaction consumed capacity after restart")
	}
	if err := reopened.DeleteEmail(first.ID); err != nil {
		t.Fatal(err)
	}
	if err := routedTestSession(t, reopened, "bob@example.com", "alice@example.com").Data(strings.NewReader(routedMessage)); err != nil {
		t.Fatal("deleted message did not release capacity", err)
	}
	if len(reopened.GetAllEmail()) != 2 {
		t.Fatal("accepted group missing copies")
	}
}
func TestRoutedByteQuotaCountsSourceMetadataAndDecodedAttachment(t *testing.T) {
	options, _ := routedOptions(t)
	dir := t.TempDir()
	server := newRoutedTestServer(t, dir, options)
	if err := routedTestSession(t, server, "alice@example.com").Data(strings.NewReader(routedMessage)); err != nil {
		t.Fatal(err)
	}
	first := server.GetAllEmail()[0]
	used, err := routedDiskUsage(dir, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if used <= first.Size+first.Attachments[0].Size {
		t.Fatal("metadata bytes excluded")
	}
	server.routedIngress.options.MaxDiskBytes = used + 128
	if err = routedTestSession(t, server, "alice@example.com").Data(strings.NewReader(routedMessage)); err == nil {
		t.Fatal("byte quota ignored")
	}
	if len(server.GetAllEmail()) != 1 {
		t.Fatal("byte-rejected message was published")
	}
	if err = server.DeleteEmail(first.ID); err != nil {
		t.Fatal(err)
	}
	// Acceptance after deletion also proves rejected staging is not billed.
	// The small margin allows RFC3339Nano timestamp formatting to vary.
	if err = routedTestSession(t, server, "alice@example.com").Data(strings.NewReader(routedMessage)); err != nil {
		t.Fatal("deleted bytes were still charged", err)
	}
}
func TestRoutedQuotaNoChargeOnPrecommitFailure(t *testing.T) {
	options, _ := routedOptions(t)
	options.MaxMessages = 1
	options.MaxTotalMessages = 2
	options.MaxTotalDiskBytes = 1 << 30
	dir := t.TempDir()
	server := newRoutedTestServer(t, dir, options)
	server.routedIngress.fault = func(stage string, _ int) error {
		if stage == "before-commit" {
			return errors.New("crash")
		}
		return nil
	}
	if err := routedTestSession(t, server, "alice@example.com", "bob@example.com").Data(strings.NewReader(routedMessage)); err == nil {
		t.Fatal("precommit failure accepted")
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := newRoutedTestServer(t, dir, options)
	if err := routedTestSession(t, reopened, "alice@example.com", "bob@example.com").Data(strings.NewReader(routedMessage)); err != nil {
		t.Fatal("failed group charged capacity", err)
	}
}
func TestRoutedQuotaUsageAdditionCannotOverflow(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	maxBytes := int64(^uint64(0) >> 1)
	if _, err := addRoutedUsage(routedUsage{messages: maxInt, bytes: maxBytes}, 1, 1, maxInt, maxBytes); !errors.Is(err, ErrMailboxQuota) {
		t.Fatal("usage overflow was accepted", err)
	}
	if _, err := addRoutedUsage(routedUsage{}, 1, maxBytes, maxInt, maxBytes); err != nil {
		t.Fatal("exact max size rejected", err)
	}
	if _, err := addRoutedUsage(routedUsage{}, 1, -1, maxInt, maxBytes); err == nil {
		t.Fatal("negative size accepted")
	}
}

func TestRoutedAttemptBudgetsSurviveIngressRestart(t *testing.T) {
	for _, kind := range []string{"auth", "rcpt"} {
		t.Run(kind, func(t *testing.T) {
			options, _ := routedOptions(t)
			if kind == "auth" {
				options.MaxAuthPerMinute = 1
			} else {
				options.MaxCreatesPerMinute = 1
			}
			dir := t.TempDir()
			server := newRoutedTestServer(t, dir, options)
			fixed := time.Unix(1800000000, 0)
			clock := func() time.Time { return fixed }
			server.routedIngress.clock = clock
			first := routedTestSession(t, server, "alice@example.com")
			first.Reset()
			if err := server.Close(); err != nil {
				t.Fatal(err)
			}
			reopened := newRoutedTestServer(t, dir, options)
			reopened.routedIngress.clock = clock
			session := &Session{mailServer: reopened}
			err := session.authenticate("gorge", "system-secret")
			if kind == "auth" {
				if err == nil {
					t.Fatal("authentication budget reset after restart")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = session.Mail("sender@example.com", nil); err != nil {
				t.Fatal(err)
			}
			if err = session.Rcpt("bob@example.com", nil); err == nil {
				t.Fatal("abandoned RCPT budget reset after restart")
			}
		})
	}
}

func TestRoutedConcurrentQuotaAdmission(t *testing.T) {
	options, _ := routedOptions(t)
	options.MaxMessages = 1
	server := newRoutedTestServer(t, t.TempDir(), options)
	sessions := []*Session{routedTestSession(t, server, "alice@example.com"), routedTestSession(t, server, "alice@example.com")}
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for _, session := range sessions {
		wg.Go(func() {
			if err := session.Data(strings.NewReader(routedMessage)); err == nil {
				accepted.Add(1)
			} else {
				var smtpErr *smtp.SMTPError
				if !errors.As(err, &smtpErr) || smtpErr.Code != 451 {
					t.Error(err)
				}
			}
		})
	}
	wg.Wait()
	if accepted.Load() != 1 || len(server.GetAllEmail()) != 1 {
		t.Fatalf("accepted %d messages, stored %d", accepted.Load(), len(server.GetAllEmail()))
	}
}

func TestRoutedGlobalMessageQuotaAcrossBoxesAndRestart(t *testing.T) {
	options, _ := routedOptions(t)
	options.MaxMessages = 10
	options.MaxTotalMessages = 2
	dir := t.TempDir()
	server := newRoutedTestServer(t, dir, options)
	if err := routedTestSession(t, server, "alice@example.com").Data(strings.NewReader(routedMessage)); err != nil {
		t.Fatal(err)
	}
	first := server.GetAllEmail()[0]
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := newRoutedTestServer(t, dir, options)
	if err := routedTestSession(t, reopened, "bob@example.com", "carol@example.com").Data(strings.NewReader(routedMessage)); err == nil {
		t.Fatal("cross-box global quota ignored after restart")
	}
	if len(reopened.GetAllEmail()) != 1 {
		t.Fatal("global quota allowed a partial group or deleted another user")
	}
	if err := reopened.DeleteEmail(first.ID); err != nil {
		t.Fatal(err)
	}
	if err := routedTestSession(t, reopened, "bob@example.com", "carol@example.com").Data(strings.NewReader(routedMessage)); err != nil {
		t.Fatal("global capacity was not released after deletion", err)
	}
	if len(reopened.GetAllEmail()) != 2 {
		t.Fatal("global capacity accepted group was incomplete")
	}
}
func TestRoutedGlobalByteQuotaIncludesOtherBoxes(t *testing.T) {
	options, _ := routedOptions(t)
	dir := t.TempDir()
	server := newRoutedTestServer(t, dir, options)
	if err := routedTestSession(t, server, "alice@example.com").Data(strings.NewReader(routedMessage)); err != nil {
		t.Fatal(err)
	}
	first := server.GetAllEmail()[0]
	used, err := routedDiskUsage(dir, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	options.MaxTotalDiskBytes = used + 128
	if err = server.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := newRoutedTestServer(t, dir, options)
	if err = routedTestSession(t, reopened, "bob@example.com").Data(strings.NewReader(routedMessage)); err == nil {
		t.Fatal("other mailbox disk use excluded from global quota")
	}
	if len(reopened.GetAllEmail()) != 1 {
		t.Fatal("global byte refusal evicted or published mail")
	}
	if err = reopened.DeleteEmail(first.ID); err != nil {
		t.Fatal(err)
	}
	if err = routedTestSession(t, reopened, "bob@example.com").Data(strings.NewReader(routedMessage)); err != nil {
		t.Fatal("global bytes did not release after delete", err)
	}
}
