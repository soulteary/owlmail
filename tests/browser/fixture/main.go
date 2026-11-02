//go:build owlmail_browser_fixture

// This command exists only for browser regression tests. It is excluded from
// normal builds and never changes the public multiuser release gate. Its
// loopback proxy issues synthetic assertions; it is not a Stargate deployment.
package main

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httputil"
	"net/smtp"
	"net/textproto"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/soulteary/owlmail/internal/api"
	"github.com/soulteary/owlmail/internal/identity"
	"github.com/soulteary/owlmail/internal/mailbox"
	"github.com/soulteary/owlmail/internal/mailserver"
	"golang.org/x/crypto/bcrypt"
)

const fixtureAddress = "127.0.0.1:21180"
const fixturePassword = "synthetic-browser-fixture-secret"

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
func run() error {
	state := flag.String("state-directory", "", "Explicit persistent test fixture state directory (test build only)")
	listen := flag.String("listen", fixtureAddress, "Loopback-only test proxy address")
	certPath := flag.String("tls-cert", "", "Persistent SMTP certificate inside test state directory")
	keyPath := flag.String("tls-key", "", "Persistent SMTP key inside test state directory")
	caPath := flag.String("tls-ca", "", "Dedicated SMTP client trust CA inside test state directory")
	flag.Parse()
	host, _, err := net.SplitHostPort(*listen)
	if err != nil || host != "127.0.0.1" {
		return fmt.Errorf("fixture listen address must bind 127.0.0.1")
	}
	root := *state
	if root == "" {
		root, err = os.MkdirTemp("", "owlmail-multiuser-browser-")
		if err != nil {
			return err
		}
		defer func() { _ = os.RemoveAll(root) }()
	} else {
		if !filepath.IsAbs(root) {
			return fmt.Errorf("fixture state directory must be absolute")
		}
		if err = os.MkdirAll(root, 0700); err != nil {
			return err
		}
	}
	serverTLS, clientTLS, err := configuredFixtureTLS(root, *certPath, *keyPath, *caPath)
	if err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(fixturePassword), 10)
	if err != nil {
		return err
	}
	accountPath := filepath.Join(root, "accounts.yaml")
	content := fmt.Sprintf("version: 1\naccounts:\n  - id: fixture-gorge\n    username: gorge\n    password-hash: %s\n    namespace: browser\n    enabled: true\n    allowed-domains: [example.test]\n", hash)
	if err = os.WriteFile(accountPath, []byte(content), 0600); err != nil {
		return err
	}
	accounts, err := mailbox.LoadAccounts(accountPath, "browser", []string{"example.test"})
	if err != nil {
		return err
	}
	entries, readErr := os.ReadDir(filepath.Join(root, "mail"))
	create := os.IsNotExist(readErr) || (readErr == nil && len(entries) == 0)
	if readErr != nil && !os.IsNotExist(readErr) {
		return readErr
	}
	control, err := mailbox.Open(filepath.Join(root, "control.sqlite"), create)
	if err != nil {
		return err
	}
	defer func() { _ = control.Close() }()
	server, err := mailserver.NewMailServerWithOptions(0, "127.0.0.1", filepath.Join(root, "mail"), mailserver.ServerOptions{
		RequireOwnership: true, AllowEphemeralPort: true, AuthRequireTLS: true, TLSConfig: serverTLS,
		RoutedIngress: &mailserver.RoutedIngressOptions{Accounts: accounts, Control: control, MaxPending: 100, MaxCreatesPerMinute: 1000, MaxAuthPerMinute: 1000},
	})
	if err != nil {
		return err
	}
	defer func() { _ = server.Close() }()
	if err = control.RunPendingMerges(context.Background(), func(ctx context.Context, item mailbox.Merge) error {
		return server.MergeOwnership(ctx, item.SourceID, item.TargetID)
	}); err != nil {
		return err
	}
	ready := make(chan struct{})
	smtpDone := make(chan error, 1)
	go func() { smtpDone <- server.ListenWithReady(func() { close(ready) }) }()
	select {
	case <-ready:
	case err = <-smtpDone:
		return err
	case <-time.After(15 * time.Second):
		return fmt.Errorf("fixture SMTP startup timeout")
	}
	proxyIdentity, err := identity.NewProxy("browser", []string{"127.0.0.1/32"}, "X-Auth-User", "X-Auth-Email", "X-Auth-Email-Verified")
	if err != nil {
		return err
	}
	backend, err := api.NewMultiuserAPIWithHTTPS(server, 0, "127.0.0.1", false, "", "", api.MultiuserOptions{Proxy: proxyIdentity, Store: control, ClaimPolicy: mailbox.VerifiedEmail})
	if err != nil {
		return err
	}
	if err = backend.SetWebAllowedOrigins([]string{"http://" + *listen}); err != nil {
		return err
	}
	defer func() { _ = backend.Close() }()
	httpReady := make(chan struct{})
	httpDone := make(chan error, 1)
	go func() { httpDone <- backend.StartWithReady(func() { close(httpReady) }) }()
	select {
	case <-httpReady:
	case err = <-httpDone:
		return err
	case <-time.After(15 * time.Second):
		return fmt.Errorf("fixture HTTP startup timeout")
	}
	target, _ := url.Parse("http://127.0.0.1:" + strconv.Itoa(backend.GetPort()))
	reverse := &httputil.ReverseProxy{}
	reverse.Rewrite = func(proxyRequest *httputil.ProxyRequest) {
		proxyRequest.SetURL(target)
		r := proxyRequest.Out
		for _, name := range []string{"X-Auth-User", "X-Auth-Email", "X-Auth-Email-Verified"} {
			r.Header.Del(name)
		}
		user := "alice"
		if cookie, err := r.Cookie("fixture-user"); err == nil {
			user = cookie.Value
		}
		if user == "alice" || user == "bob" {
			r.Header.Set("X-Auth-User", user)
			r.Header.Set("X-Auth-Email", user+"@example.test")
			verified := "true"
			if cookie, err := r.Cookie("fixture-verified"); err == nil && cookie.Value == "false" {
				verified = "false"
			}
			r.Header.Set("X-Auth-Email-Verified", verified)
		}
	}
	var previewRequests atomic.Int64
	mux := http.NewServeMux()
	mux.HandleFunc("/__fixture__/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "synthetic loopback browser fixture")
	})
	mux.HandleFunc("/__fixture__/stats", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]int64{"previews": previewRequests.Load()})
	})
	mux.HandleFunc("/__fixture__/tls", func(w http.ResponseWriter, _ *http.Request) {
		if clientTLS.InsecureSkipVerify {
			http.Error(w, "persistent verified TLS is not configured", http.StatusConflict)
			return
		}
		client, state, err := fixtureSMTP(server.GetPort(), clientTLS)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer func() { _ = client.Close() }()
		if len(state.PeerCertificates) == 0 || len(state.VerifiedChains) == 0 {
			http.Error(w, "missing verified peer", http.StatusBadGateway)
			return
		}
		leaf := state.PeerCertificates[0]
		der := sha256.Sum256(leaf.Raw)
		spki := sha256.Sum256(leaf.RawSubjectPublicKeyInfo)
		_ = json.NewEncoder(w).Encode(map[string]any{"peerDER_SHA256": fmt.Sprintf("%x", der), "peerSPKI_SHA256": fmt.Sprintf("%x", spki), "verifiedChain": true, "serverName": clientTLS.ServerName, "smtpAuthenticated": true})
	})
	mux.HandleFunc("/__fixture__/deliver", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var input struct{ Recipient, Subject, Body, Attachment string }
		if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&input); err != nil {
			http.Error(w, "invalid fixture input", http.StatusBadRequest)
			return
		}
		if input.Recipient != "alice@example.test" && input.Recipient != "bob@example.test" {
			http.Error(w, "fixture recipient not allowed", http.StatusBadRequest)
			return
		}
		if strings.ContainsAny(input.Subject, "\r\n") {
			http.Error(w, "invalid fixture subject", http.StatusBadRequest)
			return
		}
		if err := deliver(server.GetPort(), clientTLS, input.Recipient, input.Subject, input.Body, input.Attachment); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"submitted":true,"transport":"TLS SMTP"}`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/emails/preview" {
			previewRequests.Add(1)
			if cookie, err := r.Cookie("fixture-delay-ms"); err == nil {
				if ms, err := strconv.Atoi(cookie.Value); err == nil && ms > 0 && ms <= 500 {
					select {
					case <-time.After(time.Duration(ms) * time.Millisecond):
					case <-r.Context().Done():
						return
					}
				}
			}
		}
		reverse.ServeHTTP(w, r)
	})
	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		return err
	}
	frontend := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	done := make(chan error, 1)
	go func() { done <- frontend.Serve(listener) }()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return frontend.Shutdown(shutdown)
	case err = <-done:
		return err
	}
}
func fixtureSMTP(port int, config *tls.Config) (*smtp.Client, tls.ConnectionState, error) {
	var state tls.ConnectionState
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 5*time.Second)
	if err != nil {
		return nil, state, err
	}
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	client, err := smtp.NewClient(conn, "127.0.0.1")
	if err != nil {
		_ = conn.Close()
		return nil, state, err
	}
	if err = client.StartTLS(config); err == nil {
		err = client.Auth(smtp.PlainAuth("", "gorge", fixturePassword, "127.0.0.1"))
	}
	if err != nil {
		_ = client.Close()
		return nil, state, err
	}
	state, _ = client.TLSConnectionState()
	return client, state, nil
}
func deliver(port int, config *tls.Config, recipient, subject, body, attachment string) error {
	client, _, err := fixtureSMTP(port, config)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	if err = client.Mail("phorge@example.test"); err != nil {
		return err
	}
	if err = client.Rcpt(recipient); err != nil {
		return err
	}
	out, err := client.Data()
	if err != nil {
		return err
	}
	if attachment == "" {
		_, err = fmt.Fprintf(out, "From: phorge@example.test\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s", recipient, subject, body)
	} else {
		multipartWriter := multipart.NewWriter(out)
		_, err = fmt.Fprintf(out, "From: phorge@example.test\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=%q\r\n\r\n", recipient, subject, multipartWriter.Boundary())
		if err == nil {
			var part io.Writer
			part, err = multipartWriter.CreatePart(textproto.MIMEHeader{"Content-Type": {"text/plain; charset=utf-8"}})
			if err == nil {
				_, err = io.WriteString(part, body)
			}
		}
		if err == nil {
			var part io.Writer
			part, err = multipartWriter.CreatePart(textproto.MIMEHeader{"Content-Type": {"text/plain"}, "Content-Disposition": {`attachment; filename="fixture.txt"`}})
			if err == nil {
				_, err = io.WriteString(part, attachment)
			}
		}
		if closeErr := multipartWriter.Close(); err == nil {
			err = closeErr
		}
	}
	if err != nil {
		return err
	}
	if err = out.Close(); err != nil {
		return err
	}
	return client.Quit()
}

// Only this build-tagged loopback fixture accepts these flags. Resolve paths
// against the state root, rejecting symlink escapes and partial configuration.
func configuredFixtureTLS(root, certPath, keyPath, caPath string) (*mailserver.TLSConfig, *tls.Config, error) {
	server := &mailserver.TLSConfig{Enabled: true}
	if certPath == "" && keyPath == "" && caPath == "" {
		return server, &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}, nil
	}
	if certPath == "" || keyPath == "" || caPath == "" {
		return nil, nil, fmt.Errorf("fixture tls-cert, tls-key and tls-ca must be provided together")
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, nil, err
	}
	for _, file := range []string{certPath, keyPath, caPath} {
		if !filepath.IsAbs(file) {
			return nil, nil, fmt.Errorf("fixture TLS path must be absolute")
		}
		info, e := os.Lstat(file)
		if e != nil {
			return nil, nil, e
		}
		if !info.Mode().IsRegular() {
			return nil, nil, fmt.Errorf("fixture TLS path must be regular")
		}
		resolved, e := filepath.EvalSymlinks(file)
		if e != nil {
			return nil, nil, e
		}
		rel, e := filepath.Rel(realRoot, resolved)
		if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, nil, fmt.Errorf("fixture TLS path must stay inside state root")
		}
	}
	if _, err = tls.LoadX509KeyPair(certPath, keyPath); err != nil {
		return nil, nil, fmt.Errorf("fixture persistent TLS pair: %w", err)
	}
	ca, err := os.ReadFile(caPath)
	if err != nil {
		return nil, nil, err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		return nil, nil, fmt.Errorf("invalid fixture TLS CA")
	}
	server.CertFile = certPath
	server.KeyFile = keyPath
	return server, &tls.Config{RootCAs: roots, ServerName: "127.0.0.1", MinVersion: tls.VersionTLS12}, nil
}
