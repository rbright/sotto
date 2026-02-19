package ipc

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAcquireRecoversStaleSocket(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	socketPath := filepath.Join(dir, "sotto.sock")
	if err := os.WriteFile(socketPath, []byte("stale"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	rescueCalls := 0
	listener, err := Acquire(context.Background(), socketPath, 50*time.Millisecond, 2, func(context.Context) error {
		rescueCalls++
		return nil
	})
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	defer listener.Close()

	if rescueCalls == 0 {
		t.Fatalf("expected stale-socket rescue to run")
	}
}

func TestAcquireReturnsAlreadyRunningWhenSocketResponsive(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	socketPath := filepath.Join(dir, "sotto.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	defer listener.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverDone := make(chan error, 1)
	go func() {
		serverDone <- Serve(ctx, listener, HandlerFunc(func(_ context.Context, _ Request) Response {
			return Response{OK: true, State: "recording"}
		}))
	}()

	_, err = Acquire(context.Background(), socketPath, 80*time.Millisecond, 1, nil)
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("Acquire() error = %v, want ErrAlreadyRunning", err)
	}

	cancel()
	if serveErr := <-serverDone; serveErr != nil {
		t.Fatalf("Serve() error = %v", serveErr)
	}
}

func TestAcquireDoesNotUnlinkWhenProbeInconclusive(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	socketPath := filepath.Join(dir, "sotto.sock")

	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	acceptDone := make(chan struct{})
	go func() {
		defer close(acceptDone)
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				time.Sleep(250 * time.Millisecond)
			}(conn)
		}
	}()

	_, err = Acquire(context.Background(), socketPath, 30*time.Millisecond, 0, nil)
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrAlreadyRunning)
	require.Contains(t, err.Error(), "probe existing socket")

	_, statErr := os.Stat(socketPath)
	require.NoError(t, statErr)
	require.NoError(t, listener.Close())
	<-acceptDone
}

func TestRuntimeSocketPathRequiresXDG(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	_, err := RuntimeSocketPath()
	if err == nil {
		t.Fatal("expected error")
	}
}
