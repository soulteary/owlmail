package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorIsOfflineRedactedAndDoesNotCreateStorage(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "not-created")
	var stdout, stderr bytes.Buffer
	err := runDoctor([]string{"-mail-directory", directory}, &stdout, &stderr)
	if err != nil {
		t.Fatal(err, stdout.String())
	}
	var report doctorReport
	if err = json.Unmarshal(stdout.Bytes(), &report); err != nil || !report.Ready {
		t.Fatal(err, report)
	}
	if _, err = os.Stat(directory); !os.IsNotExist(err) {
		t.Fatal("doctor created storage", err)
	}
	stdout.Reset()
	secret := "private-secret-123"
	err = runDoctor([]string{"-mail-directory", directory, "-web-user", "user", "-web-password", secret, "-web-allowed-origins", "https://user:" + secret + "@example.test"}, &stdout, &stderr)
	if err == nil || strings.Contains(stdout.String()+stderr.String(), secret) {
		t.Fatal("failed validation or secret redaction", stdout.String(), stderr.String())
	}
}
func TestDoctorUsesNormalConfigPrecedenceAndFindsLocalFailures(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"smtp":1026,"web":1081}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OWLMAIL_SMTP_PORT", "1027")
	cfg, err := doctorConfig([]string{"-config", path, "-smtp", "1028"})
	if err != nil || cfg.SMTPPort != 1028 || cfg.WebPort != 1081 {
		t.Fatal(cfg, err)
	}
	cfg.WebPort = cfg.SMTPPort
	cfg.WebUser = "partial"
	cfg.TLSEnabled = true
	cfg.TLSCertFile = "missing"
	cfg.TLSKeyFile = "missing"
	report := inspectDoctorConfig(cfg)
	if report.Ready {
		t.Fatal(report)
	}
	failed := make(map[string]bool)
	for _, check := range report.Checks {
		if check.Status == "fail" {
			failed[check.Name] = true
		}
	}
	if !failed["listener_ports"] || !failed["web_auth"] || !failed["smtp_tls"] {
		t.Fatal(failed)
	}
}
func TestDoctorMalformedConfigDoesNotEchoValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	secret := "secret-in-invalid-value"
	if err := os.WriteFile(path, []byte(`{"smtp":"`+secret+`"}`), 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := runDoctor([]string{"-config", path}, &stdout, &stderr); err == nil {
		t.Fatal("accepted invalid config")
	}
	if strings.Contains(stdout.String()+stderr.String(), secret) {
		t.Fatal("echoed config secret")
	}
}
