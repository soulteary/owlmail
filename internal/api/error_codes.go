package api

// API Error Codes
// These codes are returned to the frontend, which will translate them based on user's language preference
const (
	// Email errors
	ErrorCodeEmailNotFound      = "EMAIL_NOT_FOUND"
	ErrorCodeEmailFileNotFound  = "EMAIL_FILE_NOT_FOUND"
	ErrorCodeNoEmailsFound      = "NO_EMAILS_FOUND"
	ErrorCodeNoEmailsToExport   = "NO_EMAILS_TO_EXPORT"
	ErrorCodeInvalidEmailID     = "INVALID_EMAIL_ID"
	ErrorCodeNoEmailIDsProvided = "NO_EMAIL_IDS_PROVIDED"

	// Request errors
	ErrorCodeInvalidRequest      = "INVALID_REQUEST"
	ErrorCodeInvalidEmailAddress = "INVALID_EMAIL_ADDRESS"
	ErrorCodeHostRequired        = "HOST_REQUIRED"
	ErrorCodePortOutOfRange      = "PORT_OUT_OF_RANGE"
	ErrorCodeInvalidPort         = "INVALID_PORT"

	// Relay errors
	ErrorCodeRelayFailed = "RELAY_FAILED"

	// Success messages (also use codes for consistency)
	SuccessCodeEmailDeleted         = "EMAIL_DELETED"
	SuccessCodeAllEmailsDeleted     = "ALL_EMAILS_DELETED"
	SuccessCodeEmailMarkedRead      = "EMAIL_MARKED_READ"
	SuccessCodeAllEmailsMarkedRead  = "ALL_EMAILS_MARKED_READ"
	SuccessCodeEmailRelayed         = "EMAIL_RELAYED"
	SuccessCodeMailsReloaded        = "MAILS_RELOADED"
	SuccessCodeBatchDeleteCompleted = "BATCH_DELETE_COMPLETED"
	SuccessCodeBatchReadCompleted   = "BATCH_READ_COMPLETED"
	SuccessCodeConfigUpdated        = "CONFIG_UPDATED"
)

// APIResponse represents a standardized API response
type APIResponse struct {
	Code    string      `json:"code,omitempty"`    // Error or success code
	Message string      `json:"message,omitempty"` // Optional: English message for backward compatibility
	Data    interface{} `json:"data,omitempty"`    // Response data
	Error   string      `json:"error,omitempty"`   // Error code (for error responses)
}

// ErrorResponse creates an error response with code
func ErrorResponse(code string, message string) APIResponse {
	return APIResponse{
		Code:    code,
		Error:   code,
		Message: message, // Keep for backward compatibility
	}
}

// SuccessResponse creates a success response with code
func SuccessResponse(code string, message string, data interface{}) APIResponse {
	resp := APIResponse{
		Code:    code,
		Message: message, // Keep for backward compatibility
		Data:    data,
	}
	return resp
}
