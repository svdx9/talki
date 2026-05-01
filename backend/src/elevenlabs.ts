import { WebSocket } from "ws";
import { EventEmitter } from "events";

const ELEVENLABS_WS_URL = "wss://api.elevenlabs.io/v1/text-to-speech";

export interface ElevenLabsConfig {
  apiKey: string;
  voiceId: string;
  modelId?: string;
  outputFormat?: string;
}

interface ElevenLabsEnvelope {
  audio?: string;
  isFinal?: boolean;
  error?: string;
}

export interface ElevenLabsWSClient extends EventEmitter {
  on(event: "audio", listener: (audioBuffer: Buffer) => void): this;
  on(event: "error", listener: (error: Error) => void): this;
  on(event: "close" | "isFinal", listener: () => void): this;
}

export class ElevenLabsStream extends EventEmitter {
  private ws: WebSocket | null = null;
  private config: ElevenLabsConfig;
  private sentenceBuffer = "";
  private isStreaming = false;
  private isClosed = false;
  private isClosing = false;

  get closing(): boolean {
    return this.isClosing;
  }

  constructor(config: ElevenLabsConfig) {
    super();
    this.config = {
      modelId: "eleven_flash_v2_5",
      outputFormat: "pcm_48000",
      ...config,
    };
  }

  async connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      const url = `${ELEVENLABS_WS_URL}/${this.config.voiceId}/stream-input?model_id=${this.config.modelId ?? ""}&output_format=${this.config.outputFormat ?? ""}`;

      this.ws = new WebSocket(url, {
        headers: {
          "xi-api-key": this.config.apiKey,
        },
      });

      this.ws.on("open", () => {
        this.isStreaming = true;
        this.sendInitialConfig();
        resolve();
      });

      this.ws.on("message", (data) => {
        this.handleMessage(data);
      });

      this.ws.on("error", (error) => {
        this.emit("error", error);
        reject(error);
      });

      this.ws.on("close", () => {
        this.isStreaming = false;
        this.isClosed = true;
        this.emit("close");
      });
    });
  }

  private sendInitialConfig() {
    if (this.ws?.readyState !== WebSocket.OPEN) return;

    const config = {
      text: "",
      voice_settings: {
        stability: 0.5,
        similarity_boost: 0.75,
      },
      generation_config: {
        chunk_length_schedule: [120, 160, 250, 290],
      },
    };

    this.ws.send(JSON.stringify(config));
  }

  sendText(text: string, flush = false): void {
    if (!this.isStreaming || !this.ws) return;

    const message: { text: string; flush?: boolean } = { text };

    if (flush) {
      message.flush = true;
    }

    this.ws.send(JSON.stringify(message));
  }

  sendSentence(text: string): void {
    this.sentenceBuffer += text;

    if (this.isSentenceEnd(this.sentenceBuffer)) {
      const sentence = this.sentenceBuffer.trim();
      if (sentence) {
        this.sendText(sentence, true);
      }
      this.sentenceBuffer = "";
    }
  }

  private isSentenceEnd(text: string): boolean {
    return /[.!?。！？\n]$/.test(text.trim());
  }

  flush(): void {
    if (this.sentenceBuffer.trim()) {
      this.sendText(this.sentenceBuffer.trim(), true);
      this.sentenceBuffer = "";
    }
  }

  private handleMessage(data: WebSocket.Data) {
    let jsonStr: string;
    if (typeof data === "string") {
      jsonStr = data;
    } else if (data instanceof Buffer) {
      jsonStr = data.toString("utf8");
    } else if (data instanceof ArrayBuffer) {
      jsonStr = Buffer.from(data).toString("utf8");
    } else {
      return;
    }

    let msg: ElevenLabsEnvelope;
    try {
      msg = JSON.parse(jsonStr) as ElevenLabsEnvelope;
    } catch (e) {
      this.emit("error", new Error(`Malformed ElevenLabs frame: ${(e as Error).message}`));
      return;
    }

    if (msg.error) {
      this.emit("error", new Error(msg.error));
      return;
    }
    if (msg.audio) {
      this.emit("audio", Buffer.from(msg.audio, "base64"));
    }
    if (msg.isFinal) {
      this.emit("isFinal");
    }
  }

  async close(): Promise<void> {
    return new Promise((resolve) => {
      if (!this.ws || !this.isStreaming) {
        resolve();
        return;
      }

      this.isClosing = true;
      this.flush();

      const onFinal = () => {
        this.removeListener("isFinal", onFinal);
        this.ws?.close();
        resolve();
      };

      this.on("isFinal", onFinal);

      this.ws.send(JSON.stringify({ text: "" }));

      setTimeout(() => {
        this.removeListener("isFinal", onFinal);
        this.ws?.close();
        resolve();
      }, 5000);
    });
  }
}
