package sendmail

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/emersion/go-sasl"
	smtp "github.com/emersion/go-smtp"
)

func TestParseArgs(t *testing.T) {
	lookup := func(values map[string]string) func(string) (string, bool) {
		return func(name string) (string, bool) {
			value, ok := values[name]
			return value, ok
		}
	}

	cfg, help, err := parseArgs([]string{
		"-t", "-oi", "-i", "-fbounce@example.com", "--host=mail.local",
		"--port", "2525", "--starttls", "--username", "user", "--password", "secret",
		"first@example.com", "--", "-recipient@example.com",
	}, lookup(nil))
	if err != nil || help {
		t.Fatalf("parseArgs() = %#v, %t, %v", cfg, help, err)
	}
	if cfg.host != "mail.local" || cfg.port != 2525 || !cfg.startTLS || cfg.smtps || !cfg.readHeader {
		t.Fatalf("unexpected config: %#v", cfg)
	}
	if cfg.from != "bounce@example.com" || !cfg.fromSet || cfg.username != "user" || cfg.password != "secret" {
		t.Fatalf("unexpected sender or credentials: %#v", cfg)
	}
	if !reflect.DeepEqual(cfg.recipients, []string{"first@example.com", "-recipient@example.com"}) {
		t.Fatalf("recipients = %#v", cfg.recipients)
	}

	environment := map[string]string{
		"OWLMAIL_SENDMAIL_HOST":     "env.local",
		"OWLMAIL_SENDMAIL_PORT":     "2465",
		"OWLMAIL_SENDMAIL_STARTTLS": "true",
		"OWLMAIL_SENDMAIL_USERNAME": "env-user",
		"OWLMAIL_SENDMAIL_PASSWORD": "env-secret",
	}
	cfg, _, err = parseArgs([]string{"--host", "cli.local", "--starttls=false", "to@example.com"}, lookup(environment))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.host != "cli.local" || cfg.port != 2465 || cfg.startTLS || cfg.username != "env-user" || cfg.password != "env-secret" {
		t.Fatalf("CLI did not override environment correctly: %#v", cfg)
	}
}

func TestParseArgsErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		env  map[string]string
	}{
		{name: "unknown", args: []string{"-x"}},
		{name: "missing value", args: []string{"--host"}},
		{name: "bad port", args: []string{"--port", "0"}},
		{name: "conflicting TLS", args: []string{"--starttls", "--smtps"}},
		{name: "partial auth", args: []string{"--username", "user"}},
		{name: "bad environment boolean", env: map[string]string{"OWLMAIL_SENDMAIL_SMTPS": "sometimes"}},
		{name: "bad environment port", env: map[string]string{"OWLMAIL_SENDMAIL_PORT": "smtp"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lookup := func(name string) (string, bool) {
				value, ok := test.env[name]
				return value, ok
			}
			_, _, err := parseArgs(test.args, lookup)
			if err == nil || exitCode(err) != ExitUsage {
				t.Fatalf("parseArgs() error = %v, exit = %d", err, exitCode(err))
			}
		})
	}
}

func TestPrepareMessageExtractsRecipientsAndRemovesBcc(t *testing.T) {
	input := "From: =?UTF-8?Q?Jos=C3=A9?= <sender@example.com>\n" +
		"To: One <one@example.com>,\n\tTwo <two@example.com>\n" +
		"Cc: three@example.com\n" +
		"Bcc: hidden@example.com,\n\tfolded@example.com\n" +
		"Subject: こんにちは\n\nhello\n.leading dot\n"
	message, err := prepareMessage(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if message.headerSender != "sender@example.com" {
		t.Fatalf("header sender = %q", message.headerSender)
	}
	wantRecipients := []string{"one@example.com", "two@example.com", "three@example.com", "hidden@example.com", "folded@example.com"}
	if !reflect.DeepEqual(message.headerRecipients, wantRecipients) {
		t.Fatalf("recipients = %#v, want %#v", message.headerRecipients, wantRecipients)
	}
	if !message.requiresUTF8 {
		t.Fatal("raw UTF-8 header should require SMTPUTF8")
	}
	sanitized, err := io.ReadAll(message.reader)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(bytes.ToLower(sanitized), []byte("bcc:")) || bytes.Contains(sanitized, []byte("folded@example.com")) {
		t.Fatalf("Bcc field was not removed: %q", sanitized)
	}
	if !bytes.Contains(sanitized, []byte("Subject: こんにちは")) || !bytes.Contains(sanitized, []byte(".leading dot")) {
		t.Fatalf("message content changed unexpectedly: %q", sanitized)
	}
}

func TestPrepareMessageRejectsMalformedInput(t *testing.T) {
	for _, input := range []string{
		"Subject: missing separator",
		" folded-without-field\n\nbody",
		"Not a header\n\nbody",
		"To: not-an-address\n\nbody",
	} {
		_, err := prepareMessage(strings.NewReader(input))
		if err == nil || exitCode(err) != ExitDataError {
			t.Fatalf("prepareMessage(%q) error = %v, exit = %d", input, err, exitCode(err))
		}
	}
}

func TestRunSubmitsThroughSMTP(t *testing.T) {
	backend := &fakeBackend{username: "user", password: "secret"}
	host, port := startFakeServer(t, backend, nil, false)
	message := "From: sender@example.com\n" +
		"To: =?UTF-8?Q?T=C3=A9st?= <one@example.com>\n" +
		"Cc: two@example.com\n" +
		"Bcc: hidden@example.com\n" +
		"Subject: こんにちは\n\nfirst\n.leading\nlast"
	var stdout, stderr bytes.Buffer
	exit := Run([]string{
		"-t", "-oi", "-f<>", "--host", host, "--port", port,
		"--username", "user", "--password", "secret", "explicit@example.com",
	}, strings.NewReader(message), &stdout, &stderr)
	if exit != ExitOK {
		t.Fatalf("Run() exit = %d, stderr = %q", exit, stderr.String())
	}

	received := backend.snapshot()
	if received.from != "" {
		t.Fatalf("MAIL FROM = %q, want empty reverse path", received.from)
	}
	wantRecipients := []string{"explicit@example.com", "one@example.com", "two@example.com", "hidden@example.com"}
	if !reflect.DeepEqual(received.recipients, wantRecipients) {
		t.Fatalf("RCPT TO = %#v, want %#v", received.recipients, wantRecipients)
	}
	if received.mailOptions == nil || !received.mailOptions.UTF8 {
		t.Fatalf("MAIL options = %#v, want SMTPUTF8", received.mailOptions)
	}
	if strings.Contains(strings.ToLower(received.data), "bcc:") || strings.Contains(received.data, "hidden@example.com") {
		t.Fatalf("Bcc leaked into DATA: %q", received.data)
	}
	if !strings.Contains(received.data, "\r\n.leading\r\n") || !strings.HasSuffix(received.data, "last\r\n") {
		t.Fatalf("CRLF or dot-stuffing round trip failed: %q", received.data)
	}
	if received.username != "user" || received.password != "secret" {
		t.Fatalf("AUTH credentials were not received by fake server")
	}
}

func TestSubmitSupportsSTARTTLSAndSMTPS(t *testing.T) {
	serverTLS, clientTLS := testTLSConfigs(t)
	for _, test := range []struct {
		name     string
		startTLS bool
		smtps    bool
	}{
		{name: "STARTTLS", startTLS: true},
		{name: "SMTPS", smtps: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			backend := &fakeBackend{}
			host, port := startFakeServer(t, backend, serverTLS, test.smtps)
			message, err := prepareMessage(strings.NewReader("From: sender@example.com\r\nTo: to@example.com\r\n\r\nTLS\r\n"))
			if err != nil {
				t.Fatal(err)
			}
			portNumber, err := parsePort(port, "test port")
			if err != nil {
				t.Fatal(err)
			}
			err = submit(config{
				host: host, port: portNumber, startTLS: test.startTLS, smtps: test.smtps, tlsConfig: clientTLS,
			}, "sender@example.com", []string{"to@example.com"}, message)
			if err != nil {
				t.Fatal(err)
			}
			if !backend.snapshot().tls {
				t.Fatal("fake server did not observe TLS")
			}
		})
	}
}

func TestRunClassifiesSMTPFailuresWithoutLeakingSensitiveInput(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		exit int
	}{
		{name: "temporary", err: &smtp.SMTPError{Code: 451, EnhancedCode: smtp.EnhancedCode{4, 3, 2}, Message: "try later"}, exit: ExitTempFailure},
		{name: "permanent", err: &smtp.SMTPError{Code: 552, EnhancedCode: smtp.EnhancedCode{5, 3, 4}, Message: "too large"}, exit: ExitUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			backend := &fakeBackend{dataErr: test.err}
			host, port := startFakeServer(t, backend, nil, false)
			const secret = "password-that-must-not-leak"
			const body = "body-that-must-not-leak"
			var stderr bytes.Buffer
			exit := Run([]string{"-t", "--host", host, "--port", port}, strings.NewReader(
				"From: sender@example.com\r\nTo: to@example.com\r\n\r\n"+body+"\r\n",
			), io.Discard, &stderr)
			if exit != test.exit {
				t.Fatalf("Run() exit = %d, want %d, stderr = %q", exit, test.exit, stderr.String())
			}
			if strings.Contains(stderr.String(), secret) || strings.Contains(stderr.String(), body) {
				t.Fatalf("sensitive input leaked: %q", stderr.String())
			}
		})
	}

	t.Run("authentication rejection hides password and body", func(t *testing.T) {
		backend := &fakeBackend{username: "user", password: "expected"}
		host, port := startFakeServer(t, backend, nil, false)
		const password = "password-that-must-not-leak"
		const body = "body-that-must-not-leak"
		var stderr bytes.Buffer
		exit := Run([]string{
			"-t", "--host", host, "--port", port,
			"--username", "user", "--password", password,
		}, strings.NewReader(
			"From: sender@example.com\r\nTo: to@example.com\r\n\r\n"+body+"\r\n",
		), io.Discard, &stderr)
		if exit != ExitUnavailable {
			t.Fatalf("Run() exit = %d, stderr = %q", exit, stderr.String())
		}
		if strings.Contains(stderr.String(), password) || strings.Contains(stderr.String(), body) {
			t.Fatalf("sensitive input leaked: %q", stderr.String())
		}
	})
}

func TestRunUsageAndHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if exit := Run([]string{"--help"}, nil, &stdout, &stderr); exit != ExitOK || !strings.Contains(stdout.String(), "OWLMAIL_SENDMAIL_PASSWORD") {
		t.Fatalf("help exit/output = %d, %q", exit, stdout.String())
	}
	stdout.Reset()
	if exit := Run(nil, strings.NewReader("From: sender@example.com\r\n\r\nbody"), &stdout, &stderr); exit != ExitUsage {
		t.Fatalf("missing recipient exit = %d, stderr = %q", exit, stderr.String())
	}
}

type fakeBackend struct {
	mu       sync.Mutex
	username string
	password string
	dataErr  error
	received fakeReceived
}

type fakeReceived struct {
	from        string
	recipients  []string
	data        string
	username    string
	password    string
	tls         bool
	mailOptions *smtp.MailOptions
}

func (backend *fakeBackend) NewSession(conn *smtp.Conn) (smtp.Session, error) {
	return &fakeSession{backend: backend, conn: conn}, nil
}

func (backend *fakeBackend) snapshot() fakeReceived {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	received := backend.received
	received.recipients = append([]string(nil), received.recipients...)
	if received.mailOptions != nil {
		copy := *received.mailOptions
		received.mailOptions = &copy
	}
	return received
}

type fakeSession struct {
	backend *fakeBackend
	conn    *smtp.Conn
}

func (session *fakeSession) Reset()        {}
func (session *fakeSession) Logout() error { return nil }

func (session *fakeSession) AuthMechanisms() []string { return []string{sasl.Plain} }

func (session *fakeSession) Auth(mechanism string) (sasl.Server, error) {
	if mechanism != sasl.Plain {
		return nil, smtp.ErrAuthUnknownMechanism
	}
	return sasl.NewPlainServer(func(_, username, password string) error {
		if session.backend.username != "" && (username != session.backend.username || password != session.backend.password) {
			return smtp.ErrAuthFailed
		}
		session.backend.mu.Lock()
		session.backend.received.username = username
		session.backend.received.password = password
		session.backend.mu.Unlock()
		return nil
	}), nil
}

func (session *fakeSession) Mail(from string, options *smtp.MailOptions) error {
	session.backend.mu.Lock()
	defer session.backend.mu.Unlock()
	session.backend.received.from = from
	_, session.backend.received.tls = session.conn.TLSConnectionState()
	if options != nil {
		copy := *options
		session.backend.received.mailOptions = &copy
	}
	return nil
}

func (session *fakeSession) Rcpt(to string, _ *smtp.RcptOptions) error {
	session.backend.mu.Lock()
	defer session.backend.mu.Unlock()
	session.backend.received.recipients = append(session.backend.received.recipients, to)
	return nil
}

func (session *fakeSession) Data(reader io.Reader) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	session.backend.mu.Lock()
	session.backend.received.data = string(data)
	configuredErr := session.backend.dataErr
	session.backend.mu.Unlock()
	return configuredErr
}

func startFakeServer(t *testing.T, backend smtp.Backend, tlsConfig *tls.Config, implicitTLS bool) (string, string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := smtp.NewServer(backend)
	server.Domain = "localhost"
	server.AllowInsecureAuth = true
	server.EnableSMTPUTF8 = true
	server.TLSConfig = tlsConfig
	serveListener := listener
	if implicitTLS {
		serveListener = tls.NewListener(listener, tlsConfig)
	}
	done := make(chan error, 1)
	go func() { done <- server.Serve(serveListener) }()
	t.Cleanup(func() {
		_ = server.Close()
		_ = listener.Close()
		select {
		case err := <-done:
			if err != nil && !errors.Is(err, net.ErrClosed) {
				t.Errorf("fake SMTP server: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Error("fake SMTP server did not stop")
		}
	})
	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	return host, port
}

func testTLSConfigs(t *testing.T) (*tls.Config, *tls.Config) {
	t.Helper()
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	certificateDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := tls.X509KeyPair(
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER}),
	)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	parsedCertificate, err := x509.ParseCertificate(certificateDER)
	if err != nil {
		t.Fatal(err)
	}
	roots.AddCert(parsedCertificate)
	return &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12},
		&tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
}
