package types

import (
	"testing"
	"time"

	"github.com/emersion/go-message/mail"
)

func TestEmail(t *testing.T) {
	email := &Email{
		ID:        "test-id",
		Time:      time.Now(),
		Read:      false,
		Subject:   "Test Subject",
		From:      []*mail.Address{{Address: "from@example.com"}},
		To:        []*mail.Address{{Address: "to@example.com"}},
		Text:      "Test body",
		HTML:      "<p>Test body</p>",
		Size:      1024,
		SizeHuman: "1 KB",
	}

	if email.ID != "test-id" {
		t.Errorf("Expected ID 'test-id', got '%s'", email.ID)
	}
	if email.Subject != "Test Subject" {
		t.Errorf("Expected Subject 'Test Subject', got '%s'", email.Subject)
	}
	if email.Read {
		t.Error("Expected Read to be false")
	}
	if len(email.From) != 1 {
		t.Errorf("Expected 1 From address, got %d", len(email.From))
	}
	if len(email.To) != 1 {
		t.Errorf("Expected 1 To address, got %d", len(email.To))
	}
	if email.Size != 1024 {
		t.Errorf("Expected Size 1024, got %d", email.Size)
	}
}

func TestAttachment(t *testing.T) {
	attachment := &Attachment{
		ContentType:       "text/plain",
		FileName:          "test.txt",
		GeneratedFileName: "test-123.txt",
		ContentID:         "cid:123",
		Size:              512,
		Transformed:       false,
	}

	if attachment.ContentType != "text/plain" {
		t.Errorf("Expected ContentType 'text/plain', got '%s'", attachment.ContentType)
	}
	if attachment.FileName != "test.txt" {
		t.Errorf("Expected FileName 'test.txt', got '%s'", attachment.FileName)
	}
	if attachment.Size != 512 {
		t.Errorf("Expected Size 512, got %d", attachment.Size)
	}
	if attachment.Transformed {
		t.Error("Expected Transformed to be false")
	}
}

func TestEnvelope(t *testing.T) {
	envelope := &Envelope{
		From:          "from@example.com",
		To:            []string{"to1@example.com", "to2@example.com"},
		CC:            []string{"cc@example.com"},
		BCC:           []string{"bcc@example.com"},
		CalculatedBCC: []string{"calculated@example.com"},
		Host:          "localhost",
		RemoteAddress: "127.0.0.1:12345",
	}

	if envelope.From != "from@example.com" {
		t.Errorf("Expected From 'from@example.com', got '%s'", envelope.From)
	}
	if len(envelope.To) != 2 {
		t.Errorf("Expected 2 To addresses, got %d", len(envelope.To))
	}
	if len(envelope.CC) != 1 {
		t.Errorf("Expected 1 CC address, got %d", len(envelope.CC))
	}
	if len(envelope.BCC) != 1 {
		t.Errorf("Expected 1 BCC address, got %d", len(envelope.BCC))
	}
	if envelope.Host != "localhost" {
		t.Errorf("Expected Host 'localhost', got '%s'", envelope.Host)
	}
	if envelope.RemoteAddress != "127.0.0.1:12345" {
		t.Errorf("Expected RemoteAddress '127.0.0.1:12345', got '%s'", envelope.RemoteAddress)
	}
}

func TestEmailWithEnvelope(t *testing.T) {
	email := &Email{
		ID:      "test-id",
		Subject: "Test",
		Envelope: &Envelope{
			From: "from@example.com",
			To:   []string{"to@example.com"},
		},
	}

	if email.Envelope == nil {
		t.Error("Expected Envelope to be set")
	}
	if email.Envelope.From != "from@example.com" {
		t.Errorf("Expected Envelope.From 'from@example.com', got '%s'", email.Envelope.From)
	}
}

func TestEmailWithAttachments(t *testing.T) {
	email := &Email{
		ID:      "test-id",
		Subject: "Test",
		Attachments: []*Attachment{
			{
				FileName: "file1.txt",
				Size:     100,
			},
			{
				FileName: "file2.txt",
				Size:     200,
			},
		},
	}

	if len(email.Attachments) != 2 {
		t.Errorf("Expected 2 attachments, got %d", len(email.Attachments))
	}
	if email.Attachments[0].FileName != "file1.txt" {
		t.Errorf("Expected first attachment 'file1.txt', got '%s'", email.Attachments[0].FileName)
	}
}

func TestEmailWithHeaders(t *testing.T) {
	email := &Email{
		ID:      "test-id",
		Subject: "Test",
		Headers: map[string]interface{}{
			"X-Custom-Header":  "custom-value",
			"X-Another-Header": 123,
		},
	}

	if email.Headers == nil {
		t.Error("Expected Headers to be set")
	}
	if email.Headers["X-Custom-Header"] != "custom-value" {
		t.Errorf("Expected X-Custom-Header 'custom-value', got '%v'", email.Headers["X-Custom-Header"])
	}
	if email.Headers["X-Another-Header"] != 123 {
		t.Errorf("Expected X-Another-Header 123, got '%v'", email.Headers["X-Another-Header"])
	}
}
