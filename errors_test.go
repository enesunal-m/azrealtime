package azrealtime

import (
	"errors"
	"testing"
	"time"
)

func TestConfigError(t *testing.T) {
	tests := []struct {
		name          string
		field         string
		value         string
		message       string
		expectedError string
	}{
		{
			name:          "with value",
			field:         "ResourceEndpoint",
			value:         "invalid-url",
			message:       "invalid URL format",
			expectedError: `azrealtime: invalid config field "ResourceEndpoint" (value: "invalid-url"): invalid URL format`,
		},
		{
			name:          "without value",
			field:         "Deployment",
			value:         "",
			message:       "cannot be empty",
			expectedError: `azrealtime: invalid config field "Deployment": cannot be empty`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewConfigError(tt.field, tt.value, tt.message)

			if err.Error() != tt.expectedError {
				t.Errorf("expected error %q, got %q", tt.expectedError, err.Error())
			}

			// Test error matching
			if !errors.Is(err, ErrInvalidConfig) {
				t.Error("ConfigError should match ErrInvalidConfig")
			}
		})
	}
}

func TestConnectionError(t *testing.T) {
	underlyingErr := errors.New("network unreachable")
	url := "wss://test.openai.azure.com/openai/realtime"
	operation := "dial"

	err := NewConnectionError(url, operation, underlyingErr)

	expectedError := `azrealtime: dial failed for "wss://test.openai.azure.com/openai/realtime": network unreachable`
	if err.Error() != expectedError {
		t.Errorf("expected error %q, got %q", expectedError, err.Error())
	}

	// Test error unwrapping
	if !errors.Is(err, underlyingErr) {
		t.Error("ConnectionError should unwrap to underlying error")
	}

	// Test error matching
	if !errors.Is(err, ErrConnectionFailed) {
		t.Error("ConnectionError should match ErrConnectionFailed")
	}
}

func TestSendError(t *testing.T) {
	underlyingErr := errors.New("write timeout")
	eventType := "session.update"
	eventID := "evt_123"

	err := NewSendError(eventType, eventID, underlyingErr)

	expectedError := `azrealtime: failed to send session.update event "evt_123": write timeout`
	if err.Error() != expectedError {
		t.Errorf("expected error %q, got %q", expectedError, err.Error())
	}

	// Test error unwrapping
	if !errors.Is(err, underlyingErr) {
		t.Error("SendError should unwrap to underlying error")
	}

	// Test without event ID
	errNoID := NewSendError(eventType, "", underlyingErr)
	expectedNoID := `azrealtime: failed to send session.update event: write timeout`
	if errNoID.Error() != expectedNoID {
		t.Errorf("expected error %q, got %q", expectedNoID, errNoID.Error())
	}
}

func TestSendError_IsTimeout(t *testing.T) {
	// Test with timeout error
	timeoutErr := NewSendError("test", "", ErrSendTimeout)
	if !timeoutErr.IsTimeout() {
		t.Error("expected IsTimeout() to return true for timeout error")
	}

	// Test with non-timeout error
	otherErr := NewSendError("test", "", errors.New("other error"))
	if otherErr.IsTimeout() {
		t.Error("expected IsTimeout() to return false for non-timeout error")
	}
}

func TestEventError(t *testing.T) {
	underlyingErr := errors.New("json: invalid character")
	eventType := "response.text.delta"
	rawData := []byte(`{"invalid": json}`)

	err := NewEventError(eventType, rawData, underlyingErr)

	expectedError := `azrealtime: failed to process response.text.delta event: json: invalid character`
	if err.Error() != expectedError {
		t.Errorf("expected error %q, got %q", expectedError, err.Error())
	}

	// Test error unwrapping
	if !errors.Is(err, underlyingErr) {
		t.Error("EventError should unwrap to underlying error")
	}

	// Test error matching
	if !errors.Is(err, ErrInvalidEventData) {
		t.Error("EventError should match ErrInvalidEventData")
	}

	// Check raw data is preserved
	if string(err.RawData) != string(rawData) {
		t.Error("EventError should preserve raw data")
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
		errorField  string
	}{
		{
			name: "valid config",
			config: Config{
				ResourceEndpoint: "https://test.openai.azure.com",
				Deployment:       "test-deployment",
				APIVersion:       "2025-04-01-preview",
				Credential:       APIKey("test-key"),
				DialTimeout:      15 * time.Second,
			},
			expectError: false,
		},
		{
			name: "empty resource endpoint",
			config: Config{
				Deployment: "test-deployment",
				APIVersion: "2025-04-01-preview",
				Credential: APIKey("test-key"),
			},
			expectError: true,
			errorField:  "ResourceEndpoint",
		},
		{
			name: "invalid resource endpoint URL",
			config: Config{
				ResourceEndpoint: "://invalid-url", // This will actually fail parsing
				Deployment:       "test-deployment",
				APIVersion:       "2025-04-01-preview",
				Credential:       APIKey("test-key"),
			},
			expectError: true,
			errorField:  "ResourceEndpoint",
		},
		{
			name: "empty deployment",
			config: Config{
				ResourceEndpoint: "https://test.openai.azure.com",
				APIVersion:       "2025-04-01-preview",
				Credential:       APIKey("test-key"),
			},
			expectError: true,
			errorField:  "Deployment",
		},
		{
			name: "empty API version",
			config: Config{
				ResourceEndpoint: "https://test.openai.azure.com",
				Deployment:       "test-deployment",
				Credential:       APIKey("test-key"),
			},
			expectError: true,
			errorField:  "APIVersion",
		},
		{
			name: "nil credential",
			config: Config{
				ResourceEndpoint: "https://test.openai.azure.com",
				Deployment:       "test-deployment",
				APIVersion:       "2025-04-01-preview",
			},
			expectError: true,
			errorField:  "Credential",
		},
		{
			name: "negative dial timeout",
			config: Config{
				ResourceEndpoint: "https://test.openai.azure.com",
				Deployment:       "test-deployment",
				APIVersion:       "2025-04-01-preview",
				Credential:       APIKey("test-key"),
				DialTimeout:      -1 * time.Second,
			},
			expectError: true,
			errorField:  "DialTimeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.config)

			if tt.expectError {
				if err == nil {
					t.Error("expected validation error but got nil")
					return
				}

				// Check if it's a ConfigError with the expected field
				var configErr *ConfigError
				if !errors.As(err, &configErr) {
					t.Errorf("expected ConfigError, got %T", err)
					return
				}

				if configErr.Field != tt.errorField {
					t.Errorf("expected error field %q, got %q", tt.errorField, configErr.Field)
				}

				// Test error matching
				if !errors.Is(err, ErrInvalidConfig) {
					t.Error("validation error should match ErrInvalidConfig")
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestErrorConstants(t *testing.T) {
	// Test that all error constants are defined
	errors := []error{
		ErrClosed,
		ErrInvalidConfig,
		ErrConnectionFailed,
		ErrSendTimeout,
		ErrInvalidEventData,
	}

	for i, err := range errors {
		if err == nil {
			t.Errorf("error constant at index %d is nil", i)
		}
		if err.Error() == "" {
			t.Errorf("error constant at index %d has empty message", i)
		}
	}
}

func BenchmarkConfigError_Error(b *testing.B) {
	err := NewConfigError("ResourceEndpoint", "https://test.example.com", "invalid format")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = err.Error()
	}
}

func BenchmarkValidateConfig(b *testing.B) {
	config := Config{
		ResourceEndpoint: "https://test.openai.azure.com",
		Deployment:       "test-deployment",
		APIVersion:       "2025-04-01-preview",
		Credential:       APIKey("test-key"),
		DialTimeout:      15 * time.Second,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateConfig(config)
	}
}
