package session

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/svdx9/talki/backend-go/internal/config"
	"github.com/svdx9/talki/backend-go/internal/skills"
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
	}
}

// Run starts the reader and writer goroutines and waits for both to complete.
// Returns the first non-nil error encountered, or nil if both exit cleanly.
// When ctx cancels, both goroutines unwind and the session ends.
func (s *Session) Run(ctx context.Context) error {
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
		switch typ {
		case websocket.MessageText:
			handleErr := s.handle(ctx, data)
			if handleErr != nil {
				s.log.Error("message handler error", "session", s.id, "error", handleErr)
			}
		case websocket.MessageBinary:
			if s.cfg.DebugWS {
				s.log.Debug("session binary frame", "session", s.id, "bytes", len(data))
			}
			if s.utteranceOpen {
				s.log.Debug("audio frame received", "session", s.id, "bytes", len(data))
			} else {
				s.log.Warn("received binary frame, but utterance is not open", "session", s.id)
			}
		}
	}
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

// ID returns the session ID.
func (s *Session) ID() string {
	return s.id
}
