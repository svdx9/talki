package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/svdx9/talki/backend/internal/config"
	"github.com/svdx9/talki/backend/internal/llm"
	"github.com/svdx9/talki/backend/internal/protocol"
	"github.com/svdx9/talki/backend/internal/skills"
	"github.com/svdx9/talki/backend/internal/tts"
	"github.com/svdx9/talki/backend/internal/tts/voxtral"
	"golang.org/x/sync/errgroup"
)

// writeTimeout is the per-write deadline for WebSocket sends.
// A wedged peer (TCP black-holed) that doesn't read will cause writes to
// block forever without this.
const writeTimeout = 10 * time.Second

// outgoingBuffer is the capacity of the outgoing channel. A small buffer
// lets handlers enqueue without blocking on network I/O for a single send.
const outgoingBuffer = 10

// Message represents a message in the conversation history.
type Message struct {
	Role    string
	Content string
}

type outgoingMsg struct {
	typ  websocket.MessageType
	data []byte
}

// STTDialFunc is the signature for opening a realtime STT connection.
// The default dials Voxtral; tests may substitute a local fake via this field.
type STTDialFunc func(ctx context.Context, apiKey, model string, af tts.AudioFormat) (tts.TranscriptionClient, error)

// llmStream is the interface for a streaming LLM response.
type llmStream interface {
	Deltas() <-chan string
	Wait() error
}

// llmClientIface is the interface for an LLM client that streams responses.
type llmClientIface interface {
	Stream(ctx context.Context, req llm.Request) (llmStream, error)
}

// llmClientAdapter adapts *llm.Client to llmClientIface.
type llmClientAdapter struct {
	c *llm.Client
}

func (a *llmClientAdapter) Stream(ctx context.Context, req llm.Request) (llmStream, error) {
	return a.c.Stream(ctx, req)
}

// Session represents a single WebSocket session with a client.
type Session struct {
	id            string
	conn          *websocket.Conn
	cfg           config.Config
	allSkills     skills.Repository
	log           *slog.Logger
	outgoing      chan outgoingMsg
	scenario      *skills.Skill
	transcript    strings.Builder
	utteranceOpen bool
	convHistory   []Message

	llmClient llmClientIface
	mu        sync.Mutex // protects llmCancel
	llmCancel context.CancelFunc

	dialSTT        STTDialFunc
	ttsClient      tts.AudioClient
	sttClient      tts.TranscriptionClient
	sttWg          sync.WaitGroup
	sttBytesPushed int
	utterancePeak  int
}

// binaryWriter enqueues each Write call as a binary WebSocket frame.
// It copies p before enqueuing because io.Copy reuses its buffer.
type binaryWriter struct {
	ctx      context.Context
	outgoing chan<- outgoingMsg
}

func (w *binaryWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	chunk := make([]byte, len(p))
	copy(chunk, p)
	select {
	case w.outgoing <- outgoingMsg{typ: websocket.MessageBinary, data: chunk}:
		return len(p), nil
	case <-w.ctx.Done():
		return 0, w.ctx.Err()
	}
}

// New creates a new Session with all required fields.
// The session is not started until Run is called.
func New(id string, conn *websocket.Conn, cfg config.Config, sr skills.Repository, log *slog.Logger) *Session {
	//exhaustruct:ignore
	return &Session{
		id:        id,
		conn:      conn,
		cfg:       cfg,
		outgoing:  make(chan outgoingMsg, outgoingBuffer),
		allSkills: sr,
		log:       log,
		dialSTT: func(ctx context.Context, apiKey, model string, af tts.AudioFormat) (tts.TranscriptionClient, error) {
			//exhaustruct:ignore
			return voxtral.Dial(ctx, apiKey, model, af, &voxtral.TranscriptionOptions{Logger: log})
		},
		ttsClient: voxtral.NewAudioClient(cfg.MistralAPIKey, nil),
		llmClient: &llmClientAdapter{c: llm.NewClient(cfg.LLMAPIKey, cfg.LLMBaseURL, nil)},
	}
}

// Run starts the reader and writer goroutines and waits for both to complete.
// Returns the first non-nil error encountered, or nil if both exit cleanly.
// When ctx cancels, both goroutines unwind and the session ends.
func (s *Session) Run(ctx context.Context) error {
	defer s.closeSTT()
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return s.writer(ctx) })
	g.Go(func() error { return s.reader(ctx) })
	return g.Wait()
}

// writer drains the outgoing channel onto the WebSocket connection. It is the
// sole goroutine that calls conn.Write. Each write is bounded by writeTimeout
// so a wedged peer cannot block the session indefinitely.
func (s *Session) writer(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case msg := <-s.outgoing:
			writeCtx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := s.conn.Write(writeCtx, msg.typ, msg.data)
			cancel()
			if err != nil {
				return fmt.Errorf("websocket write: %w", err)
			}
		}
	}
}

// reader reads frames from the WebSocket connection and dispatches them.
// Recoverable handler errors (decode failure, unknown message type) are logged
// and the loop continues; only transport-level failures end the session.
func (s *Session) reader(ctx context.Context) error {
	for {
		typ, data, err := s.conn.Read(ctx)
		if err != nil {
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
				return nil
			}
			if ctx.Err() != nil {
				return nil
			}
			s.log.Error("unexpected_close", "session", s.id, "error", err)
			return err
		}
		if s.cfg.DebugWS {
			s.log.Debug("session frame", "session", s.id, "type", typ, "bytes", len(data))
		}
		switch typ {
		case websocket.MessageText:
			handleErr := s.handle(ctx, data)
			if handleErr != nil {
				s.log.Error("message handler error", "session", s.id, "error", handleErr)
			}
		case websocket.MessageBinary:
			s.pushAudio(ctx, data)
		}
	}
}

// pushAudio tracks peak amplitude, forwards PCM to the STT client, and
// accumulates byte count for the per-utterance diagnostic log.
func (s *Session) pushAudio(ctx context.Context, pcm []byte) {
	// Scan 16-bit little-endian PCM samples for peak amplitude.
	for i := 0; i+1 < len(pcm); i += 2 {
		sample := int(int16(uint16(pcm[i]) | uint16(pcm[i+1])<<8))
		if sample < 0 {
			sample = -sample
		}
		if sample > s.utterancePeak {
			s.utterancePeak = sample
		}
	}
	s.sttBytesPushed += len(pcm)

	if s.sttClient == nil {
		s.log.Warn("audio frame dropped, no STT client", "session", s.id, "bytes", len(pcm))
		return
	}
	if !s.utteranceOpen {
		s.log.Warn("audio frame dropped, utterance not open", "session", s.id, "bytes", len(pcm))
		return
	}
	err := s.sttClient.SendAudio(ctx, pcm)
	if err != nil {
		s.log.Error("STT send audio error", "session", s.id, "error", err)
	}
}

// readSTTEvents ranges over the STT client's event channel and forwards
// transcript events to the WebSocket client. Runs in its own goroutine.
func (s *Session) readSTTEvents(ctx context.Context, client tts.TranscriptionClient) {
	defer s.sttWg.Done()
	for {
		select {
		case ev, ok := <-client.Events():
			if !ok {
				return
			}
			s.log.Warn("STT event", "session", s.id, "type", ev.Type)
			switch ev.Type {
			case "transcription.text.delta":
				s.transcript.WriteString(ev.Text)
				s.log.Warn("transcript delta", "session", s.id, "text", ev.Text)
				s.SendText(ctx, protocol.NewTranscript(s.transcript.String(), false))
			case "transcription.segment.delta":
				if ev.Text != "" {
					s.transcript.WriteString(ev.Text)
					s.log.Warn("transcript delta", "session", s.id, "text", ev.Text)
					s.SendText(ctx, protocol.NewTranscript(s.transcript.String(), false))
				}
			case "transcription.done":
				final := s.transcript.String()
				s.log.Warn("transcript final", "session", s.id, "text", final)
				s.SendText(ctx, protocol.NewTranscript(final, true))
				if final != "" {
					go s.streamAssistant(ctx, final)
				}
			case "error":
				s.log.Error("STT error", "session", s.id, "raw", string(ev.Raw))
				s.SendText(ctx, protocol.NewError("STT error"))
			default:
				s.log.Info("STT unknown event", "session", s.id, "type", ev.Type, "raw", string(ev.Raw))
			}
		case <-ctx.Done():
			return
		}
	}
}

// closeSTT cancels the active STT client and waits for the event reader goroutine
// to exit before returning. Safe to call with no active STT client.
func (s *Session) closeSTT() {
	client := s.sttClient
	s.sttClient = nil
	s.sttBytesPushed = 0
	s.utterancePeak = 0
	if client == nil {
		return
	}
	_ = client.Close()
	s.sttWg.Wait()
}

// SendText JSON-encodes msg and enqueues it for the writer goroutine.
// Safe to call from any goroutine. If ctx cancels before enqueue, the
// frame is dropped.
func (s *Session) SendText(ctx context.Context, msg any) {
	data, err := json.Marshal(msg)
	if err != nil {
		s.log.Error("marshal error", "session", s.id, "error", err)
		return
	}
	select {
	case s.outgoing <- outgoingMsg{data: data, typ: websocket.MessageText}:
	case <-ctx.Done():
	}
}

// streamAssistant sends the user turn to Anthropic, streams text deltas to the
// client, then appends the full exchange to conversation history and sends to TTS.
// Intended to run in its own goroutine.
func (s *Session) streamAssistant(ctx context.Context, userText string) {
	llmCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.llmCancel = cancel
	s.mu.Unlock()
	defer func() {
		cancel()
		s.mu.Lock()
		s.llmCancel = nil
		s.mu.Unlock()
	}()

	s.convHistory = append(s.convHistory, Message{Role: "user", Content: userText})

	msgs := make([]llm.Message, len(s.convHistory))
	for i, m := range s.convHistory {
		msgs[i] = llm.Message{Role: m.Role, Content: m.Content}
	}
	req := llm.Request{
		Model:     s.cfg.LLMModel,
		MaxTokens: 1024,
		System:    s.scenario.SystemPrompt,
		Messages:  msgs,
	}

	stream, err := s.llmClient.Stream(llmCtx, req)
	if err != nil {
		s.log.Error("LLM stream error", "session", s.id, "error", err)
		s.SendText(ctx, protocol.NewError("LLM error: "+err.Error()))
		return
	}

	var fullText strings.Builder
	for delta := range stream.Deltas() {
		s.SendText(llmCtx, protocol.NewAssistantTextDelta(delta))
		fullText.WriteString(delta)
	}

	waitErr := stream.Wait()
	if errors.Is(waitErr, context.Canceled) {
		s.log.Warn("assistant stream cancelled", "session", s.id)
		return
	}
	if waitErr != nil {
		s.log.Error("LLM stream wait error", "session", s.id, "error", waitErr)
		s.SendText(ctx, protocol.NewError("LLM stream error"))
		return
	}

	assistantText := fullText.String()
	if assistantText == "" {
		return
	}

	s.convHistory = append(s.convHistory, Message{Role: "assistant", Content: assistantText})
	s.SendText(ctx, protocol.NewAssistantDone(assistantText))

	sink := &binaryWriter{ctx: ctx, outgoing: s.outgoing}
	//exhaustruct:ignore
	ttsErr := s.ttsClient.Speech(ctx, tts.SpeechVoice{VoiceID: s.scenario.Voice}, assistantText, sink)
	if ttsErr != nil && !errors.Is(ttsErr, context.Canceled) {
		s.log.Error("TTS error", "session", s.id, "error", ttsErr)
	}
}

// ID returns the session ID.
func (s *Session) ID() string {
	return s.id
}
