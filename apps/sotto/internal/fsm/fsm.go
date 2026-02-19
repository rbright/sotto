package fsm

import "fmt"

// State is one lifecycle state for a dictation session.
type State string

// Event is one transition trigger consumed by the state machine.
type Event string

const (
	StateIdle         State = "idle"
	StateRecording    State = "recording"
	StateTranscribing State = "transcribing"
	StateError        State = "error"
)

const (
	EventStart       Event = "start"
	EventStop        Event = "stop"
	EventCancel      Event = "cancel"
	EventTranscribed Event = "transcribed"
	EventFail        Event = "fail"
	EventReset       Event = "reset"
)

// Transition validates and applies one state transition.
func Transition(current State, event Event) (State, error) {
	if event == EventFail {
		return StateError, nil
	}

	switch current {
	case StateIdle:
		switch event {
		case EventStart:
			return StateRecording, nil
		default:
			return current, invalidTransition(current, event)
		}
	case StateRecording:
		switch event {
		case EventStop:
			return StateTranscribing, nil
		case EventCancel:
			return StateIdle, nil
		default:
			return current, invalidTransition(current, event)
		}
	case StateTranscribing:
		switch event {
		case EventTranscribed:
			return StateIdle, nil
		default:
			return current, invalidTransition(current, event)
		}
	case StateError:
		switch event {
		case EventReset:
			return StateIdle, nil
		default:
			return current, invalidTransition(current, event)
		}
	default:
		return current, fmt.Errorf("unknown state %q", current)
	}
}

// invalidTransition formats a stable error message used by tests and callers.
func invalidTransition(state State, event Event) error {
	return fmt.Errorf("invalid transition: %s --(%s)--> ?", state, event)
}
