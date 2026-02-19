package hypr

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Controller abstracts submap control for adapters that need it.
type Controller interface {
	SetSubmap(ctx context.Context, name string) error
	ResetSubmap(ctx context.Context) error
}

// CLIController issues submap commands through hyprctl.
type CLIController struct{}

// SetSubmap sets an explicit Hyprland submap name.
func (CLIController) SetSubmap(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("submap name must not be empty")
	}
	return runHyprctl(ctx, "dispatch", "submap", name)
}

// ResetSubmap resets back to the default Hyprland submap.
func (c CLIController) ResetSubmap(ctx context.Context) error {
	return c.SetSubmap(ctx, "reset")
}

// runHyprctl executes hyprctl and discards stdout on success.
func runHyprctl(ctx context.Context, args ...string) error {
	_, err := runHyprctlOutput(ctx, args...)
	return err
}

// runHyprctlOutput executes hyprctl and returns combined output for diagnostics.
func runHyprctlOutput(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "hyprctl", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(out))
		if trimmed == "" {
			return nil, fmt.Errorf("hyprctl %v failed: %w", args, err)
		}
		return nil, fmt.Errorf("hyprctl %v failed: %w (%s)", args, err, trimmed)
	}
	return out, nil
}
