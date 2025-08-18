// Package main demonstrates the resilience features of azrealtime including
// retry logic, circuit breakers, and connection resilience.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/enesunal-m/azrealtime"
)

func main() {
	ctx := context.Background()

	// Configure client
	cfg := azrealtime.Config{
		ResourceEndpoint: getEnvOrDefault("AZURE_OPENAI_ENDPOINT", "https://example.openai.azure.com"),
		Deployment:       getEnvOrDefault("AZURE_OPENAI_REALTIME_DEPLOYMENT", "gpt-4o-realtime"),
		APIVersion:       "2025-04-01-preview",
		Credential:       azrealtime.APIKey(getEnvOrDefault("AZURE_OPENAI_API_KEY", "test-key")),
		DialTimeout:      30 * time.Second,
		Logger:           logger,
	}

	fmt.Println("=== Azure OpenAI Realtime Resilience Demo ===\n")

	// Example 1: Basic Retry Logic
	fmt.Println("1. Basic Retry Logic:")
	demonstrateBasicRetry(ctx, cfg)

	// Example 2: Custom Retry Configuration
	fmt.Println("\n2. Custom Retry Configuration:")
	demonstrateCustomRetry(ctx)

	// Example 3: Circuit Breaker Pattern
	fmt.Println("\n3. Circuit Breaker Pattern:")
	demonstrateCircuitBreaker()

	// Example 4: Resilient Client (combines retry + circuit breaking)
	fmt.Println("\n4. Resilient Client:")
	demonstrateResilientClient(ctx, cfg)

	fmt.Println("\n=== Demo Complete ===")
}

func demonstrateBasicRetry(ctx context.Context, cfg azrealtime.Config) {
	fmt.Println("  Creating client with automatic retry...")

	// Use default retry configuration
	client, err := azrealtime.DialWithRetry(ctx, cfg, azrealtime.DefaultRetryConfig())
	if err != nil {
		fmt.Printf("  ❌ Failed to create client (expected with demo config): %v\n", err)
		return
	}
	defer client.Close()

	fmt.Println("  ✅ Client created successfully with retry logic")
}

func demonstrateCustomRetry(ctx context.Context) {
	fmt.Println("  Setting up custom retry configuration...")

	// Custom retry config with more aggressive settings
	retryConfig := azrealtime.RetryConfig{
		MaxRetries: 5,                      // More attempts
		BaseDelay:  500 * time.Millisecond, // Longer base delay
		MaxDelay:   10 * time.Second,       // Higher max delay
		Multiplier: 1.5,                    // Gentler backoff
		Jitter:     0.2,                    // More jitter
		RetryableErrors: func(err error) bool {
			// Custom logic: retry everything except config errors
			var configErr *azrealtime.ConfigError
			return !errors.As(err, &configErr)
		},
	}

	fmt.Printf("  ✅ Custom retry config: %d retries, %v base delay, %v max delay\n",
		retryConfig.MaxRetries, retryConfig.BaseDelay, retryConfig.MaxDelay)

	// Demonstrate retry logic with a mock operation
	attemptCount := 0
	err := azrealtime.WithRetry(ctx, retryConfig, func() error {
		attemptCount++
		fmt.Printf("    Attempt %d...\n", attemptCount)

		if attemptCount < 3 {
			return fmt.Errorf("simulated failure %d", attemptCount)
		}
		return nil // Success on 3rd attempt
	})

	if err != nil {
		fmt.Printf("  ❌ Operation failed: %v\n", err)
	} else {
		fmt.Printf("  ✅ Operation succeeded after %d attempts\n", attemptCount)
	}
}

func demonstrateCircuitBreaker() {
	fmt.Println("  Setting up circuit breaker...")

	config := azrealtime.CircuitBreakerConfig{
		FailureThreshold: 3,
		RecoveryTimeout:  2 * time.Second,
		SuccessThreshold: 2,
	}

	cb := azrealtime.NewCircuitBreaker(config)
	fmt.Printf("  ✅ Circuit breaker created: %d failure threshold, %v recovery timeout\n",
		config.FailureThreshold, config.RecoveryTimeout)

	// Simulate failures to trigger circuit breaker
	fmt.Println("  Simulating failures to open circuit...")
	for i := 1; i <= 4; i++ {
		err := cb.Execute(func() error {
			return fmt.Errorf("simulated failure %d", i)
		})

		state := cb.State()
		var stateStr string
		switch state {
		case azrealtime.CircuitClosed:
			stateStr = "CLOSED"
		case azrealtime.CircuitOpen:
			stateStr = "OPEN"
		case azrealtime.CircuitHalfOpen:
			stateStr = "HALF-OPEN"
		}

		fmt.Printf("    Failure %d: Circuit is %s, Error: %v\n", i, stateStr, err)
	}

	// Try operation while circuit is open
	fmt.Println("  Trying operation while circuit is open...")
	err := cb.Execute(func() error {
		return nil // This won't be called
	})
	fmt.Printf("    ❌ Operation blocked: %v\n", err)

	// Wait for recovery and demonstrate half-open state
	fmt.Printf("  Waiting %v for recovery...\n", config.RecoveryTimeout)
	time.Sleep(config.RecoveryTimeout + 100*time.Millisecond)

	fmt.Println("  Attempting recovery...")
	for i := 1; i <= 2; i++ {
		err := cb.Execute(func() error {
			return nil // Success
		})

		state := cb.State()
		stateStr := map[azrealtime.CircuitBreakerState]string{
			azrealtime.CircuitClosed:   "CLOSED",
			azrealtime.CircuitOpen:     "OPEN",
			azrealtime.CircuitHalfOpen: "HALF-OPEN",
		}[state]

		if err == nil {
			fmt.Printf("    Success %d: Circuit is %s\n", i, stateStr)
		} else {
			fmt.Printf("    Failure %d: Circuit is %s, Error: %v\n", i, stateStr, err)
		}
	}
}

func demonstrateResilientClient(ctx context.Context, cfg azrealtime.Config) {
	fmt.Println("  Creating resilient client (combines retry + error handling)...")

	// Use the convenience function for maximum resilience
	client, err := azrealtime.DialResilient(ctx, cfg)
	if err != nil {
		fmt.Printf("  ❌ Failed to create resilient client (expected): %v\n", err)
		return
	}
	defer client.Close()

	fmt.Println("  ✅ Resilient client created successfully")

	// Demonstrate resilient operations
	fmt.Println("  Testing resilient session update...")

	session := azrealtime.Session{
		Voice:        azrealtime.Ptr("alloy"),
		Instructions: azrealtime.Ptr("You are a resilient assistant."),
	}

	err = client.SessionUpdate(ctx, session)
	if err != nil {
		fmt.Printf("  ❌ Session update failed: %v\n", err)
	} else {
		fmt.Println("  ✅ Session update succeeded")
	}

	// Test response creation with resilience
	fmt.Println("  Testing resilient response creation...")

	opts := azrealtime.CreateResponseOptions{
		Modalities: []string{"text"},
		Prompt:     "Test resilient response",
	}

	eventID, err := client.CreateResponse(ctx, opts)
	if err != nil {
		fmt.Printf("  ❌ Response creation failed: %v\n", err)
	} else {
		fmt.Printf("  ✅ Response creation succeeded: %s\n", eventID)
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func logger(event string, fields map[string]interface{}) {
	log.Printf("[%s] %+v", event, fields)
}
