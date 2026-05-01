import type { Component } from "solid-js";
import { For, Show } from "solid-js";

interface TranscriptEntry {
  id: number;
  speaker: "user" | "assistant";
  text: string;
  isFinal: boolean;
}

interface ChatWindowProps {
  entries: TranscriptEntry[];
  assistantText: string;
}

export const ChatWindow: Component<ChatWindowProps> = (props) => {
  return (
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
      <h3 style={{ margin: "0 0 1rem" }}>Conversation</h3>
      <div
        style={{
          display: "flex",
          "flex-direction": "column",
          gap: "0.75rem",
          "min-height": "150px",
        }}
      >
        <For each={props.entries}>
          {(entry) => (
            <div
              style={{
                display: "flex",
                "flex-direction": "column",
                "align-items": entry.speaker === "user" ? "flex-end" : "flex-start",
              }}
            >
              <span
                style={{
                  "font-size": "0.75rem",
                  color: "#888",
                  "margin-bottom": "0.25rem",
                  "text-transform": "uppercase",
                  "font-weight": "600",
                }}
              >
                {entry.speaker === "user" ? "You" : "Tutor"}
                {!entry.isFinal && " (interim)"}
              </span>
              <span
                style={{
                  padding: "0.5rem 0.75rem",
                  "border-radius": "12px",
                  "max-width": "80%",
                  "word-break": "break-word",
                  background: entry.speaker === "user" ? "#007aff" : "#e9e9e9",
                  color: entry.speaker === "user" ? "#fff" : "#333",
                  "font-style": entry.speaker === "user" && !entry.isFinal ? "italic" : "normal",
                  opacity: entry.speaker === "user" && !entry.isFinal ? 0.7 : 1,
                }}
              >
                {entry.text}
              </span>
            </div>
          )}
        </For>
        <Show when={props.assistantText}>
          <div
            style={{
              display: "flex",
              "flex-direction": "column",
              "align-items": "flex-start",
            }}
          >
            <span
              style={{
                "font-size": "0.75rem",
                color: "#888",
                "margin-bottom": "0.25rem",
                "text-transform": "uppercase",
                "font-weight": "600",
              }}
            >
              Tutor
            </span>
            <span
              style={{
                padding: "0.5rem 0.75rem",
                "border-radius": "12px",
                "max-width": "80%",
                "word-break": "break-word",
                background: "#e9e9e9",
                color: "#333",
              }}
            >
              {props.assistantText}
            </span>
          </div>
        </Show>
      </div>
    </div>
  );
};