package main

import (
	"bytes"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/config"
)

func writeProbeProcessStat(t *testing.T, path, identity string) {
	t.Helper()
	// The command name deliberately includes spaces and an embedded ')'.
	fields := append([]string{"S"}, strings.Fields(strings.Repeat("0 ", 18))...)
	fields = append(fields, identity, "0", "0")
	if err := os.WriteFile(path, []byte("1 (owlmail ) server) "+strings.Join(fields, " ")), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestContainerProbeUsesImmutablePublicStartupSettings(t *testing.T) {
	for _, secure := range []bool{false, true} {
		t.Run(strconv.FormatBool(secure), func(t *testing.T) {
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/startup/healthz" && r.URL.Path != "/startup/readyz" {
					t.Errorf("unexpected path %s", r.URL.Path)
				}
				w.WriteHeader(http.StatusOK)
			}))
			if secure {
				server.StartTLS()
			} else {
				server.Start()
			}
			defer server.Close()
			dir := t.TempDir()
			stat := filepath.Join(dir, "stat")
			path := filepath.Join(dir, "probe.json")
			writeProbeProcessStat(t, stat, "12345")
			host, port, _ := net.SplitHostPort(server.Listener.Addr().String())
			configuration := filepath.Join(dir, "config.yaml")
			if err := os.WriteFile(configuration, []byte(fmt.Sprintf("web: %s\nweb-ip: %s\nbase-pathname: /startup\n", port, host)), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := commandConfig("healthcheck", []string{"-config", configuration})
			if err != nil {
				t.Fatal(err)
			}
			cfg.HTTPSEnabled = secure
			cfg.WebPassword = "secret-web"
			cfg.SMTPPassword = "secret-smtp"
			cfg.OutgoingPass = "secret-relay"
			cfg.S3SecretAccessKey = "secret-s3"
			cfg.WebhookRedisURL = "redis://secret-redis@localhost"
			if secure {
				cfg.HTTPSCertFile = filepath.Join(dir, "combined.pem")
				certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.TLS.Certificates[0].Certificate[0]})
				combined := append(certificate, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("secret-private-key")})...)
				if err := os.WriteFile(cfg.HTTPSCertFile, combined, 0600); err != nil {
					t.Fatal(err)
				}
			}
			var certificates [][]byte
			if secure {
				certificates = server.TLS.Certificates[0].Certificate
			}
			if err := publishContainerProbeConfig(cfg, dir, path, stat, certificates...); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, secret := range []string{"secret-web", "secret-smtp", "secret-relay", "secret-s3", "secret-redis", "PRIVATE KEY"} {
				if strings.Contains(string(data), secret) {
					t.Fatalf("startup record exposes %s", secret)
				}
			}
			if err := os.Remove(configuration); err != nil {
				t.Fatal(err)
			}
			if secure {
				if err := os.Remove(cfg.HTTPSCertFile); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("OWLMAIL_WEB_PORT", "1")
			t.Setenv("OWLMAIL_DISABLE_WEB", "true")
			probe, err := readContainerProbeConfig(path, stat)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(probe.CertificatePEM), "PRIVATE KEY") {
				t.Fatal("startup record contains a private key")
			}
			for _, ready := range []bool{false, true} {
				if err := probeHealthEndpointWithCertificate(probe.config(), ready, probe.CertificatePEM); err != nil {
					t.Fatal(err)
				}
			}
			info, err := os.Stat(path)
			if err != nil || info.Mode().Perm() != 0600 {
				t.Fatalf("startup record is not private: %v %v", info, err)
			}
		})
	}
}

func TestContainerProbeRejectsMissingInvalidAndPreviousProcessRecord(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "probe.json")
	stat := filepath.Join(dir, "stat")
	writeProbeProcessStat(t, stat, "100")
	if _, err := readContainerProbeConfig(path, stat); err == nil {
		t.Fatal("accepted missing startup record")
	}
	cfg := config.DefaultConfig()
	cfg.DisableWeb = true
	cfg.SMTPHost = ""
	cfg.HTTPSEnabled = true // Irrelevant web certificate must not be read.
	cfg.HTTPSCertFile = filepath.Join(dir, "missing.pem")
	if err := publishContainerProbeConfig(cfg, dir, path, stat); err != nil {
		t.Fatal(err)
	}
	probe, err := readContainerProbeConfig(path, stat)
	if err != nil || probe.SMTPHost != "localhost" || !probe.DisableWeb {
		t.Fatalf("effective SMTP-only config: %+v, %v", probe, err)
	}
	writeProbeProcessStat(t, stat, "101")
	if _, err := readContainerProbeConfig(path, stat); err == nil {
		t.Fatal("accepted previous PID 1 process record")
	}
	for _, data := range []string{"{}", "invalid", strings.Repeat("x", maxContainerProbeConfigBytes+1)} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readContainerProbeConfig(path, stat); err == nil {
			t.Fatal("accepted invalid record")
		}
	}
	if err := publishContainerProbeConfig(cfg, dir, path, stat); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := readContainerProbeConfig(path, stat); err == nil {
		t.Fatal("accepted public record")
	}
}

func TestProcessStartIdentityRejectsMalformedStat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stat")
	for _, data := range []string{"", "1 owlmail S", "1 (owlmail) S", "1 (owlmail) " + strings.Repeat("x ", 20)} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := processStartIdentity(path); err == nil {
			t.Fatalf("accepted malformed proc stat %q", data)
		}
	}
}

func TestContainerProbeCertificateComesFromListener(t *testing.T) {
	listener := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer listener.Close()
	dir := t.TempDir()
	stat := filepath.Join(dir, "stat")
	path := filepath.Join(dir, "probe.json")
	writeProbeProcessStat(t, stat, "100")
	cfg := config.DefaultConfig()
	cfg.HTTPSEnabled = true
	cfg.HTTPSCertFile = filepath.Join(dir, "rotated.pem")
	if err := os.WriteFile(cfg.HTTPSCertFile, []byte("rotated certificate input"), 0600); err != nil {
		t.Fatal(err)
	}
	chain := listener.TLS.Certificates[0].Certificate
	if err := publishContainerProbeConfig(cfg, dir, path, stat, chain...); err != nil {
		t.Fatal("publisher reloaded the mounted certificate:", err)
	}
	probe, err := readContainerProbeConfig(path, stat)
	if err != nil {
		t.Fatal(err)
	}
	block, rest := pem.Decode(probe.CertificatePEM)
	if block == nil || block.Type != "CERTIFICATE" || !bytes.Equal(block.Bytes, chain[0]) || len(rest) != 0 {
		t.Fatal("probe did not retain the listener's actual certificate")
	}
	if err := publishContainerProbeConfig(cfg, dir, path, stat); err == nil {
		t.Fatal("accepted HTTPS record without a listener certificate")
	}
}

func TestContainerProbeConfigReadIsBounded(t *testing.T) {
	if _, err := exec.LookPath("mkfifo"); err != nil {
		t.Skip("mkfifo unavailable")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "probe.json")
	if err := exec.Command("mkfifo", path).Run(); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if _, err := readContainerProbeConfig(path, filepath.Join(dir, "stat")); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("blocked read returned %v", err)
	}
	if time.Since(started) > 2*time.Second {
		t.Fatal("blocked configuration read exceeded its deadline")
	}
	// Release the one reader goroutine before removing the test directory.
	writer, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
}
