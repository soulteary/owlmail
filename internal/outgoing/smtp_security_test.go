package outgoing

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/smtp"
	"strings"
	"testing"
	"time"
)

type testSMTPMode string

const (
	testSMTPPlain    testSMTPMode = "plain"
	testSMTPSTARTTLS testSMTPMode = "starttls"
	testSMTPS        testSMTPMode = "smtps"
)

func TestSendMailTransportModes(t *testing.T) {
	serverTLS, clientTLS := testTLSConfigs(t)
	tests := []struct {
		name       string
		serverMode testSMTPMode
		config     *OutgoingConfig
		auth       smtp.Auth
		want       []string
	}{
		{
			name:       "plain",
			serverMode: testSMTPPlain,
			config:     &OutgoingConfig{TLSMode: TLSModePlain},
			want:       []string{"MAIL FROM:<from@example.test>", "RCPT TO:<to@example.test>", "DATA", "QUIT"},
		},
		{
			name:       "mandatory STARTTLS and AUTH",
			serverMode: testSMTPSTARTTLS,
			config:     &OutgoingConfig{TLSMode: TLSModeSTARTTLS, tlsConfig: clientTLS, User: "relay", Password: "secret"},
			auth:       smtp.PlainAuth("", "relay", "secret", "127.0.0.1"),
			want:       []string{"STARTTLS", "AUTH PLAIN", "MAIL FROM:<from@example.test>", "RCPT TO:<to@example.test>", "DATA", "QUIT"},
		},
		{
			name:       "implicit TLS",
			serverMode: testSMTPS,
			config:     &OutgoingConfig{TLSMode: TLSModeSMTPS, tlsConfig: clientTLS},
			want:       []string{"MAIL FROM:<from@example.test>", "RCPT TO:<to@example.test>", "DATA", "QUIT"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			addr, commands := startTestSMTPServer(t, test.serverMode, serverTLS, false)
			err := sendMailWithConfig(context.Background(), addr, test.auth, "from@example.test", []string{"to@example.test"}, []byte("Subject: relay test\r\n\r\nbody"), test.config)
			if err != nil {
				t.Fatalf("sendMailWithConfig() error = %v", err)
			}
			got := <-commands
			for _, wanted := range test.want {
				if !containsCommand(got, wanted) {
					t.Errorf("SMTP commands = %q, missing %q", got, wanted)
				}
			}
		})
	}
}

func TestSTARTTLSFailsClosedWhenUnsupported(t *testing.T) {
	addr, commands := startTestSMTPServer(t, testSMTPPlain, nil, false)
	err := sendMailWithConfig(context.Background(), addr, nil, "from@example.test", []string{"to@example.test"}, []byte("body"), &OutgoingConfig{TLSMode: TLSModeSTARTTLS})
	if !errors.Is(err, ErrSTARTTLSUnsupported) {
		t.Fatalf("sendMailWithConfig() error = %v, want ErrSTARTTLSUnsupported", err)
	}
	got := <-commands
	for _, forbidden := range []string{"MAIL FROM:", "RCPT TO:", "DATA", "AUTH "} {
		if containsCommand(got, forbidden) {
			t.Errorf("SMTP commands = %q, unexpectedly contain %q", got, forbidden)
		}
	}
}

func TestSMTPSCertificateVerificationAndExplicitOptOut(t *testing.T) {
	serverTLS, _ := testTLSConfigs(t)
	addr, commands := startTestSMTPServer(t, testSMTPS, serverTLS, false)
	err := sendMailWithConfig(context.Background(), addr, nil, "from@example.test", []string{"to@example.test"}, []byte("body"), &OutgoingConfig{TLSMode: TLSModeSMTPS})
	if err == nil || !strings.Contains(err.Error(), "certificate") {
		t.Fatalf("sendMailWithConfig() error = %v, want certificate verification failure", err)
	}
	<-commands

	addr, commands = startTestSMTPServer(t, testSMTPS, serverTLS, false)
	err = sendMailWithConfig(context.Background(), addr, nil, "from@example.test", []string{"to@example.test"}, []byte("body"), &OutgoingConfig{
		TLSMode:            TLSModeSMTPS,
		InsecureSkipVerify: true,
	})
	if err != nil {
		t.Fatalf("explicit insecure-skip-verify relay error = %v", err)
	}
	<-commands
}

func TestOutgoingAuthenticationRequiresTLS(t *testing.T) {
	config := &OutgoingConfig{TLSMode: TLSModePlain, User: "relay", Password: "secret"}
	if err := config.Validate(); err == nil || !strings.Contains(err.Error(), "requires starttls or smtps") {
		t.Fatalf("Validate() error = %v, want clear-text AUTH rejection", err)
	}
}

func TestAbsoluteDeadlineKeepsElapsedPhaseBudget(t *testing.T) {
	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	deadline := phaseDeadline(context.Background(), 400*time.Millisecond)
	time.Sleep(250 * time.Millisecond)
	if err := setAbsoluteDeadline(context.Background(), client, deadline); err != nil {
		t.Fatal(err)
	}

	started := time.Now()
	_, err := client.Read(make([]byte, 1))
	elapsed := time.Since(started)
	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("Read() error = %v, want deadline timeout", err)
	}
	if elapsed >= 300*time.Millisecond {
		t.Fatalf("Read() took %s, absolute phase deadline was reset", elapsed)
	}
}

func TestSendMailDataDeadline(t *testing.T) {
	addr, commands := startTestSMTPServer(t, testSMTPPlain, nil, true)
	start := time.Now()
	err := sendMailWithConfig(context.Background(), addr, nil, "from@example.test", []string{"to@example.test"}, []byte("body"), &OutgoingConfig{
		TLSMode:     TLSModePlain,
		DataTimeout: "40ms",
	})
	if err == nil || time.Since(start) > time.Second {
		t.Fatalf("sendMailWithConfig() error = %v after %s, want bounded DATA timeout", err, time.Since(start))
	}
	<-commands
}

func TestSendMailEnvelopeDeadlineCoversAllRecipients(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		reader := bufio.NewReader(conn)
		_, _ = fmt.Fprint(conn, "220 localhost ESMTP\r\n")
		for {
			line, readErr := reader.ReadString('\n')
			if readErr != nil {
				return
			}
			command := strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(command, "EHLO "):
				_, _ = fmt.Fprint(conn, "250 localhost\r\n")
			case strings.HasPrefix(command, "MAIL FROM:"):
				_, _ = fmt.Fprint(conn, "250 ok\r\n")
			case strings.HasPrefix(command, "RCPT TO:"):
				time.Sleep(100 * time.Millisecond)
				if _, writeErr := fmt.Fprint(conn, "250 ok\r\n"); writeErr != nil {
					return
				}
			case command == "DATA":
				_, _ = fmt.Fprint(conn, "354 continue\r\n")
				for {
					dataLine, dataErr := reader.ReadString('\n')
					if dataErr != nil {
						return
					}
					if strings.TrimSpace(dataLine) == "." {
						break
					}
				}
				_, _ = fmt.Fprint(conn, "250 queued\r\n")
			case command == "QUIT":
				_, _ = fmt.Fprint(conn, "221 bye\r\n")
				return
			default:
				_, _ = fmt.Fprint(conn, "500 unexpected command\r\n")
			}
		}
	}()

	start := time.Now()
	err = sendMailWithConfig(
		context.Background(),
		listener.Addr().String(),
		nil,
		"from@example.test",
		[]string{"one@example.test", "two@example.test", "three@example.test"},
		[]byte("body"),
		&OutgoingConfig{TLSMode: TLSModePlain, EnvelopeTimeout: "150ms"},
	)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("sendMailWithConfig() error = nil, want the shared envelope deadline to expire")
	}
	if elapsed >= 250*time.Millisecond {
		t.Fatalf("sendMailWithConfig() took %s, envelope timeout was reset per recipient", elapsed)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed-out SMTP connection remained open")
	}
}

func TestSendMailCancellation(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		_, _ = bufio.NewReader(conn).ReadString('\n')
	}()

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(40*time.Millisecond, cancel)
	err = sendMailWithConfig(ctx, listener.Addr().String(), nil, "from@example.test", []string{"to@example.test"}, []byte("body"), &OutgoingConfig{TLSMode: TLSModePlain})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("sendMailWithConfig() error = %v, want context.Canceled", err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("canceled SMTP connection remained open")
	}
}

func TestOutgoingTimeoutValidation(t *testing.T) {
	fields := []struct {
		name  string
		apply func(*OutgoingConfig)
	}{
		{"connect", func(config *OutgoingConfig) { config.ConnectTimeout = "0s" }},
		{"TLS handshake", func(config *OutgoingConfig) { config.TLSHandshakeTimeout = "invalid" }},
		{"AUTH", func(config *OutgoingConfig) { config.AuthTimeout = "-1s" }},
		{"MAIL/RCPT", func(config *OutgoingConfig) { config.EnvelopeTimeout = "0" }},
		{"DATA", func(config *OutgoingConfig) { config.DataTimeout = "bad" }},
		{"QUIT", func(config *OutgoingConfig) { config.QuitTimeout = "0s" }},
	}
	for _, field := range fields {
		t.Run(field.name, func(t *testing.T) {
			config := &OutgoingConfig{TLSMode: TLSModeSTARTTLS}
			field.apply(config)
			if err := config.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want invalid timeout error")
			}
		})
	}
}

func testTLSConfigs(t *testing.T) (*tls.Config, *tls.Config) {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	certificate := server.TLS.Certificates[0]
	trustedCertificate := server.Certificate()
	server.Close()
	pool := x509.NewCertPool()
	pool.AddCert(trustedCertificate)
	return &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12}, &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
}

func startTestSMTPServer(t *testing.T, mode testSMTPMode, tlsConfig *tls.Config, stallData bool) (string, <-chan []string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	commands := make(chan []string, 1)
	go func() {
		seen := make([]string, 0, 12)
		defer func() { commands <- seen }()
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		tlsActive := false
		if mode == testSMTPS {
			tlsConn := tls.Server(conn, tlsConfig)
			if handshakeErr := tlsConn.Handshake(); handshakeErr != nil {
				return
			}
			conn = tlsConn
			tlsActive = true
		}
		reader := bufio.NewReader(conn)
		_, _ = fmt.Fprint(conn, "220 localhost ESMTP\r\n")
		for {
			line, readErr := reader.ReadString('\n')
			if readErr != nil {
				return
			}
			command := strings.TrimSpace(line)
			seen = append(seen, command)
			switch {
			case strings.HasPrefix(command, "EHLO "):
				if mode == testSMTPSTARTTLS && !tlsActive {
					_, _ = fmt.Fprint(conn, "250-localhost\r\n250 STARTTLS\r\n")
				} else if tlsActive {
					_, _ = fmt.Fprint(conn, "250-localhost\r\n250 AUTH PLAIN\r\n")
				} else {
					_, _ = fmt.Fprint(conn, "250 localhost\r\n")
				}
			case command == "STARTTLS":
				_, _ = fmt.Fprint(conn, "220 ready for TLS\r\n")
				tlsConn := tls.Server(conn, tlsConfig)
				if handshakeErr := tlsConn.Handshake(); handshakeErr != nil {
					return
				}
				conn = tlsConn
				reader = bufio.NewReader(conn)
				tlsActive = true
			case strings.HasPrefix(command, "AUTH PLAIN"):
				_, _ = fmt.Fprint(conn, "235 authenticated\r\n")
			case strings.HasPrefix(command, "MAIL FROM:"), strings.HasPrefix(command, "RCPT TO:"):
				_, _ = fmt.Fprint(conn, "250 ok\r\n")
			case command == "DATA":
				_, _ = fmt.Fprint(conn, "354 continue\r\n")
				for {
					dataLine, dataErr := reader.ReadString('\n')
					if dataErr != nil {
						return
					}
					if strings.TrimSpace(dataLine) == "." {
						break
					}
				}
				if stallData {
					time.Sleep(200 * time.Millisecond)
				}
				_, _ = fmt.Fprint(conn, "250 queued\r\n")
			case command == "QUIT":
				_, _ = fmt.Fprint(conn, "221 bye\r\n")
				return
			default:
				_, _ = fmt.Fprint(conn, "500 unexpected command\r\n")
			}
		}
	}()
	return listener.Addr().String(), commands
}

func containsCommand(commands []string, prefix string) bool {
	for _, command := range commands {
		if strings.HasPrefix(command, prefix) {
			return true
		}
	}
	return false
}
