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

  console.log(`Session ${sessionId} connected`);

  ws.on("message", async (data) => {
    if (typeof data === "string") {
      try {
        const msg: ClientMsg = JSON.parse(data);
        await handleClientMessage(session, msg);
      } catch (e) {
        session.send({ type: "error", message: "Invalid message format" });
      }
    }
  });

  ws.on("close", () => {
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
    session.send({ type: "session_ready", greeting: scenario.opening_line });

    // Generate TTS for opening line
    try {
      const audioBuffer = await generateTTS(scenario.opening_line, scenario.voice, config.ELEVENLABS_API_KEY);
      if (audioBuffer && session.ws.readyState === WebSocket.OPEN) {
        session.ws.send(audioBuffer);
      }
    } catch (e) {
      console.error(`TTS error for session ${session.id}:`, e);
    }
  }
}

async function generateTTS(text: string, voice: string, apiKey: string): Promise<Buffer | null> {
  const url = `https://api.elevenlabs.io/v1/text-to-speech/${voice}/stream?output_format=opus_48000_192`;

  try {
    const response = await fetch(url, {
      method: "POST",
      headers: {
        "xi-api-key": apiKey,
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        text,
        model_id: "eleven_flash_v2_5",
      }),
    });

    if (!response.ok) {
      console.error(`ElevenLabs API error: ${response.status} ${response.statusText}`);
      return null;
    }

    const arrayBuffer = await response.arrayBuffer();
    return Buffer.from(arrayBuffer);
  } catch (e) {
    console.error("TTS generation failed:", e);
    return null;
  }
}

console.log(`Server starting on port ${port}...`);
server.listen(port);
