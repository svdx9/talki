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

// payloadTypeBinary is the WebSocket payload type for binary frames.
// golang.org/x/net/websocket uses 2 for binary, 1 for text.
const payloadTypeBinary = 2

// writeTimeout is the per-write deadline for WebSocket sends.
// A wedged peer (TCP black-holed) that doesn't read will cause writes to
// block forever without this.
const writeTimeout = 10 * time.Second

// Message represents a message in the conversation history.
type Message struct {
	Role    string
	Content string
}

// Session represents a single WebSocket session with a client.
type Session struct {
	id            string
	conn          *websocket.Conn
	cfg           config.Config
	allSkills     []skills.Skill
	log           *slog.Logger
	outgoing      chan []byte
	ctx           context.Context
	cancel        context.CancelFunc
	scenario      *skills.Skill
	transcript    strings.Builder
	utteranceOpen bool
	convHistory   []Message
}

// New creates a new Session with all required fields.
// The session is not started until Run is called.
func New(id string, conn *websocket.Conn, cfg config.Config, allSkills []skills.Skill, log *slog.Logger) *Session {
	return &Session{
		id:        id,
		conn:      conn,
		cfg:       cfg,
		allSkills: allSkills,
		log:       log,
		outgoing:  make(chan []byte, 10),
	}
}

// Run starts the reader and writer goroutines and waits for both to complete.
// Returns the first non-nil error encountered, or nil if both exit cleanly.
// The parentCtx is used as the base context; when it cancels, the session ends.
func (s *Session) Run(parentCtx context.Context) error {
	// Create session context from parent, cancelling when parent does.
	// This replaces the watcher goroutine that was previously in New.
	s.ctx, s.cancel = context.WithCancel(parentCtx)

	var wg sync.WaitGroup
	var readerErr, writerErr error

	// Start the writer goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		writerErr = s.runWriter()
	}()

	// Start the reader goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		readerErr = s.runReader()
		// Signal cancellation when reader exits to unblock writer if needed
		s.cancel()
	}()

	// Wait for both goroutines to finish
	wg.Wait()

	// Only one goroutine writes each variable, no lock needed after wg.Wait()
	if readerErr != nil {
		return readerErr
	}
	return writerErr
}

// runWriter reads from the outgoing channel and writes to the WebSocket connection.
// It watches for context cancellation and sets a read deadline on the connection
// to interrupt the reader. The writer is the sole goroutine that sends on conn.
// When ctx is cancelled, it drains remaining messages before exiting
// to avoid losing frames that were already enqueued.
func (s *Session) runWriter() error {
	// Watch for context cancellation in a separate goroutine to interrupt reader
	go func() {
		<-s.ctx.Done()
		// Signal the read loop to interrupt by setting a read deadline in the past.
		// Using time.Unix(1, 0) (non-zero past) as the conventional idiom.
		_ = s.conn.SetReadDeadline(time.Unix(1, 0))
	}()

	for {
		select {
		case <-s.ctx.Done():
			// Context cancelled; drain any remaining queued messages before exiting.
			// This prevents frames that were already enqueued by handlers from being lost.
			for {
				select {
				case data := <-s.outgoing:
					_ = s.conn.SetWriteDeadline(time.Unix(1, 0)) // non-blocking send on close
					_ = websocket.Message.Send(s.conn, string(data))
				default:
					return nil
				}
			}
		case data, ok := <-s.outgoing:
			if !ok {
				return nil
			}
			// Only the writer goroutine touches conn for sending; no lock needed.
			_ = s.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := websocket.Message.Send(s.conn, string(data)); err != nil {
				return fmt.Errorf("write error: %w", err)
			}
		}
	}
}

// runReader reads from the WebSocket connection and dispatches messages.
// It handles both text and binary frames. This goroutine does NOT close outgoing;
// that is done by the writer when ctx is cancelled, preventing Send() panics.
func (s *Session) runReader() error {
	for {
		select {
		case <-s.ctx.Done():
			return nil
		default:
		}

		// Use websocket.Message.Receive to read both text and binary frames.
		// When reading with []byte, PayloadType indicates the frame type (1=text, 2=binary).
		var data []byte
		err := websocket.Message.Receive(s.conn, &data)

		if err != nil {
			// Connection closed or read error; exit cleanly.
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
		if s.conn.PayloadType == payloadTypeBinary {
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
// If ctx is cancelled, Send returns without blocking (frame may be dropped).
// The writer drains the queue on ctx cancel, so most frames sent before cancel
// are still delivered, but frames queued after cancel may be lost.
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
// Returns a pointer into the shared allSkills slice; safe because the slice
// is constructed once at server startup and never mutated.
func (s *Session) findSkillByID(id string) *skills.Skill {
	for i := range s.allSkills {
		if s.allSkills[i].ID == id {
			return &s.allSkills[i]
		}
	}
	return nil
}
