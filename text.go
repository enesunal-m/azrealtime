package azrealtime

// TextAssembler collects streaming text chunks and reassembles them into complete text responses.
// Use this to handle ResponseTextDelta events and reconstruct the full text response.
type TextAssembler struct{ data map[string][]byte }

// NewTextAssembler creates a new TextAssembler instance.
func NewTextAssembler() *TextAssembler { return &TextAssembler{data: make(map[string][]byte)} }

// OnDelta processes a ResponseTextDelta event by appending the text delta.
// Call this from your ResponseTextDelta event handler.
func (t *TextAssembler) OnDelta(e ResponseTextDelta) {
	t.data[e.ResponseID] = append(t.data[e.ResponseID], []byte(e.Delta)...)
}

// OnDone retrieves and removes the complete text response for a given ResponseTextDone event.
// Returns the full text, preferring the complete text field if available, otherwise
// returning the assembled deltas. Call this when you receive a ResponseTextDone event.
func (t *TextAssembler) OnDone(e ResponseTextDone) string {
	if e.Text != "" {
		// Complete text provided, clean up and return
		delete(t.data, e.ResponseID)
		return e.Text
	}
	// Assemble from deltas
	buf := t.data[e.ResponseID]
	delete(t.data, e.ResponseID)
	return string(buf)
}
