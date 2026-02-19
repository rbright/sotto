package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// ResolvePath applies CLI/XDG/home fallback rules for config.conf location.
func ResolvePath(explicit string) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		return explicit, nil
	}

	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		return filepath.Join(xdg, "sotto", "config.conf"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", errors.New("unable to resolve user home for config fallback")
	}

	return filepath.Join(home, ".config", "sotto", "config.conf"), nil
}
