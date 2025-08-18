// Package main demonstrates comprehensive usage of the azrealtime library
// including error handling, audio processing, and session management.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/enesunal-m/azrealtime"
)

func main() {
	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nShutting down gracefully...")
		cancel()
	}()

	if err := run(ctx); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}

func run(ctx context.Context) error {
	// Validate required environment variables
	endpoint := os.Getenv("AZURE_OPENAI_ENDPOINT")
	if endpoint == "" {
		return fmt.Errorf("AZURE_OPENAI_ENDPOINT environment variable is required")
	}
	
	deployment := os.Getenv("AZURE_OPENAI_REALTIME_DEPLOYMENT")
	if deployment == "" {
		return fmt.Errorf("AZURE_OPENAI_REALTIME_DEPLOYMENT environment variable is required")
	}
	
	apiKey := os.Getenv("AZURE_OPENAI_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("AZURE_OPENAI_API_KEY environment variable is required")
	}

	// Configure the client with all available options
	cfg := azrealtime.Config{
		ResourceEndpoint: endpoint,
		Deployment:       deployment,
		APIVersion:       "2025-04-01-preview",
		Credential:       azrealtime.APIKey(apiKey),
		DialTimeout:      30 * time.Second,
		StructuredLogger: azrealtime.NewLogger(azrealtime.LogLevelDebug),
	}

	// Create client with proper error handling
	client, err := azrealtime.Dial(ctx, cfg)
	if err != nil {
		// Handle different error types
		var configErr *azrealtime.ConfigError
		var connErr *azrealtime.ConnectionError
		
		switch {
		case ErrorAs(err, &configErr):
			return fmt.Errorf("configuration error in field %q: %s", configErr.Field, configErr.Message)
		case ErrorAs(err, &connErr):
			return fmt.Errorf("connection failed (%s): %v", connErr.Operation, connErr.Cause)
		default:
			return fmt.Errorf("failed to create client: %v", err)
		}
	}
	defer func() {
		if err := client.Close(); err != nil {
			log.Printf("Error closing client: %v", err)
		}
	}()

	// Set up event handlers
	setupEventHandlers(client)

	// Configure session with comprehensive settings
	session := azrealtime.Session{
		Voice:             azrealtime.Ptr("alloy"),
		Instructions:      azrealtime.Ptr("You are a helpful AI assistant. Speak clearly and concisely."),
		InputAudioFormat:  azrealtime.Ptr("pcm16"),
		OutputAudioFormat: azrealtime.Ptr("pcm16"),
		InputTranscription: &azrealtime.InputTranscription{
			Model:    "whisper-1",
			Language: "en",
		},
		TurnDetection: &azrealtime.TurnDetection{
			Type:              "server_vad",
			Threshold:         0.5,
			PrefixPaddingMS:   300,
			SilenceDurationMS: 1000,
			CreateResponse:    true,
		},
	}

	// Update session with error handling
	if err := client.SessionUpdate(ctx, session); err != nil {
		var sendErr *azrealtime.SendError
		if ErrorAs(err, &sendErr) {
			return fmt.Errorf("failed to update session (%s): %v", sendErr.EventType, sendErr.Cause)
		}
		return fmt.Errorf("session update failed: %v", err)
	}

	// Create an initial response
	opts := azrealtime.CreateResponseOptions{
		Modalities:  []string{"text", "audio"},
		Prompt:      "Please introduce yourself and explain what you can help me with.",
		Temperature: 0.8,
	}

	eventID, err := client.CreateResponse(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to create response: %v", err)
	}
	
	log.Printf("Created response request with event ID: %s", eventID)

	// Simulate sending audio data (replace with real audio input)
	go func() {
		time.Sleep(2 * time.Second) // Wait for initial response
		
		// Generate sample audio data (silence)
		audioData := make([]byte, azrealtime.PCM16BytesFor(200, azrealtime.DefaultSampleRate))
		
		if err := client.AppendPCM16(ctx, audioData); err != nil {
			log.Printf("Failed to append audio: %v", err)
			return
		}
		
		if err := client.InputCommit(ctx); err != nil {
			log.Printf("Failed to commit audio: %v", err)
			return
		}
		
		log.Println("Sent sample audio data")
	}()

	// Keep the application running until context is canceled
	<-ctx.Done()
	return nil
}

// setupEventHandlers configures all event handlers with comprehensive error handling
func setupEventHandlers(client *azrealtime.Client) {
	// Set up audio and text assemblers
	audioAssembler := azrealtime.NewAudioAssembler()
	textAssembler := azrealtime.NewTextAssembler()

	// Error handler
	client.OnError(func(event azrealtime.ErrorEvent) {
		log.Printf("API Error: %s - %s", event.Error.Type, event.Error.Message)
		if event.Error.Content != "" {
			log.Printf("Error content: %s", event.Error.Content)
		}
	})

	// Session lifecycle handlers
	client.OnSessionCreated(func(event azrealtime.SessionCreated) {
		log.Printf("Session created: ID=%s, Model=%s, Voice=%s", 
			event.Session.ID, event.Session.Model, event.Session.Voice)
		if len(event.Session.Modalities) > 0 {
			log.Printf("Supported modalities: %v", event.Session.Modalities)
		}
	})

	client.OnSessionUpdated(func(event azrealtime.SessionUpdated) {
		log.Printf("Session updated: %+v", event.Session)
	})

	// Rate limiting information
	client.OnRateLimitsUpdated(func(event azrealtime.RateLimitsUpdated) {
		for _, limit := range event.RateLimits {
			log.Printf("Rate limit %s: %d/%d (resets in %ds)", 
				limit.Name, limit.Remaining, limit.Limit, limit.ResetSeconds)
		}
	})

	// Text response handlers
	client.OnResponseTextDelta(func(event azrealtime.ResponseTextDelta) {
		textAssembler.OnDelta(event)
		fmt.Print(event.Delta) // Stream text to console
	})

	client.OnResponseTextDone(func(event azrealtime.ResponseTextDone) {
		completeText := textAssembler.OnDone(event)
		fmt.Printf("\n[Text Response Complete - %d characters]\n", len(completeText))
	})

	// Audio response handlers
	client.OnResponseAudioDelta(func(event azrealtime.ResponseAudioDelta) {
		if err := audioAssembler.OnDelta(event); err != nil {
			log.Printf("Error processing audio delta: %v", err)
		}
	})

	client.OnResponseAudioDone(func(event azrealtime.ResponseAudioDone) {
		pcmData := audioAssembler.OnDone(event.ResponseID)
		log.Printf("Audio response complete: %d bytes of PCM data", len(pcmData))
		
		// Convert to WAV and save (optional)
		if len(pcmData) > 0 {
			wavData := azrealtime.WAVFromPCM16Mono(pcmData, azrealtime.DefaultSampleRate)
			filename := fmt.Sprintf("response_%s.wav", event.ResponseID)
			if err := os.WriteFile(filename, wavData, 0644); err != nil {
				log.Printf("Failed to save audio file %s: %v", filename, err)
			} else {
				log.Printf("Saved audio response to %s", filename)
			}
		}
	})
}

// Note: Using structured logging with azrealtime.NewLogger() instead of custom logger function

// ErrorAs is a helper function for error type assertions
func ErrorAs(err error, target interface{}) bool {
	switch target := target.(type) {
	case **azrealtime.ConfigError:
		if configErr, ok := err.(*azrealtime.ConfigError); ok {
			*target = configErr
			return true
		}
	case **azrealtime.ConnectionError:
		if connErr, ok := err.(*azrealtime.ConnectionError); ok {
			*target = connErr
			return true
		}
	case **azrealtime.SendError:
		if sendErr, ok := err.(*azrealtime.SendError); ok {
			*target = sendErr
			return true
		}
	}
	return false
}