package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rbright/sotto/internal/fsm"
	"github.com/rbright/sotto/internal/ipc"
	"github.com/stretchr/testify/require"
)

func TestHandleStatusAndUnknownCommand(t *testing.T) {
	ctrl := NewController(nil, &fakeTranscriber{}, nil, &fakeIndicator{})

	status := ctrl.Handle(context.Background(), ipc.Request{Command: "status"})
	require.True(t, status.OK)
	require.Equal(t, string(fsm.StateIdle), status.State)

	unknown := ctrl.Handle(context.Background(), ipc.Request{Command: "definitely-unknown"})
	require.False(t, unknown.OK)
	require.Contains(t, unknown.Error, "unknown command")
}

func TestRequestStopAndCancelStateGuards(t *testing.T) {
	ctrl := NewController(nil, &fakeTranscriber{}, nil, &fakeIndicator{})

	stopFromIdle := ctrl.Handle(context.Background(), ipc.Request{Command: "stop"})
	require.False(t, stopFromIdle.OK)
	require.Contains(t, stopFromIdle.Error, "cannot stop from state idle")

	cancelFromIdle := ctrl.Handle(context.Background(), ipc.Request{Command: "cancel"})
	require.False(t, cancelFromIdle.OK)
	require.Contains(t, cancelFromIdle.Error, "cannot cancel from state idle")

	ctrl.mu.Lock()
	ctrl.state = fsm.StateTranscribing
	ctrl.mu.Unlock()

	stopFromTranscribing := ctrl.Handle(context.Background(), ipc.Request{Command: "stop"})
	require.False(t, stopFromTranscribing.OK)
	require.Contains(t, stopFromTranscribing.Error, "already transcribing")

	cancelFromTranscribing := ctrl.Handle(context.Background(), ipc.Request{Command: "cancel"})
	require.False(t, cancelFromTranscribing.OK)
	require.Contains(t, cancelFromTranscribing.Error, "cannot cancel while transcribing")
}

func TestRequestStopAndCancelAlreadyRequested(t *testing.T) {
	ctrl := NewController(nil, &fakeTranscriber{}, nil, &fakeIndicator{})

	ctrl.mu.Lock()
	ctrl.state = fsm.StateRecording
	ctrl.mu.Unlock()

	ctrl.actions <- actionStop
	stop := ctrl.requestStop("stop")
	require.True(t, stop.OK)
	require.Equal(t, "stop already requested", stop.Message)

	<-ctrl.actions
	ctrl.actions <- actionCancel
	cancel := ctrl.requestCancel()
	require.True(t, cancel.OK)
	require.Equal(t, "cancel already requested", cancel.Message)
}

func TestRunStartFailure(t *testing.T) {
	transcriber := &fakeTranscriber{startErr: errors.New("start failed")}
	indicator := &fakeIndicator{}
	ctrl := NewController(nil, transcriber, nil, indicator)

	result := ctrl.Run(context.Background())
	require.Error(t, result.Err)
	require.Equal(t, fsm.StateIdle, result.State)
	require.NotZero(t, result.FinishedAt)
	require.Equal(t, int32(0), indicator.stopCues.Load())
	require.Equal(t, int32(0), indicator.completeCues.Load())
}

func TestRunCommitFailure(t *testing.T) {
	indicator := &fakeIndicator{}
	ctrl := NewController(
		nil,
		&fakeTranscriber{transcript: "hello world"},
		CommitFunc(func(context.Context, string) error { return errors.New("commit failed") }),
		indicator,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultCh := make(chan Result, 1)
	go func() {
		resultCh <- ctrl.Run(ctx)
	}()

	waitForState(t, ctrl, fsm.StateRecording)
	resp := ctrl.Handle(ctx, ipc.Request{Command: "stop"})
	require.True(t, resp.OK)

	result := <-resultCh
	require.Error(t, result.Err)
	require.Contains(t, result.Err.Error(), "commit failed")
	require.Equal(t, int32(1), indicator.stopCues.Load())
	require.Equal(t, int32(0), indicator.completeCues.Load())
}

func TestRunContextCancelled(t *testing.T) {
	indicator := &fakeIndicator{}
	ctrl := NewController(nil, &fakeTranscriber{}, nil, indicator)

	ctx, cancel := context.WithCancel(context.Background())
	resultCh := make(chan Result, 1)
	go func() {
		resultCh <- ctrl.Run(ctx)
	}()

	waitForState(t, ctrl, fsm.StateRecording)
	cancel()

	result := <-resultCh
	require.ErrorIs(t, result.Err, context.Canceled)
	require.Equal(t, fsm.StateIdle, result.State)
	require.Equal(t, int32(1), indicator.cancelCues.Load())
	require.False(t, result.Cancelled)
}

func TestRunUnknownAction(t *testing.T) {
	ctrl := NewController(nil, &fakeTranscriber{}, nil, &fakeIndicator{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultCh := make(chan Result, 1)
	go func() {
		resultCh <- ctrl.Run(ctx)
	}()

	waitForState(t, ctrl, fsm.StateRecording)
	ctrl.actions <- action(99)

	result := <-resultCh
	require.Error(t, result.Err)
	require.Contains(t, result.Err.Error(), "unknown action")
	require.Equal(t, fsm.StateIdle, result.State)
}

func TestIsPipelineUnavailable(t *testing.T) {
	require.True(t, IsPipelineUnavailable(ErrPipelineUnavailable))
	require.False(t, IsPipelineUnavailable(errors.New("different error")))
	require.False(t, IsPipelineUnavailable(nil))
}

func TestPlaceholderTranscriberContract(t *testing.T) {
	p := PlaceholderTranscriber{}
	require.NoError(t, p.Start(context.Background()))

	result, err := p.StopAndTranscribe(context.Background())
	require.ErrorIs(t, err, ErrPipelineUnavailable)
	require.Equal(t, StopResult{}, result)

	require.NoError(t, p.Cancel(context.Background()))
}

func TestCommitFuncDelegates(t *testing.T) {
	called := false
	commit := CommitFunc(func(_ context.Context, transcript string) error {
		called = true
		require.Equal(t, "hello", transcript)
		return nil
	})

	require.NoError(t, commit.Commit(context.Background(), "hello"))
	require.True(t, called)
}

func TestResultTimestampsAdvance(t *testing.T) {
	ctrl := NewController(nil, &fakeTranscriber{transcript: "ok"}, nil, &fakeIndicator{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultCh := make(chan Result, 1)
	go func() {
		resultCh <- ctrl.Run(ctx)
	}()

	waitForState(t, ctrl, fsm.StateRecording)
	require.True(t, ctrl.Handle(ctx, ipc.Request{Command: "stop"}).OK)
	result := <-resultCh

	require.False(t, result.StartedAt.IsZero())
	require.False(t, result.FinishedAt.IsZero())
	require.True(t, result.FinishedAt.After(result.StartedAt) || result.FinishedAt.Equal(result.StartedAt))
	require.LessOrEqual(t, result.FinishedAt.Sub(result.StartedAt), 2*time.Second)
}
