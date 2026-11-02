package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/api"
	"github.com/soulteary/owlmail/internal/identity"
	"github.com/soulteary/owlmail/internal/mailbox"
	"github.com/soulteary/owlmail/internal/mailserver"
	"github.com/soulteary/owlmail/internal/types"
)

// These component tests use synthetic trusted-proxy assertions and real
// loopback SMTP/HTTP listeners. They do not open the public multiuser CLI gate
// or claim production Stargate verification or address-history continuity.
type explicitHistoryFixture struct {
	server *mailserver.MailServer
	base   string
	client *http.Client
}

func newExplicitHistoryFixture(t *testing.T) *explicitHistoryFixture {
	t.Helper()
	cfg := multiuserComponentConfig(t)
	cfg.ClaimPolicy = "explicit-history"
	cfg.SMTPPort, cfg.SMTPHost = 0, "127.0.0.1"
	server, err := createMailServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
	})
	smtpReady, smtpDone := make(chan struct{}), make(chan error, 1)
	go func() { smtpDone <- server.ListenWithReady(func() { close(smtpReady) }) }()
	waitExplicitListener(t, smtpReady, smtpDone)
	proxy, err := identity.NewProxy("main", []string{"127.0.0.1/32"}, "X-Auth-User", "X-Auth-Email", "X-Auth-Email-Verified")
	if err != nil {
		t.Fatal(err)
	}
	web, err := api.NewMultiuserAPIWithHTTPS(server, 0, "127.0.0.1", false, "", "", api.MultiuserOptions{Proxy: proxy, Store: server.MailboxControl(), ClaimPolicy: mailbox.ExplicitHistory})
	if err != nil {
		t.Fatal(err)
	}
	if err = server.AddCloser(web); err != nil {
		t.Fatal(err)
	}
	webReady, webDone := make(chan struct{}), make(chan error, 1)
	go func() { webDone <- web.StartWithReady(func() { close(webReady) }) }()
	waitExplicitListener(t, webReady, webDone)
	transport := &http.Transport{Proxy: nil}
	fixture := &explicitHistoryFixture{server: server, base: "http://" + net.JoinHostPort("127.0.0.1", strconv.Itoa(web.GetPort())), client: &http.Client{Timeout: 5 * time.Second, Transport: transport}}
	t.Cleanup(func() {
		transport.CloseIdleConnections()
		if err := server.Close(); err != nil {
			t.Error(err)
		}
		for _, done := range []chan error{smtpDone, webDone} {
			select {
			case err := <-done:
				if err != nil {
					t.Error(err)
				}
			case <-time.After(5 * time.Second):
				t.Error("component listener did not stop")
			}
		}
	})
	return fixture
}

func waitExplicitListener(t *testing.T, ready <-chan struct{}, done <-chan error) {
	t.Helper()
	select {
	case <-ready:
	case err := <-done:
		t.Fatalf("listener failed: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("listener readiness timeout")
	}
}

func (f *explicitHistoryFixture) request(subject string, verified bool, path string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, f.base+path, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("X-Auth-User", subject)
	req.Header.Set("X-Auth-Email", "recycled@example.com")
	req.Header.Set("X-Auth-Email-Verified", strconv.FormatBool(verified))
	resp, err := f.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	return resp.StatusCode, body, err
}

func (f *explicitHistoryFixture) list(t *testing.T, subject string, verified bool) []*types.Email {
	t.Helper()
	status, body, err := f.request(subject, verified, "/api/v1/emails")
	if err != nil || status != http.StatusOK {
		t.Fatalf("list %s: status=%d err=%v body=%s", subject, status, err, body)
	}
	var result struct {
		Emails []*types.Email `json:"emails"`
		Total  int            `json:"total"`
	}
	if err = json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	if result.Total != len(result.Emails) {
		t.Fatal("inconsistent personal count")
	}
	return result.Emails
}

func (f *explicitHistoryFixture) smtpRCPT(t *testing.T) *smtp.Client {
	t.Helper()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(f.server.GetPort())), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err = conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	client, err := smtp.NewClient(conn, "localhost")
	if err != nil {
		_ = conn.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	// This loopback-only component test encrypts SMTP but skips certificate
	// verification for its generated certificate; it does not verify a CA.
	if err = client.StartTLS(&tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}); err != nil {
		t.Fatal(err)
	}
	if err = client.Auth(smtp.PlainAuth("", "gorge", "component-test-secret", "localhost")); err != nil {
		t.Fatal(err)
	}
	if err = client.Mail("phorge@example.com"); err != nil {
		t.Fatal(err)
	}
	if err = client.Rcpt("recycled@example.com"); err != nil {
		t.Fatal(err)
	}
	return client
}

func explicitHistoryDATA(t *testing.T, client *smtp.Client, subject string) {
	t.Helper()
	writer, err := client.Data()
	if err != nil {
		t.Fatal(err)
	}
	message := "From: phorge@example.com\r\nTo: recycled@example.com\r\nSubject: " + subject + "\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=proof\r\n\r\n--proof\r\nContent-Type: text/plain\r\n\r\nnotification\r\n--proof\r\nContent-Type: application/octet-stream\r\nContent-Disposition: attachment; filename=history.bin\r\n\r\nprivate attachment\r\n--proof--\r\n"
	if _, err = io.WriteString(writer, message); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err = client.Quit(); err != nil {
		t.Fatal(err)
	}
}

func (f *explicitHistoryFixture) history(t *testing.T) *types.Email {
	t.Helper()
	explicitHistoryDATA(t, f.smtpRCPT(t), "old pending history")
	// QueryEmails returns detached deep snapshots, including Ownership slices.
	// Keep that pre-login snapshot to detect later historical owner mutations.
	emails, total := f.server.QueryEmails(mailserver.EmailQuery{Limit: 10})
	if total != 1 || len(emails) != 1 || len(emails[0].Attachments) != 1 || emails[0].Ownership == nil {
		t.Fatalf("history not durably captured: total=%d emails=%v", total, emails)
	}
	filename := emails[0].Attachments[0].GeneratedFileName
	if filename == "" {
		t.Fatal("history attachment has no addressable filename")
	}
	attachment, err := f.server.OpenEmailAttachment(emails[0].ID, filename)
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(attachment.Body)
	closeErr := attachment.Body.Close()
	if readErr != nil || closeErr != nil || strings.TrimSpace(string(body)) != "private attachment" {
		t.Fatalf("history attachment not intact: read=%v close=%v body=%q", readErr, closeErr, body)
	}
	return emails[0]
}

func (f *explicitHistoryFixture) denyHistory(t *testing.T, subject string, history *types.Email) {
	t.Helper()
	for _, suffix := range []string{"", "/raw", "/attachments/" + url.PathEscape(history.Attachments[0].GeneratedFileName)} {
		status, body, err := f.request(subject, true, "/api/v1/emails/"+history.ID+suffix)
		if err != nil || status != http.StatusNotFound {
			t.Fatalf("%s read old history%s: status=%d err=%v body=%s", subject, suffix, status, err, body)
		}
	}
	actual, err := f.server.GetEmail(history.ID)
	if err != nil || actual.Ownership == nil || !reflect.DeepEqual(actual.Ownership, history.Ownership) {
		t.Fatalf("isolated history changed or disappeared: %v", err)
	}
}

func TestExplicitHistorySMTPHTTPKeepsRecycledAddressHistoryPrivate(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(fmt.Sprintf("existing-personal=%t", existing), func(t *testing.T) {
			f := newExplicitHistoryFixture(t)
			if existing && len(f.list(t, "alice", false)) != 0 {
				t.Fatal("new personal mailbox was not empty")
			}
			history := f.history(t)
			// Deterministic protocol boundary: RCPT succeeds against the pending route;
			// login changes only future delivery, then this same transaction sends DATA.
			transaction := f.smtpRCPT(t)
			if got := f.list(t, "alice", true); len(got) != 0 {
				t.Fatal("verified current address exposed pending history")
			}
			f.denyHistory(t, "alice", history)
			personal, err := f.server.MailboxControl().PersonalMailbox(context.Background(), "main", "alice")
			if err != nil || personal == history.Ownership.MailboxID {
				t.Fatalf("historical box became personal: %v", err)
			}
			explicitHistoryDATA(t, transaction, "accepted RCPT before login")
			explicitHistoryDATA(t, f.smtpRCPT(t), "future delivery")
			got := f.list(t, "alice", true)
			if len(got) != 2 {
				t.Fatalf("personal count=%d want two new deliveries", len(got))
			}
			subjects := map[string]bool{}
			for _, message := range got {
				subjects[message.Subject] = true
				if message.ID == history.ID {
					t.Fatal("old history entered personal list")
				}
			}
			if !subjects["accepted RCPT before login"] || !subjects["future delivery"] {
				t.Fatalf("unexpected personal subjects: %v", subjects)
			}
			f.denyHistory(t, "alice", history)
		})
	}
}

func TestExplicitHistoryConcurrentSubjectsCannotAcquirePendingHistory(t *testing.T) {
	f := newExplicitHistoryFixture(t)
	history := f.history(t)
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, subject := range []string{"alice", "bob"} {
		go func(subject string) {
			<-start
			status, body, err := f.request(subject, true, "/api/v1/emails")
			if err == nil && status != http.StatusOK {
				err = fmt.Errorf("%s login status=%d body=%s", subject, status, body)
			}
			if err == nil {
				var result struct {
					Total int `json:"total"`
				}
				err = json.Unmarshal(body, &result)
				if err == nil && result.Total != 0 {
					err = fmt.Errorf("%s acquired old history", subject)
				}
			}
			results <- err
		}(subject)
	}
	close(start)
	for i := 0; i < 2; i++ {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	owners := map[string]string{}
	for _, subject := range []string{"alice", "bob"} {
		id, err := f.server.MailboxControl().PersonalMailbox(context.Background(), "main", subject)
		if err != nil || id == history.Ownership.MailboxID {
			t.Fatalf("%s historical ownership: %v", subject, err)
		}
		owners[subject] = id
		f.denyHistory(t, subject, history)
	}
	if owners["alice"] == owners["bob"] {
		t.Fatal("subjects share a personal mailbox")
	}
	winner := ""
	err := f.server.MailboxControl().WithRouteSnapshot(context.Background(), "main", []string{"recycled@example.com"}, func(routes []mailbox.Route) error {
		for subject, id := range owners {
			if routes[0].MailboxID == id {
				winner = subject
			}
		}
		return nil
	})
	if err != nil || winner == "" {
		t.Fatalf("future route has no unique current owner: %v", err)
	}
	explicitHistoryDATA(t, f.smtpRCPT(t), "after competing verified logins")
	for _, subject := range []string{"alice", "bob"} {
		got := f.list(t, subject, true)
		want := 0
		if subject == winner {
			want = 1
		}
		if len(got) != want {
			t.Fatalf("%s future mail count=%d want=%d", subject, len(got), want)
		}
		f.denyHistory(t, subject, history)
	}
}
