# Azure OpenAI Realtime Go Library

[![Build Status](https://github.com/enesunal-m/azrealtime/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/enesunal-m/azrealtime/actions/workflows/ci.yml)
[![codecov](https://codecov.io/github/enesunal-m/azrealtime/graph/badge.svg?token=GAGOEHK4NJ)](https://codecov.io/github/enesunal-m/azrealtime)
[![Go Report Card](https://goreportcard.com/badge/github.com/enesunal-m/azrealtime)](https://goreportcard.com/report/github.com/enesunal-m/azrealtime)
[![Go Reference](https://pkg.go.dev/badge/github.com/enesunal-m/azrealtime.svg)](https://pkg.go.dev/github.com/enesunal-m/azrealtime)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A production-ready Go client library for Azure OpenAI's GPT-4o Realtime API, enabling real-time bidirectional communication with voice and text.

## Features

✅ **Core Functionality**
- WebSocket-based real-time communication
- Audio streaming (PCM16, 24kHz, mono)
- Text response streaming
- Session configuration and management
- Conversation item management

✅ **Audio Processing**
- PCM16 audio input/output support
- Audio assembly and WAV conversion utilities
- Server-side voice activity detection
- Audio transcription support

✅ **Production Ready**
- Comprehensive error handling with typed errors
- Input validation and sanitization
- Structured logging with configurable levels
- Test coverage: 70.4%
- Rate limit monitoring

✅ **Developer Experience**
- Type-safe API with full documentation
- Multiple authentication methods (API Key, Bearer token)
- Event-driven architecture with 27 event types
- Conversation management (create, delete, truncate items)

> **Note**: Azure GPT-4o Realtime is in public preview. Use API version `2025-04-01-preview` and monitor the [official documentation](https://learn.microsoft.com/en-us/azure/ai-foundry/openai/realtime-audio-reference) for updates.

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

The library provides a WebSocket-based client for Azure OpenAI Realtime API:

### Core Components
- **WebSocket Client**: Real-time bidirectional communication
- **Event System**: 27 event types for comprehensive API coverage
- **Audio Processing**: PCM16 streaming and WAV conversion utilities
- **Session Management**: Voice, instructions, and turn detection configuration
- **Error Handling**: Structured error types with detailed context

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
- Validation tests for input sanitization
- Test coverage: 70.4%

## Common Issues and Solutions

### Audio File Processing
```go
// Correct approach: wait for server events before creating response
client.OnInputAudioBufferCommitted(func(event azrealtime.InputAudioBufferCommitted) {
    log.Printf("Audio committed: %s", event.ItemID)
    // Now create response
    client.CreateResponse(ctx, azrealtime.CreateResponseOptions{
        Modalities: []string{"text", "audio"},
    })
})

// Send audio and let server VAD handle it
client.AppendPCM16(ctx, audioData)
// Don't manually commit - let server VAD decide
```

### Microphone Streaming
```go
// Use server VAD, don't manually commit every chunk
session := azrealtime.Session{
    TurnDetection: &azrealtime.TurnDetection{
        Type:           "server_vad",
        CreateResponse: true, // Let server create responses automatically
    },
}
```

## API Reference

### Core Types

- **`Config`**: Client configuration options
- **`Client`**: Main WebSocket client
- **`Session`**: AI assistant configuration
- **`CreateResponseOptions`**: Response generation settings

### Event Types

**Session Events:**
- `SessionCreated` / `SessionUpdated`: Session lifecycle management
- `ErrorEvent`: API errors and warnings
- `RateLimitsUpdated`: Rate limiting information

**Audio Input Events:**
- `InputAudioBufferSpeechStarted` / `InputAudioBufferSpeechStopped`: Voice activity detection
- `InputAudioBufferCommitted` / `InputAudioBufferCleared`: Audio buffer management

**Conversation Events:**
- `ConversationItemCreated` / `ConversationItemDeleted` / `ConversationItemTruncated`: Item management
- `ConversationItemInputAudioTranscriptionCompleted` / `ConversationItemInputAudioTranscriptionFailed`: Transcription events

**Response Events:**
- `ResponseCreated` / `ResponseDone`: Response lifecycle
- `ResponseTextDelta` / `ResponseTextDone`: Streaming text responses
- `ResponseAudioDelta` / `ResponseAudioDone`: Streaming audio responses
- `ResponseAudioTranscriptDelta` / `ResponseAudioTranscriptDone`: Audio transcription streaming
- `ResponseOutputItemAdded` / `ResponseOutputItemDone`: Response item management
- `ResponseContentPartAdded` / `ResponseContentPartDone`: Content part management
- `ResponseFunctionCallArgumentsDelta` / `ResponseFunctionCallArgumentsDone`: Function call streaming

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
