import type { Skill } from "./skills/schema.js";
import { WebSocket } from "ws";
import Anthropic from "@anthropic-ai/sdk";
import { Mistral } from "@mistralai/mistralai";
import { createRealtimeClient, transcribeStream, AudioEncoding, type AudioFormat } from "./integrations/mistral-realtime.js";

const MISTRAL_TTS_URL = "https://api.mistral.ai/v1/audio/speech";

export class Session {
  id: string;
  ws: WebSocket;
  scenario: Skill | null = null;
  transcript = "";
  utteranceOpen = false;
  createdAt: Date;
  private anthropic: Anthropic | null = null;
   private mistral: Mistral | null = null;
   private realtimeClient: unknown = null;
  private sttController: ReadableStreamDefaultController<Uint8Array> | null = null;
  private sttStreamDone = false;
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

   initMistral(config: { apiKey: string; voiceId: string }) {
     this.mistral = new Mistral({ apiKey: config.apiKey });
     this.realtimeClient = createRealtimeClient(config.apiKey);
   }

   private asyncSttStream(): ReadableStream<Uint8Array> {
     return new ReadableStream<Uint8Array>({
       pull: (controller) => {
         if (this.sttStreamDone) {
           controller.close();
         }
       },
       cancel: () => {
         this.sttController = null;
       },
       start: (controller) => {
         this.sttController = controller;
       },
     });
   }

   startVoxtralSTT(config: { apiKey: string; voiceId: string }): void {
     this.initMistral(config);
     console.warn(`Session ${this.id}: starting Voxtral STT...`);

     const audioStream = this.asyncSttStream();
     const audioFormat: AudioFormat = {
       encoding: AudioEncoding.PcmS16le,
       sampleRate: 16000,
     };

       void (async () => {
         try {
           if (!this.realtimeClient) return;
           for await (const event of transcribeStream(this.realtimeClient, audioStream, "voxtral-mini-transcribe-realtime-2602", audioFormat)) {
             if (event.type === "transcription.text.delta") {
               this.send({ type: "transcript", text: event.text, isFinal: false });
             } else if (event.type === "transcription.done") {
               this.send({ type: "transcript", text: "", isFinal: true });
               break;
             } else if (event.type === "error") {
               const errMsg = (event as { error?: { message?: unknown } }).error?.message;
               const msg = typeof errMsg === "string" ? errMsg : JSON.stringify(errMsg);
               console.error(`Session ${this.id}: Voxtral STT error — ${msg}`);
               this.send({ type: "error", message: `STT error: ${msg}` });
               break;
             }
           }
         } catch (err: unknown) {
           console.error(`Session ${this.id}: Voxtral STT stream error:`, err);
           this.send({ type: "error", message: "STT stream error" });
         }
       })();

     console.warn(`Session ${this.id}: Voxtral STT started`);
   }

  pushClientAudio(data: Buffer | ArrayBuffer): void {
    if (!this.sttController || this.sttStreamDone) return;
    const uint8 = new Uint8Array(data instanceof Buffer ? data : new Uint8Array(data));
    this.sttController.enqueue(uint8);
  }

  endUtterance(): void {
    if (!this.sttController || this.sttStreamDone) return;
    this.sttStreamDone = true;
    this.sttController.close();
    this.sttController = null;
  }

   closeVoxtral(): void {
     this.sttStreamDone = true;
     this.sttController?.close();
     this.sttController = null;
     this.realtimeClient = null;
     this.mistral = null;
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
      console.error(`Anthropic stream error for session ${this.id}:`, err);
    }
  }
}