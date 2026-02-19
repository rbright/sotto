package indicator

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// desktopNotify sends a freedesktop notification over DBus via busctl.
// It returns the notification ID assigned by the server.
func desktopNotify(ctx context.Context, appName string, replaceID uint32, summary string, timeoutMS int) (uint32, error) {
	args := []string{
		"--user",
		"call",
		"org.freedesktop.Notifications",
		"/org/freedesktop/Notifications",
		"org.freedesktop.Notifications",
		"Notify",
		"susssasa{sv}i",
		appName,
		fmt.Sprintf("%d", replaceID),
		"",
		summary,
		"",
		"0", // actions array length
		"0", // hints map length
		fmt.Sprintf("%d", timeoutMS),
	}

	out, err := exec.CommandContext(ctx, "busctl", args...).CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(out))
		if trimmed == "" {
			return 0, fmt.Errorf("desktop notify failed: %w", err)
		}
		return 0, fmt.Errorf("desktop notify failed: %w (%s)", err, trimmed)
	}

	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) < 2 || fields[0] != "u" {
		return 0, fmt.Errorf("desktop notify invalid response: %q", strings.TrimSpace(string(out)))
	}

	value, parseErr := strconv.ParseUint(fields[1], 10, 32)
	if parseErr != nil {
		return 0, fmt.Errorf("desktop notify parse id %q: %w", fields[1], parseErr)
	}
	return uint32(value), nil
}

// desktopDismiss requests explicit close by notification ID.
func desktopDismiss(ctx context.Context, id uint32) error {
	args := []string{
		"--user",
		"call",
		"org.freedesktop.Notifications",
		"/org/freedesktop/Notifications",
		"org.freedesktop.Notifications",
		"CloseNotification",
		"u",
		fmt.Sprintf("%d", id),
	}

	out, err := exec.CommandContext(ctx, "busctl", args...).CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(out))
		if trimmed == "" {
			return fmt.Errorf("desktop dismiss failed: %w", err)
		}
		return fmt.Errorf("desktop dismiss failed: %w (%s)", err, trimmed)
	}

	return nil
}
