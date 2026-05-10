package voxtral

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/svdx9/talki/backend/internal/tts"
)

func TestSpeechClient_Speech_HappyPath(t *testing.T) {
	t.Parallel()

	mp3Body := []byte{0xFF, 0xFB, 0x90, 0x00, 0x00, 0x00, 0x00, 0x00}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"audio_data": base64.StdEncoding.EncodeToString(mp3Body),
		})
	}))
	t.Cleanup(srv.Close)

	//exhaustruct:ignore
	client := NewAudioClient("test-key", &AudioOptions{BaseURL: srv.URL})
	var buf bytes.Buffer
	//exhaustruct:ignore
	err := client.Speech(context.Background(), tts.SpeechVoice{VoiceID: "test-voice"}, "Hello", &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(buf.Bytes(), mp3Body) {
		t.Errorf("body mismatch: got %v, want %v", buf.Bytes(), mp3Body)
	}
}

func TestSpeechClient_Speech_Non2xx(t *testing.T) {
	t.Parallel()

	errBody := "unauthorized: invalid api key"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, errBody)
	}))
	t.Cleanup(srv.Close)

	//exhaustruct:ignore
	client := NewAudioClient("bad-key", &AudioOptions{BaseURL: srv.URL})
	//exhaustruct:ignore
	err := client.Speech(context.Background(), tts.SpeechVoice{VoiceID: "test-voice"}, "Hello", io.Discard)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "401") {
		t.Errorf("error should contain 401, got: %s", msg)
	}
	if !strings.Contains(msg, "unauthorized") {
		t.Errorf("error should contain body snippet, got: %s", msg)
	}
}
