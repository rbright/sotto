// Package logging configures runtime JSONL logging output.
package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// Runtime bundles the configured logger and its open file handle lifecycle.
type Runtime struct {
	Logger *slog.Logger
	Path   string
	closer io.Closer
}

// Close flushes and closes the logger output sink.
func (r Runtime) Close() error {
	if r.closer == nil {
		return nil
	}
	return r.closer.Close()
}

// New builds a JSONL logger rooted at the resolved state path.
func New() (Runtime, error) {
	path, err := resolveLogPath()
	if err != nil {
		return Runtime{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return Runtime{}, err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return Runtime{}, err
	}

	h := slog.NewJSONHandler(f, &slog.HandlerOptions{Level: slog.LevelInfo})
	logger := slog.New(h)
	return Runtime{Logger: logger, Path: path, closer: f}, nil
}

// resolveLogPath selects XDG_STATE_HOME when available, otherwise ~/.local/state.
func resolveLogPath() (string, error) {
	if xdg := strings.TrimSpace(os.Getenv("XDG_STATE_HOME")); xdg != "" {
		return filepath.Join(xdg, "sotto", "log.jsonl"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "sotto", "log.jsonl"), nil
}
