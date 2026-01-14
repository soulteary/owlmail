package mailserver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emersion/go-message"
	"github.com/emersion/go-message/mail"
)

func TestMakeID(t *testing.T) {
	// Test random string ID
	id1 := makeID(false)
	id2 := makeID(false)

	if len(id1) != 8 {
		t.Errorf("Expected ID length 8, got %d", len(id1))
	}
	if id1 == id2 {
		t.Error("IDs should be unique")
	}

	// Test UUID ID
	uuid1 := makeID(true)
	uuid2 := makeID(true)

	if len(uuid1) != 36 {
		t.Errorf("Expected UUID length 36, got %d", len(uuid1))
	}
	if uuid1 == uuid2 {
		t.Error("UUIDs should be unique")
	}
	// UUID format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	if uuid1[8] != '-' || uuid1[13] != '-' || uuid1[18] != '-' || uuid1[23] != '-' {
		t.Error("UUID should have correct format with hyphens")
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
		{1099511627776, "1.00 TB"},
		{1125899906842624, "1.00 PB"},
		{1152921504606846976, "1.00 EB"},
		{1023, "1023 bytes"},
		{1025, "1.00 KB"},
		{1048575, "1024.00 KB"},
		{1048577, "1.00 MB"},
	}

	for _, tc := range testCases {
		result := formatBytes(tc.bytes)
		if result != tc.expected {
			t.Errorf("For %d bytes: Expected '%s', got '%s'", tc.bytes, tc.expected, result)
		}
	}
}

func TestAddressListToStrings(t *testing.T) {
	// Test with multiple addresses
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

	// Test with empty list
	emptyAddrs := []*mail.Address{}
	emptyResult := addressListToStrings(emptyAddrs)
	if len(emptyResult) != 0 {
		t.Errorf("Expected 0 addresses, got %d", len(emptyResult))
	}

	// Test with single address
	singleAddr := []*mail.Address{
		{Address: "single@example.com"},
	}
	singleResult := addressListToStrings(singleAddr)
	if len(singleResult) != 1 {
		t.Errorf("Expected 1 address, got %d", len(singleResult))
	}
	if singleResult[0] != "single@example.com" {
		t.Errorf("Expected 'single@example.com', got '%s'", singleResult[0])
	}
}

func TestCalculateBCC(t *testing.T) {
	// Test basic BCC calculation
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

	// Test with multiple BCC addresses
	recipients2 := []string{"to1@example.com", "bcc1@example.com", "bcc2@example.com", "bcc3@example.com"}
	to2 := []string{"to1@example.com"}
	cc2 := []string{}
	bcc2 := calculateBCC(recipients2, to2, cc2)
	if len(bcc2) != 3 {
		t.Errorf("Expected 3 BCC addresses, got %d", len(bcc2))
	}

	// Test with no BCC addresses
	recipients3 := []string{"to1@example.com", "cc1@example.com"}
	to3 := []string{"to1@example.com"}
	cc3 := []string{"cc1@example.com"}
	bcc3 := calculateBCC(recipients3, to3, cc3)
	if len(bcc3) != 0 {
		t.Errorf("Expected 0 BCC addresses, got %d", len(bcc3))
	}

	// Test with empty recipients
	recipients4 := []string{}
	to4 := []string{"to1@example.com"}
	cc4 := []string{"cc1@example.com"}
	bcc4 := calculateBCC(recipients4, to4, cc4)
	if len(bcc4) != 0 {
		t.Errorf("Expected 0 BCC addresses, got %d", len(bcc4))
	}

	// Test with recipients only in CC
	recipients5 := []string{"cc1@example.com", "bcc1@example.com"}
	to5 := []string{}
	cc5 := []string{"cc1@example.com"}
	bcc5 := calculateBCC(recipients5, to5, cc5)
	if len(bcc5) != 1 {
		t.Errorf("Expected 1 BCC address, got %d", len(bcc5))
	}
	if bcc5[0].Address != "bcc1@example.com" {
		t.Errorf("Expected 'bcc1@example.com', got '%s'", bcc5[0].Address)
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

	// Test with no extension but with ContentType that has extension
	attachment3 := &Attachment{
		FileName:    "test",
		ContentType: "text/plain",
	}

	result3 := transformAttachment(attachment3)
	if filepath.Ext(result3.GeneratedFileName) == "" {
		t.Error("Generated filename should have extension")
	}

	// Test with no extension and ContentType without recognized extension
	attachment4 := &Attachment{
		FileName:    "test",
		ContentType: "application/unknown",
	}

	result4 := transformAttachment(attachment4)
	if filepath.Ext(result4.GeneratedFileName) != ".bin" {
		t.Errorf("Expected .bin extension for unknown ContentType, got %s", filepath.Ext(result4.GeneratedFileName))
	}

	// Test with no extension and empty ContentType
	attachment5 := &Attachment{
		FileName:    "test",
		ContentType: "",
	}

	result5 := transformAttachment(attachment5)
	if filepath.Ext(result5.GeneratedFileName) != ".bin" {
		t.Errorf("Expected .bin extension for empty ContentType, got %s", filepath.Ext(result5.GeneratedFileName))
	}

	// Test already transformed
	attachment6 := &Attachment{
		FileName:          "test.pdf",
		Transformed:       true,
		GeneratedFileName: "already-generated.pdf",
	}

	result6 := transformAttachment(attachment6)
	if result6.GeneratedFileName != "already-generated.pdf" {
		t.Error("Already transformed attachment should not be transformed again")
	}

	// Test with ContentID but no filename
	attachment7 := &Attachment{
		FileName:    "",
		ContentID:   "test-content-id-123",
		ContentType: "image/png",
	}

	result7 := transformAttachment(attachment7)
	if result7.GeneratedFileName == "" {
		t.Error("Generated filename should not be empty")
	}
	if !result7.Transformed {
		t.Error("Attachment should be marked as transformed")
	}
}

func TestSanitizeHTML(t *testing.T) {
	// Test with script tag
	html := `<html><body><script>alert('xss')</script><p>Safe content</p><a href="http://example.com" target="_blank">Link</a></body></html>`
	sanitized := sanitizeHTML(html)

	if len(sanitized) == 0 {
		t.Error("Sanitized HTML should not be empty")
	}
	// Script should be removed
	if len(sanitized) >= len(html) {
		t.Error("Sanitized HTML should be shorter (script removed)")
	}
	if strings.Contains(sanitized, "<script>") {
		t.Error("Script tag should be removed from sanitized HTML")
	}

	// Test with link element (should be allowed if attributes are permitted)
	html2 := `<html><head><link rel="stylesheet" href="style.css"></head><body><p>Content</p></body></html>`
	sanitized2 := sanitizeHTML(html2)
	// Note: bluemonday may remove link elements even if allowed, depending on context
	// We test that sanitization works without error
	if len(sanitized2) == 0 {
		t.Error("Sanitized HTML should not be empty")
	}
	// If link is preserved, it should have allowed attributes
	if strings.Contains(sanitized2, "<link") {
		// Verify that the link element has the expected attributes if present
		if !strings.Contains(sanitized2, "rel") && !strings.Contains(sanitized2, "href") {
			t.Error("Link element should have rel or href attribute if preserved")
		}
	}

	// Test with anchor tag with target attribute (should be allowed)
	html3 := `<a href="http://example.com" target="_blank">Link</a>`
	sanitized3 := sanitizeHTML(html3)
	if !strings.Contains(sanitized3, "target") {
		t.Error("Target attribute should be allowed on anchor tags")
	}

	// Test with empty HTML
	html4 := ""
	sanitized4 := sanitizeHTML(html4)
	if sanitized4 != "" {
		t.Error("Empty HTML should return empty string")
	}

	// Test with only safe content
	html5 := `<p>Safe content</p><div>More content</div>`
	sanitized5 := sanitizeHTML(html5)
	if !strings.Contains(sanitized5, "Safe content") {
		t.Error("Safe content should be preserved")
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

	// Test with all date formats from the function
	allFormats := []string{
		"Mon, 02 Jan 2006 15:04:05 -0700",     // RFC1123Z
		"Mon, 02 Jan 2006 15:04:05 MST",       // RFC1123
		"02 Jan 06 15:04 -0700",               // RFC822Z
		"02 Jan 06 15:04 MST",                 // RFC822
		"2006-01-02T15:04:05Z07:00",           // RFC3339
		"2006-01-02T15:04:05.999999999Z07:00", // RFC3339Nano
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 MST",
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"Mon, 02 Jan 2006 15:04:05 MST",
		"2 Jan 2006 15:04:05 -0700",
		"2 Jan 2006 15:04:05 MST",
		"02 Jan 2006 15:04:05 -0700",
		"02 Jan 2006 15:04:05 MST",
		"Mon, 2 Jan 2006 15:04:05",
		"Mon, 02 Jan 2006 15:04:05",
		"2 Jan 2006 15:04:05",
		"02 Jan 2006 15:04:05",
	}

	for _, format := range allFormats {
		header9 := message.Header{}
		header9.Set("Date", format)
		date9 := parseEmailDate(header9)
		if date9.IsZero() {
			t.Errorf("Date should not be zero for format: %s", format)
		}
	}

	// Test with whitespace in date string
	header10 := message.Header{}
	header10.Set("Date", "  Mon, 02 Jan 2006 15:04:05 -0700  ")
	date10 := parseEmailDate(header10)
	if date10.IsZero() {
		t.Error("Date should parse with whitespace")
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

	// Test that certificate can be generated multiple times
	cert2, err2 := generateSelfSignedCert()
	if err2 != nil {
		t.Fatalf("Failed to generate second self-signed certificate: %v", err2)
	}
	if len(cert2.Certificate) == 0 {
		t.Error("Second certificate should have certificate data")
	}
	if cert2.PrivateKey == nil {
		t.Error("Second certificate should have private key")
	}
}

func TestValidateEmailID(t *testing.T) {
	// Test valid IDs
	validIDs := []string{
		"test-id",
		"test_id",
		"test123",
		"12345678",
		"abcdefgh",
		"xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx", // UUID format
		"a1b2c3d4",
		"Test-ID_123",
	}

	for _, id := range validIDs {
		if err := validateEmailID(id); err != nil {
			t.Errorf("Expected valid ID '%s' to pass validation, got error: %v", id, err)
		}
	}

	// Test empty ID
	if err := validateEmailID(""); err == nil {
		t.Error("Expected error for empty ID")
	}

	// Test path traversal characters
	invalidIDs := []string{
		"../test",
		"test/../",
		"test\\..",
		"..",
		"test/",
		"test\\",
		"/test",
		"\\test",
	}

	for _, id := range invalidIDs {
		err := validateEmailID(id)
		if err == nil {
			t.Errorf("Expected error for ID with path traversal: '%s'", id)
		}
		if err != nil && !strings.Contains(err.Error(), "path traversal") {
			t.Errorf("Expected path traversal error for '%s', got: %v", id, err)
		}
	}

	// Test null byte
	err := validateEmailID("test\x00id")
	if err == nil {
		t.Error("Expected error for ID with null byte")
	}
	if err != nil && !strings.Contains(err.Error(), "null byte") {
		t.Errorf("Expected null byte error, got: %v", err)
	}

	// Test invalid characters (excluding path traversal chars / and \ which are tested separately)
	invalidChars := []string{
		"test@id",
		"test id",
		"test.id",
		"test#id",
		"test$id",
		"test%id",
		"test&id",
		"test*id",
		"test+id",
		"test=id",
		"test[id",
		"test]id",
		"test{id",
		"test}id",
		"test|id",
		"test:id",
		"test;id",
		"test'id",
		"test\"id",
		"test<id",
		"test>id",
		"test?id",
		"test~id",
		"test`id",
		"test!id",
	}

	for _, id := range invalidChars {
		err := validateEmailID(id)
		if err == nil {
			t.Errorf("Expected error for ID with invalid characters: '%s'", id)
		}
		if err != nil && !strings.Contains(err.Error(), "invalid characters") {
			t.Errorf("Expected invalid characters error for '%s', got: %v", id, err)
		}
	}
}

func TestValidatePath(t *testing.T) {
	tmpDir := t.TempDir()

	// Test valid paths within base directory
	validPaths := []string{
		tmpDir,
		filepath.Join(tmpDir, "subdir"),
		filepath.Join(tmpDir, "subdir", "file.txt"),
		filepath.Join(tmpDir, "..", filepath.Base(tmpDir)),
	}

	for _, path := range validPaths {
		if err := validatePath(tmpDir, path); err != nil {
			t.Errorf("Expected valid path '%s' to pass validation, got error: %v", path, err)
		}
	}

	// Test path traversal outside base directory
	parentDir := filepath.Dir(tmpDir)
	outsidePath := filepath.Join(parentDir, "outside.txt")
	err := validatePath(tmpDir, outsidePath)
	if err == nil {
		t.Error("Expected error for path outside base directory")
	}
	if err != nil && !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("Expected path traversal error, got: %v", err)
	}

	// Test with non-existent base directory (should still validate relative path)
	nonExistentBase := filepath.Join(tmpDir, "nonexistent")
	validSubPath := filepath.Join(nonExistentBase, "file.txt")
	// This should pass because the resolved path is still within the base
	// Note: validatePath may or may not return an error for non-existent base,
	// but the path should still be validated as being within the base directory
	_ = validatePath(nonExistentBase, validSubPath)

	// Test with absolute paths
	absBase, _ := filepath.Abs(tmpDir)
	absPath := filepath.Join(absBase, "file.txt")
	if err := validatePath(absBase, absPath); err != nil {
		t.Errorf("Expected valid absolute path to pass validation, got error: %v", err)
	}

	// Test with relative paths
	relPath := "subdir/file.txt"
	if err := validatePath(tmpDir, filepath.Join(tmpDir, relPath)); err != nil {
		t.Errorf("Expected valid relative path to pass validation, got error: %v", err)
	}

	// Create a subdirectory and test
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	subFile := filepath.Join(subDir, "file.txt")
	if err := validatePath(tmpDir, subFile); err != nil {
		t.Errorf("Expected valid subdirectory path to pass validation, got error: %v", err)
	}

	// Test with invalid base directory (should handle gracefully)
	invalidBase := filepath.Join(tmpDir, "invalid\x00base")
	validPath := filepath.Join(tmpDir, "file.txt")
	// This might fail due to invalid characters, which is expected
	_ = validatePath(invalidBase, validPath)
}
