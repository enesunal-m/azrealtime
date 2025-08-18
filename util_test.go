package azrealtime

import (
	"testing"
)

func TestPtr(t *testing.T) {
	// Test with string
	str := "test string"
	strPtr := Ptr(str)
	if strPtr == nil {
		t.Error("expected non-nil pointer for string")
	}
	if *strPtr != str {
		t.Errorf("expected %q, got %q", str, *strPtr)
	}

	// Test with int
	num := 42
	numPtr := Ptr(num)
	if numPtr == nil {
		t.Error("expected non-nil pointer for int")
	}
	if *numPtr != num {
		t.Errorf("expected %d, got %d", num, *numPtr)
	}

	// Test with bool
	b := true
	bPtr := Ptr(b)
	if bPtr == nil {
		t.Error("expected non-nil pointer for bool")
	}
	if *bPtr != b {
		t.Errorf("expected %v, got %v", b, *bPtr)
	}

	// Test with struct
	type testStruct struct {
		Field string
	}
	s := testStruct{Field: "value"}
	sPtr := Ptr(s)
	if sPtr == nil {
		t.Error("expected non-nil pointer for struct")
	}
	if sPtr.Field != s.Field {
		t.Errorf("expected %q, got %q", s.Field, sPtr.Field)
	}
}

func TestPtr_ZeroValues(t *testing.T) {
	// Test with zero values
	strPtr := Ptr("")
	if *strPtr != "" {
		t.Errorf("expected empty string, got %q", *strPtr)
	}

	intPtr := Ptr(0)
	if *intPtr != 0 {
		t.Errorf("expected 0, got %d", *intPtr)
	}

	boolPtr := Ptr(false)
	if *boolPtr != false {
		t.Errorf("expected false, got %v", *boolPtr)
	}
}

func TestPtr_SessionUsage(t *testing.T) {
	// Test real-world usage with Session struct
	session := Session{
		Voice:             Ptr("alloy"),
		Instructions:      Ptr("You are a helpful assistant."),
		InputAudioFormat:  Ptr("pcm16"),
		OutputAudioFormat: Ptr("pcm16"),
	}

	if session.Voice == nil || *session.Voice != "alloy" {
		t.Error("Voice pointer not set correctly")
	}
	if session.Instructions == nil || *session.Instructions != "You are a helpful assistant." {
		t.Error("Instructions pointer not set correctly")
	}
	if session.InputAudioFormat == nil || *session.InputAudioFormat != "pcm16" {
		t.Error("InputAudioFormat pointer not set correctly")
	}
	if session.OutputAudioFormat == nil || *session.OutputAudioFormat != "pcm16" {
		t.Error("OutputAudioFormat pointer not set correctly")
	}
}

func BenchmarkPtr(b *testing.B) {
	testString := "benchmark test string"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Ptr(testString)
	}
}