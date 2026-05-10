---
id: TASK-11
title: Go port — parent
status: Done
assignee: []
created_date: '2026-05-02 15:54'
updated_date: '2026-05-10 15:42'
labels:
  - go
  - port
  - backend
milestone: m-0
dependencies: []
priority: high
ordinal: 1000
---
## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Tracking task for the Go port of the backend currently implemented in TypeScript at `backend/`. The new server is built incrementally under `backend-go/cmd/server/` while the TS backend keeps running. Only when every subtask is Done and parity is manually verified do we cut over and delete the TS backend.

## Hard constraints (apply to every subtask)
- All Go code MUST live under `backend-go/cmd/server/` with feature subpackages under `backend-go/internal/<feature>/`. No top-level `internal/` and no `pkg/`. (We will reorganize to `/backend` only after cutover.)
- WebSocket (server accept + STT client dial) MUST use `github.com/coder/websocket`.
- Otherwise stdlib only. No chi, no pgx/sqlc, no oapi-codegen, no Anthropic SDK, no Mistral SDK, no YAML library, no dotenv loader.
- Skill files (`backend/src/skills/catalog/*.yaml`) are converted **once** to JSON under `backend-go/internal/skills/catalog/*.json`. The YAML originals are removed at cutover.
- Style follows the go-backend skill: explicit `err := f(); if err != nil` (no inline scoping), constructors over partial structs, contexts injected as the first parameter, `log/slog` for logging with `session_id` field, no global state.
- WS write serialization: each session has one writer goroutine fed by a `chan []byte`; `*websocket.Conn` is not safe for concurrent writes.

## Session model
One goroutine per session reads client frames; one goroutine reads STT events from Voxtral; one goroutine reads SSE deltas from Anthropic during a reply. All derive context from the session context which is cancelled on WS close. To unblock blocked reads on cancel, cancel the context passed to `conn.Read(ctx)` — `coder/websocket` ties read lifetime to context, no deadline hack needed.

## TS source-of-truth files to mirror
- Protocol: `shared/src/protocol.ts`
- Skill schema: `backend/src/skills/schema.ts`
- Session lifecycle / handler dispatch: `backend/src/session.ts`, `backend/src/index.ts`
- Voxtral STT integration (post-rewrite): `backend/src/integrations/mistral-realtime.ts`
- Voxtral TTS: `backend/src/session.ts` `sendToVoxtral`
- Anthropic streaming: `backend/src/session.ts` `streamAssistant`
- Config: `backend/src/config.ts`
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 All subtasks under this parent are Done
- [ ] #2 Manual end-to-end verification: scenario picker → greeting → push-to-talk → transcript → assistant streaming reply → TTS playback → cancel → end_session → reconnect, all working against the Go backend
- [ ] #3 TS backend (`backend/`) is removed and the new Go backend lives at `backend/` (renamed from `backend-go/`) with all root npm/dev scripts pointing at it
<!-- AC:END -->
