interface AudioWorkletPort {
  onmessage: ((event: MessageEvent) => void) | null;
  postMessage(message: unknown, transfer?: unknown[]): void;
}

declare class AudioWorkletProcessor {
  readonly port: AudioWorkletPort;
  process(
    inputs: Float32Array[][],
    outputs: Float32Array[][],
    parameters: Record<string, Float32Array>,
  ): boolean;
}

declare function registerProcessor(
  name: string,
  processorCtor: typeof AudioWorkletProcessor,
): void;
