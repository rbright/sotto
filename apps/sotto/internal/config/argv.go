package config

import (
	"fmt"
	"strings"
	"unicode"
)

func parseArgv(input string) ([]string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, nil
	}
	if strings.HasPrefix(input, "#") {
		return nil, nil
	}

	var (
		argv    []string
		current strings.Builder
		quote   rune
		escape  bool
	)

	flush := func() {
		if current.Len() == 0 {
			return
		}
		argv = append(argv, current.String())
		current.Reset()
	}

	for _, r := range input {
		switch {
		case escape:
			current.WriteRune(r)
			escape = false
		case r == '\\':
			escape = true
		case quote != 0:
			if r == quote {
				quote = 0
				continue
			}
			current.WriteRune(r)
		case r == '\'' || r == '"':
			quote = r
		case unicode.IsSpace(r):
			flush()
		default:
			current.WriteRune(r)
		}
	}

	if escape {
		return nil, fmt.Errorf("unterminated escape sequence in command: %q", input)
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote in command: %q", input)
	}

	flush()
	return argv, nil
}

func mustParseArgv(input string) []string {
	argv, err := parseArgv(input)
	if err != nil {
		panic(err)
	}
	return argv
}
