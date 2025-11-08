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
		port    = flag.Int("smtp", 1025, "SMTP port to catch emails")
		host    = flag.String("ip", "localhost", "IP address to bind SMTP service to")
		mailDir = flag.String("mail-directory", "", "Directory for persisting mails")
	)
	flag.Parse()

	// Create mail server
	server, err := NewMailServer(*port, *host, *mailDir)
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

	// Start server
	log.Printf("Starting OwlMail SMTP Server on %s:%d", *host, *port)
	if err := server.Listen(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
