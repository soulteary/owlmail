package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	var (
		smtpPort       = flag.Int("smtp", 1025, "SMTP port to catch emails")
		smtpHost       = flag.String("ip", "localhost", "IP address to bind SMTP service to")
		webPort        = flag.Int("web", 1080, "Web API port")
		webHost        = flag.String("web-ip", "localhost", "IP address to bind Web API to")
		mailDir        = flag.String("mail-directory", "", "Directory for persisting mails")
		outgoingHost   = flag.String("outgoing-host", "", "Outgoing SMTP server host")
		outgoingPort   = flag.Int("outgoing-port", 587, "Outgoing SMTP server port")
		outgoingUser   = flag.String("outgoing-user", "", "Outgoing SMTP server username")
		outgoingPass   = flag.String("outgoing-pass", "", "Outgoing SMTP server password")
		outgoingSecure = flag.Bool("outgoing-secure", false, "Use TLS for outgoing SMTP")
		autoRelay      = flag.Bool("auto-relay", false, "Automatically relay all emails")
		autoRelayAddr  = flag.String("auto-relay-addr", "", "Auto relay to specific address")
		webUser        = flag.String("web-user", "", "HTTP Basic Auth username")
		webPassword    = flag.String("web-password", "", "HTTP Basic Auth password")
	)
	flag.Parse()

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

	// Create mail server
	server, err := NewMailServerWithOutgoing(*smtpPort, *smtpHost, *mailDir, outgoingConfig)
	if err != nil {
		log.Fatalf("Failed to create mail server: %v", err)
	}

	// Register event handlers
	server.On("new", func(email *Email) {
		fromAddr := "unknown"
		if len(email.From) > 0 {
			fromAddr = email.From[0].Address
		}
		log.Printf("New email received: %s (from: %s)", email.Subject, fromAddr)
	})

	server.On("delete", func(email *Email) {
		log.Printf("Email deleted: %s", email.Subject)
	})

	// Create and start API server
	api := NewAPIWithAuth(server, *webPort, *webHost, *webUser, *webPassword)
	go func() {
		log.Printf("Starting OwlMail Web API on %s:%d", *webHost, *webPort)
		if *webUser != "" && *webPassword != "" {
			log.Printf("HTTP Basic Auth enabled for user: %s", *webUser)
		}
		if err := api.Start(); err != nil {
			log.Fatalf("Failed to start API server: %v", err)
		}
	}()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down mail server...")
		if err := server.Close(); err != nil {
			log.Printf("Error closing server: %v", err)
		}
		os.Exit(0)
	}()

	// Start SMTP server
	log.Printf("Starting OwlMail SMTP Server on %s:%d", *smtpHost, *smtpPort)
	if err := server.Listen(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
