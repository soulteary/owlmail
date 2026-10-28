package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/config"
	"github.com/soulteary/owlmail/internal/mailserver"
)

func TestListenInfoDynamicStartupAndProbe(t *testing.T) {
	loggerEventTestMu.Lock()
	defer loggerEventTestMu.Unlock()
	for _, headless := range []bool{false, true} {
		t.Run(fmt.Sprintf("smtp-only-%v", headless), func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.SMTPPort = 0
			cfg.WebPort = 0
			cfg.SMTPHost = "127.0.0.1"
			cfg.WebHost = "127.0.0.1"
			cfg.DisableWeb = headless
			cfg.MailDir = t.TempDir()
			cfg.ListenInfoFile = filepath.Join(t.TempDir(), "listen.json")
			server, err := createMailServer(cfg)
			if err != nil {
				t.Fatal(err)
			}
			result := make(chan error, 1)
			go func() { result <- startServers(server, cfg) }()
			var info listenInfo
			deadline := time.Now().Add(5 * time.Second)
			for {
				info, err = readListenInfo(cfg.ListenInfoFile)
				if err == nil {
					break
				}
				if time.Now().After(deadline) {
					_ = server.Close()
					t.Fatalf("no listener information: %v", err)
				}
				time.Sleep(10 * time.Millisecond)
			}
			stopped := false
			defer func() {
				_ = server.Close()
				if stopped {
					return
				}
				select {
				case <-result:
				case <-time.After(5 * time.Second):
					t.Error("servers did not stop")
				}
			}()
			if info.SMTP.Port == 0 || (!headless && (info.Web == nil || info.Web.Port == 0)) {
				t.Fatal("zero port published")
			}
			if cfg.SMTPPort != 0 || cfg.WebPort != 0 {
				t.Fatal("requested configuration mutated")
			}
			if err := probeHealth(cfg); err != nil {
				t.Fatalf("dynamic probe: %v", err)
			}
			if err := smtp.SendMail(net.JoinHostPort(info.SMTP.Host, fmt.Sprint(info.SMTP.Port)), nil, "sender@example.test", []string{"to@example.test"}, []byte("From: sender@example.test\r\nTo: to@example.test\r\nSubject: dynamic\r\n\r\nhello\r\n")); err != nil {
				t.Fatal(err)
			}
			if !headless {
				client := http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{Proxy: nil}}
				response, err := client.Get("http://" + net.JoinHostPort(info.Web.Host, fmt.Sprint(info.Web.Port)) + "/api/v1/emails")
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = response.Body.Close() }()
				if response.StatusCode != 200 {
					t.Fatalf("mail list status %d", response.StatusCode)
				}
			}
			if err := server.Close(); err != nil {
				t.Fatal(err)
			}
			select {
			case <-result:
				stopped = true
			case <-time.After(5 * time.Second):
				t.Fatal("server owner did not stop")
			}
			if _, err := os.Stat(cfg.ListenInfoFile); !os.IsNotExist(err) {
				t.Fatalf("record survived owner shutdown: %v", err)
			}
		})
	}
}

func TestListenInfoFailedStartupAndReservation(t *testing.T) {
	loggerEventTestMu.Lock()
	defer loggerEventTestMu.Unlock()
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = occupied.Close() }()
	cfg := config.DefaultConfig()
	cfg.SMTPHost = "127.0.0.1"
	cfg.SMTPPort = occupied.Addr().(*net.TCPAddr).Port
	cfg.MailDir = t.TempDir()
	cfg.ListenInfoFile = filepath.Join(t.TempDir(), "listen.json")
	server, err := createMailServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	if err := startServers(server, cfg); err == nil {
		t.Fatal("occupied SMTP port accepted")
	}
	if _, err := os.Stat(cfg.ListenInfoFile); !os.IsNotExist(err) {
		t.Fatal("failed startup published listener information")
	}
	cleanup, err := prepareListenInfo(cfg.ListenInfoFile)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if _, err := prepareListenInfo(cfg.ListenInfoFile); err == nil {
		t.Fatal("second writer accepted")
	}
}

func TestListenInfoHTTPBindFailureReleasesSMTP(t *testing.T) {
	loggerEventTestMu.Lock()
	defer loggerEventTestMu.Unlock()
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = occupied.Close() }()
	cfg := config.DefaultConfig()
	cfg.SMTPHost = "127.0.0.1"
	cfg.SMTPPort = 0
	cfg.WebHost = "127.0.0.1"
	cfg.WebPort = occupied.Addr().(*net.TCPAddr).Port
	cfg.MailDir = t.TempDir()
	cfg.ListenInfoFile = filepath.Join(t.TempDir(), "listen.json")
	server, err := createMailServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	result := make(chan error, 1)
	go func() { result <- startServers(server, cfg) }()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("HTTP bind failure ignored")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("HTTP bind failure hung")
	}
	if _, ok := server.BoundAddress(); ok {
		t.Fatal("SMTP retained after HTTP bind failure")
	}
	if _, err := os.Stat(cfg.ListenInfoFile); !os.IsNotExist(err) {
		t.Fatal("failed HTTP startup left listener information")
	}
}

func TestListenInfoValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "listen.json")
	cleanup, err := prepareListenInfo(path)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	info := listenInfo{Version: 1, PID: os.Getpid(), Owner: listenInfoOwner, CreatedAt: time.Now(), SMTP: mailserver.ListenerAddress{Host: "127.0.0.1", Port: 12345}, DisableWeb: true}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readListenInfo(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".lock", []byte("different-owner"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readListenInfo(path); err == nil {
		t.Fatal("mismatched owner accepted")
	}
	if _, err := resolveProbeListenerPorts(&config.Config{SMTPPort: 0, WebPort: 0}); err == nil {
		t.Fatal("port zero probed without discovery")
	}
}

func TestMCPDynamicWebBaseURLDiscovery(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.WebPort = 0
	got, err := mcpWebBaseURL(cfg)
	if err != nil || got != "" {
		t.Fatalf("undiscovered browser URL=%q err=%v", got, err)
	}
	cfg.WebExternalURL = "https://mail.example.test"
	got, err = mcpWebBaseURL(cfg)
	if err != nil || got != "https://mail.example.test" {
		t.Fatalf("external URL precedence=%q err=%v", got, err)
	}
	cfg.WebExternalURL = ""
	cfg.ListenInfoFile = filepath.Join(t.TempDir(), "listen.json")
	cleanup, err := prepareListenInfo(cfg.ListenInfoFile)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	info := listenInfo{Version: 1, PID: os.Getpid(), Owner: listenInfoOwner, CreatedAt: time.Now(), SMTPBindHost: cfg.SMTPHost, WebBindHost: cfg.WebHost, SMTP: mailserver.ListenerAddress{Host: "127.0.0.1", Port: cfg.SMTPPort}, Web: &mailserver.ListenerAddress{Host: "127.0.0.1", Port: 12345}}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.ListenInfoFile, data, 0600); err != nil {
		t.Fatal(err)
	}
	got, err = mcpWebBaseURL(cfg)
	if err != nil || got != "http://127.0.0.1:12345" {
		t.Fatalf("discovered URL=%q err=%v", got, err)
	}
	cfg.SMTPPort = 2025
	if _, err := resolveProbeListenerPorts(cfg); err == nil {
		t.Fatal("discovery accepted wrong fixed SMTP port")
	}
}
