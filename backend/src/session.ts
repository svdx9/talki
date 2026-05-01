import type { Skill } from "./skills/schema.js";
import { WebSocket } from "ws";
import { DeepgramClient } from "@deepgram/sdk";
import Anthropic from "@anthropic-ai/sdk";
import type { Config } from "./config.js";
import { ElevenLabsStream } from "./elevenlabs.js";

type DgSocket = Awaited<ReturnType<DeepgramClient["listen"]["v1"]["connect"]>>;

export class Session {
  id: string;
  ws: WebSocket;
  scenario: Skill | null = null;
  transcript = "";
  utteranceOpen = false;
  createdAt: Date;
  private dgSocket: DgSocket | null = null;
  private anthropic: Anthropic | null = null;
  private elevenLabsStream: ElevenLabsStream | null = null;
  private conversationHistory: { role: "user" | "assistant"; content: string }[] = [];

  constructor(id: string, ws: WebSocket) {
    this.id = id;
    this.ws = ws;
    this.createdAt = new Date();
  }

  setScenario(scenario: Skill) {
    this.scenario = scenario;
    this.conversationHistory = [];
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

  initAnthropic(apiKey: string) {
    this.anthropic = new Anthropic({ apiKey });
  }

  async initElevenLabs(config: { apiKey: string; voiceId: string }) {
    console.warn(`Session ${this.id}: connecting to ElevenLabs (voice=${config.voiceId})...`);
    this.elevenLabsStream = new ElevenLabsStream({
      apiKey: config.apiKey,
      voiceId: config.voiceId,
    });

    this.elevenLabsStream.on("audio", (audioBuffer: Buffer) => {
      if (this.ws.readyState === WebSocket.OPEN) {
        this.ws.send(audioBuffer);
      }
    });

    this.elevenLabsStream.on("error", (error) => {
      console.error(`ElevenLabs error for session ${this.id}:`, error);
    });

    await this.elevenLabsStream.connect();
    console.warn(`Session ${this.id}: ElevenLabs connected`);
  }

  sendToElevenLabs(text: string): void {
    if (this.elevenLabsStream && !this.elevenLabsStream.closing) {
      this.elevenLabsStream.sendSentence(text);
    }
  }

  flushElevenLabs(): void {
    this.elevenLabsStream?.flush();
  }

  async closeElevenLabs() {
    if (this.elevenLabsStream) {
      await this.elevenLabsStream.close();
      this.elevenLabsStream = null;
    }
  }

  async connectDeepgram(config: Config): Promise<void> {
    console.warn(`Session ${this.id}: connecting to Deepgram...`);
    const dg = new DeepgramClient({ apiKey: config.DEEPGRAM_API_KEY });
    this.dgSocket = await dg.listen.v1.connect({
      model: "nova-3",
      language: "fr",
      interim_results: "true",
      Authorization: `Token ${config.DEEPGRAM_API_KEY}`,
    });

    this.dgSocket.on("message", (msg) => {
      if ((msg as { type?: string }).type !== "Results") return;
      const results = msg as {
        channel?: { alternatives?: { transcript?: string }[] };
        is_final?: boolean;
      };
      const text = results.channel?.alternatives?.[0]?.transcript;
      if (!text) return;
      const isFinal = !!results.is_final;
      this.send({ type: "transcript", text, isFinal });
      if (isFinal) this.appendTranscript(text + " ");
    });

    this.dgSocket.on("error", (err) => {
      console.warn(`Session ${this.id}: Deepgram socket error: ${err instanceof Error ? err.message : String(err)}`);
    });

    this.dgSocket.connect();
    await this.dgSocket.waitForOpen();
    console.warn(`Deepgram connected for session ${this.id}`);
  }

  pushClientAudio(data: Buffer | ArrayBuffer) {
    this.dgSocket?.sendMedia(data);
  }

  endUtterance() {
    this.dgSocket?.sendCloseStream({ type: "CloseStream" });
  }

  async closeDeepgram() {
    this.dgSocket?.close();
    this.dgSocket = null;
    await this.closeElevenLabs();
  }

  async streamAssistant(userText: string): Promise<void> {
    if (!this.anthropic || !this.scenario) return;
    console.warn(`Session ${this.id}: streaming Anthropic reply (user=${JSON.stringify(userText.slice(0, 60))})`);

    this.conversationHistory.push({ role: "user", content: userText });

    const systemPrompt: Anthropic.TextBlockParam[] = [
      {
        type: "text",
        text: this.scenario.system_prompt,
        cache_control: { type: "ephemeral" },
      },
    ];

    const stream = this.anthropic.messages.stream({
      model: "claude-sonnet-4-6",
      max_tokens: 1024,
      system: systemPrompt,
      messages: this.conversationHistory.map(({ role, content }) => ({
        role,
        content,
      })),
    });

    let fullText = "";

    stream.on("text", (delta) => {
      this.send({ type: "assistant_text_delta", text: delta });
      fullText += delta;
      this.sendToElevenLabs(delta);
    });

    try {
      const finalMessage = await stream.finalMessage();
      const textContent = finalMessage.content.find((b) => b.type === "text");
      if (textContent?.type === "text") {
        this.conversationHistory.push({ role: "assistant", content: textContent.text });
      }
      this.flushElevenLabs();
      this.send({ type: "assistant_done", fullText });
    } catch (err) {
      console.error(`Anthropic stream error for session ${this.id}:`, err);
    }
  }
}
