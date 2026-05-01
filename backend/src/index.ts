import { Hono } from "hono";
import { serve } from "@hono/node-server";
import { serveStatic } from "@hono/node-server/serve-static";
import type { IncomingMessage } from "http";
import type { Duplex } from "stream";
import type { WebSocket } from "ws";
import { WebSocketServer } from "ws";
import { loadConfig } from "./config.js";
import { loadSkills } from "./skills/loader.js";
import { Session } from "./session.js";
import type { ClientMsg } from "talki-shared";

const skills = loadSkills();
console.warn(`Loaded ${String(skills.length)} skill(s): ${skills.map((s) => s.id).join(", ")}`);

const app = new Hono();
const sessions = new Map<string, Session>();

app.get("/api/health", (c) => {
  return c.json({ status: "ok", timestamp: new Date().toISOString() });
});

app.get("/api/skills", (c) => {
  return c.json(skills.map(({ id, title, level }) => ({ id, title, level })));
});

app.use("/*", serveStatic({ root: "./frontend/dist" }));

const config = loadConfig();
const port = config.PORT;

const server = serve({ fetch: app.fetch, port });

const wss = new WebSocketServer({ noServer: true });

server.on("upgrade", (request: IncomingMessage, socket: Duplex, head: Buffer) => {
  const url = new URL(request.url ?? "", `http://${request.headers.host ?? ""}`);
  if (url.pathname === "/api/ws") {
    wss.handleUpgrade(request, socket, head, (ws) => {
      wss.emit("connection", ws, request);
    });
  } else {
    socket.destroy();
  }
});

wss.on("connection", (ws: WebSocket) => {
  const sessionId = Math.random().toString(36).substring(7);
  const session = new Session(sessionId, ws);
  sessions.set(sessionId, session);
  session.initAnthropic(config.ANTHROPIC_API_KEY);

  console.warn(`Session ${sessionId} connected`);

  ws.on("message", (data) => {
    if (typeof data === "string") {
      try {
        const msg = JSON.parse(data) as ClientMsg;
        handleClientMessage(session, msg).catch(() => {
          // ignore
        });
      } catch {
        session.send({ type: "error", message: "Invalid message format" });
      }
    } else if (data instanceof Buffer || data instanceof ArrayBuffer) {
      session.pushClientAudio(data);
    }
  });

  ws.on("close", (code: number, reason: Buffer) => {
    session.closeDeepgram().catch(() => {
      // ignore
    });
    sessions.delete(sessionId);
    console.warn(`Session ${sessionId} disconnected: code ${String(code)}, reason ${String(reason)}`);
  });

  ws.on("error", (error: Error) => {
    session.closeDeepgram().catch(() => {
      // ignore
    });
    sessions.delete(sessionId);
    console.warn(`Session ${sessionId} error: ${error}`);
  });
});

async function handleClientMessage(session: Session, msg: ClientMsg) {
  if (msg.type === "start_session") {
    const scenario = skills.find((s) => s.id === msg.scenarioId);
    if (!scenario) {
      session.send({ type: "error", message: "Scenario not found" });
      return;
    }

    session.setScenario(scenario);
    session.clearTranscript();

    try {
      await session.connectDeepgram(config);
    } catch (e: unknown) {
      console.error(`Deepgram connection error for session ${session.id}:`, e);
      session.send({ type: "error", message: "Failed to connect to speech service" });
      return;
    }

    try {
      await session.initElevenLabs({
        apiKey: config.ELEVENLABS_API_KEY,
        voiceId: scenario.voice,
      });
    } catch (e: unknown) {
      console.error(`ElevenLabs connection error for session ${session.id}:`, e);
      session.send({ type: "error", message: "Failed to connect to voice service" });
      return;
    }

    session.send({ type: "session_ready", greeting: scenario.opening_line });

    // Stream TTS for opening line (connection is now open)
    session.sendToElevenLabs(scenario.opening_line);
    session.flushElevenLabs();
  }

  if (msg.type === "end_utterance") {
    session.endUtterance();
    const userText = session.transcript.trim();
    session.clearTranscript();
    if (userText) {
      void session.streamAssistant(userText).catch((err: unknown) => {
        console.error(`Assistant stream error for session ${session.id}:`, err);
      });
    }
  }
}

console.warn(`Server starting on port ${String(port)}...`);
