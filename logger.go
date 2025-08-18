package azrealtime

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// LogLevel represents the severity level of a log message
type LogLevel int

const (
	// LogLevelDebug logs everything including detailed debugging information
	LogLevelDebug LogLevel = iota
	// LogLevelInfo logs informational messages and above
	LogLevelInfo
	// LogLevelWarn logs warnings and above
	LogLevelWarn
	// LogLevelError logs only errors
	LogLevelError
	// LogLevelOff disables all logging
	LogLevelOff
)

// String returns the string representation of a LogLevel
func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	case LogLevelOff:
		return "OFF"
	default:
		return "UNKNOWN"
	}
}

// ParseLogLevel converts a string to LogLevel
func ParseLogLevel(level string) LogLevel {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return LogLevelDebug
	case "INFO":
		return LogLevelInfo
	case "WARN", "WARNING":
		return LogLevelWarn
	case "ERROR":
		return LogLevelError
	case "OFF":
		return LogLevelOff
	default:
		return LogLevelInfo
	}
}

// Logger provides structured logging with configurable levels
type Logger struct {
	level  LogLevel
	prefix string
	logger *log.Logger
}

// NewLogger creates a new structured logger
func NewLogger(level LogLevel) *Logger {
	return &Logger{
		level:  level,
		prefix: "[azrealtime]",
		logger: log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds),
	}
}

// NewLoggerFromEnv creates a logger with level from AZREALTIME_LOG_LEVEL env var
func NewLoggerFromEnv() *Logger {
	level := ParseLogLevel(os.Getenv("AZREALTIME_LOG_LEVEL"))
	return NewLogger(level)
}

// SetLevel updates the logger's minimum level
func (l *Logger) SetLevel(level LogLevel) {
	l.level = level
}

// SetPrefix updates the logger's prefix
func (l *Logger) SetPrefix(prefix string) {
	l.prefix = prefix
}

// Debug logs debug-level messages
func (l *Logger) Debug(event string, fields map[string]interface{}) {
	l.log(LogLevelDebug, event, fields)
}

// Info logs info-level messages
func (l *Logger) Info(event string, fields map[string]interface{}) {
	l.log(LogLevelInfo, event, fields)
}

// Warn logs warning-level messages
func (l *Logger) Warn(event string, fields map[string]interface{}) {
	l.log(LogLevelWarn, event, fields)
}

// Error logs error-level messages
func (l *Logger) Error(event string, fields map[string]interface{}) {
	l.log(LogLevelError, event, fields)
}

// log is the internal logging method
func (l *Logger) log(level LogLevel, event string, fields map[string]interface{}) {
	if level < l.level {
		return
	}

	var fieldStrs []string
	for k, v := range fields {
		fieldStrs = append(fieldStrs, fmt.Sprintf("%s=%v", k, v))
	}

	fieldsStr := ""
	if len(fieldStrs) > 0 {
		fieldsStr = fmt.Sprintf(" %s", strings.Join(fieldStrs, " "))
	}

	message := fmt.Sprintf("%s [%s] %s%s", l.prefix, level.String(), event, fieldsStr)
	l.logger.Print(message)
}

// LoggerFunc creates a logger function compatible with the Config.Logger field
func (l *Logger) LoggerFunc() func(string, map[string]interface{}) {
	return func(event string, fields map[string]interface{}) {
		l.Info(event, fields)
	}
}

// DefaultLogger is the default logger instance used when no custom logger is provided
var DefaultLogger = NewLoggerFromEnv()

// LogDebug logs a debug message using the default logger
func LogDebug(event string, fields map[string]interface{}) {
	DefaultLogger.Debug(event, fields)
}

// LogInfo logs an info message using the default logger
func LogInfo(event string, fields map[string]interface{}) {
	DefaultLogger.Info(event, fields)
}

// LogWarn logs a warning message using the default logger
func LogWarn(event string, fields map[string]interface{}) {
	DefaultLogger.Warn(event, fields)
}

// LogError logs an error message using the default logger
func LogError(event string, fields map[string]interface{}) {
	DefaultLogger.Error(event, fields)
}

// contextualLogger wraps the base Logger with additional context
type contextualLogger struct {
	*Logger
	context map[string]interface{}
}

// WithContext returns a logger that includes additional context in all log messages
func (l *Logger) WithContext(context map[string]interface{}) *contextualLogger {
	return &contextualLogger{
		Logger:  l,
		context: context,
	}
}

// mergeFields combines the contextual fields with message-specific fields
func (cl *contextualLogger) mergeFields(fields map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{})
	
	// Add context fields first
	for k, v := range cl.context {
		merged[k] = v
	}
	
	// Add message fields (overrides context if same key)
	for k, v := range fields {
		merged[k] = v
	}
	
	return merged
}

// Debug logs debug-level messages with context
func (cl *contextualLogger) Debug(event string, fields map[string]interface{}) {
	cl.Logger.Debug(event, cl.mergeFields(fields))
}

// Info logs info-level messages with context
func (cl *contextualLogger) Info(event string, fields map[string]interface{}) {
	cl.Logger.Info(event, cl.mergeFields(fields))
}

// Warn logs warning-level messages with context
func (cl *contextualLogger) Warn(event string, fields map[string]interface{}) {
	cl.Logger.Warn(event, cl.mergeFields(fields))
}

// Error logs error-level messages with context
func (cl *contextualLogger) Error(event string, fields map[string]interface{}) {
	cl.Logger.Error(event, cl.mergeFields(fields))
}