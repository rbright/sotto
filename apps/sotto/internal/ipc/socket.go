package ipc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var ErrAlreadyRunning = errors.New("sotto session already running")

func RuntimeSocketPath() (string, error) {
	runtimeDir := strings.TrimSpace(os.Getenv("XDG_RUNTIME_DIR"))
	if runtimeDir == "" {
		return "", errors.New("XDG_RUNTIME_DIR is not set")
	}
	return filepath.Join(runtimeDir, "sotto.sock"), nil
}

func Acquire(
	ctx context.Context,
	path string,
	probeTimeout time.Duration,
	retries int,
	rescue func(context.Context) error,
) (net.Listener, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("ensure runtime socket dir: %w", err)
	}

	for attempt := 0; attempt <= retries; attempt++ {
		listener, err := net.Listen("unix", path)
		if err == nil {
			_ = os.Chmod(path, 0o600)
			return listener, nil
		}

		if !isAddrInUse(err) {
			return nil, fmt.Errorf("listen unix %s: %w", path, err)
		}

		alive, probeErr := Probe(ctx, path, probeTimeout)
		if alive {
			return nil, ErrAlreadyRunning
		}
		if probeErr != nil {
			return nil, fmt.Errorf("probe existing socket %s: %w", path, probeErr)
		}

		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return nil, fmt.Errorf("remove stale socket %s: %w", path, removeErr)
		}

		if rescue != nil {
			_ = rescue(ctx)
		}

		if attempt < retries {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(25*(attempt+1)) * time.Millisecond):
			}
		}
	}

	return nil, fmt.Errorf("failed to acquire socket %s after %d retries", path, retries)
}

func isAddrInUse(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "address already in use")
}
