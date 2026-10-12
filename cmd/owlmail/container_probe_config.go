package main

import (
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/soulteary/owlmail/internal/config"
	"github.com/soulteary/owlmail/internal/mailserver"
)

// Container mode requires OwlMail as PID 1. A fixed local path lets the probe
// find the startup record even after a config file or environment has changed.
const containerProbeConfigPath = "/tmp/.owlmail-container-probe.json"
const maxContainerProbeConfigBytes = 1 << 20

// Deliberately enumerate only public listener settings: never serialize Config,
// which also contains SMTP, web, S3, Redis and downstream credentials.
type containerProbeConfig struct {
	Version        int    `json:"version"`
	ProcessStart   string `json:"process_start"`
	DisableWeb     bool   `json:"disable_web"`
	SMTPHost       string `json:"smtp_host"`
	SMTPPort       int    `json:"smtp_port"`
	WebHost        string `json:"web_host"`
	WebPort        int    `json:"web_port"`
	BasePathname   string `json:"base_pathname"`
	HTTPSEnabled   bool   `json:"https_enabled"`
	CertificatePEM []byte `json:"certificate_pem,omitempty"`
	MailDir        string `json:"mail_directory"`
}

func (probe containerProbeConfig) config() *config.Config {
	return &config.Config{DisableWeb: probe.DisableWeb, SMTPHost: probe.SMTPHost, SMTPPort: probe.SMTPPort,
		WebHost: probe.WebHost, WebPort: probe.WebPort, BasePathname: probe.BasePathname,
		HTTPSEnabled: probe.HTTPSEnabled, MailDir: probe.MailDir}
}

// Field 22 in /proc/PID/stat is the process start time in clock ticks. The
// command name can contain spaces and parentheses, so split after its final ')'.
func processStartIdentity(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read container process identity: %w", err)
	}
	end := strings.LastIndexByte(string(data), ')')
	if end < 0 {
		return "", fmt.Errorf("invalid container process identity")
	}
	fields := strings.Fields(string(data[end+1:]))
	if len(fields) < 20 {
		return "", fmt.Errorf("invalid container process identity")
	}
	if _, err := strconv.ParseUint(fields[19], 10, 64); err != nil {
		return "", fmt.Errorf("invalid container process identity")
	}
	return fields[19], nil
}

func publishContainerProbeConfig(cfg *config.Config, mailDir, path, processStat string, certificates ...[]byte) error {
	identity, err := processStartIdentity(processStat)
	if err != nil {
		return err
	}
	mailDir, err = filepath.Abs(mailDir)
	if err != nil {
		return fmt.Errorf("resolve container mail directory: %w", err)
	}
	probe := containerProbeConfig{Version: 1, ProcessStart: identity, DisableWeb: cfg.DisableWeb,
		SMTPHost: mailserver.ResolveSMTPHost(cfg.SMTPHost), SMTPPort: cfg.SMTPPort, WebHost: cfg.WebHost,
		WebPort: cfg.WebPort, BasePathname: cfg.BasePathname, HTTPSEnabled: cfg.HTTPSEnabled, MailDir: mailDir}
	if cfg.HTTPSEnabled && !cfg.DisableWeb {
		for _, der := range certificates {
			probe.CertificatePEM = append(probe.CertificatePEM, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})...)
		}
		if len(probe.CertificatePEM) == 0 {
			return fmt.Errorf("listener did not provide its HTTPS certificate")
		}
	}
	data, err := json.Marshal(probe)
	if err != nil {
		return err
	}
	if len(data) > maxContainerProbeConfigBytes {
		return fmt.Errorf("container probe configuration exceeds size limit")
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".owlmail-container-probe-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer func() { _ = os.Remove(temporary) }()
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func readContainerProbeConfig(path, processStat string) (containerProbeConfig, error) {
	type result struct {
		probe containerProbeConfig
		err   error
	}
	finished := make(chan result, 1)
	go func() {
		probe, err := loadContainerProbeConfig(path, processStat)
		finished <- result{probe, err}
	}()
	select {
	case result := <-finished:
		return result.probe, result.err
	case <-time.After(time.Second):
		return containerProbeConfig{}, fmt.Errorf("container probe configuration read timed out")
	}
}

func loadContainerProbeConfig(path, processStat string) (containerProbeConfig, error) {
	var probe containerProbeConfig
	file, err := os.Open(path)
	if err != nil {
		return probe, fmt.Errorf("read startup probe configuration: %w", err)
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return probe, fmt.Errorf("startup probe configuration must be a private regular file")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxContainerProbeConfigBytes+1))
	if err != nil || len(data) > maxContainerProbeConfigBytes {
		return probe, fmt.Errorf("startup probe configuration is unreadable or too large")
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return probe, fmt.Errorf("invalid startup probe configuration")
	}
	identity, err := processStartIdentity(processStat)
	if err != nil {
		return probe, err
	}
	if probe.Version != 1 || probe.ProcessStart != identity || !filepath.IsAbs(probe.MailDir) ||
		(probe.HTTPSEnabled && !probe.DisableWeb && len(probe.CertificatePEM) == 0) {
		return probe, fmt.Errorf("startup probe configuration does not match the running container")
	}
	return probe, nil
}
