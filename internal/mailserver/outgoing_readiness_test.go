package mailserver

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/attachmentstore"
	"github.com/soulteary/owlmail/internal/outgoing"
)

func startReadinessSMTP(t *testing.T) (*outgoing.OutgoingConfig, <-chan []string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	commands := make(chan []string, 8)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
				_, _ = fmt.Fprint(conn, "220 readiness SMTP\r\n")
				var seen []string
				scanner := bufio.NewScanner(conn)
				for scanner.Scan() {
					line := scanner.Text()
					seen = append(seen, line)
					if line == "QUIT" {
						_, _ = fmt.Fprint(conn, "221 bye\r\n")
						commands <- seen
						return
					}
					_, _ = fmt.Fprint(conn, "250 readiness SMTP\r\n")
				}
			}()
		}
	}()
	return &outgoing.OutgoingConfig{Host: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port}, commands
}

func TestOutgoingReadinessUsesLiveConfigurationAndCachesSafeStatus(t *testing.T) {
	cfg, commands := startReadinessSMTP(t)
	server, err := NewMailServerWithOptions(1025, "localhost", t.TempDir(), ServerOptions{OutgoingConfig: cfg})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	if err := server.StartRelayRecovery(); err != nil {
		t.Fatal(err)
	}
	server.RelayService().CheckPersistence()
	server.smtpReady.Store(true)
	server.probeMailStorage()
	if err := server.GetRuntimeReadiness().ValidateService(time.Now()); err == nil {
		t.Fatal("unprobed transport was ready")
	}
	server.probeOutgoingReadiness(context.Background())
	if err := server.GetRuntimeReadiness().ValidateService(time.Now()); err != nil {
		t.Fatal(err)
	}
	for _, command := range <-commands {
		if strings.HasPrefix(command, "MAIL") || strings.HasPrefix(command, "RCPT") || command == "DATA" {
			t.Fatalf("probe delivered mail: %s", command)
		}
	}
	// Readiness reads must not create any extra downstream connection.
	for range 3 {
		_ = server.GetRuntimeReadiness()
	}
	select {
	case got := <-commands:
		t.Fatalf("cache reader contacted downstream: %v", got)
	default:
	}
	if err := server.SetOutgoingConfig(&outgoing.OutgoingConfig{Host: "127.0.0.1", Port: 1}); err != nil {
		t.Fatal(err)
	}
	if status := server.GetRuntimeReadiness(); status.Transport == nil || status.Transport.State != attachmentstore.HealthChecking || status.ValidateService(time.Now()) == nil {
		t.Fatalf("configuration retained old healthy result: %+v", status)
	}
	server.probeOutgoingReadiness(context.Background())
	if status := server.GetRuntimeReadiness(); status.Transport == nil || status.Transport.State != attachmentstore.HealthUnready {
		t.Fatalf("unreachable transport ready: %+v", status)
	}
	if err := server.SetOutgoingConfig(cfg); err != nil {
		t.Fatal(err)
	}
	server.probeOutgoingReadiness(context.Background())
	if err := server.GetRuntimeReadiness().ValidateService(time.Now()); err != nil {
		t.Fatal(err)
	}
	server.readinessMutex.Lock()
	server.outgoingTransportHealth.CheckedAt = time.Now().Add(-time.Minute)
	server.readinessMutex.Unlock()
	if status := server.GetRuntimeReadiness(); status.Transport.State != attachmentstore.HealthUnready || status.ValidateService(time.Now()) == nil {
		t.Fatal("stale transport remained ready")
	}
	if err := server.SetOutgoingConfig(&outgoing.OutgoingConfig{}); err != nil {
		t.Fatal(err)
	}
	if status := server.GetRuntimeReadiness(); status.Transport != nil || status.ValidateService(time.Now()) != nil {
		t.Fatalf("disabled transport remained a dependency: %+v", status)
	}
}

func TestOutgoingReadinessDiscardsResultForPreviousConfiguration(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			accepted <- conn
		}
	}()
	server, err := NewMailServerWithOutgoing(1025, "localhost", t.TempDir(), &outgoing.OutgoingConfig{Host: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	finished := make(chan struct{})
	go func() { server.probeOutgoingReadiness(ctx); close(finished) }()
	var conn net.Conn
	select {
	case conn = <-accepted:
	case <-time.After(time.Second):
		t.Fatal("probe never connected")
	}
	defer func() { _ = conn.Close() }()
	if err := server.SetOutgoingConfig(&outgoing.OutgoingConfig{Host: "127.0.0.1", Port: 1}); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("canceled probe did not stop")
	}
	if status := server.GetRuntimeReadiness(); status.Transport == nil || status.Transport.State != attachmentstore.HealthChecking {
		t.Fatal("previous configuration overwrote pending result")
	}
}

func TestSharedReadinessExpiresStorageCache(t *testing.T) {
	server, err := NewMailServer(1025, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	server.smtpReady.Store(true)
	server.mailStorageHealth = attachmentstore.HealthStatus{State: attachmentstore.HealthReady, CheckedAt: time.Now().Add(-time.Minute)}
	status := server.GetRuntimeReadiness()
	if status.Storage.State != attachmentstore.HealthUnready || status.Storage.ErrorCategory != attachmentstore.HealthErrorTimeout || status.ValidateService(time.Now()) == nil {
		t.Fatalf("stale storage accepted: %+v", status)
	}
}
