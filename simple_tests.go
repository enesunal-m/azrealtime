package azrealtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// Simple tests that focus on testing new functionality without complex infrastructure

func TestNewMethods_Validation(t *testing.T) {
	// Create a client for validation testing (no connection needed)
	client := &Client{}

	t.Run("CreateConversationItem validation", func(t *testing.T) {
		// Test nil context - use context.TODO() to avoid linter warning
		err := client.CreateConversationItem(context.TODO(), ConversationItem{Type: "message"})
		// We expect this to fail due to no connection, but it validates the method works
		if err == nil {
			t.Error("Expected error due to no connection")
		}

		// Test empty type
		err = client.CreateConversationItem(context.Background(), ConversationItem{})
		if err == nil || !strings.Contains(err.Error(), "item type is required") {
			t.Error("Expected item type error")
		}

		// Test content validation
		err = client.CreateConversationItem(context.Background(), ConversationItem{
			Type:    "message",
			Content: []ContentPart{{Text: "hello"}}, // Missing Type
		})
		if err == nil || !strings.Contains(err.Error(), "content[0].type is required") {
			t.Error("Expected content type error")
		}
	})

	t.Run("TruncateConversationItem validation", func(t *testing.T) {
		// Test with valid context but no connection
		err := client.TruncateConversationItem(context.TODO(), "item", 0, 0)
		if err == nil {
			t.Error("Expected error due to no connection")
		}

		// Test empty item ID
		err = client.TruncateConversationItem(context.Background(), "", 0, 0)
		if err == nil || !strings.Contains(err.Error(), "item ID is required") {
			t.Error("Expected item ID error")
		}

		// Test negative content index
		err = client.TruncateConversationItem(context.Background(), "item", -1, 0)
		if err == nil || !strings.Contains(err.Error(), "content index must be non-negative") {
			t.Error("Expected content index error")
		}

		// Test negative audio end time
		err = client.TruncateConversationItem(context.Background(), "item", 0, -1)
		if err == nil || !strings.Contains(err.Error(), "audio end time must be non-negative") {
			t.Error("Expected audio end time error")
		}
	})

	t.Run("DeleteConversationItem validation", func(t *testing.T) {
		// Test with valid context but no connection
		err := client.DeleteConversationItem(context.TODO(), "item")
		if err == nil {
			t.Error("Expected error due to no connection")
		}

		// Test empty item ID
		err = client.DeleteConversationItem(context.Background(), "")
		if err == nil || !strings.Contains(err.Error(), "item ID is required") {
			t.Error("Expected item ID error")
		}
	})

	t.Run("CancelResponse validation", func(t *testing.T) {
		// Test with valid context but no connection
		err := client.CancelResponse(context.TODO())
		if err == nil {
			t.Error("Expected error due to no connection")
		}
	})
}

func TestNewEventStructures_JSON(t *testing.T) {
	t.Run("InputAudioBufferSpeechStarted", func(t *testing.T) {
		jsonData := `{
			"type": "input_audio_buffer.speech_started",
			"event_id": "event_001",
			"audio_start_ms": 1500,
			"item_id": "item_123"
		}`

		var event InputAudioBufferSpeechStarted
		err := json.Unmarshal([]byte(jsonData), &event)
		if err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		if event.Type != "input_audio_buffer.speech_started" {
			t.Error("Wrong event type")
		}
		if event.AudioStartMs != 1500 {
			t.Error("Wrong audio start time")
		}
		if event.ItemID != "item_123" {
			t.Error("Wrong item ID")
		}
	})

	t.Run("ConversationItemInputAudioTranscriptionCompleted", func(t *testing.T) {
		jsonData := `{
			"type": "conversation.item.input_audio_transcription.completed",
			"event_id": "event_002",
			"item_id": "item_456",
			"content_index": 0,
			"transcript": "Hello world"
		}`

		var event ConversationItemInputAudioTranscriptionCompleted
		err := json.Unmarshal([]byte(jsonData), &event)
		if err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		if event.Transcript != "Hello world" {
			t.Error("Wrong transcript")
		}
		if event.ContentIndex != 0 {
			t.Error("Wrong content index")
		}
	})

	t.Run("ResponseCreated", func(t *testing.T) {
		jsonData := `{
			"type": "response.created",
			"event_id": "event_003",
			"response": {
				"id": "resp_123",
				"object": "realtime.response",
				"status": "in_progress",
				"output": []
			}
		}`

		var event ResponseCreated
		err := json.Unmarshal([]byte(jsonData), &event)
		if err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		if event.Response.ID != "resp_123" {
			t.Error("Wrong response ID")
		}
		if event.Response.Status != "in_progress" {
			t.Error("Wrong response status")
		}
	})

	t.Run("ConversationItem", func(t *testing.T) {
		item := ConversationItem{
			ID:   "item_123",
			Type: "message",
			Role: "user",
			Content: []ContentPart{
				{Type: "text", Text: "Hello world"},
				{Type: "audio", Audio: "base64data", Transcript: "Hello world"},
			},
		}

		// Test marshaling
		data, err := json.Marshal(item)
		if err != nil {
			t.Fatalf("Failed to marshal: %v", err)
		}

		// Test unmarshaling
		var unmarshaled ConversationItem
		err = json.Unmarshal(data, &unmarshaled)
		if err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		if unmarshaled.ID != item.ID {
			t.Error("ID mismatch")
		}
		if len(unmarshaled.Content) != len(item.Content) {
			t.Error("Content length mismatch")
		}
		if unmarshaled.Content[0].Type != "text" {
			t.Error("Content type mismatch")
		}
	})
}

func TestNewEventDispatch(t *testing.T) {
	client := &Client{}

	// Test event handlers can be set without panicking
	t.Run("Event handler registration", func(t *testing.T) {
		var called bool

		client.OnInputAudioBufferSpeechStarted(func(event InputAudioBufferSpeechStarted) {
			called = true
		})

		client.OnConversationItemCreated(func(event ConversationItemCreated) {
			called = true
		})

		client.OnResponseCreated(func(event ResponseCreated) {
			called = true
		})

		client.OnResponseAudioTranscriptDone(func(event ResponseAudioTranscriptDone) {
			called = true
		})

		client.OnConversationItemInputAudioTranscriptionCompleted(func(event ConversationItemInputAudioTranscriptionCompleted) {
			called = true
		})

		// Test that handlers can be called via dispatch
		client.dispatch(envelope{Type: "input_audio_buffer.speech_started"}, []byte(`{
			"type": "input_audio_buffer.speech_started",
			"event_id": "test",
			"audio_start_ms": 1000,
			"item_id": "item_test"
		}`))

		if !called {
			t.Error("Handler should have been called")
		}
	})

	t.Run("Unknown event handling", func(t *testing.T) {
		// Should not panic on unknown events
		client.dispatch(envelope{Type: "unknown.event"}, []byte(`{"type": "unknown.event"}`))
	})

	t.Run("Malformed JSON handling", func(t *testing.T) {
		// Should not panic on malformed JSON
		client.dispatch(envelope{Type: "response.created"}, []byte(`{"type": "response.created", malformed}`))
	})
}

func TestResponseUsageStructures(t *testing.T) {
	usage := ResponseUsage{
		TotalTokens:  150,
		InputTokens:  50,
		OutputTokens: 100,
		InputTokenDetails: &ResponseUsageInputTokens{
			TextTokens:   40,
			AudioTokens:  10,
			CachedTokens: 5,
		},
		OutputTokenDetails: &ResponseUsageOutputTokens{
			TextTokens:  80,
			AudioTokens: 20,
		},
	}

	// Test JSON marshaling
	data, err := json.Marshal(usage)
	if err != nil {
		t.Fatalf("Failed to marshal usage: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled ResponseUsage
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal usage: %v", err)
	}

	if unmarshaled.TotalTokens != usage.TotalTokens {
		t.Error("Total tokens mismatch")
	}
	if unmarshaled.InputTokenDetails.TextTokens != usage.InputTokenDetails.TextTokens {
		t.Error("Input text tokens mismatch")
	}
	if unmarshaled.OutputTokenDetails.AudioTokens != usage.OutputTokenDetails.AudioTokens {
		t.Error("Output audio tokens mismatch")
	}
}

func TestAllNewEventTypes(t *testing.T) {
	client := &Client{}

	// Test that all new event handlers can be registered without issues
	eventHandlers := []func(){
		func() { client.OnInputAudioBufferSpeechStarted(func(InputAudioBufferSpeechStarted) {}) },
		func() { client.OnInputAudioBufferSpeechStopped(func(InputAudioBufferSpeechStopped) {}) },
		func() { client.OnInputAudioBufferCommitted(func(InputAudioBufferCommitted) {}) },
		func() { client.OnInputAudioBufferCleared(func(InputAudioBufferCleared) {}) },
		func() { client.OnConversationItemCreated(func(ConversationItemCreated) {}) },
		func() {
			client.OnConversationItemInputAudioTranscriptionCompleted(func(ConversationItemInputAudioTranscriptionCompleted) {})
		},
		func() {
			client.OnConversationItemInputAudioTranscriptionFailed(func(ConversationItemInputAudioTranscriptionFailed) {})
		},
		func() { client.OnConversationItemTruncated(func(ConversationItemTruncated) {}) },
		func() { client.OnConversationItemDeleted(func(ConversationItemDeleted) {}) },
		func() { client.OnResponseCreated(func(ResponseCreated) {}) },
		func() { client.OnResponseDone(func(ResponseDone) {}) },
		func() { client.OnResponseOutputItemAdded(func(ResponseOutputItemAdded) {}) },
		func() { client.OnResponseOutputItemDone(func(ResponseOutputItemDone) {}) },
		func() { client.OnResponseContentPartAdded(func(ResponseContentPartAdded) {}) },
		func() { client.OnResponseContentPartDone(func(ResponseContentPartDone) {}) },
		func() { client.OnResponseFunctionCallArgumentsDelta(func(ResponseFunctionCallArgumentsDelta) {}) },
		func() { client.OnResponseFunctionCallArgumentsDone(func(ResponseFunctionCallArgumentsDone) {}) },
		func() { client.OnResponseAudioTranscriptDelta(func(ResponseAudioTranscriptDelta) {}) },
		func() { client.OnResponseAudioTranscriptDone(func(ResponseAudioTranscriptDone) {}) },
	}

	for i, handler := range eventHandlers {
		t.Run(fmt.Sprintf("Handler_%d", i), func(t *testing.T) {
			// Should not panic
			handler()
		})
	}
}
