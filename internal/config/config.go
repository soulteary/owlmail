// Package config provides configuration parsing with MailDev environment variable compatibility.
// It uses cli-kit for environment variable management and validation.
//
// Priority order: CLI flags > MAILDEV_* env vars > OWLMAIL_* env vars > default values
package config

import (
	"flag"

	"github.com/soulteary/cli-kit/env"
	"github.com/soulteary/cli-kit/flagutil"
)

// DefaultWebhookMaxConcurrency is the recommended concurrent email delivery
// limit. Zero remains available as an explicit unlimited mode.
const DefaultWebhookMaxConcurrency = 8

// EnvMapping defines the mapping from MailDev environment variables to OwlMail environment variables.
// This maintains backward compatibility with MailDev deployments.
var EnvMapping = map[string]string{
	// SMTP server configuration
	"MAILDEV_SMTP_PORT":      "OWLMAIL_SMTP_PORT",
	"MAILDEV_IP":             "OWLMAIL_SMTP_HOST",
	"MAILDEV_MAIL_DIRECTORY": "OWLMAIL_MAIL_DIR",

	// Web API configuration
	"MAILDEV_WEB_PORT": "OWLMAIL_WEB_PORT",
	"MAILDEV_WEB_IP":   "OWLMAIL_WEB_HOST",
	"MAILDEV_WEB_USER": "OWLMAIL_WEB_USER",
	"MAILDEV_WEB_PASS": "OWLMAIL_WEB_PASSWORD",

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
	SMTPPort int
	SMTPHost string
	MailDir  string

	// Web API configuration
	WebPort     int
	WebHost     string
	WebUser     string
	WebPassword string

	// HTTPS configuration
	HTTPSEnabled  bool
	HTTPSCertFile string
	HTTPSKeyFile  string

	// Outgoing mail configuration
	OutgoingHost   string
	OutgoingPort   int
	OutgoingUser   string
	OutgoingPass   string
	OutgoingSecure bool
	AutoRelay      bool
	AutoRelayAddr  string
	AutoRelayRules string

	// SMTP authentication
	SMTPUser     string
	SMTPPassword string

	// TLS configuration for SMTP
	TLSEnabled  bool
	TLSCertFile string
	TLSKeyFile  string

	// Logging configuration
	LogLevel string

	// Email ID configuration
	UseUUIDForEmailID bool

	// Webhook forwarding configuration
	WebhookConfig         string
	WebhookMaxConcurrency int
}

// DefaultConfig returns a Config with default values
func DefaultConfig() *Config {
	return &Config{
		SMTPPort:              1025,
		SMTPHost:              "localhost",
		MailDir:               "",
		WebPort:               1080,
		WebHost:               "localhost",
		WebUser:               "",
		WebPassword:           "",
		HTTPSEnabled:          false,
		HTTPSCertFile:         "",
		HTTPSKeyFile:          "",
		OutgoingHost:          "",
		OutgoingPort:          587,
		OutgoingUser:          "",
		OutgoingPass:          "",
		OutgoingSecure:        false,
		AutoRelay:             false,
		AutoRelayAddr:         "",
		AutoRelayRules:        "",
		SMTPUser:              "",
		SMTPPassword:          "",
		TLSEnabled:            false,
		TLSCertFile:           "",
		TLSKeyFile:            "",
		LogLevel:              "normal",
		UseUUIDForEmailID:     false,
		WebhookConfig:         "",
		WebhookMaxConcurrency: DefaultWebhookMaxConcurrency,
	}
}

// FlagRefs holds references to all flag values for resolution after parsing.
type FlagRefs struct {
	SMTPPort              *int
	SMTPHost              *string
	MailDir               *string
	WebPort               *int
	WebHost               *string
	WebUser               *string
	WebPassword           *string
	HTTPSEnabled          *bool
	HTTPSCertFile         *string
	HTTPSKeyFile          *string
	OutgoingHost          *string
	OutgoingPort          *int
	OutgoingUser          *string
	OutgoingPass          *string
	OutgoingSecure        *bool
	AutoRelay             *bool
	AutoRelayAddr         *string
	AutoRelayRules        *string
	SMTPUser              *string
	SMTPPassword          *string
	TLSEnabled            *bool
	TLSCertFile           *string
	TLSKeyFile            *string
	LogLevel              *string
	UseUUIDForEmailID     *bool
	WebhookConfig         *string
	WebhookMaxConcurrency *int
}

// DefineFlags defines all configuration flags on the given FlagSet.
// It returns FlagRefs which should be passed to ResolveConfig after parsing.
func DefineFlags(fs *flag.FlagSet) *FlagRefs {
	cfg := DefaultConfig()
	return &FlagRefs{
		SMTPPort:              fs.Int("smtp", cfg.SMTPPort, "SMTP port to catch emails"),
		SMTPHost:              fs.String("ip", cfg.SMTPHost, "IP address to bind SMTP service to"),
		MailDir:               fs.String("mail-directory", cfg.MailDir, "Directory for persisting mails"),
		WebPort:               fs.Int("web", cfg.WebPort, "Web API port"),
		WebHost:               fs.String("web-ip", cfg.WebHost, "IP address to bind Web API to"),
		WebUser:               fs.String("web-user", cfg.WebUser, "HTTP Basic Auth username"),
		WebPassword:           fs.String("web-password", cfg.WebPassword, "HTTP Basic Auth password"),
		HTTPSEnabled:          fs.Bool("https", cfg.HTTPSEnabled, "Enable HTTPS for Web API"),
		HTTPSCertFile:         fs.String("https-cert", cfg.HTTPSCertFile, "HTTPS certificate file path"),
		HTTPSKeyFile:          fs.String("https-key", cfg.HTTPSKeyFile, "HTTPS private key file path"),
		OutgoingHost:          fs.String("outgoing-host", cfg.OutgoingHost, "Outgoing SMTP server host"),
		OutgoingPort:          fs.Int("outgoing-port", cfg.OutgoingPort, "Outgoing SMTP server port"),
		OutgoingUser:          fs.String("outgoing-user", cfg.OutgoingUser, "Outgoing SMTP server username"),
		OutgoingPass:          fs.String("outgoing-pass", cfg.OutgoingPass, "Outgoing SMTP server password"),
		OutgoingSecure:        fs.Bool("outgoing-secure", cfg.OutgoingSecure, "Use TLS for outgoing SMTP"),
		AutoRelay:             fs.Bool("auto-relay", cfg.AutoRelay, "Automatically relay all emails"),
		AutoRelayAddr:         fs.String("auto-relay-addr", cfg.AutoRelayAddr, "Auto relay to specific address"),
		AutoRelayRules:        fs.String("auto-relay-rules", cfg.AutoRelayRules, "JSON file path for auto relay rules"),
		SMTPUser:              fs.String("smtp-user", cfg.SMTPUser, "SMTP server username for authentication"),
		SMTPPassword:          fs.String("smtp-password", cfg.SMTPPassword, "SMTP server password for authentication"),
		TLSEnabled:            fs.Bool("tls", cfg.TLSEnabled, "Enable TLS/STARTTLS for SMTP server"),
		TLSCertFile:           fs.String("tls-cert", cfg.TLSCertFile, "TLS certificate file path"),
		TLSKeyFile:            fs.String("tls-key", cfg.TLSKeyFile, "TLS private key file path"),
		LogLevel:              fs.String("log-level", cfg.LogLevel, "Log level: silent, normal, or verbose"),
		UseUUIDForEmailID:     fs.Bool("use-uuid-for-email-id", cfg.UseUUIDForEmailID, "Use UUID instead of random string for email IDs"),
		WebhookConfig:         fs.String("webhook-config", cfg.WebhookConfig, "JSON file path for webhook forwarding targets"),
		WebhookMaxConcurrency: fs.Int("webhook-max-concurrency", cfg.WebhookMaxConcurrency, "Maximum concurrent webhook deliveries (0 = unlimited)"),
	}
}

// ResolveConfig resolves configuration from flag values and environment variables.
// This should be called after DefineFlags and fs.Parse().
// Priority: CLI flags > MAILDEV_* env > OWLMAIL_* env > default values
func ResolveConfig(fs *flag.FlagSet, refs *FlagRefs) *Config {
	return &Config{
		SMTPPort: resolveIntWithFlag(fs, "smtp", "OWLMAIL_SMTP_PORT", *refs.SMTPPort),
		SMTPHost: resolveStringWithFlag(fs, "ip", "OWLMAIL_SMTP_HOST", *refs.SMTPHost),
		MailDir:  resolveStringWithFlag(fs, "mail-directory", "OWLMAIL_MAIL_DIR", *refs.MailDir),

		WebPort:     resolveIntWithFlag(fs, "web", "OWLMAIL_WEB_PORT", *refs.WebPort),
		WebHost:     resolveStringWithFlag(fs, "web-ip", "OWLMAIL_WEB_HOST", *refs.WebHost),
		WebUser:     resolveStringWithFlag(fs, "web-user", "OWLMAIL_WEB_USER", *refs.WebUser),
		WebPassword: resolveStringWithFlag(fs, "web-password", "OWLMAIL_WEB_PASSWORD", *refs.WebPassword),

		HTTPSEnabled:  resolveBoolWithFlag(fs, "https", "OWLMAIL_HTTPS_ENABLED", *refs.HTTPSEnabled),
		HTTPSCertFile: resolveStringWithFlag(fs, "https-cert", "OWLMAIL_HTTPS_CERT", *refs.HTTPSCertFile),
		HTTPSKeyFile:  resolveStringWithFlag(fs, "https-key", "OWLMAIL_HTTPS_KEY", *refs.HTTPSKeyFile),

		OutgoingHost:   resolveStringWithFlag(fs, "outgoing-host", "OWLMAIL_OUTGOING_HOST", *refs.OutgoingHost),
		OutgoingPort:   resolveIntWithFlag(fs, "outgoing-port", "OWLMAIL_OUTGOING_PORT", *refs.OutgoingPort),
		OutgoingUser:   resolveStringWithFlag(fs, "outgoing-user", "OWLMAIL_OUTGOING_USER", *refs.OutgoingUser),
		OutgoingPass:   resolveStringWithFlag(fs, "outgoing-pass", "OWLMAIL_OUTGOING_PASSWORD", *refs.OutgoingPass),
		OutgoingSecure: resolveBoolWithFlag(fs, "outgoing-secure", "OWLMAIL_OUTGOING_SECURE", *refs.OutgoingSecure),
		AutoRelay:      resolveBoolWithFlag(fs, "auto-relay", "OWLMAIL_AUTO_RELAY", *refs.AutoRelay),
		AutoRelayAddr:  resolveStringWithFlag(fs, "auto-relay-addr", "OWLMAIL_AUTO_RELAY_ADDR", *refs.AutoRelayAddr),
		AutoRelayRules: resolveStringWithFlag(fs, "auto-relay-rules", "OWLMAIL_AUTO_RELAY_RULES", *refs.AutoRelayRules),

		SMTPUser:     resolveStringWithFlag(fs, "smtp-user", "OWLMAIL_SMTP_USER", *refs.SMTPUser),
		SMTPPassword: resolveStringWithFlag(fs, "smtp-password", "OWLMAIL_SMTP_PASSWORD", *refs.SMTPPassword),

		TLSEnabled:  resolveBoolWithFlag(fs, "tls", "OWLMAIL_TLS_ENABLED", *refs.TLSEnabled),
		TLSCertFile: resolveStringWithFlag(fs, "tls-cert", "OWLMAIL_TLS_CERT", *refs.TLSCertFile),
		TLSKeyFile:  resolveStringWithFlag(fs, "tls-key", "OWLMAIL_TLS_KEY", *refs.TLSKeyFile),

		LogLevel: resolveLogLevelWithFlag(fs, "log-level", *refs.LogLevel),

		UseUUIDForEmailID:     resolveBoolWithFlag(fs, "use-uuid-for-email-id", "OWLMAIL_USE_UUID_FOR_EMAIL_ID", *refs.UseUUIDForEmailID),
		WebhookConfig:         resolveStringWithFlag(fs, "webhook-config", "OWLMAIL_WEBHOOK_CONFIG", *refs.WebhookConfig),
		WebhookMaxConcurrency: resolveIntWithFlag(fs, "webhook-max-concurrency", "OWLMAIL_WEBHOOK_MAX_CONCURRENCY", *refs.WebhookMaxConcurrency),
	}
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
