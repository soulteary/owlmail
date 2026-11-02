package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/smtp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/api"
	"github.com/soulteary/owlmail/internal/identity"
	"github.com/soulteary/owlmail/internal/mailbox"
	"github.com/soulteary/owlmail/internal/types"
)

// This component test uses synthetic trusted-proxy assertions. It does not
// bypass the public CLI gate or assert that Stargate verifies email today.
func TestMultiuserSMTPToHTTPClaimAndIsolation(t *testing.T) {
	cfg := multiuserComponentConfig(t)
	cfg.SMTPPort = 0
	cfg.SMTPHost = "127.0.0.1"
	server, err := createMailServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	ready := make(chan struct{})
	smtpDone := make(chan error, 1)
	go func() { smtpDone <- server.ListenWithReady(func() { close(ready) }) }()
	select {
	case <-ready:
	case err := <-smtpDone:
		t.Fatal(err)
	case <-time.After(10 * time.Second):
		t.Fatal("SMTP did not bind")
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(server.GetPort())), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	client, err := smtp.NewClient(conn, "localhost")
	if err != nil {
		_ = conn.Close()
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()
	// The listener creates a self-signed certificate for this loopback test.
	if err = client.StartTLS(&tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}); err != nil {
		t.Fatal(err)
	}
	if err = client.Auth(smtp.PlainAuth("", "gorge", "component-test-secret", "localhost")); err != nil {
		t.Fatal(err)
	}
	if err = client.Mail("phorge@example.com"); err != nil {
		t.Fatal(err)
	}
	for _, address := range []string{"alice@example.com", "bob@example.com"} {
		if err = client.Rcpt(address); err != nil {
			t.Fatal(err)
		}
	}
	writer, err := client.Data()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = io.WriteString(writer, "From: phorge@example.com\r\nTo: alice@example.com, bob@example.com\r\nSubject: integration\r\n\r\nnotification"); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err = client.Quit(); err != nil {
		t.Fatal(err)
	}
	proxy, err := identity.NewProxy("main", []string{"127.0.0.1/32"}, "X-Auth-User", "X-Auth-Email", "X-Auth-Email-Verified")
	if err != nil {
		t.Fatal(err)
	}
	web, err := api.NewMultiuserAPIWithHTTPS(server, 0, "127.0.0.1", false, "", "", api.MultiuserOptions{Proxy: proxy, Store: server.MailboxControl(), ClaimPolicy: mailbox.VerifiedEmail})
	if err != nil {
		t.Fatal(err)
	}
	if err = server.AddCloser(web); err != nil {
		t.Fatal(err)
	}
	webReady := make(chan struct{})
	webDone := make(chan error, 1)
	go func() { webDone <- web.StartWithReady(func() { close(webReady) }) }()
	select {
	case <-webReady:
	case err := <-webDone:
		t.Fatal(err)
	case <-time.After(10 * time.Second):
		t.Fatal("HTTP did not bind")
	}
	httpClient := &http.Client{Timeout: 5 * time.Second}
	base := "http://" + net.JoinHostPort("127.0.0.1", strconv.Itoa(web.GetPort()))
	request := func(user, method, path string, verified bool) (int, []byte) {
		t.Helper()
		req, err := http.NewRequestWithContext(context.Background(), method, base+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		if user != "" {
			req.Header.Set("X-Auth-User", user)
			req.Header.Set("X-Auth-Email", user+"@example.com")
			req.Header.Set("X-Auth-Email-Verified", strconv.FormatBool(verified))
		}
		response, err := httpClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = response.Body.Close() }()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		return response.StatusCode, body
	}
	list := func(user string, verified bool) []*types.Email {
		t.Helper()
		status, body := request(user, "GET", "/api/v1/emails", verified)
		if status != 200 {
			t.Fatalf("list %s: %d %s", user, status, body)
		}
		var result struct {
			Emails []*types.Email `json:"emails"`
			Total  int            `json:"total"`
		}
		if err = json.Unmarshal(body, &result); err != nil {
			t.Fatal(err)
		}
		if result.Total != len(result.Emails) {
			t.Fatal("inconsistent personal total")
		}
		return result.Emails
	}
	if emails := list("alice", false); len(emails) != 0 {
		t.Fatal("unverified assertion claimed pending mail")
	}
	alice := list("alice", true)
	bob := list("bob", true)
	if len(alice) != 1 || len(bob) != 1 || alice[0].ID == bob[0].ID {
		t.Fatal("SMTP copies did not claim independently")
	}
	if status, _ := request("alice", "GET", "/api/v1/emails/"+bob[0].ID, true); status != 404 {
		t.Fatal("cross-user ID readable")
	}
	status, body := request("alice", "GET", "/api/v1/emails/stats", true)
	if status != 200 || !strings.Contains(string(body), `"total":1`) {
		t.Fatalf("wrong personal stats: %d %s", status, body)
	}
	for _, path := range []string{"/email", "/config", "/mcp", "/api/v1/ws", "/socket.io"} {
		if status, _ := request("alice", "GET", path, true); status != 404 {
			t.Fatalf("global exit %s status %d", path, status)
		}
	}
	if status, _ := request("alice", "DELETE", "/api/v1/emails", true); status != 200 {
		t.Fatal("personal cleanup failed")
	}
	if len(list("alice", true)) != 0 || len(list("bob", true)) != 1 {
		t.Fatal("personal cleanup leaked across users")
	}
	if err = server.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-smtpDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("SMTP shutdown blocked")
	}
	select {
	case err := <-webDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("HTTP shutdown blocked")
	}
}
