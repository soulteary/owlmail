package mailserver

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/emersion/go-message"
	"github.com/emersion/go-message/mail"
)

func TestMakeID(t *testing.T) {
	id1 := makeID()
	id2 := makeID()

	if len(id1) != 8 {
		t.Errorf("Expected ID length 8, got %d", len(id1))
	}
	if id1 == id2 {
		t.Error("IDs should be unique")
	}
}

func TestFormatBytes(t *testing.T) {
	testCases := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 bytes"},
		{512, "512 bytes"},
		{1024, "1.00 KB"},
		{1536, "1.50 KB"},
		{1048576, "1.00 MB"},
		{1073741824, "1.00 GB"},
	}

	for _, tc := range testCases {
		result := formatBytes(tc.bytes)
		if result != tc.expected {
			t.Errorf("For %d bytes: Expected '%s', got '%s'", tc.bytes, tc.expected, result)
		}
	}
}

func TestAddressListToStrings(t *testing.T) {
	addrs := []*mail.Address{
		{Address: "test1@example.com"},
		{Address: "test2@example.com"},
	}

	result := addressListToStrings(addrs)
	if len(result) != 2 {
		t.Errorf("Expected 2 addresses, got %d", len(result))
	}
	if result[0] != "test1@example.com" {
		t.Errorf("Expected 'test1@example.com', got '%s'", result[0])
	}
	if result[1] != "test2@example.com" {
		t.Errorf("Expected 'test2@example.com', got '%s'", result[1])
	}
}

func TestCalculateBCC(t *testing.T) {
	recipients := []string{"to1@example.com", "to2@example.com", "cc1@example.com", "bcc1@example.com"}
	to := []string{"to1@example.com", "to2@example.com"}
	cc := []string{"cc1@example.com"}

	bcc := calculateBCC(recipients, to, cc)
	if len(bcc) != 1 {
		t.Errorf("Expected 1 BCC address, got %d", len(bcc))
	}
	if bcc[0].Address != "bcc1@example.com" {
		t.Errorf("Expected 'bcc1@example.com', got '%s'", bcc[0].Address)
	}
}

func TestTransformAttachmentEdgeCases(t *testing.T) {
	// Test with empty filename
	attachment := &Attachment{
		ContentType: "text/plain",
		FileName:    "",
		Size:        100,
	}
	transformed := transformAttachment(attachment)
	if transformed.GeneratedFileName == "" {
		t.Error("Transformed attachment should have a generated filename")
	}

	// Test with very long filename
	longFilename := strings.Repeat("a", 300) + ".txt"
	attachment2 := &Attachment{
		ContentType: "text/plain",
		FileName:    longFilename,
		Size:        100,
	}
	transformed2 := transformAttachment(attachment2)
	if transformed2.GeneratedFileName == "" {
		t.Error("Transformed attachment should have a generated filename even for long filenames")
	}

	// Test with special characters in filename
	attachment3 := &Attachment{
		ContentType: "text/plain",
		FileName:    "test file with spaces & special chars.txt",
		Size:        100,
	}
	transformed3 := transformAttachment(attachment3)
	if transformed3.GeneratedFileName == "" {
		t.Error("Transformed attachment should handle special characters")
	}
}

func TestTransformAttachment(t *testing.T) {
	// Test with filename and extension
	attachment := &Attachment{
		FileName:    "test.pdf",
		ContentType: "application/pdf",
	}

	result := transformAttachment(attachment)
	if result.GeneratedFileName == "" {
		t.Error("Generated filename should not be empty")
	}
	if !result.Transformed {
		t.Error("Attachment should be marked as transformed")
	}
	if filepath.Ext(result.GeneratedFileName) != ".pdf" {
		t.Error("Generated filename should have .pdf extension")
	}

	// Test with ContentID
	attachment2 := &Attachment{
		FileName:    "test.pdf",
		ContentID:   "test-content-id",
		ContentType: "application/pdf",
	}

	result2 := transformAttachment(attachment2)
	if result2.GeneratedFileName == "" {
		t.Error("Generated filename should not be empty")
	}

	// Test with no extension
	attachment3 := &Attachment{
		FileName:    "test",
		ContentType: "text/plain",
	}

	result3 := transformAttachment(attachment3)
	if filepath.Ext(result3.GeneratedFileName) == "" {
		t.Error("Generated filename should have extension")
	}

	// Test already transformed
	attachment4 := &Attachment{
		FileName:          "test.pdf",
		Transformed:       true,
		GeneratedFileName: "already-generated.pdf",
	}

	result4 := transformAttachment(attachment4)
	if result4.GeneratedFileName != "already-generated.pdf" {
		t.Error("Already transformed attachment should not be transformed again")
	}
}

func TestSanitizeHTML(t *testing.T) {
	html := `<html><body><script>alert('xss')</script><p>Safe content</p><a href="http://example.com" target="_blank">Link</a></body></html>`
	sanitized := sanitizeHTML(html)

	if len(sanitized) == 0 {
		t.Error("Sanitized HTML should not be empty")
	}
	// Script should be removed
	if len(sanitized) >= len(html) {
		t.Error("Sanitized HTML should be shorter (script removed)")
	}
}

func TestParseEmailDate(t *testing.T) {
	// Create a message header
	header := message.Header{}
	header.Set("Date", "Mon, 02 Jan 2006 15:04:05 -0700")

	date := parseEmailDate(header)
	if date.IsZero() {
		t.Error("Date should not be zero")
	}

	// Test with empty date
	header2 := message.Header{}
	date2 := parseEmailDate(header2)
	if date2.IsZero() {
		t.Error("Date should default to current time")
	}

	// Test with invalid date
	header3 := message.Header{}
	header3.Set("Date", "invalid date")
	date3 := parseEmailDate(header3)
	if date3.IsZero() {
		t.Error("Date should default to current time for invalid date")
	}

	// Test with different date formats
	testCases := []string{
		"Mon, 02 Jan 2006 15:04:05 MST",
		"02 Jan 06 15:04 -0700",
		"02 Jan 06 15:04 MST",
		"2006-01-02T15:04:05Z07:00",
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 MST",
		"2 Jan 2006 15:04:05 -0700",
		"2 Jan 2006 15:04:05 MST",
		"Mon, 2 Jan 2006 15:04:05",
		"2 Jan 2006 15:04:05",
	}

	for _, dateStr := range testCases {
		header4 := message.Header{}
		header4.Set("Date", dateStr)
		date4 := parseEmailDate(header4)
		if date4.IsZero() {
			t.Errorf("Date should not be zero for format: %s", dateStr)
		}
	}

	// Test with timezone abbreviations
	header5 := message.Header{}
	header5.Set("Date", "Mon, 02 Jan 2006 15:04:05 -0700 (GMT)")
	date5 := parseEmailDate(header5)
	if date5.IsZero() {
		t.Error("Date should parse with timezone abbreviation")
	}

	header6 := message.Header{}
	header6.Set("Date", "Mon, 02 Jan 2006 15:04:05 -0700 GMT")
	date6 := parseEmailDate(header6)
	if date6.IsZero() {
		t.Error("Date should parse with GMT suffix")
	}

	header7 := message.Header{}
	header7.Set("Date", "Mon, 02 Jan 2006 15:04:05 -0700 (UTC)")
	date7 := parseEmailDate(header7)
	if date7.IsZero() {
		t.Error("Date should parse with UTC abbreviation")
	}

	header8 := message.Header{}
	header8.Set("Date", "Mon, 02 Jan 2006 15:04:05 -0700 UTC")
	date8 := parseEmailDate(header8)
	if date8.IsZero() {
		t.Error("Date should parse with UTC suffix")
	}
}

func TestGenerateSelfSignedCert(t *testing.T) {
	cert, err := generateSelfSignedCert()
	if err != nil {
		t.Fatalf("Failed to generate self-signed certificate: %v", err)
	}

	if len(cert.Certificate) == 0 {
		t.Error("Certificate should have certificate data")
	}
	if cert.PrivateKey == nil {
		t.Error("Certificate should have private key")
	}
}
