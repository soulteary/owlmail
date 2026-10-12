package outgoing

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestProbeSMTPTransportAndAuthenticationWithoutDelivery(t *testing.T) {
	serverTLS, clientTLS := testTLSConfigs(t)
	for _, mode := range []testSMTPMode{testSMTPPlain, testSMTPSTARTTLS, testSMTPS} {
		t.Run(string(mode), func(t *testing.T) {
			addr, commands := startTestSMTPServer(t, mode, serverTLS, false, "PLAIN", 0)
			host, port, _ := net.SplitHostPort(addr)
			number, _ := strconv.Atoi(port)
			cfg := &OutgoingConfig{Host: host, Port: number, TLSMode: TLSMode(mode), tlsConfig: clientTLS}
			if mode != testSMTPPlain {
				cfg.User = "relay"
				cfg.Password = "secret"
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := ProbeSMTP(ctx, cfg); err != nil {
				t.Fatal(err)
			}
			got := <-commands
			for _, command := range got {
				if strings.HasPrefix(command, "MAIL ") || strings.HasPrefix(command, "RCPT ") || command == "DATA" {
					t.Fatalf("probe submitted mail: %q", got)
				}
			}
			if !containsCommand(got, "EHLO localhost") || !containsCommand(got, "QUIT") {
				t.Fatalf("incomplete handshake: %q", got)
			}
			if mode != testSMTPPlain && !containsCommand(got, "AUTH PLAIN") {
				t.Fatalf("authentication not checked: %q", got)
			}
		})
	}
}

func TestProbeSMTPRejectsUnavailableTLSAndAuthentication(t *testing.T) {
	serverTLS, clientTLS := testTLSConfigs(t)
	for _, test := range []struct {
		name string
		mode testSMTPMode
		cfg  *OutgoingConfig
		want error
	}{
		{"missing STARTTLS", testSMTPPlain, &OutgoingConfig{TLSMode: TLSModeSTARTTLS}, ErrSTARTTLSUnsupported},
		{"missing AUTH", testSMTPSTARTTLS, &OutgoingConfig{TLSMode: TLSModeSTARTTLS, tlsConfig: clientTLS, User: "relay", Password: "secret"}, ErrAUTHPlainUnsupported},
		{"untrusted certificate", testSMTPS, &OutgoingConfig{TLSMode: TLSModeSMTPS}, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			addr, commands := startTestSMTPServer(t, test.mode, serverTLS, false, "", 0)
			test.cfg.Host, addr, _ = net.SplitHostPort(addr)
			test.cfg.Port, _ = strconv.Atoi(addr)
			err := ProbeSMTP(context.Background(), test.cfg)
			if err == nil || (test.want != nil && !errors.Is(err, test.want)) {
				t.Fatalf("unexpected error: %v", err)
			}
			for _, command := range <-commands {
				if strings.HasPrefix(command, "MAIL ") || command == "DATA" {
					t.Fatalf("sent %s", command)
				}
			}
		})
	}
}

func TestProbeSMTPDeadlineClosesConnection(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		buffer := make([]byte, 1)
		_, _ = conn.Read(buffer)
	}()
	host, port, _ := net.SplitHostPort(listener.Addr().String())
	number, _ := strconv.Atoi(port)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := ProbeSMTP(ctx, &OutgoingConfig{Host: host, Port: number}); err == nil {
		t.Fatal("stalled greeting passed")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("probe connection remained open")
	}
}
