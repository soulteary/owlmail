package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/soulteary/owlmail/internal/config"
	"github.com/soulteary/owlmail/internal/mailbox"
	"github.com/soulteary/owlmail/internal/mailserver"
)

// setupMultiuserIngress is below the public runtime gate so component tests
// can exercise the complete wiring without enabling an unreleased CLI mode.
// A nonempty mail directory is never paired with an automatically recreated
// authority database: loss of the database must remain a hard failure.
func setupMultiuserIngress(cfg *config.Config) (*mailserver.RoutedIngressOptions, error) {
	if cfg.Mode != "multiuser" {
		return nil, nil
	}
	if err := config.ValidateMode(cfg); err != nil {
		return nil, err
	}
	accounts, err := mailbox.LoadAccounts(cfg.SystemSMTPAccountsFile, cfg.IdentityNamespace, strings.Split(cfg.RecipientAllowedDomains, ","))
	if err != nil {
		return nil, err
	}
	var create bool
	entries, err := os.ReadDir(cfg.MailDir)
	if errors.Is(err, os.ErrNotExist) {
		create = true
	} else if err != nil {
		return nil, err
	} else {
		create = len(entries) == 0
	}
	control, err := mailbox.Open(cfg.ControlDatabase, create)
	if err != nil {
		return nil, fmt.Errorf("open multiuser authority: %w", err)
	}
	totalMessages := cfg.MailMaxMessages
	if totalMessages == 0 {
		totalMessages = 100000
	}
	totalDiskMB := cfg.MailMaxDiskMB
	if totalDiskMB == 0 {
		totalDiskMB = 10240
	}
	return &mailserver.RoutedIngressOptions{Accounts: accounts, Control: control, MaxPending: cfg.MultiuserMaxPending, MaxCreatesPerMinute: cfg.MultiuserRCPTPerMinute, MaxAuthPerMinute: cfg.MultiuserAuthPerMinute, MaxMessages: cfg.MultiuserMaxMessages, MaxDiskBytes: int64(cfg.MultiuserMaxDiskMB) << 20, MaxTotalMessages: totalMessages, MaxTotalDiskBytes: int64(totalDiskMB) << 20}, nil
}

func validateControlLocation(cfg *config.Config) error {
	root, err := filepath.Abs(cfg.MailDir)
	if err != nil {
		return err
	}
	path, err := filepath.Abs(cfg.ControlDatabase)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	if relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return errors.New("control-database must be outside mail-directory")
	}
	return nil
}
