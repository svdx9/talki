// WebSocket protocol shared between frontend and backend.
//
// Text frames carry JSON control messages (the types below).
// Binary frames carry raw PCM audio (48 kHz, 16-bit signed little-endian):
//   - client -> server: microphone capture chunks while user holds push-to-talk
//   - server -> client: ElevenLabs TTS output streamed back to the browser

export type ClientMsg =
  | { type: "start_session"; scenarioId: string }
  | { type: "end_utterance" }
  | { type: "cancel" };

export type ServerMsg =
  | { type: "session_ready"; greeting?: string }
  | { type: "transcript"; text: string; isFinal: boolean }
  | { type: "assistant_text_delta"; text: string }
  | { type: "assistant_done"; fullText: string }
  | { type: "error"; message: string };
