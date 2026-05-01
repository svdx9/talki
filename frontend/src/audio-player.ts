// PCM audio player — plays 48 kHz, 16-bit signed little-endian chunks without gaps.
// Uses Web Audio API with precise scheduling.

export class AudioPlayer {
  private ctx: AudioContext;
  private nextStartTime = 0;
  private gain: GainNode;
  private paused = false;

  constructor() {
    this.ctx = new AudioContext({ sampleRate: 48000 });
    this.gain = this.ctx.createGain();
    this.gain.connect(this.ctx.destination);
  }

  get sampleRate(): number {
    return this.ctx.sampleRate;
  }

  get currentTime(): number {
    return this.ctx.currentTime;
  }

  play(chunk: ArrayBuffer): void {
    if (this.paused) return;
    if (this.ctx.state === "suspended") {
      this.ctx.resume().catch(() => { /* ignore */ });
    }

    const pcm16 = new Int16Array(chunk);
    const float32 = new Float32Array(pcm16.length);
    for (let i = 0; i < pcm16.length; i++) {
      float32[i] = pcm16[i] / 32768;
    }

    const buffer = this.ctx.createBuffer(1, float32.length, this.ctx.sampleRate);
    buffer.getChannelData(0).set(float32);

    const source = this.ctx.createBufferSource();
    source.buffer = buffer;
    source.connect(this.gain);

    const startAt = Math.max(this.ctx.currentTime, this.nextStartTime);
    source.start(startAt);
    this.nextStartTime = startAt + buffer.duration;

    source.onended = () => {
      source.disconnect();
    };
  }

  pause(): void {
    this.paused = true;
    this.ctx.suspend().catch(() => { /* ignore */ });
  }

  resume(): void {
    this.paused = false;
    this.ctx.resume().catch(() => { /* ignore */ });
  }

  flush(): void {
    // Gap silence: reset scheduling so next chunk starts immediately
    this.nextStartTime = this.ctx.currentTime;
  }

  stop(): void {
    this.nextStartTime = this.ctx.currentTime;
    this.gain.gain.setTargetAtTime(0, this.ctx.currentTime, 0.05);
    setTimeout(() => {
      this.gain.gain.setTargetAtTime(1, this.ctx.currentTime, 0.001);
    }, 200);
  }

  destroy(): void {
    this.ctx.close().catch(() => { /* ignore */ });
  }
}
