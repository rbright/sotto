// Package output applies transcript commit side effects (clipboard and paste).
package output

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"time"

	"github.com/rbright/sotto/internal/config"
)

// Committer applies transcript output side effects (clipboard + optional paste).
type Committer struct {
	config config.Config
	logger *slog.Logger
}

// NewCommitter constructs a transcript committer from runtime config.
func NewCommitter(cfg config.Config, logger *slog.Logger) *Committer {
	return &Committer{config: cfg, logger: logger}
}

// Commit writes transcript text to clipboard and optionally dispatches paste.
func (c *Committer) Commit(ctx context.Context, transcript string) error {
	if transcript == "" {
		return nil
	}

	clipboardCtx, clipboardCancel := context.WithTimeout(ctx, 2*time.Second)
	defer clipboardCancel()
	if err := runCommandWithInput(clipboardCtx, c.config.Clipboard.Argv, transcript); err != nil {
		return fmt.Errorf("set clipboard: %w", err)
	}

	if !c.config.Paste.Enable {
		return nil
	}

	if len(c.config.PasteCmd.Argv) > 0 {
		pasteCtx, pasteCancel := context.WithTimeout(ctx, 2*time.Second)
		defer pasteCancel()
		if err := runCommandWithInput(pasteCtx, c.config.PasteCmd.Argv, ""); err != nil {
			c.logPasteFailure(err)
		}
		return nil
	}

	pasteCtx, pasteCancel := context.WithTimeout(ctx, 1200*time.Millisecond)
	defer pasteCancel()
	if err := defaultPaste(pasteCtx, c.config.Paste.Shortcut); err != nil {
		c.logPasteFailure(err)
	}
	return nil
}

// runCommandWithInput executes argv and optionally writes input to stdin.
func runCommandWithInput(ctx context.Context, argv []string, input string) error {
	if len(argv) == 0 {
		return fmt.Errorf("command argv cannot be empty")
	}

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("open stdin for %s: %w", argv[0], err)
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return fmt.Errorf("start command %s: %w", argv[0], err)
	}

	if input != "" {
		if _, err := stdin.Write([]byte(input)); err != nil {
			_ = stdin.Close()
			_ = cmd.Wait()
			return fmt.Errorf("write stdin for %s: %w", argv[0], err)
		}
	}
	_ = stdin.Close()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("wait for %s: %w", argv[0], err)
	}
	return nil
}

// logPasteFailure records paste errors while preserving clipboard success semantics.
func (c *Committer) logPasteFailure(err error) {
	if c.logger == nil || err == nil {
		return
	}
	c.logger.Error("paste dispatch failed; clipboard remains set", "error", err.Error())
}
