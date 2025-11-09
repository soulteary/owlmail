package mailserver

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/md5"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"mime"
	"net"
	"path/filepath"
	"strings"
	"time"

	"github.com/emersion/go-message"
	"github.com/emersion/go-message/mail"
	"github.com/microcosm-cc/bluemonday"
	"github.com/soulteary/owlmail/internal/common"
)

// makeID generates a unique 8-character ID
func makeID() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback to time-based if random fails
		for i := range b {
			b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
		}
	} else {
		for i := range b {
			b[i] = charset[b[i]%byte(len(charset))]
		}
	}
	return string(b)
}

// formatBytes formats bytes to human-readable string
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d bytes", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// addressListToStrings converts mail.Address list to string list
func addressListToStrings(addrs []*mail.Address) []string {
	result := make([]string, len(addrs))
	for i, addr := range addrs {
		result[i] = addr.Address
	}
	return result
}

// calculateBCC calculates BCC addresses
func calculateBCC(recipients, to, cc []string) []*mail.Address {
	bccAddresses := make([]*mail.Address, 0)
	toCopy := make([]string, len(to))
	ccCopy := make([]string, len(cc))
	copy(toCopy, to)
	copy(ccCopy, cc)

	for _, recipient := range recipients {
		// Check if in CC
		found := false
		for i, addr := range ccCopy {
			if addr == recipient {
				ccCopy = append(ccCopy[:i], ccCopy[i+1:]...)
				found = true
				break
			}
		}
		if found {
			continue
		}

		// Check if in TO
		for i, addr := range toCopy {
			if addr == recipient {
				toCopy = append(toCopy[:i], toCopy[i+1:]...)
				found = true
				break
			}
		}
		if found {
			continue
		}

		// Must be BCC
		bccAddresses = append(bccAddresses, &mail.Address{Address: recipient})
	}

	return bccAddresses
}

// transformAttachment transforms attachment filename for security
func transformAttachment(attachment *Attachment) *Attachment {
	if attachment.Transformed {
		return attachment
	}

	// Extract extension from original filename
	ext := filepath.Ext(attachment.FileName)
	if ext == "" {
		// Try to get extension from Content-Type
		if attachment.ContentType != "" {
			exts, _ := mime.ExtensionsByType(attachment.ContentType)
			if len(exts) > 0 {
				ext = exts[0]
			}
		}
		if ext == "" {
			ext = ".bin"
		}
	}

	// Generate filename from ContentID or use hash
	var name string
	if attachment.ContentID != "" {
		hash := md5.Sum([]byte(attachment.ContentID))
		name = fmt.Sprintf("%x", hash)
	} else {
		// Use filename + timestamp for uniqueness
		hash := md5.Sum([]byte(attachment.FileName + time.Now().String()))
		name = fmt.Sprintf("%x", hash)
	}

	attachment.GeneratedFileName = name + ext
	attachment.Transformed = true
	return attachment
}

// validateEmailID validates and sanitizes an email ID to prevent path traversal attacks
// It ensures the ID doesn't contain path traversal characters and is not empty
func validateEmailID(id string) error {
	if id == "" {
		return fmt.Errorf("email ID cannot be empty")
	}
	// Check for path traversal characters
	if strings.Contains(id, "..") || strings.Contains(id, "/") || strings.Contains(id, "\\") {
		return fmt.Errorf("invalid email ID: contains path traversal characters")
	}
	// Check for null bytes
	if strings.Contains(id, "\x00") {
		return fmt.Errorf("invalid email ID: contains null byte")
	}
	// Validate that ID only contains safe characters (alphanumeric, hyphen, underscore)
	// This allows test IDs like "test-id" while preventing path traversal
	for _, r := range id {
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return fmt.Errorf("invalid email ID: contains invalid characters")
		}
	}
	return nil
}

// validatePath ensures the resolved path is within the base directory to prevent path traversal
func validatePath(baseDir, resolvedPath string) error {
	// Get absolute paths
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for base directory: %w", err)
	}
	absResolved, err := filepath.Abs(resolvedPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for resolved path: %w", err)
	}
	// Check if resolved path is within base directory
	rel, err := filepath.Rel(absBase, absResolved)
	if err != nil {
		return fmt.Errorf("failed to compute relative path: %w", err)
	}
	// If relative path starts with "..", it's outside the base directory
	if strings.HasPrefix(rel, "..") {
		return fmt.Errorf("path traversal detected: path is outside base directory")
	}
	return nil
}

// sanitizeHTML sanitizes HTML content
func sanitizeHTML(html string) string {
	p := bluemonday.UGCPolicy()
	p.AllowAttrs("target").OnElements("a")
	p.AllowElements("link")
	return p.Sanitize(html)
}

// parseEmailDate parses the Date header from email headers
func parseEmailDate(headers message.Header) time.Time {
	dateStr := headers.Get("Date")
	if dateStr == "" {
		return time.Now()
	}

	// Try multiple date formats commonly used in email
	dateFormats := []string{
		time.RFC1123Z,    // Mon, 02 Jan 2006 15:04:05 -0700
		time.RFC1123,     // Mon, 02 Jan 2006 15:04:05 MST
		time.RFC822Z,     // 02 Jan 06 15:04 -0700
		time.RFC822,      // 02 Jan 06 15:04 MST
		time.RFC3339,     // 2006-01-02T15:04:05Z07:00
		time.RFC3339Nano, // 2006-01-02T15:04:05.999999999Z07:00
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

	// Try parsing with each format
	for _, format := range dateFormats {
		if date, err := time.Parse(format, dateStr); err == nil {
			return date
		}
	}

	// Try parsing with time.ParseInLocation for timezone-aware parsing
	if date, err := time.ParseInLocation(time.RFC1123Z, dateStr, time.UTC); err == nil {
		return date
	}
	if date, err := time.ParseInLocation(time.RFC1123, dateStr, time.UTC); err == nil {
		return date
	}

	// Fallback: try to parse with a more lenient approach
	// Remove common timezone abbreviations and try again
	cleanedDate := strings.TrimSpace(dateStr)
	// Remove timezone abbreviations like (GMT), (UTC), etc.
	cleanedDate = strings.TrimSuffix(cleanedDate, " (GMT)")
	cleanedDate = strings.TrimSuffix(cleanedDate, " (UTC)")
	cleanedDate = strings.TrimSuffix(cleanedDate, " GMT")
	cleanedDate = strings.TrimSuffix(cleanedDate, " UTC")

	for _, format := range dateFormats {
		if date, err := time.Parse(format, cleanedDate); err == nil {
			return date
		}
	}

	// If all parsing attempts fail, return current time
	common.Verbose("Failed to parse email date: %s, using current time", dateStr)
	return time.Now()
}

// generateSelfSignedCert generates a self-signed certificate for testing
func generateSelfSignedCert() (tls.Certificate, error) {
	// Generate private key
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"OwlMail"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{""},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour), // Valid for 1 year
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Add IP addresses and DNS names
	template.IPAddresses = []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback}
	template.DNSNames = []string{"localhost", "127.0.0.1"}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to create certificate: %w", err)
	}

	// Encode private key
	privDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to marshal private key: %w", err)
	}

	// Create PEM blocks
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privDER})

	// Load certificate
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to load certificate: %w", err)
	}

	return cert, nil
}
