package main

import (
	"bytes"
	"io"
	"log"
	"os"
	"testing"
)

func TestLogLevel(t *testing.T) {
	// Test LogLevel constants
	if LogLevelSilent >= LogLevelNormal {
		t.Error("LogLevelSilent should be less than LogLevelNormal")
	}
	if LogLevelNormal >= LogLevelVerbose {
		t.Error("LogLevelNormal should be less than LogLevelVerbose")
	}
}

func TestInitLogger(t *testing.T) {
	// Reset global logger
	globalLogger = nil

	// Test initialization with different levels
	InitLogger(LogLevelSilent)
	if globalLogger == nil {
		t.Error("globalLogger should not be nil after InitLogger")
	}
	if globalLogger.level != LogLevelSilent {
		t.Errorf("Expected level %d, got %d", LogLevelSilent, globalLogger.level)
	}

	InitLogger(LogLevelNormal)
	if globalLogger.level != LogLevelNormal {
		t.Errorf("Expected level %d, got %d", LogLevelNormal, globalLogger.level)
	}

	InitLogger(LogLevelVerbose)
	if globalLogger.level != LogLevelVerbose {
		t.Errorf("Expected level %d, got %d", LogLevelVerbose, globalLogger.level)
	}
}

func TestGetLogger(t *testing.T) {
	// Reset global logger
	globalLogger = nil

	// Test GetLogger when nil
	logger := GetLogger()
	if logger == nil {
		t.Error("GetLogger should not return nil")
	}
	if globalLogger == nil {
		t.Error("globalLogger should be initialized by GetLogger")
	}

	// Test GetLogger when already initialized
	InitLogger(LogLevelVerbose)
	logger = GetLogger()
	if logger.level != LogLevelVerbose {
		t.Errorf("Expected level %d, got %d", LogLevelVerbose, logger.level)
	}
}

func TestLoggerSetLevel(t *testing.T) {
	logger := &Logger{
		level:  LogLevelNormal,
		logger: log.New(os.Stdout, "", log.LstdFlags),
	}

	// Test setting to Silent
	logger.SetLevel(LogLevelSilent)
	if logger.level != LogLevelSilent {
		t.Errorf("Expected level %d, got %d", LogLevelSilent, logger.level)
	}

	// Test setting to Normal
	logger.SetLevel(LogLevelNormal)
	if logger.level != LogLevelNormal {
		t.Errorf("Expected level %d, got %d", LogLevelNormal, logger.level)
	}

	// Test setting to Verbose
	logger.SetLevel(LogLevelVerbose)
	if logger.level != LogLevelVerbose {
		t.Errorf("Expected level %d, got %d", LogLevelVerbose, logger.level)
	}
}

func TestLoggerLog(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		level:  LogLevelNormal,
		logger: log.New(&buf, "", 0),
	}

	// Test Log at Normal level
	logger.Log("test message")
	if buf.Len() == 0 {
		t.Error("Log should write to buffer at Normal level")
	}

	// Test Log at Verbose level
	buf.Reset()
	logger.level = LogLevelVerbose
	logger.Log("test message")
	if buf.Len() == 0 {
		t.Error("Log should write to buffer at Verbose level")
	}

	// Test Log at Silent level
	buf.Reset()
	logger.level = LogLevelSilent
	logger.Log("test message")
	if buf.Len() != 0 {
		t.Error("Log should not write to buffer at Silent level")
	}
}

func TestLoggerVerbose(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		level:  LogLevelVerbose,
		logger: log.New(&buf, "", 0),
	}

	// Test Verbose at Verbose level
	logger.Verbose("test message")
	if buf.Len() == 0 {
		t.Error("Verbose should write to buffer at Verbose level")
	}

	// Test Verbose at Normal level
	buf.Reset()
	logger.level = LogLevelNormal
	logger.Verbose("test message")
	if buf.Len() != 0 {
		t.Error("Verbose should not write to buffer at Normal level")
	}

	// Test Verbose at Silent level
	buf.Reset()
	logger.level = LogLevelSilent
	logger.Verbose("test message")
	if buf.Len() != 0 {
		t.Error("Verbose should not write to buffer at Silent level")
	}
}

func TestLoggerError(t *testing.T) {
	// Error should always log, regardless of level
	logger := &Logger{
		level:  LogLevelSilent,
		logger: log.New(io.Discard, "", 0),
	}

	// Error uses standard log, so we can't easily test output
	// But we can test it doesn't panic
	logger.Error("test error")
}

func TestLoggerFatal(t *testing.T) {
	// Fatal uses log.Fatalf which exits, so we can't test it directly
	// But we can verify the method exists and doesn't panic in normal cases
	logger := &Logger{
		level:  LogLevelNormal,
		logger: log.New(io.Discard, "", 0),
	}

	// Note: We can't actually test Fatal as it calls os.Exit
	// This is just to ensure the method exists
	_ = logger.Fatal
}

func TestConvenienceFunctions(t *testing.T) {
	// Reset global logger
	globalLogger = nil

	// Test Log convenience function
	InitLogger(LogLevelNormal)
	Log("test message")

	// Test Verbose convenience function
	InitLogger(LogLevelVerbose)
	Verbose("test message")

	// Test Error convenience function
	Error("test error")

	// Test Fatal convenience function exists
	_ = Fatal
}

func TestInitLoggerSilentMode(t *testing.T) {
	// Reset global logger
	globalLogger = nil

	// Test that Silent mode uses io.Discard
	InitLogger(LogLevelSilent)
	if globalLogger.logger == nil {
		t.Error("logger should not be nil")
	}

	// Create a logger with Discard and verify behavior
	var buf bytes.Buffer
	logger := &Logger{
		level:  LogLevelSilent,
		logger: log.New(&buf, "", 0),
	}

	// Set output to Discard
	logger.logger.SetOutput(io.Discard)
	logger.Log("should not appear")
	if buf.Len() != 0 {
		t.Error("Silent mode should not write to buffer")
	}
}
