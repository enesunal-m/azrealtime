package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/enesunal-m/azrealtime"
	"github.com/gorilla/websocket"
)

// Message types for client-server communication
type MessageType string

const (
	// Client to Server
	MsgStartSession   MessageType = "start_session"
	MsgAudioData      MessageType = "audio_data"
	MsgEndSession     MessageType = "end_session"
	MsgUpdateSession  MessageType = "update_session"
	MsgCreateResponse MessageType = "create_response"
	MsgReconnectAzure MessageType = "reconnect_azure"

	// Server to Client
	MsgSessionStarted   MessageType = "session_started"
	MsgSessionError     MessageType = "session_error"
	MsgTextDelta        MessageType = "text_delta"
	MsgTextDone         MessageType = "text_done"
	MsgAudioDelta       MessageType = "audio_delta"
	MsgAudioDone        MessageType = "audio_done"
	MsgTranscript       MessageType = "transcript"
	MsgError            MessageType = "error"
	MsgVADEvent         MessageType = "vad_event"
	MsgConnectionLost   MessageType = "connection_lost"
	MsgReconnectSuccess MessageType = "reconnect_success"
	MsgReconnectFailed  MessageType = "reconnect_failed"
	MsgResponseCreated  MessageType = "response_created"
	MsgResponseDone     MessageType = "response_done"
)

// WebSocket message structure
type WSMessage struct {
	Type MessageType `json:"type"`
	Data any         `json:"data,omitempty"`
}

// Session configuration from client
type SessionConfig struct {
	Voice             *string                        `json:"voice,omitempty"`
	Instructions      *string                        `json:"instructions,omitempty"`
	InputAudioFormat  *string                        `json:"input_audio_format,omitempty"`
	OutputAudioFormat *string                        `json:"output_audio_format,omitempty"`
	TurnDetection     *azrealtime.TurnDetection      `json:"turn_detection,omitempty"`
	Transcription     *azrealtime.InputTranscription `json:"transcription,omitempty"`
}

// Audio data from client
type AudioData struct {
	Data   string `json:"data"`   // base64 encoded PCM16 data
	Format string `json:"format"` // "pcm16"
}

// Client connection
type Client struct {
	ID              string
	WS              *websocket.Conn
	Azure           *azrealtime.Client
	Send            chan WSMessage
	mu              sync.RWMutex
	ctx             context.Context
	cancel          context.CancelFunc
	audioChunkCount int
}

// Server holds all client connections
type Server struct {
	clients    map[string]*Client
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
	upgrader   websocket.Upgrader
}

func NewServer() *Server {
	return &Server{
		clients:    make(map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for demo
			},
		},
	}
}

func (s *Server) Run() {
	for {
		select {
		case client := <-s.register:
			s.mu.Lock()
			s.clients[client.ID] = client
			s.mu.Unlock()
			log.Printf("Client %s registered", client.ID)

		case client := <-s.unregister:
			s.mu.Lock()
			if _, ok := s.clients[client.ID]; ok {
				delete(s.clients, client.ID)
				close(client.Send)
				if client.Azure != nil {
					client.Azure.Close()
				}
				client.cancel()
			}
			s.mu.Unlock()
			log.Printf("Client %s unregistered", client.ID)
		}
	}
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	clientID := fmt.Sprintf("client_%d", time.Now().UnixNano())
	ctx, cancel := context.WithCancel(context.Background())

	client := &Client{
		ID:     clientID,
		WS:     conn,
		Send:   make(chan WSMessage, 256),
		ctx:    ctx,
		cancel: cancel,
	}

	s.register <- client

	// Start goroutines for this client
	go client.writePump()
	go client.readPump(s)
}

func (c *Client) readPump(server *Server) {
	defer func() {
		server.unregister <- c
		c.WS.Close()
	}()

	c.WS.SetReadLimit(10 * 1024 * 1024) // 10MB max message size for audio data
	c.WS.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.WS.SetPongHandler(func(string) error {
		c.WS.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		var msg WSMessage
		if err := c.WS.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		c.handleMessage(msg)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.WS.Close()
	}()

	for {
		select {
		case msg, ok := <-c.Send:
			c.WS.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.WS.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.WS.WriteJSON(msg); err != nil {
				log.Printf("Write error: %v", err)
				return
			}

		case <-ticker.C:
			c.WS.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.WS.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleMessage(msg WSMessage) {
	switch msg.Type {
	case MsgStartSession:
		c.handleStartSession(msg.Data)
	case MsgAudioData:
		c.handleAudioData(msg.Data)
	case MsgEndSession:
		c.handleEndSession()
	case MsgUpdateSession:
		c.handleUpdateSession(msg.Data)
	case MsgCreateResponse:
		c.handleCreateResponse(msg.Data)
	case MsgReconnectAzure:
		c.handleReconnectAzure(msg.Data)
	default:
		log.Printf("Unknown message type: %s", msg.Type)
	}
}

func (c *Client) handleStartSession(data any) {
	var config SessionConfig
	configBytes, _ := json.Marshal(data)
	if err := json.Unmarshal(configBytes, &config); err != nil {
		c.sendError("Invalid session config", err)
		return
	}

	// Create Azure OpenAI client
	cfg := azrealtime.Config{
		ResourceEndpoint: os.Getenv("AZURE_OPENAI_ENDPOINT"),
		Deployment:       os.Getenv("AZURE_OPENAI_REALTIME_DEPLOYMENT"),
		APIVersion:       "2025-04-01-preview",
		Credential:       azrealtime.APIKey(os.Getenv("AZURE_OPENAI_API_KEY")),
		DialTimeout:      30 * time.Second,
		StructuredLogger: azrealtime.NewLogger(azrealtime.LogLevelInfo),
	}

	azureClient, err := azrealtime.Dial(c.ctx, cfg)
	if err != nil {
		c.sendError("Failed to connect to Azure OpenAI", err)
		return
	}

	c.mu.Lock()
	c.Azure = azureClient
	c.mu.Unlock()

	// Set up event handlers
	c.setupAzureEventHandlers()

	// Configure session
	session := azrealtime.Session{
		Voice:              config.Voice,
		Instructions:       config.Instructions,
		InputAudioFormat:   config.InputAudioFormat,
		OutputAudioFormat:  config.OutputAudioFormat,
		TurnDetection:      config.TurnDetection,
		InputTranscription: config.Transcription,
	}

	log.Printf("Session configuration: %+v", session)

	if err := azureClient.SessionUpdate(c.ctx, session); err != nil {
		c.sendError("Failed to update session", err)
		return
	}

	// Send success response
	c.Send <- WSMessage{
		Type: MsgSessionStarted,
		Data: map[string]string{"client_id": c.ID},
	}
}

func (c *Client) handleAudioData(data any) {
	c.mu.RLock()
	azureClient := c.Azure
	c.mu.RUnlock()

	if azureClient == nil {
		// Don't send error for audio data when no session - this is expected
		return
	}

	var audioData AudioData
	audioBytes, _ := json.Marshal(data)
	if err := json.Unmarshal(audioBytes, &audioData); err != nil {
		c.sendError("Invalid audio data", err)
		return
	}

	// Decode base64 PCM data
	pcmData, err := base64.StdEncoding.DecodeString(audioData.Data)
	if err != nil {
		c.sendError("Failed to decode audio data", err)
		return
	}

	// Log audio data info for debugging (only occasionally to avoid spam)
	c.audioChunkCount++
	if c.audioChunkCount%500 == 0 {
		log.Printf("Client %s sent %d audio chunks, current chunk size: %d bytes", c.ID, c.audioChunkCount, len(pcmData))
	}

	// Send to Azure OpenAI with better error handling
	if err := azureClient.AppendPCM16(c.ctx, pcmData); err != nil {
		// Check if it's a connection closed error
		if strings.Contains(err.Error(), "connection is closed") {
			log.Printf("Azure connection closed for client %s - stopping audio stream", c.ID)

			// Close the Azure connection to prevent further errors
			c.mu.Lock()
			if c.Azure != nil {
				c.Azure.Close()
				c.Azure = nil
			}
			c.mu.Unlock()

			// Tell the client to stop streaming and show connection lost
			c.Send <- WSMessage{
				Type: MsgConnectionLost,
				Data: map[string]string{
					"message": "Azure connection lost. Please reconnect to continue.",
				},
			}
			return
		} else {
			log.Printf("Azure AppendPCM16 error for client %s: %v", c.ID, err)
			c.sendError("Failed to send audio to Azure", err)
			return
		}
	}
}

func (c *Client) handleEndSession() {
	c.mu.Lock()
	if c.Azure != nil {
		c.Azure.Close()
		c.Azure = nil
	}
	c.mu.Unlock()
}

func (c *Client) handleUpdateSession(data any) {
	c.mu.RLock()
	azureClient := c.Azure
	c.mu.RUnlock()

	if azureClient == nil {
		c.sendError("No active session", nil)
		return
	}

	var config SessionConfig
	configBytes, _ := json.Marshal(data)
	if err := json.Unmarshal(configBytes, &config); err != nil {
		c.sendError("Invalid session config", err)
		return
	}

	session := azrealtime.Session{
		Voice:              config.Voice,
		Instructions:       config.Instructions,
		InputAudioFormat:   config.InputAudioFormat,
		OutputAudioFormat:  config.OutputAudioFormat,
		TurnDetection:      config.TurnDetection,
		InputTranscription: config.Transcription,
	}

	if err := azureClient.SessionUpdate(c.ctx, session); err != nil {
		c.sendError("Failed to update session", err)
		return
	}
}

func (c *Client) handleCreateResponse(data any) {
	c.mu.RLock()
	azureClient := c.Azure
	c.mu.RUnlock()

	if azureClient == nil {
		c.sendError("No active session", nil)
		return
	}

	var opts azrealtime.CreateResponseOptions
	if data != nil {
		optsBytes, _ := json.Marshal(data)
		json.Unmarshal(optsBytes, &opts)
	}

	// Set default modalities if not specified
	if len(opts.Modalities) == 0 {
		opts.Modalities = []string{"text", "audio"}
	}

	if _, err := azureClient.CreateResponse(c.ctx, opts); err != nil {
		c.sendError("Failed to create response", err)
		return
	}
}

func (c *Client) setupAzureEventHandlers() {
	audioAssembler := azrealtime.NewAudioAssembler()
	textAssembler := azrealtime.NewTextAssembler()

	c.Azure.OnError(func(event azrealtime.ErrorEvent) {
		log.Printf("Azure error for client %s: type=%s, message=%s, content=%v", c.ID, event.Error.Type, event.Error.Message, event.Error.Content)
		c.Send <- WSMessage{
			Type: MsgError,
			Data: map[string]any{
				"error_type": event.Error.Type,
				"message":    event.Error.Message,
				"content":    event.Error.Content,
			},
		}
	})

	// Add session lifecycle events from working example
	c.Azure.OnSessionCreated(func(event azrealtime.SessionCreated) {
		log.Printf("✅ Session created for client %s: %s", c.ID, event.Session.ID)
	})

	c.Azure.OnSessionUpdated(func(event azrealtime.SessionUpdated) {
		log.Printf("🔄 Session updated for client %s", c.ID)
	})

	// VAD events
	c.Azure.OnInputAudioBufferSpeechStarted(func(ev azrealtime.InputAudioBufferSpeechStarted) {
		c.Send <- WSMessage{
			Type: MsgVADEvent,
			Data: map[string]any{
				"event":          "speech_started",
				"audio_start_ms": ev.AudioStartMs,
				"item_id":        ev.ItemID,
			},
		}
	})

	c.Azure.OnInputAudioBufferSpeechStopped(func(ev azrealtime.InputAudioBufferSpeechStopped) {
		c.Send <- WSMessage{
			Type: MsgVADEvent,
			Data: map[string]any{
				"event":        "speech_stopped",
				"audio_end_ms": ev.AudioEndMs,
				"item_id":      ev.ItemID,
			},
		}
	})

	c.Azure.OnInputAudioBufferCommitted(func(ev azrealtime.InputAudioBufferCommitted) {
		c.Send <- WSMessage{
			Type: MsgVADEvent,
			Data: map[string]any{
				"event":   "committed",
				"item_id": ev.ItemID,
			},
		}
	})

	// Text responses
	c.Azure.OnResponseTextDelta(func(event azrealtime.ResponseTextDelta) {
		textAssembler.OnDelta(event)
		c.Send <- WSMessage{
			Type: MsgTextDelta,
			Data: map[string]any{
				"response_id":   event.ResponseID,
				"item_id":       event.ItemID,
				"output_index":  event.OutputIndex,
				"content_index": event.ContentIndex,
				"delta":         event.Delta,
			},
		}
	})

	c.Azure.OnResponseTextDone(func(event azrealtime.ResponseTextDone) {
		completeText := textAssembler.OnDone(event)
		c.Send <- WSMessage{
			Type: MsgTextDone,
			Data: map[string]any{
				"response_id":   event.ResponseID,
				"item_id":       event.ItemID,
				"output_index":  event.OutputIndex,
				"content_index": event.ContentIndex,
				"text":          completeText,
			},
		}
	})

	// Response lifecycle events
	c.Azure.OnResponseCreated(func(event azrealtime.ResponseCreated) {
		c.Send <- WSMessage{
			Type: MsgResponseCreated,
			Data: map[string]any{
				"response_id": event.Response.ID,
			},
		}
	})

	c.Azure.OnResponseDone(func(event azrealtime.ResponseDone) {
		c.Send <- WSMessage{
			Type: MsgResponseDone,
			Data: map[string]any{
				"response_id": event.Response.ID,
			},
		}
	})

	// Audio responses
	c.Azure.OnResponseAudioDelta(func(event azrealtime.ResponseAudioDelta) {
		if err := audioAssembler.OnDelta(event); err != nil {
			log.Printf("Error processing audio delta: %v", err)
			return
		}

		c.Send <- WSMessage{
			Type: MsgAudioDelta,
			Data: map[string]any{
				"response_id":   event.ResponseID,
				"item_id":       event.ItemID,
				"output_index":  event.OutputIndex,
				"content_index": event.ContentIndex,
				"delta":         base64.StdEncoding.EncodeToString([]byte(event.DeltaBase64)),
			},
		}
	})

	c.Azure.OnResponseAudioDone(func(event azrealtime.ResponseAudioDone) {
		pcmData := audioAssembler.OnDone(event.ResponseID)
		c.Send <- WSMessage{
			Type: MsgAudioDone,
			Data: map[string]any{
				"response_id":   event.ResponseID,
				"item_id":       event.ItemID,
				"output_index":  event.OutputIndex,
				"content_index": event.ContentIndex,
				"audio_data":    base64.StdEncoding.EncodeToString(pcmData),
				"sample_rate":   azrealtime.DefaultSampleRate,
			},
		}
	})

	// Transcription
	c.Azure.OnConversationItemInputAudioTranscriptionCompleted(func(event azrealtime.ConversationItemInputAudioTranscriptionCompleted) {
		log.Printf("Client %s transcript received: %s", c.ID, event.Transcript)
		c.Send <- WSMessage{
			Type: MsgTranscript,
			Data: map[string]any{
				"item_id":       event.ItemID,
				"content_index": event.ContentIndex,
				"transcript":    event.Transcript,
			},
		}
	})

	c.Azure.OnConversationItemInputAudioTranscriptionFailed(func(event azrealtime.ConversationItemInputAudioTranscriptionFailed) {
		log.Printf("❌ Transcription failed for client %s: %s", c.ID, event.Error.Message)
		c.Send <- WSMessage{
			Type: MsgError,
			Data: map[string]any{
				"error_type": "transcription_failed",
				"message":    event.Error.Message,
			},
		}
	})
}

func (c *Client) handleReconnectAzure(data any) {
	log.Printf("Client %s requested Azure reconnection", c.ID)

	var config SessionConfig
	configBytes, _ := json.Marshal(data)
	if err := json.Unmarshal(configBytes, &config); err != nil {
		c.Send <- WSMessage{
			Type: MsgReconnectFailed,
			Data: map[string]string{
				"message": "Invalid reconnection config",
				"details": err.Error(),
			},
		}
		return
	}

	// Close existing Azure connection if any
	c.mu.Lock()
	if c.Azure != nil {
		c.Azure.Close()
		c.Azure = nil
	}
	c.mu.Unlock()

	// Create new Azure OpenAI client
	cfg := azrealtime.Config{
		ResourceEndpoint: os.Getenv("AZURE_OPENAI_ENDPOINT"),
		Deployment:       os.Getenv("AZURE_OPENAI_REALTIME_DEPLOYMENT"),
		APIVersion:       "2025-04-01-preview",
		Credential:       azrealtime.APIKey(os.Getenv("AZURE_OPENAI_API_KEY")),
		DialTimeout:      30 * time.Second,
		StructuredLogger: azrealtime.NewLogger(azrealtime.LogLevelInfo),
	}

	azureClient, err := azrealtime.Dial(c.ctx, cfg)
	if err != nil {
		c.Send <- WSMessage{
			Type: MsgReconnectFailed,
			Data: map[string]string{
				"message": "Failed to reconnect to Azure OpenAI",
				"details": err.Error(),
			},
		}
		return
	}

	c.mu.Lock()
	c.Azure = azureClient
	c.mu.Unlock()

	// Set up event handlers
	c.setupAzureEventHandlers()

	// Configure session
	session := azrealtime.Session{
		Voice:              config.Voice,
		Instructions:       config.Instructions,
		InputAudioFormat:   azrealtime.Ptr("pcm16"),
		OutputAudioFormat:  azrealtime.Ptr("pcm16"),
		TurnDetection:      config.TurnDetection,
		InputTranscription: config.Transcription,
	}

	if err := azureClient.SessionUpdate(c.ctx, session); err != nil {
		c.Send <- WSMessage{
			Type: MsgReconnectFailed,
			Data: map[string]string{
				"message": "Failed to configure session after reconnection",
				"details": err.Error(),
			},
		}
		return
	}

	// Send success response
	c.Send <- WSMessage{
		Type: MsgReconnectSuccess,
		Data: map[string]string{
			"message": "Successfully reconnected to Azure OpenAI",
		},
	}

	log.Printf("Client %s successfully reconnected to Azure OpenAI", c.ID)
}

func (c *Client) recreateAzureConnection() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Close old connection if exists
	if c.Azure != nil {
		c.Azure.Close()
		c.Azure = nil
	}

	// Create new Azure OpenAI client
	cfg := azrealtime.Config{
		ResourceEndpoint: os.Getenv("AZURE_OPENAI_ENDPOINT"),
		Deployment:       os.Getenv("AZURE_OPENAI_REALTIME_DEPLOYMENT"),
		APIVersion:       "2025-04-01-preview",
		Credential:       azrealtime.APIKey(os.Getenv("AZURE_OPENAI_API_KEY")),
		DialTimeout:      30 * time.Second,
		StructuredLogger: azrealtime.NewLogger(azrealtime.LogLevelInfo),
	}

	azureClient, err := azrealtime.Dial(c.ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to reconnect to Azure OpenAI: %w", err)
	}

	c.Azure = azureClient

	// Set up event handlers again
	c.setupAzureEventHandlers()

	// Configure session with basic settings
	session := azrealtime.Session{
		Voice:             azrealtime.Ptr("alloy"),
		Instructions:      azrealtime.Ptr("You are a helpful AI assistant. Respond naturally and conversationally."),
		InputAudioFormat:  azrealtime.Ptr("pcm16"),
		OutputAudioFormat: azrealtime.Ptr("pcm16"),
		TurnDetection: &azrealtime.TurnDetection{
			Type:              "server_vad",
			CreateResponse:    true,
			InterruptResponse: true,
			Threshold:         0.5,
			PrefixPaddingMS:   300,
			SilenceDurationMS: 700,
		},
		InputTranscription: &azrealtime.InputTranscription{
			Model:    "whisper-1",
			Language: "en",
		},
	}

	if err := azureClient.SessionUpdate(c.ctx, session); err != nil {
		return fmt.Errorf("failed to update session after reconnect: %w", err)
	}

	log.Printf("Successfully reconnected Azure OpenAI for client %s", c.ID)
	return nil
}

func (c *Client) sendError(message string, err error) {
	errorData := map[string]string{
		"message": message,
	}
	if err != nil {
		errorData["details"] = err.Error()
	}

	c.Send <- WSMessage{
		Type: MsgSessionError,
		Data: errorData,
	}
}

func main() {
	// Validate environment variables
	required := []string{
		"AZURE_OPENAI_ENDPOINT",
		"AZURE_OPENAI_REALTIME_DEPLOYMENT",
		"AZURE_OPENAI_API_KEY",
	}

	for _, env := range required {
		if os.Getenv(env) == "" {
			log.Fatalf("Environment variable %s is required", env)
		}
	}

	server := NewServer()
	go server.Run()

	// Serve static files
	http.Handle("/", http.FileServer(http.Dir("../frontend/")))

	// WebSocket endpoint
	http.HandleFunc("/ws", server.handleWebSocket)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	log.Printf("Server starting on port %s", port)
	log.Printf("Frontend available at: http://localhost:%s", port)
	log.Printf("WebSocket endpoint: ws://localhost:%s/ws", port)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
