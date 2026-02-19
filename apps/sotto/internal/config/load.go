package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// Loaded captures resolved config path, parsed values, and non-fatal warnings.
type Loaded struct {
	Path     string
	Config   Config
	Warnings []Warning
	Exists   bool
}

// Load resolves, reads, parses, and validates the runtime configuration.
func Load(explicitPath string) (Loaded, error) {
	resolvedPath, err := ResolvePath(explicitPath)
	if err != nil {
		return Loaded{}, err
	}

	base := Default()
	loadedPath := resolvedPath
	warnings := make([]Warning, 0)

	content, err := os.ReadFile(resolvedPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return Loaded{}, fmt.Errorf("read config %q: %w", resolvedPath, err)
		}

		if strings.TrimSpace(explicitPath) == "" {
			legacyPath := legacyPathFor(resolvedPath)
			if legacyPath != "" {
				legacyContent, legacyErr := os.ReadFile(legacyPath)
				if legacyErr == nil {
					content = legacyContent
					loadedPath = legacyPath
					warnings = append(warnings, Warning{
						Message: fmt.Sprintf("loaded legacy config path %q; migrate to %q (JSONC)", legacyPath, resolvedPath),
					})
				} else if !errors.Is(legacyErr, os.ErrNotExist) {
					return Loaded{}, fmt.Errorf("read config %q: %w", legacyPath, legacyErr)
				}
			}
		}

		if content == nil {
			warnings = append(warnings, Warning{
				Message: fmt.Sprintf("config file %q not found; using defaults", resolvedPath),
			})
			return Loaded{
				Path:     resolvedPath,
				Config:   base,
				Warnings: warnings,
				Exists:   false,
			}, nil
		}
	}

	cfg, parseWarnings, err := Parse(string(content), base)
	if err != nil {
		return Loaded{}, fmt.Errorf("parse config %q: %w", loadedPath, err)
	}
	warnings = append(warnings, parseWarnings...)

	return Loaded{
		Path:     loadedPath,
		Config:   cfg,
		Warnings: warnings,
		Exists:   true,
	}, nil
}
