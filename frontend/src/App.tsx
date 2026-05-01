import type { Component } from "solid-js";
import { createSignal, onCleanup, onMount, Show } from "solid-js";
import { createWsStore } from "./ws-store";
import { ScenarioPicker } from "./scenario-picker";
import { ChatWindow } from "./chat-window";
import { OpusPlayer } from "./opus-player";

const SAMPLE_RATE = 16000;

const App: Component = () => {
  const ws = createWsStore();
  const [isRecording, setIsRecording] = createSignal(false);
  const [selectedScenario, setSelectedScenario] = createSignal<string | null>(null);

  let player: OpusPlayer | null = null;
  let audioContext: AudioContext | null = null;
  let workletNode: AudioWorkletNode | null = null;
  let mediaStream: MediaStream | null = null;

  const startAudioPlayer = () => {
    player = new OpusPlayer();
    player.play();
    ws.setAudioPlayer(player);
  };

  const handleScenarioSelect = (scenarioId: string) => {
    console.warn("[App] scenario selected:", scenarioId);
    setSelectedScenario(scenarioId);
    startAudioPlayer();
    ws.clearEntries();
    ws.startSession(scenarioId);
  };

  const startRecording = async () => {
    if (!player) return;
    mediaStream = await navigator.mediaDevices.getUserMedia({ audio: true });
    audioContext = new AudioContext({ sampleRate: SAMPLE_RATE });

    await audioContext.audioWorklet.addModule(
      new URL("./pcm-processor.worklet.ts", import.meta.url).href
    );

    workletNode = new AudioWorkletNode(audioContext, "pcm-processor");
    workletNode.port.onmessage = (event: MessageEvent<{ buffer: ArrayBuffer }>) => {
      const uint8 = new Uint8Array(event.data.buffer);
      ws.sendAudioChunk(uint8.buffer);
    };

    const source = audioContext.createMediaStreamSource(mediaStream);
    source.connect(workletNode);
    workletNode.connect(audioContext.destination);

    ws.startUtterance();
    setIsRecording(true);
  };

  const stopRecording = () => {
    if (workletNode) {
      workletNode.port.onmessage = null;
      workletNode.disconnect();
      workletNode = null;
    }
     if (audioContext) {
       void audioContext.close().catch((): void => {
         // Silently ignore errors closing audio context
       });
       audioContext = null;
     }
     if (mediaStream) {
       mediaStream.getTracks().forEach((t): void => {
         t.stop();
       });
       mediaStream = null;
     }
    ws.endUtterance();
    setIsRecording(false);
  };

  const toggleRecording = () => {
    if (isRecording()) {
      stopRecording();
    } else {
      startRecording().catch((err: unknown) => {
        console.error("startRecording failed:", err);
      });
    }
  };

  onMount(() => {
    ws.connect();
  });

  onCleanup(() => {
    if (isRecording()) {
      stopRecording();
    }
    if (player) {
      player.destroy();
    }
    ws.disconnect();
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

      <Show when={ws.state.status === "disconnected"}>
        <button
          onClick={() => { ws.connect(); }}
          style={{ ...btnStyle, background: "#007aff" }}
        >
          Connect
        </button>
      </Show>

      <Show when={ws.state.status === "connecting" || ws.state.status === "reconnecting"}>
        <p style={{ color: "#888" }}>
          {ws.state.status === "reconnecting"
            ? "Reconnecting... (attempt " + String(ws.state.reconnectAttempt) + ")"
            : "Connecting..."}
        </p>
      </Show>

      <Show when={ws.state.status === "connected" && !ws.state.sessionReady}>
        <ScenarioPicker onSelect={handleScenarioSelect} disabled={false} />
      </Show>

      <Show when={ws.state.sessionReady}>
        <div style={{ display: "flex", "flex-direction": "column", "align-items": "center", gap: "1rem" }}>
          <Show when={selectedScenario()}>
            <p style={{ color: "#34c759", margin: 0 }}>
              Scenario: {selectedScenario()}
            </p>
          </Show>
          <button
            onClick={toggleRecording}
            disabled={!ws.state.sessionReady}
            style={{
              ...btnStyle,
              background: isRecording() ? "#ff3b30" : "#007aff",
              opacity: ws.state.sessionReady ? 1 : 0.5,
              cursor: ws.state.sessionReady ? "pointer" : "not-allowed",
            }}
          >
            {isRecording() ? "Stop & Send" : "Push to Talk"}
          </button>
        </div>
      </Show>

      <Show when={ws.state.lastError}>
        <p style={{ color: "#ff3b30" }}>{ws.state.lastError}</p>
      </Show>

      <ChatWindow entries={ws.state.entries} assistantText={ws.state.assistantText} />
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