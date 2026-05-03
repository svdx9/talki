---
id: TASK-11.5
title: Go port — protocol types + WebSocket upgrade at /api/ws
status: To Do
assignee: []
created_date: '2026-05-02 15:55'
labels:
  - go
  - port
  - backend
milestone: m-0
dependencies: []
parent_task_id: TASK-11
priority: high
ordinal: 6000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Define the wire protocol structs (mirrors `shared/src/protocol.ts`) and accept WebSocket upgrades. No session lifecycle yet — the handler just accepts, logs, and closes.

## Hard constraints
- WebSocket library: `golang.org/x/net/websocket` ONLY.
- No third-party JSON libs — use `encoding/json`.

## ClientMsg / ServerMsg in TS (single source of truth)
```ts
ClientMsg =
  | { type: "start_session"; scenarioId: string; sampleRate: number }
  | { type: "end_session" }
  | { type: "start_utterance" }
  | { type: "end_utterance" }
  | { type: "cancel" }

ServerMsg =
  | { type: "session_ready"; greeting?: string }
  | { type: "transcript"; text: string; isFinal: boolean }
  | { type: "assistant_text_delta"; text: string }
  | { type: "assistant_done"; fullText: string }
  | { type: "error"; message: string }
```

## What to build
- `backend-go/internal/protocol/protocol.go`:
  - `type RawClientMsg struct { Type string \`json:\"type\"\` }` for first-pass discriminator decoding.
  - One struct per ClientMsg variant (e.g. `StartSession{ ScenarioID string \`json:\"scenarioId\"\`; SampleRate int \`json:\"sampleRate\"\` }`). Note: TS uses camelCase, MUST be preserved in JSON tags.
  - One struct per ServerMsg variant with constructors (`NewSessionReady(greeting string)`, `NewTranscript(text string, final bool)`, etc.) so callers cannot forget required fields.
  - `func DecodeClient(payload []byte) (any, error)` — first decodes `RawClientMsg`, then switches on `Type` and decodes the full struct. Returns `ErrUnknownMessageType` (sentinel) for unknown types.
- Round-trip JSON tests for every variant.

## WS upgrade
- `backend-go/internal/server/ws.go`:
  - Use `websocket.Server{ Handshake: func(*websocket.Config, *http.Request) error { return nil }, Handler: ... }` to accept any Origin (frontend served from same host today).
  - Mount at `/api/ws` from `Server.New`.
  - The handler currently: generates a session ID (`crypto/rand` + base32, 6 chars lowercase), logs `session XXX connected`, blocks until the conn closes, logs `session XXX disconnected`.

## x/net/websocket quirk to document in code
- It distinguishes binary vs text via the Go type passed to `Receive`/`Send` (`[]byte` vs `string`). For mixed traffic, use `websocket.Message.Receive(conn, &raw)` where `raw` is `[]byte` and inspect `conn.PayloadType` (1 = text, 2 = binary) per frame. Add a comment in `ws.go` citing this.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Every variant of ClientMsg/ServerMsg has a Go struct with correct camelCase JSON tags and round-trips through `encoding/json`
- [ ] #2 `DecodeClient` correctly dispatches on the `type` field and returns a typed value
- [ ] #3 Unknown message types produce `ErrUnknownMessageType` (testable via `errors.Is`)
- [ ] #4 Connecting to `ws://localhost:8787/api/ws` from a browser succeeds; the server logs connect/disconnect with a session ID
- [ ] #5 The frontend (`frontend/`) connects against the Go backend without code changes — its connection-status indicator goes green
<!-- AC:END -->
