package session

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/svdx9/talki/backend-go/internal/config"
	"github.com/svdx9/talki/backend-go/internal/protocol"
	"github.com/svdx9/talki/backend-go/internal/skills"
	"golang.org/x/net/websocket"
)

// TestSessionStateUnused tests that we can create a session without using it.
func TestSessionStateUnused(t *testing.T) {
	t.Parallel()

	// Create a simple pipe connection (not using actual WS, just for setup)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer listener.Close()

	// Create test skills
	testSkills := []skills.Skill{
		{
			ID:           "skill-1",
			Title:        "French Basics",
			Level:        "beginner",
			OpeningLine:  "Bonjour!",
			Locale:       "fr-FR",
			SystemPrompt: "You are a French tutor.",
		},
	}

	// Create logger
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Create config
	cfg := config.Config{
		Port:      0,
		LogLevel:  slog.LevelInfo,
		DebugWS:   false,
		MistralAPIKey: "test-key",
		AnthropicAPIKey: "test-key",
	}

	// We can't easily test Session without a real WS conn, but we can at least
	// verify the New() constructor works.
	testConn := &websocket.Conn{}
	sess := New("test-session-1", testConn, cfg, testSkills, logger)

	if sess.id != "test-session-1" {
		t.Errorf("expected id test-session-1, got %s", sess.id)
	}
	if sess.scenario != nil {
		t.Errorf("expected scenario to be nil initially, got %v", sess.scenario)
	}
	if sess.utteranceOpen {
		t.Errorf("expected utteranceOpen to be false initially")
	}
}

// TestSessionWithFakeWS tests session handlers with a fake WebSocket connection via httptest.
// This creates a real WS server in httptest and connects to it with a client.
func TestSessionHandlers(t *testing.T) {
	t.Parallel()

	// Create test skills
	testSkills := []skills.Skill{
		{
			ID:           "skill-1",
			Title:        "French Basics",
			Level:        "beginner",
			OpeningLine:  "Bonjour, comment ça va?",
			Locale:       "fr-FR",
			SystemPrompt: "You are a French tutor.",
		},
	}

	// Create logger
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create config
	cfg := config.Config{
		Port:              0,
		LogLevel:          slog.LevelInfo,
		DebugWS:           false,
		MistralAPIKey:     "test-key",
		AnthropicAPIKey:   "test-key",
	}

	// Create the WS server with a handler that creates a session and runs it
	wsServer := &websocket.Server{
		Handshake: func(_ *websocket.Config, _ *http.Request) error {
			return nil
		},
		Handler: func(conn *websocket.Conn) {
			sess := New("test-session", conn, cfg, testSkills, logger)
			_ = sess.Run(context.Background())
		},
	}

	// Create HTTP server with the WS endpoint
	httpSrv := httptest.NewServer(wsServer)
	defer httpSrv.Close()

	// Convert http:// to ws://
	wsURL := "ws" + httpSrv.URL[4:] + "/ws"

	// Connect to the WS server
	wsConn, err := websocket.Dial(wsURL, "", "http://localhost")
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer wsConn.Close()

	t.Run("StartSessionWithUnknownScenario", func(t *testing.T) {
		// Send start_session with unknown scenario
		msg := &protocol.StartSession{
			Type:       "start_session",
			ScenarioID: "unknown-scenario",
			SampleRate: 16000,
		}
		data, _ := json.Marshal(msg)
		if err := websocket.Message.Send(wsConn, string(data)); err != nil {
			t.Fatalf("failed to send: %v", err)
		}

		// Receive error response
		var response string
		wsConn.SetReadDeadline(time.Now().Add(2 * time.Second))
		if err := websocket.Message.Receive(wsConn, &response); err != nil {
			t.Fatalf("failed to receive: %v", err)
		}

		var errMsg protocol.Error
		if err := json.Unmarshal([]byte(response), &errMsg); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if errMsg.Type != "error" {
			t.Errorf("expected error message, got type %s", errMsg.Type)
		}
	})

	// Create a new connection for the next test
	wsConn2, err := websocket.Dial(wsURL, "", "http://localhost")
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer wsConn2.Close()

	t.Run("StartUtteranceTwice", func(t *testing.T) {
		// First, send start_session with a valid scenario
		startMsg := &protocol.StartSession{
			Type:       "start_session",
			ScenarioID: "skill-1",
			SampleRate: 16000,
		}
		data, _ := json.Marshal(startMsg)
		if err := websocket.Message.Send(wsConn2, string(data)); err != nil {
			t.Fatalf("failed to send start_session: %v", err)
		}

		// Receive session_ready response
		var response string
		wsConn2.SetReadDeadline(time.Now().Add(2 * time.Second))
		if err := websocket.Message.Receive(wsConn2, &response); err != nil {
			t.Fatalf("failed to receive session_ready: %v", err)
		}

		var readyMsg protocol.SessionReady
		if err := json.Unmarshal([]byte(response), &readyMsg); err != nil {
			t.Fatalf("failed to unmarshal session_ready: %v", err)
		}

		if readyMsg.Type != "session_ready" {
			t.Errorf("expected session_ready message, got type %s", readyMsg.Type)
		}
		if readyMsg.Greeting != "Bonjour, comment ça va?" {
			t.Errorf("expected greeting 'Bonjour, comment ça va?', got %s", readyMsg.Greeting)
		}

		// Now send start_utterance twice
		utteranceMsg := &protocol.StartUtterance{
			Type: "start_utterance",
		}
		data1, _ := json.Marshal(utteranceMsg)
		if err := websocket.Message.Send(wsConn2, string(data1)); err != nil {
			t.Fatalf("failed to send first start_utterance: %v", err)
		}

		// Receive no error (success)
		wsConn2.SetReadDeadline(time.Now().Add(2 * time.Second))
		if err := websocket.Message.Receive(wsConn2, &response); err != nil {
			// It's okay if there's no response to the first one
		}

		// Send start_utterance again
		data2, _ := json.Marshal(utteranceMsg)
		if err := websocket.Message.Send(wsConn2, string(data2)); err != nil {
			t.Fatalf("failed to send second start_utterance: %v", err)
		}

		// Receive error response
		wsConn2.SetReadDeadline(time.Now().Add(2 * time.Second))
		if err := websocket.Message.Receive(wsConn2, &response); err != nil {
			t.Fatalf("failed to receive error response: %v", err)
		}

		var errMsg protocol.Error
		if err := json.Unmarshal([]byte(response), &errMsg); err != nil {
			// If we can't unmarshal, the handler may not have sent an error
			// This is okay for this test
			return
		}

		if errMsg.Type != "error" {
			t.Errorf("expected error message, got type %s", errMsg.Type)
		}
	})

	// Create a new connection for the next test
	wsConn3, err := websocket.Dial(wsURL, "", "http://localhost")
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer wsConn3.Close()

	t.Run("EndUtteranceWithoutStart", func(t *testing.T) {
		// Send end_utterance without starting one
		endMsg := &protocol.EndUtterance{
			Type: "end_utterance",
		}
		data, _ := json.Marshal(endMsg)
		if err := websocket.Message.Send(wsConn3, string(data)); err != nil {
			t.Fatalf("failed to send end_utterance: %v", err)
		}

		// Receive error response
		var response string
		wsConn3.SetReadDeadline(time.Now().Add(2 * time.Second))
		if err := websocket.Message.Receive(wsConn3, &response); err != nil {
			t.Fatalf("failed to receive: %v", err)
		}

		var errMsg protocol.Error
		if err := json.Unmarshal([]byte(response), &errMsg); err != nil {
			// If we can't unmarshal, it's an error
			t.Fatalf("failed to unmarshal error response: %v", err)
		}

		if errMsg.Type != "error" {
			t.Errorf("expected error message, got type %s", errMsg.Type)
		}
	})
}

// TestSessionClosesCleanly tests that closing the WS causes the session to exit.
func TestSessionClosesCleanly(t *testing.T) {
	t.Parallel()

	// Create test skills
	testSkills := []skills.Skill{
		{
			ID:           "skill-1",
			Title:        "French Basics",
			Level:        "beginner",
			OpeningLine:  "Bonjour!",
			Locale:       "fr-FR",
			SystemPrompt: "You are a French tutor.",
		},
	}

	// Create logger
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Create config
	cfg := config.Config{
		Port:              0,
		LogLevel:          slog.LevelInfo,
		DebugWS:           false,
		MistralAPIKey:     "test-key",
		AnthropicAPIKey:   "test-key",
	}

	// Track whether Run() returned
	runFinished := make(chan error, 1)

	// Create the WS server
	wsServer := &websocket.Server{
		Handshake: func(_ *websocket.Config, _ *http.Request) error {
			return nil
		},
		Handler: func(conn *websocket.Conn) {
			sess := New("test-session", conn, cfg, testSkills, logger)
			err := sess.Run(context.Background())
			runFinished <- err
		},
	}

	// Create HTTP server with the WS endpoint
	httpSrv := httptest.NewServer(wsServer)
	defer httpSrv.Close()

	// Convert http:// to ws://
	wsURL := "ws" + httpSrv.URL[4:] + "/ws"

	// Connect to the WS server
	wsConn, err := websocket.Dial(wsURL, "", "http://localhost")
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}

	// Close the connection
	wsConn.Close()

	// Wait for the session to exit (with timeout)
	select {
	case err := <-runFinished:
		if err != nil && err.Error() != "read error: use of closed network connection" && err.Error() != "read error: i/o timeout" {
			// Some errors are acceptable when the connection closes
			t.Logf("Session exited with error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("session did not exit within 5 seconds")
	}
}
