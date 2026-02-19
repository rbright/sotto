// Package config resolves, parses, validates, and defaults sotto configuration.
package config

import "strings"

const legacyFormatWarning = "legacy key=value config format is deprecated; migrate to JSONC"

// Parse reads configuration content as JSONC (preferred) or legacy key/value format.
//
// JSONC is selected when the first non-whitespace character is `{`.
func Parse(content string, base Config) (Config, []Warning, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		validatedWarnings, err := Validate(base)
		if err != nil {
			return Config{}, nil, err
		}
		return base, validatedWarnings, nil
	}

	if strings.HasPrefix(trimmed, "{") {
		return parseJSONC(content, base)
	}

	cfg, warnings, err := parseLegacy(content, base)
	if err != nil {
		return Config{}, nil, err
	}
	warnings = append([]Warning{{Message: legacyFormatWarning}}, warnings...)
	return cfg, warnings, nil
}
