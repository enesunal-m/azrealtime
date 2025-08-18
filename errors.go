package azrealtime

import (
	"errors"
	"fmt"
	"net/url"
)

// Common error variables
var (
	// ErrClosed is returned when attempting to use a client that has been closed.
	// This error indicates that the WebSocket connection has been terminated and
	// the client is no longer usable. Create a new client to resume operations.
	ErrClosed = errors.New("azrealtime: connection is closed")

	// ErrInvalidConfig is returned when required configuration fields are missing.
	ErrInvalidConfig = errors.New("azrealtime: invalid configuration")

	// ErrConnectionFailed is returned when the WebSocket connection cannot be established.
	ErrConnectionFailed = errors.New("azrealtime: connection failed")

	// ErrSendTimeout is returned when sending a message times out.
	ErrSendTimeout = errors.New("azrealtime: send timeout")

	// ErrInvalidEventData is returned when event data cannot be parsed.
	ErrInvalidEventData = errors.New("azrealtime: invalid event data")
)

// ConfigError represents a configuration validation error.
// It provides detailed information about which configuration field is invalid.
type ConfigError struct {
	Field   string // The configuration field that is invalid
	Value   string // The invalid value (if safe to log)
	Message string // Detailed error message
}

func (e *ConfigError) Error() string {
	if e.Value != "" {
		return fmt.Sprintf("azrealtime: invalid config field %q (value: %q): %s", e.Field, e.Value, e.Message)
	}
	return fmt.Sprintf("azrealtime: invalid config field %q: %s", e.Field, e.Message)
}

// Is implements error matching for ConfigError.
func (e *ConfigError) Is(target error) bool {
	return target == ErrInvalidConfig
}

// ConnectionError represents a WebSocket connection error.
// It wraps underlying network errors with additional context.
type ConnectionError struct {
	URL       string // The WebSocket URL that failed to connect
	Cause     error  // The underlying error
	Operation string // The operation that failed (e.g., "dial", "handshake")
}

func (e *ConnectionError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("azrealtime: %s failed for %q: %v", e.Operation, e.URL, e.Cause)
	}
	return fmt.Sprintf("azrealtime: %s failed for %q", e.Operation, e.URL)
}

// Unwrap returns the underlying error for error unwrapping.
func (e *ConnectionError) Unwrap() error {
	return e.Cause
}

// Is implements error matching for ConnectionError.
func (e *ConnectionError) Is(target error) bool {
	return target == ErrConnectionFailed
}

// SendError represents an error that occurred while sending data to the API.
type SendError struct {
	EventType string        // The type of event being sent
	EventID   string        // The event ID (if available)
	Cause     error         // The underlying error
}

func (e *SendError) Error() string {
	if e.EventID != "" {
		return fmt.Sprintf("azrealtime: failed to send %s event %q: %v", e.EventType, e.EventID, e.Cause)
	}
	return fmt.Sprintf("azrealtime: failed to send %s event: %v", e.EventType, e.Cause)
}

// Unwrap returns the underlying error.
func (e *SendError) Unwrap() error {
	return e.Cause
}

// IsTimeout returns true if the error was caused by a timeout.
func (e *SendError) IsTimeout() bool {
	return errors.Is(e.Cause, ErrSendTimeout)
}

// EventError represents an error in processing an event from the API.
type EventError struct {
	EventType string // The type of event that caused the error
	RawData   []byte // The raw JSON data (if available)
	Cause     error  // The underlying parsing error
}

func (e *EventError) Error() string {
	return fmt.Sprintf("azrealtime: failed to process %s event: %v", e.EventType, e.Cause)
}

// Unwrap returns the underlying error.
func (e *EventError) Unwrap() error {
	return e.Cause
}

// Is implements error matching for EventError.
func (e *EventError) Is(target error) bool {
	return target == ErrInvalidEventData
}

// Helper functions for creating specific errors

// NewConfigError creates a new configuration error.
func NewConfigError(field, value, message string) *ConfigError {
	return &ConfigError{
		Field:   field,
		Value:   value,
		Message: message,
	}
}

// NewConnectionError creates a new connection error.
func NewConnectionError(url, operation string, cause error) *ConnectionError {
	return &ConnectionError{
		URL:       url,
		Operation: operation,
		Cause:     cause,
	}
}

// NewSendError creates a new send error.
func NewSendError(eventType, eventID string, cause error) *SendError {
	return &SendError{
		EventType: eventType,
		EventID:   eventID,
		Cause:     cause,
	}
}

// NewEventError creates a new event processing error.
func NewEventError(eventType string, rawData []byte, cause error) *EventError {
	return &EventError{
		EventType: eventType,
		RawData:   rawData,
		Cause:     cause,
	}
}

// Validation helper functions

// ValidateConfig performs comprehensive configuration validation.
func ValidateConfig(cfg Config) error {
	if cfg.ResourceEndpoint == "" {
		return NewConfigError("ResourceEndpoint", "", "cannot be empty")
	}

	// Validate ResourceEndpoint URL format
	if _, err := url.Parse(cfg.ResourceEndpoint); err != nil {
		return NewConfigError("ResourceEndpoint", cfg.ResourceEndpoint, "invalid URL format")
	}

	if cfg.Deployment == "" {
		return NewConfigError("Deployment", "", "cannot be empty")
	}

	if cfg.APIVersion == "" {
		return NewConfigError("APIVersion", "", "cannot be empty")
	}

	if cfg.Credential == nil {
		return NewConfigError("Credential", "", "cannot be nil")
	}

	// Validate DialTimeout if specified
	if cfg.DialTimeout < 0 {
		return NewConfigError("DialTimeout", cfg.DialTimeout.String(), "cannot be negative")
	}

	return nil
}
