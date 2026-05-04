package session

import (
	"context"
	"fmt"

	"github.com/svdx9/talki/backend-go/internal/protocol"
)

// handle decodes the raw client message and dispatches to the appropriate handler.
func (s *Session) handle(ctx context.Context, raw []byte) error {
	msg, err := protocol.DecodeClient(raw)
	if err != nil {
		return fmt.Errorf("decode error: %w", err)
	}

	switch m := msg.(type) {
	case *protocol.StartSession:
		return s.handleStartSession(ctx, m)
	case *protocol.StartUtterance:
		return s.handleStartUtterance(ctx)
	case *protocol.EndUtterance:
		return s.handleEndUtterance(ctx)
	case *protocol.EndSession:
		return s.handleEndSession(ctx)
	case *protocol.Cancel:
		return s.handleCancel(ctx)
	default:
		return fmt.Errorf("unhandled message type: %T", m)
	}
}

// handleStartSession validates the scenario, sets it, clears transcript, and sends session_ready.
func (s *Session) handleStartSession(ctx context.Context, msg *protocol.StartSession) error {
	s.log.Info("start_session", "session", s.id, "scenario", msg.ScenarioID, "sampleRate", msg.SampleRate)

	// Validate scenario exists
	skill := s.findSkillByID(msg.ScenarioID)
	if skill == nil {
		errMsg := fmt.Sprintf("Unknown scenario: %s", msg.ScenarioID)
		s.log.Warn("unknown scenario", "session", s.id, "scenario", msg.ScenarioID)
		s.Send(protocol.NewError(errMsg))
		return nil
	}

	// Set the scenario and clear transcript
	s.scenario = skill
	s.transcript.Reset()
	s.utteranceOpen = false
	s.convHistory = nil

	// Send session_ready with opening line
	s.Send(protocol.NewSessionReady(skill.OpeningLine))

	// TODO(stt): Initialize STT with skill's voice and locale
	// TODO(tts): Initialize TTS connection

	s.log.Info("session started", "session", s.id, "scenario", skill.ID, "title", skill.Title)
	return nil
}

// handleStartUtterance begins audio capture.
// Returns an error ServerMsg if an utterance is already open.
func (s *Session) handleStartUtterance(ctx context.Context) error {
	s.log.Info("start_utterance", "session", s.id, "utteranceOpen", s.utteranceOpen)

	if s.utteranceOpen {
		errMsg := "Utterance already open"
		s.log.Warn("double start_utterance", "session", s.id)
		s.Send(protocol.NewError(errMsg))
		return nil
	}

	if s.scenario == nil {
		errMsg := "No active scenario; send start_session first"
		s.log.Warn("start_utterance without scenario", "session", s.id)
		s.Send(protocol.NewError(errMsg))
		return nil
	}

	s.utteranceOpen = true
	s.transcript.Reset()

	// TODO(stt): Send audio to STT service

	s.log.Info("utterance started", "session", s.id)
	return nil
}

// handleEndUtterance closes the current audio capture.
// Returns an error ServerMsg if no utterance is open.
// The assistant response is left as a TODO(llm).
func (s *Session) handleEndUtterance(ctx context.Context) error {
	s.log.Info("end_utterance", "session", s.id, "utteranceOpen", s.utteranceOpen)

	if !s.utteranceOpen {
		errMsg := "No open utterance to end"
		s.log.Warn("end_utterance without start", "session", s.id)
		s.Send(protocol.NewError(errMsg))
		return nil
	}

	s.utteranceOpen = false

	// Log the buffered transcript
	transcriptText := s.transcript.String()
	s.log.Info("utterance closed", "session", s.id, "transcript", transcriptText)

	// TODO(llm): Send transcript to Anthropic and stream response

	return nil
}

// handleEndSession clears the session state.
// No upstream effects yet (STT/TTS/LLM cleanup left for later).
func (s *Session) handleEndSession(ctx context.Context) error {
	s.log.Info("end_session", "session", s.id)

	s.scenario = nil
	s.transcript.Reset()
	s.utteranceOpen = false
	s.convHistory = nil

	// TODO: Close STT/TTS connections gracefully

	return nil
}

// handleCancel aborts the current operation.
// Currently just clears state; detailed cleanup left for later.
func (s *Session) handleCancel(ctx context.Context) error {
	s.log.Info("cancel", "session", s.id)

	if s.utteranceOpen {
		s.utteranceOpen = false
		s.transcript.Reset()
		// TODO: Cancel ongoing STT/LLM operations
	}

	return nil
}
