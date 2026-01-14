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
}

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

// parseConfig parses command line flags and returns a Config struct
func parseConfig() *Config {
	var (
		// SMTP server configuration
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
		logLevel = flag.String("log-level", maildev.GetMailDevLogLevel("normal"), "Log level: silent, normal, or verbose")

		// Email ID configuration
		useUUIDForEmailID = flag.Bool("use-uuid-for-email-id", maildev.GetMailDevEnvBool("OWLMAIL_USE_UUID_FOR_EMAIL_ID", false), "Use UUID instead of random string for email IDs")
	)
	flag.Parse()

	return &Config{
		SMTPPort:          *smtpPort,
		SMTPHost:          *smtpHost,
		MailDir:           *mailDir,
		WebPort:           *webPort,
		WebHost:           *webHost,
		WebUser:           *webUser,
		WebPassword:       *webPassword,
		HTTPSEnabled:      *httpsEnabled,
		HTTPSCertFile:     *httpsCertFile,
		HTTPSKeyFile:      *httpsKeyFile,
		OutgoingHost:      *outgoingHost,
		OutgoingPort:      *outgoingPort,
		OutgoingUser:      *outgoingUser,
		OutgoingPass:      *outgoingPass,
		OutgoingSecure:    *outgoingSecure,
		AutoRelay:         *autoRelay,
		AutoRelayAddr:     *autoRelayAddr,
		AutoRelayRules:    *autoRelayRules,
		SMTPUser:          *smtpUser,
		SMTPPassword:      *smtpPassword,
		TLSEnabled:        *tlsEnabled,
		TLSCertFile:       *tlsCertFile,
		TLSKeyFile:        *tlsKeyFile,
		LogLevel:          *logLevel,
		UseUUIDForEmailID: *useUUIDForEmailID,
	}
}

// parseLogLevel parses log level string and returns LogLevel
func parseLogLevel(levelStr string) common.LogLevel {
	switch levelStr {
	case "silent":
		return common.LogLevelSilent
	case "verbose":
		return common.LogLevelVerbose
	default:
		return common.LogLevelNormal
	}
}

// setupOutgoingConfig creates outgoing mail configuration from config
func setupOutgoingConfig(cfg *Config) (*outgoing.OutgoingConfig, error) {
	if cfg.OutgoingHost == "" {
		return nil, nil
	}

	outgoingConfig := &outgoing.OutgoingConfig{
		Host:          cfg.OutgoingHost,
		Port:          cfg.OutgoingPort,
		User:          cfg.OutgoingUser,
		Password:      cfg.OutgoingPass,
		Secure:        cfg.OutgoingSecure,
		AutoRelay:     cfg.AutoRelay,
		AutoRelayAddr: cfg.AutoRelayAddr,
	}

	// Load auto relay rules from JSON file if provided
	if cfg.AutoRelayRules != "" {
		allowRules, denyRules, err := loadAutoRelayRules(cfg.AutoRelayRules)
		if err != nil {
			return nil, fmt.Errorf("failed to load auto relay rules: %w", err)
		}
		outgoingConfig.AllowRules = allowRules
		outgoingConfig.DenyRules = denyRules
		if len(allowRules) > 0 || len(denyRules) > 0 {
			common.Log("Loaded auto relay rules: %d allow rules, %d deny rules", len(allowRules), len(denyRules))
		}
	}

	return outgoingConfig, nil
}

// setupAuthConfig creates SMTP authentication configuration from config
func setupAuthConfig(cfg *Config) *mailserver.SMTPAuthConfig {
	if cfg.SMTPUser == "" || cfg.SMTPPassword == "" {
		return nil
	}
	return &mailserver.SMTPAuthConfig{
		Username: cfg.SMTPUser,
		Password: cfg.SMTPPassword,
		Enabled:  true,
	}
}

// setupTLSConfig creates TLS configuration from config
func setupTLSConfig(cfg *Config) *mailserver.TLSConfig {
	if !cfg.TLSEnabled {
		return nil
	}
	return &mailserver.TLSConfig{
		CertFile: cfg.TLSCertFile,
		KeyFile:  cfg.TLSKeyFile,
		Enabled:  true,
	}
}

// registerEventHandlers registers event handlers for the mail server
func registerEventHandlers(server *mailserver.MailServer) {
	if server == nil {
		return
	}

	server.On("new", func(email *mailserver.Email) {
		if email == nil {
			common.Log("New email received: (nil email)")
			return
		}
		fromAddr := "unknown"
		if len(email.From) > 0 && email.From[0] != nil {
			fromAddr = email.From[0].Address
		}
		subject := email.Subject
		if subject == "" {
			subject = "(no subject)"
		}
		common.Log("New email received: %s (from: %s)", subject, fromAddr)
		common.Verbose("Email details - ID: %s, Size: %s, Attachments: %d", email.ID, email.SizeHuman, len(email.Attachments))
	})

	server.On("delete", func(email *mailserver.Email) {
		if email == nil {
			common.Log("Email deleted: (nil email)")
			return
		}
		subject := email.Subject
		if subject == "" {
			subject = "(no subject)"
		}
		common.Log("Email deleted: %s", subject)
		common.Verbose("Deleted email ID: %s", email.ID)
	})
}

// startAPIServer creates and starts the API server
func startAPIServer(server *mailserver.MailServer, cfg *Config) (*api.API, error) {
	if server == nil {
		return nil, fmt.Errorf("mail server is nil")
	}
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	apiServer := api.NewAPIWithHTTPS(server, cfg.WebPort, cfg.WebHost, cfg.WebUser, cfg.WebPassword, cfg.HTTPSEnabled, cfg.HTTPSCertFile, cfg.HTTPSKeyFile)

	protocol := "http"
	if cfg.HTTPSEnabled {
		protocol = "https"
	}
	common.Log("Starting OwlMail Web API on %s://%s:%d", protocol, cfg.WebHost, cfg.WebPort)
	if cfg.WebUser != "" && cfg.WebPassword != "" {
		common.Log("HTTP Basic Auth enabled for user: %s", cfg.WebUser)
	}
	if cfg.HTTPSEnabled {
		if cfg.HTTPSCertFile != "" {
			common.Log("HTTPS enabled with certificate: %s", cfg.HTTPSCertFile)
		} else {
			common.Log("HTTPS enabled (no certificate file specified)")
		}
	}

	if err := apiServer.Start(); err != nil {
		return nil, fmt.Errorf("failed to start API server: %w", err)
	}

	return apiServer, nil
}

// setupGracefulShutdown sets up signal handling for graceful shutdown
func setupGracefulShutdown(server *mailserver.MailServer) {
	if server == nil {
		return
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		common.Log("Shutting down mail server... (signal: %v)", sig)
		common.Verbose("Received shutdown signal, closing connections...")
		if err := server.Close(); err != nil {
			common.Error("Error closing server: %v", err)
		}
		os.Exit(0)
	}()
}

// initializeApplication initializes the application (logger, etc.)
func initializeApplication(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	level := parseLogLevel(cfg.LogLevel)
	common.InitLogger(level)
	return nil
}

// createMailServer creates and configures the mail server
func createMailServer(cfg *Config) (*mailserver.MailServer, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	// Setup outgoing mail config if provided
	outgoingConfig, err := setupOutgoingConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to setup outgoing config: %w", err)
	}

	// Setup SMTP authentication config
	authConfig := setupAuthConfig(cfg)

	// Setup TLS config
	tlsConfig := setupTLSConfig(cfg)

	// Create mail server
	server, err := mailserver.NewMailServerWithFullConfig(cfg.SMTPPort, cfg.SMTPHost, cfg.MailDir, outgoingConfig, authConfig, tlsConfig, cfg.UseUUIDForEmailID)
	if err != nil {
		return nil, fmt.Errorf("failed to create mail server: %w", err)
	}

	// Register event handlers
	registerEventHandlers(server)

	return server, nil
}

// startServers starts all servers (API and SMTP)
func startServers(server *mailserver.MailServer, cfg *Config) error {
	if server == nil {
		return fmt.Errorf("mail server is nil")
	}
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	// Create and start API server with HTTPS support
	go func() {
		if _, err := startAPIServer(server, cfg); err != nil {
			if fatalErr := common.Fatal("Failed to start API server: %v", err); fatalErr != nil {
				// In test environments, this will return an error instead of exiting
				return
			}
		}
	}()

	// Handle graceful shutdown
	setupGracefulShutdown(server)

	// Start SMTP server
	common.Log("Starting OwlMail SMTP Server on %s:%d", cfg.SMTPHost, cfg.SMTPPort)
	common.Verbose("SMTP server configuration - Host: %s, Port: %d, MailDir: %s", cfg.SMTPHost, cfg.SMTPPort, cfg.MailDir)
	if cfg.TLSEnabled {
		common.Log("TLS enabled for SMTP server")
		common.Verbose("TLS certificate: %s, Key: %s", cfg.TLSCertFile, cfg.TLSKeyFile)
	}
	if err := server.Listen(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}

func main() {
	// Parse configuration
	cfg := parseConfig()

	// Initialize application
	if err := initializeApplication(cfg); err != nil {
		if fatalErr := common.Fatal("Failed to initialize application: %v", err); fatalErr != nil {
			// In test environments, this will return an error instead of exiting
			return
		}
	}

	// Create mail server
	server, err := createMailServer(cfg)
	if err != nil {
		if fatalErr := common.Fatal("Failed to create mail server: %v", err); fatalErr != nil {
			// In test environments, this will return an error instead of exiting
			return
		}
	}

	// Start servers
	if err := startServers(server, cfg); err != nil {
		if fatalErr := common.Fatal("Failed to start servers: %v", err); fatalErr != nil {
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
