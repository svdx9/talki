---
id: TASK-11.2
title: Go port — config loader + slog setup
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
ordinal: 3000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Single-source config loader and structured logger. Mirrors `backend/src/config.ts` and the partial slog setup from the previous task.

## Hard constraints
- All env-var reads MUST live in `backend-go/cmd/server/internal/config`. No `os.Getenv` outside this package.
- No dotenv loader. The dev workflow may export env vars manually or via `direnv` — that is the operator's concern, not the binary's.
- `Config` is constructed once in `main()` and passed down. No globals.

## Variables (parity with TS)
| Var | Required | Default | Notes |
| --- | --- | --- | --- |
| `MISTRAL_API_KEY` | yes | — | fail fast if missing |
| `ANTHROPIC_API_KEY` | yes | — | fail fast if missing |
| `MISTRAL_VOICE_ID` | no | `female-1` | |
| `PORT` | no | `8787` | parsed as int |
| `DEBUG_WS` | no | `false` | parses `1`, `true`, `yes` (case-insensitive) |
| `LOG_LEVEL` | no | `info` | maps to `slog.Level`: debug/info/warn/error |
| `ENV` | no | `dev` | reserved for future use |

## What to build
- `backend-go/cmd/server/internal/config/config.go` exporting:
  - `type Config struct { ... }`
  - `func Load() (Config, error)` — reads env, validates, returns wrapped errors via `fmt.Errorf("config: %s: %w", field, err)`.
  - sentinel errors `ErrMissingMistralKey`, `ErrMissingAnthropicKey` so tests can use `errors.Is`.
- `main.go` updated to call `config.Load`, build a slog handler at the configured level, log a redacted summary (`MISTRAL=set ANTHROPIC=set PORT=8787 DEBUG_WS=false`), and pass the `Config` into the (still-empty) server constructor.
- Unit tests in `config_test.go` covering: missing required key, invalid `PORT`, default values, `DEBUG_WS` parsing variants. Test cases as a slice of structs (not a map) per the go-backend skill.

## Out of scope
- No HTTP routes yet. No WS yet. No skills loading yet.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 `config.Load` returns a populated struct given the required env vars
- [ ] #2 Missing `MISTRAL_API_KEY` or `ANTHROPIC_API_KEY` produces an error matching the corresponding sentinel via `errors.Is`
- [ ] #3 Unit tests cover defaults, invalid PORT, DEBUG_WS truthy/falsy parsing
- [ ] #4 Startup logs the redacted summary (no API keys printed)
- [ ] #5 API keys are NEVER logged in any code path
<!-- AC:END -->
