package azrealtime

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nhooyr.io/websocket"
)

// MockServer provides a test WebSocket server that simulates Azure OpenAI Realtime API
type MockServer struct {
	server   *httptest.Server
	messages []interface{}
	t        *testing.T
}

// NewMockServer creates a new mock server for testing
func NewMockServer(t *testing.T) *MockServer {
	ms := &MockServer{t: t, messages: make([]interface{}, 0)}
	
	ms.server = httptest.NewServer(http.HandlerFunc(ms.handleWebSocket))
	return ms
}

// Close shuts down the mock server
func (ms *MockServer) Close() {
	ms.server.Close()
}

// URL returns the WebSocket URL for the mock server
func (ms *MockServer) URL() string {
	return "ws" + strings.TrimPrefix(ms.server.URL, "http") + "/openai/realtime"
}

// AddMessage adds a message that the server will send to clients
func (ms *MockServer) AddMessage(msg interface{}) {
	ms.messages = append(ms.messages, msg)
}

func (ms *MockServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Check for API key in header
	if r.Header.Get("api-key") == "" && r.Header.Get("Authorization") == "" {
		http.Error(w, "Missing authentication", http.StatusUnauthorized)
		return
	}

	// Upgrade to WebSocket
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // For testing only
	})
	if err != nil {
		ms.t.Errorf("failed to upgrade to websocket: %v", err)
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Send initial session created event
	sessionCreated := SessionCreated{
		Type:    "session.created",
		EventID: "evt_mock_session_created",
		Session: struct {
			ID         string   `json:"id"`
			Model      string   `json:"model"`
			Modalities []string `json:"modalities,omitempty"`
			Voice      string   `json:"voice,omitempty"`
			ExpiresAt  int64    `json:"expires_at,omitempty"`
		}{
			ID:         "sess_mock_123",
			Model:      "gpt-4o-realtime-preview",
			Modalities: []string{"text", "audio"},
			Voice:      "alloy",
			ExpiresAt:  1640995200,
		},
	}
	
	data, _ := json.Marshal(sessionCreated)
	err = conn.Write(r.Context(), websocket.MessageText, data)
	if err != nil {
		ms.t.Errorf("failed to write session created: %v", err)
		return
	}

	// Send any pre-configured messages
	for _, msg := range ms.messages {
		data, err := json.Marshal(msg)
		if err != nil {
			ms.t.Errorf("failed to marshal message: %v", err)
			continue
		}
		
		err = conn.Write(r.Context(), websocket.MessageText, data)
		if err != nil {
			ms.t.Errorf("failed to write message: %v", err)
			return
		}
	}

	// Keep connection alive and echo any received messages
	for {
		_, data, err := conn.Read(r.Context())
		if err != nil {
			return // Connection closed
		}

		// Parse and potentially respond to incoming messages
		var env envelope
		if err := json.Unmarshal(data, &env); err != nil {
			continue
		}

		// Handle specific message types
		switch env.Type {
		case "session.update":
			// Respond with session.updated
			response := SessionUpdated{
				Type:    "session.updated",
				EventID: "evt_mock_session_updated",
				Session: map[string]interface{}{"updated": true},
			}
			respData, _ := json.Marshal(response)
			conn.Write(r.Context(), websocket.MessageText, respData)
			
		case "response.create":
			// Respond with text delta and done events
			textDelta := ResponseTextDelta{
				Type:         "response.text.delta",
				ResponseID:   "resp_mock_123",
				ItemID:       "item_mock_456",
				OutputIndex:  0,
				ContentIndex: 0,
				Delta:        "Hello from mock server!",
			}
			deltaData, _ := json.Marshal(textDelta)
			conn.Write(r.Context(), websocket.MessageText, deltaData)

			textDone := ResponseTextDone{
				Type:         "response.text.done",
				ResponseID:   "resp_mock_123",
				ItemID:       "item_mock_456",
				OutputIndex:  0,
				ContentIndex: 0,
				Text:         "Hello from mock server!",
			}
			doneData, _ := json.Marshal(textDone)
			conn.Write(r.Context(), websocket.MessageText, doneData)
		}
	}
}

// CreateMockConfig creates a valid config pointing to the mock server
func CreateMockConfig(serverURL string) Config {
	// Convert ws:// to http:// for the resource endpoint
	httpURL := strings.Replace(serverURL, "ws://", "http://", 1)
	return Config{
		ResourceEndpoint: httpURL,
		Deployment:       "test-deployment",
		APIVersion:       "2025-04-01-preview",
		Credential:       APIKey("test-key"),
	}
}

// TestHelper provides common test utilities
type TestHelper struct {
	t *testing.T
}

func NewTestHelper(t *testing.T) *TestHelper {
	return &TestHelper{t: t}
}

func (th *TestHelper) AssertNoError(err error) {
	if err != nil {
		th.t.Fatalf("unexpected error: %v", err)
	}
}

func (th *TestHelper) AssertError(err error) {
	if err == nil {
		th.t.Fatal("expected error but got nil")
	}
}

func (th *TestHelper) AssertEqual(expected, actual interface{}) {
	if expected != actual {
		th.t.Errorf("expected %v, got %v", expected, actual)
	}
}

func (th *TestHelper) AssertContains(haystack, needle string) {
	if !strings.Contains(haystack, needle) {
		th.t.Errorf("expected %q to contain %q", haystack, needle)
	}
}