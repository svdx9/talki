package session

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/svdx9/talki/backend-go/internal/config"
	"github.com/svdx9/talki/backend-go/internal/protocol"
	"github.com/svdx9/talki/backend-go/internal/skills"
)

var errUnknownSkill = errors.New("unknown skill")

type fakeRepo struct {
	byID map[string]*skills.Skill
}

func (f *fakeRepo) Get(id string) (*skills.Skill, error) {
	if s, ok := f.byID[id]; ok {
		return s, nil
	}
	return nil, errUnknownSkill
}

func (f *fakeRepo) Descriptions() []skills.SkillDescription {
	out := make([]skills.SkillDescription, 0, len(f.byID))
	for _, s := range f.byID {
		out = append(out, s.SkillDescription)
	}
	return out
}

func newTestSkill() *skills.Skill {
	//exhaustruct:ignore
	return &skills.Skill{
		SkillDescription: skills.SkillDescription{
			ID:    "skill-1",
			Title: "French Basics",
			Level: "beginner",
		},
		Locale:      "fr-FR",
		OpeningLine: "Bonjour, comment ça va?",
	}
}

func newTestRepo() *fakeRepo {
	s := newTestSkill()
	return &fakeRepo{byID: map[string]*skills.Skill{s.ID: s}}
}

func testServerAndURL(t *testing.T) (*httptest.Server, string, <-chan error) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	//exhaustruct:ignore
	cfg := config.Config{
		Port:            0,
		LogLevel:        slog.LevelInfo,
		DebugWS:         false,
		MistralAPIKey:   "test-key",
		AnthropicAPIKey: "test-key",
	}
	repo := newTestRepo()
	runFinished := make(chan error, 1)

	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//exhaustruct:ignore
		wc, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			t.Errorf("accept failed: %v", err)
			return
		}
		defer func() { _ = wc.CloseNow() }()
		sess := New("test-session", wc, cfg, repo, logger)
		runFinished <- sess.Run(r.Context())
	}))
	return httpSrv, "ws" + strings.TrimPrefix(httpSrv.URL, "http") + "/ws", runFinished
}

func dial(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	return conn
}

func sendJSON(t *testing.T, conn *websocket.Conn, msg any) {
	t.Helper()
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	werr := conn.Write(ctx, websocket.MessageText, data)
	if werr != nil {
		t.Fatalf("write: %v", werr)
	}
}

func recvJSON(t *testing.T, conn *websocket.Conn, timeout time.Duration, out any) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, data, err := conn.Read(ctx)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

func TestStartSessionWithUnknownScenario(t *testing.T) {
	t.Parallel()
	httpSrv, wsURL, _ := testServerAndURL(t)
	defer httpSrv.Close()

	conn := dial(t, wsURL)
	defer func() { _ = conn.CloseNow() }()

	sendJSON(t, conn, &protocol.StartSession{
		Type:       "start_session",
		ScenarioID: "unknown-scenario",
		SampleRate: 16000,
	})

	var errMsg protocol.Error
	rerr := recvJSON(t, conn, 2*time.Second, &errMsg)
	if rerr != nil {
		t.Fatalf("failed to receive: %v", rerr)
	}
	if errMsg.Type != "error" {
		t.Errorf("expected error, got %s", errMsg.Type)
	}
}

func TestStartUtteranceTwice(t *testing.T) {
	t.Parallel()
	httpSrv, wsURL, _ := testServerAndURL(t)
	defer httpSrv.Close()

	conn := dial(t, wsURL)
	defer func() { _ = conn.CloseNow() }()

	sendJSON(t, conn, &protocol.StartSession{
		Type:       "start_session",
		ScenarioID: "skill-1",
		SampleRate: 16000,
	})

	var ready protocol.SessionReady
	rerr := recvJSON(t, conn, 2*time.Second, &ready)
	if rerr != nil {
		t.Fatalf("failed to receive session_ready: %v", rerr)
	}
	if ready.Type != "session_ready" {
		t.Errorf("expected session_ready, got %s", ready.Type)
	}
	if ready.Greeting != "Bonjour, comment ça va?" {
		t.Errorf("expected greeting, got %s", ready.Greeting)
	}

	// Send both starts back-to-back. The first should be silent,
	// the second should produce an "already open" error. If the first
	// produced an unexpected response, we'd see it here instead of the error.
	sendJSON(t, conn, &protocol.StartUtterance{Type: "start_utterance"})
	sendJSON(t, conn, &protocol.StartUtterance{Type: "start_utterance"})

	var errMsg protocol.Error
	rerr2 := recvJSON(t, conn, 2*time.Second, &errMsg)
	if rerr2 != nil {
		t.Fatalf("failed to receive error response: %v", rerr2)
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
	httpSrv, wsURL, _ := testServerAndURL(t)
	defer httpSrv.Close()

	conn := dial(t, wsURL)
	defer func() { _ = conn.CloseNow() }()

	sendJSON(t, conn, &protocol.EndUtterance{Type: "end_utterance"})

	var errMsg protocol.Error
	rerr := recvJSON(t, conn, 2*time.Second, &errMsg)
	if rerr != nil {
		t.Fatalf("failed to receive: %v", rerr)
	}
	if errMsg.Type != "error" {
		t.Errorf("expected error, got %s", errMsg.Type)
	}
}

func TestCancelHandler(t *testing.T) {
	t.Parallel()
	httpSrv, wsURL, _ := testServerAndURL(t)
	defer httpSrv.Close()

	conn := dial(t, wsURL)
	defer func() { _ = conn.CloseNow() }()

	sendJSON(t, conn, &protocol.StartSession{
		Type:       "start_session",
		ScenarioID: "skill-1",
		SampleRate: 16000,
	})
	var ready protocol.SessionReady
	rerr := recvJSON(t, conn, 2*time.Second, &ready)
	if rerr != nil {
		t.Fatalf("failed to receive session_ready: %v", rerr)
	}

	sendJSON(t, conn, &protocol.StartUtterance{Type: "start_utterance"})
	sendJSON(t, conn, &protocol.Cancel{Type: "cancel"})
	// After cancel, opening once should be silent and opening again should error.
	sendJSON(t, conn, &protocol.StartUtterance{Type: "start_utterance"})
	sendJSON(t, conn, &protocol.StartUtterance{Type: "start_utterance"})

	var errMsg protocol.Error
	rerr2 := recvJSON(t, conn, 2*time.Second, &errMsg)
	if rerr2 != nil {
		t.Fatalf("failed to receive error response: %v", rerr2)
	}
	if errMsg.Type != "error" || !strings.Contains(errMsg.Message, "already open") {
		t.Errorf("expected 'already open' error after cancel, got: %s", errMsg.Message)
	}
}

func TestSessionClosesCleanly(t *testing.T) {
	t.Parallel()
	httpSrv, wsURL, runFinished := testServerAndURL(t)
	defer httpSrv.Close()

	conn := dial(t, wsURL)
	_ = conn.CloseNow()

	select {
	case err := <-runFinished:
		if err != nil {
			t.Logf("Session exited with error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("session did not exit within 5 seconds")
	}
}
