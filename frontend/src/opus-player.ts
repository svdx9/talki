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
        return;
      }
      try {
        this.sourceBuffer = this.mediaSource.addSourceBuffer(mimeType);
        this.sourceBuffer.mode = "sequence";
        this.sourceBuffer.addEventListener("error", (_e) => {});
        this.sourceBuffer.addEventListener("updateend", () => {
          this.drainQueue();
        });
        this.drainQueue();
      } catch (_e) {}
    });
  }

  private drainQueue(): void {
    if (!this.sourceBuffer || this.sourceBuffer.updating) return;
    const next = this.queue.shift();
    if (!next) return;
    try {
      this.sourceBuffer.appendBuffer(next);
    } catch (_e) {}
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
    this.audio.play().catch((_err: unknown) => {});
  }

  pause(): void {
    this.audio.pause();
  }

  resume(): void {
    this.audio.play().catch((_err: unknown) => {});
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
