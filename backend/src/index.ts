import { Hono } from "hono";
import { createServer } from "http";
import { serveStatic } from "@hono/node-server/serve-static";
import { WebSocketServer, WebSocket } from "ws";
import { loadConfig } from "./config.js";
import { loadSkills, type Skill as SkillType } from "./skills/loader.js";
import { Session } from "./session.js";
import type { ClientMsg, ServerMsg } from "talki-shared";

const skills = loadSkills();
console.log(`Loaded ${skills.length} skill(s): ${skills.map((s) => s.id).join(", ")}`);

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

const server = createServer((req, res) => {
  app.fetch(req as any, res as any);
});

const wss = new WebSocketServer({ noServer: true });

server.on("upgrade", (request, socket, head) => {
  const url = new URL(request.url || "", `http://${request.headers.host}`);
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

  console.log(`Session ${sessionId} connected`);

  ws.on("message", async (data) => {
    if (typeof data === "string") {
      try {
        const msg: ClientMsg = JSON.parse(data);
        await handleClientMessage(session, msg);
      } catch (e) {
        session.send({ type: "error", message: "Invalid message format" });
      }
    } else if (data instanceof Buffer || data instanceof ArrayBuffer) {
      session.pushClientAudio(data);
    }
  });

  ws.on("close", () => {
    session.closeDeepgram();
    sessions.delete(sessionId);
    console.log(`Session ${sessionId} disconnected`);
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
    } catch (e) {
      console.error(`Deepgram connection error for session ${session.id}:`, e);
      session.send({ type: "error", message: "Failed to connect to speech service" });
      return;
    }

    try {
      await session.initElevenLabs({
        apiKey: config.ELEVENLABS_API_KEY,
        voiceId: scenario.voice,
      });
    } catch (e) {
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
      session.streamAssistant(userText).catch((err) => {
        console.error(`Assistant stream error for session ${session.id}:`, err);
      });
    }
  }
}

console.log(`Server starting on port ${port}...`);
server.listen(port);
