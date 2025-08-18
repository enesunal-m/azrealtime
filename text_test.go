package azrealtime

import (
	"testing"
)

func TestTextAssembler(t *testing.T) {
	assembler := NewTextAssembler()

	// Test adding delta events
	delta1 := ResponseTextDelta{
		ResponseID: "resp_123",
		Delta:      "Hello",
	}
	delta2 := ResponseTextDelta{
		ResponseID: "resp_123",
		Delta:      " World",
	}

	// Add deltas
	assembler.OnDelta(delta1)
	assembler.OnDelta(delta2)

	// Test OnDone with assembled text
	done := ResponseTextDone{
		ResponseID: "resp_123",
		Text:       "", // Empty text means use assembled deltas
	}

	result := assembler.OnDone(done)
	expected := "Hello World"
	
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}

	// Verify data is cleaned up
	done2 := ResponseTextDone{ResponseID: "resp_123", Text: ""}
	remaining := assembler.OnDone(done2)
	if remaining != "" {
		t.Errorf("expected empty string after cleanup, got %q", remaining)
	}
}

func TestTextAssembler_CompleteTextProvided(t *testing.T) {
	assembler := NewTextAssembler()

	// Add some deltas
	delta := ResponseTextDelta{
		ResponseID: "resp_123",
		Delta:      "This should be ignored",
	}
	assembler.OnDelta(delta)

	// Test OnDone with complete text provided
	done := ResponseTextDone{
		ResponseID: "resp_123",
		Text:       "Complete response text",
	}

	result := assembler.OnDone(done)
	expected := "Complete response text"
	
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestTextAssembler_MultipleResponses(t *testing.T) {
	assembler := NewTextAssembler()

	// Add deltas for multiple responses
	assembler.OnDelta(ResponseTextDelta{ResponseID: "resp_1", Delta: "First"})
	assembler.OnDelta(ResponseTextDelta{ResponseID: "resp_2", Delta: "Second"})
	assembler.OnDelta(ResponseTextDelta{ResponseID: "resp_1", Delta: " response"})
	assembler.OnDelta(ResponseTextDelta{ResponseID: "resp_2", Delta: " response"})

	// Complete first response
	result1 := assembler.OnDone(ResponseTextDone{ResponseID: "resp_1", Text: ""})
	if result1 != "First response" {
		t.Errorf("expected 'First response', got %q", result1)
	}

	// Complete second response
	result2 := assembler.OnDone(ResponseTextDone{ResponseID: "resp_2", Text: ""})
	if result2 != "Second response" {
		t.Errorf("expected 'Second response', got %q", result2)
	}
}

func TestTextAssembler_EmptyDelta(t *testing.T) {
	assembler := NewTextAssembler()

	// Add empty delta
	delta := ResponseTextDelta{
		ResponseID: "resp_123",
		Delta:      "",
	}
	assembler.OnDelta(delta)

	// Complete response
	result := assembler.OnDone(ResponseTextDone{ResponseID: "resp_123", Text: ""})
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestTextAssembler_UnicodeHandling(t *testing.T) {
	assembler := NewTextAssembler()

	// Test Unicode characters
	deltas := []string{"Hello ", "🌍", " 世界", "!"}
	for _, delta := range deltas {
		assembler.OnDelta(ResponseTextDelta{
			ResponseID: "resp_unicode",
			Delta:      delta,
		})
	}

	result := assembler.OnDone(ResponseTextDone{ResponseID: "resp_unicode", Text: ""})
	expected := "Hello 🌍 世界!"
	
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func BenchmarkTextAssembler(b *testing.B) {
	assembler := NewTextAssembler()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		responseID := "resp_" + string(rune(i))
		
		// Add multiple deltas
		for j := 0; j < 10; j++ {
			assembler.OnDelta(ResponseTextDelta{
				ResponseID: responseID,
				Delta:      "test delta content ",
			})
		}
		
		// Complete response
		assembler.OnDone(ResponseTextDone{ResponseID: responseID, Text: ""})
	}
}