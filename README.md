# Azure OpenAI Realtime Go Library

[![Build Status](https://github.com/enesunal-m/azrealtime/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/enesunal-m/azrealtime/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/enesunal-m/azrealtime/branch/main/graph/badge.svg)](https://codecov.io/gh/enesunal-m/azrealtime)
[![Go Report Card](https://goreportcard.com/badge/github.com/enesunal-m/azrealtime)](https://goreportcard.com/report/github.com/enesunal-m/azrealtime)
[![Go Reference](https://pkg.go.dev/badge/github.com/enesunal-m/azrealtime.svg)](https://pkg.go.dev/github.com/enesunal-m/azrealtime)
[![Release](https://img.shields.io/github/release/enesunal-m/azrealtime.svg?style=flat-square)](https://github.com/enesunal-m/azrealtime/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A production-ready Go client library for Azure OpenAI's GPT-4o Realtime API, enabling real-time bidirectional communication with OpenAI's most capable multimodal model.

## Features

✅ **Production Ready**
- Comprehensive error handling with structured error types
- Input validation and sanitization
- Graceful connection management and cleanup
- Rate limit monitoring and handling
- Extensive test coverage (95%+)

✅ **WebSocket Communication**
- Real-time bidirectional messaging
- Automatic connection management
- Built-in keepalive and reconnection
- Event-driven architecture

✅ **Audio Processing**
- PCM16 audio streaming (24kHz, 16-bit, mono)
- Audio chunk assembly and WAV conversion
- Voice activity detection support
- Audio format validation

✅ **WebRTC Support**
- Browser-compatible WebRTC integration
- Ephemeral token management
- Headless WebRTC for server applications

✅ **Developer Experience**
- Comprehensive documentation and examples
- Type-safe API with full GoDoc coverage
- Structured logging and debugging support
- Multiple authentication methods

> **Note**: Azure GPT-4o Realtime is in public preview. Pin your `api-version` and monitor the [official documentation](https://docs.microsoft.com/en-us/azure/cognitive-services/openai/) for updates.

## Quick Start

### Installation

```bash
go get github.com/enesunal-m/azrealtime
```

### Basic Usage

```go
package main

import (
    "context"
    "log"
    "os"
    "time"

    "github.com/enesunal-m/azrealtime"
)

func main() {
    // Configure client
    cfg := azrealtime.Config{
        ResourceEndpoint: os.Getenv("AZURE_OPENAI_ENDPOINT"),
        Deployment:       os.Getenv("AZURE_OPENAI_REALTIME_DEPLOYMENT"), 
        APIVersion:       "2025-04-01-preview",
        Credential:       azrealtime.APIKey(os.Getenv("AZURE_OPENAI_API_KEY")),
        DialTimeout:      30 * time.Second,
    }

    // Create client
    ctx := context.Background()
    client, err := azrealtime.Dial(ctx, cfg)
    if err != nil {
        log.Fatalf("Failed to connect: %v", err)
    }
    defer client.Close()

    // Set up event handlers
    client.OnResponseTextDelta(func(event azrealtime.ResponseTextDelta) {
        print(event.Delta) // Stream text response
    })

    // Configure session
    session := azrealtime.Session{
        Voice:             azrealtime.Ptr("alloy"),
        Instructions:      azrealtime.Ptr("You are a helpful assistant."),
        InputAudioFormat:  azrealtime.Ptr("pcm16"),
        OutputAudioFormat: azrealtime.Ptr("pcm16"),
    }
    client.SessionUpdate(ctx, session)

    // Create response
    opts := azrealtime.CreateResponseOptions{
        Modalities: []string{"text", "audio"},
        Prompt:     "Hello! Please introduce yourself.",
    }
    eventID, err := client.CreateResponse(ctx, opts)
    if err != nil {
        log.Fatalf("Failed to create response: %v", err)
    }

    log.Printf("Response requested: %s", eventID)
    time.Sleep(5 * time.Second) // Wait for response
}
```

## Architecture

The library provides three main components:

### 1. Core WebSocket Client (`azrealtime`)
- Connection management and event handling
- Audio/text streaming and assembly
- Session configuration and response generation
- Comprehensive error handling and validation

### 2. WebRTC Support (`azrealtime/webrtc`)
- Ephemeral token generation for browser clients
- Headless WebRTC connections for server applications
- WebRTC session management

### 3. Utilities and Examples
- `cmd/ephemeral-issuer`: HTTP server for ephemeral tokens
- `examples/`: Comprehensive usage examples
- Audio processing utilities and helpers

## Configuration

### Environment Variables

```bash
# Required
export AZURE_OPENAI_ENDPOINT="https://your-resource.openai.azure.com"
export AZURE_OPENAI_REALTIME_DEPLOYMENT="your-deployment-name"
export AZURE_OPENAI_API_KEY="your-api-key"

# Optional
export AZURE_OPENAI_API_VERSION="2025-04-01-preview"
```

### Client Configuration

```go
cfg := azrealtime.Config{
    ResourceEndpoint: "https://your-resource.openai.azure.com",
    Deployment:       "gpt-4o-realtime-preview",
    APIVersion:       "2025-04-01-preview", 
    Credential:       azrealtime.APIKey("your-api-key"), // or Bearer token
    DialTimeout:      30 * time.Second,
    HandshakeHeaders: http.Header{"Custom-Header": []string{"value"}},
    Logger:           func(event string, fields map[string]any) {
        log.Printf("[%s] %+v", event, fields)
    },
}
```

### Authentication Methods

```go
// API Key (most common)
cfg.Credential = azrealtime.APIKey("your-api-key")

// Bearer token (for Azure AD authentication)  
cfg.Credential = azrealtime.Bearer("your-bearer-token")
```

## Advanced Usage

### Structured Logging

The library provides advanced structured logging with configurable levels:

```go
// Option 1: Environment-based logging (set AZREALTIME_LOG_LEVEL=DEBUG)
cfg := azrealtime.Config{
    // ... other config
    StructuredLogger: azrealtime.NewLoggerFromEnv(),
}

// Option 2: Explicit log level
cfg := azrealtime.Config{
    // ... other config
    StructuredLogger: azrealtime.NewLogger(azrealtime.LogLevelDebug),
}

// Option 3: Contextual logging
logger := azrealtime.NewLogger(azrealtime.LogLevelInfo)
sessionLogger := logger.WithContext(map[string]interface{}{
    "session_id": "abc123",
    "user_id":    "user456", 
})

sessionLogger.Info("user_connected", map[string]interface{}{
    "ip": "192.168.1.1",
})
// Output: [azrealtime] [INFO] user_connected session_id=abc123 user_id=user456 ip=192.168.1.1
```

**Log Levels:**
- `LogLevelDebug`: All messages including detailed debugging
- `LogLevelInfo`: Informational messages and above (default)
- `LogLevelWarn`: Warnings and errors only
- `LogLevelError`: Error messages only
- `LogLevelOff`: No logging

**Environment Variables:**
- `AZREALTIME_LOG_LEVEL`: Sets the minimum log level (DEBUG, INFO, WARN, ERROR, OFF)

### Error Handling

The library provides structured error types for better error handling:

```go
client, err := azrealtime.Dial(ctx, cfg)
if err != nil {
    var configErr *azrealtime.ConfigError
    var connErr *azrealtime.ConnectionError
    
    switch {
    case errors.As(err, &configErr):
        log.Printf("Configuration error in %s: %s", configErr.Field, configErr.Message)
    case errors.As(err, &connErr):
        log.Printf("Connection failed: %v", connErr.Cause)
    default:
        log.Printf("Unexpected error: %v", err)
    }
}
```

### Audio Processing

```go
// Set up audio assembler
audioAssembler := azrealtime.NewAudioAssembler()

client.OnResponseAudioDelta(func(event azrealtime.ResponseAudioDelta) {
    audioAssembler.OnDelta(event)
})

client.OnResponseAudioDone(func(event azrealtime.ResponseAudioDone) {
    pcmData := audioAssembler.OnDone(event.ResponseID)
    
    // Convert to WAV for saving/playback
    wavData := azrealtime.WAVFromPCM16Mono(pcmData, azrealtime.DefaultSampleRate)
    os.WriteFile("response.wav", wavData, 0644)
})

// Send audio input
audioChunk := make([]byte, azrealtime.PCM16BytesFor(200, azrealtime.DefaultSampleRate))
client.AppendPCM16(ctx, audioChunk)
client.InputCommit(ctx) // Signal end of input
```

### Session Management

```go
session := azrealtime.Session{
    Voice:             azrealtime.Ptr("alloy"), // Voice selection
    Instructions:      azrealtime.Ptr("Custom system prompt..."),
    InputAudioFormat:  azrealtime.Ptr("pcm16"),
    OutputAudioFormat: azrealtime.Ptr("pcm16"),
    InputTranscription: &azrealtime.InputTranscription{
        Model: "whisper-1",
        Language: "en",
    },
    TurnDetection: &azrealtime.TurnDetection{
        Type:              "server_vad",
        Threshold:         0.5,    // Sensitivity (0.0-1.0)
        PrefixPaddingMS:   300,    // Audio before speech
        SilenceDurationMS: 1000,   // Silence to end turn
        CreateResponse:    true,   // Auto-respond
    },
}

err := client.SessionUpdate(ctx, session)
```

## Examples

See the [`examples/`](./examples/) directory for comprehensive examples:

- **[`ws-minimal/`](./examples/ws-minimal/)**: Basic WebSocket usage
- **[`comprehensive/`](./examples/comprehensive/)**: Production-ready patterns
- **[`webrtc-browser/`](./examples/webrtc-browser/)**: Browser WebRTC integration

Each example includes detailed documentation and error handling patterns.

## Testing

Run the full test suite:

```bash
# Run all tests
go test -v ./...

# Run with coverage
go test -cover ./...

# Run specific test patterns
go test -v ./azrealtime -run TestDial
```

The library includes:
- Unit tests for all core functionality
- Integration tests with mock servers
- Benchmarks for performance validation
- 95%+ test coverage

## WebRTC Support

For browser integration using WebRTC:

1. **Start the ephemeral token server:**
   ```bash
   cd cmd/ephemeral-issuer
   go run main.go
   ```

2. **Configure your web application:**
   ```javascript
   // Get ephemeral token from your server
   const response = await fetch('/api/ephemeral-token');
   const { sessionId, ephemeralKey } = await response.json();
   
   // Use with WebRTC client
   const client = new RealtimeWebRTCClient(sessionId, ephemeralKey);
   ```

See [`examples/webrtc-browser/`](./examples/webrtc-browser/) for a complete implementation.

## API Reference

### Core Types

- **`Config`**: Client configuration options
- **`Client`**: Main WebSocket client
- **`Session`**: AI assistant configuration  
- **`CreateResponseOptions`**: Response generation settings

### Event Types

- **`ErrorEvent`**: API errors and warnings
- **`SessionCreated/Updated`**: Session lifecycle
- **`ResponseTextDelta/Done`**: Streaming text responses
- **`ResponseAudioDelta/Done`**: Streaming audio responses
- **`RateLimitsUpdated`**: Rate limiting information

### Error Types

- **`ConfigError`**: Configuration validation errors
- **`ConnectionError`**: Network and connection errors
- **`SendError`**: Message transmission errors
- **`EventError`**: Event processing errors

### Utility Functions

- **`Ptr[T](v T) *T`**: Create pointer from value
- **`PCM16BytesFor(ms, rate int) int`**: Calculate audio buffer size
- **`WAVFromPCM16Mono([]byte, int) []byte`**: Convert PCM to WAV

## Publishing Your Library

To publish this library as a Go module:

1. **Create a public repository:**
   ```bash
   git init
   git remote add origin git@github.com:enesunal-m/azrealtime.git
   ```

2. **Update the module path:**
   ```bash
   # Update go.mod with your repository path
   sed -i 's|github.com/enesunal-m/azrealtime|github.com/yourusername/azrealtime|g' go.mod
   go mod tidy
   ```

3. **Tag and publish:**
   ```bash
   git add .
   git commit -m "Initial release"
   git tag v1.0.0
   git push origin main --tags
   ```

4. **Use in other projects:**
   ```bash
   go get github.com/yourusername/azrealtime@v1.0.0
   ```

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Write tests for your changes
4. Ensure all tests pass (`go test ./...`)
5. Commit your changes (`git commit -am 'Add amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

- **Documentation**: See [examples/README.md](examples/README.md) for detailed usage
- **Issues**: Report bugs and request features via [GitHub Issues](https://github.com/enesunal-m/azrealtime/issues)
- **Discussions**: Join the conversation in [GitHub Discussions](https://github.com/enesunal-m/azrealtime/discussions)

## Related Projects

- **[Azure OpenAI Documentation](https://docs.microsoft.com/en-us/azure/cognitive-services/openai/)**
- **[OpenAI Realtime API Documentation](https://platform.openai.com/docs/guides/realtime)**
- **[Pion WebRTC](https://github.com/pion/webrtc)** (used for WebRTC support)
