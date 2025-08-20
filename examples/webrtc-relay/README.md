# WebRTC Relay for Azure OpenAI Realtime API

This example demonstrates a WebRTC relay server that bridges browser audio to Azure OpenAI's Realtime API while saving conversation data and audio recordings server-side.

## Architecture

```
Browser <--WebRTC--> Relay Server <--WebRTC--> Azure OpenAI
                          |
                          ├── Saves conversation data
                          └── Records user audio
```

- **Browser**: Captures microphone audio and sends it via WebRTC
- **Relay Server**: Forwards audio between browser and Azure, saves messages and audio
- **Azure OpenAI**: Processes speech and returns AI responses

## Features

- Real-time bidirectional audio streaming
- Voice activity detection (VAD)
- Multiple voice options (Alloy, Echo, Fable, etc.)
- Conversation transcript display
- Session configuration updates
- **Server-side conversation logging and saving**
- **HTTP endpoint to retrieve conversation history**
- **Automatic audio recording of user input**
- **Web-based audio playback and download**

## Setup

1. Set required environment variables:
```bash
export AZURE_OPENAI_ENDPOINT="your-endpoint"
export AZURE_OPENAI_API_KEY="your-api-key"
export AZURE_OPENAI_REALTIME_DEPLOYMENT="your-deployment"
export AZURE_OPENAI_REGION="your-region"
```

2. Start the relay server:
```bash
cd server
go run main.go
```

3. Open http://localhost:8085 in your browser

4. Click "Connect & Send Audio" and allow microphone access

## Data Saving

### Conversation Data
The relay server automatically:
- Captures all messages between browser and Azure
- Saves conversation data to JSON files every 10 messages
- Saves when the connection closes
- Files are named: `conversation_YYYY-MM-DD_HH-MM-SS.json`

### Audio Recording
The relay server also:
- Records all user audio input in OGG format (Opus codec)
- Starts recording when browser connects
- Stops recording when browser disconnects
- Files are named: `audio_session_YYYYMMDD_HHMMSS.ogg`
- Audio files can be played directly in the browser UI
- Supports download for offline analysis

### Retrieve Data

**Conversation History:**
```bash
curl http://localhost:8085/conversation
```

**List Audio Files:**
```bash
curl http://localhost:8085/audio-files
```

**Download Audio File:**
```bash
curl -O http://localhost:8085/audio/audio_session_20231225_143022.ogg
```

## Audio Quality

The relay uses Opus codec for WebRTC transmission. If audio sounds robotic:

1. **Network Issues**: Check connection quality between browser, relay, and Azure
2. **Processing Delay**: The relay adds minimal latency (< 20ms per hop)
3. **Voice Selection**: Try different voices (Alloy tends to be most natural)
4. **Browser Audio**: Ensure browser has good microphone access and quality

The relay itself should not degrade audio quality as it forwards RTP packets without re-encoding.

## API Endpoints

- `POST /offer` - WebRTC offer/answer exchange
- `POST /ice-candidate` - ICE candidate exchange  
- `GET /conversation` - Retrieve conversation history
- `GET /audio-files` - List recorded audio files
- `GET /audio/{filename}` - Download specific audio file
- `GET /` - Serve frontend files

## File Formats

### Audio Files (OGG/Opus)
- Format: OGG container with Opus codec
- Sample Rate: 48kHz
- Channels: 2 (stereo)
- Can be played in most modern browsers and media players
- Ideal for speech analysis and archival

### Conversation Files (JSON)
- Contains timestamped messages with full metadata
- Includes message type, direction, and complete data payload
- Useful for conversation analysis and debugging

## Troubleshooting

1. **No audio/conversation**: Check environment variables and server logs
2. **Poor audio quality**: 
   - Test direct browser-to-Azure connection to isolate relay issues
   - Check network bandwidth and latency
   - Try different voice models
3. **Messages not saving**: Check file permissions in server directory
4. **Audio files not recording**: 
   - Ensure server has write permissions
   - Check disk space
   - Verify OGG writer initialization in logs

## How It Works

1. Browser establishes WebRTC connection with relay server
2. Relay mints ephemeral token for Azure authentication
3. Relay establishes second WebRTC connection with Azure
4. Audio RTP packets are forwarded between connections
5. User audio is saved to OGG files via the oggwriter
6. Data channel messages are logged and forwarded
7. Conversation data is saved to JSON files 