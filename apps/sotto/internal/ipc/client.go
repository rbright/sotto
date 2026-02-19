package ipc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"syscall"
	"time"
)

// Send opens a unix-socket request/response roundtrip with a deadline.
func Send(ctx context.Context, path string, req Request, timeout time.Duration) (Response, error) {
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "unix", path)
	if err != nil {
		return Response{}, err
	}
	defer conn.Close()

	deadline := time.Now().Add(timeout)
	if err := conn.SetDeadline(deadline); err != nil {
		return Response{}, fmt.Errorf("set deadline: %w", err)
	}

	enc := json.NewEncoder(conn)
	if err := enc.Encode(req); err != nil {
		return Response{}, fmt.Errorf("encode request: %w", err)
	}

	reader := bufio.NewReader(conn)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return Response{}, fmt.Errorf("read response: %w", err)
	}

	var resp Response
	if err := json.Unmarshal(line, &resp); err != nil {
		return Response{}, fmt.Errorf("decode response: %w", err)
	}

	return resp, nil
}

// Probe checks whether a responsive owner is currently listening on path.
func Probe(ctx context.Context, path string, timeout time.Duration) (bool, error) {
	_, err := Send(ctx, path, Request{Command: "status"}, timeout)
	if err == nil {
		return true, nil
	}
	if isSocketMissing(err) || isConnectionRefused(err) {
		return false, nil
	}
	return false, fmt.Errorf("probe socket: %w", err)
}

// isSocketMissing reports absent-socket failures.
func isSocketMissing(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, os.ErrNotExist)
}

// isConnectionRefused reports no-listener failures.
func isConnectionRefused(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, syscall.ECONNREFUSED)
}
