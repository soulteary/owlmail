package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/soulteary/owlmail/internal/api"
	"github.com/soulteary/owlmail/internal/attachmentstore"
	"github.com/soulteary/owlmail/internal/common"
	"github.com/soulteary/owlmail/internal/config"
	"github.com/soulteary/owlmail/internal/mailserver"
	"github.com/soulteary/owlmail/internal/mcpserver"
	"github.com/soulteary/owlmail/internal/outgoing"
	webhooknotify "github.com/soulteary/owlmail/internal/webhook"
)

const generatedWebPasswordBytes = 24

const (
	defaultAttachmentMigrationRetries    = 3
	defaultAttachmentMigrationTimeout    = 5 * time.Minute
	defaultAttachmentMigrationRetryDelay = time.Second
)

type webAuthCompletion struct {
	generatedPassword bool
	defaultedUsername bool
}

func completeWebAuthConfig(cfg *config.Config, randomSource io.Reader) (webAuthCompletion, error) {
	if cfg == nil {
		return webAuthCompletion{}, fmt.Errorf("config is nil")
	}

	switch {
	case cfg.WebUser != "" && cfg.WebPassword == "":
		if randomSource == nil {
			return webAuthCompletion{}, fmt.Errorf("generate HTTP Basic Auth password: random source is nil")
		}
		passwordBytes := make([]byte, generatedWebPasswordBytes)
		if _, err := io.ReadFull(randomSource, passwordBytes); err != nil {
			return webAuthCompletion{}, fmt.Errorf("generate HTTP Basic Auth password: %w", err)
		}
		cfg.WebPassword = base64.RawURLEncoding.EncodeToString(passwordBytes)
		return webAuthCompletion{generatedPassword: true}, nil
	case cfg.WebUser == "" && cfg.WebPassword != "":
		cfg.WebUser = "admin"
		return webAuthCompletion{defaultedUsername: true}, nil
	default:
		return webAuthCompletion{}, nil
	}
}

func reportWebAuthCompletion(cfg *config.Config, completion webAuthCompletion, output io.Writer) error {
	if completion.generatedPassword {
		if output == nil {
			return fmt.Errorf("print generated HTTP Basic Auth password: output is nil")
		}
		if _, err := fmt.Fprintf(output, "OwlMail generated a temporary HTTP Basic Auth password for user %q: %s (set -web-password or OWLMAIL_WEB_PASSWORD for a stable password)\n", cfg.WebUser, cfg.WebPassword); err != nil {
			return fmt.Errorf("print generated HTTP Basic Auth password: %w", err)
		}
	}
	if completion.defaultedUsername && output != nil {
		_, _ = fmt.Fprintln(output, "OwlMail defaulted the HTTP Basic Auth username to \"admin\" because only a password was configured")
	}
	return nil
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

// setupAuthConfig creates required SMTP authentication configuration. When
// both credentials are omitted OwlMail uses NO AUTH mode.
func setupAuthConfig(cfg *config.Config) (*mailserver.SMTPAuthConfig, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if (cfg.SMTPUser == "") != (cfg.SMTPPassword == "") {
		return nil, fmt.Errorf("SMTP username and password must be configured together")
	}
	if cfg.SMTPUser == "" {
		return nil, nil
	}
	return &mailserver.SMTPAuthConfig{
		Username: cfg.SMTPUser,
		Password: cfg.SMTPPassword,
		Enabled:  true,
	}, nil
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

func setupAttachmentStore(cfg *config.Config) (attachmentstore.Store, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if !cfg.S3Enabled {
		return nil, nil
	}
	store, err := attachmentstore.NewS3(context.Background(), attachmentstore.S3Config{
		Endpoint:        cfg.S3Endpoint,
		Region:          cfg.S3Region,
		Bucket:          cfg.S3Bucket,
		Prefix:          cfg.S3Prefix,
		AccessKeyID:     cfg.S3AccessKeyID,
		SecretAccessKey: cfg.S3SecretAccessKey,
		SessionToken:    cfg.S3SessionToken,
		UsePathStyle:    cfg.S3UsePathStyle,
	})
	if err != nil {
		return nil, fmt.Errorf("configure S3 attachment storage: %w", err)
	}
	return store, nil
}

func setupAttachmentHealth(store attachmentstore.Store, cfg *config.Config) (*attachmentstore.HealthMonitor, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if store == nil {
		return nil, nil
	}
	interval, err := time.ParseDuration(cfg.S3HealthInterval)
	if err != nil || interval <= 0 {
		return nil, fmt.Errorf("invalid S3 health check interval")
	}
	timeout, err := time.ParseDuration(cfg.S3HealthTimeout)
	if err != nil || timeout <= 0 {
		return nil, fmt.Errorf("invalid S3 health check timeout")
	}
	monitor, err := attachmentstore.NewHealthMonitor(store, interval, timeout)
	if err != nil {
		return nil, err
	}
	if cfg.S3StartupCheck {
		status := monitor.ProbeNow(context.Background())
		if !status.Ready() {
			_ = monitor.Close()
			return nil, fmt.Errorf("S3 startup check failed: %s", status.ErrorCategory)
		}
	}
	monitor.Start(context.Background())
	return monitor, nil
}

func runAttachmentMigration(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("migrate-attachments", flag.ContinueOnError)
	fs.SetOutput(stderr)
	refs := config.DefineFlags(fs)
	dryRun := fs.Bool("dry-run", false, "Validate and report migration work without remote or local writes")
	deleteLocal := fs.Bool("delete-local", false, "Delete each local attachment only after verified upload and metadata commit")
	retries := fs.Int("retries", defaultAttachmentMigrationRetries, "Retry count from 0 to 100 after the initial attempt for each attachment")
	attemptTimeout := fs.Duration("migration-attempt-timeout", defaultAttachmentMigrationTimeout, "Timeout for each upload and verification attempt")
	retryDelay := fs.Duration("migration-retry-delay", defaultAttachmentMigrationRetryDelay, "Delay between migration attempts")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected migration arguments: %v", fs.Args())
	}
	cfg := config.ResolveConfig(fs, refs)
	if !cfg.S3Enabled {
		return fmt.Errorf("attachment migration requires -s3-enabled or OWLMAIL_S3_ENABLED=true")
	}
	if cfg.MailDir == "" {
		return fmt.Errorf("attachment migration requires -mail-directory or OWLMAIL_MAIL_DIR")
	}
	store, err := setupAttachmentStore(cfg)
	if err != nil {
		return err
	}
	progressWriter := stdout
	if progressWriter == nil {
		progressWriter = io.Discard
	}
	summary, migrationErr := mailserver.MigrateLocalAttachments(ctx, cfg.MailDir, store, mailserver.AttachmentMigrationOptions{
		DryRun:         *dryRun,
		DeleteLocal:    *deleteLocal,
		Retries:        *retries,
		AttemptTimeout: *attemptTimeout,
		RetryDelay:     *retryDelay,
		Progress: func(progress mailserver.AttachmentMigrationProgress) {
			if progress.Status == "retrying" {
				_, _ = fmt.Fprintf(progressWriter, "%s %s/%s attempt=%d error=%v\n", progress.Status, progress.EmailID, progress.Filename, progress.Attempt, progress.Err)
				return
			}
			_, _ = fmt.Fprintf(progressWriter, "%s %s/%s\n", progress.Status, progress.EmailID, progress.Filename)
		},
	})
	encodedSummary, encodeErr := json.Marshal(summary)
	if encodeErr == nil {
		_, _ = fmt.Fprintf(progressWriter, "summary %s\n", encodedSummary)
	}
	return errors.Join(migrationErr, encodeErr)
}

func setupStoragePolicy(cfg *config.Config) (mailserver.StoragePolicy, error) {
	if cfg == nil {
		return mailserver.StoragePolicy{}, fmt.Errorf("config is nil")
	}
	interval := cfg.MailCleanupInterval
	if interval == "" {
		interval = config.DefaultMailCleanupInterval
	}
	cleanupInterval, err := time.ParseDuration(interval)
	if err != nil || cleanupInterval <= 0 {
		return mailserver.StoragePolicy{}, fmt.Errorf("invalid mail cleanup interval %q", interval)
	}
	return mailserver.StoragePolicy{
		MaxAge:          time.Duration(cfg.MailRetentionDays) * 24 * time.Hour,
		MaxMessages:     cfg.MailMaxMessages,
		MaxDiskBytes:    int64(cfg.MailMaxDiskMB) * 1024 * 1024,
		CleanupInterval: cleanupInterval,
	}, nil
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

func registerWebhookHandler(server *mailserver.MailServer, dispatcher *webhooknotify.Dispatcher, maxConcurrency int) error {
	if server == nil || dispatcher == nil {
		return nil
	}
	service, err := webhooknotify.NewService(dispatcher, webhooknotify.ServiceOptions{
		MaxConcurrency: maxConcurrency,
		SpoolDir:       server.GetMailDir(),
		OnResults:      logWebhookResults,
	})
	if err != nil {
		return fmt.Errorf("create webhook service: %w", err)
	}
	if err := registerWebhookService(server, service); err != nil {
		_ = service.Close()
		return err
	}
	if err := server.AddCloser(service); err != nil {
		_ = service.Close()
		return fmt.Errorf("register webhook closer: %w", err)
	}
	if maxConcurrency == 0 {
		common.Log("Webhook forwarding enabled with %d target(s), unlimited concurrency", dispatcher.TargetCount())
	} else {
		common.Log("Webhook forwarding enabled with %d target(s), concurrency limit %d", dispatcher.TargetCount(), maxConcurrency)
	}
	return nil
}

func setupWebhookService(cfg *config.Config, dispatcher *webhooknotify.Dispatcher, spoolDir string) (*webhooknotify.Service, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if dispatcher == nil {
		return nil, nil
	}
	shutdownTimeout, err := time.ParseDuration(cfg.WebhookShutdownTimeout)
	if err != nil || shutdownTimeout <= 0 {
		return nil, fmt.Errorf("invalid webhook shutdown timeout %q", cfg.WebhookShutdownTimeout)
	}
	service, err := webhooknotify.NewService(dispatcher, webhooknotify.ServiceOptions{
		RedisURL: cfg.WebhookRedisURL, RedisPrefix: cfg.WebhookRedisPrefix,
		MaxConcurrency: cfg.WebhookMaxConcurrency, ShutdownTimeout: shutdownTimeout,
		SpoolDir:  spoolDir,
		OnResults: logWebhookResults,
	})
	if err != nil {
		return nil, fmt.Errorf("create webhook delivery service: %w", err)
	}
	return service, nil
}

type webhookHandoffService interface {
	Enqueue(*mailserver.Email) error
	Commit(string) error
	Abort(string) error
	RecoverAcceptedPending() error
}

func registerWebhookService(server *mailserver.MailServer, service webhookHandoffService) error {
	if server == nil || service == nil {
		return nil
	}
	if err := server.OnSynchronous("new", func(email *mailserver.Email) error {
		return service.Enqueue(email)
	}); err != nil {
		return fmt.Errorf("register webhook queue handoff: %w", err)
	}
	server.On("new", func(email *mailserver.Email) {
		if err := service.Commit(email.ID); err != nil {
			common.Error("Failed to commit webhook queue handoff: %v", err)
		}
	})
	server.On("new-rollback", func(email *mailserver.Email) {
		if email == nil {
			return
		}
		if err := service.Abort(email.ID); err != nil {
			common.Error("Failed to discard rejected webhook queue handoff: %v", err)
		}
	})
	if err := service.RecoverAcceptedPending(); err != nil {
		return fmt.Errorf("recover accepted webhook queue handoffs: %w", err)
	}
	for _, email := range server.GetAllEmail() {
		if err := service.Commit(email.ID); err != nil {
			return fmt.Errorf("recover webhook queue handoff for %s: %w", email.ID, err)
		}
	}
	return nil
}

func logWebhookResults(deliveryID string, results []webhooknotify.Result) {
	for _, result := range results {
		if result.Err != nil {
			common.Error("Webhook delivery %q to %q failed after %d attempt(s): %v", deliveryID, result.Target, result.Attempts, result.Err)
			continue
		}
		common.Verbose("Webhook delivery %q to %q succeeded with HTTP %d", deliveryID, result.Target, result.StatusCode)
	}
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
	if err := apiServer.SetBasePathname(cfg.BasePathname); err != nil {
		return nil, err
	}
	if err := apiServer.SetExternalScheme(cfg.WebExternalScheme); err != nil {
		return nil, err
	}
	if cfg.MCPEnabled {
		sessionTimeout, err := time.ParseDuration(cfg.MCPSessionTimeout)
		if err != nil || sessionTimeout <= 0 {
			return nil, fmt.Errorf("invalid MCP session timeout %q", cfg.MCPSessionTimeout)
		}
		shutdownTimeout, err := time.ParseDuration(cfg.MCPShutdownTimeout)
		if err != nil || shutdownTimeout <= 0 {
			return nil, fmt.Errorf("invalid MCP shutdown timeout %q", cfg.MCPShutdownTimeout)
		}
		mcpService, err := mcpserver.New(server, mcpserver.Options{
			SessionTimeout: sessionTimeout, ShutdownTimeout: shutdownTimeout,
		})
		if err != nil {
			return nil, fmt.Errorf("create MCP service: %w", err)
		}
		if err := apiServer.SetMCPHandler(mcpService); err != nil {
			_ = mcpService.Close()
			return nil, err
		}
		if err := server.AddCloser(mcpService); err != nil {
			_ = mcpService.Close()
			return nil, fmt.Errorf("register MCP service closer: %w", err)
		}
	}

	protocol := "http"
	if cfg.HTTPSEnabled {
		protocol = "https"
	}
	if cfg.MCPEnabled {
		common.Log("Read-only MCP enabled at %s://%s:%d%s/mcp (idle timeout: %s)", protocol, cfg.WebHost, cfg.WebPort, cfg.BasePathname, cfg.MCPSessionTimeout)
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
	basePathname, err := config.NormalizeBasePathname(cfg.BasePathname)
	if err != nil {
		return err
	}
	cfg.BasePathname = basePathname
	completion, err := completeWebAuthConfig(cfg, rand.Reader)
	if err != nil {
		return err
	}
	return reportWebAuthCompletion(cfg, completion, os.Stderr)
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
	authConfig, err := setupAuthConfig(cfg)
	if err != nil {
		return nil, err
	}

	// Setup TLS config
	tlsConfig := setupTLSConfig(cfg)
	if cfg.SMTPAuthRequireTLS && tlsConfig == nil {
		return nil, fmt.Errorf("SMTP AUTH cannot require TLS without an enabled TLS configuration")
	}
	attachmentStore, err := setupAttachmentStore(cfg)
	if err != nil {
		return nil, err
	}
	attachmentHealth, err := setupAttachmentHealth(attachmentStore, cfg)
	if err != nil {
		return nil, err
	}
	healthOwned := attachmentHealth != nil
	defer func() {
		if healthOwned {
			_ = attachmentHealth.Close()
		}
	}()
	var healthProvider attachmentstore.ReadinessProvider
	if attachmentHealth != nil {
		healthProvider = attachmentHealth
	}
	maxMessageMB := cfg.SMTPMaxMessageMB
	if maxMessageMB == 0 {
		maxMessageMB = config.DefaultSMTPMaxMessageMB
	}
	if maxMessageMB < 0 {
		return nil, fmt.Errorf("SMTP max message size must be greater than zero")
	}
	const maxMessageMBWithoutOverflow = int64(^uint64(0)>>1) >> 20
	if int64(maxMessageMB) > maxMessageMBWithoutOverflow {
		return nil, fmt.Errorf("SMTP max message size is too large")
	}

	// Create mail server
	server, err := mailserver.NewMailServerWithOptions(cfg.SMTPPort, cfg.SMTPHost, cfg.MailDir, mailserver.ServerOptions{
		OutgoingConfig:   outgoingConfig,
		AuthConfig:       authConfig,
		AuthRequireTLS:   cfg.SMTPAuthRequireTLS,
		TLSConfig:        tlsConfig,
		UseUUIDForID:     cfg.UseUUIDForEmailID,
		MaxMessageBytes:  int64(maxMessageMB) << 20,
		AttachmentStore:  attachmentStore,
		AttachmentHealth: healthProvider,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create mail server: %w", err)
	}
	if attachmentHealth != nil {
		if err := server.AddCloser(attachmentHealth); err != nil {
			_ = server.Close()
			return nil, fmt.Errorf("register attachment health monitor: %w", err)
		}
	}
	healthOwned = false
	storagePolicy, err := setupStoragePolicy(cfg)
	if err != nil {
		_ = server.Close()
		return nil, err
	}
	if err := server.ConfigureStoragePolicy(storagePolicy); err != nil {
		_ = server.Close()
		return nil, fmt.Errorf("configure storage policy: %w", err)
	}
	if attachmentStore != nil {
		common.Log("S3 attachment storage enabled for bucket %s with prefix %s (startup check: %t)", cfg.S3Bucket, cfg.S3Prefix, cfg.S3StartupCheck)
	}

	// Register event handlers
	registerEventHandlers(server)
	webhookService, err := setupWebhookService(cfg, webhookDispatcher, server.GetMailDir())
	if err != nil {
		_ = server.Close()
		return nil, err
	}
	if webhookService != nil {
		if err := registerWebhookService(server, webhookService); err != nil {
			_ = webhookService.Close()
			_ = server.Close()
			return nil, err
		}
		if err := server.AddCloser(webhookService); err != nil {
			_ = webhookService.Close()
			_ = server.Close()
			return nil, err
		}
		mode := "in-memory"
		if cfg.WebhookRedisURL != "" {
			mode = "Redis durable"
		}
		common.Log("Webhook forwarding enabled with %d target(s), %s queue, concurrency %d", webhookDispatcher.TargetCount(), mode, cfg.WebhookMaxConcurrency)
	}

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
	if len(os.Args) > 1 && os.Args[1] == "migrate-attachments" {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		err := runAttachmentMigration(ctx, os.Args[2:], os.Stdout, os.Stderr)
		stop()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Attachment migration failed: %v\n", err)
			os.Exit(1)
		}
		return
	}
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
