import { createStore } from "solid-js/store";
import type { ClientMsg, ServerMsg } from "talki-shared";

interface AudioSink {
  appendFrame: (buf: ArrayBuffer) => void;
}

export type ConnectionStatus = "disconnected" | "connecting" | "connected" | "reconnecting";

interface TranscriptEntry {
  id: number;
  speaker: "user" | "assistant";
  text: string;
  isFinal: boolean;
}

interface WsState {
  status: ConnectionStatus;
  sessionReady: boolean;
  assistantText: string;
  entries: TranscriptEntry[];
  lastError: string;
  reconnectAttempt: number;
}

const WS_URL = `${location.protocol === "https:" ? "wss:" : "ws:"}//${location.host}/api/ws`;
const INITIAL_RECONNECT_DELAY = 1000;
const MAX_RECONNECT_DELAY = 30000;

let ws: WebSocket | null = null;
let reconnectTimeout: ReturnType<typeof setTimeout> | null = null;
let entryId = 0;

export function createWsStore() {
  const [state, setState] = createStore<WsState>({
    status: "disconnected",
    sessionReady: false,
    assistantText: "",
    entries: [],
    lastError: "",
    reconnectAttempt: 0,
  });

  let player: AudioSink | null = null;

  const onMessage = (msg: ServerMsg) => {
    console.warn("[ws] <-", msg.type, msg);
    switch (msg.type) {
      case "session_ready":
        setState({
          sessionReady: true,
          lastError: "",
        });
        if (msg.greeting) {
          const greeting = msg.greeting;
          setState("entries", (e) => [
            ...e,
            { id: ++entryId, speaker: "assistant" as const, text: greeting, isFinal: true },
          ]);
        }
        break;

      case "transcript": {
        const existing = state.entries.find(
          (ent) => ent.speaker === "user" && !ent.isFinal
        );
        if (existing) {
          setState("entries", (e) =>
            e.map((ent) =>
              ent.id === existing.id ? { ...ent, text: msg.text, isFinal: msg.isFinal } : ent
            )
          );
        } else {
          setState("entries", (e) => [
            ...e,
            { id: ++entryId, speaker: "user", text: msg.text, isFinal: msg.isFinal },
          ]);
        }
        break;
      }

      case "assistant_text_delta":
        setState("assistantText", (t) => t + msg.text);
        break;

      case "assistant_done":
        setState("entries", (e) => [
          ...e,
          { id: ++entryId, speaker: "assistant", text: msg.fullText, isFinal: true },
        ]);
        setState("assistantText", "");
        break;

      case "error":
        setState("lastError", msg.message);
        console.error("Server error:", msg.message);
        break;
    }
  };

  const setAudioPlayer = (p: AudioSink | null) => {
    player = p;
  };

  const connect = () => {
    if (reconnectTimeout) {
      clearTimeout(reconnectTimeout);
      reconnectTimeout = null;
    }

    setState("status", "connecting");
    ws = new WebSocket(WS_URL);

    ws.onopen = () => {
      setState({ status: "connected", reconnectAttempt: 0 });
    };

    ws.onmessage = (event) => {
      if (typeof event.data === "string") {
        const msg = JSON.parse(event.data) as ServerMsg;
        onMessage(msg);
      } else if (event.data instanceof ArrayBuffer || event.data instanceof Blob) {
        const data = event.data instanceof Blob ? event.data.arrayBuffer() : Promise.resolve(event.data);
        data.then((buf) => {
          if (player && buf.byteLength > 0) {
            player.appendFrame(buf);
          }
        }).catch((err: unknown) => {
          console.error("Failed to process audio chunk:", err);
        });
      }
    };

    ws.onclose = () => {
      setState({ status: "disconnected", sessionReady: false });
      scheduleReconnect();
    };

    ws.onerror = (e) => {
      console.error("WebSocket error:", e);
      setState("lastError", "WebSocket connection failed");
    };
  };

  const scheduleReconnect = () => {
    const delay = Math.min(
      INITIAL_RECONNECT_DELAY * Math.pow(2, state.reconnectAttempt),
      MAX_RECONNECT_DELAY
    );
    setState("reconnectAttempt", (a) => a + 1);
    setState("status", "reconnecting");
    console.warn("Scheduling reconnect in " + String(delay) + "ms");
    reconnectTimeout = setTimeout(() => {
      connect();
    }, delay);
  };

  const disconnect = () => {
    if (reconnectTimeout) {
      clearTimeout(reconnectTimeout);
      reconnectTimeout = null;
    }
    ws?.close();
    ws = null;
    setState({ status: "disconnected", sessionReady: false });
  };

  const send = (msg: ClientMsg) => {
    console.warn("[ws] -> send", msg.type, "readyState=", ws?.readyState);
    if (ws?.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify(msg));
    } else {
      console.warn("[ws] send dropped: socket not open");
    }
  };

  const sendAudioChunk = (buf: ArrayBuffer) => {
    if (ws?.readyState === WebSocket.OPEN) {
      ws.send(buf);
    }
  };

  const startSession = (scenarioId: string) => {
    setState({ entries: [], assistantText: "" });
    send({ type: "start_session", scenarioId });
  };

  const startUtterance = () => {
    send({ type: "start_utterance" });
  };

  const endUtterance = () => {
    send({ type: "end_utterance" });
  };

  const clearEntries = () => {
    setState("entries", []);
    setState("assistantText", "");
  };

  return {
    state,
    setAudioPlayer,
    connect,
    disconnect,
    send,
    sendAudioChunk,
    startSession,
    startUtterance,
    endUtterance,
    clearEntries,
  };
}

export type WsStore = ReturnType<typeof createWsStore>;