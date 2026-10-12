package api

import (
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/mailserver"
)

func TestAPIStartWithIPv6Listener(t *testing.T) {
	listener, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 unavailable: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	server, err := mailserver.NewMailServer(0, "127.0.0.1", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	api := NewAPI(server, port, "::1")
	ready := make(chan struct{})
	finished := make(chan error, 1)
	go func() { finished <- api.StartWithReady(func() { close(ready) }) }()
	select {
	case <-ready:
	case err := <-finished:
		t.Fatal(err)
	case <-time.After(3 * time.Second):
		t.Fatal("IPv6 API did not bind")
	}
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://[::1]:%d/healthz", port))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("IPv6 health status %d", resp.StatusCode)
	}
	if err := api.app.Shutdown(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("IPv6 API did not stop")
	}
}

func TestAPIRejectsMalformedBracketedHosts(t *testing.T) {
	server, err := mailserver.NewMailServer(0, "127.0.0.1", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	for _, host := range []string{"[", "]", "[]", "[::1", "::1]", "[[::1]]", "[localhost]", "[127.0.0.1]"} {
		t.Run(host, func(t *testing.T) {
			api := NewAPI(server, 0, host)
			ready := false
			if err := api.StartWithReady(func() { ready = true; _ = api.app.Shutdown() }); err == nil || ready {
				t.Fatalf("malformed host bound a listener: %q", host)
			}
		})
	}
}
