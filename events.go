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

// InputAudioBufferSpeechStarted indicates the start of speech in the input audio buffer.
// This event is generated when the server detects the beginning of speech from the user.
type InputAudioBufferSpeechStarted struct {
	Type    string `json:"type"`     // Always "input_audio_buffer.speech_started"
	EventID string `json:"event_id"` // Unique identifier for this event
	AudioStartMs int `json:"audio_start_ms"` // Milliseconds from the beginning of the input audio buffer
	ItemID  string `json:"item_id"`  // The ID of the user message item that will be created
}

// InputAudioBufferSpeechStopped indicates the end of speech in the input audio buffer.
// This event is generated when the server detects the end of speech from the user.
type InputAudioBufferSpeechStopped struct {
	Type    string `json:"type"`     // Always "input_audio_buffer.speech_stopped"
	EventID string `json:"event_id"` // Unique identifier for this event
	AudioEndMs int `json:"audio_end_ms"` // Milliseconds from the beginning of the input audio buffer
	ItemID  string `json:"item_id"`  // The ID of the user message item that will be created
}

// InputAudioBufferCommitted indicates that the input audio buffer has been committed.
type InputAudioBufferCommitted struct {
	Type           string `json:"type"`            // Always "input_audio_buffer.committed"
	EventID        string `json:"event_id"`        // Unique identifier for this event
	PreviousItemID string `json:"previous_item_id"` // The ID of the preceding item in the conversation
	ItemID         string `json:"item_id"`         // The ID of the user message item that will be created
}

// InputAudioBufferCleared indicates that the input audio buffer has been cleared.
type InputAudioBufferCleared struct {
	Type    string `json:"type"`     // Always "input_audio_buffer.cleared"
	EventID string `json:"event_id"` // Unique identifier for this event
}

// ConversationItemCreated indicates that a conversation item has been created.
type ConversationItemCreated struct {
	Type           string           `json:"type"`             // Always "conversation.item.created"
	EventID        string           `json:"event_id"`         // Unique identifier for this event
	PreviousItemID string           `json:"previous_item_id"` // The ID of the preceding item
	Item           ConversationItem `json:"item"`             // The created conversation item
}

// ConversationItemInputAudioTranscriptionCompleted indicates that transcription of user audio is complete.
type ConversationItemInputAudioTranscriptionCompleted struct {
	Type         string `json:"type"`          // Always "conversation.item.input_audio_transcription.completed"
	EventID      string `json:"event_id"`      // Unique identifier for this event
	ItemID       string `json:"item_id"`       // The ID of the user message item
	ContentIndex int    `json:"content_index"` // The index of the content part containing the audio
	Transcript   string `json:"transcript"`    // The transcribed text
}

// ConversationItemInputAudioTranscriptionFailed indicates that transcription of user audio failed.
type ConversationItemInputAudioTranscriptionFailed struct {
	Type         string `json:"type"`          // Always "conversation.item.input_audio_transcription.failed"
	EventID      string `json:"event_id"`      // Unique identifier for this event
	ItemID       string `json:"item_id"`       // The ID of the user message item
	ContentIndex int    `json:"content_index"` // The index of the content part containing the audio
	Error        struct {
		Type    string `json:"type"`    // The type of error
		Code    string `json:"code"`    // Error code, if any
		Message string `json:"message"` // A human-readable error message
		Param   string `json:"param"`   // Parameter related to the error, if any
	} `json:"error"` // Details of the transcription error
}

// ConversationItemTruncated indicates that a conversation item has been truncated.
type ConversationItemTruncated struct {
	Type         string `json:"type"`          // Always "conversation.item.truncated"
	EventID      string `json:"event_id"`      // Unique identifier for this event
	ItemID       string `json:"item_id"`       // The ID of the assistant message item
	ContentIndex int    `json:"content_index"` // The index of the content part that was truncated
	AudioEndMs   int    `json:"audio_end_ms"`  // The duration up to which the audio was truncated
}

// ConversationItemDeleted indicates that a conversation item has been deleted.
type ConversationItemDeleted struct {
	Type    string `json:"type"`     // Always "conversation.item.deleted"
	EventID string `json:"event_id"` // Unique identifier for this event
	ItemID  string `json:"item_id"`  // The ID of the deleted item
}

// ResponseCreated indicates that a response has been created.
type ResponseCreated struct {
	Type     string           `json:"type"`      // Always "response.created"
	EventID  string           `json:"event_id"`  // Unique identifier for this event
	Response ResponseObject   `json:"response"`  // The response resource
}

// ResponseDone indicates that a response is complete.
type ResponseDone struct {
	Type     string         `json:"type"`      // Always "response.done"
	EventID  string         `json:"event_id"`  // Unique identifier for this event
	Response ResponseObject `json:"response"`  // The response resource
}

// ResponseOutputItemAdded indicates that a new output item has been added to the response.
type ResponseOutputItemAdded struct {
	Type        string           `json:"type"`         // Always "response.output_item.added"
	EventID     string           `json:"event_id"`     // Unique identifier for this event
	ResponseID  string           `json:"response_id"`  // The ID of the response
	OutputIndex int              `json:"output_index"` // The index of the output item
	Item        ConversationItem `json:"item"`         // The item that was added
}

// ResponseOutputItemDone indicates that an output item is complete.
type ResponseOutputItemDone struct {
	Type        string           `json:"type"`         // Always "response.output_item.done"
	EventID     string           `json:"event_id"`     // Unique identifier for this event
	ResponseID  string           `json:"response_id"`  // The ID of the response
	OutputIndex int              `json:"output_index"` // The index of the output item
	Item        ConversationItem `json:"item"`         // The completed item
}

// ResponseContentPartAdded indicates that a new content part has been added.
type ResponseContentPartAdded struct {
	Type         string      `json:"type"`          // Always "response.content_part.added"
	EventID      string      `json:"event_id"`      // Unique identifier for this event
	ResponseID   string      `json:"response_id"`   // The ID of the response
	ItemID       string      `json:"item_id"`       // The ID of the item
	OutputIndex  int         `json:"output_index"`  // The index of the output item
	ContentIndex int         `json:"content_index"` // The index of the content part
	Part         ContentPart `json:"part"`          // The content part that was added
}

// ResponseContentPartDone indicates that a content part is complete.
type ResponseContentPartDone struct {
	Type         string      `json:"type"`          // Always "response.content_part.done"
	EventID      string      `json:"event_id"`      // Unique identifier for this event
	ResponseID   string      `json:"response_id"`   // The ID of the response
	ItemID       string      `json:"item_id"`       // The ID of the item
	OutputIndex  int         `json:"output_index"`  // The index of the output item
	ContentIndex int         `json:"content_index"` // The index of the content part
	Part         ContentPart `json:"part"`          // The completed content part
}

// ResponseFunctionCallArgumentsDelta contains incremental function call arguments.
type ResponseFunctionCallArgumentsDelta struct {
	Type         string `json:"type"`          // Always "response.function_call_arguments.delta"
	EventID      string `json:"event_id"`      // Unique identifier for this event
	ResponseID   string `json:"response_id"`   // The ID of the response
	ItemID       string `json:"item_id"`       // The ID of the function call item
	OutputIndex  int    `json:"output_index"`  // The index of the output item
	ContentIndex int    `json:"content_index"` // The index of the content part
	CallID       string `json:"call_id"`       // The ID of the function call
	Delta        string `json:"delta"`         // The incremental function arguments (JSON)
}

// ResponseFunctionCallArgumentsDone indicates that function call arguments are complete.
type ResponseFunctionCallArgumentsDone struct {
	Type         string `json:"type"`          // Always "response.function_call_arguments.done"
	EventID      string `json:"event_id"`      // Unique identifier for this event
	ResponseID   string `json:"response_id"`   // The ID of the response
	ItemID       string `json:"item_id"`       // The ID of the function call item
	OutputIndex  int    `json:"output_index"`  // The index of the output item
	ContentIndex int    `json:"content_index"` // The index of the content part
	CallID       string `json:"call_id"`       // The ID of the function call
	Arguments    string `json:"arguments"`     // The final function arguments (JSON)
}

// ResponseAudioTranscriptDelta contains incremental transcript of audio response.
type ResponseAudioTranscriptDelta struct {
	Type         string `json:"type"`          // Always "response.audio_transcript.delta"
	EventID      string `json:"event_id"`      // Unique identifier for this event
	ResponseID   string `json:"response_id"`   // The ID of the response
	ItemID       string `json:"item_id"`       // The ID of the item
	OutputIndex  int    `json:"output_index"`  // The index of the output item
	ContentIndex int    `json:"content_index"` // The index of the content part
	Delta        string `json:"delta"`         // The incremental transcript text
}

// ResponseAudioTranscriptDone indicates that audio transcript is complete.
type ResponseAudioTranscriptDone struct {
	Type         string `json:"type"`          // Always "response.audio_transcript.done"
	EventID      string `json:"event_id"`      // Unique identifier for this event
	ResponseID   string `json:"response_id"`   // The ID of the response
	ItemID       string `json:"item_id"`       // The ID of the item
	OutputIndex  int    `json:"output_index"`  // The index of the output item
	ContentIndex int    `json:"content_index"` // The index of the content part
	Transcript   string `json:"transcript"`    // The final transcript text
}
