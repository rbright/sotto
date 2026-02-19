package ipc

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSendRoundTrip(t *testing.T) {
	runtimeDir := t.TempDir()
	socketPath := filepath.Join(runtimeDir, "sotto.sock")

	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serveDone := make(chan error, 1)
	go func() {
		serveDone <- Serve(ctx, listener, HandlerFunc(func(_ context.Context, req Request) Response {
			require.Equal(t, "status", req.Command)
			return Response{OK: true, State: "recording", Message: "ok"}
		}))
	}()

	resp, err := Send(context.Background(), socketPath, Request{Command: "status"}, 200*time.Millisecond)
	require.NoError(t, err)
	require.True(t, resp.OK)
	require.Equal(t, "recording", resp.State)
	require.Equal(t, "ok", resp.Message)

	cancel()
	require.NoError(t, <-serveDone)
}

func TestSendDecodeResponseError(t *testing.T) {
	runtimeDir := t.TempDir()
	socketPath := filepath.Join(runtimeDir, "sotto.sock")

	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()

		reader := bufio.NewReader(conn)
		_, _ = reader.ReadBytes('\n')
		_, _ = conn.Write([]byte("not-json\n"))
	}()

	_, err = Send(context.Background(), socketPath, Request{Command: "status"}, 200*time.Millisecond)
	require.Error(t, err)
	require.Contains(t, err.Error(), "decode response")
}

func TestSendReadResponseError(t *testing.T) {
	runtimeDir := t.TempDir()
	socketPath := filepath.Join(runtimeDir, "sotto.sock")

	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		_ = conn.Close()
	}()

	_, err = Send(context.Background(), socketPath, Request{Command: "status"}, 200*time.Millisecond)
	require.Error(t, err)
	require.Contains(t, err.Error(), "read response")
}

func TestServeDecodeRequestErrorResponse(t *testing.T) {
	runtimeDir := t.TempDir()
	socketPath := filepath.Join(runtimeDir, "sotto.sock")

	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- Serve(ctx, listener, HandlerFunc(func(_ context.Context, _ Request) Response {
			return Response{OK: true}
		}))
	}()

	conn, err := net.Dial("unix", socketPath)
	require.NoError(t, err)
	defer conn.Close()

	_, err = conn.Write([]byte("not-json\n"))
	require.NoError(t, err)

	line, err := bufio.NewReader(conn).ReadBytes('\n')
	require.NoError(t, err)

	var resp Response
	require.NoError(t, json.Unmarshal(line, &resp))
	require.False(t, resp.OK)
	require.Contains(t, resp.Error, "decode request")

	cancel()
	require.NoError(t, <-serveDone)
}

func TestProbe(t *testing.T) {
	runtimeDir := t.TempDir()
	socketPath := filepath.Join(runtimeDir, "sotto.sock")

	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- Serve(ctx, listener, HandlerFunc(func(_ context.Context, req Request) Response {
			if req.Command == "status" {
				return Response{OK: true, State: "idle"}
			}
			return Response{OK: false, Error: "bad"}
		}))
	}()

	alive, probeErr := Probe(context.Background(), socketPath, 200*time.Millisecond)
	require.NoError(t, probeErr)
	require.True(t, alive)

	cancel()
	require.NoError(t, <-serveDone)

	alive, probeErr = Probe(context.Background(), socketPath, 100*time.Millisecond)
	require.NoError(t, probeErr)
	require.False(t, alive)
}
