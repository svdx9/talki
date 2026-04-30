import type { Skill } from "./skills/schema.js";
import { WebSocket } from "ws";
import { DeepgramClient } from "@deepgram/sdk";
import type { Config } from "./config.js";

type DgSocket = Awaited<ReturnType<DeepgramClient["listen"]["v1"]["connect"]>>;

export class Session {
  id: string;
  ws: WebSocket;
  scenario: Skill | null = null;
  transcript: string = "";
  createdAt: Date;
  private dgSocket: DgSocket | null = null;

  constructor(id: string, ws: WebSocket) {
    this.id = id;
    this.ws = ws;
    this.createdAt = new Date();
  }

  setScenario(scenario: Skill) {
    this.scenario = scenario;
  }

  appendTranscript(text: string) {
    this.transcript += text;
  }

  clearTranscript() {
    this.transcript = "";
  }

  send(message: unknown) {
    if (this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(message));
    }
  }

  async connectDeepgram(config: Config): Promise<void> {
    const dg = new DeepgramClient();
    this.dgSocket = await dg.listen.v1.connect({
      model: "nova-3",
      language: "fr",
      interim_results: "true",
      Authorization: `Token ${config.DEEPGRAM_API_KEY}`,
    });

    this.dgSocket.on("message", (msg) => {
      if ((msg as { type?: string }).type !== "Results") return;
      const results = msg as {
        channel?: { alternatives?: Array<{ transcript?: string }> };
        is_final?: boolean;
      };
      const text = results.channel?.alternatives?.[0]?.transcript;
      if (!text) return;
      const isFinal = !!results.is_final;
      this.send({ type: "transcript", text, isFinal });
      if (isFinal) this.appendTranscript(text + " ");
    });

    this.dgSocket.on("error", (err) => {
      console.error(`Deepgram error for session ${this.id}:`, err);
    });

    this.dgSocket.connect();
    await this.dgSocket.waitForOpen();
    console.log(`Deepgram connected for session ${this.id}`);
  }

  pushClientAudio(data: Buffer | ArrayBuffer) {
    this.dgSocket?.sendMedia(data as Buffer | ArrayBuffer);
  }

  endUtterance() {
    this.dgSocket?.sendCloseStream({ type: "CloseStream" });
  }

  closeDeepgram() {
    this.dgSocket?.close();
    this.dgSocket = null;
  }
}
