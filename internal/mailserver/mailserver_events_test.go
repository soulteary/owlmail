package mailserver

import (
	"testing"
	"time"
)

func TestMailServerOn(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer server.Close()

	eventFired := false
	server.On("new", func(email *Email) {
		eventFired = true
	})

	// Emit event
	email := &Email{ID: "test-id", Subject: "Test"}
	server.emit("new", email)

	// Give time for goroutine to execute
	time.Sleep(50 * time.Millisecond)

	if !eventFired {
		t.Error("Event handler should have been called")
	}
}
