package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/enesunal-m/azrealtime"
)

const SampleRate = 24000

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nShutting down gracefully...")
		cancel()
	}()

	if os.Getenv("TEST_AUDIO_FILE") == "true" {
		log.Println("Running in audio file test mode...")
		if err := testWithAudioFile(ctx); err != nil {
			log.Fatalf("Audio file test error: %v", err)
		}
		return
	}

	log.Println("Set TEST_AUDIO_FILE=true to test with audio file")
}

func testWithAudioFile(ctx context.Context) error {
	endpoint := os.Getenv("AZURE_OPENAI_ENDPOINT")
	if endpoint == "" {
		return errors.New("AZURE_OPENAI_ENDPOINT environment variable is required")
	}
	deployment := os.Getenv("AZURE_OPENAI_REALTIME_DEPLOYMENT")
	if deployment == "" {
		return errors.New("AZURE_OPENAI_REALTIME_DEPLOYMENT environment variable is required")
	}
	apiKey := os.Getenv("AZURE_OPENAI_API_KEY")
	if apiKey == "" {
		return errors.New("AZURE_OPENAI_API_KEY environment variable is required")
	}

	cfg := azrealtime.Config{
		ResourceEndpoint: endpoint,
		Deployment:       deployment,
		APIVersion:       "2025-04-01-preview",
		Credential:       azrealtime.APIKey(apiKey),
		DialTimeout:      30 * time.Second,
		StructuredLogger: azrealtime.NewLogger(azrealtime.LogLevelInfo),
	}

	client, err := azrealtime.Dial(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close()

	setupEventHandlers(client)

	// Configure session - let server VAD handle everything
	session := azrealtime.Session{
		Voice:             azrealtime.Ptr("alloy"),
		Instructions:      azrealtime.Ptr("You are a helpful AI assistant. Please respond to what you hear."),
		InputAudioFormat:  azrealtime.Ptr("pcm16"),
		OutputAudioFormat: azrealtime.Ptr("pcm16"),
		InputTranscription: &azrealtime.InputTranscription{
			Model:    "whisper-1",
			Language: "en", // Change to your language
		},
		TurnDetection: &azrealtime.TurnDetection{
			Type:              "server_vad",
			CreateResponse:    true, // Let server auto-create responses
			InterruptResponse: true, // Allow interrupting ongoing responses
			Threshold:         0.5,
			PrefixPaddingMS:   300,
			SilenceDurationMS: 500, // Shorter for quicker response
		},
	}

	if err := client.SessionUpdate(ctx, session); err != nil {
		return fmt.Errorf("session update failed: %w", err)
	}

	log.Println("Session configured. Processing audio file...")

	// Check if audio file exists
	const fname = "sound.m4a"
	if _, err := os.Stat(fname); os.IsNotExist(err) {
		return fmt.Errorf("audio file %s not found", fname)
	}

	pcmData, err := decodeToPCM16LE(fname)
	if err != nil {
		return fmt.Errorf("decode error: %w", err)
	}
	log.Printf("Decoded PCM length: %d bytes (%.2f seconds)",
		len(pcmData), float64(len(pcmData))/(2.0*SampleRate))

	// Send audio and let server VAD handle detection and response
	if err := client.AppendPCM16(ctx, pcmData); err != nil {
		return fmt.Errorf("failed to append audio: %w", err)
	}
	log.Println("Audio sent. Waiting for server VAD to detect speech...")

	// Wait for response - server will handle everything
	select {
	case <-ctx.Done():
		return nil
	case <-time.After(15 * time.Second):
		log.Println("Timeout waiting for response")
		return nil
	}
}

func decodeToPCM16LE(filename string) ([]byte, error) {
	cmd := exec.Command("ffmpeg",
		"-nostdin", "-v", "error",
		"-i", filename,
		"-f", "s16le", "-acodec", "pcm_s16le",
		"-ac", "1", "-ar", fmt.Sprintf("%d", SampleRate),
		"pipe:1",
	)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg decode failed: %w", err)
	}
	return out.Bytes(), nil
}

func setupEventHandlers(client *azrealtime.Client) {
	audioAssembler := azrealtime.NewAudioAssembler()
	textAssembler := azrealtime.NewTextAssembler()

	client.OnError(func(event azrealtime.ErrorEvent) {
		log.Printf("❌ API Error: %s - %s", event.Error.Type, event.Error.Message)
	})

	client.OnSessionCreated(func(event azrealtime.SessionCreated) {
		log.Printf("✅ Session created: %s", event.Session.ID)
	})

	client.OnSessionUpdated(func(event azrealtime.SessionUpdated) {
		log.Printf("🔄 Session updated")
	})

	// Server VAD events
	client.OnInputAudioBufferSpeechStarted(func(ev azrealtime.InputAudioBufferSpeechStarted) {
		log.Printf("🎤 Speech detected at %d ms", ev.AudioStartMs)
	})

	client.OnInputAudioBufferSpeechStopped(func(ev azrealtime.InputAudioBufferSpeechStopped) {
		log.Printf("⏹️  Speech ended at %d ms", ev.AudioEndMs)
	})

	client.OnInputAudioBufferCommitted(func(ev azrealtime.InputAudioBufferCommitted) {
		log.Printf("✅ Audio committed: %s", ev.ItemID)
	})

	// Response lifecycle
	client.OnResponseCreated(func(ev azrealtime.ResponseCreated) {
		log.Printf("🤖 Response created: %s", ev.Response.ID)
		fmt.Println("\n--- AI RESPONSE STARTING ---")
	})

	client.OnResponseDone(func(ev azrealtime.ResponseDone) {
		log.Printf("✅ Response done: %s (status: %s)", ev.Response.ID, ev.Response.Status)
		fmt.Println("--- AI RESPONSE COMPLETE ---")
	})

	// Text streaming - show response in real-time
	client.OnResponseTextDelta(func(event azrealtime.ResponseTextDelta) {
		textAssembler.OnDelta(event)
		fmt.Print(event.Delta) // This shows the text as it streams
	})

	client.OnResponseTextDone(func(event azrealtime.ResponseTextDone) {
		completeText := textAssembler.OnDone(event)
		fmt.Printf("\n📝 [Complete text response: %d characters]\n", len(completeText))

		// Optionally show the complete text
		if len(completeText) > 0 {
			fmt.Printf("Full response: %s\n", completeText)
		}
	})

	// Audio streaming
	client.OnResponseAudioDelta(func(event azrealtime.ResponseAudioDelta) {
		if err := audioAssembler.OnDelta(event); err != nil {
			log.Printf("Error processing audio delta: %v", err)
			return
		}
		// Show progress
		fmt.Print("🔊")
	})

	client.OnResponseAudioDone(func(event azrealtime.ResponseAudioDone) {
		pcmData := audioAssembler.OnDone(event.ResponseID)
		log.Printf("\n🔊 Audio complete: %d bytes", len(pcmData))

		if len(pcmData) > 0 {
			wavData := azrealtime.WAVFromPCM16Mono(pcmData, azrealtime.DefaultSampleRate)
			filename := fmt.Sprintf("response_%s.wav", event.ResponseID)
			if err := os.WriteFile(filename, wavData, 0644); err != nil {
				log.Printf("Failed to save audio: %v", err)
			} else {
				log.Printf("💾 Saved audio: %s", filename)

				// Try to play the audio automatically on macOS
				if _, err := exec.Command("which", "afplay").Output(); err == nil {
					log.Printf("🔊 Playing audio...")
					go func() {
						if err := exec.Command("afplay", filename).Run(); err != nil {
							log.Printf("Failed to play audio: %v", err)
						}
					}()
				} else {
					log.Printf("💡 To hear the response, play: %s", filename)
				}
			}
		}
	})

	// Transcription events - show what the AI heard
	client.OnConversationItemInputAudioTranscriptionCompleted(func(event azrealtime.ConversationItemInputAudioTranscriptionCompleted) {
		log.Printf("🎯 AI heard: '%s'", event.Transcript)
	})

	client.OnConversationItemInputAudioTranscriptionFailed(func(event azrealtime.ConversationItemInputAudioTranscriptionFailed) {
		log.Printf("❌ Transcription failed: %s", event.Error.Message)
	})
}
