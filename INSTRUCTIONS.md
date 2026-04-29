# Talki — Conversational French Tutor (POC)

A browser-based AI tutor for practising spoken French. The learner picks a scenario (e.g. "ordering coffee at a Lyon café, A2 level"), holds a push-to-talk button, and converses with an LLM tutor that listens, replies in voice, and stays in character at the configured CEFR level.

## What it does

1. Learner selects a scenario from the catalog.
2. Holds the **speak** button. Microphone audio streams as opus chunks over WebSocket to the backend, which forwards it to **Deepgram Nova-3** for streaming French speech-to-text.
3. On release, the final transcript goes to **Anthropic Claude Sonnet 4.6** with the scenario's system prompt + conversation history.
4. Claude's text reply streams back to the chat window and is simultaneously piped through **ElevenLabs Flash v2.5** for streaming text-to-speech.
5. The tutor's voice plays in the browser as the audio chunks arrive.

## Stack

| Layer | Tech |
|---|---|
| STT (streaming) | Deepgram Nova-3 (`nova-3`, French, opus) |
| LLM | Anthropic Claude Sonnet 4.6 (`claude-sonnet-4-6`) with prompt caching |
| TTS (streaming) | ElevenLabs Flash v2.5 (opus output) |
| Backend | Node.js 22 + TypeScript + Hono + `ws` |
| Frontend | SolidJS + TypeScript + Vite, MediaRecorder for opus capture |
| Skills/scenarios | YAML files validated with Zod |
| Monorepo | pnpm workspaces (`backend/`, `frontend/`, `shared/`) |

## Repository layout

```
talki/
├── backend/          # Hono server, WS gateway, STT/LLM/TTS clients, skills loader
├── frontend/         # SolidJS app: scenario picker, push-to-talk, chat, audio playback
├── shared/           # WebSocket message types shared between FE and BE
├── INSTRUCTIONS.md   # this file
└── backlog/          # task management (managed by Backlog.md)
```

## Environment variables

Copy `.env.example` to `.env` (gitignored) and fill in:

| Var | Purpose |
|---|---|
| `DEEPGRAM_API_KEY` | Streaming speech-to-text |
| `ANTHROPIC_API_KEY` | Claude tutor responses |
| `ELEVENLABS_API_KEY` | Streaming text-to-speech |
| `PORT` | HTTP/WS port (default `8787`) |

## Running locally (once implemented)

```bash
pnpm install
pnpm --filter backend dev      # http://localhost:8787
pnpm --filter frontend dev     # http://localhost:5173 (Vite, proxies /api → backend)
```

In production, the frontend builds to static assets that the backend serves directly:

```bash
pnpm --filter frontend build
pnpm --filter backend start
```

## WebSocket protocol summary

- **Text frames (JSON)**: control + transcript + assistant text deltas
- **Binary frames**: opus audio (mic → server, TTS → client)

Full message shapes live in `shared/src/protocol.ts`.

## Skills file format

One YAML file per scenario in `backend/src/skills/catalog/`. Each defines: `id`, `title`, CEFR `level`, `voice`, `persona`, `constraints`, `opening_line`, and the full `system_prompt`. Loaded and validated at backend startup.

## Task tracking

Implementation tasks live in `backlog/` and are managed via the Backlog.md MCP server. Run `backlog list` to see open work.

## Reference

Full implementation plan: `~/.claude/plans/i-want-to-build-iterative-goose.md`
