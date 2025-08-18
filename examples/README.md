# Azure OpenAI Realtime API Examples

This directory contains examples demonstrating how to use the `azrealtime` library for various use cases.

## Examples

### 1. WebSocket Minimal (`ws-minimal/`)

A basic example showing the simplest possible integration:

```bash
cd ws-minimal
export AZURE_OPENAI_ENDPOINT="https://your-resource.openai.azure.com"
export AZURE_OPENAI_REALTIME_DEPLOYMENT="your-deployment-name"  
export AZURE_OPENAI_API_KEY="your-api-key"
go run main.go
```

**Features demonstrated:**
- Basic connection setup
- Simple text response generation
- Audio file output

### 2. Comprehensive (`comprehensive/`)

A full-featured example with production-ready patterns:

```bash
cd comprehensive
export AZURE_OPENAI_ENDPOINT="https://your-resource.openai.azure.com"
export AZURE_OPENAI_REALTIME_DEPLOYMENT="your-deployment-name"
export AZURE_OPENAI_API_KEY="your-api-key"
go run main.go
```

**Features demonstrated:**
- Comprehensive error handling
- Graceful shutdown with signals
- Session configuration
- Audio processing and WAV file output
- Rate limit monitoring
- Structured logging
- Event handler setup

### 3. WebRTC Browser (`webrtc-browser/`)

Browser-based example using WebRTC for real-time audio:

```bash
# Start the ephemeral token issuer
cd ../cmd/ephemeral-issuer
export AZURE_OPENAI_ENDPOINT="https://your-resource.openai.azure.com"
export AZURE_OPENAI_API_KEY="your-api-key"
go run main.go

# Open webrtc-browser/index.html in your browser
```

**Features demonstrated:**
- WebRTC audio streaming
- Browser integration
- Ephemeral token authentication
- Real-time audio processing

## Common Environment Variables

All examples require these environment variables:

```bash
# Required
export AZURE_OPENAI_ENDPOINT="https://your-resource.openai.azure.com"
export AZURE_OPENAI_REALTIME_DEPLOYMENT="your-deployment-name"
export AZURE_OPENAI_API_KEY="your-api-key"

# Optional (with defaults)
export AZURE_OPENAI_API_VERSION="2025-04-01-preview"
```

## Getting Your Azure OpenAI Credentials

1. **Create an Azure OpenAI Resource:**
   - Go to [Azure Portal](https://portal.azure.com)
   - Create a new Azure OpenAI resource
   - Note the endpoint URL (e.g., `https://your-resource.openai.azure.com`)

2. **Deploy a GPT-4o Realtime Model:**
   - In Azure OpenAI Studio, go to Deployments
   - Create a new deployment with a GPT-4o Realtime model
   - Note the deployment name

3. **Get API Key:**
   - In the Azure Portal, go to your OpenAI resource
   - Navigate to "Keys and Endpoint" 
   - Copy one of the API keys

## Audio Format Requirements

The Azure OpenAI Realtime API uses:
- **Sample Rate:** 24,000 Hz
- **Format:** 16-bit PCM (little-endian)
- **Channels:** Mono (1 channel)
- **Encoding:** Raw PCM or base64-encoded for transmission

Use the provided utility functions:
```go
// Calculate buffer size for audio duration
bytes := azrealtime.PCM16BytesFor(200, azrealtime.DefaultSampleRate) // 200ms

// Convert PCM to WAV for saving/playback
wavData := azrealtime.WAVFromPCM16Mono(pcmData, azrealtime.DefaultSampleRate)
```

## Error Handling Best Practices

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

## Session Configuration

Configure the AI assistant's behavior:

```go
session := azrealtime.Session{
    Voice:             azrealtime.Ptr("alloy"), // or "echo", "fable", etc.
    Instructions:      azrealtime.Ptr("You are a helpful assistant..."),
    InputAudioFormat:  azrealtime.Ptr("pcm16"),
    OutputAudioFormat: azrealtime.Ptr("pcm16"),
    TurnDetection: &azrealtime.TurnDetection{
        Type:              "server_vad",
        Threshold:         0.5,    // Voice activity detection sensitivity
        PrefixPaddingMS:   300,    // Audio before speech starts
        SilenceDurationMS: 1000,   // Silence to end turn
        CreateResponse:    true,   // Auto-generate responses
    },
}
```

## Streaming Responses

Handle streaming text and audio:

```go
textAssembler := azrealtime.NewTextAssembler()
audioAssembler := azrealtime.NewAudioAssembler()

client.OnResponseTextDelta(func(event azrealtime.ResponseTextDelta) {
    textAssembler.OnDelta(event)
    fmt.Print(event.Delta) // Stream to console
})

client.OnResponseTextDone(func(event azrealtime.ResponseTextDone) {
    completeText := textAssembler.OnDone(event)
    // Process complete response
})
```

## Production Deployment

For production use:

1. **Use environment-specific configurations**
2. **Implement proper logging and monitoring**
3. **Set appropriate timeouts and limits**
4. **Handle network failures gracefully** 
5. **Use connection pooling if needed**
6. **Monitor rate limits and usage**

See the `comprehensive/` example for production-ready patterns.

## Troubleshooting

### Common Issues

1. **Authentication Errors:**
   - Verify API key is correct
   - Check endpoint URL format
   - Ensure deployment exists and is active

2. **Connection Failures:**
   - Check network connectivity
   - Verify firewall settings
   - Try increasing DialTimeout

3. **Audio Issues:**
   - Ensure correct PCM16 format (16-bit, 24kHz, mono)
   - Check for even number of bytes in audio data
   - Verify sample rate matches DefaultSampleRate

4. **Rate Limiting:**
   - Monitor RateLimitsUpdated events
   - Implement exponential backoff
   - Consider request batching

### Debug Logging

Enable detailed logging:

```go
cfg := azrealtime.Config{
    // ... other config
    Logger: func(event string, fields map[string]any) {
        log.Printf("[DEBUG] %s: %+v", event, fields)
    },
}
```

Common log events:
- `ws_connected`: WebSocket connection established
- `bad_event_json`: JSON parsing errors
- Custom events from your application

## Support

For issues with:
- **The azrealtime library:** Check the repository issues
- **Azure OpenAI service:** Contact Azure support  
- **API limitations:** Refer to Azure OpenAI documentation