---
id: TASK-11.12
title: 'Go port — cutover: replace TS backend, delete YAML, update root scripts'
status: To Do
assignee: []
created_date: '2026-05-02 15:58'
labels:
  - go
  - port
  - backend
  - cutover
milestone: m-0
dependencies: []
parent_task_id: TASK-11
priority: high
ordinal: 13000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Final task. Performed only after every other subtask is Done AND a full manual end-to-end smoke test against the Go backend has passed.

## Steps
1. Manual verification (do not skip):
   - Connect from the frontend.
   - Pick a scenario; hear greeting TTS.
   - Push to talk and say something; see streaming transcript and streaming assistant reply text + TTS.
   - Mid-reply, send `cancel`; verify assistant stops.
   - Send `end_session`; verify clean teardown.
   - Reload the page and repeat — second session works (regression for the multi-utterance bug we hit in TS).
2. Move the Go code into `backend/`:
   - `git rm -rf backend/`
   - `git mv backend-go backend`
   - Adjust paths inside the Go module that reference `../frontend/dist` if needed.
3. Delete the YAML skill catalog under `backend/src/skills/catalog/` (only the `.json` versions under `backend/cmd/server/internal/skills/catalog/` remain).
4. Update root `package.json` / pnpm workspace scripts so `pnpm dev` (or equivalent) starts the Go backend instead of the TS backend. The frontend dev script and shared package stay untouched.
5. Update `CLAUDE.md` and any top-level README to describe the new dev workflow (`make -C backend run` etc).
6. One commit per logical step where reasonable; never amend across the cutover boundary.

## Do NOT
- Force-push.
- Delete the TS backend until the Go backend is verified end-to-end on the user's machine.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Manual smoke test passes for: greeting TTS, push-to-talk transcript, streaming reply, cancel, end_session, reconnect
- [ ] #2 `backend/` now contains the Go server; the old TS backend is removed
- [ ] #3 YAML skill catalog is removed; only JSON remains
- [ ] #4 Root scripts run the Go backend as the default dev/prod path
- [ ] #5 CLAUDE.md / README updated
- [ ] #6 Commit history is reviewable: one logical commit per step (port move, YAML removal, scripts update, docs)
<!-- AC:END -->
