import type { Component} from "solid-js";
import { createSignal, onCleanup, onMount } from "solid-js";
import type { ClientMsg, ServerMsg } from "talki-shared";
import { AudioPlayer } from "./audio-player";

const WS_URL = `${location.protocol === "https:" ? "wss:" : "ws:"}//${location.host}/api/ws`;

const App: Component = () => {
  const [connected, setConnected] = createSignal(false);
  const [sessionReady, setSessionReady] = createSignal(false);
  const [transcript, setTranscript] = createSignal("");
  const [assistantText, setAssistantText] = createSignal("");
  const [isRecording, setIsRecording] = createSignal(false);
  const [isAssistantSpeaking, setIsAssistantSpeaking] = createSignal(false);

  let ws: WebSocket | null = null;
  let player: AudioPlayer | null = null;
  let mediaRecorder: MediaRecorder | null = null;
  let audioChunks: Blob[] = [];

  const connect = () => {
    ws = new WebSocket(WS_URL);

    ws.onopen = () => {
      setConnected(true);
    };

    ws.onmessage = (event) => {
      if (typeof event.data === "string") {
        const msg = JSON.parse(event.data) as ServerMsg;

        if (msg.type === "session_ready") {
          setSessionReady(true);
          if (msg.greeting) {
            setAssistantText(msg.greeting);
          }
        }

        if (msg.type === "transcript") {
          setTranscript((prev) => prev + (msg.isFinal ? msg.text + " " : msg.text));
        }

        if (msg.type === "assistant_text_delta") {
          setAssistantText((prev) => prev + msg.text);
          setIsAssistantSpeaking(true);
        }

        if (msg.type === "assistant_done") {
          setAssistantText(msg.fullText);
          setIsAssistantSpeaking(false);
        }

        if (msg.type === "error") {
          console.error("Server error:", msg.message);
        }
      } else if (event.data instanceof Blob || event.data instanceof ArrayBuffer) {
        // PCM audio chunk from ElevenLabs
        const data = event.data instanceof Blob ? event.data.arrayBuffer() : event.data;
        Promise.resolve(data).then((buf) => {
          if (player && buf.byteLength > 0) {
            player.play(buf);
          }
        }).catch(() => {
          // ignore
        });
      }
    };

    ws.onclose = () => {
      setConnected(false);
      setSessionReady(false);
    };

    ws.onerror = (e) => {
      console.error("WebSocket error:", e);
    };
  };

  const startSession = () => {
    if (ws?.readyState !== WebSocket.OPEN) return;
    player = new AudioPlayer();
    setTranscript("");
    setAssistantText("");
    const msg: ClientMsg = { type: "start_session", scenarioId: "cafe-a2" };
    ws.send(JSON.stringify(msg));
  };

  const startRecording = async () => {
    if (!player) return;
    const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
    mediaRecorder = new MediaRecorder(stream);
    audioChunks = [];

    mediaRecorder.ondataavailable = (e) => {
      if (e.data.size > 0) {
        audioChunks.push(e.data);
        // Send raw audio chunk to backend
        e.data.arrayBuffer().then((buf) => {
          if (ws?.readyState === WebSocket.OPEN) {
            ws.send(buf);
          }
        }).catch(() => {
          // ignore
        });
      }
    };

    mediaRecorder.onstop = () => {
      stream.getTracks().forEach((t) => { t.stop(); });
      if (ws?.readyState === WebSocket.OPEN) {
        const msg: ClientMsg = { type: "end_utterance" };
        ws.send(JSON.stringify(msg));
      }
    };

    mediaRecorder.start(100); // send chunks every 100ms
    setIsRecording(true);
  };

  const stopRecording = () => {
    mediaRecorder?.stop();
    setIsRecording(false);
  };

  const toggleRecording = () => {
    if (isRecording()) {
      stopRecording();
    } else {
      startRecording().catch(() => {
        // ignore
      });
    }
  };

  onMount(() => {
    connect();
  });

  onCleanup(() => {
    if (player) player.destroy();
    player = null;
    ws?.close();
  });

  return (
    <div
      style={{
        "min-height": "100vh",
        display: "flex",
        "flex-direction": "column",
        "align-items": "center",
        "font-family": "system-ui, sans-serif",
        background: "#f5f5f5",
        padding: "2rem",
      }}
    >
      <h1 style={{ "font-size": "2.5rem", margin: "0 0 0.5rem" }}>Talki</h1>
      <p style={{ color: "#555", margin: "0 0 2rem" }}>Conversational French tutor</p>

      {!connected() && (
        <button onClick={connect} style={{ ...btnStyle, background: "#007aff" }}>
          Connect
        </button>
      )}

      {connected() && !sessionReady() && (
        <button onClick={startSession} style={{ ...btnStyle, background: "#34c759" }}>
          Start Session (Café A2)
        </button>
      )}

      {sessionReady() && (
        <button
          onClick={toggleRecording}
          style={{
            ...btnStyle,
            background: isRecording() ? "#ff3b30" : "#007aff",
          }}
        >
          {isRecording() ? "Stop & Send" : "Push to Talk"}
        </button>
      )}

      {isAssistantSpeaking() && (
        <p style={{ color: "#007aff", "font-style": "italic" }}>Tutor is speaking...</p>
      )}

      <div
        style={{
          "margin-top": "2rem",
          width: "100%",
          "max-width": "600px",
          background: "#fff",
          padding: "1rem",
          "border-radius": "8px",
          "text-align": "left",
        }}
      >
        <h3 style={{ margin: "0 0 0.5rem" }}>Transcript</h3>
        <p style={{ "white-space": "pre-wrap", margin: 0 }}>{transcript()}</p>
      </div>

      <div
        style={{
          "margin-top": "1rem",
          width: "100%",
          "max-width": "600px",
          background: "#fff",
          padding: "1rem",
          "border-radius": "8px",
          "text-align": "left",
        }}
      >
        <h3 style={{ margin: "0 0 0.5rem" }}>Tutor</h3>
        <p style={{ "white-space": "pre-wrap", margin: 0 }}>{assistantText()}</p>
      </div>
    </div>
  );
};

const btnStyle = {
  padding: "0.75rem 1.5rem",
  color: "#fff",
  border: "none",
  "border-radius": "8px",
  "font-size": "1rem",
  cursor: "pointer",
};

export default App;
