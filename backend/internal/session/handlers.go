package session

import (
	"context"
	"fmt"
	"math"

	"github.com/svdx9/talki/backend/internal/protocol"
	"github.com/svdx9/talki/backend/internal/tts"
)

const voxtralSTTModel = "voxtral-mini-transcribe-realtime-2602"

// handle decodes the raw client message and dispatches to the appropriate handler.
// State-machine errors (invalid transitions) are sent as error ServerMsgs and return nil.
// Decode errors or unhandled types are returned for logging.
func (s *Session) handle(ctx context.Context, raw []byte) error {
	msg, err := protocol.DecodeClient(raw)
	if err != nil {
		s.SendText(ctx, protocol.NewError(fmt.Sprintf("Decode error: %v", err)))
		s.log.Warn("decode error", "session", s.id, "error", err)
		return nil
	}

	switch m := msg.(type) {
	case *protocol.StartSession:
		s.handleStartSession(ctx, m)
	case *protocol.StartUtterance:
		s.handleStartUtterance(ctx)
	case *protocol.EndUtterance:
		s.handleEndUtterance(ctx)
	case *protocol.EndSession:
		s.handleEndSession(ctx)
	case *protocol.Cancel:
		s.handleCancel(ctx)
	default:
		s.log.Warn("unhandled message type", "session", s.id, "type", fmt.Sprintf("%T", m))
		s.SendText(ctx, protocol.NewError(fmt.Sprintf("Unhandled message type: %T", m)))
	}
	return nil
}

// handleStartSession validates the scenario, opens a Voxtral STT connection,
// and sends session_ready to the client.
func (s *Session) handleStartSession(ctx context.Context, msg *protocol.StartSession) {
	s.log.Info("start_session", "session", s.id, "scenario", msg.ScenarioID, "sampleRate", msg.SampleRate)

	skill, err := s.allSkills.Get(msg.ScenarioID)
	if err != nil {
		s.log.Warn("unknown scenario", "session", s.id, "scenario", msg.ScenarioID)
		s.SendText(ctx, protocol.NewError("Unknown scenario: "+msg.ScenarioID))
		return
	}

	// Close any previous STT connection before opening a new one.
	s.closeSTT()

	s.scenario = skill
	s.transcript.Reset()
	s.utteranceOpen = false
	s.convHistory = nil

	af := tts.AudioFormat{Encoding: "pcm_s16le", SampleRate: msg.SampleRate}
	client, err := s.dialSTT(ctx, s.cfg.MistralAPIKey, voxtralSTTModel, af)
	if err != nil {
		s.log.Error("STT dial failed", "session", s.id, "error", err)
		s.SendText(ctx, protocol.NewError("STT connection failed"))
		return
	}
	s.sttClient = client
	s.sttBytesPushed = 0
	s.utterancePeak = 0

	s.sttWg.Add(1)
	go s.readSTTEvents(ctx, client)

	s.SendText(ctx, protocol.NewSessionReady(skill.OpeningLine))

	go func() {
		sink := &binaryWriter{ctx: ctx, outgoing: s.outgoing}
		//exhaustruct:ignore
		err := s.ttsClient.Speech(ctx, tts.SpeechVoice{VoiceID: skill.Voice}, skill.OpeningLine, sink)
		if err != nil {
			s.log.Error("TTS stream error", "session", s.id, "error", err)
		}
	}()

	s.log.Info("session started", "session", s.id, "scenario", skill.ID, "title", skill.Title)
}

// handleStartUtterance begins audio capture.
// Sends an error ServerMsg if an utterance is already open or if no scenario is active.
func (s *Session) handleStartUtterance(ctx context.Context) {
	s.log.Info("start_utterance", "session", s.id)

	if s.utteranceOpen {
		s.log.Warn("double start_utterance", "session", s.id)
		s.SendText(ctx, protocol.NewError("Utterance already open"))
		return
	}

	if s.scenario == nil {
		s.log.Warn("start_utterance without scenario", "session", s.id)
		s.SendText(ctx, protocol.NewError("No active scenario; send start_session first"))
		return
	}

	s.utteranceOpen = true
	s.transcript.Reset()
	s.sttBytesPushed = 0
	s.utterancePeak = 0
}

// handleEndUtterance closes the current audio capture and flushes to the STT service.
// Sends an error ServerMsg if no utterance is open.
func (s *Session) handleEndUtterance(ctx context.Context) {
	s.log.Info("end_utterance", "session", s.id)

	if !s.utteranceOpen {
		s.log.Warn("end_utterance without start", "session", s.id)
		s.SendText(ctx, protocol.NewError("No open utterance to end"))
		return
	}

	s.utteranceOpen = false

	if s.sttClient == nil || s.sttBytesPushed == 0 {
		s.log.Warn("end_utterance with no audio — skipping flush", "session", s.id)
		s.sttBytesPushed = 0
		s.utterancePeak = 0
		return
	}

	peakDB := math.Inf(-1)
	if s.utterancePeak > 0 {
		peakDB = 20 * math.Log10(float64(s.utterancePeak)/32768.0)
	}
	s.log.Warn("utterance stats",
		"session", s.id,
		"peak", s.utterancePeak,
		"dBFS", fmt.Sprintf("%.1f", peakDB),
		"bytes", s.sttBytesPushed,
	)

	err := s.sttClient.Flush(ctx)
	if err != nil {
		s.log.Error("STT flush error", "session", s.id, "error", err)
	}

	s.sttBytesPushed = 0
	s.utterancePeak = 0

}

// handleEndSession closes the STT connection and clears session state.
func (s *Session) handleEndSession(_ context.Context) {
	s.log.Info("end_session", "session", s.id)

	s.mu.Lock()
	if s.llmCancel != nil {
		s.llmCancel()
	}
	s.mu.Unlock()

	s.closeSTT()

	s.scenario = nil
	s.transcript.Reset()
	s.utteranceOpen = false
	s.convHistory = nil
}

// handleCancel aborts any open utterance. STT remains open so the user can speak again.
func (s *Session) handleCancel(_ context.Context) {
	s.log.Info("cancel", "session", s.id)

	if s.utteranceOpen {
		s.utteranceOpen = false
		s.transcript.Reset()
		s.sttBytesPushed = 0
		s.utterancePeak = 0
	}

	s.mu.Lock()
	if s.llmCancel != nil {
		s.llmCancel()
	}
	s.mu.Unlock()
}
