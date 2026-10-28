package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/soulteary/owlmail/internal/api"
	"github.com/soulteary/owlmail/internal/config"
	"github.com/soulteary/owlmail/internal/mailserver"
)

const maximumListenInfoBytes = 1 << 20

var listenInfoOwner = rand.Text()
var listenInfoMutex sync.Mutex

type closeFunc func()

func (f closeFunc) Close() error { f(); return nil }

// listenInfo is credential-free startup discovery, not a readiness assertion.
// The owner token lets cleanup avoid deleting a replacement process's record.
type listenInfo struct {
	Version       int                         `json:"version"`
	PID           int                         `json:"pid"`
	Owner         string                      `json:"owner"`
	CreatedAt     time.Time                   `json:"created_at"`
	SMTPBindHost  string                      `json:"smtp_bind_host"`
	WebBindHost   string                      `json:"web_bind_host"`
	SMTP          mailserver.ListenerAddress  `json:"smtp"`
	Web           *mailserver.ListenerAddress `json:"web,omitempty"`
	SMTPS         *mailserver.ListenerAddress `json:"smtps,omitempty"`
	DisableWeb    bool                        `json:"disable_web"`
	HTTPS         bool                        `json:"https"`
	BasePathname  string                      `json:"base_pathname"`
	MailDirectory string                      `json:"mail_directory"`
}

// prepareListenInfo reserves one writer per output file before any listener
// starts. A crashed owner's lock requires explicit cleanup rather than risking
// overwriting discovery information belonging to a still-running process.
func prepareListenInfo(path string) (func(), error) {
	listenInfoMutex.Lock()
	defer listenInfoMutex.Unlock()
	if path == "" {
		return func() {}, nil
	}
	lock, err := os.OpenFile(path+".lock", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, fmt.Errorf("reserve listen-info-file (use a unique path; remove a stale .lock only after its owner exits): %w", err)
	}
	if _, err := lock.WriteString(listenInfoOwner); err != nil {
		_ = lock.Close()
		_ = os.Remove(path + ".lock")
		return nil, err
	}
	if err := lock.Close(); err != nil {
		_ = os.Remove(path + ".lock")
		return nil, err
	}
	// We own this name. Remove a previous completed record before binding so a
	// failed start cannot leave an old success-looking record behind.
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(path + ".lock")
		return nil, fmt.Errorf("clear old listener information: %w", err)
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			listenInfoMutex.Lock()
			defer listenInfoMutex.Unlock()
			owner, err := os.ReadFile(path + ".lock")
			if err != nil || string(owner) != listenInfoOwner {
				return
			}
			_ = os.Remove(path)
			_ = os.Remove(path + ".lock")
		})
	}, nil
}

func publishListenInfo(cfg *config.Config, server *mailserver.MailServer, web *api.API) error {
	listenInfoMutex.Lock()
	defer listenInfoMutex.Unlock()
	if cfg.ListenInfoFile == "" {
		return nil
	}
	owner, err := os.ReadFile(cfg.ListenInfoFile + ".lock")
	if err != nil || string(owner) != listenInfoOwner {
		return fmt.Errorf("listen-info-file is not reserved by this process")
	}
	smtpAddress, ok := server.BoundAddress()
	if !ok {
		return fmt.Errorf("SMTP listener has not bound")
	}
	mailDir, err := filepath.Abs(server.GetMailDir())
	if err != nil {
		return err
	}
	info := listenInfo{SMTPBindHost: mailserver.ResolveSMTPHost(cfg.SMTPHost), WebBindHost: cfg.WebHost, Version: 1, PID: os.Getpid(), Owner: listenInfoOwner, CreatedAt: time.Now().UTC(), SMTP: smtpAddress, DisableWeb: cfg.DisableWeb, HTTPS: cfg.HTTPSEnabled, BasePathname: cfg.BasePathname, MailDirectory: mailDir}
	if !cfg.DisableWeb {
		if web == nil {
			return fmt.Errorf("HTTP listener has not bound")
		}
		address, ok := web.BoundAddress()
		if !ok {
			return fmt.Errorf("HTTP listener has not bound")
		}
		info.Web = &address
	}
	if port := server.GetSMTPSPort(); port != 0 {
		info.SMTPS = &mailserver.ListenerAddress{Host: server.GetHost(), Port: port}
	}
	data, err := json.Marshal(info)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(cfg.ListenInfoFile), ".owlmail-listen-info-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer func() { _ = os.Remove(temporary) }()
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporary, cfg.ListenInfoFile); err != nil {
		return fmt.Errorf("publish listener information: %w", err)
	}
	return nil
}

func readListenInfo(path string) (listenInfo, error) {
	var info listenInfo
	file, err := os.Open(path)
	if err != nil {
		return info, fmt.Errorf("read listener information: %w", err)
	}
	defer func() { _ = file.Close() }()
	stat, err := file.Stat()
	if err != nil || !stat.Mode().IsRegular() || (runtime.GOOS != "windows" && stat.Mode().Perm()&0077 != 0) {
		return info, fmt.Errorf("listener information must be a private regular file")
	}
	data, err := io.ReadAll(io.LimitReader(file, maximumListenInfoBytes+1))
	if err != nil || len(data) > maximumListenInfoBytes {
		return info, fmt.Errorf("listener information is unreadable or too large")
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return info, fmt.Errorf("parse listener information: %w", err)
	}
	if info.Version != 1 || info.PID <= 0 || info.Owner == "" || info.CreatedAt.IsZero() || config.ValidatePort(info.SMTP.Port, "SMTP port") != nil || (!info.DisableWeb && (info.Web == nil || config.ValidatePort(info.Web.Port, "Web port") != nil)) {
		return info, fmt.Errorf("invalid listener information")
	}
	owner, err := os.ReadFile(path + ".lock")
	if err != nil || string(owner) != info.Owner {
		return info, fmt.Errorf("listener information has no matching active owner lock")
	}
	if err := validateListenInfoProcess(info.PID); err != nil {
		return info, err
	}
	return info, nil
}

func resolveProbeListenerPorts(cfg *config.Config) (*config.Config, error) {
	if cfg.SMTPPort != 0 && (cfg.DisableWeb || cfg.WebPort != 0) {
		return cfg, nil
	}
	if cfg.ListenInfoFile == "" {
		return nil, fmt.Errorf("OS-selected ports require -listen-info-file for healthcheck discovery")
	}
	info, err := readListenInfo(cfg.ListenInfoFile)
	if err != nil {
		return nil, err
	}
	if info.DisableWeb != cfg.DisableWeb || (!cfg.DisableWeb && info.HTTPS != cfg.HTTPSEnabled) || info.BasePathname != cfg.BasePathname {
		return nil, fmt.Errorf("listener information does not match probe configuration")
	}
	canonicalHost := func(host string) string { return strings.ToLower(strings.Trim(host, "[]")) }
	if canonicalHost(mailserver.ResolveSMTPHost(cfg.SMTPHost)) != canonicalHost(info.SMTPBindHost) || (!cfg.DisableWeb && canonicalHost(cfg.WebHost) != canonicalHost(info.WebBindHost)) {
		return nil, fmt.Errorf("listener information does not match configured bind hosts")
	}
	if (cfg.SMTPPort != 0 && cfg.SMTPPort != info.SMTP.Port) || (!cfg.DisableWeb && cfg.WebPort != 0 && cfg.WebPort != info.Web.Port) {
		return nil, fmt.Errorf("listener information does not match configured ports")
	}
	if cfg.MailDir != "" {
		mailDir, err := filepath.Abs(cfg.MailDir)
		if err != nil || filepath.Clean(mailDir) != filepath.Clean(info.MailDirectory) {
			return nil, fmt.Errorf("listener information does not match mail directory")
		}
	}
	resolved := *cfg
	if cfg.SMTPPort == 0 {
		resolved.SMTPPort = info.SMTP.Port
	}
	if !cfg.DisableWeb && cfg.WebPort == 0 {
		resolved.WebHost, resolved.WebPort = info.Web.Host, info.Web.Port
	}
	if cfg.DisableWeb && resolved.MailDir == "" {
		resolved.MailDir = info.MailDirectory
	}
	return &resolved, nil
}
