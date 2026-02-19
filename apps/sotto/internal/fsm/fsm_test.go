package fsm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTransitionHappyPath(t *testing.T) {
	s := StateIdle

	next, err := Transition(s, EventStart)
	require.NoError(t, err)
	require.Equal(t, StateRecording, next)

	next, err = Transition(next, EventStop)
	require.NoError(t, err)
	require.Equal(t, StateTranscribing, next)

	next, err = Transition(next, EventTranscribed)
	require.NoError(t, err)
	require.Equal(t, StateIdle, next)
}

func TestTransitionFailFromAnyStateGoesError(t *testing.T) {
	states := []State{StateIdle, StateRecording, StateTranscribing, StateError}
	for _, state := range states {
		next, err := Transition(state, EventFail)
		require.NoError(t, err)
		require.Equal(t, StateError, next)
	}
}

func TestTransitionMatrixInvalidTransitions(t *testing.T) {
	tests := []struct {
		name    string
		state   State
		event   Event
		want    State
		wantErr bool
	}{
		{name: "idle stop invalid", state: StateIdle, event: EventStop, want: StateIdle, wantErr: true},
		{name: "idle cancel invalid", state: StateIdle, event: EventCancel, want: StateIdle, wantErr: true},
		{name: "recording start invalid", state: StateRecording, event: EventStart, want: StateRecording, wantErr: true},
		{name: "recording transcribed invalid", state: StateRecording, event: EventTranscribed, want: StateRecording, wantErr: true},
		{name: "transcribing stop invalid", state: StateTranscribing, event: EventStop, want: StateTranscribing, wantErr: true},
		{name: "transcribing cancel invalid", state: StateTranscribing, event: EventCancel, want: StateTranscribing, wantErr: true},
		{name: "error start invalid", state: StateError, event: EventStart, want: StateError, wantErr: true},
		{name: "error stop invalid", state: StateError, event: EventStop, want: StateError, wantErr: true},
		{name: "error reset valid", state: StateError, event: EventReset, want: StateIdle, wantErr: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			next, err := Transition(tc.state, tc.event)
			require.Equal(t, tc.want, next)
			if tc.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), "invalid transition")
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestTransitionUnknownState(t *testing.T) {
	next, err := Transition(State("mystery"), EventStart)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown state")
	require.Equal(t, State("mystery"), next)
}
