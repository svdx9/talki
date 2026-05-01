---
id: TASK-9
title: Adopt cast-iron type-aware ESLint across the workspace
status: Done
assignee: []
created_date: '2026-05-01 09:07'
updated_date: '2026-05-01 14:06'
labels:
  - tooling
  - quality
dependencies: []
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
## Why

A bug shipped in commit `1f8b3b4` (task-6) where `Session.closeDeepgram()` was promoted from sync to `async`, but its sole caller at `backend/src/index.ts:68` kept discarding the (now Promise) return value inside `ws.on("close", …)`. Result: the ElevenLabs WS could outlive the session by up to 5 s, and any rejection from the close path becomes an unhandled rejection. No tooling caught it.

This task installs a strict, type-aware ESLint setup at the repo root so floating promises, misused promise-returning callbacks, await-on-non-thenable, and unsafe-`any` flow fail the build (and pre-commit). One root flat config, three workspaces (`backend`, `frontend`, `shared`), no escape hatches.

## Approach

- ESLint v9 flat config at the repo root (`eslint.config.mjs`), using `typescript-eslint` with `projectService: true` so each workspace's `tsconfig.json` is auto-discovered.
- Layer: `tseslint.configs.strictTypeChecked` + `tseslint.configs.stylisticTypeChecked` + `eslint-plugin-solid` for the frontend.
- Bug-class rules pinned to `error`: `no-floating-promises` (`ignoreVoid: false`), `no-misused-promises` (full `checksVoidReturn`), `await-thenable`, `require-await`, `return-await`, `promise-function-async`.
- Unsafe-any rules pinned to `error`: `no-unsafe-{assignment,member-access,call,return,argument}`, `no-explicit-any`. Forces `JSON.parse` results to be narrowed (relevant in `backend/src/elevenlabs.ts:130-148`).
- Hygiene: `consistent-type-imports`, `no-unused-vars` with `_`-prefix escape, `no-console` warn (allow `warn`/`error`), `eqeqeq`.
- Pre-commit: `husky` + `lint-staged` running `eslint --max-warnings=0` on staged `*.{ts,tsx}`.
- Root `package.json` scripts: `lint`, `lint:fix`, `typecheck` (`pnpm -r exec tsc --noEmit`).
- First-run cleanup: fix everything the new rules surface in this same PR (the floating promise at `index.ts:68`, the indentation slip at `session.ts:130`, the `JSON.parse(any)` cascade in `elevenlabs.ts`, the floating `event.data.arrayBuffer().then(...)` in `frontend/src/App.tsx`, etc.). No `eslint-disable` allowlists.

## Out of Scope

- Prettier / formatting (separate concern).
- Migrating to a different test runner / CI provider.
- Other findings from the task-6 review unless they are surfaced by the new lint rules — track those separately if needed.

## References

- Plan file: `~/.claude/plans/review-of-commit-1f8b3b4968acfcf188c32fb-bubbly-pascal.md`
- Bug commit: `1f8b3b4968acfcf188c32fbfce0771cd43340388`
- Failing call site: `backend/src/index.ts:68`
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Root `eslint.config.mjs` exists using ESLint v9 flat config and `typescript-eslint` with `projectService: true`; lints `backend/`, `frontend/`, and `shared/` with no per-package config files.
- [x] #2 `@typescript-eslint/no-floating-promises` is `error` with `ignoreVoid: false`; running `pnpm lint` against the pre-fix `backend/src/index.ts:68` reproduces a floating-promise error.
- [x] #3 `@typescript-eslint/no-misused-promises` is `error` with all `checksVoidReturn` sub-options enabled.
- [x] #4 `await-thenable`, `require-await`, `return-await`, and `promise-function-async` are all `error`.
- [x] #5 `no-unsafe-{assignment,member-access,call,return,argument}` and `no-explicit-any` are all `error`.
- [x] #6 `eslint-plugin-solid` is applied to `frontend/**/*.{ts,tsx}` with browser globals; backend uses Node globals.
- [x] #7 Root `package.json` exposes `lint`, `lint:fix`, and `typecheck` scripts; `pnpm lint` exits 0 across the whole workspace after the cleanup pass.
- [x] #8 `husky` + `lint-staged` is wired so that committing a TS file with a lint error is blocked (`eslint --max-warnings=0` on staged `*.{ts,tsx}`).
- [x] #9 All lint errors surfaced on the first run are fixed in this PR with real fixes — no `eslint-disable` comments and no rule downgrades.
- [x] #10 `pnpm -r build` still passes after the changes.
- [x] #11 Manual regression check: temporarily drop the `.catch(...)` from `session.streamAssistant(userText).catch(...)` in `backend/src/index.ts` and confirm `pnpm lint` fails; revert before commit.
<!-- AC:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Strict type-aware ESLint adopted at the repo root via flat config (`eslint.config.mjs`) using `typescript-eslint` with `projectService: true`. All bug-class promise rules and unsafe-any rules pinned to `error`. `eslint-plugin-solid` applied to frontend; Node globals on backend. Husky + lint-staged block commits with lint errors. All initial lint findings fixed with real fixes (no eslint-disable, no rule downgrades). `pnpm lint`, `pnpm typecheck`, and `pnpm -r build` all pass. AC #11 manually verified: removing the `.catch(...)` from `session.streamAssistant(...)` reproduces a `no-floating-promises` error.
<!-- SECTION:FINAL_SUMMARY:END -->
