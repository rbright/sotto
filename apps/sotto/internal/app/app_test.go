package app

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/rbright/sotto/internal/fsm"
	"github.com/rbright/sotto/internal/ipc"
	"github.com/rbright/sotto/internal/session"
	"github.com/stretchr/testify/require"
)

func TestExecuteHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Execute(context.Background(), []string{"--help"}, &stdout, &stderr)
	require.Equal(t, 0, exitCode)
	require.Contains(t, stdout.String(), "Usage:")
	require.Empty(t, stderr.String())
}

func TestExecuteVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Execute(context.Background(), []string{"version"}, &stdout, &stderr)
	require.Equal(t, 0, exitCode)
	require.Contains(t, stdout.String(), "sotto")
	require.Empty(t, stderr.String())
}

func TestExecuteUnknownCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Execute(context.Background(), []string{"definitely-not-a-command"}, &stdout, &stderr)
	require.Equal(t, 2, exitCode)
	require.Contains(t, stderr.String(), "unknown command")
	require.Contains(t, stderr.String(), "Usage:")
}

func TestRunnerStatusIdleWhenSocketUnavailable(t *testing.T) {
	paths := setupRunnerEnv(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := Runner{Stdout: &stdout, Stderr: &stderr}

	exitCode := runner.Execute(context.Background(), []string{"--config", paths.configPath, "status"})
	require.Equal(t, 0, exitCode)
	require.Equal(t, "idle\n", stdout.String())
	require.Empty(t, stderr.String())
}

func TestRunnerStopReturnsNoActiveSession(t *testing.T) {
	paths := setupRunnerEnv(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := Runner{Stdout: &stdout, Stderr: &stderr}

	exitCode := runner.Execute(context.Background(), []string{"--config", paths.configPath, "stop"})
	require.Equal(t, 1, exitCode)
	require.Contains(t, stderr.String(), "no active sotto session")
}

func TestRunnerForwardsCommandsToActiveSession(t *testing.T) {
	paths := setupRunnerEnv(t)
	commands := make(chan string, 8)

	shutdown := startIPCServerForRunnerTest(t, filepath.Join(paths.runtimeDir, "sotto.sock"), func(_ context.Context, req ipc.Request) ipc.Response {
		commands <- req.Command
		switch req.Command {
		case "status":
			return ipc.Response{OK: true, State: "recording"}
		case "stop", "cancel", "toggle":
			return ipc.Response{OK: true, Message: req.Command + " handled"}
		default:
			return ipc.Response{OK: false, Error: "unsupported"}
		}
	})
	defer shutdown()

	runner := Runner{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}

	for _, cmd := range []string{"status", "stop", "cancel", "toggle"} {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		runner.Stdout = stdout
		runner.Stderr = stderr

		exitCode := runner.Execute(context.Background(), []string{"--config", paths.configPath, cmd})
		require.Equal(t, 0, exitCode, cmd)
		require.Empty(t, stderr.String(), cmd)
	}

	got := []string{<-commands, <-commands, <-commands, <-commands}
	require.ElementsMatch(t, []string{"status", "stop", "cancel", "toggle"}, got)
}

func TestTryForwardSuccessAndFailureResponses(t *testing.T) {
	runtimeDir := t.TempDir()
	socketPath := filepath.Join(runtimeDir, "sotto.sock")

	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	serverCtx, cancelServer := context.WithCancel(context.Background())
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- ipc.Serve(serverCtx, listener, ipc.HandlerFunc(func(_ context.Context, req ipc.Request) ipc.Response {
			switch req.Command {
			case "status":
				return ipc.Response{OK: true, State: "recording"}
			default:
				return ipc.Response{OK: false, Error: "unsupported"}
			}
		}))
	}()

	resp, handled, err := tryForward(context.Background(), socketPath, "status")
	require.True(t, handled)
	require.NoError(t, err)
	require.Equal(t, "recording", resp.State)

	_, handled, err = tryForward(context.Background(), socketPath, "cancel")
	require.True(t, handled)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported")

	cancelServer()
	require.NoError(t, <-serverDone)
}

func TestTryForwardDoesNotRemoveSocketPathOnForwardFailure(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "sotto.sock")
	require.NoError(t, os.WriteFile(socketPath, []byte("stale"), 0o600))

	_, handled, err := tryForward(context.Background(), socketPath, "status")
	require.False(t, handled)
	require.NoError(t, err)

	_, statErr := os.Stat(socketPath)
	require.NoError(t, statErr)
}

func TestTryForwardTreatsReadFailuresAsHandledErrors(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "sotto.sock")

	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, acceptErr := listener.Accept()
		if acceptErr == nil {
			_ = conn.Close()
		}
	}()

	_, handled, err := tryForward(context.Background(), socketPath, "status")
	require.True(t, handled)
	require.Error(t, err)
	require.Contains(t, err.Error(), "forward command \"status\":")

	<-done
	_, statErr := os.Stat(socketPath)
	require.NoError(t, statErr)
	require.NoError(t, listener.Close())
}

func TestRunnerDoctorCommandDispatchesAndPrintsReport(t *testing.T) {
	paths := setupRunnerEnv(t)
	t.Setenv("XDG_SESSION_TYPE", "x11")
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := Runner{Stdout: &stdout, Stderr: &stderr}

	exitCode := runner.Execute(context.Background(), []string{"--config", paths.configPath, "doctor"})
	require.Equal(t, 1, exitCode)
	require.Contains(t, stdout.String(), "config: loaded")
	require.Contains(t, stdout.String(), "XDG_SESSION_TYPE")
}

func TestRunnerDevicesCommandDispatches(t *testing.T) {
	paths := setupRunnerEnv(t)
	t.Setenv("PULSE_SERVER", "unix:/tmp/definitely-missing-pulse-server")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := Runner{Stdout: &stdout, Stderr: &stderr}

	exitCode := runner.Execute(context.Background(), []string{"--config", paths.configPath, "devices"})
	require.Equal(t, 1, exitCode)
	require.Contains(t, stderr.String(), "error:")
}

func TestRunnerToggleOwnerPathReturnsErrorWhenCaptureStartupFails(t *testing.T) {
	paths := setupRunnerEnv(t)
	t.Setenv("PULSE_SERVER", "unix:/tmp/definitely-missing-pulse-server")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := Runner{Stdout: &stdout, Stderr: &stderr}

	exitCode := runner.Execute(context.Background(), []string{"--config", paths.configPath, "toggle"})
	require.Equal(t, 1, exitCode)
	require.Contains(t, stderr.String(), "error:")

	// owner path should clean up runtime socket on exit
	_, statErr := os.Stat(filepath.Join(paths.runtimeDir, "sotto.sock"))
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestRunnerStatusFallsBackToIdleWhenServerStateEmpty(t *testing.T) {
	paths := setupRunnerEnv(t)

	shutdown := startIPCServerForRunnerTest(t, filepath.Join(paths.runtimeDir, "sotto.sock"), func(_ context.Context, req ipc.Request) ipc.Response {
		require.Equal(t, "status", req.Command)
		return ipc.Response{OK: true, State: ""}
	})
	defer shutdown()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := Runner{Stdout: &stdout, Stderr: &stderr}

	exitCode := runner.Execute(context.Background(), []string{"--config", paths.configPath, "status"})
	require.Equal(t, 0, exitCode)
	require.Equal(t, "idle\n", stdout.String())
	require.Empty(t, stderr.String())
}

func TestSocketErrorHelpers(t *testing.T) {
	require.False(t, isSocketMissing(nil))
	require.False(t, isConnectionRefused(nil))

	require.True(t, isSocketMissing(os.ErrNotExist))
	require.True(t, isSocketMissing(errors.New("dial unix /tmp/sotto.sock: no such file or directory")))
	require.False(t, isSocketMissing(errors.New("other error")))

	require.True(t, isConnectionRefused(syscall.ECONNREFUSED))
	require.False(t, isConnectionRefused(errors.New("other error")))
}

func TestLogSessionResultWritesFailureAndSuccess(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))

	started := time.Now()
	finished := started.Add(1500 * time.Millisecond)

	logSessionResult(logger, session.Result{
		State:         fsm.StateIdle,
		Cancelled:     false,
		StartedAt:     started,
		FinishedAt:    finished,
		AudioDevice:   "Mic",
		BytesCaptured: 123,
		Transcript:    "hello",
		GRPCLatency:   20 * time.Millisecond,
	})

	require.Contains(t, logBuf.String(), "session complete")
	require.Contains(t, logBuf.String(), "\"transcript_length\":5")

	logBuf.Reset()
	logSessionResult(logger, session.Result{
		State:       fsm.StateIdle,
		StartedAt:   started,
		FinishedAt:  finished,
		Transcript:  "",
		Err:         errors.New("boom"),
		GRPCLatency: 2 * time.Millisecond,
	})
	require.Contains(t, logBuf.String(), "session failed")
	require.Contains(t, logBuf.String(), "boom")
}

type runnerPaths struct {
	configPath string
	runtimeDir string
}

func setupRunnerEnv(t *testing.T) runnerPaths {
	t.Helper()

	xdgStateHome := t.TempDir()
	runtimeDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", xdgStateHome)
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)

	configPath := filepath.Join(t.TempDir(), "config.conf")
	require.NoError(t, os.WriteFile(configPath, []byte("\n"), 0o600))

	return runnerPaths{configPath: configPath, runtimeDir: runtimeDir}
}

func startIPCServerForRunnerTest(t *testing.T, socketPath string, handler func(context.Context, ipc.Request) ipc.Response) func() {
	t.Helper()

	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- ipc.Serve(ctx, listener, ipc.HandlerFunc(handler))
	}()

	return func() {
		cancel()
		require.NoError(t, <-done)
	}
}
