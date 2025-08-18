// Package azrealtime provides a production-ready Go client for Azure OpenAI's GPT-4o Realtime API.
//
// This library enables real-time bidirectional communication with Azure OpenAI's GPT-4o model
// through WebSockets, supporting both text and audio interactions. It handles connection management,
// event dispatching, and provides utilities for audio/text processing.
//
// Key Features:
//   - WebSocket client for Azure OpenAI Realtime API
//   - Event-driven architecture with callback handlers
//   - Audio streaming with PCM16 format support
//   - Text streaming with delta and completion events
//   - Session management and configuration
//   - Connection resilience with ping/pong keepalives
//   - WebRTC support for browser integration
//
// Basic Usage:
//
//	cfg := azrealtime.Config{
//		ResourceEndpoint: "https://your-resource.openai.azure.com",
//		Deployment:       "gpt-4o-realtime-preview",
//		APIVersion:       "2025-04-01-preview",
//		Credential:       azrealtime.APIKey("your-api-key"),
//	}
//	client, err := azrealtime.Dial(ctx, cfg)
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer client.Close()
//
// The client provides callback methods for handling various events:
//   - OnResponseTextDelta: Handle streaming text responses
//   - OnResponseAudioDelta: Handle streaming audio responses  
//   - OnError: Handle API errors
//   - OnSessionCreated/Updated: Handle session lifecycle events
//
// This package is designed for production use with proper error handling,
// logging support, and resource cleanup.
package azrealtime
