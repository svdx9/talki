import type { Skill } from "./skills/schema.js";
import { WebSocket } from "ws";
import Anthropic from "@anthropic-ai/sdk";
import { connectRealtimeStt, AudioEncoding, type AudioFormat, type RealtimeStt } from "./integrations/mistral-realtime.js";

const MISTRAL_TTS_URL = "https://api.mistral.ai/v1/audio/speech";

export class Session {
  id: string;
  ws: WebSocket;
  scenario: Skill | null = null;
  transcript = "";
  utteranceOpen = false;
  createdAt: Date;
  private anthropic: Anthropic | null = null;
  private stt: RealtimeStt | null = null;
  private sttBytesPushed = 0;
  private conversationHistory: { role: "user" | "assistant"; content: string }[] = [];
  private assistantAbort: AbortController | null = null;

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

  startVoxtralSTT(config: { apiKey: string; voiceId: string; sampleRate: number }): void {
    console.warn(`Session ${this.id}: starting Voxtral STT at ${String(config.sampleRate)} Hz...`);
    const audioFormat: AudioFormat = {
      encoding: AudioEncoding.PcmS16le,
      sampleRate: config.sampleRate,
    };

    void (async () => {
      try {
        const stt = await connectRealtimeStt(config.apiKey, "voxtral-mini-transcribe-realtime-2602", audioFormat);
        this.stt = stt;
        console.warn(`Session ${this.id}: Voxtral STT connected`);

        for await (const event of stt.events()) {
          console.warn(`Session ${this.id}: STT event ${event.type}`);
          if (event.type === "transcription.text.delta") {
            const text = (event as { text?: string }).text ?? "";
            console.warn(`Session ${this.id}: transcript delta ${JSON.stringify(text)}`);
            this.appendTranscript(text);
            this.send({ type: "transcript", text, isFinal: false });
          } else if (event.type === "transcription.segment.delta") {
            const text = (event as { text?: string }).text ?? "";
            if (text) {
              console.warn(`Session ${this.id}: transcript segment ${JSON.stringify(text)}`);
              this.appendTranscript(text);
              this.send({ type: "transcript", text, isFinal: false });
            }
          } else if (event.type === "transcription.done") {
            console.warn(`Session ${this.id}: transcript final ${JSON.stringify(this.transcript)}`);
            this.send({ type: "transcript", text: "", isFinal: true });
          } else if (event.type === "error") {
            const errMsg = (event as { error?: { message?: unknown } }).error?.message;
            const msg = typeof errMsg === "string" ? errMsg : JSON.stringify(errMsg);
            // Voxtral emits a spurious "Cannot flush audio before sending any audio bytes"
            // event right after session start; keep the connection alive when it appears.
            if (msg.includes("Cannot flush audio before sending any audio bytes")) {
              console.warn(`Session ${this.id}: Voxtral STT spurious flush ignored`);
              continue;
            }
            console.error(`Session ${this.id}: Voxtral STT error — ${msg}`);
            this.send({ type: "error", message: `STT error: ${msg}` });
          }
          if (stt.isClosed) break;
        }
      } catch (err: unknown) {
        console.error(`Session ${this.id}: Voxtral STT stream error:`, err);
        this.send({ type: "error", message: "STT stream error" });
      }
    })();
  }

  private utterancePeak = 0;

  pushClientAudio(data: Buffer | ArrayBuffer): void {
    if (!this.stt || this.stt.isClosed) return;
    const uint8 = data instanceof Buffer
      ? new Uint8Array(data.buffer, data.byteOffset, data.byteLength)
      : new Uint8Array(data);
    if (uint8.byteLength === 0) return;
    const samples = new Int16Array(uint8.buffer, uint8.byteOffset, Math.floor(uint8.byteLength / 2));
    let peak = 0;
    for (const s of samples) {
      const v = Math.abs(s);
      if (v > peak) peak = v;
    }
    if (peak > this.utterancePeak) this.utterancePeak = peak;
    this.sttBytesPushed += uint8.byteLength;
    this.stt.sendAudio(uint8).catch((err: unknown) => {
      console.error(`Session ${this.id}: STT sendAudio error:`, err);
    });
  }

  endUtterance(): void {
    if (!this.stt || this.stt.isClosed) return;
    if (this.sttBytesPushed === 0) {
      console.warn(`Session ${this.id}: end_utterance with no audio — skipping flush`);
      return;
    }
    const peakDb = this.utterancePeak > 0 ? 20 * Math.log10(this.utterancePeak / 32768) : -Infinity;
    console.warn(`Session ${this.id}: utterance peak=${String(this.utterancePeak)} (${peakDb.toFixed(1)} dBFS), bytes=${String(this.sttBytesPushed)}`);
    this.sttBytesPushed = 0;
    this.utterancePeak = 0;
    this.stt.flushAudio().catch((err: unknown) => {
      console.error(`Session ${this.id}: STT flushAudio error:`, err);
    });
  }

  closeVoxtral(): void {
    const stt = this.stt;
    this.stt = null;
    this.sttBytesPushed = 0;
    if (!stt) return;
    stt.endAudio()
      .catch(() => undefined)
      .finally(() => {
        stt.close().catch(() => undefined);
      });
  }

  async sendToVoxtral(text: string): Promise<void> {
    if (!this.scenario) return;
    const apiKey = process.env.MISTRAL_API_KEY;
    if (!apiKey) return;

    try {
      const response = await fetch(MISTRAL_TTS_URL, {
        method: "POST",
        headers: {
          "Authorization": `Bearer ${apiKey}`,
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          model: "voxtral-mini-tts-2603",
          input: text,
          voice_id: this.scenario.voice,
          response_format: "mp3",
        }),
      });

      if (!response.ok) {
        const body = await response.text();
        console.error(`Session ${this.id}: Voxtral TTS HTTP ${String(response.status)}: ${body}`);
        return;
      }

      const body = response.body;
      if (!body) return;

      const reader = body.getReader();
      try {
        let chunk = await reader.read();
        while (!chunk.done) {
          // eslint-disable-next-line @typescript-eslint/no-unnecessary-condition
          if (chunk.value) {
            this.ws.send(chunk.value);
          }
          chunk = await reader.read();
        }
      } finally {
        reader.releaseLock();
      }
    } catch (err: unknown) {
      console.error(`Session ${this.id}: Voxtral TTS error:`, err);
    }
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

    this.assistantAbort?.abort();
    const abort = new AbortController();
    this.assistantAbort = abort;

    const stream = this.anthropic.messages.stream(
      {
        model: "claude-sonnet-4-6",
        max_tokens: 1024,
        system: systemPrompt,
        messages: this.conversationHistory.map(({ role, content }) => ({
          role,
          content,
        })),
      },
      { signal: abort.signal },
    );

    let fullText = "";

    stream.on("text", (delta) => {
      this.send({ type: "assistant_text_delta", text: delta });
      fullText += delta;
      this.sendToVoxtral(delta).catch((err: unknown) => {
        console.error(`Session ${this.id}: TTS error:`, err);
      });
    });

    try {
      const finalMessage = await stream.finalMessage();
      const textContent = finalMessage.content.find((b) => b.type === "text");
      if (textContent?.type === "text") {
        this.conversationHistory.push({ role: "assistant", content: textContent.text });
      }
      this.send({ type: "assistant_done", fullText });
    } catch (err) {
      if (abort.signal.aborted) {
        console.warn(`Session ${this.id}: assistant stream cancelled`);
      } else {
        console.error(`Anthropic stream error for session ${this.id}:`, err);
      }
    } finally {
      if (this.assistantAbort === abort) this.assistantAbort = null;
    }
  }

  cancelAssistant(): void {
    this.assistantAbort?.abort();
    this.assistantAbort = null;
  }
}
