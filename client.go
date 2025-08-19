package azrealtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"nhooyr.io/websocket"
)

// Client represents a connection to the Azure OpenAI Realtime API.
// It manages the WebSocket connection, handles event dispatching, and provides
// methods for sending requests to the API. The client is designed to be safe
// for concurrent use across multiple goroutines.
//
// The client uses an event-driven architecture where you register callback
// functions to handle different types of events received from the API.
type Client struct {
	cfg Config // Configuration used to create this client

	// Connection state
	conn       *websocket.Conn    // Underlying WebSocket connection
	writeMu    sync.Mutex         // Protects writes to the WebSocket
	readCancel context.CancelFunc // Cancels the read loop when closing
	closedCh   chan struct{}      // Signals when the client is closed
	closeOnce  sync.Once          // Ensures closedCh is only closed once

	// Event handlers - these functions are called when corresponding events are received
	handlerMu                                        sync.RWMutex                                               // Protects event handler fields
	onError                                          func(ErrorEvent)                                           // Called for API errors
	onSessionCreated                                 func(SessionCreated)                                       // Called when session is established
	onSessionUpdated                                 func(SessionUpdated)                                       // Called when session config changes
	onRateLimitsUpdated                              func(RateLimitsUpdated)                                    // Called for rate limit updates
	onResponseTextDelta                              func(ResponseTextDelta)                                    // Called for streaming text responses
	onResponseTextDone                               func(ResponseTextDone)                                     // Called when text response completes
	onResponseAudioDelta                             func(ResponseAudioDelta)                                   // Called for streaming audio responses
	onResponseAudioDone                              func(ResponseAudioDone)                                    // Called when audio response completes
	onInputAudioBufferSpeechStarted                  func(InputAudioBufferSpeechStarted)                        // Called when user starts speaking
	onInputAudioBufferSpeechStopped                  func(InputAudioBufferSpeechStopped)                        // Called when user stops speaking
	onInputAudioBufferCommitted                      func(InputAudioBufferCommitted)                            // Called when audio buffer is committed
	onInputAudioBufferCleared                        func(InputAudioBufferCleared)                              // Called when audio buffer is cleared
	onConversationItemCreated                        func(ConversationItemCreated)                              // Called when conversation item is created
	onConversationItemInputAudioTranscriptionCompleted func(ConversationItemInputAudioTranscriptionCompleted) // Called when audio transcription completes
	onConversationItemInputAudioTranscriptionFailed func(ConversationItemInputAudioTranscriptionFailed)       // Called when audio transcription fails
	onConversationItemTruncated                      func(ConversationItemTruncated)                            // Called when conversation item is truncated
	onConversationItemDeleted                        func(ConversationItemDeleted)                              // Called when conversation item is deleted
	onResponseCreated                                func(ResponseCreated)                                      // Called when response is created
	onResponseDone                                   func(ResponseDone)                                         // Called when response is complete
	onResponseOutputItemAdded                        func(ResponseOutputItemAdded)                              // Called when output item is added
	onResponseOutputItemDone                         func(ResponseOutputItemDone)                               // Called when output item is complete
	onResponseContentPartAdded                       func(ResponseContentPartAdded)                             // Called when content part is added
	onResponseContentPartDone                        func(ResponseContentPartDone)                              // Called when content part is complete
	onResponseFunctionCallArgumentsDelta             func(ResponseFunctionCallArgumentsDelta)                   // Called for streaming function arguments
	onResponseFunctionCallArgumentsDone              func(ResponseFunctionCallArgumentsDone)                    // Called when function arguments are complete
	onResponseAudioTranscriptDelta                   func(ResponseAudioTranscriptDelta)                         // Called for streaming audio transcript
	onResponseAudioTranscriptDone                    func(ResponseAudioTranscriptDone)                          // Called when audio transcript is complete
}

// Dial establishes a WebSocket connection to the Azure OpenAI Realtime API.
// It validates the configuration, constructs the WebSocket URL, performs authentication,
// and starts the background goroutines for handling messages and keepalives.
//
// The returned client is ready to use and will automatically handle incoming events.
// Call Close() when finished to properly clean up resources.
//
// Returns an error if configuration is invalid, connection fails, or authentication is rejected.
func Dial(ctx context.Context, cfg Config) (*Client, error) {
	// Validate configuration using new validation system
	if err := ValidateConfig(cfg); err != nil {
		return nil, err
	}

	// Construct WebSocket URL from HTTP endpoint
	u, err := url.Parse(cfg.ResourceEndpoint)
	if err != nil {
		return nil, NewConfigError("ResourceEndpoint", cfg.ResourceEndpoint, "invalid URL format")
	}

	// Set WebSocket scheme based on HTTP scheme
	if u.Scheme == "https" {
		u.Scheme = "wss"
	} else {
		u.Scheme = "ws" // For HTTP (mainly for testing)
	}
	u.Path = "/openai/realtime"
	q := u.Query()
	q.Set("api-version", cfg.APIVersion)
	q.Set("deployment", cfg.Deployment)
	u.RawQuery = q.Encode()

	// Prepare authentication and custom headers
	h := http.Header{}
	if cfg.HandshakeHeaders != nil {
		for k, vals := range cfg.HandshakeHeaders {
			for _, v := range vals {
				h.Add(k, v)
			}
		}
	}
	cfg.Credential.apply(h)

	// Apply dial timeout if specified
	dialCtx := ctx
	if cfg.DialTimeout > 0 {
		var cancel context.CancelFunc
		dialCtx, cancel = context.WithTimeout(ctx, cfg.DialTimeout)
		defer cancel()
	}

	// Establish WebSocket connection
	ws, _, err := websocket.Dial(dialCtx, u.String(), &websocket.DialOptions{HTTPHeader: h})
	if err != nil {
		return nil, NewConnectionError(u.String(), "dial", err)
	}

	// Create client and start background operations
	c := &Client{cfg: cfg, conn: ws, closedCh: make(chan struct{})}
	c.log("ws_connected", map[string]any{"url": u.String()})

	// Start read loop in separate goroutine
	rcCtx, cancel := context.WithCancel(context.Background())
	c.readCancel = cancel
	go c.readLoop(rcCtx)

	// Start ping loop to maintain connection
	go c.pingLoop()
	return c, nil
}

// DialResilient creates a new client with built-in retry and resilience features.
// This is a convenience function that combines Dial with retry logic and circuit breaker.
func DialResilient(ctx context.Context, cfg Config) (*WithRetryableClient, error) {
	retryConfig := DefaultRetryConfig()

	client, err := DialWithRetry(ctx, cfg, retryConfig)
	if err != nil {
		return nil, err
	}

	return NewRetryableClient(client, retryConfig), nil
}

// Close gracefully shuts down the client and cleans up all resources.
// This method is safe to call multiple times and will not block.
// After calling Close(), the client should not be used for further operations.
func (c *Client) Close() error {
	// Cancel the read loop to stop processing incoming messages
	if c.readCancel != nil {
		c.readCancel()
	}

	// Close the WebSocket connection safely
	c.writeMu.Lock()
	if c.conn != nil {
		_ = c.conn.Close(websocket.StatusNormalClosure, "closing")
		c.conn = nil
	}
	c.writeMu.Unlock()

	// Signal that the client is closed
	c.closeOnce.Do(func() {
		close(c.closedCh)
	})
	return nil
}

// Event handler registration methods
// These methods allow you to register callback functions for different event types.
// Callbacks are executed in the read loop goroutine, so they should not block.

// OnError registers a callback for API error events.
func (c *Client) OnError(fn func(ErrorEvent)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.onError = fn
}

// OnSessionCreated registers a callback for session creation events.
func (c *Client) OnSessionCreated(fn func(SessionCreated)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.onSessionCreated = fn
}

// OnSessionUpdated registers a callback for session update events.
func (c *Client) OnSessionUpdated(fn func(SessionUpdated)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.onSessionUpdated = fn
}

// OnRateLimitsUpdated registers a callback for rate limit update events.
func (c *Client) OnRateLimitsUpdated(fn func(RateLimitsUpdated)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.onRateLimitsUpdated = fn
}

// OnResponseTextDelta registers a callback for streaming text response events.
func (c *Client) OnResponseTextDelta(fn func(ResponseTextDelta)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.onResponseTextDelta = fn
}

// OnResponseTextDone registers a callback for completed text response events.
func (c *Client) OnResponseTextDone(fn func(ResponseTextDone)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.onResponseTextDone = fn
}

// OnResponseAudioDelta registers a callback for streaming audio response events.
func (c *Client) OnResponseAudioDelta(fn func(ResponseAudioDelta)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.onResponseAudioDelta = fn
}

// OnResponseAudioDone registers a callback for completed audio response events.
func (c *Client) OnResponseAudioDone(fn func(ResponseAudioDone)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.onResponseAudioDone = fn
}

// OnInputAudioBufferSpeechStarted registers a callback for speech start events.
func (c *Client) OnInputAudioBufferSpeechStarted(fn func(InputAudioBufferSpeechStarted)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.onInputAudioBufferSpeechStarted = fn
}

// OnInputAudioBufferSpeechStopped registers a callback for speech stop events.
func (c *Client) OnInputAudioBufferSpeechStopped(fn func(InputAudioBufferSpeechStopped)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.onInputAudioBufferSpeechStopped = fn
}

// OnInputAudioBufferCommitted registers a callback for audio buffer committed events.
func (c *Client) OnInputAudioBufferCommitted(fn func(InputAudioBufferCommitted)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.onInputAudioBufferCommitted = fn
}

// OnInputAudioBufferCleared registers a callback for audio buffer cleared events.
func (c *Client) OnInputAudioBufferCleared(fn func(InputAudioBufferCleared)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.onInputAudioBufferCleared = fn
}

// OnConversationItemCreated registers a callback for conversation item created events.
func (c *Client) OnConversationItemCreated(fn func(ConversationItemCreated)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.onConversationItemCreated = fn
}

// OnConversationItemInputAudioTranscriptionCompleted registers a callback for audio transcription completed events.
func (c *Client) OnConversationItemInputAudioTranscriptionCompleted(fn func(ConversationItemInputAudioTranscriptionCompleted)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.onConversationItemInputAudioTranscriptionCompleted = fn
}

// OnConversationItemInputAudioTranscriptionFailed registers a callback for audio transcription failed events.
func (c *Client) OnConversationItemInputAudioTranscriptionFailed(fn func(ConversationItemInputAudioTranscriptionFailed)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.onConversationItemInputAudioTranscriptionFailed = fn
}

// OnConversationItemTruncated registers a callback for conversation item truncated events.
func (c *Client) OnConversationItemTruncated(fn func(ConversationItemTruncated)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.onConversationItemTruncated = fn
}

// OnConversationItemDeleted registers a callback for conversation item deleted events.
func (c *Client) OnConversationItemDeleted(fn func(ConversationItemDeleted)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.onConversationItemDeleted = fn
}

// OnResponseCreated registers a callback for response created events.
func (c *Client) OnResponseCreated(fn func(ResponseCreated)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.onResponseCreated = fn
}

// OnResponseDone registers a callback for response done events.
func (c *Client) OnResponseDone(fn func(ResponseDone)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.onResponseDone = fn
}

// OnResponseOutputItemAdded registers a callback for response output item added events.
func (c *Client) OnResponseOutputItemAdded(fn func(ResponseOutputItemAdded)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.onResponseOutputItemAdded = fn
}

// OnResponseOutputItemDone registers a callback for response output item done events.
func (c *Client) OnResponseOutputItemDone(fn func(ResponseOutputItemDone)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.onResponseOutputItemDone = fn
}

// OnResponseContentPartAdded registers a callback for response content part added events.
func (c *Client) OnResponseContentPartAdded(fn func(ResponseContentPartAdded)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.onResponseContentPartAdded = fn
}

// OnResponseContentPartDone registers a callback for response content part done events.
func (c *Client) OnResponseContentPartDone(fn func(ResponseContentPartDone)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.onResponseContentPartDone = fn
}

// OnResponseFunctionCallArgumentsDelta registers a callback for function call arguments delta events.
func (c *Client) OnResponseFunctionCallArgumentsDelta(fn func(ResponseFunctionCallArgumentsDelta)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.onResponseFunctionCallArgumentsDelta = fn
}

// OnResponseFunctionCallArgumentsDone registers a callback for function call arguments done events.
func (c *Client) OnResponseFunctionCallArgumentsDone(fn func(ResponseFunctionCallArgumentsDone)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.onResponseFunctionCallArgumentsDone = fn
}

// OnResponseAudioTranscriptDelta registers a callback for audio transcript delta events.
func (c *Client) OnResponseAudioTranscriptDelta(fn func(ResponseAudioTranscriptDelta)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.onResponseAudioTranscriptDelta = fn
}

// OnResponseAudioTranscriptDone registers a callback for audio transcript done events.
func (c *Client) OnResponseAudioTranscriptDone(fn func(ResponseAudioTranscriptDone)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.onResponseAudioTranscriptDone = fn
}

// readLoop continuously reads messages from the WebSocket connection.
// It runs in a separate goroutine and handles message parsing and event dispatching.
// The loop terminates when the context is canceled or the connection fails.
func (c *Client) readLoop(ctx context.Context) {
	defer func() {
		// Clean up connection state when read loop exits
		c.writeMu.Lock()
		if c.conn != nil {
			_ = c.conn.Close(websocket.StatusNormalClosure, "reader_exit")
			c.conn = nil
		}
		c.writeMu.Unlock()
		c.closeOnce.Do(func() {
			close(c.closedCh)
		})
	}()

	for {
		// Read next message from WebSocket
		typ, data, err := c.conn.Read(ctx)
		if err != nil {
			return
		} // Connection closed or error occurred

		// Only process text messages (JSON events)
		if typ != websocket.MessageText {
			continue
		}

		// Parse the event envelope to determine event type
		var env envelope
		if err := json.Unmarshal(data, &env); err != nil {
			c.logError("bad_event_json", map[string]any{"err": err, "raw_data": string(data)})
			continue
		}

		// Dispatch to appropriate event handler
		c.dispatch(env, data)
	}
}

func (c *Client) pingLoop() {
	t := time.NewTicker(20 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-c.closedCh:
			return
		case <-t.C:
			c.writeMu.Lock()
			if c.conn != nil {
				_ = c.conn.Ping(context.Background())
			}
			c.writeMu.Unlock()
		}
	}
}

func (c *Client) dispatch(env envelope, raw []byte) {
	switch env.Type {
	case "error":
		var e ErrorEvent
		_ = json.Unmarshal(raw, &e)
		c.handlerMu.RLock()
		if c.onError != nil {
			c.onError(e)
		}
		c.handlerMu.RUnlock()
	case "session.created":
		var e SessionCreated
		_ = json.Unmarshal(raw, &e)
		c.handlerMu.RLock()
		if c.onSessionCreated != nil {
			c.onSessionCreated(e)
		}
		c.handlerMu.RUnlock()
	case "session.updated":
		var e SessionUpdated
		_ = json.Unmarshal(raw, &e)
		c.handlerMu.RLock()
		if c.onSessionUpdated != nil {
			c.onSessionUpdated(e)
		}
		c.handlerMu.RUnlock()
	case "rate_limits.updated":
		var e RateLimitsUpdated
		_ = json.Unmarshal(raw, &e)
		c.handlerMu.RLock()
		if c.onRateLimitsUpdated != nil {
			c.onRateLimitsUpdated(e)
		}
		c.handlerMu.RUnlock()
	case "response.text.delta":
		var e ResponseTextDelta
		_ = json.Unmarshal(raw, &e)
		c.handlerMu.RLock()
		if c.onResponseTextDelta != nil {
			c.onResponseTextDelta(e)
		}
		c.handlerMu.RUnlock()
	case "response.text.done":
		var e ResponseTextDone
		_ = json.Unmarshal(raw, &e)
		c.handlerMu.RLock()
		if c.onResponseTextDone != nil {
			c.onResponseTextDone(e)
		}
		c.handlerMu.RUnlock()
	case "response.audio.delta":
		var e ResponseAudioDelta
		_ = json.Unmarshal(raw, &e)
		c.handlerMu.RLock()
		if c.onResponseAudioDelta != nil {
			c.onResponseAudioDelta(e)
		}
		c.handlerMu.RUnlock()
	case "response.audio.done":
		var e ResponseAudioDone
		_ = json.Unmarshal(raw, &e)
		c.handlerMu.RLock()
		if c.onResponseAudioDone != nil {
			c.onResponseAudioDone(e)
		}
		c.handlerMu.RUnlock()
	case "input_audio_buffer.speech_started":
		var e InputAudioBufferSpeechStarted
		_ = json.Unmarshal(raw, &e)
		c.handlerMu.RLock()
		if c.onInputAudioBufferSpeechStarted != nil {
			c.onInputAudioBufferSpeechStarted(e)
		}
		c.handlerMu.RUnlock()
	case "input_audio_buffer.speech_stopped":
		var e InputAudioBufferSpeechStopped
		_ = json.Unmarshal(raw, &e)
		c.handlerMu.RLock()
		if c.onInputAudioBufferSpeechStopped != nil {
			c.onInputAudioBufferSpeechStopped(e)
		}
		c.handlerMu.RUnlock()
	case "input_audio_buffer.committed":
		var e InputAudioBufferCommitted
		_ = json.Unmarshal(raw, &e)
		c.handlerMu.RLock()
		if c.onInputAudioBufferCommitted != nil {
			c.onInputAudioBufferCommitted(e)
		}
		c.handlerMu.RUnlock()
	case "input_audio_buffer.cleared":
		var e InputAudioBufferCleared
		_ = json.Unmarshal(raw, &e)
		c.handlerMu.RLock()
		if c.onInputAudioBufferCleared != nil {
			c.onInputAudioBufferCleared(e)
		}
		c.handlerMu.RUnlock()
	case "conversation.item.created":
		var e ConversationItemCreated
		_ = json.Unmarshal(raw, &e)
		c.handlerMu.RLock()
		if c.onConversationItemCreated != nil {
			c.onConversationItemCreated(e)
		}
		c.handlerMu.RUnlock()
	case "conversation.item.input_audio_transcription.completed":
		var e ConversationItemInputAudioTranscriptionCompleted
		_ = json.Unmarshal(raw, &e)
		c.handlerMu.RLock()
		if c.onConversationItemInputAudioTranscriptionCompleted != nil {
			c.onConversationItemInputAudioTranscriptionCompleted(e)
		}
		c.handlerMu.RUnlock()
	case "conversation.item.input_audio_transcription.failed":
		var e ConversationItemInputAudioTranscriptionFailed
		_ = json.Unmarshal(raw, &e)
		c.handlerMu.RLock()
		if c.onConversationItemInputAudioTranscriptionFailed != nil {
			c.onConversationItemInputAudioTranscriptionFailed(e)
		}
		c.handlerMu.RUnlock()
	case "conversation.item.truncated":
		var e ConversationItemTruncated
		_ = json.Unmarshal(raw, &e)
		c.handlerMu.RLock()
		if c.onConversationItemTruncated != nil {
			c.onConversationItemTruncated(e)
		}
		c.handlerMu.RUnlock()
	case "conversation.item.deleted":
		var e ConversationItemDeleted
		_ = json.Unmarshal(raw, &e)
		c.handlerMu.RLock()
		if c.onConversationItemDeleted != nil {
			c.onConversationItemDeleted(e)
		}
		c.handlerMu.RUnlock()
	case "response.created":
		var e ResponseCreated
		_ = json.Unmarshal(raw, &e)
		c.handlerMu.RLock()
		if c.onResponseCreated != nil {
			c.onResponseCreated(e)
		}
		c.handlerMu.RUnlock()
	case "response.done":
		var e ResponseDone
		_ = json.Unmarshal(raw, &e)
		c.handlerMu.RLock()
		if c.onResponseDone != nil {
			c.onResponseDone(e)
		}
		c.handlerMu.RUnlock()
	case "response.output_item.added":
		var e ResponseOutputItemAdded
		_ = json.Unmarshal(raw, &e)
		c.handlerMu.RLock()
		if c.onResponseOutputItemAdded != nil {
			c.onResponseOutputItemAdded(e)
		}
		c.handlerMu.RUnlock()
	case "response.output_item.done":
		var e ResponseOutputItemDone
		_ = json.Unmarshal(raw, &e)
		c.handlerMu.RLock()
		if c.onResponseOutputItemDone != nil {
			c.onResponseOutputItemDone(e)
		}
		c.handlerMu.RUnlock()
	case "response.content_part.added":
		var e ResponseContentPartAdded
		_ = json.Unmarshal(raw, &e)
		c.handlerMu.RLock()
		if c.onResponseContentPartAdded != nil {
			c.onResponseContentPartAdded(e)
		}
		c.handlerMu.RUnlock()
	case "response.content_part.done":
		var e ResponseContentPartDone
		_ = json.Unmarshal(raw, &e)
		c.handlerMu.RLock()
		if c.onResponseContentPartDone != nil {
			c.onResponseContentPartDone(e)
		}
		c.handlerMu.RUnlock()
	case "response.function_call_arguments.delta":
		var e ResponseFunctionCallArgumentsDelta
		_ = json.Unmarshal(raw, &e)
		c.handlerMu.RLock()
		if c.onResponseFunctionCallArgumentsDelta != nil {
			c.onResponseFunctionCallArgumentsDelta(e)
		}
		c.handlerMu.RUnlock()
	case "response.function_call_arguments.done":
		var e ResponseFunctionCallArgumentsDone
		_ = json.Unmarshal(raw, &e)
		c.handlerMu.RLock()
		if c.onResponseFunctionCallArgumentsDone != nil {
			c.onResponseFunctionCallArgumentsDone(e)
		}
		c.handlerMu.RUnlock()
	case "response.audio_transcript.delta":
		var e ResponseAudioTranscriptDelta
		_ = json.Unmarshal(raw, &e)
		c.handlerMu.RLock()
		if c.onResponseAudioTranscriptDelta != nil {
			c.onResponseAudioTranscriptDelta(e)
		}
		c.handlerMu.RUnlock()
	case "response.audio_transcript.done":
		var e ResponseAudioTranscriptDone
		_ = json.Unmarshal(raw, &e)
		c.handlerMu.RLock()
		if c.onResponseAudioTranscriptDone != nil {
			c.onResponseAudioTranscriptDone(e)
		}
		c.handlerMu.RUnlock()
	default:
		// Log unknown event types for debugging
		c.log("unknown_event", map[string]any{"type": env.Type})
	}
}

func (c *Client) send(ctx context.Context, payload any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.conn == nil {
		return ErrClosed
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return NewSendError("unknown", "", fmt.Errorf("marshal payload: %w", err))
	}

	// Apply send timeout
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	err = c.conn.Write(ctx, websocket.MessageText, b)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return NewSendError("unknown", "", ErrSendTimeout)
		}
		return NewSendError("unknown", "", err)
	}

	return nil
}

func (c *Client) nextEventID(ctx context.Context, payload map[string]any) (string, error) {
	id := fmt.Sprintf("evt_%d", time.Now().UnixNano())
	payload["event_id"] = id
	return id, c.send(ctx, payload)
}
func (c *Client) log(event string, fields map[string]any) {
	if c.cfg.StructuredLogger != nil {
		c.cfg.StructuredLogger.Info(event, fields)
	} else if c.cfg.Logger != nil {
		c.cfg.Logger(event, fields)
	}
}


func (c *Client) logError(event string, fields map[string]any) {
	if c.cfg.StructuredLogger != nil {
		c.cfg.StructuredLogger.Error(event, fields)
	} else if c.cfg.Logger != nil {
		c.cfg.Logger("ERROR: "+event, fields)
	}
}
