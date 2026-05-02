---
id: TASK-11.3
title: Go port — skills loader + YAML→JSON catalog conversion
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
ordinal: 4000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Port the skill catalog and loader. Stdlib has no YAML, so we convert the existing YAML files to JSON once. The originals stay until cutover (TASK-22).

## What to build
- For each file in `backend/src/skills/catalog/*.yaml`, produce `backend-go/cmd/server/internal/skills/catalog/<id>.json`. Use any tool you like for the one-time conversion (`yq -o=json`, a small script). The JSON files are committed.
- `backend-go/cmd/server/internal/skills/skill.go`:
  - `type Skill struct { ... }` mirroring `backend/src/skills/schema.ts`. JSON tags should match the TS keys (snake_case where applicable).
  - `func Load(dir string) ([]Skill, error)` walks `dir`, reads each `*.json`, decodes with `json.NewDecoder(...).DisallowUnknownFields()`, returns the slice. On any decode failure, return a wrapped error including the file path.
- `main.go` calls `skills.Load("./cmd/server/internal/skills/catalog")` and logs `loaded N skills: <id list>` — parity with the current TS startup log.
- Unit tests: a tiny fixture catalog under `internal/skills/testdata/`, plus a test that loads the real catalog from the repo path and asserts non-empty.

## Notes
- Field names to mirror precisely: `id`, `title`, `level`, `voice`, `opening_line`, `system_prompt`, plus any others in `schema.ts`. Read the TS schema carefully — don't rely on the YAML alone, since some optional fields may not appear there.
- Do NOT add `gopkg.in/yaml.v3` or any other YAML lib.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Each `.yaml` file in `backend/src/skills/catalog/` has a matching `.json` file under `backend-go/cmd/server/internal/skills/catalog/`
- [ ] #2 `skills.Load` returns the same number of skills as `loadSkills()` in the TS backend, with identical IDs
- [ ] #3 Server logs the same `loaded N skills:` line as the TS server
- [ ] #4 Unit tests run on a small testdata fixture AND smoke-load the real catalog
- [ ] #5 No YAML dependency in `go.mod`
<!-- AC:END -->
