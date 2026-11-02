//go:build gorge_integration || phorge_integration

// Package gorgeintegration exercises the unchanged external Gorge SMTP adapter.
// Identity headers below are synthetic component inputs, never a Stargate proof.
package gorgeintegration

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"debug/buildinfo"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/mail"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/api"
	"github.com/soulteary/owlmail/internal/config"
	"github.com/soulteary/owlmail/internal/identity"
	"github.com/soulteary/owlmail/internal/mailbox"
	"github.com/soulteary/owlmail/internal/mailserver"
	"golang.org/x/crypto/bcrypt"
)

const gorgeCommit = "e9ca3b7af2cc8914207fc22ffb499f5069d3364c"
const smtpPassword = "synthetic-gorge-smtp-only-password"
const serviceToken = "synthetic-gorge-http-service-token"

func TestPinnedGorgeSMTP(t *testing.T) {
	runPinnedGorgeSMTP(t, nil)
}

// submit optionally exercises a real language adapter instead of hand-built HTTP.
func runPinnedGorgeSMTP(t *testing.T, submit func(*testing.T, string, map[string]any, bool)) {
	if runtime.GOOS != "linux" {
		t.Fatal("this integration requires Linux SSL_CERT_FILE trust; run the documented isolated Linux container instead of changing host trust")
	}
	binary := os.Getenv("OWLMAIL_GORGE_BINARY")
	if !filepath.IsAbs(binary) {
		t.Fatal("OWLMAIL_GORGE_BINARY must point to the pinned, unmodified Gorge binary; this integration never skips")
	}
	info, err := buildinfo.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	settings := map[string]string{}
	for _, s := range info.Settings {
		settings[s.Key] = s.Value
	}
	if info.Path != "github.com/soulteary/gorge/go/cmd/gorge-mailer" || settings["vcs.revision"] != gorgeCommit || settings["vcs.modified"] != "false" {
		t.Fatalf("unexpected Gorge provenance: %s %#v", info.Path, settings)
	}
	root := t.TempDir()
	cert, key := certificate(t, root, "trusted")
	wrongCA, _ := certificate(t, root, "untrusted")
	hash, err := bcrypt.GenerateFromPassword([]byte(smtpPassword), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	accountsPath := filepath.Join(root, "accounts.yaml")
	account := fmt.Sprintf("version: 1\naccounts:\n  - id: gorge-system\n    username: gorge\n    password-hash: %s\n    namespace: gorge-test\n    enabled: true\n    allowed-domains: [example.test]\n", hash)
	if err = os.WriteFile(accountsPath, []byte(account), 0600); err != nil {
		t.Fatal(err)
	}
	accounts, err := mailbox.LoadAccounts(accountsPath, "gorge-test", []string{"example.test"})
	if err != nil {
		t.Fatal(err)
	}
	control, err := mailbox.Open(filepath.Join(root, "control.sqlite"), true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = control.Close() })
	server, err := mailserver.NewMailServerWithOptions(0, "127.0.0.1", filepath.Join(root, "mail"), mailserver.ServerOptions{
		AllowEphemeralPort: true, RequireOwnership: true, AuthRequireTLS: true, SMTPSPort: unusedPort(t),
		TLSConfig:     &mailserver.TLSConfig{Enabled: true, CertFile: cert, KeyFile: key},
		RoutedIngress: &mailserver.RoutedIngressOptions{Accounts: accounts, Control: control, MaxPending: 100, MaxCreatesPerMinute: 1000, MaxAuthPerMinute: 1000},
	})
	if err != nil {
		t.Fatal(err)
	}
	smtpReady := make(chan struct{})
	smtpDone := make(chan error, 1)
	go func() { smtpDone <- server.ListenWithReady(func() { close(smtpReady) }) }()
	t.Cleanup(func() {
		_ = server.Close()
		select {
		case <-smtpDone:
		case <-time.After(5 * time.Second):
			t.Error("SMTP cleanup deadline")
		}
	})
	waitReady(t, smtpReady, smtpDone)
	proxy, err := identity.NewProxy("gorge-test", []string{"127.0.0.1/32"}, "X-Auth-User", "X-Auth-Email", "X-Auth-Email-Verified")
	if err != nil {
		t.Fatal(err)
	}
	backend, err := api.NewMultiuserAPIWithHTTPS(server, 0, "127.0.0.1", false, "", "", api.MultiuserOptions{Proxy: proxy, Store: control, ClaimPolicy: mailbox.VerifiedEmail})
	if err != nil {
		t.Fatal(err)
	}
	httpReady := make(chan struct{})
	httpDone := make(chan error, 1)
	go func() { httpDone <- backend.StartWithReady(func() { close(httpReady) }) }()
	t.Cleanup(func() {
		_ = backend.Close()
		select {
		case <-httpDone:
		case <-time.After(5 * time.Second):
			t.Error("HTTP cleanup deadline")
		}
	})
	waitReady(t, httpReady, httpDone)
	endpoint := "http://127.0.0.1:" + strconv.Itoa(backend.GetPort())
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 10 * time.Second}
	t.Cleanup(client.CloseIdleConnections)
	gorge := startGorge(t, binary, cert, server.GetSMTPSPort(), smtpPassword, client)
	payload := map[string]any{"message": map[string]any{
		"from": map[string]string{"address": "phorge@example.test"}, "to": []any{map[string]string{"address": "alice@example.test"}}, "cc": []any{map[string]string{"address": "bob@example.test"}},
		"subject": "Gorge SMTP isolation — synthetic", "textBody": "Shared body explicitly mentions bob@example.test; Phorge owns content authorization.", "htmlBody": "<p>Shared synthetic body</p>",
		"headers":     []any{map[string]string{"name": "Message-ID", "value": "<gorge-synthetic@example.test>"}},
		"attachments": []any{map[string]string{"filename": "synthetic.bin", "mimeType": "application/octet-stream", "data": base64.StdEncoding.EncodeToString([]byte{0, 1, 2, 255, 10})}},
	}, "mailerKeys": []string{"owlmail-only"}}
	var status int
	var body []byte
	if submit != nil {
		submit(t, gorge, payload, false)
	} else {
		status, body = request(t, client, http.MethodPost, gorge+"/api/mailer/send", payload, map[string]string{"X-Service-Token": serviceToken})
		if status != 200 {
			t.Fatalf("Gorge send: %d %s", status, body)
		}
		var receipt struct {
			Data struct {
				MailerKey string `json:"mailerKey"`
			} `json:"data"`
		}
		decode(t, body, &receipt)
		if receipt.Data.MailerKey != "owlmail-only" {
			t.Fatalf("wrong backend receipt: %s", body)
		}
	}
	// Basic SMTP credentials carry no browser identity, including on raw/detail exits.
	status, _ = request(t, client, http.MethodGet, endpoint+"/api/v1/emails", nil, map[string]string{"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte("gorge:"+smtpPassword))})
	if status != 401 {
		t.Fatalf("system credentials gained HTTP access: %d", status)
	}
	// These are explicitly synthetic trusted-proxy assertions; no default identity.
	if got := list(t, client, endpoint, "alice", false); got.Total != 0 {
		t.Fatal("unverified assertion claimed pending mail")
	}
	alice := list(t, client, endpoint, "alice", true)
	bob := list(t, client, endpoint, "bob", true)
	if alice.Total != 1 || bob.Total != 1 || len(alice.Emails) != 1 || len(bob.Emails) != 1 || alice.Emails[0].ID == bob.Emails[0].ID {
		t.Fatalf("independent inbox copies: %#v %#v", alice, bob)
	}
	var originals [][]byte
	for i, user := range []string{"alice", "bob"} {
		id := []string{alice.Emails[0].ID, bob.Emails[0].ID}[i]
		status, detail := request(t, client, http.MethodGet, endpoint+"/api/v1/emails/"+id, nil, assertion(user, true))
		if status != 200 {
			t.Fatalf("detail %d %s", status, detail)
		}
		var message struct {
			Envelope struct {
				From string   `json:"from"`
				To   []string `json:"to"`
			} `json:"envelope"`
		}
		decode(t, detail, &message)
		if message.Envelope.From != "phorge@example.test" || !reflect.DeepEqual(message.Envelope.To, []string{user + "@example.test"}) {
			t.Fatalf("envelope not scoped: %s", detail)
		}
		status, raw := request(t, client, http.MethodGet, endpoint+"/api/v1/emails/"+id+"/raw", nil, assertion(user, true))
		if status != 200 {
			t.Fatalf("raw %d", status)
		}
		checkMIME(t, raw)
		originals = append(originals, raw)
		other := []string{bob.Emails[0].ID, alice.Emails[0].ID}[i]
		status, _ = request(t, client, http.MethodGet, endpoint+"/api/v1/emails/"+other+"/raw", nil, assertion(user, true))
		if status != 404 {
			t.Fatalf("cross inbox raw visible: %d", status)
		}
		for _, suffix := range []string{"", "/raw"} {
			status, _ = request(t, client, http.MethodGet, endpoint+"/api/v1/emails/"+id+suffix, nil, map[string]string{"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte("gorge:"+smtpPassword))})
			if status != 401 {
				t.Fatalf("SMTP credentials read %s: %d", suffix, status)
			}
		}
	}
	if !bytes.Equal(originals[0], originals[1]) {
		t.Fatal("original MIME was rewritten per recipient")
	}
	// Gorge must reject the entire SMTP transaction before DATA on any bad RCPT.
	bad := map[string]any{}
	raw, _ := json.Marshal(payload)
	decode(t, raw, &bad)
	bad["message"].(map[string]any)["cc"] = []any{map[string]string{"address": "outside@forbidden.invalid"}}
	if submit != nil {
		submit(t, gorge, bad, true)
	} else {
		status, body = request(t, client, http.MethodPost, gorge+"/api/mailer/send", bad, map[string]string{"X-Service-Token": serviceToken})
		if status != 422 || !bytes.Contains(body, []byte("ERR_PERMANENT_FAILURE")) {
			t.Fatalf("unknown-domain rejection: %d %s", status, body)
		}
	}
	assertTotals(t, client, endpoint)
	for _, item := range []struct{ name, ca, password, reason string }{{"untrusted TLS", wrongCA, smtpPassword, "x509: certificate signed by unknown authority"}, {"invalid SMTP credentials", cert, "wrong-synthetic-password", "535"}} {
		t.Run(item.name, func(t *testing.T) {
			other := startGorge(t, binary, item.ca, server.GetSMTPSPort(), item.password, client)
			status, body := request(t, client, http.MethodPost, other+"/api/mailer/send", payload, map[string]string{"X-Service-Token": serviceToken})
			if status != http.StatusBadGateway || !bytes.Contains(body, []byte("ERR_SEND_FAILED")) || !bytes.Contains(body, []byte(item.reason)) {
				t.Fatalf("transport rejection mismatch: %d %s", status, body)
			}
			t.Logf("verified HTTP 502/ERR_SEND_FAILED: %s", item.reason)
			assertTotals(t, client, endpoint)
		})
	}
	// Outage cannot reach a real provider: only one adapter is configured and the
	// subprocess receives no inherited GORGE/provider/SMTP configuration.
	if err = server.Close(); err != nil {
		t.Fatal(err)
	}
	status, body = request(t, client, http.MethodPost, gorge+"/api/mailer/send", payload, map[string]string{"X-Service-Token": serviceToken})
	if status != http.StatusBadGateway || !bytes.Contains(body, []byte("ERR_SEND_FAILED")) || !bytes.Contains(body, []byte("connect: connection refused")) {
		t.Fatalf("outage rejection mismatch: %d %s", status, body)
	}
	assertTotals(t, client, endpoint)
	cfg := config.DefaultConfig()
	cfg.Mode = "multiuser"
	cfg.IdentityNamespace = "gorge-test"
	cfg.IdentityTrustedProxies = "127.0.0.1/32"
	cfg.ClaimPolicy = "verified-email"
	cfg.ControlDatabase = filepath.Join(root, "unused-gate-control.sqlite")
	cfg.MailDir = filepath.Join(root, "unused-gate-mail")
	cfg.SystemSMTPAccountsFile = accountsPath
	cfg.RecipientAllowedDomains = "example.test"
	cfg.TLSEnabled = true
	cfg.SMTPAuthRequireTLS = true
	if err = config.ValidateMode(cfg); err != nil {
		t.Fatalf("invalid gate test config: %v", err)
	}
	if err = config.ValidateModeRuntime(cfg); err == nil || !strings.Contains(err.Error(), "multiuser mode is not available yet") {
		t.Fatal("production mode gate unexpectedly open")
	}
	t.Logf("PASS unchanged Gorge %s: implicit TLS, SMTP system auth, independent envelope copies, unchanged shared MIME, isolated HTTP reads, rejected domain/TLS/auth/outage; synthetic identity only", gorgeCommit)
}

type listing struct {
	Total  int `json:"total"`
	Emails []struct {
		ID string `json:"id"`
	} `json:"emails"`
}

func assertion(user string, verified bool) map[string]string {
	return map[string]string{"X-Auth-User": user, "X-Auth-Email": user + "@example.test", "X-Auth-Email-Verified": strconv.FormatBool(verified)}
}
func list(t *testing.T, c *http.Client, endpoint, user string, verified bool) listing {
	t.Helper()
	status, body := request(t, c, http.MethodGet, endpoint+"/api/v1/emails", nil, assertion(user, verified))
	if status != 200 {
		t.Fatalf("list: %d %s", status, body)
	}
	var out listing
	decode(t, body, &out)
	return out
}
func assertTotals(t *testing.T, c *http.Client, endpoint string) {
	t.Helper()
	for _, u := range []string{"alice", "bob"} {
		if list(t, c, endpoint, u, true).Total != 1 {
			t.Fatal("rejected delivery published a partial/extra copy")
		}
	}
}
func decode(t *testing.T, b []byte, out any) {
	t.Helper()
	if err := json.Unmarshal(b, out); err != nil {
		t.Fatalf("JSON: %v: %s", err, b)
	}
}
func request(t *testing.T, c *http.Client, method, url string, payload any, headers map[string]string) (int, []byte) {
	t.Helper()
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, url, body)
	if err != nil {
		t.Fatal(err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, raw
}
func unusedPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := l.Addr().(*net.TCPAddr).Port
	if err = l.Close(); err != nil {
		t.Fatal(err)
	}
	return p
}
func waitReady(t *testing.T, ready <-chan struct{}, done <-chan error) {
	t.Helper()
	select {
	case <-ready:
	case err := <-done:
		t.Fatalf("listener exited: %v", err)
	case <-time.After(15 * time.Second):
		t.Fatal("listener readiness timeout")
	}
}
func certificate(t *testing.T, dir, name string) (string, string) {
	t.Helper()
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "Synthetic Gorge integration"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, pub, key)
	if err != nil {
		t.Fatal(err)
	}
	private, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	cp, kp := filepath.Join(dir, name+".pem"), filepath.Join(dir, name+".key")
	for p, b := range map[string][]byte{cp: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), kp: pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private})} {
		if err = os.WriteFile(p, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return cp, kp
}

func startGorge(t *testing.T, binary, ca string, smtpPort int, password string, client *http.Client) string {
	t.Helper()
	dir := t.TempDir()
	port := unusedPort(t)
	address := "127.0.0.1:" + strconv.Itoa(port)
	cfg := map[string]any{"listenAddr": address, "serviceToken": serviceToken, "maxRetries": 0, "retryWaitSec": 0, "mailers": []any{map[string]any{"key": "owlmail-only", "type": "smtp", "options": map[string]string{"host": "127.0.0.1", "port": strconv.Itoa(smtpPort), "protocol": "ssl", "user": "gorge", "password": password}}}}
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "gorge.json")
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	logFile, err := os.OpenFile(filepath.Join(dir, "gorge.log"), os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	emptyRoots := filepath.Join(dir, "empty-root-directory")
	if err = os.Mkdir(emptyRoots, 0700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary)
	cmd.Dir = dir
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "TMPDIR=" + dir, "GORGE_MAILER_CONFIG_FILE=" + path, "SSL_CERT_FILE=" + ca, "SSL_CERT_DIR=" + emptyRoots}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err = cmd.Start(); err != nil {
		_ = logFile.Close()
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Error("Gorge cleanup timeout")
			}
		}
		_ = logFile.Close()
		if t.Failed() {
			b, _ := os.ReadFile(filepath.Join(dir, "gorge.log"))
			t.Logf("Gorge log: %s", b)
		}
	})
	endpoint := "http://" + address
	deadline := time.Now().Add(15 * time.Second)
	for {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, endpoint+"/readyz", nil)
		if err != nil {
			t.Fatal(err)
		}
		response, err := client.Do(req)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == 200 {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("Gorge readiness timeout")
		}
		time.Sleep(25 * time.Millisecond)
	}
	status, body := request(t, client, http.MethodGet, endpoint+"/api/mailer/mailers", nil, map[string]string{"X-Service-Token": serviceToken})
	if status != 200 {
		t.Fatalf("mailers: %d %s", status, body)
	}
	var backends struct {
		Data []struct{ Key, Type string } `json:"data"`
	}
	decode(t, body, &backends)
	if len(backends.Data) != 1 || backends.Data[0].Key != "owlmail-only" || backends.Data[0].Type != "smtp" {
		t.Fatalf("unexpected fallback configured: %s", body)
	}
	return endpoint
}

func checkMIME(t *testing.T, raw []byte) {
	t.Helper()
	m, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	for header, want := range map[string]string{"To": "alice@example.test", "Cc": "bob@example.test", "Message-ID": "<gorge-synthetic@example.test>"} {
		if m.Header.Get(header) != want {
			t.Fatalf("original header %s: %q", header, m.Header.Get(header))
		}
	}
	media, params, err := mime.ParseMediaType(m.Header.Get("Content-Type"))
	if err != nil || media != "multipart/mixed" {
		t.Fatalf("mixed MIME missing: %s %v", media, err)
	}
	var bodies []string
	attachments := 0
	var walk func(io.Reader, string)
	walk = func(reader io.Reader, boundary string) {
		mr := multipart.NewReader(reader, boundary)
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			kind, params, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.HasPrefix(kind, "multipart/") {
				walk(part, params["boundary"])
				continue
			}
			var payload io.Reader = part
			if part.Header.Get("Content-Transfer-Encoding") == "base64" {
				payload = base64.NewDecoder(base64.StdEncoding, part)
			}
			data, err := io.ReadAll(payload)
			if err != nil {
				t.Fatal(err)
			}
			if part.FileName() != "" {
				attachments++
				if part.FileName() != "synthetic.bin" || !bytes.Equal(data, []byte{0, 1, 2, 255, 10}) {
					t.Fatal("attachment bytes changed")
				}
			} else {
				bodies = append(bodies, string(data))
			}
		}
	}
	walk(m.Body, params["boundary"])
	if attachments != 1 || !reflect.DeepEqual(bodies, []string{"Shared body explicitly mentions bob@example.test; Phorge owns content authorization.", "<p>Shared synthetic body</p>"}) {
		t.Fatalf("MIME body boundary: %q / %d", bodies, attachments)
	}
}
