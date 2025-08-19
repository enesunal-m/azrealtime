package azrealtime

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestEnvelope_Unmarshal(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		expected envelope
	}{
		{
			name:     "error event envelope",
			jsonData: `{"type":"error"}`,
			expected: envelope{Type: "error"},
		},
		{
			name:     "session created envelope",
			jsonData: `{"type":"session.created"}`,
			expected: envelope{Type: "session.created"},
		},
		{
			name:     "response text delta envelope",
			jsonData: `{"type":"response.text.delta"}`,
			expected: envelope{Type: "response.text.delta"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var env envelope
			err := json.Unmarshal([]byte(tt.jsonData), &env)
			if err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}

			if env.Type != tt.expected.Type {
				t.Errorf("expected type %q, got %q", tt.expected.Type, env.Type)
			}
		})
	}
}

func TestErrorEvent_Unmarshal(t *testing.T) {
	jsonData := `{
		"type": "error",
		"error": {
			"type": "invalid_request_error",
			"message": "Invalid request format",
			"role": "user",
			"content": "test content"
		}
	}`

	var event ErrorEvent
	err := json.Unmarshal([]byte(jsonData), &event)
	if err != nil {
		t.Fatalf("failed to unmarshal ErrorEvent: %v", err)
	}

	expected := ErrorEvent{
		Type: "error",
		Error: struct {
			Type    string `json:"type,omitempty"`
			Message string `json:"message,omitempty"`
			Role    string `json:"role,omitempty"`
			Content string `json:"content,omitempty"`
		}{
			Type:    "invalid_request_error",
			Message: "Invalid request format",
			Role:    "user",
			Content: "test content",
		},
	}

	if !reflect.DeepEqual(event, expected) {
		t.Errorf("expected %+v, got %+v", expected, event)
	}
}

func TestSessionCreated_Unmarshal(t *testing.T) {
	jsonData := `{
		"type": "session.created",
		"event_id": "evt_123",
		"session": {
			"id": "sess_456",
			"model": "gpt-4o-realtime-preview",
			"modalities": ["text", "audio"],
			"voice": "alloy",
			"expires_at": 1640995200
		}
	}`

	var event SessionCreated
	err := json.Unmarshal([]byte(jsonData), &event)
	if err != nil {
		t.Fatalf("failed to unmarshal SessionCreated: %v", err)
	}

	if event.Type != "session.created" {
		t.Errorf("expected type 'session.created', got %q", event.Type)
	}
	if event.EventID != "evt_123" {
		t.Errorf("expected event_id 'evt_123', got %q", event.EventID)
	}
	if event.Session.ID != "sess_456" {
		t.Errorf("expected session.id 'sess_456', got %q", event.Session.ID)
	}
	if event.Session.Model != "gpt-4o-realtime-preview" {
		t.Errorf("expected model 'gpt-4o-realtime-preview', got %q", event.Session.Model)
	}
}

func TestResponseTextDelta_Unmarshal(t *testing.T) {
	jsonData := `{
		"type": "response.text.delta",
		"response_id": "resp_123",
		"item_id": "item_456",
		"output_index": 0,
		"content_index": 0,
		"delta": "Hello"
	}`

	var event ResponseTextDelta
	err := json.Unmarshal([]byte(jsonData), &event)
	if err != nil {
		t.Fatalf("failed to unmarshal ResponseTextDelta: %v", err)
	}

	if event.Type != "response.text.delta" {
		t.Errorf("expected type 'response.text.delta', got %q", event.Type)
	}
	if event.ResponseID != "resp_123" {
		t.Errorf("expected response_id 'resp_123', got %q", event.ResponseID)
	}
	if event.Delta != "Hello" {
		t.Errorf("expected delta 'Hello', got %q", event.Delta)
	}
}

func TestResponseAudioDelta_Unmarshal(t *testing.T) {
	jsonData := `{
		"type": "response.audio.delta",
		"response_id": "resp_123",
		"item_id": "item_456",
		"output_index": 0,
		"content_index": 0,
		"delta": "SGVsbG8gV29ybGQ="
	}`

	var event ResponseAudioDelta
	err := json.Unmarshal([]byte(jsonData), &event)
	if err != nil {
		t.Fatalf("failed to unmarshal ResponseAudioDelta: %v", err)
	}

	if event.Type != "response.audio.delta" {
		t.Errorf("expected type 'response.audio.delta', got %q", event.Type)
	}
	if event.DeltaBase64 != "SGVsbG8gV29ybGQ=" {
		t.Errorf("expected delta 'SGVsbG8gV29ybGQ=', got %q", event.DeltaBase64)
	}
}
