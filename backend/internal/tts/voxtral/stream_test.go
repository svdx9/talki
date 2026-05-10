package voxtral

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/svdx9/talki/backend/internal/tts"
)

func fakeServer(t *testing.T, handler func(conn *websocket.Conn)) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//exhaustruct:ignore
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			t.Logf("server accept error: %v", err)
			return
		}
		defer func() { _ = conn.CloseNow() }()
		handler(conn)
	}))
	t.Cleanup(srv.Close)
	return "ws" + strings.TrimPrefix(srv.URL, "http")
}

func serverWrite(t *testing.T, conn *websocket.Conn, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("server marshal: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = conn.Write(ctx, websocket.MessageText, data)
	if err != nil {
		t.Logf("server write: %v", err)
	}
}

func serverRead(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("server read: %v", err)
	}
	var m map[string]any
	err = json.Unmarshal(data, &m)
	if err != nil {
		t.Fatalf("server unmarshal: %v", err)
	}
	return m
}

func doHandshake(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	serverWrite(t, conn, map[string]string{"type": "session.created"})
	update := serverRead(t, conn)
	if update["type"] != "session.update" {
		t.Errorf("expected session.update, got %v", update["type"])
	}
	serverWrite(t, conn, map[string]string{"type": "session.updated"})
}

func dial(t *testing.T, url string) *TranscriptionStreamingClient {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	//exhaustruct:ignore
	client, err := Dial(ctx, "test-key", "test-model", tts.AudioFormat{Encoding: "pcm_s16le", SampleRate: 16000}, &TranscriptionOptions{BaseURL: url})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return client
}

func TestDial_HappyPath(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})
	url := fakeServer(t, func(conn *websocket.Conn) {
		defer close(done)
		doHandshake(t, conn)

		audio := serverRead(t, conn)
		if audio["type"] != "input_audio.append" {
			t.Errorf("expected input_audio.append, got %v", audio["type"])
		}
		if _, ok := audio["audio"]; !ok {
			t.Error("input_audio.append missing audio field")
		}

		flush := serverRead(t, conn)
		if flush["type"] != "input_audio.flush" {
			t.Errorf("expected input_audio.flush, got %v", flush["type"])
		}

		serverWrite(t, conn, map[string]string{"type": "transcription.done"})
	})

	client := dial(t, url)

	var err error
	pcm := make([]byte, 1024)
	err = client.SendAudio(context.Background(), pcm)
	if err != nil {
		t.Fatalf("SendAudio: %v", err)
	}
	err = client.Flush(context.Background())
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}

	select {
	case ev, ok := <-client.Events():
		if !ok {
			t.Fatal("events channel closed before transcription.done")
		}
		if ev.Type != "transcription.done" {
			t.Errorf("expected transcription.done, got %s", ev.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for transcription.done")
	}

	_ = client.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("server goroutine did not finish")
	}
}

func TestDial_SpuriousFlush(t *testing.T) {
	t.Parallel()

	url := fakeServer(t, func(conn *websocket.Conn) {
		doHandshake(t, conn)
		serverWrite(t, conn, map[string]any{
			"type": "error",
			"error": map[string]string{
				"message": "Cannot flush audio before sending any audio bytes",
			},
		})
		serverWrite(t, conn, map[string]string{"type": "transcription.done"})
	})

	client := dial(t, url)
	defer func() { _ = client.Close() }()

	select {
	case ev, ok := <-client.Events():
		if !ok {
			t.Fatal("events channel closed unexpectedly")
		}
		if ev.Type == "error" {
			t.Errorf("spurious flush error leaked to events channel: %s", string(ev.Raw))
		}
		if ev.Type != "transcription.done" {
			t.Errorf("expected transcription.done after spurious error, got %s", ev.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event after spurious flush error")
	}
}

func TestDial_UnknownEventType(t *testing.T) {
	t.Parallel()

	url := fakeServer(t, func(conn *websocket.Conn) {
		doHandshake(t, conn)
		serverWrite(t, conn, map[string]string{"type": "some_future_event", "text": "hello"})
		serverWrite(t, conn, map[string]string{"type": "transcription.done"})
	})

	client := dial(t, url)
	defer func() { _ = client.Close() }()

	select {
	case ev, ok := <-client.Events():
		if !ok {
			t.Fatal("events channel closed unexpectedly")
		}
		if ev.Type != "some_future_event" {
			t.Errorf("expected some_future_event, got %s", ev.Type)
		}
		if len(ev.Raw) == 0 {
			t.Error("Raw payload is empty for unknown event")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for unknown event")
	}
}

func TestClose_WhileInFlight(t *testing.T) {
	t.Parallel()

	serverClosed := make(chan struct{})
	url := fakeServer(t, func(conn *websocket.Conn) {
		defer close(serverClosed)
		doHandshake(t, conn)
		_, _, _ = conn.Read(context.Background())
	})

	client := dial(t, url)

	pcm := make([]byte, 512)
	_ = client.SendAudio(context.Background(), pcm)
	_ = client.SendAudio(context.Background(), pcm)

	_ = client.Close()

	select {
	case _, ok := <-client.Events():
		if ok {
			for range client.Events() {
			}
		}
	default:
	}

	select {
	case <-serverClosed:
	case <-time.After(2 * time.Second):
		t.Fatal("server goroutine did not finish after client Close")
	}
}
