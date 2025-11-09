package maildev

import (
	"os"
	"strconv"
)

// ============================================================================
// MailDev Compatibility Layer
// ============================================================================
//
// This file contains all MailDev compatibility-related code, including:
// 1. Environment variable compatibility layer: provides mapping from MailDev
//    environment variables to OwlMail environment variables
// 2. API route compatibility layer: provides fully compatible API routes with MailDev
//
// Environment Variable Compatibility Layer:
// Prioritizes MailDev environment variables, falls back to OwlMail environment variables
// if MailDev variables are not present
//
// Supported MailDev environment variable mappings:
//   - MAILDEV_SMTP_PORT → OWLMAIL_SMTP_PORT
//   - MAILDEV_IP → OWLMAIL_SMTP_HOST
//   - MAILDEV_MAIL_DIRECTORY → OWLMAIL_MAIL_DIR
//   - MAILDEV_WEB_PORT → OWLMAIL_WEB_PORT
//   - MAILDEV_WEB_IP → OWLMAIL_WEB_HOST
//   - MAILDEV_WEB_USER → OWLMAIL_WEB_USER
//   - MAILDEV_WEB_PASS → OWLMAIL_WEB_PASSWORD
//   - MAILDEV_HTTPS → OWLMAIL_HTTPS_ENABLED
//   - MAILDEV_HTTPS_CERT → OWLMAIL_HTTPS_CERT
//   - MAILDEV_HTTPS_KEY → OWLMAIL_HTTPS_KEY
//   - MAILDEV_OUTGOING_HOST → OWLMAIL_OUTGOING_HOST
//   - MAILDEV_OUTGOING_PORT → OWLMAIL_OUTGOING_PORT
//   - MAILDEV_OUTGOING_USER → OWLMAIL_OUTGOING_USER
//   - MAILDEV_OUTGOING_PASS → OWLMAIL_OUTGOING_PASSWORD
//   - MAILDEV_OUTGOING_SECURE → OWLMAIL_OUTGOING_SECURE
//   - MAILDEV_AUTO_RELAY → OWLMAIL_AUTO_RELAY
//   - MAILDEV_AUTO_RELAY_ADDR → OWLMAIL_AUTO_RELAY_ADDR
//   - MAILDEV_AUTO_RELAY_RULES → OWLMAIL_AUTO_RELAY_RULES
//   - MAILDEV_INCOMING_USER → OWLMAIL_SMTP_USER
//   - MAILDEV_INCOMING_PASS → OWLMAIL_SMTP_PASSWORD
//   - MAILDEV_INCOMING_SECURE → OWLMAIL_TLS_ENABLED
//   - MAILDEV_INCOMING_CERT → OWLMAIL_TLS_CERT
//   - MAILDEV_INCOMING_KEY → OWLMAIL_TLS_KEY
//
// Log level support:
//   - MAILDEV_VERBOSE → verbose
//   - MAILDEV_SILENT → silent
//   - OWLMAIL_LOG_LEVEL → normal/verbose/silent

// getEnvStringWithMailDevCompat gets environment variable value, prioritizing MailDev
// environment variables, falling back to OwlMail environment variables if not present
func getEnvStringWithMailDevCompat(maildevKey, owlmailKey, defaultValue string) string {
	// Check MailDev environment variable first
	if value := os.Getenv(maildevKey); value != "" {
		return value
	}
	// If MailDev environment variable is not present, use OwlMail environment variable
	if value := os.Getenv(owlmailKey); value != "" {
		return value
	}
	return defaultValue
}

// getEnvIntWithMailDevCompat gets environment variable integer value, prioritizing MailDev
// environment variables
func getEnvIntWithMailDevCompat(maildevKey, owlmailKey string, defaultValue int) int {
	// Check MailDev environment variable first
	if value := os.Getenv(maildevKey); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	// If MailDev environment variable is not present, use OwlMail environment variable
	if value := os.Getenv(owlmailKey); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvBoolWithMailDevCompat gets environment variable boolean value, prioritizing MailDev
// environment variables
func getEnvBoolWithMailDevCompat(maildevKey, owlmailKey string, defaultValue bool) bool {
	// Check MailDev environment variable first
	if value := os.Getenv(maildevKey); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	// If MailDev environment variable is not present, use OwlMail environment variable
	if value := os.Getenv(owlmailKey); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

// MailDev environment variable mapping table
// Mapping relationship: MAILDEV_* → OWLMAIL_*
var maildevEnvMapping = map[string]string{
	// SMTP server configuration
	"MAILDEV_SMTP_PORT":      "OWLMAIL_SMTP_PORT",
	"MAILDEV_IP":             "OWLMAIL_SMTP_HOST",
	"MAILDEV_MAIL_DIRECTORY": "OWLMAIL_MAIL_DIR",

	// Web API configuration
	"MAILDEV_WEB_PORT": "OWLMAIL_WEB_PORT",
	"MAILDEV_WEB_IP":   "OWLMAIL_WEB_HOST",
	"MAILDEV_WEB_USER": "OWLMAIL_WEB_USER",
	"MAILDEV_WEB_PASS": "OWLMAIL_WEB_PASSWORD",

	// HTTPS configuration
	"MAILDEV_HTTPS":      "OWLMAIL_HTTPS_ENABLED",
	"MAILDEV_HTTPS_CERT": "OWLMAIL_HTTPS_CERT",
	"MAILDEV_HTTPS_KEY":  "OWLMAIL_HTTPS_KEY",

	// Outgoing mail configuration
	"MAILDEV_OUTGOING_HOST":   "OWLMAIL_OUTGOING_HOST",
	"MAILDEV_OUTGOING_PORT":   "OWLMAIL_OUTGOING_PORT",
	"MAILDEV_OUTGOING_USER":   "OWLMAIL_OUTGOING_USER",
	"MAILDEV_OUTGOING_PASS":   "OWLMAIL_OUTGOING_PASSWORD",
	"MAILDEV_OUTGOING_SECURE": "OWLMAIL_OUTGOING_SECURE",

	// Auto relay configuration
	"MAILDEV_AUTO_RELAY":       "OWLMAIL_AUTO_RELAY",
	"MAILDEV_AUTO_RELAY_ADDR":  "OWLMAIL_AUTO_RELAY_ADDR",
	"MAILDEV_AUTO_RELAY_RULES": "OWLMAIL_AUTO_RELAY_RULES",

	// SMTP authentication configuration
	"MAILDEV_INCOMING_USER": "OWLMAIL_SMTP_USER",
	"MAILDEV_INCOMING_PASS": "OWLMAIL_SMTP_PASSWORD",

	// TLS configuration
	"MAILDEV_INCOMING_SECURE": "OWLMAIL_TLS_ENABLED",
	"MAILDEV_INCOMING_CERT":   "OWLMAIL_TLS_CERT",
	"MAILDEV_INCOMING_KEY":    "OWLMAIL_TLS_KEY",
}

// GetMailDevEnvString gets environment variable value with MailDev compatibility support
// Prioritizes MailDev environment variables, falls back to OwlMail environment variables
// if not present
func GetMailDevEnvString(owlmailKey string, defaultValue string) string {
	// Find corresponding MailDev environment variable name
	for maildevKey, mappedKey := range maildevEnvMapping {
		if mappedKey == owlmailKey {
			return getEnvStringWithMailDevCompat(maildevKey, owlmailKey, defaultValue)
		}
	}
	// If no mapping found, use OwlMail environment variable directly
	if value := os.Getenv(owlmailKey); value != "" {
		return value
	}
	return defaultValue
}

// GetMailDevEnvInt gets environment variable integer value with MailDev compatibility support
func GetMailDevEnvInt(owlmailKey string, defaultValue int) int {
	// Find corresponding MailDev environment variable name
	for maildevKey, mappedKey := range maildevEnvMapping {
		if mappedKey == owlmailKey {
			return getEnvIntWithMailDevCompat(maildevKey, owlmailKey, defaultValue)
		}
	}
	// If no mapping found, use OwlMail environment variable directly
	if value := os.Getenv(owlmailKey); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// GetMailDevEnvBool gets environment variable boolean value with MailDev compatibility support
func GetMailDevEnvBool(owlmailKey string, defaultValue bool) bool {
	// Find corresponding MailDev environment variable name
	for maildevKey, mappedKey := range maildevEnvMapping {
		if mappedKey == owlmailKey {
			return getEnvBoolWithMailDevCompat(maildevKey, owlmailKey, defaultValue)
		}
	}
	// If no mapping found, use OwlMail environment variable directly
	if value := os.Getenv(owlmailKey); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

// GetMailDevLogLevel gets log level with MailDev compatibility support
// MailDev uses --verbose and --silent flags, here we provide compatibility via environment variables
func GetMailDevLogLevel(defaultValue string) string {
	// MailDev uses --verbose and --silent flags, which don't have corresponding environment variables
	// But we can check for MAILDEV_VERBOSE or MAILDEV_SILENT environment variables
	if os.Getenv("MAILDEV_VERBOSE") != "" {
		return "verbose"
	}
	if os.Getenv("MAILDEV_SILENT") != "" {
		return "silent"
	}
	// If not set, use OwlMail's log level environment variable
	if value := os.Getenv("OWLMAIL_LOG_LEVEL"); value != "" {
		return value
	}
	return defaultValue
}

// ============================================================================
// MailDev API Route Compatibility Layer
// ============================================================================
//
// This section provides fully compatible API routes with MailDev, maintaining
// backward compatibility. All MailDev-compatible API routes are defined in this file.
// The new API design is implemented in api.go using a more reasonable RESTful design.
//
// Note: This function is called in api.go's setupRoutes()
//
// MailDev Original API Endpoints (maintained for compatibility):
//   - GET    /email                    - Get all emails
//   - GET    /email/:id                - Get single email
//   - GET    /email/:id/html           - Get email HTML
//   - GET    /email/:id/attachment/:filename - Download attachment
//   - GET    /email/:id/download        - Download raw .eml file
//   - GET    /email/:id/source         - Get email raw source
//   - DELETE /email/:id                - Delete single email
//   - DELETE /email/all                 - Delete all emails
//   - PATCH  /email/read-all            - Mark all emails as read
//   - POST   /email/:id/relay/:relayTo? - Relay email
//   - GET    /config                    - Get configuration
//   - GET    /healthz                   - Health check
//   - GET    /reloadMailsFromDirectory  - Reload emails
//   - GET    /socket.io                 - WebSocket connection
//
// New API Design (more reasonable):
//   - GET    /api/v1/emails             - Get all emails (plural resource)
//   - GET    /api/v1/emails/:id         - Get single email
//   - GET    /api/v1/emails/:id/html    - Get email HTML
//   - GET    /api/v1/emails/:id/attachments/:filename - Download attachment (plural)
//   - GET    /api/v1/emails/:id/raw     - Get raw email (clearer naming)
//   - GET    /api/v1/emails/:id/source  - Get email source
//   - DELETE /api/v1/emails/:id         - Delete single email
//   - DELETE /api/v1/emails              - Delete all emails (batch operation)
//   - PATCH  /api/v1/emails/read         - Mark all emails as read (clearer)
//   - PATCH  /api/v1/emails/:id/read    - Mark single email as read
//   - POST   /api/v1/emails/:id/actions/relay - Relay email (clearer action)
//   - GET    /api/v1/emails/stats       - Email statistics
//   - GET    /api/v1/emails/preview     - Email preview
//   - DELETE /api/v1/emails/batch      - Batch delete (more RESTful)
//   - PATCH  /api/v1/emails/batch/read  - Batch mark as read
//   - GET    /api/v1/emails/export      - Export emails
//   - GET    /api/v1/settings           - Get all settings
//   - GET    /api/v1/settings/outgoing - Get outgoing configuration
//   - PUT    /api/v1/settings/outgoing - Update outgoing configuration
//   - PATCH  /api/v1/settings/outgoing - Partially update outgoing configuration
//   - GET    /api/v1/health             - Health check (more standard)
//   - POST   /api/v1/emails/reload     - Reload emails (POST is more appropriate)
//   - GET    /api/v1/ws                 - WebSocket connection (clearer)
//
// API Design Improvements:
// 1. Resource naming uses plural form: /emails instead of /email
// 2. More standard RESTful design: DELETE /emails represents batch deletion
// 3. Clearer action naming: /actions/relay clearly indicates this is an action
// 4. More standard sub-resource naming: /attachments uses plural form
// 5. Clearer configuration API: /settings is more semantic than /config
// 6. More standard health check: /health is more common than /healthz
// 7. Reload uses POST: POST /emails/reload is more appropriate than GET
// 8. Clearer WebSocket path: /ws is more concise than /socket.io
// 9. API versioning: /api/v1/ provides version control
// 10. More RESTful batch operations: DELETE /emails/batch instead of POST /email/batch/delete
