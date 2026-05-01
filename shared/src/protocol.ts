// WebSocket protocol shared between frontend and backend.
//
// Text frames carry JSON control messages (the types below).
// Binary frames are only valid inside an utterance window:
//   client opens it with `start_utterance`, streams audio chunks (webm/opus),
//   and closes it with `end_utterance`. Binary frames received outside this
//   window are protocol violations and the server replies with an `error`.

export type ClientMsg =
  | { type: "start_session"; scenarioId: string }
  | { type: "start_utterance" }
  | { type: "end_utterance" }
  | { type: "cancel" };

export type ServerMsg =
  | { type: "session_ready"; greeting?: string }
  | { type: "transcript"; text: string; isFinal: boolean }
  | { type: "assistant_text_delta"; text: string }
  | { type: "assistant_done"; fullText: string }
  | { type: "error"; message: string };
