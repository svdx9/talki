package protocol

import (
	"encoding/json"
	"errors"
	"testing"
)

// TestRoundTripStartSession verifies StartSession encodes/decodes correctly.
func TestRoundTripStartSession(t *testing.T) {
	t.Parallel()
	original := &StartSession{
		Type:       "start_session",
		ScenarioID: "scenario-123",
		SampleRate: 16000,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	msg, err := DecodeClient(data)
	if err != nil {
		t.Fatalf("DecodeClient failed: %v", err)
	}

	decoded, ok := msg.(*StartSession)
	if !ok {
		t.Fatalf("expected *StartSession, got %T", msg)
	}

	if decoded.Type != original.Type || decoded.ScenarioID != original.ScenarioID || decoded.SampleRate != original.SampleRate {
		t.Errorf("decoded mismatch: %+v != %+v", decoded, original)
	}
}

// TestRoundTripEndSession verifies EndSession encodes/decodes correctly.
func TestRoundTripEndSession(t *testing.T) {
	t.Parallel()
	original := &EndSession{Type: "end_session"}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	msg, err := DecodeClient(data)
	if err != nil {
		t.Fatalf("DecodeClient failed: %v", err)
	}

	_, ok := msg.(*EndSession)
	if !ok {
		t.Fatalf("expected *EndSession, got %T", msg)
	}
}

// TestRoundTripStartUtterance verifies StartUtterance encodes/decodes correctly.
func TestRoundTripStartUtterance(t *testing.T) {
	t.Parallel()
	original := &StartUtterance{Type: "start_utterance"}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	msg, err := DecodeClient(data)
	if err != nil {
		t.Fatalf("DecodeClient failed: %v", err)
	}

	_, ok := msg.(*StartUtterance)
	if !ok {
		t.Fatalf("expected *StartUtterance, got %T", msg)
	}
}

// TestRoundTripEndUtterance verifies EndUtterance encodes/decodes correctly.
func TestRoundTripEndUtterance(t *testing.T) {
	t.Parallel()
	original := &EndUtterance{Type: "end_utterance"}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	msg, err := DecodeClient(data)
	if err != nil {
		t.Fatalf("DecodeClient failed: %v", err)
	}

	_, ok := msg.(*EndUtterance)
	if !ok {
		t.Fatalf("expected *EndUtterance, got %T", msg)
	}
}

// TestRoundTripCancel verifies Cancel encodes/decodes correctly.
func TestRoundTripCancel(t *testing.T) {
	t.Parallel()
	original := &Cancel{Type: "cancel"}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	msg, err := DecodeClient(data)
	if err != nil {
		t.Fatalf("DecodeClient failed: %v", err)
	}

	_, ok := msg.(*Cancel)
	if !ok {
		t.Fatalf("expected *Cancel, got %T", msg)
	}
}

// TestRoundTripSessionReady verifies SessionReady with greeting encodes/decodes correctly.
func TestRoundTripSessionReady(t *testing.T) {
	t.Parallel()
	original := NewSessionReady("Welcome!")
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded SessionReady
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Type != "session_ready" || decoded.Greeting != "Welcome!" {
		t.Errorf("decoded mismatch: %+v", decoded)
	}
}

// TestRoundTripSessionReadyNoGreeting verifies SessionReady without greeting (omitempty) encodes/decodes correctly.
func TestRoundTripSessionReadyNoGreeting(t *testing.T) {
	t.Parallel()
	original := NewSessionReady("")
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Empty greeting should be omitted from JSON
	if string(data) != `{"type":"session_ready"}` {
		t.Errorf("expected greeting to be omitted, got: %s", string(data))
	}

	var decoded SessionReady
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Type != "session_ready" || decoded.Greeting != "" {
		t.Errorf("decoded mismatch: %+v", decoded)
	}
}

// TestRoundTripTranscript verifies Transcript encodes/decodes correctly.
func TestRoundTripTranscript(t *testing.T) {
	t.Parallel()
	original := NewTranscript("hello world", true)
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Transcript
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Type != "transcript" || decoded.Text != "hello world" || decoded.IsFinal != true {
		t.Errorf("decoded mismatch: %+v", decoded)
	}
}

// TestRoundTripAssistantTextDelta verifies AssistantTextDelta encodes/decodes correctly.
func TestRoundTripAssistantTextDelta(t *testing.T) {
	t.Parallel()
	original := NewAssistantTextDelta("response chunk")
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded AssistantTextDelta
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Type != "assistant_text_delta" || decoded.Text != "response chunk" {
		t.Errorf("decoded mismatch: %+v", decoded)
	}
}

// TestRoundTripAssistantDone verifies AssistantDone encodes/decodes correctly.
func TestRoundTripAssistantDone(t *testing.T) {
	t.Parallel()
	original := NewAssistantDone("full response")
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded AssistantDone
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Type != "assistant_done" || decoded.FullText != "full response" {
		t.Errorf("decoded mismatch: %+v", decoded)
	}
}

// TestRoundTripError verifies Error encodes/decodes correctly.
func TestRoundTripError(t *testing.T) {
	t.Parallel()
	original := NewError("something went wrong")
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Error
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Type != "error" || decoded.Message != "something went wrong" {
		t.Errorf("decoded mismatch: %+v", decoded)
	}
}

// TestUnknownMessageType verifies that unknown types produce ErrUnknownMessageType.
func TestUnknownMessageType(t *testing.T) {
	t.Parallel()
	payload := []byte(`{"type":"unknown_type"}`)

	_, err := DecodeClient(payload)
	if !errors.Is(err, ErrUnknownMessageType) {
		t.Errorf("expected ErrUnknownMessageType, got %v", err)
	}
}

// TestDecodeClientMissingType verifies that missing type field produces an error.
func TestDecodeClientMissingType(t *testing.T) {
	t.Parallel()
	payload := []byte(`{"scenarioId":"test","sampleRate":16000}`)

	_, err := DecodeClient(payload)
	if err == nil {
		t.Errorf("expected error for missing type field, got nil")
	}
}
