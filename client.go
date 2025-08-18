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
    conn       *websocket.Conn     // Underlying WebSocket connection
    writeMu    sync.Mutex          // Protects writes to the WebSocket
    readCancel context.CancelFunc  // Cancels the read loop when closing
    closedCh   chan struct{}       // Signals when the client is closed

    // Event handlers - these functions are called when corresponding events are received
    onError              func(ErrorEvent)        // Called for API errors
    onSessionCreated     func(SessionCreated)    // Called when session is established
    onSessionUpdated     func(SessionUpdated)    // Called when session config changes
    onRateLimitsUpdated  func(RateLimitsUpdated) // Called for rate limit updates
    onResponseTextDelta  func(ResponseTextDelta) // Called for streaming text responses
    onResponseTextDone   func(ResponseTextDone)  // Called when text response completes
    onResponseAudioDelta func(ResponseAudioDelta)// Called for streaming audio responses
    onResponseAudioDone  func(ResponseAudioDone) // Called when audio response completes
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
            for _, v := range vals { h.Add(k, v) }
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
    c := &Client{ cfg: cfg, conn: ws, closedCh: make(chan struct{}) }
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
    if c.readCancel != nil { c.readCancel() }
    
    // Close the WebSocket connection safely
    c.writeMu.Lock()
    if c.conn != nil { _ = c.conn.Close(websocket.StatusNormalClosure, "closing"); c.conn = nil }
    c.writeMu.Unlock()
    
    // Signal that the client is closed
    select { case <-c.closedCh: default: close(c.closedCh) }
    return nil
}

// Event handler registration methods
// These methods allow you to register callback functions for different event types.
// Callbacks are executed in the read loop goroutine, so they should not block.

// OnError registers a callback for API error events.
func (c *Client) OnError(fn func(ErrorEvent)) { c.onError = fn }

// OnSessionCreated registers a callback for session creation events.
func (c *Client) OnSessionCreated(fn func(SessionCreated)) { c.onSessionCreated = fn }

// OnSessionUpdated registers a callback for session update events.
func (c *Client) OnSessionUpdated(fn func(SessionUpdated)) { c.onSessionUpdated = fn }

// OnRateLimitsUpdated registers a callback for rate limit update events.
func (c *Client) OnRateLimitsUpdated(fn func(RateLimitsUpdated)) { c.onRateLimitsUpdated = fn }

// OnResponseTextDelta registers a callback for streaming text response events.
func (c *Client) OnResponseTextDelta(fn func(ResponseTextDelta)) { c.onResponseTextDelta = fn }

// OnResponseTextDone registers a callback for completed text response events.
func (c *Client) OnResponseTextDone(fn func(ResponseTextDone)) { c.onResponseTextDone = fn }

// OnResponseAudioDelta registers a callback for streaming audio response events.
func (c *Client) OnResponseAudioDelta(fn func(ResponseAudioDelta)) { c.onResponseAudioDelta = fn }

// OnResponseAudioDone registers a callback for completed audio response events.
func (c *Client) OnResponseAudioDone(fn func(ResponseAudioDone)) { c.onResponseAudioDone = fn }

// readLoop continuously reads messages from the WebSocket connection.
// It runs in a separate goroutine and handles message parsing and event dispatching.
// The loop terminates when the context is canceled or the connection fails.
func (c *Client) readLoop(ctx context.Context) {
    defer func() {
        // Clean up connection state when read loop exits
        c.writeMu.Lock()
        if c.conn != nil { _ = c.conn.Close(websocket.StatusNormalClosure, "reader_exit"); c.conn = nil }
        c.writeMu.Unlock()
        select { case <-c.closedCh: default: close(c.closedCh) }
    }()

    for {
        // Read next message from WebSocket
        typ, data, err := c.conn.Read(ctx)
        if err != nil { return } // Connection closed or error occurred
        
        // Only process text messages (JSON events)
        if typ != websocket.MessageText { continue }
        
        // Parse the event envelope to determine event type
        var env envelope
        if err := json.Unmarshal(data, &env); err != nil {
            c.log("bad_event_json", map[string]any{"err": err, "raw_data": string(data)})
            continue
        }
        
        // Dispatch to appropriate event handler
        c.dispatch(env, data)
    }
}

func (c *Client) pingLoop() {
    t := time.NewTicker(20 * time.Second); defer t.Stop()
    for {
        select {
        case <-c.closedCh: return
        case <-t.C:
            c.writeMu.Lock()
            if c.conn != nil { _ = c.conn.Ping(context.Background()) }
            c.writeMu.Unlock()
        }
    }
}

func (c *Client) dispatch(env envelope, raw []byte) {
    switch env.Type {
    case "error":
        var e ErrorEvent; _ = json.Unmarshal(raw, &e); if c.onError != nil { c.onError(e) }
    case "session.created":
        var e SessionCreated; _ = json.Unmarshal(raw, &e); if c.onSessionCreated != nil { c.onSessionCreated(e) }
    case "session.updated":
        var e SessionUpdated; _ = json.Unmarshal(raw, &e); if c.onSessionUpdated != nil { c.onSessionUpdated(e) }
    case "rate_limits.updated":
        var e RateLimitsUpdated; _ = json.Unmarshal(raw, &e); if c.onRateLimitsUpdated != nil { c.onRateLimitsUpdated(e) }
    case "response.text.delta":
        var e ResponseTextDelta; _ = json.Unmarshal(raw, &e); if c.onResponseTextDelta != nil { c.onResponseTextDelta(e) }
    case "response.text.done":
        var e ResponseTextDone; _ = json.Unmarshal(raw, &e); if c.onResponseTextDone != nil { c.onResponseTextDone(e) }
    case "response.audio.delta":
        var e ResponseAudioDelta; _ = json.Unmarshal(raw, &e); if c.onResponseAudioDelta != nil { c.onResponseAudioDelta(e) }
    case "response.audio.done":
        var e ResponseAudioDone; _ = json.Unmarshal(raw, &e); if c.onResponseAudioDone != nil { c.onResponseAudioDone(e) }
    default:
        // extend as needed
    }
}

func (c *Client) send(ctx context.Context, payload any) error {
    c.writeMu.Lock(); defer c.writeMu.Unlock()
    if c.conn == nil { return ErrClosed }
    
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
func (c *Client) log(event string, fields map[string]any) { if c.cfg.Logger != nil { c.cfg.Logger(event, fields) } }
