package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/config"
)

func TestContainerServerArgs(t *testing.T) {
	for _, test := range []struct {
		name    string
		command string
		want    []string
	}{
		{"defaults", "/app/owlmail\x00", []string{}},
		{"flags", "/app/owlmail\x00-web\x009999\x00-config\x00/app/config.yaml\x00", []string{"-web", "9999", "-config", "/app/config.yaml"}},
		{"final empty value", "/app/owlmail\x00-disable-web\x00-smtp\x0018084\x00-ip\x00\x00", []string{"-disable-web", "-smtp", "18084", "-ip", ""}},
		{"empty values", "/app/owlmail\x00-ip\x00\x00-web-ip\x00\x00", []string{"-ip", "", "-web-ip", ""}},
		{"consecutive trailing empty arguments", "/app/owlmail\x00-web-user\x00\x00\x00", []string{"-web-user", "", ""}},
		{"unterminated command", "/app/owlmail\x00-web\x009999", []string{"-web", "9999"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			args, err := containerServerArgs([]byte(test.command))
			if err != nil || !reflect.DeepEqual(args, test.want) {
				t.Fatalf("args=%q want=%q err=%v", args, test.want, err)
			}
		})
	}
	for _, command := range []string{"/app/owlmail\x00doctor\x00", "/app/owlmail\x00mcp-stdio\x00", "/app/owlmail\x00\x00", "/bin/sh\x00", ""} {
		if _, err := containerServerArgs([]byte(command)); err == nil {
			t.Fatalf("accepted non-server command %q", command)
		}
	}
}

func TestContainerServerArgsPreservesEmptyFinalFlagValue(t *testing.T) {
	t.Setenv("OWLMAIL_SMTP_HOST", "127.0.0.1")
	args, err := containerServerArgs([]byte("/app/owlmail\x00-disable-web\x00-smtp\x0018084\x00-ip\x00\x00"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := commandConfig("healthcheck", args)
	if err != nil {
		t.Fatalf("empty final flag value failed configuration parsing: %v", err)
	}
	if !cfg.DisableWeb || cfg.SMTPPort != 18084 || cfg.SMTPHost != "" {
		t.Fatalf("startup options changed: disable-web=%t smtp=%d ip=%q", cfg.DisableWeb, cfg.SMTPPort, cfg.SMTPHost)
	}
}

func TestProbeHealth(t *testing.T) {
	for _, secure := range []bool{false, true} {
		t.Run(strconv.FormatBool(secure), func(t *testing.T) {
			status := http.StatusOK
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/mail/healthz" {
					t.Errorf("path=%s", r.URL.Path)
				}
				w.WriteHeader(status)
			}))
			if secure {
				server.StartTLS()
			} else {
				server.Start()
			}
			defer server.Close()
			host, port, _ := net.SplitHostPort(server.Listener.Addr().String())
			cfg := config.DefaultConfig()
			cfg.WebHost = host
			cfg.WebPort, _ = strconv.Atoi(port)
			cfg.BasePathname = "/mail"
			cfg.HTTPSEnabled = secure
			if secure {
				pair := server.TLS.Certificates[0]
				cfg.HTTPSCertFile = filepath.Join(t.TempDir(), "cert.pem")
				if err := os.WriteFile(cfg.HTTPSCertFile, append(
					pem.EncodeToMemory(&pem.Block{Type: "EC PARAMETERS", Bytes: []byte{0x06, 0x08, 0x2a, 0x86, 0x48, 0xce, 0x3d, 0x03, 0x01, 0x07}}),
					pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: pair.Certificate[0]})...), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := probeHealth(cfg); err != nil {
				t.Fatal(err)
			}
			status = http.StatusServiceUnavailable
			if err := probeHealth(cfg); err == nil {
				t.Fatal("accepted unhealthy response")
			}
			status = http.StatusFound
			if err := probeHealth(cfg); err == nil {
				t.Fatal("accepted redirect")
			}
		})
	}
}

func TestHealthcheckConfigUsesStartupPrecedence(t *testing.T) {
	t.Setenv("OWLMAIL_WEB_PORT", "9001")
	cfg, err := commandConfig("healthcheck", []string{"-web", "9002", "-base-pathname", "/mail"})
	if err != nil || cfg.WebPort != 9002 || cfg.BasePathname != "/mail" {
		t.Fatalf("config=%+v err=%v", cfg, err)
	}
}

func TestHealthcheckConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("web: 9003\nbase-pathname: /configured\nhttps: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := commandConfig("healthcheck", []string{"-config", path})
	if err != nil || cfg.WebPort != 9003 || !cfg.HTTPSEnabled || cfg.BasePathname != "/configured" {
		t.Fatalf("config=%+v err=%v", cfg, err)
	}
}

func TestHealthcheckIPv6Hosts(t *testing.T) {
	for input, want := range map[string]string{"[::1]": "::1", "[::]": "::1", "::": "::1", "::1": "::1", "[2001:db8::1]": "2001:db8::1", "0.0.0.0": "127.0.0.1", "localhost": "localhost"} {
		if got := healthcheckHost(input); got != want {
			t.Errorf("healthcheckHost(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestProbeHealthBracketedIPv6(t *testing.T) {
	listener, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 loopback unavailable: %v", err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	server.Listener = listener
	server.Start()
	defer server.Close()
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultConfig()
	cfg.WebHost = "[::1]"
	cfg.WebPort, err = strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	if err := probeHealth(cfg); err != nil {
		t.Fatal(err)
	}
}

func TestProbeHealthExternalIPCertificate(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IPAddresses: []net.IP{net.ParseIP("192.0.2.10")}, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}, MinVersion: tls.VersionTLS12}
	server.StartTLS()
	defer server.Close()
	_, port, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultConfig()
	cfg.WebHost = "0.0.0.0"
	cfg.WebPort, err = strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	cfg.HTTPSEnabled = true
	cfg.HTTPSCertFile = filepath.Join(t.TempDir(), "cert.pem")
	if err := os.WriteFile(cfg.HTTPSCertFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
	if err := probeHealth(cfg); err != nil {
		t.Fatal(err)
	}
}

func TestHealthcheckScopedIPv6URL(t *testing.T) {
	raw, err := healthcheckURL("http", healthcheckHost("[fe80::1%eth0]"), 1080, "/mail")
	if err != nil {
		t.Fatal(err)
	}
	if raw != "http://[fe80::1%25eth0]:1080/mail/healthz" {
		t.Fatalf("URL=%q", raw)
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Hostname() != "fe80::1%eth0" {
		t.Fatalf("hostname=%q", parsed.Hostname())
	}
}

func TestProbeHealthEscapedBasePaths(t *testing.T) {
	for _, base := range []string{"/team%20mail", "/caf%C3%A9"} {
		t.Run(base, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.RequestURI != base+"/healthz" {
					t.Errorf("RequestURI=%q", r.RequestURI)
					w.WriteHeader(http.StatusNotFound)
					return
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()
			host, port, err := net.SplitHostPort(server.Listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			cfg := config.DefaultConfig()
			cfg.WebHost = host
			cfg.WebPort, err = strconv.Atoi(port)
			if err != nil {
				t.Fatal(err)
			}
			cfg.BasePathname = base
			if err := probeHealth(cfg); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestReadyProbeTargetsReadinessWithoutChangingDefaultLiveness(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/mail/readyz" {
			w.WriteHeader(503)
		} else if r.URL.Path != "/mail/healthz" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	host, port, _ := net.SplitHostPort(server.Listener.Addr().String())
	cfg := config.DefaultConfig()
	cfg.WebHost = host
	cfg.WebPort, _ = strconv.Atoi(port)
	cfg.BasePathname = "/mail"
	if err := probeHealthEndpoint(cfg, false); err != nil {
		t.Fatal(err)
	}
	if err := probeHealthEndpoint(cfg, true); err == nil {
		t.Fatal("readiness failure was accepted")
	}
}
