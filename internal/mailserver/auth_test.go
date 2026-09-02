package mailserver

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
)

func TestCredentialVerifier(t *testing.T) {
	verifier, err := newCredentialVerifier("user", "pass")
	if err != nil {
		t.Fatal(err)
	}
	if !verifier.credentialsEqual("user", "pass") {
		t.Fatal("matching credentials were rejected")
	}
	for _, test := range []struct {
		username string
		password string
	}{
		{username: "uses", password: "pass"},
		{username: "other", password: "pass"},
		{username: "user", password: "fail"},
		{username: "user", password: "other"},
		{username: "", password: ""},
	} {
		if verifier.credentialsEqual(test.username, test.password) {
			t.Fatalf("credentials %q/%q unexpectedly matched", test.username, test.password)
		}
	}

	shortTag := verifier.tag("x")
	longTag := verifier.tag(strings.Repeat("x", 1024))
	if len(shortTag) != sha256.Size || len(longTag) != sha256.Size {
		t.Fatalf("credential tag lengths = %d/%d, want %d", len(shortTag), len(longTag), sha256.Size)
	}
	if bytes.Equal(shortTag[:], longTag[:]) {
		t.Fatal("different credentials produced matching tags")
	}
	if !verifier.stringsEqual("identity", "identity") {
		t.Fatal("matching authorization identities were rejected")
	}
	if verifier.stringsEqual("same", "different-length") {
		t.Fatal("different authorization identities unexpectedly matched")
	}
}

func TestLoginServerWithoutInitialResponse(t *testing.T) {
	var gotUsername, gotPassword string
	server := newLoginServer(func(username, password string) error {
		gotUsername, gotPassword = username, password
		return nil
	})

	challenge, done, err := server.Next(nil)
	if err != nil || done || string(challenge) != "Username:" {
		t.Fatalf("initial challenge = %q, %v, %v", challenge, done, err)
	}
	challenge, done, err = server.Next([]byte("user"))
	if err != nil || done || string(challenge) != "Password:" {
		t.Fatalf("password challenge = %q, %v, %v", challenge, done, err)
	}
	challenge, done, err = server.Next([]byte("pass"))
	if err != nil || !done || challenge != nil {
		t.Fatalf("completion = %q, %v, %v", challenge, done, err)
	}
	if gotUsername != "user" || gotPassword != "pass" {
		t.Fatalf("credentials = %q/%q", gotUsername, gotPassword)
	}
	if _, _, err := server.Next([]byte("extra")); !errors.Is(err, sasl.ErrUnexpectedClientResponse) {
		t.Fatalf("extra response error = %v", err)
	}
}

func TestLoginServerWithInitialResponse(t *testing.T) {
	server := newLoginServer(func(username, password string) error {
		if username != "user" || password != "pass" {
			t.Fatalf("credentials = %q/%q", username, password)
		}
		return nil
	})

	challenge, done, err := server.Next([]byte("user"))
	if err != nil || done || string(challenge) != "Password:" {
		t.Fatalf("password challenge = %q, %v, %v", challenge, done, err)
	}
	if _, done, err = server.Next([]byte("pass")); err != nil || !done {
		t.Fatalf("completion = %v, %v", done, err)
	}
}

func TestAuthRejectsUnknownMechanism(t *testing.T) {
	server, err := NewMailServer(1025, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	session := &Session{mailServer: server}
	if _, err := session.Auth("CRAM-MD5"); err != smtp.ErrAuthUnknownMechanism {
		t.Fatalf("Auth() error = %v, want %v", err, smtp.ErrAuthUnknownMechanism)
	}
}

func TestNewMailServerRejectsIncompleteAuthConfig(t *testing.T) {
	for _, authConfig := range []*SMTPAuthConfig{
		{Enabled: true, Username: "user"},
		{Enabled: true, Password: "pass"},
	} {
		if _, err := NewMailServerWithConfig(1025, "localhost", t.TempDir(), nil, authConfig, nil); err == nil {
			t.Fatalf("incomplete auth config %#v should fail", authConfig)
		}
	}
}

func TestSMTPNoAuthMode(t *testing.T) {
	server, address := startSMTPAuthTestServer(t, nil)

	client := dialSMTPTestClient(t, address)
	assertAuthMechanisms(t, client)
	sendSMTPTestMessage(t, client, "no-auth@example.test")
	quitSMTPTestClient(t, client)

	client = dialSMTPTestClient(t, address)
	if err := client.Auth(sasl.NewPlainClient("", "arbitrary-user", "arbitrary-password")); err != nil {
		t.Fatalf("NO AUTH PLAIN failed: %v", err)
	}
	sendSMTPTestMessage(t, client, "plain@example.test")
	quitSMTPTestClient(t, client)

	client = dialSMTPTestClient(t, address)
	if err := client.Auth(sasl.NewLoginClient("arbitrary-user", "arbitrary-password")); err != nil {
		t.Fatalf("NO AUTH LOGIN failed: %v", err)
	}
	sendSMTPTestMessage(t, client, "login@example.test")
	quitSMTPTestClient(t, client)

	if got := len(server.GetAllEmail()); got != 3 {
		t.Fatalf("stored messages = %d, want 3", got)
	}
}

func TestSMTPAuthRequiredMode(t *testing.T) {
	authConfig := &SMTPAuthConfig{Enabled: true, Username: "user", Password: "pass"}
	server, address := startSMTPAuthTestServer(t, authConfig)

	client := dialSMTPTestClient(t, address)
	assertAuthMechanisms(t, client)
	err := client.Mail("unauthenticated@example.test", nil)
	assertSMTPErrorCode(t, err, 530)
	_ = client.Close()

	for name, authClient := range map[string]sasl.Client{
		"PLAIN same-length password":      sasl.NewPlainClient("", "user", "fail"),
		"PLAIN different-length password": sasl.NewPlainClient("", "user", "wrong"),
		"LOGIN same-length username":      sasl.NewLoginClient("uses", "pass"),
		"LOGIN different-length username": sasl.NewLoginClient("wrong", "pass"),
	} {
		t.Run("reject wrong "+name, func(t *testing.T) {
			client := dialSMTPTestClient(t, address)
			defer func() { _ = client.Close() }()
			assertSMTPErrorCode(t, client.Auth(authClient), 535)
		})
	}

	client = dialSMTPTestClient(t, address)
	assertSMTPErrorCode(t, client.Auth(sasl.NewPlainClient("other", "user", "pass")), 535)
	_ = client.Close()

	for name, authClient := range map[string]sasl.Client{
		"PLAIN": sasl.NewPlainClient("", "user", "pass"),
		"LOGIN": sasl.NewLoginClient("user", "pass"),
	} {
		t.Run("accept "+name, func(t *testing.T) {
			client := dialSMTPTestClient(t, address)
			if err := client.Auth(authClient); err != nil {
				t.Fatalf("AUTH failed: %v", err)
			}
			sendSMTPTestMessage(t, client, name+"@example.test")
			quitSMTPTestClient(t, client)
		})
	}

	if got := len(server.GetAllEmail()); got != 2 {
		t.Fatalf("stored messages = %d, want 2", got)
	}
}

func TestSMTPAuthRequireTLSRejectsPlaintext(t *testing.T) {
	authConfig := &SMTPAuthConfig{Enabled: true, Username: "user", Password: "pass"}
	_, address := startSMTPAuthTLSTestServer(t, authConfig, false)

	for name, authClient := range map[string]sasl.Client{
		"PLAIN": sasl.NewPlainClient("", "user", "pass"),
		"LOGIN": sasl.NewLoginClient("user", "pass"),
	} {
		t.Run(name, func(t *testing.T) {
			client := dialSMTPTestClient(t, address)
			defer func() { _ = client.Close() }()
			if client.SupportsAuth(name) {
				t.Fatalf("plaintext connection advertised AUTH %s", name)
			}
			if err := client.Auth(authClient); err == nil {
				t.Fatalf("plaintext AUTH %s succeeded", name)
			}
		})
	}
}

func TestSMTPAuthRequireTLSAllowsSTARTTLS(t *testing.T) {
	authConfig := &SMTPAuthConfig{Enabled: true, Username: "user", Password: "pass"}
	server, address := startSMTPAuthTLSTestServer(t, authConfig, false)

	for name, authClient := range map[string]sasl.Client{
		"PLAIN": sasl.NewPlainClient("", "user", "pass"),
		"LOGIN": sasl.NewLoginClient("user", "pass"),
	} {
		t.Run(name, func(t *testing.T) {
			client, err := smtp.DialStartTLS(address, insecureSMTPTestTLSConfig())
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = client.Close() }()
			if !client.SupportsAuth(name) {
				t.Fatalf("STARTTLS connection did not advertise AUTH %s", name)
			}
			if err := client.Auth(authClient); err != nil {
				t.Fatalf("STARTTLS AUTH %s failed: %v", name, err)
			}
			sendSMTPTestMessage(t, client, strings.ToLower(name)+"-starttls@example.test")
			quitSMTPTestClient(t, client)
		})
	}

	if got := len(server.GetAllEmail()); got != 2 {
		t.Fatalf("stored STARTTLS messages = %d, want 2", got)
	}
}

func TestSMTPAuthRequireTLSAllowsSMTPS(t *testing.T) {
	authConfig := &SMTPAuthConfig{Enabled: true, Username: "user", Password: "pass"}
	server, address := startSMTPAuthTLSTestServer(t, authConfig, true)

	for name, authClient := range map[string]sasl.Client{
		"PLAIN": sasl.NewPlainClient("", "user", "pass"),
		"LOGIN": sasl.NewLoginClient("user", "pass"),
	} {
		t.Run(name, func(t *testing.T) {
			client, err := smtp.DialTLS(address, insecureSMTPTestTLSConfig())
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = client.Close() }()
			if !client.SupportsAuth(name) {
				t.Fatalf("SMTPS connection did not advertise AUTH %s", name)
			}
			if err := client.Auth(authClient); err != nil {
				t.Fatalf("SMTPS AUTH %s failed: %v", name, err)
			}
			sendSMTPTestMessage(t, client, strings.ToLower(name)+"-smtps@example.test")
			quitSMTPTestClient(t, client)
		})
	}

	if got := len(server.GetAllEmail()); got != 2 {
		t.Fatalf("stored SMTPS messages = %d, want 2", got)
	}
}

func TestSMTPAuthRequireTLSKeepsNoAuthAnonymousDelivery(t *testing.T) {
	server, address := startSMTPAuthTLSTestServer(t, nil, false)
	client := dialSMTPTestClient(t, address)
	defer func() { _ = client.Close() }()

	if client.SupportsAuth(sasl.Plain) || client.SupportsAuth(sasl.Login) {
		t.Fatal("plaintext NO AUTH connection advertised AUTH while TLS was required")
	}
	sendSMTPTestMessage(t, client, "anonymous@example.test")
	quitSMTPTestClient(t, client)

	if got := len(server.GetAllEmail()); got != 1 {
		t.Fatalf("stored anonymous messages = %d, want 1", got)
	}
}

func startSMTPAuthTestServer(t *testing.T, authConfig *SMTPAuthConfig) (*MailServer, string) {
	t.Helper()
	server, err := NewMailServerWithConfig(1025, "127.0.0.1", t.TempDir(), nil, authConfig, nil)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = server.Close()
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- server.smtpServer.Serve(listener)
	}()
	t.Cleanup(func() {
		_ = server.Close()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("SMTP test server did not stop")
		}
	})
	return server, listener.Addr().String()
}

func startSMTPAuthTLSTestServer(t *testing.T, authConfig *SMTPAuthConfig, implicitTLS bool) (*MailServer, string) {
	t.Helper()
	server, err := NewMailServerWithOptions(1025, "127.0.0.1", t.TempDir(), ServerOptions{
		AuthConfig:     authConfig,
		AuthRequireTLS: true,
		TLSConfig:      &TLSConfig{Enabled: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	rawListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = server.Close()
		t.Fatal(err)
	}
	listener := net.Listener(rawListener)
	serve := server.smtpServer
	if implicitTLS {
		serve = server.smtpsServer
		listener = tls.NewListener(rawListener, serve.TLSConfig)
	}
	done := make(chan error, 1)
	go func() {
		done <- serve.Serve(listener)
	}()
	t.Cleanup(func() {
		_ = server.Close()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("SMTP TLS test server did not stop")
		}
	})
	return server, rawListener.Addr().String()
}

func insecureSMTPTestTLSConfig() *tls.Config {
	return &tls.Config{
		// The test server uses a fresh self-signed certificate.
		InsecureSkipVerify: true, //nolint:gosec
	}
}

func dialSMTPTestClient(t *testing.T, address string) *smtp.Client {
	t.Helper()
	client, err := smtp.Dial(address)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func assertAuthMechanisms(t *testing.T, client *smtp.Client) {
	t.Helper()
	if !client.SupportsAuth(sasl.Plain) || !client.SupportsAuth(sasl.Login) {
		t.Fatal("server did not advertise AUTH PLAIN LOGIN")
	}
}

func sendSMTPTestMessage(t *testing.T, client *smtp.Client, from string) {
	t.Helper()
	if err := client.Mail(from, nil); err != nil {
		t.Fatalf("MAIL FROM failed: %v", err)
	}
	if err := client.Rcpt("recipient@example.test", nil); err != nil {
		t.Fatalf("RCPT TO failed: %v", err)
	}
	writer, err := client.Data()
	if err != nil {
		t.Fatalf("DATA failed: %v", err)
	}
	message := "From: " + from + "\r\nTo: recipient@example.test\r\nSubject: SMTP AUTH test\r\n\r\nbody\r\n"
	if _, err := writer.Write([]byte(message)); err != nil {
		t.Fatalf("write DATA failed: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("finish DATA failed: %v", err)
	}
}

func quitSMTPTestClient(t *testing.T, client *smtp.Client) {
	t.Helper()
	if err := client.Quit(); err != nil {
		t.Fatalf("QUIT failed: %v", err)
	}
}

func assertSMTPErrorCode(t *testing.T, err error, want int) {
	t.Helper()
	if err == nil {
		t.Fatalf("SMTP command succeeded, want code %d", want)
	}
	var smtpErr *smtp.SMTPError
	if !errors.As(err, &smtpErr) || smtpErr.Code != want {
		t.Fatalf("SMTP error = %v, want code %d", err, want)
	}
}
