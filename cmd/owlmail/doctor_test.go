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

func TestDoctorListenerAddressOverlap(t *testing.T) {
	for _, tc := range []struct {
		left, right        string
		overlap, uncertain bool
	}{
		{"127.0.0.1", "127.0.0.2", false, false},
		{"127.0.0.1", "127.0.0.1", true, false},
		{"0.0.0.0", "127.0.0.2", true, false},
		{"::", "127.0.0.1", true, false},
		{"localhost", "127.0.0.1", false, true},
	} {
		got, uncertain := doctorListenerOverlap(tc.left, 1025, tc.right, 1025)
		if got != tc.overlap || uncertain != tc.uncertain {
			t.Fatalf("%+v: %v %v", tc, got, uncertain)
		}
	}
}
func TestDoctorRejectsManagedIndexNamespace(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".owlmail-meta"), 0700); err != nil {
		t.Fatal(err)
	}
	cfg, err := doctorConfig([]string{"-mail-directory", dir, "-mail-index-path", filepath.Join(dir, ".owlmail-meta", "index.db")})
	if err != nil {
		t.Fatal(err)
	}
	report := inspectDoctorConfig(cfg)
	for _, check := range report.Checks {
		if check.Name == "mail_index_namespace" && check.Status == "fail" {
			return
		}
	}
	t.Fatal("managed index was accepted", report)
}

func TestDoctorAcceptsMissingIndexParentAndRejectsDirectoryTarget(t *testing.T) {
	dir := t.TempDir()
	cfg, err := doctorConfig([]string{"-mail-directory", dir, "-mail-index-path", filepath.Join(dir, "derived", "index.db")})
	if err != nil {
		t.Fatal(err)
	}
	if report := inspectDoctorConfig(cfg); !report.Ready {
		t.Fatal(report)
	}
	if _, err := os.Stat(filepath.Join(dir, "derived")); !os.IsNotExist(err) {
		t.Fatal("doctor created parent", err)
	}
	cfg.MailIndexPath = dir
	report := inspectDoctorConfig(cfg)
	for _, check := range report.Checks {
		if check.Name == "mail_index_target" && check.Status == "fail" {
			return
		}
	}
	t.Fatal("accepted directory index", report)
}
