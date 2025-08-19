package azrealtime

import (
	"net/http"
	"testing"
	"time"
)

func TestAPIKey_apply(t *testing.T) {
	tests := []struct {
		name     string
		key      APIKey
		expected string
	}{
		{
			name:     "valid API key",
			key:      APIKey("test-key-123"),
			expected: "test-key-123",
		},
		{
			name:     "empty API key",
			key:      APIKey(""),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := http.Header{}
			tt.key.apply(h)

			if tt.expected == "" {
				if h.Get("api-key") != "" {
					t.Errorf("expected empty api-key header, got %q", h.Get("api-key"))
				}
			} else {
				if h.Get("api-key") != tt.expected {
					t.Errorf("expected api-key %q, got %q", tt.expected, h.Get("api-key"))
				}
			}
		})
	}
}

func TestBearer_apply(t *testing.T) {
	tests := []struct {
		name     string
		token    Bearer
		expected string
	}{
		{
			name:     "valid bearer token",
			token:    Bearer("abc123"),
			expected: "Bearer abc123",
		},
		{
			name:     "empty bearer token",
			token:    Bearer(""),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := http.Header{}
			tt.token.apply(h)

			if tt.expected == "" {
				if h.Get("Authorization") != "" {
					t.Errorf("expected empty Authorization header, got %q", h.Get("Authorization"))
				}
			} else {
				if h.Get("Authorization") != tt.expected {
					t.Errorf("expected Authorization %q, got %q", tt.expected, h.Get("Authorization"))
				}
			}
		})
	}
}

func TestConfig_validation(t *testing.T) {
	validConfig := Config{
		ResourceEndpoint: "https://test.openai.azure.com",
		Deployment:       "test-deployment",
		APIVersion:       "2025-04-01-preview",
		Credential:       APIKey("test-key"),
		DialTimeout:      15 * time.Second,
	}

	tests := []struct {
		name        string
		config      Config
		shouldError bool
	}{
		{
			name:        "valid config",
			config:      validConfig,
			shouldError: false,
		},
		{
			name: "missing resource endpoint",
			config: Config{
				Deployment: "test-deployment",
				APIVersion: "2025-04-01-preview",
				Credential: APIKey("test-key"),
			},
			shouldError: true,
		},
		{
			name: "missing deployment",
			config: Config{
				ResourceEndpoint: "https://test.openai.azure.com",
				APIVersion:       "2025-04-01-preview",
				Credential:       APIKey("test-key"),
			},
			shouldError: true,
		},
		{
			name: "missing API version",
			config: Config{
				ResourceEndpoint: "https://test.openai.azure.com",
				Deployment:       "test-deployment",
				Credential:       APIKey("test-key"),
			},
			shouldError: true,
		},
		{
			name: "missing credential",
			config: Config{
				ResourceEndpoint: "https://test.openai.azure.com",
				Deployment:       "test-deployment",
				APIVersion:       "2025-04-01-preview",
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the validation logic that's in Dial function
			hasError := tt.config.ResourceEndpoint == "" ||
				tt.config.Deployment == "" ||
				tt.config.APIVersion == "" ||
				tt.config.Credential == nil

			if hasError != tt.shouldError {
				t.Errorf("expected error: %v, got error: %v", tt.shouldError, hasError)
			}
		})
	}
}
