---
id: TASK-11.1
title: Go port — module skeleton + main + graceful shutdown
status: Done
assignee: []
created_date: '2026-05-02 15:54'
updated_date: '2026-05-02 16:37'
labels:
  - go
  - port
  - backend
milestone: m-0
dependencies: []
parent_task_id: TASK-11
priority: high
ordinal: 2000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Bootstrap the Go module under `backend-go/`. Establishes the directory shape, build/run targets, and a minimal HTTP server that boots and shuts down cleanly. No business logic yet.

## Hard constraints (must read parent TASK-11)
- WebSocket: `golang.org/x/net/websocket` ONLY (added now even though no WS handler is wired yet, so the dependency is captured).
- Otherwise stdlib only.
- All code lives under `backend-go/cmd/server/`. Feature subpackages live under `backend-go/cmd/server/internal/<feature>/`. There is no top-level `internal/` for now.

## What to build
- `backend-go/go.mod` (module path `github.com/svdx9/talki/backend-go`).
- `backend-go/cmd/server/main.go`: parses flags (none yet), reads `PORT` from env (default 8787), constructs an `*http.Server` with no routes, listens, and shuts down on SIGINT/SIGTERM via `signal.NotifyContext` + `srv.Shutdown(ctx)` with a 5s grace.
- `backend-go/Makefile` with targets: `run` (`go run ./cmd/server`), `build`, `tidy`, `vet`, `test`. Header comment notes that we deviate from the go-backend skill's tools-dir convention because there is no DB or codegen.
- `backend-go/.gitignore` (binary output dir if any).

## Notes for implementer
- Use `slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))` and set as default; level configurable later via `LOG_LEVEL`.
- Follow the explicit-error-assignment style: `err := srv.ListenAndServe(); if err != nil && !errors.Is(err, http.ErrServerClosed) { ... }` — never inline `if err := ...`.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 `make -C backend-go run` boots and logs `server starting on :8787`
- [x] #2 Sending SIGINT triggers a clean shutdown log line and the process exits 0
- [x] #3 `go vet ./...` is clean
- [x] #4 `golang.org/x/net/websocket` is in `go.mod`/`go.sum`
<!-- AC:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Bootstrap complete. Created `backend-go/go.mod`, `backend-go/cmd/server/main.go`, `backend-go/Makefile`, and `backend-go/.gitignore`. Server listens on :8787 (PORT env configurable), uses slog text handler, handles SIGINT/SIGTERM with 5s grace shutdown. `golang.org/x/net/websocket` dependency captured for future WS support.
<!-- SECTION:FINAL_SUMMARY:END -->
