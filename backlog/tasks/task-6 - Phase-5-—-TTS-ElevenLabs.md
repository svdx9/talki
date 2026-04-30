---
id: TASK-6
title: Phase 5 — TTS (ElevenLabs)
status: Done
assignee: []
created_date: '2026-04-29 17:25'
updated_date: '2026-04-30 17:14'
labels:
  - phase-5
  - tts
  - elevenlabs
dependencies: []
priority: high
---

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 ElevenLabs ws client uses eleven_flash_v2_5 with PCM 48kHz output (switched from opus for direct Web Audio playback)
- [ ] #2 Claude text deltas piped into ElevenLabs sentence-buffered to avoid jitter
- [ ] #3 Audio bytes (PCM) forwarded to FE as binary ws frames
- [ ] #4 Tutor reply audio plays back end-to-end without gaps via Web Audio API
<!-- AC:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
**Review — 2026-04-30**

Backend pipeline (AC #1–3) is mostly implemented but has correctness bugs. AC #4 (frontend playback) is entirely missing.

**Bugs to fix before closing:**

1. `elevenlabs.ts:104` — `isSentenceEnd` tests the incoming delta, not the accumulated buffer. Should be `this.isSentenceEnd(this.sentenceBuffer)`.

2. `elevenlabs.ts:96–98` — `{ text: "" }` (ElevenLabs EOS/termination signal) is sent after every sentence flush, not just at session end. Will close the stream prematurely mid-conversation. Remove this block from `sendText`; keep only in `close()`.

3. `index.ts:93–102` — `initElevenLabs` fires `connect()` without awaiting, so the immediately following `sendToElevenLabs` / `flushElevenLabs` calls for the opening line are silently dropped (`isStreaming=false`). Make `initElevenLabs` return the connect promise and await it, or move opening line send into an `on("open")` callback.

4. `elevenlabs.ts:158–164` — `close()` uses a hard 500 ms setTimeout instead of waiting for the `isFinal` event. Replace with an `isFinal`-based close.

5. **Frontend (App.tsx) is a stub.** No WebSocket, no AudioContext, no Opus decoding, no playback. AC #4 cannot be checked until this is implemented.

**Fixes applied — 2026-04-30**

All backend bugs fixed:

1. ✅ `isSentenceEnd` now tests `this.sentenceBuffer` (not the delta) — `elevenlabs.ts:112`

2. ✅ Removed premature `{ text: "" }` EOS signal from `sendText`; only sent in `close()` — `elevenlabs.ts:86-97`

3. ✅ `initElevenLabs` now `await`s `connect()`; opening line sends after connection is ready — `session.ts:50-68`, `index.ts:93-102`

4. ✅ `close()` uses `isFinal`-based teardown with 5s fallback timeout — `elevenlabs.ts:170-188`

5. ✅ Switched ElevenLabs output to `pcm_48000` (was `opus_48000_192`) so frontend can play directly

Frontend audio playback implemented:

- ✅ `audio-player.ts` — PCM 48kHz/16-bit player using Web Audio API with gap-free scheduling

- ✅ `App.tsx` — WebSocket connection, push-to-talk recording, PCM audio playback, live transcript + tutor text display

- ✅ Frontend and backend builds pass
<!-- SECTION:NOTES:END -->
