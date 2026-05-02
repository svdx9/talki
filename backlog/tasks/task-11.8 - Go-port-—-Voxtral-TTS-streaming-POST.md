---
id: TASK-11.8
title: Go port — Voxtral TTS streaming POST
status: To Do
assignee: []
created_date: '2026-05-02 15:57'
labels:
  - go
  - port
  - backend
  - voxtral
milestone: m-0
dependencies: []
parent_task_id: TASK-11
priority: high
ordinal: 9000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Port the TTS path. Smallest of the upstream integrations.

## Wire protocol
- `POST https://api.mistral.ai/v1/audio/speech`
- Headers: `Authorization: Bearer <key>`, `Content-Type: application/json`.
- Body: `{\"model\":\"voxtral-mini-tts-2603\",\"input\":<text>,\"voice_id\":<voiceID>,\"response_format\":\"mp3\"}`
- Response: streaming MP3 bytes in the body.

## What to build
- `backend-go/cmd/server/internal/tts/voxtral.go`:
  - `func Stream(ctx context.Context, apiKey, voiceID, text string, sink io.Writer) error` — does the POST with `http.NewRequestWithContext`, on non-2xx returns a wrapped error including status + first 1KB of body, otherwise `io.Copy`s the body into `sink`.
- The session passes a sink that wraps the writer-goroutine: every chunk written to the sink becomes a binary frame on the client WS (binary frames carry MP3 audio for the OpusPlayer in the browser).
- `handleStartSession` triggers `tts.Stream(ctx, ..., scenario.OpeningLine, sink)` in a goroutine after sending `session_ready`. Errors logged but never crash the session.

## Tests
- `httptest.NewServer` returning a fixed MP3 body; assert that `Stream` writes the exact bytes to a `bytes.Buffer` sink.
- Error case: server returns 401; assert wrapped error message contains `401` and a snippet of the body.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Selecting a scenario triggers a TTS playback in the browser of that scenario's `opening_line`, identical to TS backend behavior
- [ ] #2 Non-2xx upstream responses produce a session-scoped log line with status + body snippet, but do NOT close the WS
- [ ] #3 Tests pass against an httptest server
<!-- AC:END -->
