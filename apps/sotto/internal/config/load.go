package config

import (
	"errors"
	"fmt"
	"os"
)

type Loaded struct {
	Path     string
	Config   Config
	Warnings []Warning
	Exists   bool
}

func Load(explicitPath string) (Loaded, error) {
	resolvedPath, err := ResolvePath(explicitPath)
	if err != nil {
		return Loaded{}, err
	}

	base := Default()
	content, err := os.ReadFile(resolvedPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Loaded{
				Path:   resolvedPath,
				Config: base,
				Warnings: []Warning{{
					Message: fmt.Sprintf("config file %q not found; using defaults", resolvedPath),
				}},
				Exists: false,
			}, nil
		}
		return Loaded{}, fmt.Errorf("read config %q: %w", resolvedPath, err)
	}

	cfg, warnings, err := Parse(string(content), base)
	if err != nil {
		return Loaded{}, fmt.Errorf("parse config %q: %w", resolvedPath, err)
	}

	return Loaded{
		Path:     resolvedPath,
		Config:   cfg,
		Warnings: warnings,
		Exists:   true,
	}, nil
}
