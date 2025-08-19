package azrealtime

// envelope is used for initial JSON parsing to determine the event type
// before unmarshaling into the specific event struct.
type envelope struct {
	Type string `json:"type"`
}

// ErrorEvent represents an error received from the Azure OpenAI Realtime API.
// This includes both API-level errors (authentication, rate limits) and
// conversation-level errors (invalid requests, content policy violations).
type ErrorEvent struct {
	Type  string `json:"type"` // Always "error"
	Error struct {
		Type    string `json:"type,omitempty"`    // Error category (e.g., "invalid_request_error")
		Message string `json:"message,omitempty"` // Human-readable error description
		Role    string `json:"role,omitempty"`    // Role associated with error (if applicable)
		Content string `json:"content,omitempty"` // Error content or context
	} `json:"error"`
}

// SessionCreated is sent by the server when a new session is established.
// This event provides the session configuration and metadata.
type SessionCreated struct {
	Type    string `json:"type"`     // Always "session.created"
	EventID string `json:"event_id"` // Unique identifier for this event
	Session struct {
		ID         string   `json:"id"`                   // Unique session identifier
		Model      string   `json:"model"`                // Model name (e.g., "gpt-4o-realtime-preview")
		Modalities []string `json:"modalities,omitempty"` // Supported modalities: ["text", "audio"]
		Voice      string   `json:"voice,omitempty"`      // Voice used for audio responses
		ExpiresAt  int64    `json:"expires_at,omitempty"` // Session expiration timestamp (Unix)
	} `json:"session"`
}

// SessionUpdated is sent when session configuration is modified.
// This occurs after sending a session.update event.
type SessionUpdated struct {
	Type    string `json:"type"`               // Always "session.updated"
	EventID string `json:"event_id,omitempty"` // Event identifier (may be empty)
	Session any    `json:"session"`            // Updated session configuration (dynamic structure)
}

// RateLimitsUpdated provides current rate limiting information.
// This helps clients implement proper rate limiting and backoff strategies.
type RateLimitsUpdated struct {
	Type       string `json:"type"` // Always "rate_limits.updated"
	RateLimits []struct {
		Name         string `json:"name"`          // Rate limit name (e.g., "requests", "tokens")
		Limit        int    `json:"limit"`         // Maximum allowed per time window
		Remaining    int    `json:"remaining"`     // Remaining quota in current window
		ResetSeconds int    `json:"reset_seconds"` // Seconds until quota resets
	} `json:"rate_limits"`
}

// ResponseTextDelta contains incremental text content from the assistant.
// Multiple deltas are sent for a single response to enable streaming text display.
type ResponseTextDelta struct {
	Type         string `json:"type"`          // Always "response.text.delta"
	ResponseID   string `json:"response_id"`   // Unique identifier for the response
	ItemID       string `json:"item_id"`       // Identifier for the content item
	OutputIndex  int    `json:"output_index"`  // Index of this output in the response
	ContentIndex int    `json:"content_index"` // Index of this content within the output
	Delta        string `json:"delta"`         // Incremental text content
}

// ResponseTextDone signals completion of a text response.
// Contains the complete text content and marks the end of text streaming.
type ResponseTextDone struct {
	Type         string `json:"type"`          // Always "response.text.done"
	ResponseID   string `json:"response_id"`   // Unique identifier for the response
	ItemID       string `json:"item_id"`       // Identifier for the content item
	OutputIndex  int    `json:"output_index"`  // Index of this output in the response
	ContentIndex int    `json:"content_index"` // Index of this content within the output
	Text         string `json:"text"`          // Complete text content (may be empty if using deltas)
}

// ResponseAudioDelta contains incremental audio data from the assistant.
// Audio is provided as base64-encoded PCM16 data at 24kHz sample rate.
type ResponseAudioDelta struct {
	Type         string `json:"type"`          // Always "response.audio.delta"
	ResponseID   string `json:"response_id"`   // Unique identifier for the response
	ItemID       string `json:"item_id"`       // Identifier for the content item
	OutputIndex  int    `json:"output_index"`  // Index of this output in the response
	ContentIndex int    `json:"content_index"` // Index of this content within the output
	DeltaBase64  string `json:"delta"`         // Base64-encoded PCM16 audio data
}

// ResponseAudioDone signals completion of an audio response.
// Use this event to finalize audio processing and playback.
type ResponseAudioDone struct {
	Type         string `json:"type"`          // Always "response.audio.done"
	ResponseID   string `json:"response_id"`   // Unique identifier for the response
	ItemID       string `json:"item_id"`       // Identifier for the content item
	OutputIndex  int    `json:"output_index"`  // Index of this output in the response
	ContentIndex int    `json:"content_index"` // Index of this content within the output
}
