---
id: TASK-10
title: Replace Deepgram/ElevenLabs with Mistral Voxtral for STT/TTS
status: Done
assignee: []
created_date: '2026-05-01 15:49'
updated_date: '2026-05-01 16:01'
labels:
  - audio
  - backend
  - frontend
  - refactor
dependencies: []
references:
  - voxtral.examples.md
  - backend/src/session.ts
  - backend/src/elevenlabs.ts
  - backend/src/config.ts
  - backend/src/index.ts
  - frontend/src/App.tsx
  - >-
    https://docs.mistral.ai/studio-api/audio/speech_to_text/realtime_transcription
  - 'https://docs.mistral.ai/studio-api/audio/text_to_speech/speech'
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Remove Deepgram (STT) and ElevenLabs (TTS) from the backend and replace both with Mistral's Voxtral API using the `@mistralai/mistralai` TypeScript SDK. A single `MISTRAL_API_KEY` replaces both `DEEPGRAM_API_KEY` and `ELEVENLABS_API_KEY`.

## Current Architecture

**STT (Deepgram)**
- `backend/src/session.ts` — `connectDeepgram()`, `pushClientAudio()`, `endUtterance()`, `closeDeepgram()`
- `@deepgram/sdk` npm package, model `nova-3`, language `fr`, interim results enabled
- Frontend sends `audio/webm;codecs=opus` via `MediaRecorder` at 250ms chunks

**TTS (ElevenLabs)**
- `backend/src/elevenlabs.ts` — custom WebSocket client against `wss://api.elevenlabs.io/v1/text-to-speech/{voiceId}/stream-input`
- `backend/src/session.ts` — `initElevenLabs()`, `sendToElevenLabs()`, `flushElevenLabs()`, `closeElevenLabs()`
- Returns MP3 audio; played in browser via `MediaSource` + `SourceBuffer` in `frontend/src/opus-player.ts`

## Target Architecture

**STT (Voxtral Realtime)**
- SDK: `@mistralai/mistralai/extra/realtime` → `RealtimeTranscription`
- Model: `voxtral-mini-transcribe-realtime-2602`
- Audio format: `pcm_s16le` @ 16 kHz (see critical note below)
- Events: `transcription.text.delta`, `transcription.done`, `error`

**TTS (Voxtral TTS)**
- SDK: `@mistralai/mistralai` → `client.audio.speech` (streaming variant for lowest latency)
- Model: `voxtral-mini-tts-2603`
- Output format: `pcm` or `mp3` (pcm recommended for streaming; mp3 compatible with existing `OpusPlayer`)

## Critical: Audio Format Mismatch

The frontend currently sends `audio/webm;codecs=opus` frames. Voxtral STT requires raw `pcm_s16le` at 16 kHz. Two viable approaches:

**Option A — Transcode on the backend (lower frontend change)**
Pipe incoming webm/opus WebSocket frames through an `ffmpeg` child process (or use the `opusscript` package already in `backend/package.json` to decode Opus, then resample to 16 kHz PCM) before forwarding to `RealtimeTranscription`.

**Option B — Capture raw PCM in the frontend (cleaner, no backend transcoding)**
Replace `MediaRecorder` in `frontend/src/App.tsx` with a `Web Audio API` + `AudioWorklet` pipeline that captures 16-bit mono PCM at 16 kHz and sends raw binary frames over the WebSocket. Eliminates the transcoding step entirely.

Recommend Option B for latency and simplicity, but either works.

## Files to Change

| File | Change |
|---|---|
| `backend/src/elevenlabs.ts` | **Delete** entirely |
| `backend/src/session.ts` | Rewrite STT/TTS methods; replace Deepgram + ElevenLabs with Voxtral equivalents |
| `backend/src/config.ts` | Replace `DEEPGRAM_API_KEY` + `ELEVENLABS_API_KEY` with `MISTRAL_API_KEY` (+ optional `MISTRAL_VOICE_ID`) |
| `backend/src/index.ts` | Update config references; replace `connectDeepgram`/`initElevenLabs` call-sites |
| `backend/package.json` | Remove `@deepgram/sdk`; add `@mistralai/mistralai` (if not already present) |
| `frontend/src/App.tsx` | If Option B: replace `MediaRecorder` with AudioWorklet PCM capture |
| `.env.example` | Replace `DEEPGRAM_API_KEY`/`ELEVENLABS_API_KEY` with `MISTRAL_API_KEY` |

## SDK Usage Reference (TypeScript)

**STT — from `voxtral.examples.md` in repo root:**
```ts
import { RealtimeTranscription, AudioEncoding } from "@mistralai/mistralai/extra/realtime";

const client = new RealtimeTranscription({ apiKey });
for await (const event of client.transcribeStream(audioStream, model, { audioFormat })) {
  if (event.type === "transcription.text.delta") { /* partial */ }
  if (event.type === "transcription.done") { /* final */ }
}
```
`audioStream` is `AsyncGenerator<Uint8Array>` of `pcm_s16le` chunks.

**TTS — streaming (PCM recommended for lowest latency):**
```ts
import Mistral from "@mistralai/mistralai";
const mistral = new Mistral({ apiKey });
const stream = await mistral.audio.speech.stream({
  model: "voxtral-mini-tts-2603",
  input: text,
  voice_id: voiceId,
  response_format: "pcm", // or "mp3"
});
for await (const chunk of stream) { ws.send(chunk); }
```

## Backlog Tasks Being Superseded
- Task 4 (Phase 3 — STT Deepgram) and Task 6 (Phase 5 — TTS ElevenLabs) are replaced by this task.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Remove `@deepgram/sdk` from `backend/package.json` and delete `backend/src/elevenlabs.ts`
- [ ] #2 Config (`backend/src/config.ts`) accepts `MISTRAL_API_KEY` (and optionally `MISTRAL_VOICE_ID`) instead of `DEEPGRAM_API_KEY` / `ELEVENLABS_API_KEY`
- [ ] #3 STT: `session.ts` uses `RealtimeTranscription` from `@mistralai/mistralai/extra/realtime`; `transcription.text.delta` events are forwarded to the frontend as `{ type: 'transcript' }` messages
- [ ] #4 STT: audio arriving at the backend is in `pcm_s16le` format at 16 kHz before being passed to Voxtral (either via frontend PCM capture or backend transcoding)
- [ ] #5 TTS: `session.ts` streams Voxtral TTS audio chunks directly to the client WebSocket as binary frames, replacing the ElevenLabs pipeline
- [ ] #6 End-to-end voice conversation works: mic → Voxtral STT → Anthropic LLM → Voxtral TTS → browser audio playback
- [ ] #7 `.env.example` updated; no references to Deepgram or ElevenLabs remain in source or config files
- [ ] #8 No TypeScript compilation errors (`pnpm --filter backend tsc --noEmit`)
<!-- AC:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
**Audio capture approach: Option B — Frontend AudioWorklet PCM**

Replace `MediaRecorder` in `frontend/src/App.tsx` with a `Web Audio API` + `AudioWorklet` pipeline that captures raw `pcm_s16le` at 16 kHz mono and sends binary frames directly over the WebSocket. No backend transcoding needed.

Implementation sketch:
1. Create `frontend/src/pcm-processor.worklet.ts` (or `.js`) — an `AudioWorkletProcessor` that receives `Float32Array` samples from the audio graph, converts to `Int16Array` (multiply by 32767, clamp), and `postMessage`s the buffer back to the main thread.
2. In `App.tsx`: `getUserMedia` → `AudioContext` (sampleRate 16000) → `createMediaStreamSource` → `audioWorklet.addModule(...)` → `AudioWorkletNode` → on `message`, send raw `ArrayBuffer` over the WebSocket.
3. Remove `MediaRecorder` usage entirely; remove `opusscript` from `backend/package.json` (no longer needed).
4. The `end_utterance` signal stays the same — user stops recording, frontend sends the control message as before.

Backend receives raw `pcm_s16le` binary frames and passes them straight to `RealtimeTranscription` as `Uint8Array` chunks — no format conversion needed.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
## Implementation Summary

### What was changed

**Frontend:**
- `frontend/src/pcm-processor.worklet.ts` — new AudioWorkletProcessor that converts `Float32Array` microphone samples → `Int16Array` PCM chunks and posts them back to the main thread
- `frontend/src/App.tsx` — replaced `MediaRecorder` (webm/opus) with `AudioContext` + `AudioWorkletNode` pipeline at 16 kHz mono PCM; binary frames sent directly over WebSocket as raw PCM
- `frontend/src/worklet.d.ts` — type declarations for `AudioWorkletProcessor`, `registerProcessor`, `AudioWorkletPort` globals

**Backend:**
- `backend/src/session.ts` — completely rewritten STT/TTS:
  - STT: `startVoxtralSTT()` uses `RealtimeTranscription` from `@mistralai/mistralai/extra/realtime` with a `ReadableStream` wrapper that queues incoming PCM frames and feeds them as `AsyncIterable<Uint8Array>` to `transcribeStream()`. Events forwarded to frontend as `{ type: 'transcript' }`
  - TTS: `sendToVoxtral()` uses direct HTTP `fetch` to `POST https://api.mistral.ai/v1/audio/speech` (streaming response) — audio chunks piped directly to client WebSocket as binary frames. SDK doesn't expose `audio.speech` yet, so REST API used instead
- `backend/src/config.ts` — replaced `DEEPGRAM_API_KEY` + `ELEVENLABS_API_KEY` with `MISTRAL_API_KEY` + `MISTRAL_VOICE_ID` (optional, defaults to `female-1`)
- `backend/src/index.ts` — replaced `connectDeepgram`/`initElevenLabs`/`closeDeepgram` call-sites with `startVoxtralSTT`/`closeVoxtral`
- `backend/package.json` — removed `@deepgram/sdk` and `opusscript`, added `@mistralai/mistralai`
- `backend/src/elevenlabs.ts` — deleted
- `.env.example` — updated to reference `MISTRAL_API_KEY`

### Key design decisions
- **Option B (frontend PCM capture)** chosen: `MediaRecorder` → `AudioWorklet` at 16 kHz mono `pcm_s16le`; no backend transcoding needed
- **TTS via HTTP fetch** rather than SDK: `@mistralai/mistralai` 1.15.1 only exposes `audio.transcriptions`, not `audio.speech`. Direct REST calls to `api.mistral.ai/v1/audio/speech` with streaming response body achieve the same result
- Module augmentation for `@mistralai/mistralai/extra/realtime` uses `// @ts-ignore` + local type alias to work around `Node16` strict package exports resolution

### TypeScript
Both `backend` and `frontend` packages pass `tsc --noEmit` with no errors.
<!-- SECTION:FINAL_SUMMARY:END -->
