package azrealtime

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestAudioAssembler(t *testing.T) {
	assembler := NewAudioAssembler()

	// Test adding delta events
	delta1 := ResponseAudioDelta{
		ResponseID:  "resp_123",
		DeltaBase64: base64.StdEncoding.EncodeToString([]byte("Hello")),
	}
	delta2 := ResponseAudioDelta{
		ResponseID:  "resp_123",
		DeltaBase64: base64.StdEncoding.EncodeToString([]byte(" World")),
	}

	// Add first delta
	err := assembler.OnDelta(delta1)
	if err != nil {
		t.Fatalf("failed to add first delta: %v", err)
	}

	// Add second delta
	err = assembler.OnDelta(delta2)
	if err != nil {
		t.Fatalf("failed to add second delta: %v", err)
	}

	// Get complete audio
	complete := assembler.OnDone("resp_123")
	expected := "Hello World"

	if string(complete) != expected {
		t.Errorf("expected %q, got %q", expected, string(complete))
	}

	// Verify data is cleaned up
	remaining := assembler.OnDone("resp_123")
	if len(remaining) != 0 {
		t.Errorf("expected empty data after cleanup, got %v", remaining)
	}
}

func TestAudioAssembler_InvalidBase64(t *testing.T) {
	assembler := NewAudioAssembler()

	delta := ResponseAudioDelta{
		ResponseID:  "resp_123",
		DeltaBase64: "invalid-base64!",
	}

	err := assembler.OnDelta(delta)
	if err == nil {
		t.Error("expected error for invalid base64, got nil")
	}
}

func TestPCM16BytesFor(t *testing.T) {
	tests := []struct {
		name       string
		ms         int
		sampleRate int
		expected   int
	}{
		{
			name:       "200ms at 24kHz",
			ms:         200,
			sampleRate: 24000,
			expected:   9600, // (200 * 24000 * 2) / 1000
		},
		{
			name:       "1000ms at 16kHz",
			ms:         1000,
			sampleRate: 16000,
			expected:   32000, // (1000 * 16000 * 2) / 1000
		},
		{
			name:       "0ms",
			ms:         0,
			sampleRate: 24000,
			expected:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PCM16BytesFor(tt.ms, tt.sampleRate)
			if result != tt.expected {
				t.Errorf("expected %d bytes, got %d", tt.expected, result)
			}
		})
	}
}

func TestWAVFromPCM16Mono(t *testing.T) {
	// Create simple test PCM data (4 bytes = 2 samples)
	pcmData := []byte{0x00, 0x01, 0xFF, 0xFE} // Little-endian 16-bit samples
	sampleRate := 24000

	wav := WAVFromPCM16Mono(pcmData, sampleRate)

	// Check WAV file structure
	if len(wav) != 44+len(pcmData) {
		t.Errorf("expected WAV length %d, got %d", 44+len(pcmData), len(wav))
	}

	// Check RIFF header
	if !bytes.Equal(wav[0:4], []byte("RIFF")) {
		t.Error("missing RIFF header")
	}

	// Check WAVE format
	if !bytes.Equal(wav[8:12], []byte("WAVE")) {
		t.Error("missing WAVE format")
	}

	// Check fmt chunk
	if !bytes.Equal(wav[12:16], []byte("fmt ")) {
		t.Error("missing fmt chunk")
	}

	// Check data chunk
	if !bytes.Equal(wav[36:40], []byte("data")) {
		t.Error("missing data chunk")
	}

	// Check that PCM data is correctly appended
	if !bytes.Equal(wav[44:], pcmData) {
		t.Error("PCM data not correctly appended")
	}
}

func TestWAVFromPCM16Mono_EmptyData(t *testing.T) {
	wav := WAVFromPCM16Mono([]byte{}, 24000)

	// Should still create valid WAV header
	if len(wav) != 44 {
		t.Errorf("expected WAV length 44 for empty PCM, got %d", len(wav))
	}
}

func BenchmarkAudioAssembler(b *testing.B) {
	assembler := NewAudioAssembler()
	testData := base64.StdEncoding.EncodeToString(make([]byte, 1024))

	delta := ResponseAudioDelta{
		ResponseID:  "resp_123",
		DeltaBase64: testData,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		delta.ResponseID = "resp_" + string(rune(i))
		_ = assembler.OnDelta(delta)
		assembler.OnDone(delta.ResponseID)
	}
}

func BenchmarkWAVFromPCM16Mono(b *testing.B) {
	pcmData := make([]byte, 9600) // 200ms at 24kHz

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = WAVFromPCM16Mono(pcmData, 24000)
	}
}
