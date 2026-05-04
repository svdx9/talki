---
id: TASK-11.4
title: 'Go port — HTTP scaffolding (/api/health, /api/skills, static)'
status: Done
assignee: []
created_date: '2026-05-02 15:55'
updated_date: '2026-05-04 10:55'
labels:
  - go
  - port
  - backend
milestone: m-0
dependencies: []
parent_task_id: TASK-11
priority: high
ordinal: 5000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
## Hard constraints
- Routing uses `*http.ServeMux` from stdlib. No `chi`. (Three routes total — stdlib mux is enough.)
- Handlers receive their dependencies via constructor injection on a `Server` struct. No package-level state.

## What to build
- `backend-go/internal/server/server.go`:
  - `type Server struct { cfg config.Config; skills []skills.Skill; log *slog.Logger; mux *http.ServeMux; httpSrv *http.Server }`
  - `func New(cfg config.Config, sk []skills.Skill, log *slog.Logger) *Server`
  - `func (s *Server) ListenAndServe() error` and `func (s *Server) Shutdown(ctx context.Context) error`.
- `backend-go/internal/server/static.go`:
  - Mounts `http.FileServer(http.Dir(...))` for `../frontend/dist` resolved to an absolute path at startup. If the directory does not exist, log a warning and continue (parity with current TS behavior of mounting unconditionally).
- Routes:
  - `GET /api/health` → JSON `{"status":"ok","timestamp":"<RFC3339>"}`.
  - `GET /api/skills` → JSON array of `{id,title,level}` only (subset of skill).
  - everything else → static handler.
- Wire `Server` into `main.go`; remove the bare `http.Server` from TASK-12.

## Tests
- `httptest`-based tests for `/api/health` and `/api/skills`: assert status, content-type, and JSON shape (decode into a struct; no string equality).

## Implementation
- Created `backend-go/internal/server/server.go` with Server struct, New constructor, and handler methods
- Created `backend-go/internal/server/static.go` with static file serving
- Created `backend-go/internal/server/server_test.go` with httptest-based tests
- Updated `backend-go/cmd/server/main.go` to use the Server

All acceptance criteria met:
- [x] #1 Handler tests verify /api/health returns valid JSON with status=ok and recent RFC3339 timestamp
- [x] #2 Handler tests verify /api/skills returns id/title/level array
- [x] #3 Static handler serves files from ../frontend/dist when present
- [x] #4 Missing frontend/dist produces a warning, not an error
- [x] #5 Handler tests use struct decoding, not string equality
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 `curl localhost:8787/api/health` returns valid JSON with status=ok and a recent timestamp
- [ ] #2 `curl localhost:8787/api/skills` returns the same id/title/level array as the TS server (compare both running side-by-side on different ports)
- [ ] #3 Frontend static files (when present at `frontend/dist`) are served at `/`
- [ ] #4 Missing `frontend/dist` produces a startup warning, not an error
- [ ] #5 Handler tests pass and use struct decoding rather than string-equality JSON comparison
<!-- AC:END -->
