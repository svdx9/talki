---
id: TASK-3
title: Phase 2 — WebSocket plumbing
status: To Do
assignee: []
created_date: '2026-04-29 17:25'
labels:
  - phase-2
  - websocket
dependencies: []
priority: high
---

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 ws server attached to Hono HTTP server on /api/ws
- [ ] #2 Session class created per connection holding scenario, transcript, sub-clients
- [ ] #3 start_session handler loads scenario, replies session_ready, plays opening_line via TTS
- [ ] #4 End-to-end: open browser, pick scenario, hear greeting (validates WS+TTS before STT/LLM)
<!-- AC:END -->
