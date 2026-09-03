package common

import (
	"fmt"
	"os"
)

// ErrorHandler defines the error handling interface
type ErrorHandler interface {
	Fatal(format string, v ...interface{}) error
}

// DefaultErrorHandler is the default error handler for production environments
type DefaultErrorHandler struct{}

var exitProcess = os.Exit

func (h *DefaultErrorHandler) Fatal(format string, v ...interface{}) error {
	msg := fmt.Sprintf(format, v...)
	exitProcess(1)
	// Test substitutes may return; production os.Exit never does.
	return fmt.Errorf("[FATAL] %s", msg)
}

// TestErrorHandler is a test error handler for testing environments
type TestErrorHandler struct {
	LastError error
}

func (h *TestErrorHandler) Fatal(format string, v ...interface{}) error {
	msg := fmt.Sprintf(format, v...)
	h.LastError = fmt.Errorf("[FATAL] %s", msg)
	return h.LastError
}

// Global error handler
var globalErrorHandler ErrorHandler = &DefaultErrorHandler{}

// SetErrorHandler sets the error handler (used for testing)
func SetErrorHandler(handler ErrorHandler) {
	globalErrorHandler = handler
}

// ResetErrorHandler resets to the default error handler
func ResetErrorHandler() {
	globalErrorHandler = &DefaultErrorHandler{}
}

// GetErrorHandler returns the global error handler
func GetErrorHandler() ErrorHandler {
	return globalErrorHandler
}
