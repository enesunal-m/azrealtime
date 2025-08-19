# Working Audio Example

This example demonstrates how to properly use the Azure OpenAI Realtime API with audio files, showing both text and audio responses.

## Setup

1. Install dependencies:
   ```bash
   go mod tidy
   ```

2. Make sure you have `ffmpeg` installed for audio conversion:
   ```bash
   # macOS
   brew install ffmpeg
   
   # Ubuntu/Debian
   sudo apt install ffmpeg
   ```

3. Set your environment variables:
   ```bash
   export AZURE_OPENAI_ENDPOINT="https://your-resource.openai.azure.com"
   export AZURE_OPENAI_REALTIME_DEPLOYMENT="your-deployment-name"  
   export AZURE_OPENAI_API_KEY="your-api-key"
   ```

4. Put an audio file named `sound.m4a` in the same directory (or modify the filename in the code).

## Running

```bash
# Build the example
go build -o audio-example main.go

# Test with audio file
TEST_AUDIO_FILE=true ./audio-example
```

## What You'll See

### Text Response
The AI's text response will stream in real-time:
```
--- AI RESPONSE STARTING ---
Hello! I heard you say something about...
📝 [Complete text response: 45 characters]
--- AI RESPONSE COMPLETE ---
```

### Audio Response  
1. **Progress indicators**: You'll see `🔊` symbols showing audio streaming
2. **Saved WAV file**: `response_[id].wav` will be saved
3. **Auto-playback**: On macOS, the audio will automatically play using `afplay`
4. **Manual playback**: On other systems, play the saved WAV file

### What the AI Heard
If transcription is enabled, you'll see:
```
🎯 AI heard: 'Your spoken text here'
```

## How It Works

1. **Server VAD**: The server automatically detects speech in your audio file
2. **Auto-commit**: When speech ends, the server commits the audio buffer  
3. **Auto-response**: The server automatically creates a response
4. **Streaming**: Both text and audio responses stream back in real-time

## Key Features Used

- **New TurnDetection fields**: `InterruptResponse: true`
- **Proper event handling**: Listen to server lifecycle events
- **Audio assembly**: Collect streaming audio chunks into complete audio
- **Transcription**: See what the AI understood from your input
- **Auto-playback**: Hear the response immediately

## Troubleshooting

If you don't see responses:
1. Check that your audio file contains clear speech
2. Verify your Azure deployment supports the Realtime API  
3. Make sure your API key has proper permissions
4. Try with a different audio format (MP3, WAV, etc.)