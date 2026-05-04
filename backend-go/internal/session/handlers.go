package session

import (
	"context"
	"fmt"

	"github.com/svdx9/talki/backend-go/internal/protocol"
)

// handle decodes the raw client message and dispatches to the appropriate handler.
// State-machine errors (invalid transitions) are sent as error ServerMsgs and return nil.
// Decode errors or unhandled types are returned for logging.
func (s *Session) handle(ctx context.Context, raw []byte) error {
	msg, err := protocol.DecodeClient(raw)
	if err != nil {
		s.Send(protocol.NewError(fmt.Sprintf("Decode error: %v", err)))
		return fmt.Errorf("decode error: %w", err)
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
		return fmt.Errorf("unhandled message type: %T", m)
	}
	return nil
}

// handleStartSession validates the scenario, sets it, clears transcript, and sends session_ready.
// The ctx parameter is unused today but will be passed to STT/TTS initialization in 11.7+.
func (s *Session) handleStartSession(ctx context.Context, msg *protocol.StartSession) {
	// TODO: use ctx for STT/TTS upstream calls when they land
	s.log.Info("start_session", "session", s.id, "scenario", msg.ScenarioID, "sampleRate", msg.SampleRate)

	skill := s.findSkillByID(msg.ScenarioID)
	if skill == nil {
		s.log.Warn("unknown scenario", "session", s.id, "scenario", msg.ScenarioID)
		s.Send(protocol.NewError(fmt.Sprintf("Unknown scenario: %s", msg.ScenarioID)))
		return
	}

	s.scenario = skill
	s.transcript.Reset()
	s.utteranceOpen = false
	s.convHistory = nil

	s.Send(protocol.NewSessionReady(skill.OpeningLine))

	// TODO(stt): Initialize STT with skill's voice, locale, and msg.SampleRate
	// TODO(tts): Initialize TTS connection with skill's voice

	s.log.Info("session started", "session", s.id, "scenario", skill.ID, "title", skill.Title)
}

// handleStartUtterance begins audio capture.
// Sends an error ServerMsg if an utterance is already open or if no scenario is active.
func (s *Session) handleStartUtterance(ctx context.Context) {
	s.log.Info("start_utterance", "session", s.id)

	if s.utteranceOpen {
		s.log.Warn("double start_utterance", "session", s.id)
		s.Send(protocol.NewError("Utterance already open"))
		return
	}

	if s.scenario == nil {
		s.log.Warn("start_utterance without scenario", "session", s.id)
		s.Send(protocol.NewError("No active scenario; send start_session first"))
		return
	}

	s.utteranceOpen = true
	s.transcript.Reset()

	// TODO(stt): Send audio to STT service
}

// handleEndUtterance closes the current audio capture.
// Sends an error ServerMsg if no utterance is open.
// The assistant response is left as a TODO(llm).
func (s *Session) handleEndUtterance(ctx context.Context) {
	s.log.Info("end_utterance", "session", s.id)

	if !s.utteranceOpen {
		s.log.Warn("end_utterance without start", "session", s.id)
		s.Send(protocol.NewError("No open utterance to end"))
		return
	}

	s.utteranceOpen = false
	transcriptText := s.transcript.String()
	s.log.Info("utterance closed", "session", s.id, "transcript", transcriptText)

	// TODO(llm): Send transcript to Anthropic and stream response
}

// handleEndSession clears the session state.
// No upstream effects yet (STT/TTS/LLM cleanup left for later).
func (s *Session) handleEndSession(ctx context.Context) {
	s.log.Info("end_session", "session", s.id)

	s.scenario = nil
	s.transcript.Reset()
	s.utteranceOpen = false
	s.convHistory = nil

	// TODO: Close STT/TTS connections gracefully
}

// handleCancel aborts the current operation.
// Clears any open utterance; detailed STT/LLM cancellation left for later.
func (s *Session) handleCancel(ctx context.Context) {
	s.log.Info("cancel", "session", s.id)

	if s.utteranceOpen {
		s.utteranceOpen = false
		s.transcript.Reset()
		// TODO: Cancel ongoing STT/LLM operations
	}
}
