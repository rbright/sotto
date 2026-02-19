package output

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rbright/sotto/internal/hypr"
)

func defaultPaste(ctx context.Context, shortcut string) error {
	window, err := activeWindowWithRetry(ctx, 5, 10*time.Millisecond)
	if err != nil {
		return err
	}

	payload, err := buildPasteShortcut(shortcut, strings.TrimSpace(window.Address))
	if err != nil {
		return err
	}
	return hypr.SendShortcut(ctx, payload)
}

func buildPasteShortcut(shortcut string, windowAddress string) (string, error) {
	shortcut = strings.TrimSpace(shortcut)
	if shortcut == "" {
		return "", fmt.Errorf("paste shortcut cannot be empty")
	}

	address := strings.TrimSpace(windowAddress)
	if address == "" {
		return "", fmt.Errorf("active window address is required")
	}

	return fmt.Sprintf("%s,address:%s", shortcut, address), nil
}

func activeWindowWithRetry(ctx context.Context, attempts int, delay time.Duration) (hypr.ActiveWindow, error) {
	if attempts <= 0 {
		attempts = 1
	}

	var lastErr error
	for i := 0; i < attempts; i++ {
		window, err := hypr.QueryActiveWindow(ctx)
		if err == nil {
			return window, nil
		}
		lastErr = err
		if i == attempts-1 {
			break
		}
		select {
		case <-ctx.Done():
			return hypr.ActiveWindow{}, ctx.Err()
		case <-time.After(delay):
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("active window unavailable")
	}
	return hypr.ActiveWindow{}, fmt.Errorf("resolve active window: %w", lastErr)
}
