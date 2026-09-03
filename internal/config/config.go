// Package config provides configuration parsing with MailDev environment variable compatibility.
// It uses cli-kit for environment variable management and validation.
//
// Priority order: CLI flags > MAILDEV_* env vars > OWLMAIL_* env vars > default values
package config

import (
	"flag"
	"strconv"

	"github.com/soulteary/cli-kit/env"
	"github.com/soulteary/cli-kit/flagutil"
)

// DefaultMailCleanupInterval balances prompt retention with filesystem load.
const DefaultMailCleanupInterval = "1h"

// DefaultSMTPMaxMessageMB raises the historical 1 MiB limit while retaining a
// bounded default suitable for development and test environments.
const DefaultSMTPMaxMessageMB = 100

// DefaultSMTPMaxConcurrency bounds concurrent high-resource SMTP DATA
// transactions within one OwlMail process. Zero remains an explicit unlimited
// mode for deployments that need the historical behavior.
const DefaultSMTPMaxConcurrency = 8

// SMTP connection limits apply consistently to SMTP, STARTTLS, and SMTPS.
const DefaultSMTPReadTimeout = "10s"
const DefaultSMTPWriteTimeout = "10s"
const DefaultSMTPMaxRecipients = 50

const invalidSMTPTimeout = "invalid"

const DefaultS3Region = "us-east-1"
const DefaultS3Prefix = "owlmail/attachments"
const DefaultS3HealthCheckInterval = "30s"
const DefaultS3HealthCheckTimeout = "5s"

// DefaultWebhookMaxConcurrency is the recommended concurrent email delivery
// limit. Zero remains available as an explicit unlimited mode.
const DefaultWebhookMaxConcurrency = 8
const DefaultWebhookRedisPrefix = "owlmail:webhooks"
const DefaultWebhookShutdownTimeout = "15s"

// MCP is opt-in. Idle sessions and process shutdown are bounded so abandoned
// test-agent connections do not live forever.
const DefaultMCPSessionTimeout = "30m"
const DefaultMCPShutdownTimeout = "5s"

const DefaultOutgoingConnectTimeout = "10s"
const DefaultOutgoingTLSHandshakeTimeout = "10s"
const DefaultOutgoingAuthTimeout = "10s"
const DefaultOutgoingEnvelopeTimeout = "10s"
const DefaultOutgoingDataTimeout = "30s"
const DefaultOutgoingQuitTimeout = "5s"

// EnvMapping defines the mapping from MailDev environment variables to OwlMail environment variables.
// This maintains backward compatibility with MailDev deployments.
var EnvMapping = map[string]string{
	// SMTP server configuration
	"MAILDEV_SMTP_PORT":      "OWLMAIL_SMTP_PORT",
	"MAILDEV_IP":             "OWLMAIL_SMTP_HOST",
	"MAILDEV_MAIL_DIRECTORY": "OWLMAIL_MAIL_DIR",

	// Web API configuration
	"MAILDEV_WEB_PORT":      "OWLMAIL_WEB_PORT",
	"MAILDEV_WEB_IP":        "OWLMAIL_WEB_HOST",
	"MAILDEV_WEB_USER":      "OWLMAIL_WEB_USER",
	"MAILDEV_WEB_PASS":      "OWLMAIL_WEB_PASSWORD",
	"MAILDEV_BASE_PATHNAME": "OWLMAIL_BASE_PATHNAME",

	// HTTPS configuration
	"MAILDEV_HTTPS":      "OWLMAIL_HTTPS_ENABLED",
	"MAILDEV_HTTPS_CERT": "OWLMAIL_HTTPS_CERT",
	"MAILDEV_HTTPS_KEY":  "OWLMAIL_HTTPS_KEY",

	// Outgoing mail configuration
	"MAILDEV_OUTGOING_HOST":   "OWLMAIL_OUTGOING_HOST",
	"MAILDEV_OUTGOING_PORT":   "OWLMAIL_OUTGOING_PORT",
	"MAILDEV_OUTGOING_USER":   "OWLMAIL_OUTGOING_USER",
	"MAILDEV_OUTGOING_PASS":   "OWLMAIL_OUTGOING_PASSWORD",
	"MAILDEV_OUTGOING_SECURE": "OWLMAIL_OUTGOING_SECURE",

	// Auto relay configuration
	"MAILDEV_AUTO_RELAY":       "OWLMAIL_AUTO_RELAY",
	"MAILDEV_AUTO_RELAY_ADDR":  "OWLMAIL_AUTO_RELAY_ADDR",
	"MAILDEV_AUTO_RELAY_RULES": "OWLMAIL_AUTO_RELAY_RULES",

	// SMTP authentication configuration
	"MAILDEV_INCOMING_USER": "OWLMAIL_SMTP_USER",
	"MAILDEV_INCOMING_PASS": "OWLMAIL_SMTP_PASSWORD",

	// TLS configuration
	"MAILDEV_INCOMING_SECURE": "OWLMAIL_TLS_ENABLED",
	"MAILDEV_INCOMING_CERT":   "OWLMAIL_TLS_CERT",
	"MAILDEV_INCOMING_KEY":    "OWLMAIL_TLS_KEY",
}

// reverseEnvMapping creates a reverse mapping from OWLMAIL_* to MAILDEV_*
var reverseEnvMapping map[string]string

func init() {
	reverseEnvMapping = make(map[string]string, len(EnvMapping))
	for maildevKey, owlmailKey := range EnvMapping {
		reverseEnvMapping[owlmailKey] = maildevKey
	}
}

// GetMailDevKey returns the corresponding MailDev environment variable key for an OwlMail key.
// Returns empty string if no mapping exists.
func GetMailDevKey(owlmailKey string) string {
	return reverseEnvMapping[owlmailKey]
}

// ResolveString resolves a string configuration value with MailDev compatibility.
// Priority: CLI flag > MAILDEV_* env > OWLMAIL_* env > default value
func ResolveString(fs *flag.FlagSet, flagName, owlmailKey, defaultValue string) string {
	// Priority 1: CLI flag (highest priority)
	if fs != nil && flagutil.HasFlag(fs, flagName) {
		return flagutil.GetString(fs, flagName, defaultValue)
	}

	// Priority 2: MAILDEV_* environment variable
	maildevKey := GetMailDevKey(owlmailKey)
	if maildevKey != "" && env.Has(maildevKey) {
		if value := env.Get(maildevKey, ""); value != "" {
			return value
		}
	}

	// Priority 3: OWLMAIL_* environment variable
	if env.Has(owlmailKey) {
		if value := env.Get(owlmailKey, ""); value != "" {
			return value
		}
	}

	// Priority 4: Default value
	return defaultValue
}

// ResolveInt resolves an integer configuration value with MailDev compatibility.
// Priority: CLI flag > MAILDEV_* env > OWLMAIL_* env > default value
func ResolveInt(fs *flag.FlagSet, flagName, owlmailKey string, defaultValue int) int {
	// Priority 1: CLI flag (highest priority)
	if fs != nil && flagutil.HasFlag(fs, flagName) {
		return flagutil.GetInt(fs, flagName, defaultValue)
	}

	// Priority 2: MAILDEV_* environment variable
	maildevKey := GetMailDevKey(owlmailKey)
	if maildevKey != "" && env.Has(maildevKey) {
		if value := env.GetInt(maildevKey, 0); value != 0 {
			return value
		}
		// Handle explicit zero value by checking if string is "0"
		if env.Get(maildevKey, "") == "0" {
			return 0
		}
	}

	// Priority 3: OWLMAIL_* environment variable
	if env.Has(owlmailKey) {
		if value := env.GetInt(owlmailKey, 0); value != 0 {
			return value
		}
		// Handle explicit zero value by checking if string is "0"
		if env.Get(owlmailKey, "") == "0" {
			return 0
		}
	}

	// Priority 4: Default value
	return defaultValue
}

// ResolveBool resolves a boolean configuration value with MailDev compatibility.
// Priority: CLI flag > MAILDEV_* env > OWLMAIL_* env > default value
func ResolveBool(fs *flag.FlagSet, flagName, owlmailKey string, defaultValue bool) bool {
	// Priority 1: CLI flag (highest priority)
	if fs != nil && flagutil.HasFlag(fs, flagName) {
		return flagutil.GetBool(fs, flagName, defaultValue)
	}

	// Priority 2: MAILDEV_* environment variable
	maildevKey := GetMailDevKey(owlmailKey)
	if maildevKey != "" && env.Has(maildevKey) {
		return env.GetBool(maildevKey, defaultValue)
	}

	// Priority 3: OWLMAIL_* environment variable
	if env.Has(owlmailKey) {
		return env.GetBool(owlmailKey, defaultValue)
	}

	// Priority 4: Default value
	return defaultValue
}

// resolveOutgoingTransport resolves the modern TLS mode and the legacy secure
// alias as one setting, so a higher-priority source cannot be shadowed by the
// other selector. At the same priority, the explicit TLS mode is preferred.
func resolveOutgoingTransport(fs *flag.FlagSet, defaultSecure bool, defaultTLSMode string) (bool, string) {
	if fs != nil && flagutil.HasFlag(fs, "outgoing-tls-mode") {
		return false, flagutil.GetString(fs, "outgoing-tls-mode", defaultTLSMode)
	}
	if fs != nil && flagutil.HasFlag(fs, "outgoing-secure") {
		return flagutil.GetBool(fs, "outgoing-secure", defaultSecure), ""
	}

	if env.Has("MAILDEV_OUTGOING_SECURE") {
		return env.GetBool("MAILDEV_OUTGOING_SECURE", defaultSecure), ""
	}
	if value := env.Get("OWLMAIL_OUTGOING_TLS_MODE", ""); value != "" {
		return false, value
	}
	if env.Has("OWLMAIL_OUTGOING_SECURE") {
		return env.GetBool("OWLMAIL_OUTGOING_SECURE", defaultSecure), ""
	}
	return defaultSecure, defaultTLSMode
}

// ResolveLogLevel resolves the log level with MailDev compatibility.
// MailDev uses MAILDEV_VERBOSE and MAILDEV_SILENT environment variables.
// OwlMail uses OWLMAIL_LOG_LEVEL with values: silent, normal, verbose
// Priority: CLI flag > MAILDEV_VERBOSE/SILENT > OWLMAIL_LOG_LEVEL > default
func ResolveLogLevel(fs *flag.FlagSet, flagName, defaultValue string) string {
	// Priority 1: CLI flag (highest priority)
	if fs != nil && flagutil.HasFlag(fs, flagName) {
		return flagutil.GetString(fs, flagName, defaultValue)
	}

	// Priority 2: MAILDEV_VERBOSE/SILENT environment variables
	if env.Has("MAILDEV_VERBOSE") && env.Get("MAILDEV_VERBOSE", "") != "" {
		return "verbose"
	}
	if env.Has("MAILDEV_SILENT") && env.Get("MAILDEV_SILENT", "") != "" {
		return "silent"
	}

	// Priority 3: OWLMAIL_LOG_LEVEL environment variable
	if env.Has("OWLMAIL_LOG_LEVEL") {
		if value := env.Get("OWLMAIL_LOG_LEVEL", ""); value != "" {
			return value
		}
	}

	// Priority 4: Default value
	return defaultValue
}

// Config holds all application configuration
type Config struct {
	// SMTP server configuration
	SMTPPort            int
	SMTPHost            string
	SMTPMaxMessageMB    int
	SMTPMaxConcurrency  int
	SMTPReadTimeout     string
	SMTPWriteTimeout    string
	SMTPMaxRecipients   int
	MailDir             string
	MailRetentionDays   int
	MailMaxMessages     int
	MailMaxDiskMB       int
	MailCleanupInterval string
	MailIndexPath       string

	// Web API configuration
	WebPort            int
	WebHost            string
	WebUser            string
	WebPassword        string
	WebExternalScheme  string
	WebExternalURL     string
	BasePathname       string
	MailDevRESTCompat  bool
	MetricsEnabled     bool
	MCPEnabled         bool
	MCPSessionTimeout  string
	MCPShutdownTimeout string

	// HTTPS configuration
	HTTPSEnabled  bool
	HTTPSCertFile string
	HTTPSKeyFile  string

	// Outgoing mail configuration
	OutgoingHost                string
	OutgoingPort                int
	OutgoingUser                string
	OutgoingPass                string
	OutgoingSecure              bool
	OutgoingTLSMode             string
	OutgoingInsecureSkipVerify  bool
	OutgoingConnectTimeout      string
	OutgoingTLSHandshakeTimeout string
	OutgoingAuthTimeout         string
	OutgoingEnvelopeTimeout     string
	OutgoingDataTimeout         string
	OutgoingQuitTimeout         string
	AutoRelay                   bool
	AutoRelayAddr               string
	AutoRelayRules              string

	// SMTP authentication
	SMTPUser           string
	SMTPPassword       string
	SMTPAuthRequireTLS bool

	// TLS configuration for SMTP
	TLSEnabled  bool
	TLSCertFile string
	TLSKeyFile  string

	// Logging configuration
	LogLevel  string
	LogFormat string

	// Email ID configuration
	UseUUIDForEmailID bool

	// Optional S3-compatible attachment storage. Raw messages and metadata
	// always remain in MailDir.
	S3Enabled         bool
	S3Endpoint        string
	S3Region          string
	S3Bucket          string
	S3Prefix          string
	S3AccessKeyID     string
	S3SecretAccessKey string
	S3SessionToken    string
	S3UsePathStyle    bool
	S3StartupCheck    bool
	S3HealthInterval  string
	S3HealthTimeout   string

	// Webhook forwarding configuration
	WebhookConfig          string
	WebhookMaxConcurrency  int
	WebhookRedisURL        string
	WebhookRedisPrefix     string
	WebhookShutdownTimeout string
}

// DefaultConfig returns a Config with default values
func DefaultConfig() *Config {
	return &Config{
		SMTPPort:                    1025,
		SMTPHost:                    "localhost",
		SMTPMaxMessageMB:            DefaultSMTPMaxMessageMB,
		SMTPMaxConcurrency:          DefaultSMTPMaxConcurrency,
		SMTPReadTimeout:             DefaultSMTPReadTimeout,
		SMTPWriteTimeout:            DefaultSMTPWriteTimeout,
		SMTPMaxRecipients:           DefaultSMTPMaxRecipients,
		MailDir:                     "",
		MailRetentionDays:           0,
		MailMaxMessages:             0,
		MailMaxDiskMB:               0,
		MailCleanupInterval:         DefaultMailCleanupInterval,
		MailIndexPath:               "",
		WebPort:                     1080,
		WebHost:                     "localhost",
		WebUser:                     "",
		WebPassword:                 "",
		WebExternalScheme:           "",
		WebExternalURL:              "",
		BasePathname:                "",
		MailDevRESTCompat:           false,
		MetricsEnabled:              false,
		MCPEnabled:                  false,
		MCPSessionTimeout:           DefaultMCPSessionTimeout,
		MCPShutdownTimeout:          DefaultMCPShutdownTimeout,
		HTTPSEnabled:                false,
		HTTPSCertFile:               "",
		HTTPSKeyFile:                "",
		OutgoingHost:                "",
		OutgoingPort:                587,
		OutgoingUser:                "",
		OutgoingPass:                "",
		OutgoingSecure:              false,
		OutgoingTLSMode:             "",
		OutgoingInsecureSkipVerify:  false,
		OutgoingConnectTimeout:      DefaultOutgoingConnectTimeout,
		OutgoingTLSHandshakeTimeout: DefaultOutgoingTLSHandshakeTimeout,
		OutgoingAuthTimeout:         DefaultOutgoingAuthTimeout,
		OutgoingEnvelopeTimeout:     DefaultOutgoingEnvelopeTimeout,
		OutgoingDataTimeout:         DefaultOutgoingDataTimeout,
		OutgoingQuitTimeout:         DefaultOutgoingQuitTimeout,
		AutoRelay:                   false,
		AutoRelayAddr:               "",
		AutoRelayRules:              "",
		SMTPUser:                    "",
		SMTPPassword:                "",
		SMTPAuthRequireTLS:          false,
		TLSEnabled:                  false,
		TLSCertFile:                 "",
		TLSKeyFile:                  "",
		LogLevel:                    "normal",
		LogFormat:                   "console",
		UseUUIDForEmailID:           false,
		S3Enabled:                   false,
		S3Endpoint:                  "",
		S3Region:                    DefaultS3Region,
		S3Bucket:                    "",
		S3Prefix:                    DefaultS3Prefix,
		S3AccessKeyID:               "",
		S3SecretAccessKey:           "",
		S3SessionToken:              "",
		S3UsePathStyle:              false,
		S3StartupCheck:              false,
		S3HealthInterval:            DefaultS3HealthCheckInterval,
		S3HealthTimeout:             DefaultS3HealthCheckTimeout,
		WebhookConfig:               "",
		WebhookMaxConcurrency:       DefaultWebhookMaxConcurrency,
		WebhookRedisURL:             "",
		WebhookRedisPrefix:          DefaultWebhookRedisPrefix,
		WebhookShutdownTimeout:      DefaultWebhookShutdownTimeout,
	}
}

// FlagRefs holds references to all flag values for resolution after parsing.
type FlagRefs struct {
	SMTPPort                    *int
	SMTPHost                    *string
	SMTPMaxMessageMB            *int
	SMTPMaxConcurrency          *int
	SMTPReadTimeout             *string
	SMTPWriteTimeout            *string
	SMTPMaxRecipients           *int
	MailDir                     *string
	MailRetentionDays           *int
	MailMaxMessages             *int
	MailMaxDiskMB               *int
	MailCleanupInterval         *string
	MailIndexPath               *string
	WebPort                     *int
	WebHost                     *string
	WebUser                     *string
	WebPassword                 *string
	WebExternalURL              *string
	BasePathname                *string
	MailDevRESTCompat           *bool
	MetricsEnabled              *bool
	MCPEnabled                  *bool
	MCPSessionTimeout           *string
	MCPShutdownTimeout          *string
	HTTPSEnabled                *bool
	HTTPSCertFile               *string
	HTTPSKeyFile                *string
	OutgoingHost                *string
	OutgoingPort                *int
	OutgoingUser                *string
	OutgoingPass                *string
	OutgoingSecure              *bool
	OutgoingTLSMode             *string
	OutgoingInsecureSkipVerify  *bool
	OutgoingConnectTimeout      *string
	OutgoingTLSHandshakeTimeout *string
	OutgoingAuthTimeout         *string
	OutgoingEnvelopeTimeout     *string
	OutgoingDataTimeout         *string
	OutgoingQuitTimeout         *string
	AutoRelay                   *bool
	AutoRelayAddr               *string
	AutoRelayRules              *string
	SMTPUser                    *string
	SMTPPassword                *string
	SMTPAuthRequireTLS          *bool
	TLSEnabled                  *bool
	TLSCertFile                 *string
	TLSKeyFile                  *string
	LogLevel                    *string
	LogFormat                   *string
	UseUUIDForEmailID           *bool
	S3Enabled                   *bool
	S3Endpoint                  *string
	S3Region                    *string
	S3Bucket                    *string
	S3Prefix                    *string
	S3AccessKeyID               *string
	S3SecretAccessKey           *string
	S3SessionToken              *string
	S3UsePathStyle              *bool
	S3StartupCheck              *bool
	S3HealthInterval            *string
	S3HealthTimeout             *string
	WebhookConfig               *string
	WebhookMaxConcurrency       *int
	WebhookRedisURL             *string
	WebhookRedisPrefix          *string
	WebhookShutdownTimeout      *string
}

// DefineFlags defines all configuration flags on the given FlagSet.
// It returns FlagRefs which should be passed to ResolveConfig after parsing.
func DefineFlags(fs *flag.FlagSet) *FlagRefs {
	cfg := DefaultConfig()
	return &FlagRefs{
		SMTPPort:                    fs.Int("smtp", cfg.SMTPPort, "SMTP port to catch emails"),
		SMTPHost:                    fs.String("ip", cfg.SMTPHost, "IP address to bind SMTP service to"),
		SMTPMaxMessageMB:            fs.Int("smtp-max-message-mb", cfg.SMTPMaxMessageMB, "Maximum inbound message size in MiB"),
		SMTPMaxConcurrency:          fs.Int("smtp-max-concurrency", cfg.SMTPMaxConcurrency, "Maximum concurrent SMTP DATA transactions per process (0 = unlimited)"),
		SMTPReadTimeout:             fs.String("smtp-read-timeout", cfg.SMTPReadTimeout, "SMTP command and DATA read timeout"),
		SMTPWriteTimeout:            fs.String("smtp-write-timeout", cfg.SMTPWriteTimeout, "SMTP response write timeout"),
		SMTPMaxRecipients:           fs.Int("smtp-max-recipients", cfg.SMTPMaxRecipients, "Maximum recipients accepted per message"),
		MailDir:                     fs.String("mail-directory", cfg.MailDir, "Directory for persisting mails"),
		MailRetentionDays:           fs.Int("mail-retention-days", cfg.MailRetentionDays, "Delete mail older than this many days (0 = unlimited)"),
		MailMaxMessages:             fs.Int("mail-max-messages", cfg.MailMaxMessages, "Maximum stored message count (0 = unlimited)"),
		MailMaxDiskMB:               fs.Int("mail-max-disk-mb", cfg.MailMaxDiskMB, "Maximum mailbox disk usage in MiB (0 = unlimited)"),
		MailCleanupInterval:         fs.String("mail-cleanup-interval", cfg.MailCleanupInterval, "Storage cleanup interval"),
		MailIndexPath:               fs.String("mail-index-path", cfg.MailIndexPath, "Optional SQLite mailbox index path"),
		WebPort:                     fs.Int("web", cfg.WebPort, "Web API port"),
		WebHost:                     fs.String("web-ip", cfg.WebHost, "IP address to bind Web API to"),
		WebUser:                     fs.String("web-user", cfg.WebUser, "HTTP Basic Auth username"),
		WebPassword:                 fs.String("web-password", cfg.WebPassword, "HTTP Basic Auth password"),
		WebExternalURL:              fs.String("web-external-url", cfg.WebExternalURL, "Browser-visible Web origin used in generated links"),
		BasePathname:                fs.String("base-pathname", cfg.BasePathname, "Browser-visible URL path prefix (for example /owlmail)"),
		MailDevRESTCompat:           fs.Bool("maildev-rest-compat", cfg.MailDevRESTCompat, "Enable the optional MailDev REST compatibility facade under /api"),
		MetricsEnabled:              fs.Bool("metrics-enabled", cfg.MetricsEnabled, "Expose Prometheus metrics at /metrics"),
		MCPEnabled:                  fs.Bool("mcp-enabled", cfg.MCPEnabled, "Enable the read-only MCP Streamable HTTP endpoint"),
		MCPSessionTimeout:           fs.String("mcp-session-timeout", cfg.MCPSessionTimeout, "Idle timeout for MCP sessions"),
		MCPShutdownTimeout:          fs.String("mcp-shutdown-timeout", cfg.MCPShutdownTimeout, "Maximum time to close MCP sessions during shutdown"),
		HTTPSEnabled:                fs.Bool("https", cfg.HTTPSEnabled, "Enable HTTPS for Web API"),
		HTTPSCertFile:               fs.String("https-cert", cfg.HTTPSCertFile, "HTTPS certificate file path"),
		HTTPSKeyFile:                fs.String("https-key", cfg.HTTPSKeyFile, "HTTPS private key file path"),
		OutgoingHost:                fs.String("outgoing-host", cfg.OutgoingHost, "Outgoing SMTP server host"),
		OutgoingPort:                fs.Int("outgoing-port", cfg.OutgoingPort, "Outgoing SMTP server port"),
		OutgoingUser:                fs.String("outgoing-user", cfg.OutgoingUser, "Outgoing SMTP server username"),
		OutgoingPass:                fs.String("outgoing-pass", cfg.OutgoingPass, "Outgoing SMTP server password"),
		OutgoingSecure:              fs.Bool("outgoing-secure", cfg.OutgoingSecure, "Use implicit TLS/SMTPS for outgoing SMTP (MailDev compatibility)"),
		OutgoingTLSMode:             fs.String("outgoing-tls-mode", cfg.OutgoingTLSMode, "Outgoing SMTP transport: plain, starttls, or smtps"),
		OutgoingInsecureSkipVerify:  fs.Bool("outgoing-insecure-skip-verify", cfg.OutgoingInsecureSkipVerify, "Skip outgoing SMTP certificate verification (unsafe)"),
		OutgoingConnectTimeout:      fs.String("outgoing-connect-timeout", cfg.OutgoingConnectTimeout, "Outgoing SMTP connect and greeting timeout"),
		OutgoingTLSHandshakeTimeout: fs.String("outgoing-tls-handshake-timeout", cfg.OutgoingTLSHandshakeTimeout, "Outgoing SMTP TLS handshake timeout"),
		OutgoingAuthTimeout:         fs.String("outgoing-auth-timeout", cfg.OutgoingAuthTimeout, "Outgoing SMTP AUTH timeout"),
		OutgoingEnvelopeTimeout:     fs.String("outgoing-envelope-timeout", cfg.OutgoingEnvelopeTimeout, "Outgoing SMTP MAIL/RCPT command timeout"),
		OutgoingDataTimeout:         fs.String("outgoing-data-timeout", cfg.OutgoingDataTimeout, "Outgoing SMTP DATA timeout"),
		OutgoingQuitTimeout:         fs.String("outgoing-quit-timeout", cfg.OutgoingQuitTimeout, "Outgoing SMTP QUIT timeout"),
		AutoRelay:                   fs.Bool("auto-relay", cfg.AutoRelay, "Automatically relay all emails"),
		AutoRelayAddr:               fs.String("auto-relay-addr", cfg.AutoRelayAddr, "Auto relay to specific address"),
		AutoRelayRules:              fs.String("auto-relay-rules", cfg.AutoRelayRules, "JSON file path for auto relay rules"),
		SMTPUser:                    fs.String("smtp-user", cfg.SMTPUser, "SMTP username; set with smtp-password to require authentication"),
		SMTPPassword:                fs.String("smtp-password", cfg.SMTPPassword, "SMTP password; set with smtp-user to require authentication"),
		SMTPAuthRequireTLS:          fs.Bool("smtp-auth-require-tls", cfg.SMTPAuthRequireTLS, "Require TLS before accepting SMTP AUTH"),
		TLSEnabled:                  fs.Bool("tls", cfg.TLSEnabled, "Enable TLS/STARTTLS for SMTP server"),
		TLSCertFile:                 fs.String("tls-cert", cfg.TLSCertFile, "TLS certificate file path"),
		TLSKeyFile:                  fs.String("tls-key", cfg.TLSKeyFile, "TLS private key file path"),
		LogLevel:                    fs.String("log-level", cfg.LogLevel, "Log level: silent, normal, or verbose"),
		LogFormat:                   fs.String("log-format", cfg.LogFormat, "Log format: console or json"),
		UseUUIDForEmailID:           fs.Bool("use-uuid-for-email-id", cfg.UseUUIDForEmailID, "Use UUID instead of random string for email IDs"),
		S3Enabled:                   fs.Bool("s3-enabled", cfg.S3Enabled, "Store decoded attachments in S3-compatible object storage"),
		S3Endpoint:                  fs.String("s3-endpoint", cfg.S3Endpoint, "S3-compatible endpoint URL (empty uses AWS S3)"),
		S3Region:                    fs.String("s3-region", cfg.S3Region, "S3 region"),
		S3Bucket:                    fs.String("s3-bucket", cfg.S3Bucket, "S3 bucket for attachments"),
		S3Prefix:                    fs.String("s3-prefix", cfg.S3Prefix, "S3 object key prefix for attachments"),
		S3AccessKeyID:               fs.String("s3-access-key", cfg.S3AccessKeyID, "S3 static access key (optional)"),
		S3SecretAccessKey:           fs.String("s3-secret-key", cfg.S3SecretAccessKey, "S3 static secret key (optional)"),
		S3SessionToken:              fs.String("s3-session-token", cfg.S3SessionToken, "S3 static credential session token (optional)"),
		S3UsePathStyle:              fs.Bool("s3-use-path-style", cfg.S3UsePathStyle, "Use path-style S3 bucket addressing"),
		S3StartupCheck:              fs.Bool("s3-startup-check", cfg.S3StartupCheck, "Fail startup when the initial S3 bucket check fails"),
		S3HealthInterval:            fs.String("s3-health-check-interval", cfg.S3HealthInterval, "Background S3 health-check interval"),
		S3HealthTimeout:             fs.String("s3-health-check-timeout", cfg.S3HealthTimeout, "Timeout for each S3 health check"),
		WebhookConfig:               fs.String("webhook-config", cfg.WebhookConfig, "JSON file path for webhook forwarding targets"),
		WebhookMaxConcurrency:       fs.Int("webhook-max-concurrency", cfg.WebhookMaxConcurrency, "Maximum concurrent webhook deliveries (0 = unlimited)"),
		WebhookRedisURL:             fs.String("webhook-redis-url", cfg.WebhookRedisURL, "Redis URL for durable webhook delivery"),
		WebhookRedisPrefix:          fs.String("webhook-redis-prefix", cfg.WebhookRedisPrefix, "Redis key prefix for webhook delivery"),
		WebhookShutdownTimeout:      fs.String("webhook-shutdown-timeout", cfg.WebhookShutdownTimeout, "Maximum time to drain webhook delivery during shutdown"),
	}
}

// ResolveConfig resolves configuration from flag values and environment variables.
// This should be called after DefineFlags and fs.Parse().
// Priority: CLI flags > MAILDEV_* env > OWLMAIL_* env > default values
func ResolveConfig(fs *flag.FlagSet, refs *FlagRefs) *Config {
	outgoingSecure, outgoingTLSMode := resolveOutgoingTransport(fs, *refs.OutgoingSecure, *refs.OutgoingTLSMode)
	return &Config{
		SMTPPort:            resolveIntWithFlag(fs, "smtp", "OWLMAIL_SMTP_PORT", *refs.SMTPPort),
		SMTPHost:            resolveStringWithFlag(fs, "ip", "OWLMAIL_SMTP_HOST", *refs.SMTPHost),
		SMTPMaxMessageMB:    resolveIntWithFlag(fs, "smtp-max-message-mb", "OWLMAIL_SMTP_MAX_MESSAGE_MB", *refs.SMTPMaxMessageMB),
		SMTPMaxConcurrency:  resolveSMTPMaxConcurrencyWithFlag(fs, *refs.SMTPMaxConcurrency),
		SMTPReadTimeout:     resolveSMTPTimeoutWithFlag(fs, "smtp-read-timeout", "OWLMAIL_SMTP_READ_TIMEOUT", *refs.SMTPReadTimeout),
		SMTPWriteTimeout:    resolveSMTPTimeoutWithFlag(fs, "smtp-write-timeout", "OWLMAIL_SMTP_WRITE_TIMEOUT", *refs.SMTPWriteTimeout),
		SMTPMaxRecipients:   resolveSMTPMaxRecipientsWithFlag(fs, *refs.SMTPMaxRecipients),
		MailDir:             resolveStringWithFlag(fs, "mail-directory", "OWLMAIL_MAIL_DIR", *refs.MailDir),
		MailRetentionDays:   resolveIntWithFlag(fs, "mail-retention-days", "OWLMAIL_MAIL_RETENTION_DAYS", *refs.MailRetentionDays),
		MailMaxMessages:     resolveIntWithFlag(fs, "mail-max-messages", "OWLMAIL_MAIL_MAX_MESSAGES", *refs.MailMaxMessages),
		MailMaxDiskMB:       resolveIntWithFlag(fs, "mail-max-disk-mb", "OWLMAIL_MAIL_MAX_DISK_MB", *refs.MailMaxDiskMB),
		MailCleanupInterval: resolveStringWithFlag(fs, "mail-cleanup-interval", "OWLMAIL_MAIL_CLEANUP_INTERVAL", *refs.MailCleanupInterval),
		MailIndexPath:       resolveStringWithFlag(fs, "mail-index-path", "OWLMAIL_MAIL_INDEX_PATH", *refs.MailIndexPath),

		WebPort:            resolveIntWithFlag(fs, "web", "OWLMAIL_WEB_PORT", *refs.WebPort),
		WebHost:            resolveStringWithFlag(fs, "web-ip", "OWLMAIL_WEB_HOST", *refs.WebHost),
		WebUser:            resolveStringWithFlag(fs, "web-user", "OWLMAIL_WEB_USER", *refs.WebUser),
		WebPassword:        resolveStringWithFlag(fs, "web-password", "OWLMAIL_WEB_PASSWORD", *refs.WebPassword),
		WebExternalScheme:  ResolveString(nil, "", "OWLMAIL_WEB_EXTERNAL_SCHEME", ""),
		WebExternalURL:     resolveStringWithFlag(fs, "web-external-url", "OWLMAIL_WEB_EXTERNAL_URL", *refs.WebExternalURL),
		BasePathname:       resolveStringWithFlag(fs, "base-pathname", "OWLMAIL_BASE_PATHNAME", *refs.BasePathname),
		MailDevRESTCompat:  resolveBoolWithFlag(fs, "maildev-rest-compat", "OWLMAIL_MAILDEV_REST_COMPAT", *refs.MailDevRESTCompat),
		MetricsEnabled:     resolveBoolWithFlag(fs, "metrics-enabled", "OWLMAIL_METRICS_ENABLED", *refs.MetricsEnabled),
		MCPEnabled:         resolveBoolWithFlag(fs, "mcp-enabled", "OWLMAIL_MCP_ENABLED", *refs.MCPEnabled),
		MCPSessionTimeout:  resolveStringWithFlag(fs, "mcp-session-timeout", "OWLMAIL_MCP_SESSION_TIMEOUT", *refs.MCPSessionTimeout),
		MCPShutdownTimeout: resolveStringWithFlag(fs, "mcp-shutdown-timeout", "OWLMAIL_MCP_SHUTDOWN_TIMEOUT", *refs.MCPShutdownTimeout),

		HTTPSEnabled:  resolveBoolWithFlag(fs, "https", "OWLMAIL_HTTPS_ENABLED", *refs.HTTPSEnabled),
		HTTPSCertFile: resolveStringWithFlag(fs, "https-cert", "OWLMAIL_HTTPS_CERT", *refs.HTTPSCertFile),
		HTTPSKeyFile:  resolveStringWithFlag(fs, "https-key", "OWLMAIL_HTTPS_KEY", *refs.HTTPSKeyFile),

		OutgoingHost:                resolveStringWithFlag(fs, "outgoing-host", "OWLMAIL_OUTGOING_HOST", *refs.OutgoingHost),
		OutgoingPort:                resolveIntWithFlag(fs, "outgoing-port", "OWLMAIL_OUTGOING_PORT", *refs.OutgoingPort),
		OutgoingUser:                resolveStringWithFlag(fs, "outgoing-user", "OWLMAIL_OUTGOING_USER", *refs.OutgoingUser),
		OutgoingPass:                resolveStringWithFlag(fs, "outgoing-pass", "OWLMAIL_OUTGOING_PASSWORD", *refs.OutgoingPass),
		OutgoingSecure:              outgoingSecure,
		OutgoingTLSMode:             outgoingTLSMode,
		OutgoingInsecureSkipVerify:  resolveBoolWithFlag(fs, "outgoing-insecure-skip-verify", "OWLMAIL_OUTGOING_INSECURE_SKIP_VERIFY", *refs.OutgoingInsecureSkipVerify),
		OutgoingConnectTimeout:      resolveStringWithFlag(fs, "outgoing-connect-timeout", "OWLMAIL_OUTGOING_CONNECT_TIMEOUT", *refs.OutgoingConnectTimeout),
		OutgoingTLSHandshakeTimeout: resolveStringWithFlag(fs, "outgoing-tls-handshake-timeout", "OWLMAIL_OUTGOING_TLS_HANDSHAKE_TIMEOUT", *refs.OutgoingTLSHandshakeTimeout),
		OutgoingAuthTimeout:         resolveStringWithFlag(fs, "outgoing-auth-timeout", "OWLMAIL_OUTGOING_AUTH_TIMEOUT", *refs.OutgoingAuthTimeout),
		OutgoingEnvelopeTimeout:     resolveStringWithFlag(fs, "outgoing-envelope-timeout", "OWLMAIL_OUTGOING_ENVELOPE_TIMEOUT", *refs.OutgoingEnvelopeTimeout),
		OutgoingDataTimeout:         resolveStringWithFlag(fs, "outgoing-data-timeout", "OWLMAIL_OUTGOING_DATA_TIMEOUT", *refs.OutgoingDataTimeout),
		OutgoingQuitTimeout:         resolveStringWithFlag(fs, "outgoing-quit-timeout", "OWLMAIL_OUTGOING_QUIT_TIMEOUT", *refs.OutgoingQuitTimeout),
		AutoRelay:                   resolveBoolWithFlag(fs, "auto-relay", "OWLMAIL_AUTO_RELAY", *refs.AutoRelay),
		AutoRelayAddr:               resolveStringWithFlag(fs, "auto-relay-addr", "OWLMAIL_AUTO_RELAY_ADDR", *refs.AutoRelayAddr),
		AutoRelayRules:              resolveStringWithFlag(fs, "auto-relay-rules", "OWLMAIL_AUTO_RELAY_RULES", *refs.AutoRelayRules),

		SMTPUser:           resolveStringWithFlag(fs, "smtp-user", "OWLMAIL_SMTP_USER", *refs.SMTPUser),
		SMTPPassword:       resolveStringWithFlag(fs, "smtp-password", "OWLMAIL_SMTP_PASSWORD", *refs.SMTPPassword),
		SMTPAuthRequireTLS: resolveBoolWithFlag(fs, "smtp-auth-require-tls", "OWLMAIL_SMTP_AUTH_REQUIRE_TLS", *refs.SMTPAuthRequireTLS),

		TLSEnabled:  resolveBoolWithFlag(fs, "tls", "OWLMAIL_TLS_ENABLED", *refs.TLSEnabled),
		TLSCertFile: resolveStringWithFlag(fs, "tls-cert", "OWLMAIL_TLS_CERT", *refs.TLSCertFile),
		TLSKeyFile:  resolveStringWithFlag(fs, "tls-key", "OWLMAIL_TLS_KEY", *refs.TLSKeyFile),

		LogLevel:  resolveLogLevelWithFlag(fs, "log-level", *refs.LogLevel),
		LogFormat: resolveStringWithFlag(fs, "log-format", "OWLMAIL_LOG_FORMAT", *refs.LogFormat),

		UseUUIDForEmailID:      resolveBoolWithFlag(fs, "use-uuid-for-email-id", "OWLMAIL_USE_UUID_FOR_EMAIL_ID", *refs.UseUUIDForEmailID),
		S3Enabled:              resolveBoolWithFlag(fs, "s3-enabled", "OWLMAIL_S3_ENABLED", *refs.S3Enabled),
		S3Endpoint:             resolveStringWithFlag(fs, "s3-endpoint", "OWLMAIL_S3_ENDPOINT", *refs.S3Endpoint),
		S3Region:               resolveStringWithFlag(fs, "s3-region", "OWLMAIL_S3_REGION", *refs.S3Region),
		S3Bucket:               resolveStringWithFlag(fs, "s3-bucket", "OWLMAIL_S3_BUCKET", *refs.S3Bucket),
		S3Prefix:               resolveStringWithFlag(fs, "s3-prefix", "OWLMAIL_S3_PREFIX", *refs.S3Prefix),
		S3AccessKeyID:          resolveStringWithFlag(fs, "s3-access-key", "OWLMAIL_S3_ACCESS_KEY", *refs.S3AccessKeyID),
		S3SecretAccessKey:      resolveStringWithFlag(fs, "s3-secret-key", "OWLMAIL_S3_SECRET_KEY", *refs.S3SecretAccessKey),
		S3SessionToken:         resolveStringWithFlag(fs, "s3-session-token", "OWLMAIL_S3_SESSION_TOKEN", *refs.S3SessionToken),
		S3UsePathStyle:         resolveBoolWithFlag(fs, "s3-use-path-style", "OWLMAIL_S3_USE_PATH_STYLE", *refs.S3UsePathStyle),
		S3StartupCheck:         resolveBoolWithFlag(fs, "s3-startup-check", "OWLMAIL_S3_STARTUP_CHECK", *refs.S3StartupCheck),
		S3HealthInterval:       resolveStringWithFlag(fs, "s3-health-check-interval", "OWLMAIL_S3_HEALTH_CHECK_INTERVAL", *refs.S3HealthInterval),
		S3HealthTimeout:        resolveStringWithFlag(fs, "s3-health-check-timeout", "OWLMAIL_S3_HEALTH_CHECK_TIMEOUT", *refs.S3HealthTimeout),
		WebhookConfig:          resolveStringWithFlag(fs, "webhook-config", "OWLMAIL_WEBHOOK_CONFIG", *refs.WebhookConfig),
		WebhookMaxConcurrency:  resolveIntWithFlag(fs, "webhook-max-concurrency", "OWLMAIL_WEBHOOK_MAX_CONCURRENCY", *refs.WebhookMaxConcurrency),
		WebhookRedisURL:        resolveStringWithFlag(fs, "webhook-redis-url", "OWLMAIL_WEBHOOK_REDIS_URL", *refs.WebhookRedisURL),
		WebhookRedisPrefix:     resolveStringWithFlag(fs, "webhook-redis-prefix", "OWLMAIL_WEBHOOK_REDIS_PREFIX", *refs.WebhookRedisPrefix),
		WebhookShutdownTimeout: resolveStringWithFlag(fs, "webhook-shutdown-timeout", "OWLMAIL_WEBHOOK_SHUTDOWN_TIMEOUT", *refs.WebhookShutdownTimeout),
	}
}

// resolveSMTPTimeoutWithFlag keeps an omitted field compatible with manually
// constructed Config values while preserving an explicitly empty flag as an
// invalid duration for startup validation.
func resolveSMTPTimeoutWithFlag(fs *flag.FlagSet, flagName, owlmailKey, flagValue string) string {
	if flagutil.HasFlag(fs, flagName) && flagValue == "" {
		return invalidSMTPTimeout
	}
	return resolveStringWithFlag(fs, flagName, owlmailKey, flagValue)
}

// resolveSMTPMaxConcurrencyWithFlag keeps the established CLI-over-environment
// priority while rejecting malformed environment values. ResolveConfig cannot
// return an error without breaking its public API, so malformed values use the
// invalid sentinel -1 and are rejected by validation and server construction.
func resolveSMTPMaxConcurrencyWithFlag(fs *flag.FlagSet, flagValue int) int {
	if flagutil.HasFlag(fs, "smtp-max-concurrency") {
		return flagValue
	}
	if !env.Has("OWLMAIL_SMTP_MAX_CONCURRENCY") {
		return flagValue
	}
	raw := env.Get("OWLMAIL_SMTP_MAX_CONCURRENCY", "")
	if raw == "" {
		return flagValue
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return -1
	}
	return value
}

// resolveSMTPMaxRecipientsWithFlag preserves invalid explicit input as a
// validation failure instead of silently falling back to the default.
func resolveSMTPMaxRecipientsWithFlag(fs *flag.FlagSet, flagValue int) int {
	if flagutil.HasFlag(fs, "smtp-max-recipients") {
		if flagValue == 0 {
			return -1
		}
		return flagValue
	}
	if !env.Has("OWLMAIL_SMTP_MAX_RECIPIENTS") {
		return flagValue
	}
	raw := env.Get("OWLMAIL_SMTP_MAX_RECIPIENTS", "")
	if raw == "" {
		return flagValue
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value == 0 {
		return -1
	}
	return value
}

// ParseFlags is a convenience function that defines flags, parses arguments, and resolves config.
// Note: This uses flag.CommandLine, so it should only be used in main().
// For tests, use DefineFlags and ResolveConfig separately.
func ParseFlags() *Config {
	fs := flag.CommandLine
	refs := DefineFlags(fs)
	flag.Parse()
	return ResolveConfig(fs, refs)
}

// resolveStringWithFlag resolves a string value considering CLI flag was already parsed
func resolveStringWithFlag(fs *flag.FlagSet, flagName, owlmailKey, flagValue string) string {
	// If flag was explicitly set, use it
	if flagutil.HasFlag(fs, flagName) {
		return flagValue
	}
	// Otherwise, check environment variables
	return ResolveString(nil, flagName, owlmailKey, flagValue)
}

// resolveIntWithFlag resolves an int value considering CLI flag was already parsed
func resolveIntWithFlag(fs *flag.FlagSet, flagName, owlmailKey string, flagValue int) int {
	// If flag was explicitly set, use it
	if flagutil.HasFlag(fs, flagName) {
		return flagValue
	}
	// Otherwise, check environment variables
	return ResolveInt(nil, flagName, owlmailKey, flagValue)
}

// resolveBoolWithFlag resolves a bool value considering CLI flag was already parsed
func resolveBoolWithFlag(fs *flag.FlagSet, flagName, owlmailKey string, flagValue bool) bool {
	// If flag was explicitly set, use it
	if flagutil.HasFlag(fs, flagName) {
		return flagValue
	}
	// Otherwise, check environment variables
	return ResolveBool(nil, flagName, owlmailKey, flagValue)
}

// resolveLogLevelWithFlag resolves log level considering CLI flag was already parsed
func resolveLogLevelWithFlag(fs *flag.FlagSet, flagName, flagValue string) string {
	// If flag was explicitly set, use it
	if flagutil.HasFlag(fs, flagName) {
		return flagValue
	}
	// Otherwise, check environment variables
	return ResolveLogLevel(nil, flagName, flagValue)
}
