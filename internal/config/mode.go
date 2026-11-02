package config

import (
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/soulteary/owlmail/internal/identity"
)

// ValidateMode checks declarative configuration. Empty Mode preserves legacy
// callers that construct Config literals instead of DefaultConfig.
func ValidateMode(cfg *Config) error {
	if cfg == nil {
		return errors.New("config cannot be nil")
	}
	switch cfg.Mode {
	case "", "standalone":
		return nil
	case "multiuser":
	default:
		return errors.New("mode must be standalone or multiuser")
	}
	if cfg.MultiuserMaxPending <= 0 || cfg.MultiuserRCPTPerMinute <= 0 || cfg.MultiuserAuthPerMinute <= 0 || cfg.MultiuserMaxMessages <= 0 || cfg.MultiuserMaxDiskMB <= 0 || cfg.PendingRetentionDays <= 0 || cfg.PendingEmptyRetentionHours <= 0 {
		return errors.New("multiuser quotas and retention limits must be positive")
	}
	const maxDurationHours = int64(^uint64(0)>>1) / int64(time.Hour)
	if int64(cfg.PendingRetentionDays) > maxDurationHours/24 || int64(cfg.PendingEmptyRetentionHours) > maxDurationHours || int64(cfg.MultiuserMaxDiskMB) > int64(^uint64(0)>>1)>>20 {
		return errors.New("multiuser policy exceeds supported range")
	}
	if interval, err := time.ParseDuration(cfg.PendingCleanupInterval); err != nil || interval <= 0 {
		return errors.New("pending-cleanup-interval must be a positive duration")
	}
	if cfg.MailRetentionDays != 0 {
		return errors.New("multiuser mode uses pending-retention-days; automatic personal mail retention is unsupported")
	}
	if cfg.MailMaxMessages < 0 || cfg.MailMaxDiskMB < 0 || int64(cfg.MailMaxDiskMB) > int64(^uint64(0)>>1)>>20 {
		return errors.New("invalid multiuser global hard quota")
	}
	if cfg.MCPEnabled || cfg.MailDevRESTCompat || cfg.MailCatcherRESTCompat || cfg.DisableWeb {
		return errors.New("multiuser mode cannot enable MCP, REST compatibility or disable-web")
	}
	if cfg.S3Enabled || cfg.OutgoingHost != "" || cfg.AutoRelay || cfg.AutoRelayAddr != "" || cfg.AutoRelayRules != "" || cfg.WebhookConfig != "" || cfg.WebhookRedisURL != "" {
		return errors.New("multiuser external attachment storage, relay and webhooks are not supported yet")
	}
	if cfg.WebUser != "" || cfg.WebPassword != "" {
		return errors.New("multiuser mode uses trusted proxy identity, not Web Basic Auth")
	}
	if cfg.SMTPUser != "" || cfg.SMTPPassword != "" {
		return errors.New("multiuser mode uses system-smtp-accounts-file, not single-user SMTP credentials")
	}
	if !cfg.TLSEnabled || !cfg.SMTPAuthRequireTLS {
		return errors.New("multiuser SMTP requires TLS and smtp-auth-require-tls")
	}
	if cfg.ClaimPolicy != "verified-email" && cfg.ClaimPolicy != "explicit-history" {
		return errors.New("multiuser mode requires an explicit claim-policy")
	}
	if cfg.ControlDatabase == "" || !filepath.IsAbs(cfg.ControlDatabase) || cfg.MailDir == "" || !filepath.IsAbs(cfg.MailDir) {
		return errors.New("multiuser mode requires absolute control-database and mail-directory paths")
	}
	if cfg.SystemSMTPAccountsFile == "" || cfg.RecipientAllowedDomains == "" {
		return errors.New("multiuser mode requires system SMTP accounts and allowed recipient domains")
	}
	if _, err := identity.NewProxy(cfg.IdentityNamespace, strings.Split(cfg.IdentityTrustedProxies, ","), cfg.IdentitySubjectHeader, cfg.IdentityEmailHeader, cfg.IdentityVerifiedHeader); err != nil {
		return err
	}
	return nil
}

// ValidateModeRuntime is deliberately separate from config diagnostics. Until
// the durable ingress and every data exit are scoped, no listener may expose
// the experimental mode. Remove this gate only with the final release evidence.
func ValidateModeRuntime(cfg *Config) error {
	if err := ValidateMode(cfg); err != nil {
		return err
	}
	if cfg.Mode == "multiuser" {
		return errors.New("multiuser mode is not available yet: verified identity integration and release acceptance remain pending")
	}
	return nil
}
