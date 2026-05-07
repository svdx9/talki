---
id: TASK-11.7
title: Go port — Voxtral STT realtime client over coder/websocket
status: Done
assignee: []
created_date: '2026-05-02 15:56'
updated_date: '2026-05-07 09:13'
labels:
  - go
  - port
  - backend
  - voxtral
milestone: m-0
dependencies: []
parent_task_id: TASK-11
priority: high
ordinal: 8000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Hand-rolled client for Mistral's realtime transcription WebSocket API, replacing the SDK we use in TS (`backend/src/integrations/mistral-realtime.ts`). This is the largest single piece of the port.

## Hard constraints
- WS dial uses `github.com/coder/websocket`.
- No Mistral SDK.

## Wire protocol (verified against the SDK source during the TS rewrite)
- URL: `wss://api.mistral.ai/v1/audio/transcriptions/realtime?model=<model>`
- Header: `Authorization: Bearer <api_key>`
- Outgoing JSON messages (text frames):
  - `{\"type\":\"session.update\",\"session\":{\"audio_format\":{\"encoding\":\"pcm_s16le\",\"sample_rate\":<n>}}}`
  - `{\"type\":\"input_audio.append\",\"audio\":\"<base64-pcm>\"}`
  - `{\"type\":\"input_audio.flush\"}`
  - `{\"type\":\"input_audio.end\"}`
- Incoming events (text frames, JSON):
  - `session.created`, `session.updated`
  - `transcription.language`
  - `transcription.text.delta` (`{type, text}`)
  - `transcription.segment.delta` (`{type, text?, ...}`)
  - `transcription.done`
  - `error` (`{type, error: {message: string|object}}`)

## Spurious-flush workaround (carry over from TS)
After session.updated, Voxtral emits an `error` event with message containing `Cannot flush audio before sending any audio bytes`. **Drop it and continue.** Cite the cause in a code comment. Do NOT close the connection. See [backend/src/session.ts](backend/src/session.ts) for the equivalent comment text.

## What to build
- `backend-go/internal/stt/voxtral.go`:
```go
type AudioFormat struct { Encoding string; SampleRate int }
type Event struct { Type string; Text string; Raw json.RawMessage }
type Client struct { /* unexported */ }

func Dial(ctx context.Context, apiKey, model string, fmt AudioFormat) (*Client, error)
func (c *Client) SendAudio(ctx context.Context, pcm []byte) error
func (c *Client) Flush(ctx context.Context) error
func (c *Client) End(ctx context.Context) error
func (c *Client) Events() <-chan Event
func (c *Client) Close() error
```
- `Dial` opens the WS via `websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": []string{"Bearer " + apiKey}}})`, reads the first frame, asserts `session.created`, sends a `session.update` with the requested format, waits for `session.updated`, then starts a read goroutine that decodes frames into typed `Event`s on a buffered channel (size 64). Unknown types still get an `Event{Type: ..., Raw: ...}` so the caller can log them.
- Read frames with `conn.Read(ctx)` → `(websocket.MessageType, []byte, error)`.
- Write frames with `conn.Write(ctx, websocket.MessageText, data)`. Writer methods take a `*sync.Mutex` so multiple senders cannot interleave. `SendAudio` base64-encodes the PCM and json-marshals once per call.
- `Close` cancels the read loop, calls `End` best-effort, closes the conn.
- Spurious-flush filter: in the read loop, if event is `error` AND message contains `Cannot flush audio before sending any audio bytes`, log at warn and continue. Else forward.

## Wire into Session
- `handleStartSession` calls `stt.Dial(ctx, cfg.MistralAPIKey, \"voxtral-mini-transcribe-realtime-2602\", AudioFormat{\"pcm_s16le\", msg.SampleRate})`. On error, send `error` ServerMsg and bail.
- A per-session goroutine reads `client.Events()` and translates:
  - `transcription.text.delta` → `appendTranscript(text)` + send `transcript{text, isFinal:false}`
  - `transcription.segment.delta` (if `text != \"\"`) → same
  - `transcription.done` → send `transcript{\"\", isFinal:true}`
  - `error` (non-spurious) → log + send `error` ServerMsg
- `pushClientAudio` calls `stt.SendAudio`. Track `sttBytesPushed` and `utterancePeak` per utterance for the diagnostic log.
- `endUtterance` calls `stt.Flush` if `sttBytesPushed > 0`; otherwise log `end_utterance with no audio — skipping flush`. Reset counters.
- `closeVoxtral` (on session end / WS close): cancel read loop, call `End` best-effort, `Close` the conn.

## Tests
- A faked Voxtral server using `httptest.NewServer` + `websocket.Accept(w, r, nil)` (from `github.com/coder/websocket`) that accepts a connection, emits scripted JSON events, and verifies the messages the client sends.
- Test cases (slice-of-structs):
  - Happy path: dial → session.created → session.updated → caller sends 1KB audio → flush → done event flows out the events channel.
  - Spurious flush: server emits the early error; client must drop it and remain functional for subsequent audio.
  - Unknown event type: surfaced as `Event{Type:..., Raw:...}` rather than dropped silently.
  - Close while audio in flight: no panic, `Events()` channel closes.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Speaking `Bonjour` produces `transcription.text.delta` events that the Go server forwards to the browser as `transcript` ServerMsgs, identical to the TS backend behavior
- [ ] #2 The spurious flush error is logged at warn and never reaches the client
- [ ] #3 `go test -race ./cmd/server/internal/stt` passes including the faked-server tests
- [ ] #4 Two consecutive utterances in the same session both produce transcripts (regression test for the TS bug where the second utterance was silent)
- [ ] #5 Closing the client WS during an active utterance does not leak goroutines (verify with a small leak check or `runtime.NumGoroutine` before/after)
<!-- AC:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Implemented `backend-go/internal/stt/voxtral.go` with `Dial`, `SendAudio`, `Flush`, `End`, `Events`, `Close` and a background `readLoop` that filters the spurious flush error. Exported `DialURL` for test injection.

Wired into session: `Session` gained `dialSTT STTDialFunc`, `sttClient`, `sttWg`, `sttBytesPushed`, `utterancePeak` fields. `handleStartSession` dials STT and starts `readSTTEvents` goroutine; `handleEndUtterance` logs peak/dBFS and calls `Flush`; `handleEndSession` calls `closeSTT`; `Run` defers `closeSTT` for WS-close cleanup. `pushAudio` scans PCM samples for peak amplitude and forwards bytes to the STT client.

Four tests in `voxtral_test.go` (happy path, spurious flush, unknown event type, close while in flight) all pass with `-race`. Session tests updated to use a `fakeVoxtralServer` injected via `dialSTT`; full suite passes.
<!-- SECTION:FINAL_SUMMARY:END -->
