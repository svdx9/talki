---
id: TASK-11.11
title: Go port — DEBUG_WS + per-utterance peak-amplitude diagnostic
status: Done
assignee: []
created_date: '2026-05-02 15:57'
updated_date: '2026-05-10 14:55'
labels:
  - go
  - port
  - backend
  - diagnostics
milestone: m-0
dependencies: []
parent_task_id: TASK-11
priority: medium
ordinal: 12000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Carry over the diagnostic logs that proved load-bearing during the TS debugging effort.

## What to add
- When `DEBUG_WS=true`: per-frame log on the session WS read loop, format `session XXX <- text|binary frame, N bytes` (already in TASK-17 if it was implemented; otherwise add here).
- Per-utterance peak-amplitude log on `endUtterance`:
  - As 16-bit PCM streams in via `pushClientAudio`, scan each `int16` sample for `abs(s)`; track the running `utterancePeak`.
  - On `endUtterance`, log `session XXX: utterance peak=N (X.X dBFS), bytes=M` where dBFS = `20*log10(peak/32768)`. Reset both counters.
- Per-event STT log: `session XXX: STT event <type>` — already needed in TASK-18, ensure it is at info or warn level.
- Spurious-flush log line in STT client: `session XXX: Voxtral STT spurious flush ignored` (warn).
- Transcript content logs (warn level so they show up by default during debugging):
  - `session XXX: transcript delta <json-quoted>` per delta.
  - `session XXX: transcript final <json-quoted>` on `transcription.done`.

## Notes
- All these logs go through `slog` with structured fields (e.g. `slog.String(\"session_id\", id)`, `slog.Int(\"peak\", n)`) — do not hand-format prefixes.
- API keys must never be logged.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Setting `DEBUG_WS=true` produces per-frame logs identical in spirit to the TS backend (verify by running both backends and a single utterance)
- [x] #2 Each `end_utterance` produces a peak/dBFS/bytes log line
- [x] #3 STT events are logged at the right level (warn for events, error for non-spurious upstream errors)
- [x] #4 No API key value ever appears in any log output
- [x] #5 Transcript final-content log line appears on `transcription.done` and matches the buffered transcript
<!-- AC:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
## Implementation Complete

All diagnostic logs carried over from TS backend into Go port:

✓ **DEBUG_WS frame logs** — Already in place at `session.go:178-180`
✓ **Peak amplitude tracking** — Already in place at `session.go:196-205` and `handlers.go:134-143`
✓ **STT event logs** — Already in place at `session.go:232` (warn level)
✓ **Transcript logs** — Already in place at `session.go:236,241,245` (warn level)
✓ **Spurious flush warn log** — Added in `voxtral/stream.go:169`

## Changes Made

1. **`backend-go/internal/tts/voxtral/stream.go`**
   - Added `log/slog` import
   - Added `Logger *slog.Logger` field to `TranscriptionOptions`
   - Added `log *slog.Logger` field to `TranscriptionStreamingClient`
   - In `Dial()`, resolve logger from opts or use `slog.Default()`
   - In `readLoop()`, emit warn log on spurious flush

2. **`backend-go/internal/session/session.go`**
   - Updated `dialSTT` closure in `New()` to thread session logger through `TranscriptionOptions`

## Verification

- All Go tests pass (go test ./...)
- Code compiles without errors
- Spurious flush log now appears with structured fields when ignored
<!-- SECTION:FINAL_SUMMARY:END -->
