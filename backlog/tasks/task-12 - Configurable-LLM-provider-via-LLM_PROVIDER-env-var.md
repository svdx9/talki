---
id: TASK-12
title: Configurable LLM provider via LLM_PROVIDER env var
status: To Do
assignee: []
created_date: '2026-05-10 16:00'
labels:
  - backend
  - config
  - llm
dependencies: []
priority: medium
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Allow the LLM provider to be swapped via environment configuration rather than being hardcoded to Anthropic.

## Design

A `LLM_PROVIDER` env var selects the provider. Each provider has a hardcoded base URL and requires its own API key env var.

| `LLM_PROVIDER` | Base URL | API key env var |
|---|---|---|
| `anthropic` | `https://api.anthropic.com/v1/messages` | `ANTHROPIC_API_KEY` |
| `opencode` | `https://opencode.ai/zen/v1/messages` | `OPENCODE_API_KEY` |

`LLM_MODEL` is optional (default: `claude-sonnet-4-6`).

## Changes required

- `backend/internal/config/config.go` — add `LLMProvider`, `LLMAPIKey`, `LLMBaseURL`, `LLMModel` fields; validate provider; load provider-specific key
- `backend/internal/llm/anthropic.go` — expose `baseURL` in `NewClient(apiKey, baseURL string, hc *http.Client)`
- `backend/internal/session/session.go` — wire `cfg.LLMAPIKey`, `cfg.LLMBaseURL`, `cfg.LLMModel` through
- `.env.example` — document new env vars
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 LLM_PROVIDER=anthropic with ANTHROPIC_API_KEY works as before
- [ ] #2 LLM_PROVIDER=opencode with OPENCODE_API_KEY routes requests to opencode.ai/zen/v1/messages
- [ ] #3 Missing or invalid LLM_PROVIDER produces a clear startup error
- [ ] #4 LLM_MODEL overrides the default model name
- [ ] #5 All Go lint and tests pass
<!-- AC:END -->
