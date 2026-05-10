package tts

import (
	"context"
	"encoding/json"
	"io"
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

// TranscriptionClient is the interface for realtime speech-to-text streaming.
// Implementations handle provider-specific wire details; callers only see this.
type TranscriptionClient interface {
	// SendAudio sends raw PCM bytes to the STT service.
	SendAudio(ctx context.Context, pcm []byte) error
	// Flush signals the end of one utterance and triggers transcription.
	Flush(ctx context.Context) error
	// Events returns a channel of decoded STT events. Closed when the connection closes.
	Events() <-chan Event
	// Close cancels the stream and releases all resources.
	Close() error
}

// SpeechVoice specifies the voice for a TTS request.
// Set exactly one field: VoiceID for a Mistral preset/created voice,
// or RefAudio for zero-shot cloning from raw audio bytes.
//
//exhaustruct:ignore
type SpeechVoice struct {
	VoiceID  string
	RefAudio []byte
}

// AudioClient synthesises speech from text.
type AudioClient interface {
	Speech(ctx context.Context, voice SpeechVoice, text string, sink io.Writer) error
}
