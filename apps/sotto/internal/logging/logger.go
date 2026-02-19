package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

type Runtime struct {
	Logger *slog.Logger
	Path   string
	closer io.Closer
}

func (r Runtime) Close() error {
	if r.closer == nil {
		return nil
	}
	return r.closer.Close()
}

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
