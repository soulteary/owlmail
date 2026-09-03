package common

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestLogLevel(t *testing.T) {
	if LogLevelSilent >= LogLevelNormal {
		t.Error("LogLevelSilent should be less than LogLevelNormal")
	}
	if LogLevelNormal >= LogLevelVerbose {
		t.Error("LogLevelNormal should be less than LogLevelVerbose")
	}
}

func TestInitLogger(t *testing.T) {
	InitLogger(LogLevelSilent)
	InitLogger(LogLevelNormal)
	InitLogger(LogLevelVerbose)
	InitLoggerWithFormat(LogLevelNormal, "json")
	InitLoggerWithFormat(LogLevelNormal, "JSON")
	InitLoggerWithFormat(LogLevelNormal, "console")
}

func TestLog(t *testing.T) {
	InitLogger(LogLevelNormal)
	Log("test message")
}

func TestVerbose(t *testing.T) {
	InitLogger(LogLevelVerbose)
	Verbose("test message")
}

func TestError(t *testing.T) {
	InitLogger(LogLevelSilent)
	Error("test error")
}

func TestLogWithFormatting(t *testing.T) {
	InitLogger(LogLevelNormal)
	Log("test message: %s", "value")
	Log("test: %s %d", "value", 42)
}

func TestVerboseWithFormatting(t *testing.T) {
	InitLogger(LogLevelVerbose)
	Verbose("test message: %s", "value")
	Verbose("test: %s %d", "value", 42)
}

func TestErrorWithFormatting(t *testing.T) {
	Error("test error: %s", "value")
	Error("test error: %s %d", "value", 42)
}

func TestConvenienceFunctions(t *testing.T) {
	InitLogger(LogLevelNormal)
	Log("test message")

	InitLogger(LogLevelVerbose)
	Verbose("test message")

	Error("test error")
	_ = Fatal
}

func TestInitLoggerSilentMode(t *testing.T) {
	InitLogger(LogLevelSilent)
}

func TestInitLoggerOutputWithFormatUsesJSON(t *testing.T) {
	var output bytes.Buffer
	InitLoggerOutputWithFormat(LogLevelNormal, "json", &output)
	defer InitLogger(LogLevelNormal)
	Log("stdio diagnostic")

	var entry map[string]interface{}
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatalf("stderr log is not JSON: %q: %v", output.String(), err)
	}
	if entry["message"] != "stdio diagnostic" {
		t.Fatalf("JSON message = %#v", entry["message"])
	}
}

// TestGlobalFatalWithErrorHandler tests global Fatal function with error handler
func TestGlobalFatalWithErrorHandler(t *testing.T) {
	testHandler := &TestErrorHandler{}
	SetErrorHandler(testHandler)
	defer ResetErrorHandler()

	err := Fatal("test global fatal message")
	if err == nil {
		t.Error("Fatal should return an error")
	}

	if testHandler.LastError == nil {
		t.Error("Error handler should record the error")
	}

	if testHandler.LastError.Error() != err.Error() {
		t.Errorf("Expected error %v, got %v", testHandler.LastError, err)
	}
}

// TestDefaultErrorHandler tests default error handler behavior
func TestDefaultErrorHandler(t *testing.T) {
	handler := &DefaultErrorHandler{}
	var _ ErrorHandler = handler
	_ = handler
}

// TestTestErrorHandler tests test error handler
func TestTestErrorHandler(t *testing.T) {
	handler := &TestErrorHandler{}

	err := handler.Fatal("test message: %s", "error")
	if err == nil {
		t.Error("Fatal should return an error")
	}

	if handler.LastError == nil {
		t.Error("LastError should be set")
	}

	if handler.LastError != err {
		t.Error("LastError should equal returned error")
	}

	err2 := handler.Fatal("another message")
	if err2 == nil {
		t.Error("Fatal should return an error")
	}

	if handler.LastError != err2 {
		t.Error("LastError should be updated")
	}
}
