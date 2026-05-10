package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

var (
	errUpstreamStatus = errors.New("anthropic: upstream error")
	errAnthropicEvent = errors.New("anthropic: upstream error event")
)

// Message is a single turn in a conversation.
type Message struct {
	Role    string
	Content string
}

// Request describes a streaming messages call.
type Request struct {
	Model     string
	MaxTokens int
	System    string // plain text; sent as a single ephemeral cache block
	Messages  []Message
}

// Client calls the Anthropic Messages API.
type Client struct {
	hc      *http.Client
	apiKey  string
	baseURL string
}

// NewClient returns a Client for the given API key and base URL.
// If baseURL is empty, it defaults to the Anthropic Messages API endpoint.
// Pass a non-nil hc to override the default http.Client (useful for tests).
func NewClient(apiKey, baseURL string, hc *http.Client) *Client {
	if hc == nil {
		hc = &http.Client{}
	}
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1/messages"
	}
	return &Client{
		hc:      hc,
		apiKey:  apiKey,
		baseURL: baseURL,
	}
}

// Stream sends a streaming request to the Anthropic API and returns a *Stream.
// The caller must call Stream.Wait() to release resources even if Deltas() is exhausted.
func (c *Client) Stream(ctx context.Context, req Request) (*Stream, error) {
	msgs := make([]map[string]string, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = map[string]string{"role": m.Role, "content": m.Content}
	}
	body := map[string]any{
		"model":      req.Model,
		"max_tokens": req.MaxTokens,
		"system": []map[string]any{
			{
				"type":          "text",
				"text":          req.System,
				"cache_control": map[string]string{"type": "ephemeral"},
			},
		},
		"messages": msgs,
		"stream":   true,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("anthropic: build request: %w", err)
	}
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("content-type", "application/json")

	resp, err := c.hc.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: do request: %w", err)
	}

	if resp.StatusCode/100 != 2 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		_ = resp.Body.Close()
		return nil, fmt.Errorf("%w: status %d: %s", errUpstreamStatus, resp.StatusCode, bytes.TrimSpace(snippet))
	}

	s := parse(ctx, resp.Body)

	// Close the response body once the stream goroutine finishes.
	go func() {
		<-s.done
		_ = resp.Body.Close()
	}()

	return s, nil
}

// Stream carries the channel of text deltas and the terminal error.
type Stream struct {
	deltas chan string
	done   chan struct{}
	err    error
}

// Deltas returns a channel that yields each text delta in order.
// The channel is closed when the stream ends (normally or due to error/cancel).
func (s *Stream) Deltas() <-chan string {
	return s.deltas
}

// Wait blocks until the stream has fully terminated and returns the terminal error.
// Returns nil on a clean end_turn, context.Canceled on cancellation, or a wrapped
// upstream error if Anthropic sent an error event.
func (s *Stream) Wait() error {
	<-s.done
	return s.err
}

// parse starts the consume goroutine and returns the *Stream immediately.
func parse(ctx context.Context, r io.Reader) *Stream {
	//exhaustruct:ignore
	s := &Stream{
		deltas: make(chan string),
		done:   make(chan struct{}),
	}
	go s.consume(ctx, r)
	return s
}

func (s *Stream) consume(ctx context.Context, r io.Reader) {
	defer close(s.done)
	defer close(s.deltas)

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)

	var eventName, eventData string
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			eventName = strings.TrimPrefix(line, "event: ")
			eventData = ""
		case strings.HasPrefix(line, "data: "):
			eventData = strings.TrimPrefix(line, "data: ")
		case line == "":
			if eventName != "" {
				stop, err := s.dispatch(ctx, eventName, eventData)
				eventName = ""
				eventData = ""
				if stop {
					s.err = err
					return
				}
			}
		}
	}

	scanErr := scanner.Err()
	if scanErr != nil {
		if ctx.Err() != nil {
			s.err = ctx.Err()
		} else {
			s.err = fmt.Errorf("SSE scan: %w", scanErr)
		}
	} else if ctx.Err() != nil {
		s.err = ctx.Err()
	}
}

type contentBlockDeltaEvent struct {
	Delta struct {
		Text string `json:"text"`
	} `json:"delta"`
}

type anthropicErrorEvent struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// dispatch processes one complete SSE event. Returns (stop, err) where stop=true
// signals that the goroutine should exit.
func (s *Stream) dispatch(ctx context.Context, event, data string) (bool, error) {
	switch event {
	case "content_block_delta":
		var ev contentBlockDeltaEvent
		unmarshalErr := json.Unmarshal([]byte(data), &ev)
		if unmarshalErr != nil {
			slog.Debug("anthropic: failed to unmarshal content_block_delta", "error", unmarshalErr)
			return false, nil
		}
		if ev.Delta.Text == "" {
			return false, nil
		}
		select {
		case s.deltas <- ev.Delta.Text:
		case <-ctx.Done():
			return true, ctx.Err()
		}

	case "message_delta":
		// stop_reason is informational; no action required.

	case "message_stop":
		return true, nil

	case "error":
		var ev anthropicErrorEvent
		unmarshalErr := json.Unmarshal([]byte(data), &ev)
		if unmarshalErr != nil {
			return true, fmt.Errorf("anthropic: error event (unparseable): %w", unmarshalErr)
		}
		return true, fmt.Errorf("%w: %s", errAnthropicEvent, ev.Error.Message)

	default:
		slog.Debug("anthropic: unknown SSE event", "event", event)
	}

	return false, nil
}
