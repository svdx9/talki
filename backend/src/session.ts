import type { Skill } from "./skills/schema.js";
import { WebSocket } from "ws";

export class Session {
  id: string;
  ws: WebSocket;
  scenario: Skill | null = null;
  transcript: string = "";
  createdAt: Date;

  constructor(id: string, ws: WebSocket) {
    this.id = id;
    this.ws = ws;
    this.createdAt = new Date();
  }

  setScenario(scenario: Skill) {
    this.scenario = scenario;
  }

  appendTranscript(text: string) {
    this.transcript += text;
  }

  clearTranscript() {
    this.transcript = "";
  }

  send(message: unknown) {
    if (this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(message));
    }
  }
}
