package llm

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

func feedFixture(t *testing.T) io.Reader {
	t.Helper()
	f, err := os.Open("testdata/stream.sse")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

func TestStream_HappyPath(t *testing.T) {
	t.Parallel()

	s := parse(context.Background(), feedFixture(t))

	var got strings.Builder
	for delta := range s.Deltas() {
		got.WriteString(delta)
	}

	waitErr := s.Wait()
	if waitErr != nil {
		t.Fatalf("Wait() = %v, want nil", waitErr)
	}
	if got.Len() == 0 {
		t.Fatal("expected non-empty accumulated text, got empty")
	}
	if !strings.Contains(got.String(), "Hello") {
		t.Errorf("accumulated text %q does not contain expected word", got.String())
	}
}

func TestStream_Cancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	pr, pw := io.Pipe()

	s := parse(ctx, pr)

	// Write one delta, then cancel and close the pipe to unblock the scanner.
	go func() {
		_, _ = io.WriteString(pw, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"hi\"}}\n\n")
		cancel()
		_ = pw.CloseWithError(context.Canceled)
	}()

	// Drain all deltas so the consume goroutine can exit.
	for range s.Deltas() {
	}

	waitErr := s.Wait()
	if !errors.Is(waitErr, context.Canceled) {
		t.Fatalf("Wait() = %v, want context.Canceled", waitErr)
	}
}

func TestStream_ErrorEvent(t *testing.T) {
	t.Parallel()

	body := "event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"overloaded_error\",\"message\":\"Overloaded\"}}\n\n"
	s := parse(context.Background(), strings.NewReader(body))

	for range s.Deltas() {
	}

	err := s.Wait()
	if err == nil {
		t.Fatal("Wait() = nil, want non-nil error")
	}
	if !strings.Contains(err.Error(), "anthropic:") {
		t.Errorf("error %q does not contain 'anthropic:'", err.Error())
	}
}

func TestStream_UnknownEvent(t *testing.T) {
	t.Parallel()

	body := "event: unknown_event_type\ndata: {\"type\":\"unknown\"}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	s := parse(context.Background(), strings.NewReader(body))

	for range s.Deltas() {
	}

	unknownErr := s.Wait()
	if unknownErr != nil {
		t.Fatalf("Wait() = %v, want nil (unknown event must be ignored)", unknownErr)
	}
}
