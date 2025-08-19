package azrealtime

import (
	"context"
	"errors"
	"fmt"
)

// CreateResponseOptions configures how the assistant should generate a response.
// This provides fine-grained control over the response generation process.
type CreateResponseOptions struct {
	// Modalities specifies which output types to generate.
	// Supported values: ["text", "audio"]
	Modalities []string `json:"modalities,omitempty"`

	// Prompt provides a direct instruction for this specific response.
	// This is added to the conversation context temporarily.
	Prompt string `json:"prompt,omitempty"`

	// Conversation specifies a conversation ID if managing multiple conversations.
	Conversation string `json:"conversation,omitempty"`

	// Metadata allows attaching custom data to the response for tracking purposes.
	Metadata map[string]any `json:"metadata,omitempty"`

	// Instructions provide response-specific guidance, overriding session instructions.
	Instructions string `json:"instructions,omitempty"`

	// Temperature controls randomness in the response (0.0 = deterministic, 1.0 = very random).
	// Typical range: 0.0-1.0
	Temperature float64 `json:"temperature,omitempty"`

	// Input provides explicit input items for the response (advanced usage).
	Input []any `json:"input,omitempty"`
}

// CreateResponse requests the assistant to generate a response with the given options.
// Returns the event ID for tracking this response request.
// The actual response will be delivered through the registered event handlers.
func (c *Client) CreateResponse(ctx context.Context, opts CreateResponseOptions) (string, error) {
	if ctx == nil {
		return "", NewSendError("response.create", "", errors.New("context cannot be nil"))
	}

	// Validate response options
	if err := ValidateCreateResponseOptions(opts); err != nil {
		return "", NewSendError("response.create", "", err)
	}

	payload := map[string]any{"type": "response.create", "response": opts}
	return c.nextEventID(ctx, payload)
}

// ValidateCreateResponseOptions validates response creation options.
func ValidateCreateResponseOptions(opts CreateResponseOptions) error {
	// Validate modalities
	if len(opts.Modalities) > 0 {
		validModalities := map[string]bool{"text": true, "audio": true}
		for _, modality := range opts.Modalities {
			if !validModalities[modality] {
				return fmt.Errorf("invalid modality %q, must be 'text' or 'audio'", modality)
			}
		}
	}

	// Validate temperature
	if opts.Temperature < 0.0 || opts.Temperature > 2.0 {
		return fmt.Errorf("temperature must be between 0.0 and 2.0, got %f", opts.Temperature)
	}

	// Validate prompt length
	if len(opts.Prompt) > 10000 {
		return fmt.Errorf("prompt too long (%d characters), maximum is 10000", len(opts.Prompt))
	}

	// Validate instructions length
	if len(opts.Instructions) > 10000 {
		return fmt.Errorf("instructions too long (%d characters), maximum is 10000", len(opts.Instructions))
	}

	// Validate conversation ID format (if specified)
	if opts.Conversation != "" {
		if len(opts.Conversation) > 100 {
			return fmt.Errorf("conversation ID too long (%d characters), maximum is 100", len(opts.Conversation))
		}
		// Could add more specific format validation here
	}

	return nil
}

// CancelResponse cancels an in-progress response.
// This stops the assistant from continuing to generate the current response.
func (c *Client) CancelResponse(ctx context.Context) error {
	if ctx == nil {
		return NewSendError("response.cancel", "", errors.New("context cannot be nil"))
	}

	payload := map[string]any{"type": "response.cancel"}
	return c.send(ctx, payload)
}
