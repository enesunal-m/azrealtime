package azrealtime

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestNewMethodsWithMockServer(t *testing.T) {
	server := NewMockServer(t)
	defer server.Close()

	// Create client with mock server
	cfg := Config{
		ResourceEndpoint: server.URL(),
		Deployment:       "test-deployment",
		APIVersion:       "2024-10-01-preview",
		Credential:       APIKey("test-key"),
		DialTimeout:      5 * time.Second,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := Dial(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer client.Close()

	t.Run("CreateConversationItem", func(t *testing.T) {
		item := ConversationItem{
			Type: "message",
			Role: "user",
			Content: []ContentPart{
				{Type: "text", Text: "Hello, assistant!"},
			},
		}

		err := client.CreateConversationItem(ctx, item)
		if err != nil {
			t.Errorf("CreateConversationItem failed: %v", err)
		}
	})

	t.Run("TruncateConversationItem", func(t *testing.T) {
		err := client.TruncateConversationItem(ctx, "item_123", 0, 1000)
		if err != nil {
			t.Errorf("TruncateConversationItem failed: %v", err)
		}
	})

	t.Run("DeleteConversationItem", func(t *testing.T) {
		err := client.DeleteConversationItem(ctx, "item_123")
		if err != nil {
			t.Errorf("DeleteConversationItem failed: %v", err)
		}
	})

	t.Run("CancelResponse", func(t *testing.T) {
		err := client.CancelResponse(ctx)
		if err != nil {
			t.Errorf("CancelResponse failed: %v", err)
		}
	})
}

func TestAllNewEventHandlers(t *testing.T) {
	server := NewMockServer(t)
	defer server.Close()

	cfg := Config{
		ResourceEndpoint: server.URL(),
		Deployment:       "test-deployment",
		APIVersion:       "2024-10-01-preview",
		Credential:       APIKey("test-key"),
		DialTimeout:      5 * time.Second,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := Dial(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer client.Close()

	// Test all new event handlers by setting them and triggering dispatch
	var eventsCalled = make(map[string]bool)

	// Input audio buffer events
	client.OnInputAudioBufferSpeechStarted(func(event InputAudioBufferSpeechStarted) {
		eventsCalled["speech_started"] = true
	})
	client.OnInputAudioBufferSpeechStopped(func(event InputAudioBufferSpeechStopped) {
		eventsCalled["speech_stopped"] = true
	})
	client.OnInputAudioBufferCommitted(func(event InputAudioBufferCommitted) {
		eventsCalled["committed"] = true
	})
	client.OnInputAudioBufferCleared(func(event InputAudioBufferCleared) {
		eventsCalled["cleared"] = true
	})

	// Conversation item events
	client.OnConversationItemCreated(func(event ConversationItemCreated) {
		eventsCalled["item_created"] = true
	})
	client.OnConversationItemInputAudioTranscriptionCompleted(func(event ConversationItemInputAudioTranscriptionCompleted) {
		eventsCalled["transcription_completed"] = true
	})
	client.OnConversationItemInputAudioTranscriptionFailed(func(event ConversationItemInputAudioTranscriptionFailed) {
		eventsCalled["transcription_failed"] = true
	})
	client.OnConversationItemTruncated(func(event ConversationItemTruncated) {
		eventsCalled["item_truncated"] = true
	})
	client.OnConversationItemDeleted(func(event ConversationItemDeleted) {
		eventsCalled["item_deleted"] = true
	})

	// Response lifecycle events
	client.OnResponseCreated(func(event ResponseCreated) {
		eventsCalled["response_created"] = true
	})
	client.OnResponseDone(func(event ResponseDone) {
		eventsCalled["response_done"] = true
	})
	client.OnResponseOutputItemAdded(func(event ResponseOutputItemAdded) {
		eventsCalled["output_item_added"] = true
	})
	client.OnResponseOutputItemDone(func(event ResponseOutputItemDone) {
		eventsCalled["output_item_done"] = true
	})
	client.OnResponseContentPartAdded(func(event ResponseContentPartAdded) {
		eventsCalled["content_part_added"] = true
	})
	client.OnResponseContentPartDone(func(event ResponseContentPartDone) {
		eventsCalled["content_part_done"] = true
	})

	// Function call events
	client.OnResponseFunctionCallArgumentsDelta(func(event ResponseFunctionCallArgumentsDelta) {
		eventsCalled["function_args_delta"] = true
	})
	client.OnResponseFunctionCallArgumentsDone(func(event ResponseFunctionCallArgumentsDone) {
		eventsCalled["function_args_done"] = true
	})

	// Audio transcript events
	client.OnResponseAudioTranscriptDelta(func(event ResponseAudioTranscriptDelta) {
		eventsCalled["audio_transcript_delta"] = true
	})
	client.OnResponseAudioTranscriptDone(func(event ResponseAudioTranscriptDone) {
		eventsCalled["audio_transcript_done"] = true
	})

	// Simulate all new events through dispatch
	testEvents := []struct {
		eventType string
		jsonData  string
		checkKey  string
	}{
		{
			"input_audio_buffer.speech_started",
			`{"type":"input_audio_buffer.speech_started","event_id":"e1","audio_start_ms":1000,"item_id":"item1"}`,
			"speech_started",
		},
		{
			"input_audio_buffer.speech_stopped",
			`{"type":"input_audio_buffer.speech_stopped","event_id":"e2","audio_end_ms":2000,"item_id":"item1"}`,
			"speech_stopped",
		},
		{
			"input_audio_buffer.committed",
			`{"type":"input_audio_buffer.committed","event_id":"e3","previous_item_id":"item0","item_id":"item1"}`,
			"committed",
		},
		{
			"input_audio_buffer.cleared",
			`{"type":"input_audio_buffer.cleared","event_id":"e4"}`,
			"cleared",
		},
		{
			"conversation.item.created",
			`{"type":"conversation.item.created","event_id":"e5","previous_item_id":"item0","item":{"id":"item1","type":"message","role":"user","content":[]}}`,
			"item_created",
		},
		{
			"conversation.item.input_audio_transcription.completed",
			`{"type":"conversation.item.input_audio_transcription.completed","event_id":"e6","item_id":"item1","content_index":0,"transcript":"Hello"}`,
			"transcription_completed",
		},
		{
			"conversation.item.input_audio_transcription.failed",
			`{"type":"conversation.item.input_audio_transcription.failed","event_id":"e7","item_id":"item1","content_index":0,"error":{"type":"error","message":"failed"}}`,
			"transcription_failed",
		},
		{
			"conversation.item.truncated",
			`{"type":"conversation.item.truncated","event_id":"e8","item_id":"item1","content_index":0,"audio_end_ms":1500}`,
			"item_truncated",
		},
		{
			"conversation.item.deleted",
			`{"type":"conversation.item.deleted","event_id":"e9","item_id":"item1"}`,
			"item_deleted",
		},
		{
			"response.created",
			`{"type":"response.created","event_id":"e10","response":{"id":"resp1","object":"realtime.response","status":"in_progress","output":[]}}`,
			"response_created",
		},
		{
			"response.done",
			`{"type":"response.done","event_id":"e11","response":{"id":"resp1","object":"realtime.response","status":"completed","output":[]}}`,
			"response_done",
		},
		{
			"response.output_item.added",
			`{"type":"response.output_item.added","event_id":"e12","response_id":"resp1","output_index":0,"item":{"id":"item2","type":"message","role":"assistant"}}`,
			"output_item_added",
		},
		{
			"response.output_item.done",
			`{"type":"response.output_item.done","event_id":"e13","response_id":"resp1","output_index":0,"item":{"id":"item2","type":"message","role":"assistant"}}`,
			"output_item_done",
		},
		{
			"response.content_part.added",
			`{"type":"response.content_part.added","event_id":"e14","response_id":"resp1","item_id":"item2","output_index":0,"content_index":0,"part":{"type":"text","text":""}}`,
			"content_part_added",
		},
		{
			"response.content_part.done",
			`{"type":"response.content_part.done","event_id":"e15","response_id":"resp1","item_id":"item2","output_index":0,"content_index":0,"part":{"type":"text","text":"Hello!"}}`,
			"content_part_done",
		},
		{
			"response.function_call_arguments.delta",
			`{"type":"response.function_call_arguments.delta","event_id":"e16","response_id":"resp1","item_id":"item2","output_index":0,"content_index":0,"call_id":"call1","delta":"{\"loc"}`,
			"function_args_delta",
		},
		{
			"response.function_call_arguments.done",
			`{"type":"response.function_call_arguments.done","event_id":"e17","response_id":"resp1","item_id":"item2","output_index":0,"content_index":0,"call_id":"call1","arguments":"{\"location\":\"NYC\"}"}`,
			"function_args_done",
		},
		{
			"response.audio_transcript.delta",
			`{"type":"response.audio_transcript.delta","event_id":"e18","response_id":"resp1","item_id":"item2","output_index":0,"content_index":0,"delta":"Hello"}`,
			"audio_transcript_delta",
		},
		{
			"response.audio_transcript.done",
			`{"type":"response.audio_transcript.done","event_id":"e19","response_id":"resp1","item_id":"item2","output_index":0,"content_index":0,"transcript":"Hello world"}`,
			"audio_transcript_done",
		},
	}

	for _, testEvent := range testEvents {
		t.Run(testEvent.eventType, func(t *testing.T) {
			// Dispatch the event
			client.dispatch(envelope{Type: testEvent.eventType}, []byte(testEvent.jsonData))
			
			// Check if handler was called
			if !eventsCalled[testEvent.checkKey] {
				t.Errorf("Handler for %s was not called", testEvent.eventType)
			}
		})
	}

	// Test unknown event handling
	t.Run("unknown_event", func(t *testing.T) {
		// Should not panic
		client.dispatch(envelope{Type: "unknown.event.type"}, []byte(`{"type":"unknown.event.type"}`))
	})

	// Test malformed JSON handling
	t.Run("malformed_json", func(t *testing.T) {
		// Should not panic
		client.dispatch(envelope{Type: "response.created"}, []byte(`{"type":"response.created","invalid":json}`))
	})
}

func TestConversationDataStructures(t *testing.T) {
	// Test ConversationItem with all possible fields
	item := ConversationItem{
		ID:     "item_123",
		Type:   "function_call",
		Status: "completed",
		Role:   "assistant",
		Content: []ContentPart{
			{
				Type:       "text",
				Text:       "Function call result",
				Audio:      "",
				Transcript: "",
			},
		},
		CallID:    "call_456",
		Name:      "get_weather",
		Arguments: `{"location": "New York", "unit": "celsius"}`,
		Output:    `{"temperature": 22, "condition": "sunny"}`,
	}

	// Test all conversation item types
	itemTypes := []string{"message", "function_call", "function_call_output"}
	for _, itemType := range itemTypes {
		t.Run("ConversationItem_"+itemType, func(t *testing.T) {
			testItem := item
			testItem.Type = itemType
			
			// Should be able to marshal/unmarshal
			data, err := json.Marshal(testItem)
			if err != nil {
				t.Errorf("Failed to marshal %s item: %v", itemType, err)
			}
			
			var unmarshaled ConversationItem
			err = json.Unmarshal(data, &unmarshaled)
			if err != nil {
				t.Errorf("Failed to unmarshal %s item: %v", itemType, err)
			}
			
			if unmarshaled.Type != itemType {
				t.Errorf("Type mismatch for %s: expected %s, got %s", itemType, itemType, unmarshaled.Type)
			}
		})
	}

	// Test ContentPart types
	contentTypes := []string{"text", "audio", "input_text", "input_audio"}
	for _, contentType := range contentTypes {
		t.Run("ContentPart_"+contentType, func(t *testing.T) {
			part := ContentPart{
				Type:       contentType,
				Text:       "Sample text",
				Audio:      "base64audiodata",
				Transcript: "Sample transcript",
			}
			
			data, err := json.Marshal(part)
			if err != nil {
				t.Errorf("Failed to marshal %s content: %v", contentType, err)
			}
			
			var unmarshaled ContentPart
			err = json.Unmarshal(data, &unmarshaled)
			if err != nil {
				t.Errorf("Failed to unmarshal %s content: %v", contentType, err)
			}
			
			if unmarshaled.Type != contentType {
				t.Errorf("Type mismatch for %s: expected %s, got %s", contentType, contentType, unmarshaled.Type)
			}
		})
	}

	// Test ResponseObject with full usage information
	t.Run("ResponseObject_full", func(t *testing.T) {
		response := ResponseObject{
			ID:     "resp_456",
			Object: "realtime.response",
			Status: "completed",
			StatusDetails: map[string]interface{}{
				"type":   "completed",
				"reason": "success",
			},
			Output: []ConversationItem{item},
			Usage: &ResponseUsage{
				TotalTokens:  250,
				InputTokens:  100,
				OutputTokens: 150,
				InputTokenDetails: &ResponseUsageInputTokens{
					TextTokens:   80,
					AudioTokens:  20,
					CachedTokens: 10,
				},
				OutputTokenDetails: &ResponseUsageOutputTokens{
					TextTokens:  120,
					AudioTokens: 30,
				},
			},
			Metadata: map[string]interface{}{
				"request_id": "req_789",
				"session_id": "sess_321",
			},
		}

		data, err := json.Marshal(response)
		if err != nil {
			t.Errorf("Failed to marshal ResponseObject: %v", err)
		}

		var unmarshaled ResponseObject
		err = json.Unmarshal(data, &unmarshaled)
		if err != nil {
			t.Errorf("Failed to unmarshal ResponseObject: %v", err)
		}

		if unmarshaled.ID != response.ID {
			t.Error("ResponseObject ID mismatch")
		}
		if unmarshaled.Usage.TotalTokens != response.Usage.TotalTokens {
			t.Error("Usage total tokens mismatch")
		}
		if unmarshaled.Usage.InputTokenDetails.CachedTokens != response.Usage.InputTokenDetails.CachedTokens {
			t.Error("Cached tokens mismatch")
		}
	})
}