package api

import (
	"mime"
	"strings"
)

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
	switch strings.ToLower(mediaType) {
	case "text/html", "application/xhtml+xml", "image/svg+xml", "application/xml", "text/xml", "application/javascript", "text/javascript":
		return true
	default:
		return false
	}
}
