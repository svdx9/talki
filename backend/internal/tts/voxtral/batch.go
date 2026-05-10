package voxtral

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/svdx9/talki/backend/internal/tts"
)

var errUpstreamStatus = errors.New("voxtral speech: upstream error")

// AudioOptions configures a NewAudioClient call. All fields are optional.
type AudioOptions struct {
	// BaseURL overrides the default Voxtral TTS endpoint. Intended for tests.
	BaseURL string
	// HTTPClient overrides the default http.Client. Intended for tests.
	HTTPClient *http.Client
}

// AudioClient calls the Voxtral TTS HTTP API.
type AudioClient struct {
	httpClient *http.Client
	apiKey     string
	baseURL    string
}

// NewAudioClient returns a AudioClient for the given API key.
// Pass a non-nil opts to override defaults (e.g. BaseURL for tests).
func NewAudioClient(apiKey string, opts *AudioOptions) *AudioClient {
	c := &AudioClient{
		httpClient: &http.Client{},
		apiKey:     apiKey,
		baseURL:    "https://" + voxtralHost + speechURL,
	}
	if opts != nil {
		if opts.BaseURL != "" {
			c.baseURL = opts.BaseURL
		}
		if opts.HTTPClient != nil {
			c.httpClient = opts.HTTPClient
		}
	}
	return c
}

// Speech POSTs to the Voxtral TTS API and copies the MP3 response body into sink.
// On non-2xx responses it returns a wrapped error containing the status code and
// up to 1 KB of the response body.
func (c *AudioClient) Speech(ctx context.Context, voice tts.SpeechVoice, text string, sink io.Writer) error {
	body := map[string]any{
		"model":           "voxtral-mini-tts-2603",
		"input":           text,
		"response_format": "mp3",
	}
	switch {
	case voice.VoiceID != "":
		body["voice_id"] = voice.VoiceID
	case len(voice.RefAudio) > 0:
		body["ref_audio"] = base64.StdEncoding.EncodeToString(voice.RefAudio)
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("voxtral speech: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("voxtral speech: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("voxtral speech: do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode/100 != 2 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("%w: status %d: %s", errUpstreamStatus, resp.StatusCode, bytes.TrimSpace(snippet))
	}

	var result struct {
		AudioData string `json:"audio_data"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return fmt.Errorf("voxtral speech: decode response: %w", err)
	}

	dec := base64.NewDecoder(base64.StdEncoding, strings.NewReader(result.AudioData))
	_, err = io.Copy(sink, dec)
	if err != nil {
		return fmt.Errorf("voxtral speech: decode audio: %w", err)
	}
	return nil
}
