---
id: TASK-11.9
title: Go port — Anthropic SSE streaming client (hand-rolled)
status: To Do
assignee: []
created_date: '2026-05-02 15:57'
labels:
  - go
  - port
  - backend
  - anthropic
milestone: m-0
dependencies: []
parent_task_id: TASK-11
priority: high
ordinal: 10000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Hand-rolled SSE client for Anthropic's Messages API. Replaces the official SDK used in TS (`backend/src/session.ts` `streamAssistant`).

## Wire protocol
- `POST https://api.anthropic.com/v1/messages`
- Headers:
  - `x-api-key: <key>`
  - `anthropic-version: 2023-06-01`
  - `content-type: application/json`
- Request body: `{model, max_tokens, system: [{type:\"text\", text:..., cache_control:{type:\"ephemeral\"}}], messages: [{role, content}], stream: true}`. Use the same `model = \"claude-sonnet-4-6\"` and `max_tokens = 1024` as TS.
- Response: SSE — `event: <name>\\ndata: <json>\\n\\n` records.

## Events to handle
- `content_block_delta` → extract `delta.text` and yield as a Delta.
- `message_delta` → carries `stop_reason`; record on the stream object.
- `message_stop` → terminates the stream.
- `error` (Anthropic-side) → terminate with wrapped error.
- Anything else → log at debug and ignore (forward-compatible).

## What to build
- `backend-go/internal/llm/anthropic.go`:
```go
type Message struct { Role string; Content string }
type Request struct { Model string; MaxTokens int; System string; Messages []Message }
type Stream struct { /* unexported */ }

func NewClient(apiKey string, hc *http.Client) *Client
func (c *Client) Stream(ctx context.Context, req Request) (*Stream, error)
func (s *Stream) Deltas() <-chan string
func (s *Stream) Wait() error  // returns terminal error or nil
```
- Use `bufio.Scanner` with `MaxTokenSize` raised to 1MB to handle long content blocks.
- Cancel via context: cancelling the ctx closes the response body and unblocks the scanner.

## Tests
- Capture a real Anthropic SSE response body to a fixture under `internal/llm/testdata/stream.sse`. Test the parser by feeding the fixture through the same code that reads from the real response (factor a `parse(io.Reader) (*Stream, error)`).
- Test cases:
  - Happy path: deltas accumulate to expected text; `Wait()` returns nil.
  - Mid-stream cancel: `Wait()` returns `context.Canceled` and the deltas channel closes.
  - Anthropic `error` event: `Wait()` returns a wrapped error containing the upstream message.
  - Unknown event type in the fixture: ignored, no crash.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Real call to Anthropic with both API keys exported produces streaming `assistant_text_delta` ServerMsgs to the browser, character-by-character
- [ ] #2 SSE parser unit tests pass against a captured fixture committed under `internal/llm/testdata/`
- [ ] #3 Cancelling the context mid-stream returns `context.Canceled` from `Stream.Wait()` and the underlying HTTP body is closed (no leaked connections)
- [ ] #4 Unknown SSE event types do not crash the parser
<!-- AC:END -->
