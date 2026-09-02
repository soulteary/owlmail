package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/soulteary/cli-kit/validator"
)

// ValidLogLevels defines the allowed log level values
var ValidLogLevels = []string{"silent", "normal", "verbose"}

// ValidateConfig validates all configuration values and returns an error if any are invalid.
func ValidateConfig(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config cannot be nil")
	}

	// Validate ports
	if err := ValidatePort(cfg.SMTPPort, "SMTP port"); err != nil {
		return err
	}
	if err := ValidatePort(cfg.WebPort, "Web port"); err != nil {
		return err
	}
	if cfg.OutgoingHost != "" {
		if err := ValidatePort(cfg.OutgoingPort, "Outgoing port"); err != nil {
			return err
		}
	}
	if cfg.SMTPMaxMessageMB <= 0 {
		return fmt.Errorf("SMTP max message size must be greater than zero")
	}
	const maxMessageMB = int64(^uint64(0)>>1) >> 20
	if int64(cfg.SMTPMaxMessageMB) > maxMessageMB {
		return fmt.Errorf("SMTP max message size is too large")
	}
	if cfg.SMTPMaxConcurrency < 0 {
		return fmt.Errorf("SMTP max concurrency must be a non-negative integer")
	}
	if readTimeout, err := time.ParseDuration(cfg.SMTPReadTimeout); err != nil || readTimeout <= 0 {
		return fmt.Errorf("SMTP read timeout must be a positive duration")
	}
	if writeTimeout, err := time.ParseDuration(cfg.SMTPWriteTimeout); err != nil || writeTimeout <= 0 {
		return fmt.Errorf("SMTP write timeout must be a positive duration")
	}
	if cfg.SMTPMaxRecipients <= 0 {
		return fmt.Errorf("SMTP max recipients must be greater than zero")
	}
	if (cfg.SMTPUser == "") != (cfg.SMTPPassword == "") {
		return fmt.Errorf("SMTP username and password must be configured together")
	}
	if cfg.SMTPAuthRequireTLS && !cfg.TLSEnabled {
		return fmt.Errorf("SMTP AUTH cannot require TLS unless SMTP TLS is enabled")
	}
	if cfg.WebExternalScheme != "" && cfg.WebExternalScheme != "http" && cfg.WebExternalScheme != "https" {
		return fmt.Errorf("web external scheme must be http or https")
	}
	if _, err := NormalizeBasePathname(cfg.BasePathname); err != nil {
		return err
	}
	if sessionTimeout, err := time.ParseDuration(cfg.MCPSessionTimeout); err != nil || sessionTimeout <= 0 {
		return fmt.Errorf("MCP session timeout must be a positive duration")
	}
	if shutdownTimeout, err := time.ParseDuration(cfg.MCPShutdownTimeout); err != nil || shutdownTimeout <= 0 {
		return fmt.Errorf("MCP shutdown timeout must be a positive duration")
	}
	if cfg.MailRetentionDays < 0 || cfg.MailMaxMessages < 0 || cfg.MailMaxDiskMB < 0 {
		return fmt.Errorf("mail retention limits cannot be negative")
	}
	cleanupInterval, err := time.ParseDuration(cfg.MailCleanupInterval)
	if err != nil || cleanupInterval <= 0 {
		return fmt.Errorf("mail cleanup interval must be a positive duration")
	}

	// Validate log level
	if err := ValidateLogLevel(cfg.LogLevel); err != nil {
		return err
	}

	// Validate TLS configuration
	if cfg.TLSEnabled {
		if cfg.TLSCertFile == "" {
			return fmt.Errorf("TLS certificate file is required when TLS is enabled")
		}
		if cfg.TLSKeyFile == "" {
			return fmt.Errorf("TLS key file is required when TLS is enabled")
		}
	}

	// Validate HTTPS configuration
	if cfg.HTTPSEnabled {
		if cfg.HTTPSCertFile == "" {
			return fmt.Errorf("HTTPS certificate file is required when HTTPS is enabled")
		}
		if cfg.HTTPSKeyFile == "" {
			return fmt.Errorf("HTTPS key file is required when HTTPS is enabled")
		}
	}

	if cfg.S3Enabled {
		if strings.TrimSpace(cfg.S3Bucket) == "" {
			return fmt.Errorf("S3 bucket is required when S3 attachment storage is enabled")
		}
		if strings.TrimSpace(cfg.S3Region) == "" {
			return fmt.Errorf("S3 region is required when S3 attachment storage is enabled")
		}
		if (cfg.S3AccessKeyID == "") != (cfg.S3SecretAccessKey == "") {
			return fmt.Errorf("S3 access key and secret key must be configured together")
		}
		if cfg.S3SessionToken != "" && cfg.S3AccessKeyID == "" {
			return fmt.Errorf("S3 session token requires static access key credentials")
		}
		if cfg.S3Endpoint != "" {
			endpoint, err := url.Parse(cfg.S3Endpoint)
			if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
				return fmt.Errorf("S3 endpoint must be an HTTP or HTTPS URL without credentials, query, or fragment")
			}
		}
		if healthInterval, err := time.ParseDuration(cfg.S3HealthInterval); err != nil || healthInterval <= 0 {
			return fmt.Errorf("S3 health check interval must be a positive duration")
		}
		if healthTimeout, err := time.ParseDuration(cfg.S3HealthTimeout); err != nil || healthTimeout <= 0 {
			return fmt.Errorf("S3 health check timeout must be a positive duration")
		}
	}

	// Validate auto relay rules file path if specified
	if cfg.AutoRelayRules != "" {
		if _, err := ValidatePath(cfg.AutoRelayRules); err != nil {
			return fmt.Errorf("auto relay rules file: %w", err)
		}
	}
	if cfg.WebhookConfig != "" {
		if _, err := ValidatePath(cfg.WebhookConfig); err != nil {
			return fmt.Errorf("webhook config file: %w", err)
		}
	}
	if cfg.WebhookMaxConcurrency < 0 {
		return fmt.Errorf("webhook max concurrency must be zero or greater")
	}
	if strings.TrimSpace(cfg.WebhookRedisPrefix) == "" {
		return fmt.Errorf("webhook Redis prefix cannot be empty")
	}
	shutdownTimeout, err := time.ParseDuration(cfg.WebhookShutdownTimeout)
	if err != nil || shutdownTimeout <= 0 {
		return fmt.Errorf("webhook shutdown timeout must be a positive duration")
	}

	return nil
}

// NormalizeBasePathname converts a browser-visible URL prefix to the canonical
// form used by both the HTTP router and embedded browser assets. Root is stored
// as an empty string; non-root values have one leading slash and no trailing
// slash.
func NormalizeBasePathname(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || value == "/" {
		return "", nil
	}
	if strings.ContainsAny(value, "?#\\") || strings.Contains(value, "://") {
		return "", fmt.Errorf("base pathname must be a URL path without query, fragment, backslash, or scheme")
	}
	if strings.ContainsAny(value, ":*+<>()") {
		return "", fmt.Errorf("base pathname cannot contain router metacharacters")
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}

	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("base pathname must be a valid URL path without query or fragment")
	}
	escapedSegments := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
	normalizedSegments := make([]string, 0, len(escapedSegments))
	for _, segment := range escapedSegments {
		decoded, decodeErr := url.PathUnescape(segment)
		if decodeErr != nil || decoded == "" || decoded == "." || decoded == ".." ||
			strings.ContainsAny(decoded, "/\\%?#:*+<>()") || strings.IndexFunc(decoded, unicode.IsControl) >= 0 {
			return "", fmt.Errorf("base pathname contains an unsafe path segment")
		}
		normalizedSegments = append(normalizedSegments, url.PathEscape(decoded))
	}

	if strings.Contains(parsed.Path, "//") {
		return "", fmt.Errorf("base pathname cannot contain empty path segments")
	}
	return "/" + strings.Join(normalizedSegments, "/"), nil
}

// ValidatePort validates that a port number is within the valid range (1-65535).
func ValidatePort(port int, name string) error {
	if err := validator.ValidatePort(port); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

// ValidateLogLevel validates that the log level is one of the allowed values.
func ValidateLogLevel(level string) error {
	if err := validator.ValidateEnum(level, ValidLogLevels, false); err != nil {
		return fmt.Errorf("log level: %w", err)
	}
	return nil
}

// ValidatePath validates a file path to prevent path traversal attacks.
// Returns the absolute path if valid, or an error if the path is invalid.
func ValidatePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	absPath, err := validator.ValidatePath(path, nil)
	if err != nil {
		return "", err
	}

	return absPath, nil
}

// ValidatePathWithOptions validates a file path with custom options.
func ValidatePathWithOptions(path string, allowRelative bool, allowedDirs []string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	opts := &validator.PathOptions{
		AllowRelative:  allowRelative,
		AllowedDirs:    allowedDirs,
		CheckTraversal: true,
	}

	return validator.ValidatePath(path, opts)
}

// ValidateFileExists validates that a file exists at the given path.
func ValidateFileExists(path string) error {
	return validator.ValidateFileExists(path)
}

// ValidateFileReadable validates that a file exists and is readable.
func ValidateFileReadable(path string) error {
	return validator.ValidateFileReadable(path)
}

// ValidateDirExists validates that a directory exists at the given path.
func ValidateDirExists(path string) error {
	return validator.ValidateDirExists(path)
}

// ValidateDirWritable validates that a directory exists and is writable.
func ValidateDirWritable(path string) error {
	return validator.ValidateDirWritable(path)
}

// ValidateMailDir validates the mail directory configuration.
// If mailDir is empty, no validation is performed (emails stored in memory).
// If mailDir is specified, validates it's a valid, writable directory path.
func ValidateMailDir(mailDir string) error {
	if mailDir == "" {
		// Empty mail directory means in-memory storage, which is valid
		return nil
	}

	// Validate path doesn't contain traversal characters
	if _, err := ValidatePath(mailDir); err != nil {
		return fmt.Errorf("mail directory: %w", err)
	}

	return nil
}

// ValidateTLSFiles validates that TLS certificate and key files exist and are readable.
func ValidateTLSFiles(certFile, keyFile string) error {
	if err := ValidateFileReadable(certFile); err != nil {
		return fmt.Errorf("TLS certificate file: %w", err)
	}
	if err := ValidateFileReadable(keyFile); err != nil {
		return fmt.Errorf("TLS key file: %w", err)
	}
	return nil
}

// ValidateHTTPSFiles validates that HTTPS certificate and key files exist and are readable.
func ValidateHTTPSFiles(certFile, keyFile string) error {
	if err := ValidateFileReadable(certFile); err != nil {
		return fmt.Errorf("HTTPS certificate file: %w", err)
	}
	if err := ValidateFileReadable(keyFile); err != nil {
		return fmt.Errorf("HTTPS key file: %w", err)
	}
	return nil
}

// ParseLogLevel parses a log level string and returns a normalized value.
// Returns the default value if the input is invalid.
func ParseLogLevel(levelStr string) string {
	if err := ValidateLogLevel(levelStr); err != nil {
		return "normal"
	}
	return levelStr
}
