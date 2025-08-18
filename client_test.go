package azrealtime

import (
	"context"
	"net/url"
	"sync"
	"testing"
	"time"
)

func TestDial_InvalidConfig(t *testing.T) {
	ctx := context.Background()
	
	tests := []struct {
		name   string
		config Config
	}{
		{
			name:   "empty config",
			config: Config{},
		},
		{
			name: "missing deployment",
			config: Config{
				ResourceEndpoint: "https://test.openai.azure.com",
				APIVersion:       "2025-04-01-preview",
				Credential:       APIKey("test-key"),
			},
		},
		{
			name: "missing credential",
			config: Config{
				ResourceEndpoint: "https://test.openai.azure.com",
				Deployment:       "test-deployment",
				APIVersion:       "2025-04-01-preview",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := Dial(ctx, tt.config)
			if err == nil {
				t.Error("expected error for invalid config")
				if client != nil {
					client.Close()
				}
			}
		})
	}
}

func TestDial_InvalidEndpoint(t *testing.T) {
	ctx := context.Background()
	
	config := Config{
		ResourceEndpoint: "invalid-url",
		Deployment:       "test-deployment",
		APIVersion:       "2025-04-01-preview",
		Credential:       APIKey("test-key"),
	}

	client, err := Dial(ctx, config)
	if err == nil {
		t.Error("expected error for invalid endpoint URL")
		if client != nil {
			client.Close()
		}
	}
}

func TestClient_WithMockServer(t *testing.T) {
	mockServer := NewMockServer(t)
	defer mockServer.Close()

	// Create config pointing to mock server
	config := CreateMockConfig(mockServer.URL())
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Dial the mock server
	client, err := Dial(ctx, config)
	if err != nil {
		t.Fatalf("failed to dial mock server: %v", err)
	}
	defer client.Close()

	// Verify we can receive session created event
	var sessionCreatedReceived bool
	var mu sync.Mutex

	client.OnSessionCreated(func(event SessionCreated) {
		mu.Lock()
		defer mu.Unlock()
		sessionCreatedReceived = true
		
		if event.Type != "session.created" {
			t.Errorf("expected session.created, got %s", event.Type)
		}
		if event.Session.Model != "gpt-4o-realtime-preview" {
			t.Errorf("expected model gpt-4o-realtime-preview, got %s", event.Session.Model)
		}
	})

	// Wait a bit for the session created event
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	received := sessionCreatedReceived
	mu.Unlock()

	if !received {
		t.Error("did not receive session created event")
	}
}

func TestClient_SessionUpdate(t *testing.T) {
	mockServer := NewMockServer(t)
	defer mockServer.Close()

	config := CreateMockConfig(mockServer.URL())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := Dial(ctx, config)
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	defer client.Close()

	// Set up handler for session updated
	var sessionUpdated bool
	var mu sync.Mutex

	client.OnSessionUpdated(func(event SessionUpdated) {
		mu.Lock()
		defer mu.Unlock()
		sessionUpdated = true
	})

	// Send session update
	session := Session{
		Voice:        Ptr("alloy"),
		Instructions: Ptr("Test instructions"),
	}

	err = client.SessionUpdate(ctx, session)
	if err != nil {
		t.Fatalf("failed to send session update: %v", err)
	}

	// Wait for response
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	updated := sessionUpdated
	mu.Unlock()

	if !updated {
		t.Error("did not receive session updated event")
	}
}

func TestClient_CreateResponse(t *testing.T) {
	mockServer := NewMockServer(t)
	defer mockServer.Close()

	config := CreateMockConfig(mockServer.URL())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := Dial(ctx, config)
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	defer client.Close()

	// Set up handlers for response events
	var textDeltaReceived, textDoneReceived bool
	var receivedText string
	var mu sync.Mutex

	client.OnResponseTextDelta(func(event ResponseTextDelta) {
		mu.Lock()
		defer mu.Unlock()
		textDeltaReceived = true
		receivedText += event.Delta
	})

	client.OnResponseTextDone(func(event ResponseTextDone) {
		mu.Lock()
		defer mu.Unlock()
		textDoneReceived = true
	})

	// Create a response
	options := CreateResponseOptions{
		Modalities: []string{"text"},
		Prompt:     "Say hello",
	}

	eventID, err := client.CreateResponse(ctx, options)
	if err != nil {
		t.Fatalf("failed to create response: %v", err)
	}

	if eventID == "" {
		t.Error("expected non-empty event ID")
	}

	// Wait for response events
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	deltaReceived := textDeltaReceived
	doneReceived := textDoneReceived
	text := receivedText
	mu.Unlock()

	if !deltaReceived {
		t.Error("did not receive text delta event")
	}
	if !doneReceived {
		t.Error("did not receive text done event")
	}
	if text != "Hello from mock server!" {
		t.Errorf("expected 'Hello from mock server!', got %q", text)
	}
}

func TestClient_EventHandlers(t *testing.T) {
	mockServer := NewMockServer(t)
	defer mockServer.Close()

	// Add a custom error event to the mock server
	errorEvent := ErrorEvent{
		Type: "error",
		Error: struct {
			Type    string `json:"type,omitempty"`
			Message string `json:"message,omitempty"`
			Role    string `json:"role,omitempty"`
			Content string `json:"content,omitempty"`
		}{
			Type:    "test_error",
			Message: "Test error message",
		},
	}
	mockServer.AddMessage(errorEvent)

	config := CreateMockConfig(mockServer.URL())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := Dial(ctx, config)
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	defer client.Close()

	// Test error handler
	var errorReceived bool
	var mu sync.Mutex

	client.OnError(func(event ErrorEvent) {
		mu.Lock()
		defer mu.Unlock()
		errorReceived = true
		
		if event.Error.Message != "Test error message" {
			t.Errorf("expected 'Test error message', got %q", event.Error.Message)
		}
	})

	// Wait for the error event
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	received := errorReceived
	mu.Unlock()

	if !received {
		t.Error("did not receive error event")
	}
}

func TestClient_Close(t *testing.T) {
	mockServer := NewMockServer(t)
	defer mockServer.Close()

	config := CreateMockConfig(mockServer.URL())
	ctx := context.Background()

	client, err := Dial(ctx, config)
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}

	// Close the client
	err = client.Close()
	if err != nil {
		t.Errorf("unexpected error closing client: %v", err)
	}

	// Try to use closed client - should return ErrClosed
	err = client.SessionUpdate(ctx, Session{})
	if err != ErrClosed {
		t.Errorf("expected ErrClosed, got %v", err)
	}
}

func TestClient_URLConstruction(t *testing.T) {
	tests := []struct {
		name             string
		resourceEndpoint string
		deployment       string
		apiVersion       string
		expectedPath     string
		expectedQuery    string
	}{
		{
			name:             "standard config",
			resourceEndpoint: "https://test.openai.azure.com",
			deployment:       "gpt-4o-realtime",
			apiVersion:       "2025-04-01-preview",
			expectedPath:     "/openai/realtime",
			expectedQuery:    "api-version=2025-04-01-preview&deployment=gpt-4o-realtime",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the endpoint like Dial does
			u, err := url.Parse(tt.resourceEndpoint)
			if err != nil {
				t.Fatalf("failed to parse endpoint: %v", err)
			}
			
			u.Scheme = "wss"
			u.Path = "/openai/realtime"
			q := u.Query()
			q.Set("api-version", tt.apiVersion)
			q.Set("deployment", tt.deployment)
			u.RawQuery = q.Encode()

			if u.Path != tt.expectedPath {
				t.Errorf("expected path %q, got %q", tt.expectedPath, u.Path)
			}
			if u.RawQuery != tt.expectedQuery {
				t.Errorf("expected query %q, got %q", tt.expectedQuery, u.RawQuery)
			}
		})
	}
}

func TestClient_Dispatch_UnknownEventType(t *testing.T) {
	client := &Client{}
	
	// Test with unknown event type - should not panic
	env := envelope{Type: "unknown.event.type"}
	rawJSON := []byte(`{"type":"unknown.event.type","data":"test"}`)
	
	// This should not panic
	client.dispatch(env, rawJSON)
}

func TestClient_NextEventID(t *testing.T) {
	mockServer := NewMockServer(t)
	defer mockServer.Close()

	config := CreateMockConfig(mockServer.URL())
	ctx := context.Background()

	client, err := Dial(ctx, config)
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	defer client.Close()

	// Test nextEventID generates unique IDs
	payload := map[string]any{"type": "test"}
	
	id1, err := client.nextEventID(ctx, payload)
	if err != nil {
		t.Fatalf("failed to generate event ID: %v", err)
	}
	
	id2, err := client.nextEventID(ctx, payload)
	if err != nil {
		t.Fatalf("failed to generate second event ID: %v", err)
	}

	if id1 == id2 {
		t.Error("expected unique event IDs")
	}
	
	if id1 == "" || id2 == "" {
		t.Error("expected non-empty event IDs")
	}
}