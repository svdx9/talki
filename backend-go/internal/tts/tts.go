package tts

import (
	"context"
	"encoding/json"
)

// AudioFormat specifies the PCM encoding and sample rate for an STT stream.
type AudioFormat struct {
	Encoding   string
	SampleRate int
}

// Event is a decoded event from a realtime STT stream.
// Type and Text hold the common fields; Raw is the full JSON payload so callers
// can log unknown event types without losing data.
type Event struct {
	Type string
	Text string
	Raw  json.RawMessage
}

// STTClient is the interface for realtime speech-to-text streaming.
// Implementations handle provider-specific wire details; callers only see this.
type STTClient interface {
	// SendAudio sends raw PCM bytes to the STT service.
	SendAudio(ctx context.Context, pcm []byte) error
	// Flush signals the end of one utterance and triggers transcription.
	Flush(ctx context.Context) error
	// Events returns a channel of decoded STT events. Closed when the connection closes.
	Events() <-chan Event
	// Close cancels the stream and releases all resources.
	Close() error
}
