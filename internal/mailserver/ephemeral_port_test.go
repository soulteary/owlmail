package mailserver

import (
	"fmt"
	"net"
	"net/smtp"
	"sync"
	"testing"
	"time"
)

func TestEphemeralSMTPPorts(t *testing.T) {
	servers := make([]*MailServer, 0, 2)
	for range 2 {
		server, err := NewMailServerWithOptions(0, "127.0.0.1", t.TempDir(), ServerOptions{AllowEphemeralPort: true})
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := server.BoundAddress(); ok || server.GetPort() != 0 {
			t.Fatal("address available before binding")
		}
		ready := make(chan struct{})
		result := make(chan error, 1)
		go func() { result <- server.ListenWithReady(func() { close(ready) }) }()
		select {
		case <-ready:
		case err := <-result:
			t.Fatal(err)
		case <-time.After(5 * time.Second):
			t.Fatal("SMTP bind timed out")
		}
		servers = append(servers, server)
		t.Cleanup(func() {
			_ = server.Close()
			select {
			case <-result:
			case <-time.After(5 * time.Second):
				t.Error("SMTP did not stop")
			}
		})
	}
	if servers[0].GetPort() == servers[1].GetPort() {
		t.Fatal("OS-selected listeners share a port")
	}
	var wg sync.WaitGroup
	for i, server := range servers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			address, ok := server.BoundAddress()
			if !ok {
				t.Error("no bound address")
				return
			}
			target := net.JoinHostPort(address.Host, fmt.Sprint(address.Port))
			if err := smtp.SendMail(target, nil, "sender@example.test", []string{"to@example.test"}, fmt.Appendf(nil, "From: sender@example.test\r\nTo: to@example.test\r\nSubject: worker-%d\r\n\r\nhello\r\n", i)); err != nil {
				t.Errorf("SMTP delivery: %v", err)
			}
		}()
	}
	wg.Wait()
	for _, server := range servers {
		if len(server.GetAllEmail()) != 1 {
			t.Fatal("isolated mailbox did not capture one message")
		}
	}
}

func TestEphemeralSMTPCompatibilityAndConcurrentAddressReads(t *testing.T) {
	legacy, err := NewMailServer(0, "127.0.0.1", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = legacy.Close() }()
	if legacy.GetPort() != defaultPort {
		t.Fatalf("legacy zero port changed: %d", legacy.GetPort())
	}
	server, err := NewMailServerWithOptions(0, "127.0.0.1", t.TempDir(), ServerOptions{AllowEphemeralPort: true})
	if err != nil {
		t.Fatal(err)
	}
	ready := make(chan struct{})
	result := make(chan error, 1)
	stop := make(chan struct{})
	var readers sync.WaitGroup
	for range 4 {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_, _ = server.BoundAddress()
					_ = server.GetPort()
				}
			}
		}()
	}
	go func() { result <- server.ListenWithReady(func() { close(ready) }) }()
	select {
	case <-ready:
	case err := <-result:
		t.Fatal(err)
	case <-time.After(5 * time.Second):
		t.Fatal("bind timeout")
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		_, status := server.GetServiceReadiness()
		if status.State == "ready" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("storage readiness did not complete")
		}
		time.Sleep(time.Millisecond)
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-result:
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown timeout")
	}
	close(stop)
	readers.Wait()
	if _, ok := server.BoundAddress(); ok {
		t.Fatal("bound address retained after shutdown")
	}
	if server.GetPort() != 0 {
		t.Fatal("requested dynamic port was mutated")
	}
}

func TestEphemeralShutdownRejectsLateRegistrations(t *testing.T) {
	server, err := NewMailServerWithOptions(0, "127.0.0.1", t.TempDir(), ServerOptions{AllowEphemeralPort: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	if server.setSMTPListener(listener) || server.setSMTPSListener(listener) {
		t.Fatal("closed owner accepted listener")
	}
	if err := server.AddCloser(listener); err == nil {
		t.Fatal("closed owner accepted closer")
	}
	called := false
	if err := server.ListenWithReady(func() { called = true }); err == nil || called {
		t.Fatalf("closed server restarted: err=%v ready=%v", err, called)
	}
}
