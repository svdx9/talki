package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/svdx9/talki/backend-go/internal/config"
	"github.com/svdx9/talki/backend-go/internal/protocol"
	"github.com/svdx9/talki/backend-go/internal/skills"
	"golang.org/x/net/websocket"
)

// Message represents a message in the conversation history.
type Message struct {
	Role    string
	Content string
}

// Session represents a single WebSocket session with a client.
type Session struct {
	id             string
	conn           *websocket.Conn
	cfg            config.Config
	allSkills      []skills.Skill
	log            *slog.Logger
	outgoing       chan []byte
	ctx            context.Context
	cancel         context.CancelFunc
	scenario      *skills.Skill
	transcript    strings.Builder
	utteranceOpen bool
	convHistory   []Message
}

// New creates a new Session with all required fields.
// The parent context is used to derive the session context.
func New(id string, conn *websocket.Conn, cfg config.Config, allSkills []skills.Skill, log *slog.Logger) *Session {
	ctx, cancel := context.WithCancel(context.Background())
	return &Session{
		id:        id,
		conn:      conn,
		cfg:       cfg,
		allSkills: allSkills,
		log:       log,
		outgoing:  make(chan []byte, 10),
		ctx:       ctx,
		cancel:    cancel,
		transcript: strings.Builder{},
	}
}

// Run starts the reader and writer goroutines and waits for both to complete.
// Returns the first non-nil error encountered, or nil if both exit cleanly.
func (s *Session) Run(parentCtx context.Context) error {
	// Create a goroutine that watches the parent context and cancels our context if needed
	go func() {
		select {
		case <-parentCtx.Done():
			s.cancel()
		case <-s.ctx.Done():
		}
	}()

	var wg sync.WaitGroup
	var readerErr, writerErr error
	var errMu sync.Mutex

	// Start the writer goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := s.runWriter()
		errMu.Lock()
		if err != nil && writerErr == nil {
			writerErr = err
		}
		errMu.Unlock()
	}()

	// Start the reader goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := s.runReader()
		errMu.Lock()
		if err != nil && readerErr == nil {
			readerErr = err
		}
		errMu.Unlock()
		// Signal cancellation when reader exits to unblock writer if needed
		s.cancel()
	}()

	// Wait for both goroutines to finish
	wg.Wait()

	errMu.Lock()
	defer errMu.Unlock()
	if readerErr != nil {
		return readerErr
	}
	if writerErr != nil {
		return writerErr
	}
	return nil
}

// runWriter reads from the outgoing channel and writes to the WebSocket connection.
// It also watches for context cancellation and sets a read deadline on the connection
// to interrupt the reader. The writer is the sole goroutine that sends on conn.
// Note: outgoing frames may be lost if ctx is cancelled while frames are enqueued.
// This is intentional — on shutdown (client disconnect), we do not wait to flush.
func (s *Session) runWriter() error {
	// Watch for context cancellation in a separate goroutine
	go func() {
		<-s.ctx.Done()
		// Signal the read loop to interrupt by setting a read deadline in the past
		_ = s.conn.SetReadDeadline(time.Now())
	}()

	for {
		select {
		case <-s.ctx.Done():
			return nil
		case data, ok := <-s.outgoing:
			if !ok {
				return nil
			}
			// Only the writer goroutine touches conn for sending; no lock needed.
			err := websocket.Message.Send(s.conn, string(data))
			if err != nil {
				return fmt.Errorf("write error: %w", err)
			}
		}
	}
}

// runReader reads from the WebSocket connection and dispatches messages.
// It handles both text and binary frames.
func (s *Session) runReader() error {
	defer close(s.outgoing)

	for {
		select {
		case <-s.ctx.Done():
			return nil
		default:
		}

		// Use websocket.Message.Receive to read both text and binary frames
		// When reading with []byte, PayloadType indicates the frame type (1=text, 2=binary)
		var data []byte
		err := websocket.Message.Receive(s.conn, &data)

		if err != nil {
			// Connection closed or read error; exit cleanly
			// Check for common read-side errors without string matching.
			if errors.Is(err, io.EOF) {
				return nil
			}
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			// Also check for timeout using type assertion
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				return nil
			}
			return fmt.Errorf("read error: %w", err)
		}

		// Check the payload type
		// PayloadType: 1 = text, 2 = binary
		if s.conn.PayloadType == 2 { // Binary frame
			if s.cfg.DebugWS {
				s.log.Debug("session binary frame", "session", s.id, "bytes", len(data))
			}
			if s.utteranceOpen {
				// Log the audio frame length (no STT yet)
				s.log.Debug("audio frame received", "session", s.id, "bytes", len(data))
			} else {
				// Error: unexpected audio frame
				s.Send(protocol.NewError("Unexpected audio frame: send start_utterance first"))
			}
			continue
		}

		// Text frame
		if s.cfg.DebugWS {
			s.log.Debug("session text frame", "session", s.id, "bytes", len(data))
		}

		// Decode and handle the client message
		err = s.handle(s.ctx, data)
		if err != nil {
			s.log.Error("message handler error", "session", s.id, "error", err)
			s.Send(protocol.NewError(fmt.Sprintf("Handler error: %v", err)))
		}
	}
}

// Send JSON-encodes the message and pushes it onto the outgoing channel.
// Send is safe to call from any goroutine; it enqueues for the writer goroutine.
// If ctx is cancelled, Send returns without blocking (frame may be lost).
// Note: Future use from TTS/LLM streaming goroutines requires close(outgoing)
// to be gated to avoid panic; see comment in runReader.
func (s *Session) Send(msg any) {
	data, err := json.Marshal(msg)
	if err != nil {
		s.log.Error("marshal error", "session", s.id, "error", err)
		return
	}
	select {
	case s.outgoing <- data:
	case <-s.ctx.Done():
		// Context cancelled, don't block trying to send
	}
}

// Context returns the session's context.
func (s *Session) Context() context.Context {
	return s.ctx
}

// ID returns the session ID.
func (s *Session) ID() string {
	return s.id
}

// findSkillByID searches the skills list for a skill with the given ID.
func (s *Session) findSkillByID(id string) *skills.Skill {
	for i := range s.allSkills {
		if s.allSkills[i].ID == id {
			return &s.allSkills[i]
		}
	}
	return nil
}
