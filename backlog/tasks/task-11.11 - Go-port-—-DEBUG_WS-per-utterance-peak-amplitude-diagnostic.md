---
id: TASK-11.11
title: Go port — DEBUG_WS + per-utterance peak-amplitude diagnostic
status: To Do
assignee: []
created_date: '2026-05-02 15:57'
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
- [ ] #1 Setting `DEBUG_WS=true` produces per-frame logs identical in spirit to the TS backend (verify by running both backends and a single utterance)
- [ ] #2 Each `end_utterance` produces a peak/dBFS/bytes log line
- [ ] #3 STT events are logged at the right level (warn for events, error for non-spurious upstream errors)
- [ ] #4 No API key value ever appears in any log output
- [ ] #5 Transcript final-content log line appears on `transcription.done` and matches the buffered transcript
<!-- AC:END -->
