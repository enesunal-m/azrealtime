package azrealtime

import (
	"context"
	"errors"
	"fmt"
	"slices"
)

// Session defines the configuration for a realtime conversation session.
// Use this to customize the AI assistant's behavior, audio formats, and interaction modes.
type Session struct {
	// Voice specifies which voice to use for audio responses.
	// Available voices: "alloy", "echo", "fable", "onyx", "nova", "shimmer", "verse"
	Voice *string `json:"voice,omitempty"`

	// Instructions provide system-level guidance to the assistant.
	// This is similar to the system message in chat completions.
	Instructions *string `json:"instructions,omitempty"`

	// InputAudioFormat specifies the format for audio input from the client.
	// Supported: "pcm16" (16-bit PCM at 24kHz), "g711_ulaw", "g711_alaw"
	InputAudioFormat *string `json:"input_audio_format,omitempty"`

	// OutputAudioFormat specifies the format for audio output from the assistant.
	// Supported: "pcm16" (16-bit PCM at 24kHz), "g711_ulaw", "g711_alaw"
	OutputAudioFormat *string `json:"output_audio_format,omitempty"`

	// InputTranscription configures automatic transcription of user audio input.
	InputTranscription *InputTranscription `json:"input_audio_transcription,omitempty"`

	// TurnDetection configures when the assistant should start/stop responding.
	TurnDetection *TurnDetection `json:"turn_detection,omitempty"`

	// Tools defines function calling capabilities available to the assistant.
	Tools []any `json:"tools,omitempty"`
}

// InputTranscription configures automatic speech recognition for user input.
type InputTranscription struct {
	Model    string  `json:"model,omitempty"`    // Transcription model to use
	Language string  `json:"language,omitempty"` // Expected language code (e.g., "en")
	Prompt   *string `json:"prompt,omitempty"`   // Context to improve transcription accuracy
}

// TurnDetection configures voice activity detection and response timing.
type TurnDetection struct {
	// Type specifies the turn detection method.
	// Supported values: "server_vad", "semantic_vad"
	Type string `json:"type"`

	// Threshold is the activation threshold for server VAD (0.0-1.0).
	// Higher values reduce false positives in noisy environments.
	// Lower values reduce false negatives in quiet environments.
	// Default: 0.5. Only applicable for server_vad.
	Threshold float64 `json:"threshold,omitempty"`

	// PrefixPaddingMS is the duration of speech audio (in milliseconds)
	// to include before the start of detected speech.
	// Default: 300ms. Only applicable for server_vad.
	PrefixPaddingMS int `json:"prefix_padding_ms,omitempty"`

	// SilenceDurationMS is the duration of silence (in milliseconds)
	// to detect the end of speech.
	// Lower values = quicker response but may cut off speech.
	// Higher values = wait longer but more complete speech.
	// Default: 200ms. Only applicable for server_vad.
	SilenceDurationMS int `json:"silence_duration_ms,omitempty"`

	// CreateResponse indicates whether the server will automatically
	// create a response when VAD detects speech end.
	// Default: true.
	CreateResponse bool `json:"create_response,omitempty"`

	// InterruptResponse indicates whether the server will automatically
	// interrupt any ongoing response when a VAD start event occurs.
	// Default: true.
	InterruptResponse bool `json:"interrupt_response,omitempty"`

	// Eagerness controls the model's eagerness to respond and interrupt.
	// Values: "low" (wait longer), "high" (chunk quickly), "auto"/"medium" (balanced).
	// Default: "auto". Only applicable for semantic_vad.
	Eagerness string `json:"eagerness,omitempty"`
}

// SessionUpdate sends a session configuration update to the API.
// This allows you to change settings like voice, instructions, and turn detection
// without creating a new connection.
func (c *Client) SessionUpdate(ctx context.Context, s Session) error {
	if ctx == nil {
		return NewSendError("session.update", "", errors.New("context cannot be nil"))
	}

	// Validate session configuration
	if err := ValidateSession(s); err != nil {
		return NewSendError("session.update", "", err)
	}

	payload := map[string]any{"type": "session.update", "session": s}
	return c.send(ctx, payload)
}

// ValidateSession performs validation on session configuration.
func ValidateSession(s Session) error {
	// Validate voice if specified
	if s.Voice != nil {
		validVoices := []string{"alloy", "echo", "fable", "onyx", "nova", "shimmer", "verse"}
		valid := slices.Contains(validVoices, *s.Voice)
		if !valid {
			return fmt.Errorf("invalid voice %q, must be one of: %v", *s.Voice, validVoices)
		}
	}

	// Validate audio formats
	if s.InputAudioFormat != nil {
		validFormats := []string{"pcm16", "g711_ulaw", "g711_alaw"}
		valid := slices.Contains(validFormats, *s.InputAudioFormat)
		if !valid {
			return fmt.Errorf("invalid input audio format %q, must be one of: %v", *s.InputAudioFormat, validFormats)
		}
	}

	if s.OutputAudioFormat != nil {
		validFormats := []string{"pcm16", "g711_ulaw", "g711_alaw"}
		valid := slices.Contains(validFormats, *s.OutputAudioFormat)
		if !valid {
			return fmt.Errorf("invalid output audio format %q, must be one of: %v", *s.OutputAudioFormat, validFormats)
		}
	}

	// Validate turn detection
	if s.TurnDetection != nil {
		if s.TurnDetection.Type == "" {
			return errors.New("turn detection type cannot be empty")
		}

		validTypes := []string{"server_vad", "semantic_vad"}
		if !slices.Contains(validTypes, s.TurnDetection.Type) {
			return fmt.Errorf("invalid turn detection type %q, must be one of: %v", s.TurnDetection.Type, validTypes)
		}

		// Server VAD specific validations
		if s.TurnDetection.Type == "server_vad" {
			if s.TurnDetection.Threshold < 0.0 || s.TurnDetection.Threshold > 1.0 {
				return fmt.Errorf("turn detection threshold must be between 0.0 and 1.0, got %f", s.TurnDetection.Threshold)
			}
			if s.TurnDetection.PrefixPaddingMS < 0 {
				return fmt.Errorf("prefix padding must be non-negative, got %d", s.TurnDetection.PrefixPaddingMS)
			}
			if s.TurnDetection.SilenceDurationMS < 0 {
				return fmt.Errorf("silence duration must be non-negative, got %d", s.TurnDetection.SilenceDurationMS)
			}
		}

		// Semantic VAD specific validations
		if s.TurnDetection.Type == "semantic_vad" {
			if s.TurnDetection.Eagerness != "" {
				validEagerness := []string{"low", "medium", "high", "auto"}
				if !slices.Contains(validEagerness, s.TurnDetection.Eagerness) {
					return fmt.Errorf("invalid eagerness %q, must be one of: %v", s.TurnDetection.Eagerness, validEagerness)
				}
			}
		}
	}

	// Validate instructions length (reasonable limit)
	if s.Instructions != nil && len(*s.Instructions) > 10000 {
		return fmt.Errorf("instructions too long (%d characters), maximum is 10000", len(*s.Instructions))
	}

	return nil
}
