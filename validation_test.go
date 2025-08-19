package azrealtime

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestAppendPCM16_Validation(t *testing.T) {
	mockServer := NewMockServer(t)
	defer mockServer.Close()

	config := CreateMockConfig(mockServer.URL())
	ctx := context.Background()

	client, err := Dial(ctx, config)
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	defer client.Close()

	tests := []struct {
		name        string
		ctx         context.Context
		data        []byte
		expectError bool
		errorMsg    string
	}{
		{
			name:        "nil context",
			ctx:         nil,
			data:        []byte{0x00, 0x01},
			expectError: true,
			errorMsg:    "context cannot be nil",
		},
		{
			name:        "empty data",
			ctx:         ctx,
			data:        []byte{},
			expectError: false,
		},
		{
			name:        "valid PCM16 data",
			ctx:         ctx,
			data:        []byte{0x00, 0x01, 0xFF, 0xFE}, // 2 samples
			expectError: false,
		},
		{
			name:        "odd number of bytes",
			ctx:         ctx,
			data:        []byte{0x00, 0x01, 0xFF}, // Invalid for 16-bit samples
			expectError: true,
			errorMsg:    "PCM16 data must have even number of bytes",
		},
		{
			name:        "data too large",
			ctx:         ctx,
			data:        make([]byte, 2*1024*1024), // 2MB > 1MB limit
			expectError: true,
			errorMsg:    "PCM data too large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.AppendPCM16(tt.ctx, tt.data)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestValidateSession(t *testing.T) {
	tests := []struct {
		name        string
		session     Session
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid session",
			session: Session{
				Voice:             Ptr("alloy"),
				InputAudioFormat:  Ptr("pcm16"),
				OutputAudioFormat: Ptr("pcm16"),
				Instructions:      Ptr("You are helpful"),
				TurnDetection: &TurnDetection{
					Type:              "server_vad",
					Threshold:         0.5,
					PrefixPaddingMS:   300,
					SilenceDurationMS: 200,
					CreateResponse:    true,
				},
			},
			expectError: false,
		},
		{
			name: "invalid voice",
			session: Session{
				Voice: Ptr("invalid_voice"),
			},
			expectError: true,
			errorMsg:    "invalid voice",
		},
		{
			name: "invalid input audio format",
			session: Session{
				InputAudioFormat: Ptr("mp3"),
			},
			expectError: true,
			errorMsg:    "invalid input audio format",
		},
		{
			name: "invalid output audio format",
			session: Session{
				OutputAudioFormat: Ptr("wav"),
			},
			expectError: true,
			errorMsg:    "invalid output audio format",
		},
		{
			name: "empty turn detection type",
			session: Session{
				TurnDetection: &TurnDetection{
					Type: "",
				},
			},
			expectError: true,
			errorMsg:    "turn detection type cannot be empty",
		},
		{
			name: "invalid turn detection type",
			session: Session{
				TurnDetection: &TurnDetection{
					Type: "client_vad",
				},
			},
			expectError: true,
			errorMsg:    "invalid turn detection type",
		},
		{
			name: "invalid threshold",
			session: Session{
				TurnDetection: &TurnDetection{
					Type:      "server_vad",
					Threshold: 1.5, // > 1.0
				},
			},
			expectError: true,
			errorMsg:    "turn detection threshold must be between 0.0 and 1.0",
		},
		{
			name: "negative prefix padding",
			session: Session{
				TurnDetection: &TurnDetection{
					Type:            "server_vad",
					PrefixPaddingMS: -100,
				},
			},
			expectError: true,
			errorMsg:    "prefix padding must be non-negative",
		},
		{
			name: "negative silence duration",
			session: Session{
				TurnDetection: &TurnDetection{
					Type:              "server_vad",
					SilenceDurationMS: -50,
				},
			},
			expectError: true,
			errorMsg:    "silence duration must be non-negative",
		},
		{
			name: "instructions too long",
			session: Session{
				Instructions: Ptr(strings.Repeat("a", 10001)),
			},
			expectError: true,
			errorMsg:    "instructions too long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSession(tt.session)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestValidateCreateResponseOptions(t *testing.T) {
	tests := []struct {
		name        string
		opts        CreateResponseOptions
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid options",
			opts: CreateResponseOptions{
				Modalities:   []string{"text", "audio"},
				Prompt:       "Hello world",
				Instructions: "Be helpful",
				Temperature:  0.7,
				Conversation: "conv_123",
			},
			expectError: false,
		},
		{
			name: "invalid modality",
			opts: CreateResponseOptions{
				Modalities: []string{"video"}, // Invalid
			},
			expectError: true,
			errorMsg:    "invalid modality",
		},
		{
			name: "temperature too high",
			opts: CreateResponseOptions{
				Temperature: 3.0, // > 2.0
			},
			expectError: true,
			errorMsg:    "temperature must be between 0.0 and 2.0",
		},
		{
			name: "temperature too low",
			opts: CreateResponseOptions{
				Temperature: -0.1, // < 0.0
			},
			expectError: true,
			errorMsg:    "temperature must be between 0.0 and 2.0",
		},
		{
			name: "prompt too long",
			opts: CreateResponseOptions{
				Prompt: strings.Repeat("a", 10001),
			},
			expectError: true,
			errorMsg:    "prompt too long",
		},
		{
			name: "instructions too long",
			opts: CreateResponseOptions{
				Instructions: strings.Repeat("a", 10001),
			},
			expectError: true,
			errorMsg:    "instructions too long",
		},
		{
			name: "conversation ID too long",
			opts: CreateResponseOptions{
				Conversation: strings.Repeat("a", 101),
			},
			expectError: true,
			errorMsg:    "conversation ID too long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCreateResponseOptions(tt.opts)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestValidateConfig_Enhanced(t *testing.T) {
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
			name: "invalid URL scheme",
			config: Config{
				ResourceEndpoint: "ftp://test.openai.azure.com",
				Deployment:       "test-deployment",
				APIVersion:       "2025-04-01-preview",
				Credential:       APIKey("test-key"),
			},
			expectError: false, // URL parsing succeeds, scheme validation is not enforced
		},
		{
			name: "malformed URL",
			config: Config{
				ResourceEndpoint: "://invalid-url",
				Deployment:       "test-deployment",
				APIVersion:       "2025-04-01-preview",
				Credential:       APIKey("test-key"),
			},
			expectError: true,
			errorField:  "ResourceEndpoint",
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

				var configErr *ConfigError
				if !ErrorAs(err, &configErr) {
					t.Errorf("expected ConfigError, got %T", err)
					return
				}

				if configErr.Field != tt.errorField {
					t.Errorf("expected error field %q, got %q", tt.errorField, configErr.Field)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
			}
		})
	}
}

// Helper function for error type assertions (Go 1.13+ compatibility)
func ErrorAs(err error, target interface{}) bool {
	// Simple type assertion for our test
	switch target := target.(type) {
	case **ConfigError:
		if configErr, ok := err.(*ConfigError); ok {
			*target = configErr
			return true
		}
	}
	return false
}

func BenchmarkValidateSession(b *testing.B) {
	session := Session{
		Voice:             Ptr("alloy"),
		InputAudioFormat:  Ptr("pcm16"),
		OutputAudioFormat: Ptr("pcm16"),
		Instructions:      Ptr("You are a helpful assistant."),
		TurnDetection: &TurnDetection{
			Type:              "server_vad",
			Threshold:         0.5,
			PrefixPaddingMS:   300,
			SilenceDurationMS: 200,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateSession(session)
	}
}

func BenchmarkValidateCreateResponseOptions(b *testing.B) {
	opts := CreateResponseOptions{
		Modalities:   []string{"text", "audio"},
		Prompt:       "Generate a helpful response",
		Instructions: "Be concise and accurate",
		Temperature:  0.7,
		Conversation: "conv_123",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateCreateResponseOptions(opts)
	}
}
