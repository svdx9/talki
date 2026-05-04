package session

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/svdx9/talki/backend-go/internal/config"
	"github.com/svdx9/talki/backend-go/internal/protocol"
	"github.com/svdx9/talki/backend-go/internal/skills"
	"golang.org/x/net/websocket"
)

// TestSessionHandlers tests session handlers with a WebSocket connection via httptest.
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

		// Receive response to first start_utterance (should be silent, no error)
		wsConn2.SetReadDeadline(time.Now().Add(1 * time.Second))
		firstResp := ""
		receiveErr := websocket.Message.Receive(wsConn2, &firstResp)
		if receiveErr == nil && firstResp != "" {
			// Unexpected message received; check it's not an error
			var unexpectedMsg protocol.Error
			if json.Unmarshal([]byte(firstResp), &unexpectedMsg) == nil && unexpectedMsg.Type == "error" {
				t.Fatalf("first start_utterance should not produce an error")
			}
		}

		// Send start_utterance again (should fail)
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
			t.Fatalf("failed to unmarshal error response: %v", err)
		}

		if errMsg.Type != "error" {
			t.Fatalf("expected error message, got type %s", errMsg.Type)
		}
		if !strings.Contains(errMsg.Message, "already open") {
			t.Errorf("expected error message about utterance being open, got: %s", errMsg.Message)
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
		// Acceptable errors when connection closes
		if err != nil {
			errStr := err.Error()
			if !strings.Contains(errStr, "closed") && !strings.Contains(errStr, "timeout") && !strings.Contains(errStr, "EOF") {
				t.Logf("Session exited with unexpected error: %v", err)
			}
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("session did not exit within 5 seconds")
	}
}
