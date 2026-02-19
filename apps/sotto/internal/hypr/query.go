package hypr

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ActiveWindow contains the fields needed for paste dispatch targeting.
type ActiveWindow struct {
	Address      string `json:"address"`
	Class        string `json:"class"`
	InitialClass string `json:"initialClass"`
}

type monitor struct {
	Name    string `json:"name"`
	Focused bool   `json:"focused"`
}

// QueryActiveWindow fetches and validates the active-window contract from hyprctl.
func QueryActiveWindow(ctx context.Context) (ActiveWindow, error) {
	output, err := runHyprctlJSON(ctx, "activewindow")
	if err != nil {
		return ActiveWindow{}, err
	}

	var window ActiveWindow
	if err := json.Unmarshal(output, &window); err != nil {
		return ActiveWindow{}, fmt.Errorf("decode hyprctl activewindow json: %w", err)
	}
	window.Address = strings.TrimSpace(window.Address)
	window.Class = strings.TrimSpace(window.Class)
	window.InitialClass = strings.TrimSpace(window.InitialClass)
	if window.Address == "" {
		return ActiveWindow{}, fmt.Errorf("hyprctl activewindow returned empty address")
	}
	return window, nil
}

// QueryFocusedMonitor returns the focused monitor name (or the first monitor fallback).
func QueryFocusedMonitor(ctx context.Context) (string, error) {
	output, err := runHyprctlJSON(ctx, "monitors")
	if err != nil {
		return "", err
	}

	var monitors []monitor
	if err := json.Unmarshal(output, &monitors); err != nil {
		return "", fmt.Errorf("decode hyprctl monitors json: %w", err)
	}
	for _, mon := range monitors {
		if mon.Focused {
			return strings.TrimSpace(mon.Name), nil
		}
	}
	if len(monitors) == 0 {
		return "", fmt.Errorf("hyprctl monitors returned no outputs")
	}
	return strings.TrimSpace(monitors[0].Name), nil
}

// SendShortcut sends a literal hyprctl sendshortcut payload.
func SendShortcut(ctx context.Context, shortcut string) error {
	shortcut = strings.TrimSpace(shortcut)
	if shortcut == "" {
		return fmt.Errorf("sendshortcut requires a non-empty payload")
	}
	return runHyprctl(ctx, "--quiet", "dispatch", "sendshortcut", shortcut)
}

// Notify sends a Hyprland notification payload.
func Notify(ctx context.Context, icon int, timeoutMS int, color string, text string) error {
	if strings.TrimSpace(color) == "" {
		color = "rgb(89b4fa)"
	}
	return runHyprctl(
		ctx,
		"--quiet",
		"dispatch",
		"notify",
		strconv.Itoa(icon),
		strconv.Itoa(timeoutMS),
		color,
		text,
	)
}

// DismissNotify dismisses active Hyprland notifications.
func DismissNotify(ctx context.Context) error {
	return runHyprctl(ctx, "--quiet", "dispatch", "dismissnotify")
}

// runHyprctlJSON executes a JSON-returning hyprctl subcommand.
func runHyprctlJSON(ctx context.Context, target string) ([]byte, error) {
	output, err := runHyprctlOutput(ctx, "-j", target)
	if err != nil {
		return nil, err
	}
	return output, nil
}
