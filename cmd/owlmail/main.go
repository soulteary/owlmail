package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/soulteary/owlmail/internal/api"
	"github.com/soulteary/owlmail/internal/common"
	"github.com/soulteary/owlmail/internal/config"
	"github.com/soulteary/owlmail/internal/mailserver"
	"github.com/soulteary/owlmail/internal/outgoing"
	webhooknotify "github.com/soulteary/owlmail/internal/webhook"
)

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
func setupOutgoingConfig(cfg *config.Config) (*outgoing.OutgoingConfig, error) {
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
func setupAuthConfig(cfg *config.Config) *mailserver.SMTPAuthConfig {
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
func setupTLSConfig(cfg *config.Config) *mailserver.TLSConfig {
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

func setupWebhookDispatcher(cfg *config.Config) (*webhooknotify.Dispatcher, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if cfg.WebhookConfig == "" {
		return nil, nil
	}

	dispatcher, err := webhooknotify.Load(cfg.WebhookConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to load webhook forwarding config: %w", err)
	}
	return dispatcher, nil
}

func registerWebhookHandler(server *mailserver.MailServer, dispatcher *webhooknotify.Dispatcher) {
	if server == nil || dispatcher == nil {
		return
	}

	server.On("new", func(email *mailserver.Email) {
		for _, result := range dispatcher.Dispatch(context.Background(), email) {
			if result.Err != nil {
				common.Error("Webhook delivery to %q failed after %d attempt(s): %v", result.Target, result.Attempts, result.Err)
				continue
			}
			common.Verbose("Webhook delivery to %q succeeded with HTTP %d", result.Target, result.StatusCode)
		}
	})
	common.Log("Webhook forwarding enabled with %d target(s)", dispatcher.TargetCount())
}

// startAPIServer creates and starts the API server
func startAPIServer(server *mailserver.MailServer, cfg *config.Config) (*api.API, error) {
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
func initializeApplication(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	level := parseLogLevel(cfg.LogLevel)
	common.InitLogger(level)
	return nil
}

// createMailServer creates and configures the mail server
func createMailServer(cfg *config.Config) (*mailserver.MailServer, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	// Validate and compile webhook configuration before starting server resources.
	webhookDispatcher, err := setupWebhookDispatcher(cfg)
	if err != nil {
		return nil, err
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
	registerWebhookHandler(server, webhookDispatcher)

	return server, nil
}

// startServers starts all servers (API and SMTP)
func startServers(server *mailserver.MailServer, cfg *config.Config) error {
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
	// Parse configuration using the config package
	cfg := config.ParseFlags()

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
