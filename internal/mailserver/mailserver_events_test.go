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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	eventFired := make(chan bool, 1)
	server.On("new", func(email *Email) {
		eventFired <- true
	})

	// Emit event
	email := &Email{ID: "test-id", Subject: "Test"}
	server.emit("new", email)

	// Wait for event handler to be called
	select {
	case <-eventFired:
		// Event handler was called
	case <-time.After(1 * time.Second):
		t.Error("Event handler should have been called")
	}
}
