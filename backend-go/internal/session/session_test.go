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
	"github.com/svdx9/talki/backend-go/internal/llm"
	"github.com/svdx9/talki/backend-go/internal/protocol"
	"github.com/svdx9/talki/backend-go/internal/skills"
	"github.com/svdx9/talki/backend-go/internal/tts"
	"github.com/svdx9/talki/backend-go/internal/tts/voxtral"
)

var errUnknownSkill = errors.New("unknown skill")

type noopTTSClient struct{}

func (noopTTSClient) Speech(_ context.Context, _ tts.SpeechVoice, _ string, _ io.Writer) error {
	return nil
}

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

// fakeVoxtralServer starts a local server that mimics the Voxtral STT WebSocket API.
// It performs the handshake and then holds the connection open until the client closes.
// Returns a STTDialFunc that connects to the fake server instead of Voxtral.
func fakeVoxtralServer(t *testing.T) STTDialFunc {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//exhaustruct:ignore
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			t.Logf("fake voxtral accept: %v", err)
			return
		}
		defer func() { _ = conn.CloseNow() }()

		ctx := r.Context()
		writeJSON := func(v any) {
			data, _ := json.Marshal(v)
			_ = conn.Write(ctx, websocket.MessageText, data)
		}

		writeJSON(map[string]string{"type": "session.created"})
		_, _, _ = conn.Read(ctx) // consume session.update
		writeJSON(map[string]string{"type": "session.updated"})

		// Hold connection open until client closes.
		for {
			_, _, err := conn.Read(ctx)
			if err != nil {
				return
			}
		}
	}))
	t.Cleanup(srv.Close)

	fakeURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	return func(ctx context.Context, apiKey, model string, af tts.AudioFormat) (tts.TranscriptionClient, error) {
		return voxtral.Dial(ctx, apiKey, model, af, &voxtral.TranscriptionOptions{BaseURL: fakeURL})
	}
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
	dialSTT := fakeVoxtralServer(t)

	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//exhaustruct:ignore
		wc, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			t.Errorf("accept failed: %v", err)
			return
		}
		defer func() { _ = wc.CloseNow() }()
		sess := New("test-session", wc, cfg, repo, logger)
		sess.dialSTT = dialSTT
		sess.ttsClient = noopTTSClient{}
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

type fakeLLMStream struct {
	deltas chan string
	waitCh chan error
}

func (f *fakeLLMStream) Deltas() <-chan string { return f.deltas }
func (f *fakeLLMStream) Wait() error           { return <-f.waitCh }

type fakeLLMClient struct {
	deltas  []string
	waitErr error
}

func (f *fakeLLMClient) Stream(_ context.Context, _ llm.Request) (llmStream, error) {
	fs := &fakeLLMStream{
		deltas: make(chan string, len(f.deltas)),
		waitCh: make(chan error, 1),
	}
	for _, d := range f.deltas {
		fs.deltas <- d
	}
	close(fs.deltas)
	fs.waitCh <- f.waitErr
	return fs, nil
}

func newDirectSession(llmC llmClientIface, ttsC tts.AudioClient) (*Session, chan outgoingMsg) {
	out := make(chan outgoingMsg, 64)
	//exhaustruct:ignore
	return &Session{
		id:        "test",
		outgoing:  out,
		log:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		scenario:  newTestSkill(),
		llmClient: llmC,
		ttsClient: ttsC,
	}, out
}

func TestStreamAssistantHappyPath(t *testing.T) {
	t.Parallel()
	deltas := []string{"Hello", ", ", "world"}
	fakeClient := &fakeLLMClient{deltas: deltas, waitErr: nil}
	s, out := newDirectSession(fakeClient, noopTTSClient{})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	s.streamAssistant(ctx, "hello user")

	// Collect messages: 3 deltas + 1 done
	var msgs []map[string]any
	for i := 0; i < len(deltas)+1; i++ {
		select {
		case msg := <-out:
			if msg.typ != websocket.MessageText {
				t.Fatalf("expected text message, got type %d", msg.typ)
			}
			var m map[string]any
			if err := json.Unmarshal(msg.data, &m); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}
			msgs = append(msgs, m)
		case <-ctx.Done():
			t.Fatal("timeout waiting for messages")
		}
	}

	// Verify 3 assistant_text_delta messages
	for i, delta := range deltas {
		if msgType, ok := msgs[i]["type"].(string); !ok || msgType != "assistant_text_delta" {
			t.Errorf("msg %d: expected assistant_text_delta, got %v", i, msgs[i]["type"])
		}
		if text, ok := msgs[i]["text"].(string); !ok || text != delta {
			t.Errorf("msg %d: expected delta %q, got %q", i, delta, text)
		}
	}

	// Verify 1 assistant_done with concatenated text
	lastIdx := len(deltas)
	if msgType, ok := msgs[lastIdx]["type"].(string); !ok || msgType != "assistant_done" {
		t.Errorf("final message: expected assistant_done, got %v", msgs[lastIdx]["type"])
	}
	if fullText, ok := msgs[lastIdx]["fullText"].(string); !ok || fullText != "Hello, world" {
		t.Errorf("final message: expected fullText='Hello, world', got %q", fullText)
	}

	// Verify history was updated
	if len(s.convHistory) != 2 {
		t.Errorf("expected 2 messages in history, got %d", len(s.convHistory))
	}
	if len(s.convHistory) >= 1 && s.convHistory[0].Role != "user" {
		t.Errorf("expected first message to be user, got %s", s.convHistory[0].Role)
	}
	if len(s.convHistory) >= 2 && s.convHistory[1].Role != "assistant" {
		t.Errorf("expected second message to be assistant, got %s", s.convHistory[1].Role)
	}
}

type blockingLLMClient struct {
	blockCh chan struct{}
}

func (b *blockingLLMClient) Stream(llmCtx context.Context, _ llm.Request) (llmStream, error) {
	fs := &fakeLLMStream{
		deltas: make(chan string, 1),
		waitCh: make(chan error, 1),
	}
	fs.deltas <- "first"

	// Block until ctx is cancelled or blockCh is closed
	go func() {
		<-llmCtx.Done()
		close(b.blockCh)
	}()
	go func() {
		<-b.blockCh
		close(fs.deltas)
		fs.waitCh <- context.Canceled
	}()

	return fs, nil
}

func TestStreamAssistantCancelStopsStream(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	fakeClient := &blockingLLMClient{blockCh: make(chan struct{})}
	s, out := newDirectSession(fakeClient, noopTTSClient{})

	// Start streamAssistant in a goroutine
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.streamAssistant(ctx, "test input")
	}()

	// Receive the first delta
	var firstMsg map[string]any
	select {
	case m := <-out:
		if err := json.Unmarshal(m.data, &firstMsg); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for first delta")
	}

	if msgType, ok := firstMsg["type"].(string); !ok || msgType != "assistant_text_delta" {
		t.Errorf("expected assistant_text_delta, got %v", firstMsg["type"])
	}

	// Call cancel
	s.handleCancel(ctx)

	// Wait for streamAssistant to finish
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("streamAssistant did not finish after cancel")
	}

	// Verify no assistant_done in the outgoing channel
	select {
	case m := <-out:
		var msg map[string]any
		json.Unmarshal(m.data, &msg)
		if msgType, ok := msg["type"].(string); ok && msgType == "assistant_done" {
			t.Error("expected no assistant_done after cancel")
		}
	default:
		// Good, no more messages
	}
}
