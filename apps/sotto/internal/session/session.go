// Package session coordinates dictation lifecycle state, actions, and commit flow.
package session

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/rbright/sotto/internal/fsm"
	"github.com/rbright/sotto/internal/ipc"
)

type action int

const (
	actionStop action = iota + 1
	actionCancel
)

// Result is the complete lifecycle output returned by one Run invocation.
type Result struct {
	State          fsm.State
	Transcript     string
	Cancelled      bool
	Err            error
	AudioDevice    string
	BytesCaptured  int64
	GRPCLatency    time.Duration
	StartedAt      time.Time
	FinishedAt     time.Time
	FocusedMonitor string
}

// Indicator is the session-facing subset of indicator behavior.
type Indicator interface {
	ShowRecording(context.Context)
	ShowTranscribing(context.Context)
	ShowError(context.Context, string)
	CueStop(context.Context)
	CueComplete(context.Context)
	CueCancel(context.Context)
	Hide(context.Context)
	FocusedMonitor() string
}

// noopIndicator preserves session flow when no indicator is wired.
type noopIndicator struct{}

func (noopIndicator) ShowRecording(context.Context)     {}
func (noopIndicator) ShowTranscribing(context.Context)  {}
func (noopIndicator) ShowError(context.Context, string) {}
func (noopIndicator) CueStop(context.Context)           {}
func (noopIndicator) CueComplete(context.Context)       {}
func (noopIndicator) CueCancel(context.Context)         {}
func (noopIndicator) Hide(context.Context)              {}
func (noopIndicator) FocusedMonitor() string            { return "" }

// Controller orchestrates session state transitions and side effects.
type Controller struct {
	logger     *slog.Logger
	transcribe Transcriber
	commit     Committer
	indicator  Indicator

	mu    sync.RWMutex
	state fsm.State

	actions chan action
}

// NewController constructs a session controller with safe default fallbacks.
func NewController(
	logger *slog.Logger,
	transcriber Transcriber,
	committer Committer,
	indicator Indicator,
) *Controller {
	if transcriber == nil {
		transcriber = PlaceholderTranscriber{}
	}
	if committer == nil {
		committer = CommitFunc(func(context.Context, string) error { return nil })
	}
	if indicator == nil {
		indicator = noopIndicator{}
	}

	return &Controller{
		logger:     logger,
		transcribe: transcriber,
		commit:     committer,
		indicator:  indicator,
		state:      fsm.StateIdle,
		actions:    make(chan action, 1),
	}
}

// State returns the current FSM state snapshot.
func (c *Controller) State() fsm.State {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// transition applies one FSM event to the controller state.
func (c *Controller) transition(event fsm.Event) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	next, err := fsm.Transition(c.state, event)
	if err != nil {
		return err
	}
	c.state = next
	return nil
}

// Run executes one owner lifecycle from start to stop/cancel/failure completion.
func (c *Controller) Run(ctx context.Context) Result {
	result := Result{StartedAt: time.Now()}

	if err := c.transition(fsm.EventStart); err != nil {
		result.State = c.State()
		result.Err = err
		result.FinishedAt = time.Now()
		return result
	}

	c.indicator.ShowRecording(ctx)

	if err := c.transcribe.Start(ctx); err != nil {
		c.indicator.ShowError(ctx, "Unable to start recording")
		c.toErrorAndReset()
		result.State = c.State()
		result.Err = err
		result.FinishedAt = time.Now()
		result.FocusedMonitor = c.indicator.FocusedMonitor()
		return result
	}

	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
		defer cancel()
		c.indicator.Hide(cleanupCtx)
	}()

	select {
	case <-ctx.Done():
		_ = c.transcribe.Cancel(context.Background())
		c.indicator.CueCancel(context.Background())
		c.indicator.ShowError(context.Background(), "Cancelled")
		c.toErrorAndReset()
		result.State = c.State()
		result.Err = ctx.Err()
		result.FinishedAt = time.Now()
		result.FocusedMonitor = c.indicator.FocusedMonitor()
		return result
	case a := <-c.actions:
		switch a {
		case actionCancel:
			_ = c.transcribe.Cancel(context.Background())
			c.indicator.CueCancel(context.Background())
			_ = c.transition(fsm.EventCancel)
			result.State = c.State()
			result.Cancelled = true
			result.FinishedAt = time.Now()
			result.FocusedMonitor = c.indicator.FocusedMonitor()
			return result
		case actionStop:
			if err := c.transition(fsm.EventStop); err != nil {
				c.toErrorAndReset()
				result.State = c.State()
				result.Err = err
				result.FinishedAt = time.Now()
				result.FocusedMonitor = c.indicator.FocusedMonitor()
				return result
			}
			c.indicator.ShowTranscribing(ctx)

			stopResult, err := c.transcribe.StopAndTranscribe(ctx)
			c.indicator.CueStop(context.Background())
			if err != nil {
				c.indicator.ShowError(context.Background(), "Speech recognition failed")
				c.toErrorAndReset()
				result.State = c.State()
				result.Err = err
				result.BytesCaptured = stopResult.BytesCaptured
				result.AudioDevice = stopResult.AudioDevice
				result.GRPCLatency = stopResult.GRPCLatency
				result.FinishedAt = time.Now()
				result.FocusedMonitor = c.indicator.FocusedMonitor()
				return result
			}

			if strings.TrimSpace(stopResult.Transcript) == "" {
				c.indicator.ShowError(context.Background(), "No speech detected")
				c.toErrorAndReset()
				result.State = c.State()
				result.Err = ErrEmptyTranscript
				result.Transcript = stopResult.Transcript
				result.AudioDevice = stopResult.AudioDevice
				result.BytesCaptured = stopResult.BytesCaptured
				result.GRPCLatency = stopResult.GRPCLatency
				result.FinishedAt = time.Now()
				result.FocusedMonitor = c.indicator.FocusedMonitor()
				return result
			}

			if err := c.commit.Commit(ctx, stopResult.Transcript); err != nil {
				c.indicator.ShowError(context.Background(), "Output dispatch failed")
				c.toErrorAndReset()
				result.State = c.State()
				result.Err = err
				result.Transcript = stopResult.Transcript
				result.AudioDevice = stopResult.AudioDevice
				result.BytesCaptured = stopResult.BytesCaptured
				result.GRPCLatency = stopResult.GRPCLatency
				result.FinishedAt = time.Now()
				result.FocusedMonitor = c.indicator.FocusedMonitor()
				return result
			}
			c.indicator.CueComplete(context.Background())

			if err := c.transition(fsm.EventTranscribed); err != nil {
				result.State = c.State()
				result.Err = err
				result.Transcript = stopResult.Transcript
				result.AudioDevice = stopResult.AudioDevice
				result.BytesCaptured = stopResult.BytesCaptured
				result.GRPCLatency = stopResult.GRPCLatency
				result.FinishedAt = time.Now()
				result.FocusedMonitor = c.indicator.FocusedMonitor()
				return result
			}

			result.State = c.State()
			result.Transcript = stopResult.Transcript
			result.AudioDevice = stopResult.AudioDevice
			result.BytesCaptured = stopResult.BytesCaptured
			result.GRPCLatency = stopResult.GRPCLatency
			result.FinishedAt = time.Now()
			result.FocusedMonitor = c.indicator.FocusedMonitor()
			return result
		default:
			c.toErrorAndReset()
			result.State = c.State()
			result.Err = fmt.Errorf("unknown action %d", a)
			result.FinishedAt = time.Now()
			result.FocusedMonitor = c.indicator.FocusedMonitor()
			return result
		}
	}
}

// Handle serves IPC commands for the active owner session.
func (c *Controller) Handle(_ context.Context, req ipc.Request) ipc.Response {
	switch req.Command {
	case "status":
		return ipc.Response{OK: true, State: string(c.State()), Message: "status"}
	case "toggle":
		return c.requestStop("toggle")
	case "stop":
		return c.requestStop("stop")
	case "cancel":
		return c.requestCancel()
	default:
		return ipc.Response{OK: false, State: string(c.State()), Error: fmt.Sprintf("unknown command: %s", req.Command)}
	}
}

// requestStop enqueues a stop action when state permits it.
func (c *Controller) requestStop(source string) ipc.Response {
	state := c.State()
	if state == fsm.StateTranscribing {
		return ipc.Response{OK: false, State: string(state), Error: "already transcribing"}
	}
	if state != fsm.StateRecording {
		return ipc.Response{OK: false, State: string(state), Error: fmt.Sprintf("cannot %s from state %s", source, state)}
	}

	select {
	case c.actions <- actionStop:
		return ipc.Response{OK: true, State: string(state), Message: "stop requested"}
	default:
		return ipc.Response{OK: true, State: string(state), Message: "stop already requested"}
	}
}

// requestCancel enqueues a cancel action when state permits it.
func (c *Controller) requestCancel() ipc.Response {
	state := c.State()
	if state == fsm.StateTranscribing {
		return ipc.Response{OK: false, State: string(state), Error: "cannot cancel while transcribing"}
	}
	if state != fsm.StateRecording {
		return ipc.Response{OK: false, State: string(state), Error: fmt.Sprintf("cannot cancel from state %s", state)}
	}

	select {
	case c.actions <- actionCancel:
		return ipc.Response{OK: true, State: string(state), Message: "cancel requested"}
	default:
		return ipc.Response{OK: true, State: string(state), Message: "cancel already requested"}
	}
}

// toErrorAndReset transitions to error and back to idle best-effort.
func (c *Controller) toErrorAndReset() {
	_ = c.transition(fsm.EventFail)
	_ = c.transition(fsm.EventReset)
}

// IsPipelineUnavailable reports whether an error represents missing pipeline wiring.
func IsPipelineUnavailable(err error) bool {
	return errors.Is(err, ErrPipelineUnavailable)
}
