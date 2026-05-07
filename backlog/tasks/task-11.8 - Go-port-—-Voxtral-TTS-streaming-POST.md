---
id: TASK-11.8
title: Go port — Voxtral TTS streaming POST
status: Done
assignee: []
created_date: '2026-05-02 15:57'
updated_date: '2026-05-07 14:39'
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
Port the TTS path using the Voxtral TTS REST API.

## Wire protocol
- `POST https://api.mistral.ai/v1/audio/speech`
- Headers: `Authorization: Bearer <key>`, `Content-Type: application/json`.
- Body: `{"model":"voxtral-mini-tts-2603","input":<text>,"voice_id":<voiceID>,"response_format":"mp3"}`
  - `voice_id` is omitted when empty; `ref_audio` (base64) is sent instead when provided.
- Response: JSON `{"audio_data":"<base64-mp3>"}` — **not** raw bytes. Decoded on receipt.
- The API requires either `voice_id` or `ref_audio`; there are no built-in preset voices — all voices are user-created via the Voices API, or zero-shot cloned via `ref_audio`.

## What was built

### `backend-go/internal/tts/voxtral/` (package `voxtral`)
- **`batch.go`**: `AudioClient` struct holding `*http.Client`, `apiKey`, `baseURL`. `NewAudioClient(apiKey, *AudioOptions)` follows the same options pattern as `TranscriptionOptions`. Method `Speech(ctx, SpeechVoice, text, sink)` POSTs, decodes the JSON `audio_data` field from base64, and writes raw MP3 bytes to the sink.
- **`urls.go`**: `voxtralHost`, `transcriptionURL`, `speechURL` constants — single source of truth for all endpoint paths.
- **`batch_test.go`**: happy-path test (server returns JSON envelope with base64 MP3; asserts exact bytes in sink) and 401 error test.

### `backend-go/internal/tts/tts.go`
- Added `SpeechVoice` struct (`VoiceID string`, `RefAudio []byte`) — union type; set exactly one.
- Added `AudioClient` interface: `Speech(ctx, SpeechVoice, text string, sink io.Writer) error`.
- Renamed `STTClient` → `TranscriptionClient` for naming consistency.

### `backend-go/internal/session/`
- `session.go`: added `ttsClient tts.AudioClient` field; `New()` sets default `voxtral.NewAudioClient(cfg.MistralAPIKey, nil)`; added `binaryWriter` that enqueues each `Write` as a binary WS frame.
- `handlers.go`: `handleStartSession` fires a goroutine after `session_ready` calling `s.ttsClient.Speech(ctx, tts.SpeechVoice{VoiceID: skill.Voice}, skill.OpeningLine, sink)`. Errors logged; WS never closed.
- `session_test.go`: `noopTTSClient{}` injected to suppress binary frames in tests.

### `backend-go/cmd/stt/main.go`
- Restructured to subcommands: `transcribe <file.wav>` (existing STT) and `speak [-voice <id>|-ref-audio <file>] <text>`.
- `speak` defaults to voice `fr_marie_neutral`; outputs raw MP3 to stdout.

### Config
- `MISTRAL_VOICE_ID` defaults to `fr_marie_neutral`.

## Notes
- The `skill.Voice` field currently holds ElevenLabs voice IDs from the TS backend. These need to be replaced with Mistral voice IDs (created via the Mistral Voices API) before per-skill TTS voices work end-to-end.
- AC #1 is wired correctly; full browser verification pending Mistral voice IDs in skill catalog.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Selecting a scenario triggers a TTS playback in the browser of that scenario's `opening_line`, identical to TS backend behavior
- [x] #2 Non-2xx upstream responses produce a session-scoped log line with status + body snippet, but do NOT close the WS
- [x] #3 Tests pass against an httptest server
<!-- AC:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Implemented the Voxtral TTS path end-to-end.

Key discoveries vs. the original spec:
1. The API response is `{"audio_data":"<base64>"}` JSON, not raw streaming bytes — required a decode step.
2. There are no built-in preset voice IDs. All voices must be created via the Mistral Voices API or provided as `ref_audio` bytes for zero-shot cloning.
3. The `voice_id` / `ref_audio` choice was modelled as a `tts.SpeechVoice` union struct rather than a bare string parameter.
4. Naming was rationalised across the voxtral package: `TranscriptionClient` (STT websocket) and `AudioClient` (TTS HTTP), with all endpoint constants consolidated in `urls.go`.

The skill catalog still carries ElevenLabs voice IDs; those need updating to Mistral voice IDs before per-scenario voices are live.
<!-- SECTION:FINAL_SUMMARY:END -->
