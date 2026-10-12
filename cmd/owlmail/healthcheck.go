package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/soulteary/owlmail/internal/config"
	"github.com/soulteary/owlmail/internal/mailserver"
)

// containerServerArgs reads the actual PID 1 command rather than assuming that
// environment defaults describe the listener (CLI flags and config files win).
func containerServerArgs(data []byte) ([]string, error) {
	argv := strings.Split(strings.TrimRight(string(data), "\x00"), "\x00")
	if len(argv) == 0 || filepath.Base(argv[0]) != "owlmail" {
		return nil, fmt.Errorf("container healthcheck requires OwlMail as PID 1")
	}
	args := argv[1:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		return nil, fmt.Errorf("container command does not run the mail server")
	}
	return args, nil
}

func runHealthcheck(args []string) error {
	ready := false
	containerMode := false
options:
	for len(args) > 0 {
		switch args[0] {
		case "--ready", "-ready":
			ready = true
		case "--container":
			containerMode = true
		default:
			break options
		}
		args = args[1:]
	}
	if containerMode && len(args) != 0 {
		return fmt.Errorf("container probe cannot override server configuration")
	}
	if containerMode {
		data, err := os.ReadFile("/proc/1/cmdline")
		if err != nil {
			return fmt.Errorf("read container startup command: %w", err)
		}
		_, err = containerServerArgs(data)
		if err != nil {
			return err
		}
		probe, err := readContainerProbeConfig(containerProbeConfigPath, "/proc/1/stat")
		if err != nil {
			return err
		}
		return probeHealthEndpointWithCertificate(probe.config(), ready, probe.CertificatePEM)
	}
	cfg, err := commandConfig("healthcheck", args)
	if err != nil {
		return fmt.Errorf("resolve healthcheck configuration: %w", err)
	}
	return probeHealthEndpoint(cfg, ready)
}

func probeHealth(cfg *config.Config) error { return probeHealthEndpoint(cfg, false) }

func probeHealthEndpoint(cfg *config.Config, ready bool) error {
	return probeHealthEndpointWithCertificate(cfg, ready, nil)
}

func probeHealthEndpointWithCertificate(cfg *config.Config, ready bool, certificatePEM []byte) error {
	if cfg.DisableWeb {
		if err := probeSMTPHealth(cfg); err != nil {
			return err
		}
		if ready {
			return probeSMTPReadiness(cfg)
		}
		return nil
	}
	base, err := config.NormalizeBasePathname(cfg.BasePathname)
	if err != nil {
		return err
	}
	host := healthcheckHost(cfg.WebHost)
	scheme := "http"
	transport := &http.Transport{Proxy: nil}
	defer transport.CloseIdleConnections()
	if cfg.HTTPSEnabled {
		scheme = "https"
		if certificatePEM == nil {
			certificatePEM, err = os.ReadFile(cfg.HTTPSCertFile)
			if err != nil {
				return fmt.Errorf("read HTTPS certificate: %w", err)
			}
		}
		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM(certificatePEM) {
			return fmt.Errorf("invalid HTTPS certificate")
		}
		// Trust only the configured local server certificate. A DNS SAN supplies
		// the verification name while the connection still targets the bind host.
		var block *pem.Block
		remaining := certificatePEM
		for len(remaining) > 0 {
			block, remaining = pem.Decode(remaining)
			if block == nil || block.Type == "CERTIFICATE" {
				break
			}
		}
		if block == nil || block.Type != "CERTIFICATE" {
			return fmt.Errorf("invalid HTTPS certificate")
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return err
		}
		name := host
		if len(cert.DNSNames) > 0 {
			name = strings.Replace(cert.DNSNames[0], "*.", "healthcheck.", 1)
		} else if len(cert.IPAddresses) > 0 {
			name = cert.IPAddresses[0].String()
		}
		transport.TLSClientConfig = &tls.Config{RootCAs: roots, ServerName: name, MinVersion: tls.VersionTLS12}
	}
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	endpoint, err := healthcheckURL(scheme, host, cfg.WebPort, base)
	if ready {
		endpoint = strings.TrimSuffix(endpoint, "/healthz") + "/readyz"
	}
	if err != nil {
		return err
	}
	response, err := client.Get(endpoint)
	if err != nil {
		return fmt.Errorf("healthcheck request failed: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck returned HTTP %d", response.StatusCode)
	}
	return nil
}

func healthcheckHost(host string) string {
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	}
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	if host == "::" {
		host = "::1"
	}
	return host
}

func healthcheckURL(scheme, host string, port int, base string) (string, error) {
	rawPath := base + "/healthz"
	decodedPath, err := url.PathUnescape(rawPath)
	if err != nil {
		return "", err
	}
	return (&url.URL{Scheme: scheme, Host: net.JoinHostPort(host, fmt.Sprint(port)), Path: decodedPath, RawPath: rawPath}).String(), nil
}

// A successful greeting, EHLO/HELO and QUIT verifies a real SMTP listener
// without authenticating, delivering a message, or mutating the mailbox.
func probeSMTPHealth(cfg *config.Config) error {
	host := healthcheckHost(cfg.SMTPHost)
	connection, err := net.DialTimeout("tcp", net.JoinHostPort(host, fmt.Sprint(cfg.SMTPPort)), 2*time.Second)
	if err != nil {
		return fmt.Errorf("SMTP healthcheck connect: %w", err)
	}
	defer func() { _ = connection.Close() }()
	if err := connection.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		return err
	}
	client, err := smtp.NewClient(connection, host)
	if err != nil {
		return fmt.Errorf("SMTP healthcheck greeting: %w", err)
	}
	if err := client.Hello("localhost"); err != nil {
		return fmt.Errorf("SMTP healthcheck EHLO: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("SMTP healthcheck QUIT: %w", err)
	}
	return nil
}

// SMTP-only readiness reads the running owner's cached dependency state.
// The probe does not open storage owners or contact downstream SMTP itself.
func probeSMTPReadiness(cfg *config.Config) error {
	if cfg.MailDir == "" {
		return fmt.Errorf("SMTP readiness requires -mail-directory or OWLMAIL_MAIL_DIR; the default temporary directory is private to the server process")
	}
	if !filepath.IsAbs(cfg.MailDir) {
		return fmt.Errorf("SMTP readiness requires an absolute mail directory; relative paths depend on the server working directory")
	}
	return probeRuntimeReadiness(cfg)
}

// The buffered result channel allows a blocked filesystem call to finish after
// the caller times out. Healthcheck is a one-shot process, not a recurring owner.
func probeRuntimeReadiness(cfg *config.Config) error {
	result := make(chan error, 1)
	go func() {
		file, err := os.Open(mailserver.RuntimeReadinessPath(cfg.MailDir, mailserver.ResolveSMTPHost(cfg.SMTPHost), cfg.SMTPPort))
		if err != nil {
			result <- fmt.Errorf("runtime readiness is unavailable")
			return
		}
		defer func() { _ = file.Close() }()
		var status mailserver.RuntimeReadiness
		data, err := io.ReadAll(io.LimitReader(file, 64*1024+1))
		if err != nil || len(data) > 64*1024 || json.Unmarshal(data, &status) != nil {
			result <- fmt.Errorf("runtime readiness is invalid")
			return
		}
		result <- status.Validate(mailserver.ResolveSMTPHost(cfg.SMTPHost), cfg.SMTPPort, time.Now())
	}()
	select {
	case err := <-result:
		return err
	case <-time.After(2 * time.Second):
		return fmt.Errorf("runtime readiness read timed out")
	}
}
