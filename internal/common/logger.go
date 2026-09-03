package common

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	logger "github.com/soulteary/logger-kit/v2"
)

// LogLevel represents the logging level (owlmail legacy: silent / normal / verbose).
type LogLevel int

const (
	// LogLevelSilent suppresses all logs except errors
	LogLevelSilent LogLevel = iota
	// LogLevelNormal shows normal logs
	LogLevelNormal
	// LogLevelVerbose shows detailed logs
	LogLevelVerbose
)

var (
	defaultLog   *logger.Logger
	defaultLogMu sync.RWMutex
)

// owlmailToLoggerLevel maps owlmail LogLevel to logger-kit Level.
func owlmailToLoggerLevel(l LogLevel) logger.Level {
	switch l {
	case LogLevelSilent:
		return logger.ErrorLevel // only errors in silent mode
	case LogLevelNormal:
		return logger.InfoLevel
	case LogLevelVerbose:
		return logger.DebugLevel
	default:
		return logger.InfoLevel
	}
}

// getLoggerUnsafe returns the current logger. Caller must hold defaultLogMu (RLock or Lock).
func getLoggerUnsafe() *logger.Logger {
	if defaultLog != nil {
		return defaultLog
	}
	return logger.Default()
}

// InitLogger initializes the global logger. Holds the lock for the whole call so it does
// not race with Log/Verbose/Error: logger.New() writes zerolog globals, and Msg() reads them.
func InitLogger(level LogLevel) {
	InitLoggerWithFormat(level, "console")
}

// InitLoggerOutput initializes the global console logger on a selected stream.
// Stdio protocols use stderr so logs cannot corrupt protocol messages.
func InitLoggerOutput(level LogLevel, output io.Writer) {
	initLogger(level, "console", output)
}

// InitLoggerWithFormat initializes the global logger in console or JSON form.
func InitLoggerWithFormat(level LogLevel, format string) {
	initLogger(level, format, os.Stdout)
}

func initLogger(level LogLevel, format string, output io.Writer) {
	defaultLogMu.Lock()
	defer defaultLogMu.Unlock()
	kitLevel := owlmailToLoggerLevel(level)
	kitFormat := logger.FormatConsole
	if strings.EqualFold(strings.TrimSpace(format), "json") {
		kitFormat = logger.FormatJSON
	}
	l := logger.New(logger.Config{
		Level:       kitLevel,
		Output:      output,
		Format:      kitFormat,
		ServiceName: "owlmail",
	})
	defaultLog = l
}

// Log prints a log message at info level (normal and verbose).
func Log(format string, v ...interface{}) {
	defaultLogMu.RLock()
	l := getLoggerUnsafe()
	msg := fmt.Sprintf(format, v...)
	l.Info().Msg(msg)
	defaultLogMu.RUnlock()
}

// Verbose prints a log message at debug level (verbose only).
func Verbose(format string, v ...interface{}) {
	defaultLogMu.RLock()
	l := getLoggerUnsafe()
	msg := fmt.Sprintf(format, v...)
	l.Debug().Msg(msg)
	defaultLogMu.RUnlock()
}

// Error prints an error message (always shown).
func Error(format string, v ...interface{}) {
	defaultLogMu.RLock()
	l := getLoggerUnsafe()
	msg := fmt.Sprintf(format, v...)
	l.Error().Msg(msg)
	defaultLogMu.RUnlock()
}

// Fatal logs a fatal error via the error handler (returns error in tests, exits in production).
func Fatal(format string, v ...interface{}) error {
	defaultLogMu.RLock()
	l := getLoggerUnsafe()
	msg := fmt.Sprintf(format, v...)
	l.Error().Msg(msg)
	defaultLogMu.RUnlock()
	return GetErrorHandler().Fatal(format, v...)
}
