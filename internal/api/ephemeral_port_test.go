package api

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/mailserver"
)

func TestEphemeralHTTPPortAndOrigins(t *testing.T) {
	server, err := mailserver.NewMailServer(0, "127.0.0.1", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	for _, host := range []string{"127.0.0.1", "[::1]"} {
		t.Run(host, func(t *testing.T) {
			if host == "[::1]" {
				probe, err := net.Listen("tcp", "[::1]:0")
				if err != nil {
					t.Skipf("IPv6 unavailable: %v", err)
				}
				_ = probe.Close()
			}
			api := NewAPI(server, 0, host)
			api.SetMailDevRESTCompat(true)
			if _, ok := api.BoundAddress(); ok {
				t.Fatal("address available before binding")
			}
			ready := make(chan struct{})
			result := make(chan error, 1)
			go func() { result <- api.StartWithReady(func() { close(ready) }) }()
			select {
			case <-ready:
			case err := <-result:
				t.Fatal(err)
			case <-time.After(5 * time.Second):
				t.Fatal("HTTP bind timeout")
			}
			defer func() {
				_ = api.Close()
				select {
				case <-result:
				case <-time.After(5 * time.Second):
					t.Error("HTTP shutdown timeout")
				}
			}()
			address, ok := api.BoundAddress()
			if !ok || address.Port == 0 || address.Port != api.GetPort() {
				t.Fatal("no actual HTTP port")
			}
			target := "http://" + net.JoinHostPort(address.Host, fmt.Sprint(address.Port))
			request, err := http.NewRequest(http.MethodGet, target+"/api/v1/settings", nil)
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Origin", target)
			client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{Proxy: nil}}
			response, err := client.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = response.Body.Close() }()
			if response.StatusCode != 200 {
				t.Fatalf("actual-port origin rejected: %d", response.StatusCode)
			}
			var settings struct {
				Web struct {
					Port int `json:"port"`
				} `json:"web"`
			}
			if err := json.NewDecoder(response.Body).Decode(&settings); err != nil {
				t.Fatal(err)
			}
			if settings.Web.Port != address.Port {
				t.Fatalf("settings port %d != %d", settings.Web.Port, address.Port)
			}
		})
	}
}

func TestEphemeralHTTPReadyErrorReleasesAddress(t *testing.T) {
	server, err := mailserver.NewMailServer(0, "127.0.0.1", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	api := NewAPI(server, 0, "127.0.0.1")
	var address mailserver.ListenerAddress
	sentinel := fmt.Errorf("publication failed")
	err = api.StartWithCertificateReady(func(_ [][]byte) error { address, _ = api.BoundAddress(); return sentinel })
	if err != sentinel {
		t.Fatalf("callback error lost: %v", err)
	}
	if _, ok := api.BoundAddress(); ok {
		t.Fatal("failed start retained a bound address")
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(address.Host, fmt.Sprint(address.Port)))
	if err != nil {
		t.Fatalf("failed startup retained socket: %v", err)
	}
	_ = listener.Close()
}
