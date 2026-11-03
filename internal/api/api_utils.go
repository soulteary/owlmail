package api

import (
	"mime"
	"strings"
)

// setEmailHTMLResponseHeaders gives direct HTML views the same default remote
// resource boundary as the browser preview. The JSON body remains available to
// clients, and the UI's explicit remote-content preview is unaffected.
func setEmailHTMLResponseHeaders(c interface{ Set(string, string) }) {
	c.Set("Content-Type", "text/html; charset=utf-8")
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set("Referrer-Policy", "no-referrer")
	c.Set("Content-Security-Policy", "sandbox; default-src 'none'; base-uri 'none'; form-action 'none'; img-src 'self' data:; media-src 'self' data:; style-src 'unsafe-inline' 'self'; font-src 'self' data:")
}

// sanitizeFilename sanitizes a filename for safe download
func sanitizeFilename(filename string) string {
	// Remove or replace invalid characters
	filename = strings.ReplaceAll(filename, "/", "_")
	filename = strings.ReplaceAll(filename, "\\", "_")
	filename = strings.ReplaceAll(filename, ":", "_")
	filename = strings.ReplaceAll(filename, "*", "_")
	filename = strings.ReplaceAll(filename, "?", "_")
	filename = strings.ReplaceAll(filename, "\"", "_")
	filename = strings.ReplaceAll(filename, "<", "_")
	filename = strings.ReplaceAll(filename, ">", "_")
	filename = strings.ReplaceAll(filename, "|", "_")

	// Limit length
	if len(filename) > 100 {
		filename = filename[:100]
	}

	return filename
}

// setAttachmentResponseHeaders prevents browsers from MIME-sniffing untrusted
// message parts. Formats that can execute active content are downloads by
// default; passive formats retain inline compatibility.
func setAttachmentResponseHeaders(c interface{ Set(string, string) }, contentType, filename string) {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Set("Content-Type", contentType)
	c.Set("X-Content-Type-Options", "nosniff")
	if isActiveAttachmentType(contentType) {
		name := sanitizeFilename(filename)
		if name == "" {
			name = "attachment"
		}
		c.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	}
}

func isActiveAttachmentType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType = strings.TrimSpace(strings.Split(contentType, ";")[0])
	}
	mediaType = strings.ToLower(mediaType)
	if strings.HasSuffix(mediaType, "+xml") ||
		strings.Contains(mediaType, "javascript") ||
		strings.Contains(mediaType, "ecmascript") {
		return true
	}
	switch mediaType {
	case "text/html", "application/xml", "text/xml", "application/wasm":
		return true
	default:
		return false
	}
}
