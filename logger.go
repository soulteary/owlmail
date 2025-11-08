package main

import (
	"io"
	"log"
	"os"
)

// LogLevel represents the logging level
type LogLevel int

const (
	// LogLevelSilent suppresses all logs except errors
	LogLevelSilent LogLevel = iota
	// LogLevelNormal shows normal logs
	LogLevelNormal
	// LogLevelVerbose shows detailed logs
	LogLevelVerbose
)

// Logger wraps the standard log package with level support
type Logger struct {
	level        LogLevel
	logger       *log.Logger
	errorHandler ErrorHandler // Error handler for fatal errors
}

var globalLogger *Logger

// InitLogger initializes the global logger with the specified level
func InitLogger(level LogLevel) {
	var output io.Writer = os.Stdout

	// In silent mode, discard all output except errors
	if level == LogLevelSilent {
		output = io.Discard
	}

	globalLogger = &Logger{
		level:  level,
		logger: log.New(output, "", log.LstdFlags),
	}
}

// GetLogger returns the global logger instance
func GetLogger() *Logger {
	if globalLogger == nil {
		InitLogger(LogLevelNormal)
	}
	return globalLogger
}

// SetLevel sets the logging level
func (l *Logger) SetLevel(level LogLevel) {
	l.level = level
	if level == LogLevelSilent {
		l.logger.SetOutput(io.Discard)
	} else {
		l.logger.SetOutput(os.Stdout)
	}
}

// Log prints a log message if level allows
func (l *Logger) Log(format string, v ...interface{}) {
	if l.level >= LogLevelNormal {
		l.logger.Printf(format, v...)
	}
}

// Verbose prints a verbose log message if verbose mode is enabled
func (l *Logger) Verbose(format string, v ...interface{}) {
	if l.level >= LogLevelVerbose {
		l.logger.Printf("[VERBOSE] "+format, v...)
	}
}

// Error prints an error message (always shown)
func (l *Logger) Error(format string, v ...interface{}) {
	// Errors are always shown, even in silent mode
	log.Printf("[ERROR] "+format, v...)
}

// Fatal logs a fatal error and exits
// Uses the error handler to return an error instead of exiting in test environments
func (l *Logger) Fatal(format string, v ...interface{}) error {
	if l.errorHandler != nil {
		return l.errorHandler.Fatal(format, v...)
	}
	// Fallback to global error handler
	return globalErrorHandler.Fatal(format, v...)
}

// Convenience functions for global logger
func Log(format string, v ...interface{}) {
	GetLogger().Log(format, v...)
}

func Verbose(format string, v ...interface{}) {
	GetLogger().Verbose(format, v...)
}

func Error(format string, v ...interface{}) {
	GetLogger().Error(format, v...)
}

func Fatal(format string, v ...interface{}) error {
	return GetLogger().Fatal(format, v...)
}
