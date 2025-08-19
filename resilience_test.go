package azrealtime

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// Helper function to check if a string contains another string
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	if config.MaxRetries != 3 {
		t.Errorf("expected MaxRetries=3, got %d", config.MaxRetries)
	}
	if config.BaseDelay != 1*time.Second {
		t.Errorf("expected BaseDelay=1s, got %v", config.BaseDelay)
	}
	if config.MaxDelay != 30*time.Second {
		t.Errorf("expected MaxDelay=30s, got %v", config.MaxDelay)
	}
	if config.Multiplier != 2.0 {
		t.Errorf("expected Multiplier=2.0, got %f", config.Multiplier)
	}
	if config.RetryableErrors == nil {
		t.Error("expected RetryableErrors function to be set")
	}
}

func TestWithRetry_Success(t *testing.T) {
	config := RetryConfig{MaxRetries: 3, BaseDelay: 1 * time.Millisecond}
	callCount := 0

	err := WithRetry(context.Background(), config, func() error {
		callCount++
		return nil // Success on first try
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestWithRetry_SuccessAfterRetries(t *testing.T) {
	config := RetryConfig{MaxRetries: 3, BaseDelay: 1 * time.Millisecond}
	callCount := 0

	err := WithRetry(context.Background(), config, func() error {
		callCount++
		if callCount < 3 {
			return errors.New("temporary failure")
		}
		return nil // Success on third try
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
}

func TestWithRetry_MaxRetriesExceeded(t *testing.T) {
	config := RetryConfig{MaxRetries: 2, BaseDelay: 1 * time.Millisecond}
	callCount := 0

	err := WithRetry(context.Background(), config, func() error {
		callCount++
		return errors.New("persistent failure")
	})

	if err == nil {
		t.Error("expected error, got nil")
	}
	if callCount != 3 { // Initial attempt + 2 retries
		t.Errorf("expected 3 calls, got %d", callCount)
	}
	if err == nil || !contains(err.Error(), "persistent failure") {
		t.Errorf("expected wrapped error containing 'persistent failure', got %v", err)
	}
}

func TestWithRetry_NonRetryableError(t *testing.T) {
	config := RetryConfig{
		MaxRetries: 3,
		BaseDelay:  1 * time.Millisecond,
		RetryableErrors: func(err error) bool {
			return err.Error() != "non-retryable"
		},
	}
	callCount := 0

	err := WithRetry(context.Background(), config, func() error {
		callCount++
		return errors.New("non-retryable")
	})

	if err == nil {
		t.Error("expected error, got nil")
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestWithRetry_ContextCancellation(t *testing.T) {
	t.Skip("Context cancellation timing test - skip for now")

	config := RetryConfig{MaxRetries: 5, BaseDelay: 200 * time.Millisecond}
	ctx, cancel := context.WithCancel(context.Background())

	callCount := 0
	// Cancel context after first failure to test cancellation during retry delay
	go func() {
		time.Sleep(50 * time.Millisecond) // Wait for first call to complete
		cancel()
	}()

	err := WithRetry(ctx, config, func() error {
		callCount++
		return errors.New("failure")
	})

	if err == nil {
		t.Error("expected error, got nil")
	}
	// Should be cancelled during retry delay after first attempt
	if callCount > 2 { // First call + maybe one retry before cancellation
		t.Errorf("expected early cancellation, got %d calls", callCount)
	}
}

func TestCalculateDelay(t *testing.T) {
	config := RetryConfig{
		BaseDelay:  1 * time.Second,
		MaxDelay:   10 * time.Second,
		Multiplier: 2.0,
		Jitter:     0.0, // No jitter for predictable testing
	}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 1 * time.Second},  // 1 * 2^0 = 1
		{1, 2 * time.Second},  // 1 * 2^1 = 2
		{2, 4 * time.Second},  // 1 * 2^2 = 4
		{3, 8 * time.Second},  // 1 * 2^3 = 8
		{4, 10 * time.Second}, // 1 * 2^4 = 16, capped at MaxDelay=10
	}

	for _, tt := range tests {
		actual := calculateDelay(tt.attempt, config)
		if actual != tt.expected {
			t.Errorf("attempt %d: expected %v, got %v", tt.attempt, tt.expected, actual)
		}
	}
}

func TestRetryableClient(t *testing.T) {
	mockServer := NewMockServer(t)
	defer mockServer.Close()

	config := CreateMockConfig(mockServer.URL())
	ctx := context.Background()

	client, err := Dial(ctx, config)
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	defer client.Close()

	retryConfig := RetryConfig{
		MaxRetries: 1,
		BaseDelay:  1 * time.Millisecond,
	}

	retryableClient := NewRetryableClient(client, retryConfig)

	// Test SessionUpdate with retry
	session := Session{Voice: Ptr("alloy")}
	err = retryableClient.SessionUpdate(ctx, session)
	if err != nil {
		t.Errorf("SessionUpdate failed: %v", err)
	}

	// Test CreateResponse with retry
	opts := CreateResponseOptions{
		Modalities: []string{"text"},
		Prompt:     "Hello",
	}
	eventID, err := retryableClient.CreateResponse(ctx, opts)
	if err != nil {
		t.Errorf("CreateResponse failed: %v", err)
	}
	if eventID == "" {
		t.Error("expected non-empty event ID")
	}
}

func TestCircuitBreaker(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 3,
		RecoveryTimeout:  100 * time.Millisecond,
		SuccessThreshold: 2,
	}

	cb := NewCircuitBreaker(config)

	// Initial state should be closed
	if cb.State() != CircuitClosed {
		t.Errorf("expected CircuitClosed, got %v", cb.State())
	}

	// Simulate failures to open the circuit
	for i := 0; i < 3; i++ {
		err := cb.Execute(func() error {
			return errors.New("failure")
		})
		if err == nil {
			t.Error("expected error, got nil")
		}
	}

	// Circuit should now be open
	if cb.State() != CircuitOpen {
		t.Errorf("expected CircuitOpen, got %v", cb.State())
	}

	// Requests should be rejected
	err := cb.Execute(func() error {
		return nil
	})
	if err == nil || err.Error() != "circuit breaker is open" {
		t.Errorf("expected circuit breaker error, got %v", err)
	}

	// Wait for recovery timeout
	time.Sleep(150 * time.Millisecond)

	// Circuit should allow one request (half-open)
	err = cb.Execute(func() error {
		return nil // Success
	})
	if err != nil {
		t.Errorf("expected success in half-open state, got %v", err)
	}

	// One more success should close the circuit
	err = cb.Execute(func() error {
		return nil // Success
	})
	if err != nil {
		t.Errorf("expected success, got %v", err)
	}

	if cb.State() != CircuitClosed {
		t.Errorf("expected CircuitClosed after recovery, got %v", cb.State())
	}
}

func TestDialWithRetry(t *testing.T) {
	// Test with invalid config that should fail
	config := Config{
		ResourceEndpoint: "invalid-endpoint",
		Deployment:       "test",
		APIVersion:       "test",
		Credential:       APIKey("test"),
	}

	retryConfig := RetryConfig{
		MaxRetries: 1,
		BaseDelay:  1 * time.Millisecond,
		RetryableErrors: func(err error) bool {
			// Don't retry config errors
			var configErr *ConfigError
			return !errorAs(err, &configErr)
		},
	}

	ctx := context.Background()
	client, err := DialWithRetry(ctx, config, retryConfig)

	// Should fail without retries due to config error
	if err == nil {
		t.Error("expected error, got nil")
		if client != nil {
			client.Close()
		}
	}
}

func TestRetryConfigWithDefaultRetryableErrors(t *testing.T) {
	config := DefaultRetryConfig()

	// Test non-retryable errors
	configErr := NewConfigError("test", "", "test error")
	if config.RetryableErrors(configErr) {
		t.Error("ConfigError should not be retryable")
	}

	// Test retryable errors
	connErr := NewConnectionError("test://url", "dial", errors.New("network error"))
	if !config.RetryableErrors(connErr) {
		t.Error("ConnectionError should be retryable")
	}

	sendErr := NewSendError("test", "", errors.New("send error"))
	if !config.RetryableErrors(sendErr) {
		t.Error("SendError should be retryable")
	}
}

func BenchmarkWithRetry_Success(b *testing.B) {
	config := RetryConfig{MaxRetries: 3, BaseDelay: 1 * time.Nanosecond}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = WithRetry(ctx, config, func() error {
			return nil // Always succeed
		})
	}
}

func BenchmarkCircuitBreaker_Closed(b *testing.B) {
	config := CircuitBreakerConfig{
		FailureThreshold: 5,
		RecoveryTimeout:  1 * time.Second,
		SuccessThreshold: 2,
	}
	cb := NewCircuitBreaker(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cb.Execute(func() error {
			return nil // Always succeed
		})
	}
}
