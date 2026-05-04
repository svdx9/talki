package server

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"net/http"
	"strings"

	"github.com/svdx9/talki/backend-go/internal/session"
	"golang.org/x/net/websocket"
)

// newSessionID generates a random 6-character lowercase session ID using base32 encoding.
// Panics if crypto/rand is unavailable (unrecoverable, matches stdlib convention).
func newSessionID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	// base32 with NoPadding encodes 4 bytes to 6 characters, all lowercase
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)
	return strings.ToLower(encoded)
}

// wsHandler is the WebSocket connection handler.
// Creates a Session and runs its state machine.
//
// Note: x/net/websocket.Handler does not provide request context,
// so we use context.Background() as the parent. This means server
// graceful shutdown (where the parent context is cancelled) will NOT
// propagate to in-flight sessions. A sessions registry is needed
// to coordinate shutdown (future work).
func (s *Server) wsHandler(conn *websocket.Conn) {
	sessionID := newSessionID()
	s.log.Info("session connected", "sessionID", sessionID)

	// Create a new session
	sess := session.New(sessionID, conn, s.cfg, s.sk, s.log)

	// Run the session's state machine (blocks until the WS closes).
	// Parent context is Background(); session will exit cleanly when conn closes.
	err := sess.Run(context.Background())
	if err != nil {
		s.log.Error("session error", "sessionID", sessionID, "error", err)
	}

	s.log.Info("session disconnected", "sessionID", sessionID)
}

// newWSServer returns a configured websocket.Server that accepts connections from any origin.
// Note: golang.org/x/net/websocket distinguishes binary vs text traffic via the type passed to
// Receive/Send ([]byte for binary, string for text). For mixed traffic, use websocket.Message.Receive
// with []byte and inspect conn.PayloadType (1 = text, 2 = binary) per frame.
func (s *Server) newWSServer() *websocket.Server {
	return &websocket.Server{
		Handshake: func(_ *websocket.Config, _ *http.Request) error {
			// Accept any origin (frontend is served from the same host today).
			// TODO: implement origin validation once frontend is served from a different host.
			return nil
		},
		Handler: s.wsHandler,
	}
}
