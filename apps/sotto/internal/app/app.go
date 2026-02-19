package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/rbright/sotto/internal/audio"
	"github.com/rbright/sotto/internal/cli"
	"github.com/rbright/sotto/internal/config"
	"github.com/rbright/sotto/internal/doctor"
	"github.com/rbright/sotto/internal/indicator"
	"github.com/rbright/sotto/internal/ipc"
	"github.com/rbright/sotto/internal/logging"
	"github.com/rbright/sotto/internal/output"
	"github.com/rbright/sotto/internal/pipeline"
	"github.com/rbright/sotto/internal/session"
	"github.com/rbright/sotto/internal/version"
)

// Runner holds process-level dependencies used by command handlers.
type Runner struct {
	Stdout io.Writer
	Stderr io.Writer
	Logger *slog.Logger
}

// Execute is the package entrypoint used by cmd/sotto/main.go.
func Execute(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	r := Runner{Stdout: stdout, Stderr: stderr}
	return r.Execute(ctx, args)
}

// Execute parses CLI arguments, loads config/logging, and dispatches a command.
func (r Runner) Execute(ctx context.Context, args []string) int {
	parsed, err := cli.Parse(args)
	if err != nil {
		fmt.Fprintf(r.Stderr, "error: %v\n\n", err)
		fmt.Fprint(r.Stderr, cli.HelpText("sotto"))
		return 2
	}

	if parsed.ShowHelp {
		fmt.Fprint(r.Stdout, cli.HelpText("sotto"))
		return 0
	}

	if parsed.Command == cli.CommandVersion {
		fmt.Fprintln(r.Stdout, version.String())
		return 0
	}

	logRuntime, err := logging.New()
	if err != nil {
		fmt.Fprintf(r.Stderr, "error: setup logging: %v\n", err)
		return 1
	}
	defer func() { _ = logRuntime.Close() }()

	logger := r.Logger
	if logger == nil {
		logger = logRuntime.Logger
	}

	cfgLoaded, err := config.Load(parsed.ConfigPath)
	if err != nil {
		fmt.Fprintf(r.Stderr, "error: %v\n", err)
		logger.Error("load config failed", "error", err.Error())
		return 1
	}
	for _, w := range cfgLoaded.Warnings {
		msg := w.Message
		if w.Line > 0 {
			msg = fmt.Sprintf("line %d: %s", w.Line, w.Message)
		}
		fmt.Fprintf(r.Stderr, "warning: %s\n", msg)
		logger.Warn("config warning", "line", w.Line, "message", w.Message)
	}

	if speechPlan, _, err := config.BuildSpeechPhrases(cfgLoaded.Config); err == nil {
		logger.Debug("speech context plan", "phrase_count", len(speechPlan), "phrases", speechPlan)
	}

	logger.Info("command start",
		"command", parsed.Command,
		"config", cfgLoaded.Path,
		"log", logRuntime.Path,
	)

	switch parsed.Command {
	case cli.CommandDoctor:
		report := doctor.Run(cfgLoaded)
		fmt.Fprintln(r.Stdout, report.String())
		if report.OK() {
			return 0
		}
		return 1
	case cli.CommandDevices:
		return r.commandDevices(ctx)
	case cli.CommandStatus:
		return r.commandStatus(ctx)
	case cli.CommandStop:
		return r.forwardOrFail(ctx, "stop")
	case cli.CommandCancel:
		return r.forwardOrFail(ctx, "cancel")
	case cli.CommandToggle:
		return r.commandToggle(ctx, cfgLoaded.Config, logger)
	default:
		fmt.Fprintf(r.Stderr, "error: unsupported command %q\n", parsed.Command)
		return 2
	}
}

// commandDevices prints discovered input devices and key availability metadata.
func (r Runner) commandDevices(ctx context.Context) int {
	devices, err := audio.ListDevices(ctx)
	if err != nil {
		fmt.Fprintf(r.Stderr, "error: %v\n", err)
		return 1
	}
	if len(devices) == 0 {
		fmt.Fprintln(r.Stdout, "no audio devices found")
		return 1
	}

	for _, device := range devices {
		defaultMark := " "
		if device.Default {
			defaultMark = "*"
		}
		availability := "yes"
		if !device.Available {
			availability = "no"
		}
		muted := "no"
		if device.Muted {
			muted = "yes"
		}
		fmt.Fprintf(
			r.Stdout,
			"%s id=%s | description=%q | state=%s | available=%s | muted=%s\n",
			defaultMark,
			device.ID,
			device.Description,
			device.State,
			availability,
			muted,
		)
	}

	return 0
}

// commandStatus queries the active owner (if any) and prints session state.
func (r Runner) commandStatus(ctx context.Context) int {
	socketPath, err := ipc.RuntimeSocketPath()
	if err != nil {
		fmt.Fprintln(r.Stdout, "idle")
		return 0
	}

	resp, handled, err := tryForward(ctx, socketPath, "status")
	if handled {
		if err != nil {
			fmt.Fprintf(r.Stderr, "error: %v\n", err)
			return 1
		}
		if resp.State == "" {
			resp.State = "idle"
		}
		fmt.Fprintln(r.Stdout, resp.State)
		return 0
	}

	fmt.Fprintln(r.Stdout, "idle")
	return 0
}

// forwardOrFail forwards a command to the active owner and fails when no owner exists.
func (r Runner) forwardOrFail(ctx context.Context, command string) int {
	socketPath, err := ipc.RuntimeSocketPath()
	if err != nil {
		fmt.Fprintf(r.Stderr, "error: %v\n", err)
		return 1
	}

	resp, handled, err := tryForward(ctx, socketPath, command)
	if !handled {
		fmt.Fprintf(r.Stderr, "error: no active sotto session\n")
		return 1
	}
	if err != nil {
		fmt.Fprintf(r.Stderr, "error: %v\n", err)
		return 1
	}
	if resp.Message != "" {
		fmt.Fprintln(r.Stdout, resp.Message)
	}
	return 0
}

// commandToggle starts a new owner session or forwards toggle to an existing owner.
func (r Runner) commandToggle(ctx context.Context, cfg config.Config, logger *slog.Logger) int {
	socketPath, err := ipc.RuntimeSocketPath()
	if err != nil {
		fmt.Fprintf(r.Stderr, "error: %v\n", err)
		return 1
	}

	resp, handled, err := tryForward(ctx, socketPath, "toggle")
	if handled {
		if err != nil {
			fmt.Fprintf(r.Stderr, "error: %v\n", err)
			return 1
		}
		if resp.Message != "" {
			fmt.Fprintln(r.Stdout, resp.Message)
		}
		return 0
	}

	listener, err := ipc.Acquire(ctx, socketPath, 180*time.Millisecond, 8, nil)
	if err != nil {
		if errors.Is(err, ipc.ErrAlreadyRunning) {
			resp, _, forwardErr := tryForward(ctx, socketPath, "toggle")
			if forwardErr != nil {
				fmt.Fprintf(r.Stderr, "error: %v\n", forwardErr)
				return 1
			}
			if resp.Message != "" {
				fmt.Fprintln(r.Stdout, resp.Message)
			}
			return 0
		}
		fmt.Fprintf(r.Stderr, "error: %v\n", err)
		return 1
	}
	defer func() {
		_ = listener.Close()
		_ = os.Remove(socketPath)
	}()

	transcriber := pipeline.NewTranscriber(cfg, logger)
	committer := output.NewCommitter(cfg, logger)
	indicatorCtl := indicator.NewHyprNotify(cfg.Indicator, logger)
	controller := session.NewController(logger, transcriber, committer, indicatorCtl)

	serverCtx, serverCancel := context.WithCancel(ctx)
	defer serverCancel()

	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- ipc.Serve(serverCtx, listener, controller)
	}()

	result := controller.Run(ctx)
	serverCancel()
	if serverErr := <-serverErrCh; serverErr != nil {
		fmt.Fprintf(r.Stderr, "error: ipc server failed: %v\n", serverErr)
		return 1
	}

	logSessionResult(logger, result)

	if result.Cancelled {
		fmt.Fprintln(r.Stdout, "cancelled")
		return 0
	}
	if result.Err != nil {
		fmt.Fprintf(r.Stderr, "error: %v\n", result.Err)
		return 1
	}
	if strings.TrimSpace(result.Transcript) != "" {
		fmt.Fprintln(r.Stdout, strings.TrimSpace(result.Transcript))
	}

	return 0
}

// logSessionResult writes normalized session metrics into the runtime logger.
func logSessionResult(logger *slog.Logger, result session.Result) {
	if logger == nil {
		return
	}
	fields := []any{
		"state", result.State,
		"cancelled", result.Cancelled,
		"started_at", result.StartedAt.Format(time.RFC3339Nano),
		"finished_at", result.FinishedAt.Format(time.RFC3339Nano),
		"duration_ms", result.FinishedAt.Sub(result.StartedAt).Milliseconds(),
		"audio_device", result.AudioDevice,
		"bytes_captured", result.BytesCaptured,
		"transcript_length", len(result.Transcript),
		"grpc_latency_ms", result.GRPCLatency.Milliseconds(),
		"focused_monitor", result.FocusedMonitor,
	}

	if result.Err != nil {
		logger.Error("session failed", append(fields, "error", result.Err.Error())...)
		return
	}
	logger.Info("session complete", fields...)
}

// tryForward attempts to send a command to an existing owner and classifies outcome.
//
// handled=false means there was no active owner to handle the request.
func tryForward(ctx context.Context, socketPath string, command string) (ipc.Response, bool, error) {
	resp, err := ipc.Send(ctx, socketPath, ipc.Request{Command: command}, 220*time.Millisecond)
	if err == nil {
		if resp.OK {
			return resp, true, nil
		}
		return resp, true, errors.New(resp.Error)
	}

	if isSocketMissing(err) {
		return ipc.Response{}, false, nil
	}
	if isConnectionRefused(err) {
		return ipc.Response{}, false, nil
	}

	return ipc.Response{}, true, fmt.Errorf("forward command %q: %w", command, err)
}

// isSocketMissing reports whether forwarding failed because the owner socket is absent.
func isSocketMissing(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, os.ErrNotExist) ||
		strings.Contains(err.Error(), "no such file or directory")
}

// isConnectionRefused reports whether forwarding failed because no owner is listening.
func isConnectionRefused(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, syscall.ECONNREFUSED)
}
