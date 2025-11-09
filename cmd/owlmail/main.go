package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/soulteary/owlmail/internal/api"
	"github.com/soulteary/owlmail/internal/common"
	"github.com/soulteary/owlmail/internal/maildev"
	"github.com/soulteary/owlmail/internal/mailserver"
	"github.com/soulteary/owlmail/internal/outgoing"
)

// getEnvString returns environment variable value or default
func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt returns environment variable value as int or default
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvBool returns environment variable value as bool or default
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

// getLogLevelFromEnv returns log level from environment variable
// Supports both MailDev (MAILDEV_VERBOSE/MAILDEV_SILENT) and OwlMail (OWLMAIL_LOG_LEVEL) environment variables
func getLogLevelFromEnv() common.LogLevel {
	levelStr := maildev.GetMailDevLogLevel("normal")
	switch levelStr {
	case "silent":
		return common.LogLevelSilent
	case "verbose":
		return common.LogLevelVerbose
	default:
		return common.LogLevelNormal
	}
}

func main() {
	var (
		// SMTP server configuration
		// Supports both MAILDEV_* and OWLMAIL_* environment variables
		smtpPort = flag.Int("smtp", maildev.GetMailDevEnvInt("OWLMAIL_SMTP_PORT", 1025), "SMTP port to catch emails")
		smtpHost = flag.String("ip", maildev.GetMailDevEnvString("OWLMAIL_SMTP_HOST", "localhost"), "IP address to bind SMTP service to")
		mailDir  = flag.String("mail-directory", maildev.GetMailDevEnvString("OWLMAIL_MAIL_DIR", ""), "Directory for persisting mails")

		// Web API configuration
		webPort     = flag.Int("web", maildev.GetMailDevEnvInt("OWLMAIL_WEB_PORT", 1080), "Web API port")
		webHost     = flag.String("web-ip", maildev.GetMailDevEnvString("OWLMAIL_WEB_HOST", "localhost"), "IP address to bind Web API to")
		webUser     = flag.String("web-user", maildev.GetMailDevEnvString("OWLMAIL_WEB_USER", ""), "HTTP Basic Auth username")
		webPassword = flag.String("web-password", maildev.GetMailDevEnvString("OWLMAIL_WEB_PASSWORD", ""), "HTTP Basic Auth password")

		// HTTPS configuration
		httpsEnabled  = flag.Bool("https", maildev.GetMailDevEnvBool("OWLMAIL_HTTPS_ENABLED", false), "Enable HTTPS for Web API")
		httpsCertFile = flag.String("https-cert", maildev.GetMailDevEnvString("OWLMAIL_HTTPS_CERT", ""), "HTTPS certificate file path")
		httpsKeyFile  = flag.String("https-key", maildev.GetMailDevEnvString("OWLMAIL_HTTPS_KEY", ""), "HTTPS private key file path")

		// Outgoing mail configuration
		outgoingHost   = flag.String("outgoing-host", maildev.GetMailDevEnvString("OWLMAIL_OUTGOING_HOST", ""), "Outgoing SMTP server host")
		outgoingPort   = flag.Int("outgoing-port", maildev.GetMailDevEnvInt("OWLMAIL_OUTGOING_PORT", 587), "Outgoing SMTP server port")
		outgoingUser   = flag.String("outgoing-user", maildev.GetMailDevEnvString("OWLMAIL_OUTGOING_USER", ""), "Outgoing SMTP server username")
		outgoingPass   = flag.String("outgoing-pass", maildev.GetMailDevEnvString("OWLMAIL_OUTGOING_PASSWORD", ""), "Outgoing SMTP server password")
		outgoingSecure = flag.Bool("outgoing-secure", maildev.GetMailDevEnvBool("OWLMAIL_OUTGOING_SECURE", false), "Use TLS for outgoing SMTP")
		autoRelay      = flag.Bool("auto-relay", maildev.GetMailDevEnvBool("OWLMAIL_AUTO_RELAY", false), "Automatically relay all emails")
		autoRelayAddr  = flag.String("auto-relay-addr", maildev.GetMailDevEnvString("OWLMAIL_AUTO_RELAY_ADDR", ""), "Auto relay to specific address")
		autoRelayRules = flag.String("auto-relay-rules", maildev.GetMailDevEnvString("OWLMAIL_AUTO_RELAY_RULES", ""), "JSON file path for auto relay rules")

		// SMTP authentication
		smtpUser     = flag.String("smtp-user", maildev.GetMailDevEnvString("OWLMAIL_SMTP_USER", ""), "SMTP server username for authentication")
		smtpPassword = flag.String("smtp-password", maildev.GetMailDevEnvString("OWLMAIL_SMTP_PASSWORD", ""), "SMTP server password for authentication")

		// TLS configuration for SMTP
		tlsEnabled  = flag.Bool("tls", maildev.GetMailDevEnvBool("OWLMAIL_TLS_ENABLED", false), "Enable TLS/STARTTLS for SMTP server")
		tlsCertFile = flag.String("tls-cert", maildev.GetMailDevEnvString("OWLMAIL_TLS_CERT", ""), "TLS certificate file path")
		tlsKeyFile  = flag.String("tls-key", maildev.GetMailDevEnvString("OWLMAIL_TLS_KEY", ""), "TLS private key file path")

		// Logging configuration
		// Supports both MAILDEV_VERBOSE/MAILDEV_SILENT and OWLMAIL_LOG_LEVEL
		logLevel = flag.String("log-level", maildev.GetMailDevLogLevel("normal"), "Log level: silent, normal, or verbose")
	)
	flag.Parse()

	// Initialize logger based on log level
	var level common.LogLevel
	switch *logLevel {
	case "silent":
		level = common.LogLevelSilent
	case "verbose":
		level = common.LogLevelVerbose
	default:
		level = common.LogLevelNormal
	}
	common.InitLogger(level)

	// Setup outgoing mail config if provided
	var outgoingConfig *outgoing.OutgoingConfig
	if *outgoingHost != "" {
		outgoingConfig = &outgoing.OutgoingConfig{
			Host:          *outgoingHost,
			Port:          *outgoingPort,
			User:          *outgoingUser,
			Password:      *outgoingPass,
			Secure:        *outgoingSecure,
			AutoRelay:     *autoRelay,
			AutoRelayAddr: *autoRelayAddr,
		}

		// Load auto relay rules from JSON file if provided
		if *autoRelayRules != "" {
			allowRules, denyRules, err := loadAutoRelayRules(*autoRelayRules)
			if err != nil {
				if fatalErr := common.Fatal("Failed to load auto relay rules: %v", err); fatalErr != nil {
					// In test environments, this will return an error instead of exiting
					return
				}
			}
			outgoingConfig.AllowRules = allowRules
			outgoingConfig.DenyRules = denyRules
			if len(allowRules) > 0 || len(denyRules) > 0 {
				common.Log("Loaded auto relay rules: %d allow rules, %d deny rules", len(allowRules), len(denyRules))
			}
		}
	}

	// Setup SMTP authentication config
	var authConfig *mailserver.SMTPAuthConfig
	if *smtpUser != "" && *smtpPassword != "" {
		authConfig = &mailserver.SMTPAuthConfig{
			Username: *smtpUser,
			Password: *smtpPassword,
			Enabled:  true,
		}
	}

	// Setup TLS config
	var tlsConfig *mailserver.TLSConfig
	if *tlsEnabled {
		tlsConfig = &mailserver.TLSConfig{
			CertFile: *tlsCertFile,
			KeyFile:  *tlsKeyFile,
			Enabled:  true,
		}
	}

	// Create mail server
	server, err := mailserver.NewMailServerWithConfig(*smtpPort, *smtpHost, *mailDir, outgoingConfig, authConfig, tlsConfig)
	if err != nil {
		if fatalErr := common.Fatal("Failed to create mail server: %v", err); fatalErr != nil {
			// In test environments, this will return an error instead of exiting
			return
		}
	}

	// Register event handlers
	server.On("new", func(email *mailserver.Email) {
		fromAddr := "unknown"
		if len(email.From) > 0 {
			fromAddr = email.From[0].Address
		}
		common.Log("New email received: %s (from: %s)", email.Subject, fromAddr)
		common.Verbose("Email details - ID: %s, Size: %s, Attachments: %d", email.ID, email.SizeHuman, len(email.Attachments))
	})

	server.On("delete", func(email *mailserver.Email) {
		common.Log("Email deleted: %s", email.Subject)
		common.Verbose("Deleted email ID: %s", email.ID)
	})

	// Create and start API server with HTTPS support
	apiServer := api.NewAPIWithHTTPS(server, *webPort, *webHost, *webUser, *webPassword, *httpsEnabled, *httpsCertFile, *httpsKeyFile)
	go func() {
		protocol := "http"
		if *httpsEnabled {
			protocol = "https"
		}
		common.Log("Starting OwlMail Web API on %s://%s:%d", protocol, *webHost, *webPort)
		if *webUser != "" && *webPassword != "" {
			common.Log("HTTP Basic Auth enabled for user: %s", *webUser)
		}
		if *httpsEnabled {
			common.Log("HTTPS enabled with certificate: %s", *httpsCertFile)
		}
		if err := apiServer.Start(); err != nil {
			if fatalErr := common.Fatal("Failed to start API server: %v", err); fatalErr != nil {
				// In test environments, this will return an error instead of exiting
				return
			}
		}
	}()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		common.Log("Shutting down mail server...")
		common.Verbose("Received shutdown signal, closing connections...")
		if err := server.Close(); err != nil {
			common.Error("Error closing server: %v", err)
		}
		os.Exit(0)
	}()

	// Start SMTP server
	common.Log("Starting OwlMail SMTP Server on %s:%d", *smtpHost, *smtpPort)
	common.Verbose("SMTP server configuration - Host: %s, Port: %d, MailDir: %s", *smtpHost, *smtpPort, *mailDir)
	if *tlsEnabled {
		common.Log("TLS enabled for SMTP server")
		common.Verbose("TLS certificate: %s, Key: %s", *tlsCertFile, *tlsKeyFile)
	}
	if err := server.Listen(); err != nil {
		if fatalErr := common.Fatal("Failed to start server: %v", err); fatalErr != nil {
			// In test environments, this will return an error instead of exiting
			return
		}
	}
}

// AutoRelayRule represents a single rule in the JSON file
type AutoRelayRule struct {
	Allow string `json:"allow,omitempty"`
	Deny  string `json:"deny,omitempty"`
}

// loadAutoRelayRules loads auto relay rules from a JSON file
// The JSON file format matches MailDev's format:
// [
//
//	{ "allow": "*" },
//	{ "deny": "*@test.com" },
//	{ "allow": "ok@test.com" }
//
// ]
func loadAutoRelayRules(filePath string) ([]string, []string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read rules file: %w", err)
	}

	var rules []AutoRelayRule
	if err := json.Unmarshal(data, &rules); err != nil {
		return nil, nil, fmt.Errorf("failed to parse rules JSON: %w", err)
	}

	var allowRules []string
	var denyRules []string

	// Process rules in order (last matching rule wins, like MailDev)
	for _, rule := range rules {
		if rule.Allow != "" {
			allowRules = append(allowRules, rule.Allow)
		}
		if rule.Deny != "" {
			denyRules = append(denyRules, rule.Deny)
		}
	}

	return allowRules, denyRules, nil
}
