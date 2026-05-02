/**
 * Mistral Realtime STT wrapper module.
 *
 * Uses the lower-level `connect()` API instead of `transcribeStream`, because the latter
 * tears down on the first error event — and Voxtral emits a spurious "Cannot flush audio
 * before sending any audio bytes" error right after session start, which kills the loop
 * before any real audio is sent. Owning the event loop here lets us ignore that one event.
 */

// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore — Mistral SDK does not export realtime subpath
import { RealtimeTranscription, AudioEncoding } from "@mistralai/mistralai/extra/realtime";
import type { AudioFormat } from "@mistralai/mistralai/extra/realtime";

export type { AudioFormat };
export { AudioEncoding };

export type TranscriptionEvent =
  | { type: "session.created" }
  | { type: "session.updated" }
  | { type: "transcription.language"; language?: string }
  | { type: "transcription.text.delta"; text: string }
  | { type: "transcription.segment.delta"; text?: string }
  | { type: "transcription.done" }
  | { type: "error"; error: { message: unknown } }
  | { type: string };

export interface RealtimeStt {
  events(): AsyncIterable<TranscriptionEvent>;
  sendAudio(bytes: Uint8Array): Promise<void>;
  flushAudio(): Promise<void>;
  endAudio(): Promise<void>;
  close(): Promise<void>;
  readonly isClosed: boolean;
}

export async function connectRealtimeStt(
  apiKey: string,
  model: string,
  audioFormat: AudioFormat,
): Promise<RealtimeStt> {
  const client = new RealtimeTranscription({ apiKey });
  // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-unsafe-call, @typescript-eslint/no-unsafe-member-access, @typescript-eslint/no-explicit-any
  const connection = await (client as any).connect(model, { audioFormat });

  return {
    events(): AsyncIterable<TranscriptionEvent> {
      // eslint-disable-next-line @typescript-eslint/no-unsafe-call, @typescript-eslint/no-unsafe-member-access
      return connection.events() as AsyncIterable<TranscriptionEvent>;
    },
    async sendAudio(bytes: Uint8Array): Promise<void> {
      // eslint-disable-next-line @typescript-eslint/no-unsafe-call, @typescript-eslint/no-unsafe-member-access
      await connection.sendAudio(bytes);
    },
    async flushAudio(): Promise<void> {
      // eslint-disable-next-line @typescript-eslint/no-unsafe-call, @typescript-eslint/no-unsafe-member-access
      await connection.flushAudio();
    },
    async endAudio(): Promise<void> {
      // eslint-disable-next-line @typescript-eslint/no-unsafe-call, @typescript-eslint/no-unsafe-member-access
      await connection.endAudio();
    },
    async close(): Promise<void> {
      // eslint-disable-next-line @typescript-eslint/no-unsafe-call, @typescript-eslint/no-unsafe-member-access
      await connection.close();
    },
    get isClosed(): boolean {
      // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
      return connection.isClosed as boolean;
    },
  };
}
