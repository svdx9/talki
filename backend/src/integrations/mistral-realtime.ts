/**
 * Mistral Realtime STT wrapper module.
 *
 * This module isolates type safety issues with the Mistral SDK's realtime API,
 * which does not properly export type definitions for the /extra/realtime subpath.
 *
 * All unsafe-any suppressions are contained here, keeping the rest of the codebase clean.
 */

// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore — Mistral SDK does not export realtime subpath
import { RealtimeTranscription, AudioEncoding } from "@mistralai/mistralai/extra/realtime";
import type { AudioFormat } from "@mistralai/mistralai/extra/realtime";

export type { AudioFormat };
export { AudioEncoding };

/**
 * Creates a Mistral realtime transcription client.
 * Safe wrapper that hides SDK type safety issues internally.
 */
export function createRealtimeClient(apiKey: string): unknown {
    return new RealtimeTranscription({ apiKey });
  }

/**
 * Transcribes an audio stream in real-time from Mistral.
 * Yields partial and final transcription events.
 */
export async function* transcribeStream(
  client: unknown,
  audioStream: ReadableStream<Uint8Array>,
  model: string,
  audioFormat: AudioFormat,
): AsyncGenerator<TranscriptionEvent, void> {
  // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access, @typescript-eslint/no-unsafe-call, @typescript-eslint/no-explicit-any
  const iter = (client as any)?.transcribeStream?.(audioStream, model, { audioFormat }) as AsyncIterable<unknown>;

  for await (const event of iter) {
    yield event as TranscriptionEvent;
  }
}

export type TranscriptionEvent =
  | { type: "transcription.session.created" }
  | { type: "transcription.text.delta"; text: string }
  | { type: "transcription.done" }
  | { type: "error"; error: { message: unknown } };
