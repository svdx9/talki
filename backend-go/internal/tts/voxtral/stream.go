package voxtral

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/coder/websocket"
	"github.com/svdx9/talki/backend-go/internal/tts"
)

var errUnexpectedEventType = errors.New("unexpected event type")

// TranscriptionOptions configures a Dial call. All fields are optional.
type TranscriptionOptions struct {
	// BaseURL overrides the default Voxtral WebSocket endpoint.
	// Intended for tests that spin up a local fake server.
	BaseURL string
	// Logger is the structured logger for spurious flush diagnostics.
	// If nil, slog.Default() is used.
	Logger *slog.Logger
}

// TranscriptionStreamingClient is a live Voxtral STT connection. Always create via Dial.
// It implements tts.TranscriptionStreamingClient.
type TranscriptionStreamingClient struct {
	conn   *websocket.Conn
	events chan tts.Event
	cancel context.CancelFunc
	done   chan struct{}
	mu     sync.Mutex
	log    *slog.Logger
}

type rawServerEvent struct {
	Type  string          `json:"type"`
	Text  string          `json:"text"`
	Error *rawServerError `json:"error"`
}

type rawServerError struct {
	// Message may be a JSON string or an object, depending on the error.
	Message json.RawMessage `json:"message"`
}

// Dial opens a Voxtral realtime transcription connection, performs the session
// handshake (session.created → session.update → session.updated), then starts
// the background read loop. Returns a ready-to-use TranscriptionClient on success.
// Pass a non-nil opts to override defaults (e.g. BaseURL for tests).
func Dial(ctx context.Context, apiKey, model string, af tts.AudioFormat, opts *TranscriptionOptions) (*TranscriptionStreamingClient, error) {
	base := "wss://" + voxtralHost + transcriptionURL
	if opts != nil && opts.BaseURL != "" {
		base = opts.BaseURL
	}
	url := base + "?model=" + model

	//exhaustruct:ignore
	conn, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Authorization": []string{"Bearer " + apiKey},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("voxtral dial: %w", err)
	}

	err = expectEvent(ctx, conn, "session.created")
	if err != nil {
		_ = conn.CloseNow()
		return nil, err
	}

	updateMsg, err := json.Marshal(map[string]any{
		"type": "session.update",
		"session": map[string]any{
			"audio_format": map[string]any{
				"encoding":    af.Encoding,
				"sample_rate": af.SampleRate,
			},
		},
	})
	if err != nil {
		_ = conn.CloseNow()
		return nil, fmt.Errorf("voxtral: marshal session.update: %w", err)
	}
	err = conn.Write(ctx, websocket.MessageText, updateMsg)
	if err != nil {
		_ = conn.CloseNow()
		return nil, fmt.Errorf("voxtral: send session.update: %w", err)
	}

	err = expectEvent(ctx, conn, "session.updated")
	if err != nil {
		_ = conn.CloseNow()
		return nil, err
	}

	log := slog.Default()
	if opts != nil && opts.Logger != nil {
		log = opts.Logger
	}

	readCtx, cancel := context.WithCancel(context.Background())
	c := &TranscriptionStreamingClient{
		conn:   conn,
		events: make(chan tts.Event, 64),
		cancel: cancel,
		done:   make(chan struct{}),
		mu:     sync.Mutex{},
		log:    log,
	}
	go c.readLoop(readCtx)
	return c, nil
}

func expectEvent(ctx context.Context, conn *websocket.Conn, wantType string) error {
	_, data, err := conn.Read(ctx)
	if err != nil {
		return fmt.Errorf("voxtral: reading %s: %w", wantType, err)
	}
	var ev rawServerEvent
	err = json.Unmarshal(data, &ev)
	if err != nil {
		return fmt.Errorf("voxtral: decoding %s: %w", wantType, err)
	}
	if ev.Type != wantType {
		if ev.Error != nil {
			return fmt.Errorf("voxtral: expected %s, got %s: %s: %w", wantType, ev.Type, string(ev.Error.Message), errUnexpectedEventType)
		}
		return fmt.Errorf("voxtral: expected %s, got %s: %w", wantType, ev.Type, errUnexpectedEventType)
	}
	return nil
}

func (c *TranscriptionStreamingClient) readLoop(ctx context.Context) {
	// Close events before done so that any goroutine ranging over events
	// exits before the caller of Close() is unblocked by <-c.done.
	defer close(c.done)
	defer close(c.events)

	for {
		_, data, err := c.conn.Read(ctx)
		if err != nil {
			return
		}

		var ev rawServerEvent
		err = json.Unmarshal(data, &ev)
		if err != nil {
			select {
			case c.events <- tts.Event{Type: "unknown", Raw: data, Text: ""}:
			case <-ctx.Done():
				return
			}
			continue
		}

		// Voxtral emits a spurious error immediately after session.updated:
		// "Cannot flush audio before sending any audio bytes". This happens
		// because the SDK's internal event loop fires a flush before any audio
		// is queued. Drop it and keep the connection alive.
		if ev.Type == "error" && ev.Error != nil && isSpuriousFlushError(ev.Error.Message) {
			c.log.Warn("Voxtral STT spurious flush ignored")
			continue
		}

		select {
		case c.events <- tts.Event{Type: ev.Type, Text: ev.Text, Raw: data}:
		case <-ctx.Done():
			return
		}
	}
}

func isSpuriousFlushError(msg json.RawMessage) bool {
	var s string
	if json.Unmarshal(msg, &s) == nil {
		return strings.Contains(s, "Cannot flush audio before sending any audio bytes")
	}
	return strings.Contains(string(msg), "Cannot flush audio before sending any audio bytes")
}

// Events returns the channel of decoded STT events. The channel is closed
// when the connection closes or the read loop exits.
func (c *TranscriptionStreamingClient) Events() <-chan tts.Event {
	return c.events
}

// SendAudio base64-encodes pcm and sends it as an input_audio.append message.
// Safe to call concurrently; a mutex serializes writes to the connection.
func (c *TranscriptionStreamingClient) SendAudio(ctx context.Context, pcm []byte) error {
	payload, err := json.Marshal(map[string]string{
		"type":  "input_audio.append",
		"audio": base64.StdEncoding.EncodeToString(pcm),
	})
	if err != nil {
		return fmt.Errorf("voxtral: marshal audio: %w", err)
	}
	return c.write(ctx, payload)
}

// Flush sends input_audio.flush, signalling the end of the current utterance.
func (c *TranscriptionStreamingClient) Flush(ctx context.Context) error {
	payload, err := json.Marshal(map[string]string{"type": "input_audio.flush"})
	if err != nil {
		return fmt.Errorf("voxtral: marshal flush: %w", err)
	}
	return c.write(ctx, payload)
}

// End sends input_audio.end, signalling no more audio will be sent.
func (c *TranscriptionStreamingClient) End(ctx context.Context) error {
	payload, err := json.Marshal(map[string]string{"type": "input_audio.end"})
	if err != nil {
		return fmt.Errorf("voxtral: marshal end: %w", err)
	}
	return c.write(ctx, payload)
}

func (c *TranscriptionStreamingClient) write(ctx context.Context, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.Write(ctx, websocket.MessageText, data)
}

// Close cancels the read loop, sends End best-effort, and closes the connection.
func (c *TranscriptionStreamingClient) Close() error {
	c.cancel()
	_ = c.End(context.Background())
	<-c.done
	return c.conn.CloseNow()
}
