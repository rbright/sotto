package hypr

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type Controller interface {
	SetSubmap(ctx context.Context, name string) error
	ResetSubmap(ctx context.Context) error
}

type CLIController struct{}

func (CLIController) SetSubmap(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("submap name must not be empty")
	}
	return runHyprctl(ctx, "dispatch", "submap", name)
}

func (c CLIController) ResetSubmap(ctx context.Context) error {
	return c.SetSubmap(ctx, "reset")
}

func runHyprctl(ctx context.Context, args ...string) error {
	_, err := runHyprctlOutput(ctx, args...)
	return err
}

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
