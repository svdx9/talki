---
id: m-0
title: "Go backend port"
---

## Description

Port the existing Node/TypeScript backend to Go. All code lives under `backend-go/internal/` until cutover. WebSocket: `golang.org/x/net/websocket` only. Otherwise stdlib only — no chi/sqlc/pgx/oapi-codegen and no Anthropic/Mistral SDKs. Style follows the go-backend skill (explicit err assignment, no globals, constructors, slog).
