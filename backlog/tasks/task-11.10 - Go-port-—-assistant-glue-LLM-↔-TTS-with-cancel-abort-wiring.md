---
id: TASK-11.10
title: Go port — assistant glue (LLM ↔ TTS) with cancel/abort wiring
status: To Do
assignee: []
created_date: '2026-05-02 15:57'
labels:
  - go
  - port
  - backend
milestone: m-0
dependencies: []
parent_task_id: TASK-11
priority: high
ordinal: 11000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Tie together STT → LLM → TTS. End-to-end conversation works after this task.

## What to build
- In `internal/session/session.go`:
  - Add `assistantCancel context.CancelFunc` for the in-flight LLM stream.
  - `func (s *Session) streamAssistant(parent context.Context, userText string)` — mirrors `backend/src/session.ts` `streamAssistant`:
    - Append `{role: \"user\", content: userText}` to history.
    - Build the `system` text block from `s.scenario.SystemPrompt`. (Note: TS uses an array with `cache_control: ephemeral`. The Go SDK-less path can pass `system` as a single string in v1; if cache_control is required for parity, switch to the array form using `json.RawMessage`.)
    - Cancel any prior `assistantCancel`. Build a new `ctx, cancel := context.WithCancel(parent)`. Store cancel on the session.
    - Open `llm.Stream(ctx, req)`. Read deltas; for each:
      - Send `assistant_text_delta` ServerMsg.
      - Append to `fullText`.
      - Fire-and-forget a goroutine that calls `tts.Stream(ctx, ..., delta, sink)` so audio plays per-delta (parity with TS).
    - On `Stream.Wait()` returning nil: append assistant turn to history, send `assistant_done{fullText}`.
    - On ctx-cancelled: log `assistant stream cancelled` (warn) and DO NOT send `assistant_done`.
    - On other error: log + send `error` ServerMsg.
- `handleEndUtterance` calls `streamAssistant` in a goroutine if transcript is non-empty.
- `handleCancel` and `handleEndSession` call `cancelAssistant()` which fires the stored cancel and clears it.

## Concurrency notes
- Per-delta TTS goroutines may race. They write into the same WS-writer channel, which serializes; they are independent HTTP calls so concurrency is fine. But: a SECOND assistant turn cancelling while TTS goroutines from the FIRST turn are still in flight — those goroutines should see ctx-cancel and abort their HTTP requests cleanly. Verify by sharing the `ctx` passed to `streamAssistant` with the TTS goroutines.

## Tests
- An integration-flavored test that wires fake `llm.Client` and `tts` into a session and verifies that:
  - User text yields N deltas → N `assistant_text_delta` server messages → 1 `assistant_done` with the concatenation.
  - Calling `Cancel` during streaming stops further deltas and prevents the `assistant_done` message.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Full happy path: speak French, see French transcript, see streaming assistant reply text, hear streaming TTS playback in the browser — entirely served by the Go backend
- [ ] #2 Pressing cancel mid-stream stops both the assistant text delta stream and any further TTS bytes for that turn
- [ ] #3 Conversation history is correctly accumulated turn-over-turn (assistant remembers previous user input)
- [ ] #4 No goroutine leaks across many turns — verify via `runtime.NumGoroutine` baseline
<!-- AC:END -->
