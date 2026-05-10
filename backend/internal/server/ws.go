package server

import (
	"crypto/rand"
	"encoding/base32"
	"log/slog"
	"net/http"
	"strings"

	"github.com/coder/websocket"
	"github.com/svdx9/talki/backend/internal/config"
	"github.com/svdx9/talki/backend/internal/session"
	"github.com/svdx9/talki/backend/internal/skills"
)

// newSessionID generates a random 6-character lowercase session ID using base32 encoding.
// Panics if crypto/rand is unavailable (unrecoverable, matches stdlib convention).
func newSessionID() string {
	b := make([]byte, 4)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	// base32 with NoPadding encodes 4 bytes to 6 characters, all lowercase
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)
	return strings.ToLower(encoded)
}

type chatServer struct {
	l      *slog.Logger
	cfg    config.Config
	skills skills.Repository
}

// maxBinaryFrameBytes caps inbound WS messages. Sized for typical
// 16 kHz PCM audio chunks (~20 KiB per 200 ms) with headroom.
const maxBinaryFrameBytes = 64 * 1024

func (c *chatServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// TODO: implement origin validation once frontend is served from a different host.
	//exhaustruct:ignore
	wc, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		c.l.Error("websocket_init", "error", err)
		return
	}
	defer func() { _ = wc.CloseNow() }()
	wc.SetReadLimit(maxBinaryFrameBytes)

	sessionID := newSessionID()
	c.l.Info("session connected", "sessionID", sessionID)

	sess := session.New(sessionID, wc, c.cfg, c.skills, c.l)
	runErr := sess.Run(r.Context())
	if runErr != nil {
		c.l.Error("session ended with error", "sessionID", sessionID, "error", runErr)
	}
	c.l.Info("session disconnected", "sessionID", sessionID)

	_ = wc.Close(websocket.StatusNormalClosure, "")
}
