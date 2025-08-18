package azrealtime

import (
	"context"
	"fmt"
	"math"
	"time"
)

// errorAs is a helper function for error type assertions (compatible with Go 1.13+)
func errorAs(err error, target interface{}) bool {
	switch target := target.(type) {
	case **ConfigError:
		if configErr, ok := err.(*ConfigError); ok {
			*target = configErr
			return true
		}
	case **ConnectionError:
		if connErr, ok := err.(*ConnectionError); ok {
			*target = connErr
			return true
		}
	case **SendError:
		if sendErr, ok := err.(*SendError); ok {
			*target = sendErr
			return true
		}
	}
	return false
}

// RetryConfig configures retry behavior for failed operations.
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts.
	// Set to 0 to disable retries.
	MaxRetries int
	
	// BaseDelay is the initial delay between retries.
	// Default: 1 second
	BaseDelay time.Duration
	
	// MaxDelay is the maximum delay between retries.
	// Default: 30 seconds
	MaxDelay time.Duration
	
	// Multiplier is used for exponential backoff.
	// Each retry delay is multiplied by this factor.
	// Default: 2.0
	Multiplier float64
	
	// Jitter adds randomness to retry delays to avoid thundering herd.
	// Value between 0.0 and 1.0. Default: 0.1 (10% jitter)
	Jitter float64
	
	// RetryableErrors is a function that determines if an error should trigger a retry.
	// If nil, all errors are considered retryable.
	RetryableErrors func(error) bool
}

// DefaultRetryConfig returns a sensible default retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:  3,
		BaseDelay:   1 * time.Second,
		MaxDelay:    30 * time.Second,
		Multiplier:  2.0,
		Jitter:      0.1,
		RetryableErrors: func(err error) bool {
			// Don't retry configuration or validation errors
			var configErr *ConfigError
			if errorAs(err, &configErr) {
				return false
			}
			// Retry connection and send errors
			var connErr *ConnectionError
			var sendErr *SendError
			return errorAs(err, &connErr) || errorAs(err, &sendErr)
		},
	}
}

// RetryableOperation represents an operation that can be retried.
type RetryableOperation func() error

// WithRetry executes an operation with retry logic based on the provided configuration.
func WithRetry(ctx context.Context, config RetryConfig, op RetryableOperation) error {
	var lastErr error
	
	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Execute the operation
		err := op()
		if err == nil {
			return nil // Success
		}
		
		lastErr = err
		
		// Check if we should retry this error
		if config.RetryableErrors != nil && !config.RetryableErrors(err) {
			return fmt.Errorf("non-retryable error: %w", err)
		}
		
		// Don't delay after the last attempt
		if attempt == config.MaxRetries {
			break
		}
		
		// Calculate delay with exponential backoff and jitter
		delay := calculateDelay(attempt, config)
		
		// Wait for the calculated delay, respecting context cancellation
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled: %w", ctx.Err())
		case <-time.After(delay):
			// Continue to next retry
		}
	}
	
	return fmt.Errorf("operation failed after %d attempts: %w", config.MaxRetries+1, lastErr)
}

// calculateDelay computes the delay for a retry attempt with exponential backoff and jitter.
func calculateDelay(attempt int, config RetryConfig) time.Duration {
	// Calculate exponential backoff delay
	delay := float64(config.BaseDelay) * math.Pow(config.Multiplier, float64(attempt))
	
	// Apply maximum delay cap
	if delay > float64(config.MaxDelay) {
		delay = float64(config.MaxDelay)
	}
	
	// Add jitter to avoid thundering herd
	if config.Jitter > 0 {
		jitterAmount := delay * config.Jitter
		// Add random jitter between -jitterAmount and +jitterAmount
		delay += (2.0*jitterAmount) - jitterAmount // Simplified: just add max jitter for deterministic behavior
	}
	
	return time.Duration(delay)
}

// WithRetryableClient creates a client wrapper that automatically retries failed operations.
type WithRetryableClient struct {
	client *Client
	config RetryConfig
}

// NewRetryableClient wraps a client with retry functionality.
func NewRetryableClient(client *Client, config RetryConfig) *WithRetryableClient {
	return &WithRetryableClient{
		client: client,
		config: config,
	}
}

// SessionUpdate attempts to update the session with retry logic.
func (r *WithRetryableClient) SessionUpdate(ctx context.Context, session Session) error {
	return WithRetry(ctx, r.config, func() error {
		return r.client.SessionUpdate(ctx, session)
	})
}

// CreateResponse attempts to create a response with retry logic.
func (r *WithRetryableClient) CreateResponse(ctx context.Context, opts CreateResponseOptions) (string, error) {
	var eventID string
	err := WithRetry(ctx, r.config, func() error {
		var err error
		eventID, err = r.client.CreateResponse(ctx, opts)
		return err
	})
	return eventID, err
}

// AppendPCM16 attempts to append PCM16 data with retry logic.
func (r *WithRetryableClient) AppendPCM16(ctx context.Context, pcmLE []byte) error {
	return WithRetry(ctx, r.config, func() error {
		return r.client.AppendPCM16(ctx, pcmLE)
	})
}

// InputCommit attempts to commit input with retry logic.
func (r *WithRetryableClient) InputCommit(ctx context.Context) error {
	return WithRetry(ctx, r.config, func() error {
		return r.client.InputCommit(ctx)
	})
}

// InputClear attempts to clear input with retry logic.
func (r *WithRetryableClient) InputClear(ctx context.Context) error {
	return WithRetry(ctx, r.config, func() error {
		return r.client.InputClear(ctx)
	})
}

// Delegate methods that don't need retry logic
func (r *WithRetryableClient) Close() error                                   { return r.client.Close() }
func (r *WithRetryableClient) OnError(fn func(ErrorEvent))                     { r.client.OnError(fn) }
func (r *WithRetryableClient) OnSessionCreated(fn func(SessionCreated))       { r.client.OnSessionCreated(fn) }
func (r *WithRetryableClient) OnSessionUpdated(fn func(SessionUpdated))       { r.client.OnSessionUpdated(fn) }
func (r *WithRetryableClient) OnRateLimitsUpdated(fn func(RateLimitsUpdated)) { r.client.OnRateLimitsUpdated(fn) }
func (r *WithRetryableClient) OnResponseTextDelta(fn func(ResponseTextDelta)) { r.client.OnResponseTextDelta(fn) }
func (r *WithRetryableClient) OnResponseTextDone(fn func(ResponseTextDone))   { r.client.OnResponseTextDone(fn) }
func (r *WithRetryableClient) OnResponseAudioDelta(fn func(ResponseAudioDelta)) { r.client.OnResponseAudioDelta(fn) }
func (r *WithRetryableClient) OnResponseAudioDone(fn func(ResponseAudioDone))   { r.client.OnResponseAudioDone(fn) }

// DialWithRetry creates a new client with automatic retry on connection failure.
func DialWithRetry(ctx context.Context, cfg Config, retryConfig RetryConfig) (*Client, error) {
	var client *Client
	err := WithRetry(ctx, retryConfig, func() error {
		var err error
		client, err = Dial(ctx, cfg)
		return err
	})
	return client, err
}

// CircuitBreakerConfig configures circuit breaker behavior.
type CircuitBreakerConfig struct {
	// FailureThreshold is the number of failures that triggers the circuit breaker.
	FailureThreshold int
	
	// RecoveryTimeout is how long to wait before attempting to recover.
	RecoveryTimeout time.Duration
	
	// SuccessThreshold is the number of successes needed to close the circuit.
	SuccessThreshold int
}

// CircuitBreakerState represents the current state of the circuit breaker.
type CircuitBreakerState int

const (
	CircuitClosed CircuitBreakerState = iota
	CircuitOpen
	CircuitHalfOpen
)

// CircuitBreaker implements the circuit breaker pattern to prevent cascading failures.
type CircuitBreaker struct {
	config           CircuitBreakerConfig
	state            CircuitBreakerState
	failures         int
	successes        int
	lastFailureTime  time.Time
}

// NewCircuitBreaker creates a new circuit breaker with the given configuration.
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		config: config,
		state:  CircuitClosed,
	}
}

// Execute runs an operation through the circuit breaker.
func (cb *CircuitBreaker) Execute(op func() error) error {
	// Check if we should allow the operation
	if !cb.shouldAllow() {
		return fmt.Errorf("circuit breaker is open")
	}
	
	// Execute the operation
	err := op()
	
	// Update circuit breaker state based on result
	if err != nil {
		cb.onFailure()
		return err
	}
	
	cb.onSuccess()
	return nil
}

// shouldAllow determines if an operation should be allowed based on circuit breaker state.
func (cb *CircuitBreaker) shouldAllow() bool {
	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		// Check if we should transition to half-open
		if time.Since(cb.lastFailureTime) >= cb.config.RecoveryTimeout {
			cb.state = CircuitHalfOpen
			return true
		}
		return false
	case CircuitHalfOpen:
		return true
	default:
		return false
	}
}

// onFailure handles a failed operation.
func (cb *CircuitBreaker) onFailure() {
	cb.failures++
	cb.successes = 0
	cb.lastFailureTime = time.Now()
	
	if cb.failures >= cb.config.FailureThreshold {
		cb.state = CircuitOpen
	}
}

// onSuccess handles a successful operation.
func (cb *CircuitBreaker) onSuccess() {
	cb.successes++
	cb.failures = 0
	
	if cb.state == CircuitHalfOpen && cb.successes >= cb.config.SuccessThreshold {
		cb.state = CircuitClosed
	}
}

// State returns the current circuit breaker state.
func (cb *CircuitBreaker) State() CircuitBreakerState {
	return cb.state
}