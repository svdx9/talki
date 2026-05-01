// MediaSource-based audio player for streaming audio frames.
// Uses MediaSource + SourceBuffer to append incoming MP3/AAC frames.

export class OpusPlayer {
  private mediaSource: MediaSource | null = null;
  private sourceBuffer: SourceBuffer | null = null;
  private audio: HTMLAudioElement;
  private queue: ArrayBuffer[] = [];

  constructor() {
    this.audio = document.createElement("audio");
    this.audio.autoplay = true;
  }

  initMediaSource(): void {
    this.mediaSource = new MediaSource();
    this.audio.src = URL.createObjectURL(this.mediaSource);

    this.mediaSource.addEventListener("sourceopen", () => {
      if (!this.mediaSource) return;
      const mimeType = this.getSupportedMimeType();
      if (!mimeType) {
        console.error("No supported audio MIME type for MediaSource");
        return;
      }
      try {
        this.sourceBuffer = this.mediaSource.addSourceBuffer(mimeType);
        this.sourceBuffer.addEventListener("error", (e) => {
          console.error("SourceBuffer error:", e);
        });
        this.sourceBuffer.addEventListener("updateend", () => {
          this.drainQueue();
        });
        this.drainQueue();
      } catch (e) {
        console.error("Failed to create SourceBuffer:", e);
      }
    });
  }

  private drainQueue(): void {
    if (!this.sourceBuffer || this.sourceBuffer.updating) return;
    const next = this.queue.shift();
    if (!next) return;
    try {
      this.sourceBuffer.appendBuffer(next);
    } catch (e) {
      console.error("Failed to append audio chunk:", e);
    }
  }

  private getSupportedMimeType(): string | null {
    if (MediaSource.isTypeSupported("audio/mpeg")) {
      return "audio/mpeg";
    }
    if (MediaSource.isTypeSupported("audio/mp4; codecs=mp4a.40.2")) {
      return "audio/mp4; codecs=mp4a.40.2";
    }
    return null;
  }

  appendFrame(chunk: ArrayBuffer): void {
    this.queue.push(chunk);
    this.drainQueue();
  }

  play(): void {
    if (this.mediaSource?.readyState !== "open") {
      this.initMediaSource();
    }
    this.audio.play().catch((err: unknown) => {
      console.error("Audio play failed:", err);
    });
  }

  pause(): void {
    this.audio.pause();
  }

  resume(): void {
    this.audio.play().catch((err: unknown) => {
      console.error("Audio resume failed:", err);
    });
  }

  flush(): void {
    if (this.sourceBuffer?.updating) {
      this.sourceBuffer.abort();
    }
  }

  stop(): void {
    if (this.mediaSource && this.mediaSource.readyState !== "ended") {
      this.mediaSource.endOfStream();
    }
    this.audio.pause();
    this.audio.currentTime = 0;
  }

  destroy(): void {
    this.stop();
    this.sourceBuffer = null;
    this.mediaSource = null;
    this.audio.src = "";
    this.audio.remove();
  }
}