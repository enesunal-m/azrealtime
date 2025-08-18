package azrealtime

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
)

func TestLogLevel_String(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{LogLevelDebug, "DEBUG"},
		{LogLevelInfo, "INFO"},
		{LogLevelWarn, "WARN"},
		{LogLevelError, "ERROR"},
		{LogLevelOff, "OFF"},
		{LogLevel(999), "UNKNOWN"},
	}

	for _, test := range tests {
		if got := test.level.String(); got != test.expected {
			t.Errorf("LogLevel(%d).String() = %q, want %q", test.level, got, test.expected)
		}
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected LogLevel
	}{
		{"DEBUG", LogLevelDebug},
		{"debug", LogLevelDebug},
		{"INFO", LogLevelInfo},
		{"info", LogLevelInfo},
		{"WARN", LogLevelWarn},
		{"WARNING", LogLevelWarn},
		{"warn", LogLevelWarn},
		{"ERROR", LogLevelError},
		{"error", LogLevelError},
		{"OFF", LogLevelOff},
		{"off", LogLevelOff},
		{"invalid", LogLevelInfo}, // default
		{"", LogLevelInfo},        // default
	}

	for _, test := range tests {
		if got := ParseLogLevel(test.input); got != test.expected {
			t.Errorf("ParseLogLevel(%q) = %v, want %v", test.input, got, test.expected)
		}
	}
}

func TestNewLogger(t *testing.T) {
	logger := NewLogger(LogLevelDebug)
	if logger.level != LogLevelDebug {
		t.Errorf("NewLogger(LogLevelDebug).level = %v, want %v", logger.level, LogLevelDebug)
	}
	if logger.prefix != "[azrealtime]" {
		t.Errorf("NewLogger().prefix = %q, want %q", logger.prefix, "[azrealtime]")
	}
}

func TestNewLoggerFromEnv(t *testing.T) {
	// Test with environment variable set
	os.Setenv("AZREALTIME_LOG_LEVEL", "ERROR")
	defer os.Unsetenv("AZREALTIME_LOG_LEVEL")

	logger := NewLoggerFromEnv()
	if logger.level != LogLevelError {
		t.Errorf("NewLoggerFromEnv() with ERROR env = %v, want %v", logger.level, LogLevelError)
	}

	// Test without environment variable (should default to INFO)
	os.Unsetenv("AZREALTIME_LOG_LEVEL")
	logger = NewLoggerFromEnv()
	if logger.level != LogLevelInfo {
		t.Errorf("NewLoggerFromEnv() without env = %v, want %v", logger.level, LogLevelInfo)
	}
}

func TestLogger_SetLevel(t *testing.T) {
	logger := NewLogger(LogLevelInfo)
	logger.SetLevel(LogLevelError)
	if logger.level != LogLevelError {
		t.Errorf("After SetLevel(LogLevelError), level = %v, want %v", logger.level, LogLevelError)
	}
}

func TestLogger_SetPrefix(t *testing.T) {
	logger := NewLogger(LogLevelInfo)
	logger.SetPrefix("[test]")
	if logger.prefix != "[test]" {
		t.Errorf("After SetPrefix([test]), prefix = %q, want %q", logger.prefix, "[test]")
	}
}

func TestLogger_LoggingLevels(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(LogLevelWarn)
	logger.logger = log.New(&buf, "", 0) // Remove timestamps for testing

	// These should not log (below threshold)
	logger.Debug("debug event", map[string]interface{}{"key": "value"})
	logger.Info("info event", nil)

	// These should log (at or above threshold)
	logger.Warn("warn event", map[string]interface{}{"level": "warning"})
	logger.Error("error event", map[string]interface{}{"code": 500})

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != 2 {
		t.Fatalf("Expected 2 log lines, got %d: %q", len(lines), output)
	}

	// Check warn message
	if !strings.Contains(lines[0], "[WARN] warn event level=warning") {
		t.Errorf("Warn log doesn't match expected format: %q", lines[0])
	}

	// Check error message
	if !strings.Contains(lines[1], "[ERROR] error event code=500") {
		t.Errorf("Error log doesn't match expected format: %q", lines[1])
	}
}

func TestLogger_LoggerFunc(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(LogLevelDebug)
	logger.logger = log.New(&buf, "", 0)

	loggerFunc := logger.LoggerFunc()
	loggerFunc("test event", map[string]interface{}{"test": true})

	output := buf.String()
	if !strings.Contains(output, "[INFO] test event test=true") {
		t.Errorf("LoggerFunc output doesn't match expected format: %q", output)
	}
}

func TestDefaultLogger(t *testing.T) {
	// Test that default logger functions work
	var buf bytes.Buffer
	originalLogger := DefaultLogger.logger
	DefaultLogger.logger = log.New(&buf, "", 0)
	DefaultLogger.SetLevel(LogLevelDebug)

	defer func() {
		DefaultLogger.logger = originalLogger
		DefaultLogger.SetLevel(LogLevelInfo)
	}()

	LogDebug("debug test", map[string]interface{}{"debug": true})
	LogInfo("info test", map[string]interface{}{"info": true})
	LogWarn("warn test", map[string]interface{}{"warn": true})
	LogError("error test", map[string]interface{}{"error": true})

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != 4 {
		t.Fatalf("Expected 4 log lines, got %d", len(lines))
	}

	expectedLevels := []string{"DEBUG", "INFO", "WARN", "ERROR"}
	for i, level := range expectedLevels {
		if !strings.Contains(lines[i], fmt.Sprintf("[%s]", level)) {
			t.Errorf("Line %d doesn't contain [%s]: %q", i, level, lines[i])
		}
	}
}

func TestContextualLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(LogLevelDebug)
	logger.logger = log.New(&buf, "", 0)

	contextLogger := logger.WithContext(map[string]interface{}{
		"session_id": "test-session",
		"client_id":  "test-client",
	})

	contextLogger.Info("test event", map[string]interface{}{
		"action": "connect",
		"status": "success",
	})

	output := buf.String()

	// Check that all fields are present
	expectedFields := []string{"session_id=test-session", "client_id=test-client", "action=connect", "status=success"}
	for _, field := range expectedFields {
		if !strings.Contains(output, field) {
			t.Errorf("Output doesn't contain expected field %q: %q", field, output)
		}
	}
}

func TestContextualLogger_FieldOverride(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(LogLevelDebug)
	logger.logger = log.New(&buf, "", 0)

	contextLogger := logger.WithContext(map[string]interface{}{
		"key": "context-value",
	})

	// Message fields should override context fields
	contextLogger.Info("test event", map[string]interface{}{
		"key": "message-value",
	})

	output := buf.String()

	// Should contain the message value, not context value
	if !strings.Contains(output, "key=message-value") {
		t.Errorf("Expected message field to override context field: %q", output)
	}
	if strings.Contains(output, "key=context-value") {
		t.Errorf("Context field should have been overridden: %q", output)
	}
}

func TestLogger_EmptyFields(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(LogLevelInfo)
	logger.logger = log.New(&buf, "", 0)

	logger.Info("test event", nil)
	logger.Info("test event 2", map[string]interface{}{})

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	for _, line := range lines {
		if !strings.Contains(line, "[INFO] test event") {
			t.Errorf("Line doesn't match expected format: %q", line)
		}
		// Should not have extra spaces for empty fields
		if strings.Contains(line, "  ") {
			t.Errorf("Line contains extra spaces: %q", line)
		}
	}
}
