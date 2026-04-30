---
id: TASK-5
title: Phase 4 — LLM (Anthropic Claude)
status: Done
assignee: []
created_date: '2026-04-29 17:25'
updated_date: '2026-04-30 15:38'
labels:
  - phase-4
  - llm
  - anthropic
dependencies: []
priority: high
---

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Anthropic streaming client uses claude-sonnet-4-6 via messages.stream
- [ ] #2 System prompt from scenario YAML carries cache_control for prompt caching
- [ ] #3 Per-session transcript appends user + assistant turns
- [ ] #4 assistant_text_delta and assistant_done messages stream to FE correctly
<!-- AC:END -->
