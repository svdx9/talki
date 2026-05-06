package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ErrUnknownMessageType indicates that a client message has an unrecognized type field.
var ErrUnknownMessageType = errors.New("unknown message type")

// ClientMsg variants

// RawClientMsg is a discriminator struct for first-pass decoding.
type RawClientMsg struct {
	Type string `json:"type"`
}

// StartSession is the client message to begin a session.
type StartSession struct {
	Type       string `json:"type"`
	ScenarioID string `json:"scenarioId"`
	SampleRate int    `json:"sampleRate"`
}

// EndSession is the client message to terminate the session.
type EndSession struct {
	Type string `json:"type"`
}

// StartUtterance is the client message to begin audio input.
type StartUtterance struct {
	Type string `json:"type"`
}

// EndUtterance is the client message to finish audio input.
type EndUtterance struct {
	Type string `json:"type"`
}

// Cancel is the client message to abort the current operation.
type Cancel struct {
	Type string `json:"type"`
}

// ServerMsg variants with constructors to prevent missing required fields.

// SessionReady is the server message indicating the session is initialized.
type SessionReady struct {
	Type     string `json:"type"`
	Greeting string `json:"greeting,omitempty"`
}

// NewSessionReady creates a SessionReady message.
func NewSessionReady(greeting string) *SessionReady {
	return &SessionReady{
		Type:     "session_ready",
		Greeting: greeting,
	}
}

// Transcript is the server message carrying recognized text.
type Transcript struct {
	Type    string `json:"type"`
	Text    string `json:"text"`
	IsFinal bool   `json:"isFinal"`
}

// NewTranscript creates a Transcript message.
func NewTranscript(text string, isFinal bool) *Transcript {
	return &Transcript{
		Type:    "transcript",
		Text:    text,
		IsFinal: isFinal,
	}
}

// AssistantTextDelta is the server message carrying incremental assistant text.
type AssistantTextDelta struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// NewAssistantTextDelta creates an AssistantTextDelta message.
func NewAssistantTextDelta(text string) *AssistantTextDelta {
	return &AssistantTextDelta{
		Type: "assistant_text_delta",
		Text: text,
	}
}

// AssistantDone is the server message indicating assistant response completion.
type AssistantDone struct {
	Type     string `json:"type"`
	FullText string `json:"fullText"`
}

// NewAssistantDone creates an AssistantDone message.
func NewAssistantDone(fullText string) *AssistantDone {
	return &AssistantDone{
		Type:     "assistant_done",
		FullText: fullText,
	}
}

// Error is the server message carrying error information.
type Error struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// NewError creates an Error message.
func NewError(message string) *Error {
	return &Error{
		Type:    "error",
		Message: message,
	}
}

// DecodeClient decodes a client message from JSON bytes.
// It first decodes the RawClientMsg to determine the type,
// then dispatches to the appropriate concrete struct decoder.
// Returns ErrUnknownMessageType if the type field is unrecognized.
func DecodeClient(payload []byte) (any, error) {
	var raw RawClientMsg
	err := json.Unmarshal(payload, &raw)
	if err != nil {
		return nil, fmt.Errorf("failed to decode raw message: %w", err)
	}

	switch raw.Type {
	case "start_session":
		var msg StartSession
		err = json.Unmarshal(payload, &msg)
		if err != nil {
			return nil, fmt.Errorf("failed to decode start_session: %w", err)
		}
		return &msg, nil

	case "end_session":
		var msg EndSession
		err = json.Unmarshal(payload, &msg)
		if err != nil {
			return nil, fmt.Errorf("failed to decode end_session: %w", err)
		}
		return &msg, nil

	case "start_utterance":
		var msg StartUtterance
		err = json.Unmarshal(payload, &msg)
		if err != nil {
			return nil, fmt.Errorf("failed to decode start_utterance: %w", err)
		}
		return &msg, nil

	case "end_utterance":
		var msg EndUtterance
		err = json.Unmarshal(payload, &msg)
		if err != nil {
			return nil, fmt.Errorf("failed to decode end_utterance: %w", err)
		}
		return &msg, nil

	case "cancel":
		var msg Cancel
		err = json.Unmarshal(payload, &msg)
		if err != nil {
			return nil, fmt.Errorf("failed to decode cancel: %w", err)
		}
		return &msg, nil

	default:
		return nil, fmt.Errorf("type %q: %w", raw.Type, ErrUnknownMessageType)
	}
}
