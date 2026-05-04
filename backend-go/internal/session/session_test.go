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

func testServerAndURL(t *testing.T) (*httptest.Server, string) {
	testSkills := []skills.Skill{
		{
			ID:          "skill-1",
			Title:       "French Basics",
			Level:       "beginner",
			OpeningLine: "Bonjour, comment ça va?",
			Locale:      "fr-FR",
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.Config{
		Port:            0,
		LogLevel:        slog.LevelInfo,
		DebugWS:         false,
		MistralAPIKey:   "test-key",
		AnthropicAPIKey: "test-key",
	}
	wsServer := &websocket.Server{
		Handshake: func(_ *websocket.Config, _ *http.Request) error {
			return nil
		},
		Handler: func(conn *websocket.Conn) {
			sess := New("test-session", conn, cfg, testSkills, logger)
			_ = sess.Run(context.Background())
		},
	}
	httpSrv := httptest.NewServer(wsServer)
	return httpSrv, "ws" + httpSrv.URL[4:] + "/ws"
}

func TestStartSessionWithUnknownScenario(t *testing.T) {
	t.Parallel()
	httpSrv, wsURL := testServerAndURL(t)
	defer httpSrv.Close()

	conn, err := websocket.Dial(wsURL, "", "http://localhost")
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	defer conn.Close()

	msg := &protocol.StartSession{
		Type:       "start_session",
		ScenarioID: "unknown-scenario",
		SampleRate: 16000,
	}
	data, _ := json.Marshal(msg)
	if err := websocket.Message.Send(conn, string(data)); err != nil {
		t.Fatalf("failed to send: %v", err)
	}

	var response string
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err := websocket.Message.Receive(conn, &response); err != nil {
		t.Fatalf("failed to receive: %v", err)
	}

	var errMsg protocol.Error
	if err := json.Unmarshal([]byte(response), &errMsg); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if errMsg.Type != "error" {
		t.Errorf("expected error, got %s", errMsg.Type)
	}
}

func TestStartUtteranceTwice(t *testing.T) {
	t.Parallel()
	httpSrv, wsURL := testServerAndURL(t)
	defer httpSrv.Close()

	conn, err := websocket.Dial(wsURL, "", "http://localhost")
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	defer conn.Close()

	// Send start_session with a valid scenario
	startMsg := &protocol.StartSession{
		Type:       "start_session",
		ScenarioID: "skill-1",
		SampleRate: 16000,
	}
	data, _ := json.Marshal(startMsg)
	if err := websocket.Message.Send(conn, string(data)); err != nil {
		t.Fatalf("failed to send start_session: %v", err)
	}

	var response string
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err := websocket.Message.Receive(conn, &response); err != nil {
		t.Fatalf("failed to receive session_ready: %v", err)
	}

	var readyMsg protocol.SessionReady
	if err := json.Unmarshal([]byte(response), &readyMsg); err != nil {
		t.Fatalf("failed to unmarshal session_ready: %v", err)
	}
	if readyMsg.Type != "session_ready" {
		t.Errorf("expected session_ready, got %s", readyMsg.Type)
	}
	if readyMsg.Greeting != "Bonjour, comment ça va?" {
		t.Errorf("expected greeting, got %s", readyMsg.Greeting)
	}

	// Send start_utterance twice
	utteranceMsg := &protocol.StartUtterance{Type: "start_utterance"}
	data1, _ := json.Marshal(utteranceMsg)
	if err := websocket.Message.Send(conn, string(data1)); err != nil {
		t.Fatalf("failed to send first start_utterance: %v", err)
	}

	// First start_utterance should be silent (no response)
	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	if err := websocket.Message.Receive(conn, &response); err == nil {
		var unexpectedMsg protocol.Error
		if json.Unmarshal([]byte(response), &unexpectedMsg) == nil && unexpectedMsg.Type == "error" {
			t.Fatalf("first start_utterance should not produce an error")
		}
	}

	// Second start_utterance should fail
	data2, _ := json.Marshal(utteranceMsg)
	if err := websocket.Message.Send(conn, string(data2)); err != nil {
		t.Fatalf("failed to send second start_utterance: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err := websocket.Message.Receive(conn, &response); err != nil {
		t.Fatalf("failed to receive error response: %v", err)
	}

	var errMsg protocol.Error
	if err := json.Unmarshal([]byte(response), &errMsg); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}
	if errMsg.Type != "error" {
		t.Fatalf("expected error, got %s", errMsg.Type)
	}
	if !strings.Contains(errMsg.Message, "already open") {
		t.Errorf("expected 'already open' error, got: %s", errMsg.Message)
	}
}

func TestEndUtteranceWithoutStart(t *testing.T) {
	t.Parallel()
	httpSrv, wsURL := testServerAndURL(t)
	defer httpSrv.Close()

	conn, err := websocket.Dial(wsURL, "", "http://localhost")
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	defer conn.Close()

	endMsg := &protocol.EndUtterance{Type: "end_utterance"}
	data, _ := json.Marshal(endMsg)
	if err := websocket.Message.Send(conn, string(data)); err != nil {
		t.Fatalf("failed to send end_utterance: %v", err)
	}

	var response string
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err := websocket.Message.Receive(conn, &response); err != nil {
		t.Fatalf("failed to receive: %v", err)
	}

	var errMsg protocol.Error
	if err := json.Unmarshal([]byte(response), &errMsg); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}
	if errMsg.Type != "error" {
		t.Errorf("expected error, got %s", errMsg.Type)
	}
}

// TODO: Test binary frame before start_utterance.
// The x/net/websocket library's binary frame handling needs investigation;
// websocket.Message.Send with []byte may not send opcode 2 as expected.
// Re-enable once we verify binary frame transmission works correctly.
func TestBinaryFrameBeforeStartUtterance(t *testing.T) {
	t.Skip("binary frame sending via x/net/websocket needs verification")
}

func TestCancelHandler(t *testing.T) {
	t.Parallel()
	httpSrv, wsURL := testServerAndURL(t)
	defer httpSrv.Close()

	conn, err := websocket.Dial(wsURL, "", "http://localhost")
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	defer conn.Close()

	// Send start_session and start_utterance to open an utterance
	startMsg := &protocol.StartSession{
		Type:       "start_session",
		ScenarioID: "skill-1",
		SampleRate: 16000,
	}
	data, _ := json.Marshal(startMsg)
	if err := websocket.Message.Send(conn, string(data)); err != nil {
		t.Fatalf("failed to send start_session: %v", err)
	}

	// Receive session_ready
	var response string
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err := websocket.Message.Receive(conn, &response); err != nil {
		t.Fatalf("failed to receive session_ready: %v", err)
	}

	// Start utterance
	utteranceMsg := &protocol.StartUtterance{Type: "start_utterance"}
	data, _ = json.Marshal(utteranceMsg)
	if err := websocket.Message.Send(conn, string(data)); err != nil {
		t.Fatalf("failed to send start_utterance: %v", err)
	}

	// Send cancel - should clear the utterance without error
	cancelMsg := &protocol.Cancel{Type: "cancel"}
	data, _ = json.Marshal(cancelMsg)
	if err := websocket.Message.Send(conn, string(data)); err != nil {
		t.Fatalf("failed to send cancel: %v", err)
	}

	// Cancel should be silent (no response expected)
	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	if err := websocket.Message.Receive(conn, &response); err == nil {
		t.Fatalf("cancel should not produce a response, got: %s", response)
	}

	// Now start_utterance again - should work (utterance was cleared by cancel)
	data, _ = json.Marshal(utteranceMsg)
	if err := websocket.Message.Send(conn, string(data)); err != nil {
		t.Fatalf("failed to send start_utterance after cancel: %v", err)
	}

	// Second start_utterance should fail (double open)
	data2, _ := json.Marshal(utteranceMsg)
	if err := websocket.Message.Send(conn, string(data2)); err != nil {
		t.Fatalf("failed to send second start_utterance: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err := websocket.Message.Receive(conn, &response); err != nil {
		t.Fatalf("failed to receive error response: %v", err)
	}

	var errMsg protocol.Error
	if err := json.Unmarshal([]byte(response), &errMsg); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}
	if errMsg.Type != "error" || !strings.Contains(errMsg.Message, "already open") {
		t.Errorf("expected 'already open' error after cancel, got: %s", errMsg.Message)
	}
}

func TestSessionClosesCleanly(t *testing.T) {
	t.Parallel()
	testSkills := []skills.Skill{
		{
			ID:          "skill-1",
			Title:       "French Basics",
			Level:       "beginner",
			OpeningLine: "Bonjour!",
			Locale:      "fr-FR",
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.Config{
		Port:            0,
		LogLevel:        slog.LevelInfo,
		DebugWS:         false,
		MistralAPIKey:   "test-key",
		AnthropicAPIKey: "test-key",
	}

	runFinished := make(chan error, 1)
	wsServer := &websocket.Server{
		Handshake: func(_ *websocket.Config, _ *http.Request) error {
			return nil
		},
		Handler: func(conn *websocket.Conn) {
			sess := New("test-session", conn, cfg, testSkills, logger)
			runFinished <- sess.Run(context.Background())
		},
	}
	httpSrv := httptest.NewServer(wsServer)
	defer httpSrv.Close()

	wsURL := "ws" + httpSrv.URL[4:] + "/ws"
	conn, err := websocket.Dial(wsURL, "", "http://localhost")
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	conn.Close()

	select {
	case err := <-runFinished:
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
