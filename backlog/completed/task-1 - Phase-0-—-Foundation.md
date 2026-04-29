---
id: TASK-1
title: Phase 0 — Foundation
status: Done
assignee: []
created_date: '2026-04-29 17:25'
updated_date: '2026-04-29 20:07'
labels:
  - phase-0
  - foundation
dependencies: []
priority: high
---

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 pnpm workspaces initialised with backend/, frontend/, shared/
- [x] #2 tsconfig.base.json, .env.example, .gitignore, README in place
- [x] #3 shared/src/protocol.ts exports the WS message types
- [x] #4 Hono backend on :8787 with /api/health and static serving of frontend/dist
- [x] #5 Vite + Solid frontend skeleton with dev proxy to backend
- [x] #6 backend/src/config.ts validates env with Zod (3 API keys + PORT)
<!-- AC:END -->

## Implementation Notes

Phase 0 foundation complete: pnpm workspaces, Hono backend on :8787 with `/api/health` and static serving of `frontend/dist`, Vite + Solid frontend with `/api` proxy, Zod-validated config (Deepgram/Anthropic/ElevenLabs keys + PORT), and `shared/src/protocol.ts` with typed `ClientMsg`/`ServerMsg` discriminated unions and binary opus audio frames. Both packages typecheck clean.
