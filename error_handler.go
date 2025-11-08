package main

import (
	"fmt"
	"log"
)

// ErrorHandler defines the error handling interface
type ErrorHandler interface {
	Fatal(format string, v ...interface{}) error
}

// DefaultErrorHandler is the default error handler for production environments
type DefaultErrorHandler struct{}

func (h *DefaultErrorHandler) Fatal(format string, v ...interface{}) error {
	msg := fmt.Sprintf(format, v...)
	log.Fatalf("[FATAL] %s", msg)
	// This line will never execute, but satisfies the interface requirement
	return fmt.Errorf("%s", msg)
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
