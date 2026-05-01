import { Hono } from "hono";
import { serve } from "@hono/node-server";
import { serveStatic } from "@hono/node-server/serve-static";
import type { IncomingMessage } from "http";
import type { Duplex } from "stream";
import type { WebSocket } from "ws";
import { WebSocketServer } from "ws";
import { loadConfig, type Config } from "./config.js";
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

let config: Config;
try {
  config = loadConfig();
} catch (err) {
  console.error("Config load failed — set MISTRAL_API_KEY and ANTHROPIC_API_KEY in .env");
  console.error(err instanceof Error ? err.message : err);
  process.exit(1);
}
const port = config.PORT;
console.warn(
  `Config loaded: MISTRAL=${config.MISTRAL_API_KEY ? "set" : "missing"} ` +
  `ANTHROPIC=${config.ANTHROPIC_API_KEY ? "set" : "missing"}`
);

const server = serve({ fetch: app.fetch, port });

const wss = new WebSocketServer({ noServer: true });

server.on("upgrade", (request: IncomingMessage, socket: Duplex, head: Buffer) => {
  const url = new URL(request.url ?? "", `http://${request.headers.host ?? ""}`);
  if (url.pathname === "/api/ws") {
    console.warn("WebSocket upgrade");
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

  ws.on("message", (data: Buffer, isBinary: boolean) => {
    if (isBinary) {
      if (!session.utteranceOpen) {
        session.send({
          type: "error",
          message: "Unexpected audio frame: send start_utterance first",
        });
        return;
      }
      session.pushClientAudio(data);
      return;
    }
    let msg: ClientMsg;
    try {
      msg = JSON.parse(data.toString("utf8")) as ClientMsg;
    } catch {
      session.send({ type: "error", message: "Invalid message format" });
      return;
    }
     console.warn(`Session ${sessionId} <- ${msg.type}`);
     try {
       handleClientMessage(session, msg);
     } catch (err: unknown) {
       console.error(`Session ${sessionId} handler error for ${msg.type}:`, err);
       session.send({
         type: "error",
         message: err instanceof Error ? err.message : "Handler failed",
       });
     }
  });

   ws.on("close", (code: number, reason: Buffer) => {
     session.closeVoxtral();
     sessions.delete(sessionId);
     console.warn(`Session ${sessionId} disconnected: code ${String(code)}, reason ${String(reason)}`);
   });

   ws.on("error", (error: Error) => {
     session.closeVoxtral();
     sessions.delete(sessionId);
     console.warn(`Session ${sessionId} error: ${error}`);
   });
});

function _describeUpstreamError(e: unknown): string {
   const raw = extractErrorText(e);
   const status = /\b(\d{3})\b/.exec(raw)?.[1];
   if (status === "401" || status === "403") return "authentication failed (check API key)";
   if (status === "429") return "rate limited";
   if (status && /^5\d\d$/.test(status)) return `upstream ${status}`;
   return "service error";
 }
 
function extractErrorText(e: unknown): string {
  if (e instanceof Error) return e.message;
  if (typeof e === "string") return e;
  if (e && typeof e === "object") {
    const o = e as Record<string, unknown>;
    const candidates = [
      o.message,
      o.error,
      o.reason,
      o.statusText,
      (o.body as Record<string, unknown> | undefined)?.message,
    ];
    for (const c of candidates) {
      if (typeof c === "string" && c.length > 0) return c;
    }
    const status = o.status ?? o.statusCode ?? o.code;
    if (typeof status === "number" || typeof status === "string") {
      return `status ${String(status)}`;
    }
    try {
      return JSON.stringify(e);
    } catch {
      return "unknown error";
    }
  }
  return String(e);
}

function handleClientMessage(session: Session, msg: ClientMsg) {
   if (msg.type === "start_session") {
     const scenario = skills.find((s) => s.id === msg.scenarioId);
     if (!scenario) {
       session.send({ type: "error", message: "Scenario not found" });
       return;
     }

     session.setScenario(scenario);
     session.clearTranscript();

     session.startVoxtralSTT({
       apiKey: config.MISTRAL_API_KEY,
       voiceId: scenario.voice,
     });

     session.send({ type: "session_ready", greeting: scenario.opening_line });

     session.sendToVoxtral(scenario.opening_line).catch((e: unknown) => {
       console.error("TTS error:", e);
     });
   }

  if (msg.type === "start_utterance") {
    if (session.utteranceOpen) {
      session.send({ type: "error", message: "Utterance already in progress" });
      return;
    }
    session.utteranceOpen = true;
    return;
  }

  if (msg.type === "end_utterance") {
    if (!session.utteranceOpen) {
      session.send({ type: "error", message: "No utterance in progress" });
      return;
    }
    session.utteranceOpen = false;
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