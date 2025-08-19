# Full-Stack WebSocket Demo

A complete full-stack implementation of Azure OpenAI Realtime API with WebSocket communication between frontend and backend.

## Architecture

```
┌─────────────────┐    WebSocket     ┌─────────────────┐    WebSocket     ┌──────────────────┐
│   Frontend      │ ◄────────────► │   Go Server     │ ◄────────────► │  Azure OpenAI    │
│   (HTML/JS)     │                │                 │                │  Realtime API    │
└─────────────────┘                └─────────────────┘                └──────────────────┘
```

### Components

1. **Frontend** (`frontend/`): HTML/JavaScript client with audio recording and real-time conversation UI
2. **Server** (`server/`): Go WebSocket server that proxies between clients and Azure OpenAI
3. **Integration**: Real-time bidirectional communication with voice activity detection and streaming responses

## Features

### Frontend Features
- 🎤 **Real-time voice recording** with audio level monitoring
- 🔊 **Audio playback** of AI responses
- 📝 **Live text streaming** as AI responds
- ⚙️ **Session configuration** (voice, language, VAD settings)
- 🎯 **Transcription display** showing what the AI heard
- 📊 **Real-time metrics** (recording time, audio levels, response count)
- 🐛 **Debug console** with detailed event logging

### Server Features
- 🔄 **WebSocket proxy** between multiple clients and Azure OpenAI
- 🎤 **Audio processing** (PCM16 conversion, base64 encoding)
- 📡 **Event streaming** (VAD events, text/audio deltas, transcriptions)
- 🔧 **Session management** (start, update, end sessions)
- 🛡️ **Error handling** with structured error messages
- 📊 **Multi-client support** with connection management

### Integration Features
- 🎯 **Server VAD** and **Semantic VAD** support
- 🔄 **Real-time audio streaming** with automatic response generation
- 📝 **Live transcription** of user speech
- 🔊 **Streaming audio responses** with automatic playback
- ⚡ **Low-latency communication** optimized for real-time interaction

## Setup

### Prerequisites

1. **Go 1.21+** installed
2. **Azure OpenAI** account with Realtime API access
3. **Modern web browser** (Chrome/Firefox/Safari)
4. **Microphone access** permissions

### Environment Variables

Create a `.env` file or set these environment variables:

```bash
# Required
export AZURE_OPENAI_ENDPOINT="https://your-resource.openai.azure.com"
export AZURE_OPENAI_REALTIME_DEPLOYMENT="your-deployment-name"
export AZURE_OPENAI_API_KEY="your-api-key"

# Optional
export PORT="8080"
```

### Installation

1. **Clone and navigate to the example:**
   ```bash
   cd examples/fullstack-ws/server
   ```

2. **Install dependencies:**
   ```bash
   go mod tidy
   ```

3. **Run the server:**
   ```bash
   go run main.go
   ```

4. **Open your browser:**
   ```
   http://localhost:8080
   ```

## Usage

### 1. Connect to Server
- Click **"Connect"** to establish WebSocket connection
- Server will automatically create an Azure OpenAI session

### 2. Configure Session
- **Voice**: Choose from 7 available voices (Alloy, Echo, etc.)
- **Language**: Set transcription language
- **Turn Detection**: Choose Server VAD or Semantic VAD
- **VAD Threshold**: Adjust sensitivity (0.0-1.0)
- **Instructions**: Set AI behavior and personality
- Click **"Update Session"** to apply changes

### 3. Voice Interaction
- Click **"Start Recording"** to begin speaking
- Watch audio level indicator and recording time
- Server VAD will automatically detect speech end and generate responses
- **Or** click **"Stop Recording"** manually
- **Or** click **"Generate Response"** to request AI response

### 4. Real-Time Responses
- **Text**: Streams live as AI speaks
- **Audio**: Plays automatically when response completes
- **Transcription**: Shows what the AI understood from your speech
- **VAD Events**: Debug panel shows speech detection events

## API Reference

### WebSocket Messages

#### Client → Server

**Start Session:**
```json
{
  "type": "start_session",
  "data": {
    "voice": "alloy",
    "instructions": "You are helpful...",
    "input_audio_format": "pcm16",
    "output_audio_format": "pcm16",
    "turn_detection": {
      "type": "server_vad",
      "threshold": 0.5,
      "create_response": true
    },
    "transcription": {
      "model": "whisper-1",
      "language": "en"
    }
  }
}
```

**Send Audio:**
```json
{
  "type": "audio_data", 
  "data": {
    "data": "base64-encoded-pcm16-audio",
    "format": "pcm16"
  }
}
```

**Update Session:**
```json
{
  "type": "update_session",
  "data": {
    "voice": "nova",
    "instructions": "New instructions..."
  }
}
```

**Create Response:**
```json
{
  "type": "create_response",
  "data": {
    "modalities": ["text", "audio"]
  }
}
```

#### Server → Client

**Session Started:**
```json
{
  "type": "session_started",
  "data": {
    "client_id": "client_1234567890"
  }
}
```

**VAD Events:**
```json
{
  "type": "vad_event",
  "data": {
    "event": "speech_started",
    "audio_start_ms": 1500,
    "item_id": "item_abc123"
  }
}
```

**Text Streaming:**
```json
{
  "type": "text_delta",
  "data": {
    "response_id": "resp_xyz789",
    "delta": "Hello there! How can I help you today?"
  }
}
```

**Audio Response:**
```json
{
  "type": "audio_done",
  "data": {
    "response_id": "resp_xyz789",
    "audio_data": "base64-encoded-pcm16-audio",
    "sample_rate": 24000
  }
}
```

**Transcription:**
```json
{
  "type": "transcript",
  "data": {
    "item_id": "item_abc123",
    "transcript": "Hello, how are you today?"
  }
}
```

## Configuration Options

### Session Configuration

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `voice` | string | Voice selection (alloy, echo, fable, etc.) | "alloy" |
| `instructions` | string | AI behavior instructions | "You are helpful..." |
| `input_audio_format` | string | Audio input format | "pcm16" |
| `output_audio_format` | string | Audio output format | "pcm16" |
| `turn_detection.type` | string | "server_vad" or "semantic_vad" | "server_vad" |
| `turn_detection.threshold` | number | VAD sensitivity (0.0-1.0) | 0.5 |
| `turn_detection.create_response` | boolean | Auto-create responses | true |
| `turn_detection.interrupt_response` | boolean | Allow response interruption | true |
| `transcription.language` | string | Language code (en, es, fr, etc.) | "en" |

### Audio Settings

- **Sample Rate**: 24kHz (Azure OpenAI requirement)
- **Format**: PCM16 (16-bit signed integers)
- **Channels**: Mono (1 channel)
- **Encoding**: Base64 for WebSocket transmission

## Development

### Adding New Features

1. **Server-side**: Add message types in `main.go`
2. **Client-side**: Add handlers in `app.js`
3. **UI**: Update `index.html` with new controls

### Error Handling

The server provides structured error messages:
```json
{
  "type": "error",
  "data": {
    "error_type": "invalid_request_error",
    "message": "Audio buffer too small",
    "content": "Expected 100ms minimum"
  }
}
```

### Debugging

- **Debug Console**: Shows all WebSocket messages and events
- **Network Tab**: Monitor WebSocket traffic in browser dev tools
- **Server Logs**: Go server outputs connection and Azure OpenAI events

## Performance Tips

1. **Audio Chunk Size**: Default 100ms chunks balance latency and efficiency
2. **Connection Limits**: Server supports multiple concurrent clients
3. **Memory Usage**: Audio data is streamed, not buffered
4. **Network**: WebSocket provides low-latency real-time communication

## Troubleshooting

### Common Issues

**"Failed to connect to Azure OpenAI"**
- Verify `AZURE_OPENAI_ENDPOINT` is correct
- Check `AZURE_OPENAI_API_KEY` has proper permissions
- Ensure deployment name matches `AZURE_OPENAI_REALTIME_DEPLOYMENT`

**"Microphone not working"**
- Grant microphone permissions in browser
- Check browser compatibility (Chrome/Firefox recommended)
- Verify microphone is not used by other applications

**"No audio playback"**
- Check browser auto-play policies
- Verify audio codec support
- Try manual audio playback if auto-play fails

**"WebSocket connection failed"**
- Check server is running on correct port
- Verify firewall settings
- Try different port with `PORT=8081 go run main.go`

### Browser Support

| Browser | Audio Recording | WebSocket | Auto-play |
|---------|----------------|-----------|-----------|
| Chrome 80+ | ✅ | ✅ | ✅* |
| Firefox 75+ | ✅ | ✅ | ✅* |
| Safari 14+ | ✅ | ✅ | ⚠️ |
| Edge 80+ | ✅ | ✅ | ✅* |

*Auto-play may require user interaction first

## Security Notes

- **CORS**: Currently allows all origins for demo purposes
- **Authentication**: No auth implemented - add JWT/session auth for production
- **Rate Limiting**: No rate limiting - add per-client limits for production
- **Input Validation**: Basic validation implemented - enhance for production

## Contributing

To extend this example:

1. Fork the repository
2. Add new features to both frontend and server
3. Test with multiple clients
4. Submit pull request with description

## License

This example is part of the azrealtime library under MIT license.