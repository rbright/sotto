package session

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rbright/sotto/internal/fsm"
	"github.com/rbright/sotto/internal/ipc"
)

type fakeIndicator struct {
	stopCues     atomic.Int32
	completeCues atomic.Int32
	cancelCues   atomic.Int32
}

func (*fakeIndicator) ShowRecording(context.Context)     {}
func (*fakeIndicator) ShowTranscribing(context.Context)  {}
func (*fakeIndicator) ShowError(context.Context, string) {}
func (f *fakeIndicator) CueStop(context.Context)         { f.stopCues.Add(1) }
func (f *fakeIndicator) CueComplete(context.Context)     { f.completeCues.Add(1) }
func (f *fakeIndicator) CueCancel(context.Context)       { f.cancelCues.Add(1) }
func (*fakeIndicator) Hide(context.Context)              {}
func (*fakeIndicator) FocusedMonitor() string            { return "DP-1" }

type fakeTranscriber struct {
	startErr    error
	transcript  string
	stopErr     error
	cancelCalls atomic.Int32
}

func (f *fakeTranscriber) Start(context.Context) error {
	return f.startErr
}

func (f *fakeTranscriber) StopAndTranscribe(context.Context) (StopResult, error) {
	return StopResult{
		Transcript:    f.transcript,
		AudioDevice:   "test mic",
		BytesCaptured: 3200,
		GRPCLatency:   200 * time.Millisecond,
	}, f.stopErr
}

func (f *fakeTranscriber) Cancel(context.Context) error {
	f.cancelCalls.Add(1)
	return nil
}

func TestControllerCancel(t *testing.T) {
	transcriber := &fakeTranscriber{}
	ind := &fakeIndicator{}
	ctrl := NewController(nil, transcriber, nil, ind)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultCh := make(chan Result, 1)
	go func() {
		resultCh <- ctrl.Run(ctx)
	}()

	waitForState(t, ctrl, fsm.StateRecording)
	resp := ctrl.Handle(ctx, ipc.Request{Command: "cancel"})
	if !resp.OK {
		t.Fatalf("cancel response not OK: %+v", resp)
	}

	result := <-resultCh
	if !result.Cancelled {
		t.Fatalf("expected cancelled result, got %+v", result)
	}
	if state := ctrl.State(); state != fsm.StateIdle {
		t.Fatalf("expected idle state after cancel, got %s", state)
	}
	if transcriber.cancelCalls.Load() == 0 {
		t.Fatalf("expected cancel to propagate to transcriber")
	}
	if ind.cancelCues.Load() == 0 {
		t.Fatalf("expected cancel cue to play")
	}
	if ind.stopCues.Load() != 0 {
		t.Fatalf("expected no stop cue on cancel")
	}
	if ind.completeCues.Load() != 0 {
		t.Fatalf("expected no complete cue on cancel")
	}
}

func TestControllerStopCommitsTranscript(t *testing.T) {
	var committed atomic.Bool
	ind := &fakeIndicator{}
	ctrl := NewController(
		nil,
		&fakeTranscriber{transcript: "hello world"},
		CommitFunc(func(context.Context, string) error {
			committed.Store(true)
			return nil
		}),
		ind,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultCh := make(chan Result, 1)
	go func() {
		resultCh <- ctrl.Run(ctx)
	}()

	waitForState(t, ctrl, fsm.StateRecording)
	resp := ctrl.Handle(ctx, ipc.Request{Command: "stop"})
	if !resp.OK {
		t.Fatalf("stop response not OK: %+v", resp)
	}

	result := <-resultCh
	if result.Err != nil {
		t.Fatalf("unexpected result error: %v", result.Err)
	}
	if result.Transcript != "hello world" {
		t.Fatalf("unexpected transcript: %q", result.Transcript)
	}
	if result.AudioDevice != "test mic" {
		t.Fatalf("unexpected audio device: %q", result.AudioDevice)
	}
	if result.BytesCaptured != 3200 {
		t.Fatalf("unexpected bytes captured: %d", result.BytesCaptured)
	}
	if !committed.Load() {
		t.Fatalf("expected committer to run")
	}
	if ind.stopCues.Load() == 0 {
		t.Fatalf("expected stop cue to play")
	}
	if ind.cancelCues.Load() != 0 {
		t.Fatalf("expected no cancel cue on stop")
	}
	if ind.completeCues.Load() == 0 {
		t.Fatalf("expected complete cue on successful commit")
	}
}

func TestControllerStopPipelineError(t *testing.T) {
	ind := &fakeIndicator{}
	ctrl := NewController(nil, &fakeTranscriber{stopErr: ErrPipelineUnavailable}, nil, ind)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultCh := make(chan Result, 1)
	go func() {
		resultCh <- ctrl.Run(ctx)
	}()

	waitForState(t, ctrl, fsm.StateRecording)
	resp := ctrl.Handle(ctx, ipc.Request{Command: "toggle"})
	if !resp.OK {
		t.Fatalf("toggle response not OK: %+v", resp)
	}

	result := <-resultCh
	if !errors.Is(result.Err, ErrPipelineUnavailable) {
		t.Fatalf("unexpected result error: %v", result.Err)
	}
	if state := ctrl.State(); state != fsm.StateIdle {
		t.Fatalf("expected idle after error reset, got %s", state)
	}
	if ind.stopCues.Load() == 0 {
		t.Fatalf("expected stop cue even on pipeline error")
	}
	if ind.completeCues.Load() != 0 {
		t.Fatalf("did not expect complete cue when stop fails")
	}
}

func TestControllerStopEmptyTranscriptReturnsError(t *testing.T) {
	var committed atomic.Bool
	ind := &fakeIndicator{}
	ctrl := NewController(
		nil,
		&fakeTranscriber{transcript: ""},
		CommitFunc(func(context.Context, string) error {
			committed.Store(true)
			return nil
		}),
		ind,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultCh := make(chan Result, 1)
	go func() {
		resultCh <- ctrl.Run(ctx)
	}()

	waitForState(t, ctrl, fsm.StateRecording)
	resp := ctrl.Handle(ctx, ipc.Request{Command: "stop"})
	if !resp.OK {
		t.Fatalf("stop response not OK: %+v", resp)
	}

	result := <-resultCh
	if !errors.Is(result.Err, ErrEmptyTranscript) {
		t.Fatalf("unexpected result error: %v", result.Err)
	}
	if committed.Load() {
		t.Fatalf("expected committer not to run for empty transcript")
	}
	if state := ctrl.State(); state != fsm.StateIdle {
		t.Fatalf("expected idle after empty transcript error reset, got %s", state)
	}
	if ind.stopCues.Load() == 0 {
		t.Fatalf("expected stop cue on empty transcript")
	}
	if ind.completeCues.Load() != 0 {
		t.Fatalf("did not expect complete cue on empty transcript")
	}
}

func waitForState(t *testing.T, ctrl *Controller, desired fsm.State) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if ctrl.State() == desired {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for state %s (current=%s)", desired, ctrl.State())
}
