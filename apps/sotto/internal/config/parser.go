package config

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

type parseState struct {
	inVocabSet *VocabSet
}

func Parse(content string, base Config) (Config, []Warning, error) {
	cfg := base
	warnings := make([]Warning, 0)
	state := &parseState{}

	scanner := bufio.NewScanner(strings.NewReader(content))
	for line := 1; scanner.Scan(); line++ {
		raw := scanner.Text()
		trimmed := strings.TrimSpace(stripComments(raw))
		if trimmed == "" {
			continue
		}

		if state.inVocabSet != nil {
			if trimmed == "}" {
				cfg.Vocab.Sets[state.inVocabSet.Name] = *state.inVocabSet
				state.inVocabSet = nil
				continue
			}

			key, value, err := parseAssignment(trimmed)
			if err != nil {
				return Config{}, nil, lineError(line, err)
			}
			if err := applyVocabSetKey(state.inVocabSet, key, value); err != nil {
				return Config{}, nil, lineError(line, err)
			}
			continue
		}

		if strings.HasPrefix(trimmed, "vocabset ") {
			set, err := parseVocabSetHeader(trimmed)
			if err != nil {
				return Config{}, nil, lineError(line, err)
			}
			if _, exists := cfg.Vocab.Sets[set.Name]; exists {
				warnings = append(warnings, Warning{
					Line:    line,
					Message: fmt.Sprintf("vocabset %q redefined; last definition wins", set.Name),
				})
			}
			state.inVocabSet = &set
			continue
		}

		key, value, err := parseAssignment(trimmed)
		if err != nil {
			return Config{}, nil, lineError(line, err)
		}
		if err := applyRootKey(&cfg, key, value); err != nil {
			return Config{}, nil, lineError(line, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return Config{}, nil, err
	}

	if state.inVocabSet != nil {
		return Config{}, nil, fmt.Errorf("line %d: unterminated vocabset %q block", scannerPosition(content), state.inVocabSet.Name)
	}

	validatedWarnings, err := Validate(cfg)
	if err != nil {
		return Config{}, nil, err
	}
	warnings = append(warnings, validatedWarnings...)

	return cfg, warnings, nil
}

func parseAssignment(line string) (string, string, error) {
	idx := strings.Index(line, "=")
	if idx < 0 {
		return "", "", fmt.Errorf("expected key = value")
	}
	key := strings.TrimSpace(line[:idx])
	value := strings.TrimSpace(line[idx+1:])
	if key == "" {
		return "", "", fmt.Errorf("empty key")
	}
	if value == "" {
		return "", "", fmt.Errorf("missing value for key %q", key)
	}
	return key, value, nil
}

func parseVocabSetHeader(line string) (VocabSet, error) {
	if !strings.HasSuffix(line, "{") {
		return VocabSet{}, fmt.Errorf("vocabset declaration must end with '{'")
	}
	line = strings.TrimSpace(strings.TrimSuffix(line, "{"))
	parts := strings.Fields(line)
	if len(parts) != 2 {
		return VocabSet{}, fmt.Errorf("invalid vocabset declaration; expected: vocabset <name> {")
	}
	if parts[0] != "vocabset" {
		return VocabSet{}, fmt.Errorf("invalid block type %q", parts[0])
	}

	return VocabSet{Name: parts[1]}, nil
}

func applyVocabSetKey(set *VocabSet, key, value string) error {
	switch key {
	case "boost":
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("invalid boost value: %w", err)
		}
		set.Boost = f
	case "phrases":
		phrases, err := parseStringList(value)
		if err != nil {
			return err
		}
		set.Phrases = phrases
	default:
		return fmt.Errorf("unknown vocabset key %q", key)
	}
	return nil
}

func applyRootKey(cfg *Config, key, value string) error {
	switch key {
	case "riva_grpc":
		v, err := parseStringValue(value)
		if err != nil {
			return err
		}
		cfg.RivaGRPC = v
	case "riva_http":
		v, err := parseStringValue(value)
		if err != nil {
			return err
		}
		cfg.RivaHTTP = v
	case "riva_health_path":
		v, err := parseStringValue(value)
		if err != nil {
			return err
		}
		cfg.RivaHealthPath = v
	case "audio.input":
		v, err := parseStringValue(value)
		if err != nil {
			return err
		}
		cfg.Audio.Input = v
	case "audio.fallback":
		v, err := parseStringValue(value)
		if err != nil {
			return err
		}
		cfg.Audio.Fallback = v
	case "paste.enable":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid bool for paste.enable: %w", err)
		}
		cfg.Paste.Enable = b
	case "paste.shortcut":
		v, err := parseStringValue(value)
		if err != nil {
			return err
		}
		cfg.Paste.Shortcut = strings.TrimSpace(v)
	case "asr.automatic_punctuation":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid bool for asr.automatic_punctuation: %w", err)
		}
		cfg.ASR.AutomaticPunctuation = b
	case "asr.language_code":
		v, err := parseStringValue(value)
		if err != nil {
			return err
		}
		cfg.ASR.LanguageCode = v
	case "asr.model":
		v, err := parseStringValue(value)
		if err != nil {
			return err
		}
		cfg.ASR.Model = v
	case "transcript.trailing_space":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid bool for transcript.trailing_space: %w", err)
		}
		cfg.Transcript.TrailingSpace = b
	case "indicator.enable":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid bool for indicator.enable: %w", err)
		}
		cfg.Indicator.Enable = b
	case "indicator.sound_enable":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid bool for indicator.sound_enable: %w", err)
		}
		cfg.Indicator.SoundEnable = b
	case "indicator.sound_start_file":
		v, err := parseStringValue(value)
		if err != nil {
			return err
		}
		cfg.Indicator.SoundStartFile = v
	case "indicator.sound_stop_file":
		v, err := parseStringValue(value)
		if err != nil {
			return err
		}
		cfg.Indicator.SoundStopFile = v
	case "indicator.sound_complete_file":
		v, err := parseStringValue(value)
		if err != nil {
			return err
		}
		cfg.Indicator.SoundCompleteFile = v
	case "indicator.sound_cancel_file":
		v, err := parseStringValue(value)
		if err != nil {
			return err
		}
		cfg.Indicator.SoundCancelFile = v
	case "indicator.height":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid int for indicator.height: %w", err)
		}
		cfg.Indicator.Height = n
	case "indicator.text_recording":
		v, err := parseStringValue(value)
		if err != nil {
			return err
		}
		cfg.Indicator.TextRecording = v
	case "indicator.text_processing", "indicator.text_transcribing":
		v, err := parseStringValue(value)
		if err != nil {
			return err
		}
		cfg.Indicator.TextProcessing = v
	case "indicator.text_error":
		v, err := parseStringValue(value)
		if err != nil {
			return err
		}
		cfg.Indicator.TextError = v
	case "indicator.error_timeout_ms":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid int for indicator.error_timeout_ms: %w", err)
		}
		cfg.Indicator.ErrorTimeoutMS = n
	case "clipboard_cmd":
		v, err := parseStringValue(value)
		if err != nil {
			return err
		}
		argv, err := parseArgv(v)
		if err != nil {
			return fmt.Errorf("invalid clipboard_cmd: %w", err)
		}
		cfg.Clipboard = CommandConfig{Raw: v, Argv: argv}
	case "paste_cmd":
		v, err := parseStringValue(value)
		if err != nil {
			return err
		}
		argv, err := parseArgv(v)
		if err != nil {
			return fmt.Errorf("invalid paste_cmd: %w", err)
		}
		cfg.PasteCmd = CommandConfig{Raw: v, Argv: argv}
	case "vocab.global":
		sets := strings.Split(value, ",")
		cfg.Vocab.GlobalSets = cfg.Vocab.GlobalSets[:0]
		for _, s := range sets {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			cfg.Vocab.GlobalSets = append(cfg.Vocab.GlobalSets, s)
		}
	case "vocab.max_phrases":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid int for vocab.max_phrases: %w", err)
		}
		cfg.Vocab.MaxPhrases = n
	case "debug.audio_dump":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid bool for debug.audio_dump: %w", err)
		}
		cfg.Debug.EnableAudioDump = b
	case "debug.grpc_dump":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid bool for debug.grpc_dump: %w", err)
		}
		cfg.Debug.EnableGRPCDump = b
	default:
		return fmt.Errorf("unknown key %q", key)
	}

	return nil
}

func parseStringValue(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("value cannot be empty")
	}

	if strings.HasPrefix(raw, "\"") {
		quoted, err := strconv.Unquote(raw)
		if err != nil {
			return "", fmt.Errorf("invalid quoted string %q: %w", raw, err)
		}
		return quoted, nil
	}
	if strings.HasPrefix(raw, "'") {
		return parseSingleQuotedString(raw)
	}

	return raw, nil
}

func parseSingleQuotedString(raw string) (string, error) {
	if len(raw) < 2 || !strings.HasSuffix(raw, "'") {
		return "", fmt.Errorf("invalid quoted string %q: missing closing single quote", raw)
	}

	inner := raw[1 : len(raw)-1]
	var (
		out    strings.Builder
		escape bool
	)
	for _, r := range inner {
		switch {
		case escape:
			out.WriteRune(r)
			escape = false
		case r == '\\':
			escape = true
		default:
			out.WriteRune(r)
		}
	}
	if escape {
		return "", fmt.Errorf("invalid quoted string %q: unterminated escape", raw)
	}
	return out.String(), nil
}

func parseStringList(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "[") || !strings.HasSuffix(raw, "]") {
		return nil, fmt.Errorf("phrases must be in [ ... ]")
	}

	raw = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(raw, "["), "]"))
	if raw == "" {
		return nil, nil
	}

	parts := splitCommaAware(raw)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		val, err := parseStringValue(part)
		if err != nil {
			return nil, fmt.Errorf("invalid phrase %q: %w", part, err)
		}
		out = append(out, val)
	}
	return out, nil
}

func splitCommaAware(input string) []string {
	var (
		parts  []string
		start  int
		quote  rune
		escape bool
	)

	for i, r := range input {
		switch {
		case escape:
			escape = false
		case r == '\\':
			escape = true
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '\'' || r == '"':
			quote = r
		case r == ',':
			parts = append(parts, input[start:i])
			start = i + 1
		}
	}

	parts = append(parts, input[start:])
	return parts
}

func stripComments(line string) string {
	var (
		quote  rune
		escape bool
	)
	for i, r := range line {
		switch {
		case escape:
			escape = false
		case r == '\\':
			escape = true
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '\'' || r == '"':
			quote = r
		case r == '#':
			return line[:i]
		}
	}
	return line
}

func lineError(line int, err error) error {
	return fmt.Errorf("line %d: %w", line, err)
}

func scannerPosition(content string) int {
	if content == "" {
		return 1
	}
	return strings.Count(content, "\n") + 1
}
