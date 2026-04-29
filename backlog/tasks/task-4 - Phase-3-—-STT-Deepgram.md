---
id: TASK-4
title: Phase 3 — STT (Deepgram)
status: To Do
assignee: []
created_date: '2026-04-29 17:25'
labels:
  - phase-3
  - stt
  - deepgram
dependencies: []
priority: high
---

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Deepgram streaming client connects with model=nova-3, language=fr, encoding=opus, interim_results=true
- [ ] #2 Browser binary frames forwarded into Deepgram ws
- [ ] #3 Interim and final transcripts emitted to FE as {type:transcript}
- [ ] #4 end_utterance closes Deepgram input and resolves final transcript
<!-- AC:END -->
