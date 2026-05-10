# Talki

Real-time language learning platform with AI conversation partners.

## Structure

- `backend/` - Go HTTP server with WebSocket support
- `frontend/` - SolidJS + TypeScript + Vite client
- `shared/` - Common types and protocols

## Stack

- **Backend**: Go 1.25 + chi router
- **Frontend**: SolidJS + TypeScript + Vite
- **STT**: Deepgram Nova-3
- **LLM**: Anthropic Claude Sonnet 4.6
- **TTS**: ElevenLabs Flash v2.5

## Setup

```bash
pnpm install                   # Install frontend dependencies
make -C backend run            # Start Go backend (http://localhost:8787)
pnpm --filter frontend dev     # Start frontend dev server (http://localhost:5173)
```

## Development

- Backend (Go): runs on `:8787`, serves static frontend assets
- Frontend: dev server on `:5173` with WebSocket proxy to backend
- See `INSTRUCTIONS.md` for detailed setup and architecture information
