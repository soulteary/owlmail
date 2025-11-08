package main

import (
	"flag"
	"os"
	"os/signal"
	"strconv"
	"syscall"
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
func getLogLevelFromEnv() LogLevel {
	levelStr := getEnvString("OWLMAIL_LOG_LEVEL", "normal")
	switch levelStr {
	case "silent":
		return LogLevelSilent
	case "verbose":
		return LogLevelVerbose
	default:
		return LogLevelNormal
	}
}

func main() {
	var (
		// SMTP server configuration
		smtpPort = flag.Int("smtp", getEnvInt("OWLMAIL_SMTP_PORT", 1025), "SMTP port to catch emails")
		smtpHost = flag.String("ip", getEnvString("OWLMAIL_SMTP_HOST", "localhost"), "IP address to bind SMTP service to")
		mailDir  = flag.String("mail-directory", getEnvString("OWLMAIL_MAIL_DIR", ""), "Directory for persisting mails")

		// Web API configuration
		webPort     = flag.Int("web", getEnvInt("OWLMAIL_WEB_PORT", 1080), "Web API port")
		webHost     = flag.String("web-ip", getEnvString("OWLMAIL_WEB_HOST", "localhost"), "IP address to bind Web API to")
		webUser     = flag.String("web-user", getEnvString("OWLMAIL_WEB_USER", ""), "HTTP Basic Auth username")
		webPassword = flag.String("web-password", getEnvString("OWLMAIL_WEB_PASSWORD", ""), "HTTP Basic Auth password")

		// HTTPS configuration
		httpsEnabled  = flag.Bool("https", getEnvBool("OWLMAIL_HTTPS_ENABLED", false), "Enable HTTPS for Web API")
		httpsCertFile = flag.String("https-cert", getEnvString("OWLMAIL_HTTPS_CERT", ""), "HTTPS certificate file path")
		httpsKeyFile  = flag.String("https-key", getEnvString("OWLMAIL_HTTPS_KEY", ""), "HTTPS private key file path")

		// Outgoing mail configuration
		outgoingHost   = flag.String("outgoing-host", getEnvString("OWLMAIL_OUTGOING_HOST", ""), "Outgoing SMTP server host")
		outgoingPort   = flag.Int("outgoing-port", getEnvInt("OWLMAIL_OUTGOING_PORT", 587), "Outgoing SMTP server port")
		outgoingUser   = flag.String("outgoing-user", getEnvString("OWLMAIL_OUTGOING_USER", ""), "Outgoing SMTP server username")
		outgoingPass   = flag.String("outgoing-pass", getEnvString("OWLMAIL_OUTGOING_PASSWORD", ""), "Outgoing SMTP server password")
		outgoingSecure = flag.Bool("outgoing-secure", getEnvBool("OWLMAIL_OUTGOING_SECURE", false), "Use TLS for outgoing SMTP")
		autoRelay      = flag.Bool("auto-relay", getEnvBool("OWLMAIL_AUTO_RELAY", false), "Automatically relay all emails")
		autoRelayAddr  = flag.String("auto-relay-addr", getEnvString("OWLMAIL_AUTO_RELAY_ADDR", ""), "Auto relay to specific address")

		// SMTP authentication
		smtpUser     = flag.String("smtp-user", getEnvString("OWLMAIL_SMTP_USER", ""), "SMTP server username for authentication")
		smtpPassword = flag.String("smtp-password", getEnvString("OWLMAIL_SMTP_PASSWORD", ""), "SMTP server password for authentication")

		// TLS configuration for SMTP
		tlsEnabled  = flag.Bool("tls", getEnvBool("OWLMAIL_TLS_ENABLED", false), "Enable TLS/STARTTLS for SMTP server")
		tlsCertFile = flag.String("tls-cert", getEnvString("OWLMAIL_TLS_CERT", ""), "TLS certificate file path")
		tlsKeyFile  = flag.String("tls-key", getEnvString("OWLMAIL_TLS_KEY", ""), "TLS private key file path")

		// Logging configuration
		logLevel = flag.String("log-level", getEnvString("OWLMAIL_LOG_LEVEL", "normal"), "Log level: silent, normal, or verbose")
	)
	flag.Parse()

	// Initialize logger based on log level
	var level LogLevel
	switch *logLevel {
	case "silent":
		level = LogLevelSilent
	case "verbose":
		level = LogLevelVerbose
	default:
		level = LogLevelNormal
	}
	InitLogger(level)

	// Setup outgoing mail config if provided
	var outgoingConfig *OutgoingConfig
	if *outgoingHost != "" {
		outgoingConfig = &OutgoingConfig{
			Host:          *outgoingHost,
			Port:          *outgoingPort,
			User:          *outgoingUser,
			Password:      *outgoingPass,
			Secure:        *outgoingSecure,
			AutoRelay:     *autoRelay,
			AutoRelayAddr: *autoRelayAddr,
		}
	}

	// Setup SMTP authentication config
	var authConfig *SMTPAuthConfig
	if *smtpUser != "" && *smtpPassword != "" {
		authConfig = &SMTPAuthConfig{
			Username: *smtpUser,
			Password: *smtpPassword,
			Enabled:  true,
		}
	}

	// Setup TLS config
	var tlsConfig *TLSConfig
	if *tlsEnabled {
		tlsConfig = &TLSConfig{
			CertFile: *tlsCertFile,
			KeyFile:  *tlsKeyFile,
			Enabled:  true,
		}
	}

	// Create mail server
	server, err := NewMailServerWithConfig(*smtpPort, *smtpHost, *mailDir, outgoingConfig, authConfig, tlsConfig)
	if err != nil {
		Fatal("Failed to create mail server: %v", err)
	}

	// Register event handlers
	server.On("new", func(email *Email) {
		fromAddr := "unknown"
		if len(email.From) > 0 {
			fromAddr = email.From[0].Address
		}
		Log("New email received: %s (from: %s)", email.Subject, fromAddr)
		Verbose("Email details - ID: %s, Size: %s, Attachments: %d", email.ID, email.SizeHuman, len(email.Attachments))
	})

	server.On("delete", func(email *Email) {
		Log("Email deleted: %s", email.Subject)
		Verbose("Deleted email ID: %s", email.ID)
	})

	// Create and start API server with HTTPS support
	api := NewAPIWithHTTPS(server, *webPort, *webHost, *webUser, *webPassword, *httpsEnabled, *httpsCertFile, *httpsKeyFile)
	go func() {
		protocol := "http"
		if *httpsEnabled {
			protocol = "https"
		}
		Log("Starting OwlMail Web API on %s://%s:%d", protocol, *webHost, *webPort)
		if *webUser != "" && *webPassword != "" {
			Log("HTTP Basic Auth enabled for user: %s", *webUser)
		}
		if *httpsEnabled {
			Log("HTTPS enabled with certificate: %s", *httpsCertFile)
		}
		if err := api.Start(); err != nil {
			Fatal("Failed to start API server: %v", err)
		}
	}()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		Log("Shutting down mail server...")
		Verbose("Received shutdown signal, closing connections...")
		if err := server.Close(); err != nil {
			Error("Error closing server: %v", err)
		}
		os.Exit(0)
	}()

	// Start SMTP server
	Log("Starting OwlMail SMTP Server on %s:%d", *smtpHost, *smtpPort)
	Verbose("SMTP server configuration - Host: %s, Port: %d, MailDir: %s", *smtpHost, *smtpPort, *mailDir)
	if *tlsEnabled {
		Log("TLS enabled for SMTP server")
		Verbose("TLS certificate: %s, Key: %s", *tlsCertFile, *tlsKeyFile)
	}
	if err := server.Listen(); err != nil {
		Fatal("Failed to start server: %v", err)
	}
}
