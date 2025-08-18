package azrealtime

import (
    "context"
    "encoding/base64"
    "encoding/binary"
    "errors"
    "fmt"
    "time"
)

// AppendPCM16 sends PCM16 audio data to the assistant's input buffer.
// The audio should be 16-bit little-endian PCM at 24kHz sample rate.
// Audio data is automatically base64-encoded before transmission.
func (c *Client) AppendPCM16(ctx context.Context, pcmLE []byte) error {
    if ctx == nil {
        return NewSendError("input_audio_buffer.append", "", errors.New("context cannot be nil"))
    }
    if len(pcmLE) == 0 { 
        return nil // Empty data is valid (no-op)
    }
    
    // Validate PCM data format (should be even number of bytes for 16-bit samples)
    if len(pcmLE)%2 != 0 {
        return NewSendError("input_audio_buffer.append", "", errors.New("PCM16 data must have even number of bytes"))
    }
    
    // Check for reasonable size limits (prevent massive payloads)
    const maxChunkSize = 1024 * 1024 // 1MB per chunk
    if len(pcmLE) > maxChunkSize {
        return NewSendError("input_audio_buffer.append", "", 
            fmt.Errorf("PCM data too large (%d bytes), maximum is %d bytes", len(pcmLE), maxChunkSize))
    }
    
    payload := map[string]any{
        "type": "input_audio_buffer.append", 
        "audio": base64.StdEncoding.EncodeToString(pcmLE),
    }
    return c.send(ctx, payload)
}

// InputCommit signals that the current audio input is complete and ready for processing.
// This triggers the assistant to process the accumulated audio data.
func (c *Client) InputCommit(ctx context.Context) error { 
    if ctx == nil {
        return NewSendError("input_audio_buffer.commit", "", errors.New("context cannot be nil"))
    }
    return c.send(ctx, map[string]any{"type": "input_audio_buffer.commit"}) 
}

// InputClear removes all audio data from the input buffer.
// Use this to cancel/reset audio input before committing.
func (c *Client) InputClear(ctx context.Context) error  { 
    if ctx == nil {
        return NewSendError("input_audio_buffer.clear", "", errors.New("context cannot be nil"))
    }
    return c.send(ctx, map[string]any{"type": "input_audio_buffer.clear"}) 
}

// AudioAssembler collects streaming audio chunks and reassembles them into complete audio data.
// Use this to handle ResponseAudioDelta events and reconstruct the full audio response.
type AudioAssembler struct{ data map[string][]byte }

// NewAudioAssembler creates a new AudioAssembler instance.
func NewAudioAssembler() *AudioAssembler { return &AudioAssembler{data: make(map[string][]byte)} }

// OnDelta processes a ResponseAudioDelta event by decoding and appending the audio data.
// Call this from your ResponseAudioDelta event handler.
func (a *AudioAssembler) OnDelta(e ResponseAudioDelta) error {
    b, err := base64.StdEncoding.DecodeString(e.DeltaBase64); if err != nil { return err }
    a.data[e.ResponseID] = append(a.data[e.ResponseID], b...); return nil
}

// OnDone retrieves and removes the complete audio data for a given response ID.
// Call this when you receive a ResponseAudioDone event to get the final audio.
func (a *AudioAssembler) OnDone(id string) []byte { buf := a.data[id]; delete(a.data, id); return buf }

// WAVFromPCM16Mono converts raw PCM16 audio data to a complete WAV file.
// This is useful for saving audio responses to disk or streaming to audio players.
// The input should be 16-bit little-endian PCM data (mono channel).
func WAVFromPCM16Mono(pcm []byte, sampleRate int) []byte {
    blockAlign := uint16(2)
    byteRate := uint32(sampleRate) * uint32(blockAlign)
    dataLen := uint32(len(pcm))
    riffLen := 36 + dataLen
    out := make([]byte, 44+len(pcm))
    
    // RIFF header
    copy(out[0:], []byte("RIFF"))
    binary.LittleEndian.PutUint32(out[4:], riffLen)
    copy(out[8:], []byte("WAVE"))
    
    // Format chunk
    copy(out[12:], []byte("fmt "))
    binary.LittleEndian.PutUint32(out[16:], 16)        // fmt chunk size
    binary.LittleEndian.PutUint16(out[20:], 1)         // audio format (PCM)
    binary.LittleEndian.PutUint16(out[22:], 1)         // num channels (mono)
    binary.LittleEndian.PutUint32(out[24:], uint32(sampleRate))
    binary.LittleEndian.PutUint32(out[28:], byteRate)
    binary.LittleEndian.PutUint16(out[32:], blockAlign)
    binary.LittleEndian.PutUint16(out[34:], 16)        // bits per sample
    
    // Data chunk
    copy(out[36:], []byte("data"))
    binary.LittleEndian.PutUint32(out[40:], dataLen)
    copy(out[44:], pcm)
    return out
}

// Audio processing constants and utilities

// DefaultChunkMS is the recommended chunk size for streaming audio (200ms).
const DefaultChunkMS = 200

// DefaultSampleRate is the standard sample rate used by Azure OpenAI Realtime API (24kHz).
const DefaultSampleRate = 24000

// PCM16BytesFor calculates the number of bytes needed for PCM16 audio of given duration.
// Formula: (milliseconds * sampleRate * 2 bytes per sample) / 1000
func PCM16BytesFor(ms int, sampleRate int) int { return (ms * sampleRate * 2) / 1000 }

// SleepApprox provides a simple sleep utility for timing audio operations.
func SleepApprox(ms int) { time.Sleep(time.Duration(ms) * time.Millisecond) }
