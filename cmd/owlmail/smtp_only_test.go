package main

import (
	"fmt"
	"net"
	"net/smtp"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/config"
)

func TestSMTPOnlyConfiguration(t *testing.T) {
	t.Setenv("OWLMAIL_DISABLE_WEB", "true")
	cfg, err := commandConfig("healthcheck", nil)
	if err != nil || !cfg.DisableWeb {
		t.Fatalf("env configuration: %v", err)
	}
	cfg, err = commandConfig("healthcheck", []string{"-disable-web=false"})
	if err != nil || cfg.DisableWeb {
		t.Fatalf("CLI override: %v", err)
	}
	t.Setenv("OWLMAIL_DISABLE_WEB", "")
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("disable-web: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err = commandConfig("healthcheck", []string{"-config", path})
	if err != nil || !cfg.DisableWeb {
		t.Fatalf("file configuration: %v", err)
	}
	cfg.WebPort = 0
	cfg.HTTPSEnabled = true // Inactive web certificate settings must not block SMTP.
	if err := config.ValidateConfig(cfg); err != nil {
		t.Fatal(err)
	}
	for _, feature := range []string{"mcp-enabled", "metrics-enabled", "maildev-rest-compat", "mailcatcher-rest-compat"} {
		cfg, err = commandConfig("healthcheck", []string{"-disable-web", "-" + feature})
		if err != nil {
			t.Fatal(err)
		}
		if err := config.ValidateConfig(cfg); err == nil {
			t.Fatalf("accepted HTTP feature %s", feature)
		}
		if err := initializeApplication(cfg); err == nil {
			t.Fatalf("startup accepted HTTP feature %s", feature)
		}
	}
}

func TestSMTPOnlyServicesAndProbe(t *testing.T) {
	// Occupy the web port: headless initialization must succeed without binding it.
	web, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = web.Close() }()
	cfg := config.DefaultConfig()
	cfg.DisableWeb = true
	cfg.WebHost = "127.0.0.1"
	cfg.WebPort = web.Addr().(*net.TCPAddr).Port
	cfg.SMTPHost = "127.0.0.1"
	smtpPort, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	cfg.SMTPPort = smtpPort.Addr().(*net.TCPAddr).Port
	if err := smtpPort.Close(); err != nil {
		t.Fatal(err)
	}
	cfg.MailDir = t.TempDir()
	server, err := createMailServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	if _, err := startAPIServer(server, cfg); err != nil {
		t.Fatal(err)
	}
	ready := make(chan struct{})
	result := make(chan error, 1)
	go func() { result <- server.ListenWithReady(func() { close(ready) }) }()
	select {
	case <-ready:
	case err := <-result:
		t.Fatal(err)
	case <-time.After(5 * time.Second):
		t.Fatal("SMTP did not start")
	}
	cfg.SMTPPort = server.GetPort()
	if err := probeHealth(cfg); err != nil {
		t.Fatal(err)
	}
	if err := probeHealthEndpoint(cfg, true); err != nil {
		t.Fatalf("SMTP readiness: %v", err)
	}
	address := net.JoinHostPort(cfg.SMTPHost, fmt.Sprint(cfg.SMTPPort))
	if err := smtp.SendMail(address, nil, "sender@example.test", []string{"reader@example.test"}, []byte("Subject: Headless capture\r\n\r\nBody\r\n")); err != nil {
		t.Fatal(err)
	}
	if len(server.GetAllEmail()) != 1 {
		t.Fatal("SMTP capture did not persist a message")
	}
}

func TestSMTPOnlyIgnoresWebAuthAndDoctorListener(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DisableWeb = true
	cfg.WebUser = "unused"
	cfg.WebPort = cfg.SMTPPort
	cfg.HTTPSEnabled = true
	cfg.MailDir = t.TempDir()
	if err := initializeApplication(cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.WebPassword != "" {
		t.Fatal("generated an inactive web password")
	}
	if report := inspectDoctorConfig(cfg); !report.Ready {
		t.Fatalf("inactive web settings failed doctor: %+v", report.Checks)
	}
}

func TestSMTPOnlyRejectsNonSMTPListener(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	done := make(chan struct{})
	go func() {
		defer close(done)
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		defer func() { _ = connection.Close() }()
		_, _ = connection.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
	}()
	cfg := config.DefaultConfig()
	cfg.DisableWeb = true
	cfg.SMTPHost = "127.0.0.1"
	cfg.SMTPPort = listener.Addr().(*net.TCPAddr).Port
	if err := probeHealth(cfg); err == nil {
		t.Fatal("accepted a non-SMTP listener")
	}
	<-done
}

func TestSMTPReadinessStorageAndDownstreamFailures(t *testing.T) {
	cfg := config.DefaultConfig()
	if err := probeSMTPReadiness(cfg); err == nil {
		t.Fatal("probe guessed a server-private temporary directory")
	}
	cfg.MailDir = t.TempDir()
	if err := probeSMTPReadiness(cfg); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(cfg.MailDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("probe left files: %v, %v", entries, err)
	}
	cfg.MailDir = filepath.Join(cfg.MailDir, "missing")
	if err := probeSMTPReadiness(cfg); err == nil {
		t.Fatal("missing storage reported ready")
	}
	if _, err := os.Stat(cfg.MailDir); !os.IsNotExist(err) {
		t.Fatal("probe created storage")
	}
	cfg.MailDir = t.TempDir()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OutgoingHost = "127.0.0.1"
	cfg.OutgoingPort = listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	if err := probeSMTPReadiness(cfg); err == nil {
		t.Fatal("unreachable downstream reported ready")
	}
}
