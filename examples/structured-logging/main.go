// Package main demonstrates structured logging features of azrealtime
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/enesunal-m/azrealtime"
)

func main() {
	fmt.Println("=== Azure OpenAI Realtime Structured Logging Demo ===\n")

	// Example 1: Using environment variable for log level
	fmt.Println("1. Environment-based logging (set AZREALTIME_LOG_LEVEL=DEBUG):")
	os.Setenv("AZREALTIME_LOG_LEVEL", "DEBUG")
	logger1 := azrealtime.NewLoggerFromEnv()
	
	logger1.Debug("debug_example", map[string]interface{}{
		"level": "debug",
		"msg":   "This is a debug message",
	})
	logger1.Info("info_example", map[string]interface{}{
		"level": "info", 
		"msg":   "This is an info message",
	})
	logger1.Warn("warn_example", map[string]interface{}{
		"level": "warn",
		"msg":   "This is a warning message",
	})
	logger1.Error("error_example", map[string]interface{}{
		"level": "error",
		"msg":   "This is an error message",
	})

	fmt.Println("\n2. Filtered logging (set to WARN level):")
	logger2 := azrealtime.NewLogger(azrealtime.LogLevelWarn)
	
	// These won't be logged (below threshold)
	logger2.Debug("debug_filtered", map[string]interface{}{"visible": false})
	logger2.Info("info_filtered", map[string]interface{}{"visible": false})
	
	// These will be logged (at or above threshold)
	logger2.Warn("warn_visible", map[string]interface{}{"visible": true})
	logger2.Error("error_visible", map[string]interface{}{"visible": true})

	fmt.Println("\n3. Contextual logging:")
	sessionLogger := logger1.WithContext(map[string]interface{}{
		"session_id": "demo-session-123",
		"user_id":    "user-456",
	})
	
	sessionLogger.Info("user_action", map[string]interface{}{
		"action": "connect",
		"status": "success",
	})

	fmt.Println("\n4. Client configuration with structured logging:")
	demonstrateClientConfig()

	fmt.Println("\n=== Demo Complete ===")
}

func demonstrateClientConfig() {
	// Show how to configure client with different logging options
	
	// Option 1: Simple function logger
	cfg1 := azrealtime.Config{
		ResourceEndpoint: "https://example.openai.azure.com",
		Deployment:       "gpt-4o-realtime",
		APIVersion:       "2025-04-01-preview", 
		Credential:       azrealtime.APIKey("demo-key"),
		Logger: func(event string, fields map[string]any) {
			log.Printf("SIMPLE: [%s] %+v", event, fields)
		},
	}
	fmt.Printf("Config with simple logger: %T\n", cfg1.Logger)

	// Option 2: Structured logger with custom level
	cfg2 := azrealtime.Config{
		ResourceEndpoint: "https://example.openai.azure.com",
		Deployment:       "gpt-4o-realtime", 
		APIVersion:       "2025-04-01-preview",
		Credential:       azrealtime.APIKey("demo-key"),
		StructuredLogger: azrealtime.NewLogger(azrealtime.LogLevelDebug),
	}
	fmt.Printf("Config with structured logger: %T\n", cfg2.StructuredLogger)

	// Option 3: Environment-based structured logger
	cfg3 := azrealtime.Config{
		ResourceEndpoint: "https://example.openai.azure.com",
		Deployment:       "gpt-4o-realtime",
		APIVersion:       "2025-04-01-preview",
		Credential:       azrealtime.APIKey("demo-key"),
		StructuredLogger: azrealtime.NewLoggerFromEnv(),
	}
	fmt.Printf("Config with env-based logger: %T\n", cfg3.StructuredLogger)

	// Note: These configs would be used with azrealtime.Dial(ctx, cfg)
	fmt.Println("Note: In real usage, these configs would be passed to azrealtime.Dial()")
}