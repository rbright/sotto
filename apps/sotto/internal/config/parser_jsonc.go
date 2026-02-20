package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

type jsoncConfig struct {
	Riva       *jsoncRiva       `json:"riva"`
	Audio      *jsoncAudio      `json:"audio"`
	Paste      *jsoncPaste      `json:"paste"`
	ASR        *jsoncASR        `json:"asr"`
	Transcript *jsoncTranscript `json:"transcript"`
	Indicator  *jsoncIndicator  `json:"indicator"`

	ClipboardCmd *string     `json:"clipboard_cmd"`
	PasteCmd     *string     `json:"paste_cmd"`
	Vocab        *jsoncVocab `json:"vocab"`
	Debug        *jsoncDebug `json:"debug"`
}

type jsoncRiva struct {
	GRPC       *string `json:"grpc"`
	HTTP       *string `json:"http"`
	HealthPath *string `json:"health_path"`
}

type jsoncAudio struct {
	Input    *string `json:"input"`
	Fallback *string `json:"fallback"`
}

type jsoncPaste struct {
	Enable   *bool   `json:"enable"`
	Shortcut *string `json:"shortcut"`
}

type jsoncASR struct {
	AutomaticPunctuation *bool   `json:"automatic_punctuation"`
	LanguageCode         *string `json:"language_code"`
	Model                *string `json:"model"`
}

type jsoncTranscript struct {
	TrailingSpace *bool `json:"trailing_space"`
}

type jsoncIndicator struct {
	Enable         *bool   `json:"enable"`
	Backend        *string `json:"backend"`
	DesktopAppName *string `json:"desktop_app_name"`
	SoundEnable    *bool   `json:"sound_enable"`
	Height         *int    `json:"height"`
	ErrorTimeoutMS *int    `json:"error_timeout_ms"`
}

type jsoncVocab struct {
	Global     *jsoncStringList         `json:"global"`
	MaxPhrases *int                     `json:"max_phrases"`
	Sets       map[string]jsoncVocabSet `json:"sets"`
}

type jsoncVocabSet struct {
	Boost   *float64 `json:"boost"`
	Phrases []string `json:"phrases"`
}

type jsoncDebug struct {
	AudioDump *bool `json:"audio_dump"`
	GRPCDump  *bool `json:"grpc_dump"`
}

type jsoncStringList []string

func (l *jsoncStringList) UnmarshalJSON(data []byte) error {
	var list []string
	if err := json.Unmarshal(data, &list); err == nil {
		*l = list
		return nil
	}

	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		parts := strings.Split(single, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			out = append(out, part)
		}
		*l = out
		return nil
	}

	return fmt.Errorf("expected string array or comma-delimited string")
}

func parseJSONC(content string, base Config) (Config, []Warning, error) {
	normalized, err := normalizeJSONC(content)
	if err != nil {
		return Config{}, nil, err
	}

	decoder := json.NewDecoder(strings.NewReader(normalized))
	decoder.DisallowUnknownFields()

	var payload jsoncConfig
	if err := decoder.Decode(&payload); err != nil {
		return Config{}, nil, wrapJSONDecodeError(normalized, err)
	}
	if err := ensureSingleJSONValue(decoder); err != nil {
		return Config{}, nil, wrapJSONDecodeError(normalized, err)
	}

	cfg := base
	warnings, err := payload.applyTo(&cfg)
	if err != nil {
		return Config{}, nil, err
	}

	validatedWarnings, err := Validate(cfg)
	if err != nil {
		return Config{}, nil, err
	}
	warnings = append(warnings, validatedWarnings...)
	return cfg, warnings, nil
}

func (payload jsoncConfig) applyTo(cfg *Config) ([]Warning, error) {
	warnings := make([]Warning, 0)

	if payload.Riva != nil {
		if payload.Riva.GRPC != nil {
			cfg.RivaGRPC = *payload.Riva.GRPC
		}
		if payload.Riva.HTTP != nil {
			cfg.RivaHTTP = *payload.Riva.HTTP
		}
		if payload.Riva.HealthPath != nil {
			cfg.RivaHealthPath = *payload.Riva.HealthPath
		}
	}

	if payload.Audio != nil {
		if payload.Audio.Input != nil {
			cfg.Audio.Input = *payload.Audio.Input
		}
		if payload.Audio.Fallback != nil {
			cfg.Audio.Fallback = *payload.Audio.Fallback
		}
	}

	if payload.Paste != nil {
		if payload.Paste.Enable != nil {
			cfg.Paste.Enable = *payload.Paste.Enable
		}
		if payload.Paste.Shortcut != nil {
			cfg.Paste.Shortcut = strings.TrimSpace(*payload.Paste.Shortcut)
		}
	}

	if payload.ASR != nil {
		if payload.ASR.AutomaticPunctuation != nil {
			cfg.ASR.AutomaticPunctuation = *payload.ASR.AutomaticPunctuation
		}
		if payload.ASR.LanguageCode != nil {
			cfg.ASR.LanguageCode = *payload.ASR.LanguageCode
		}
		if payload.ASR.Model != nil {
			cfg.ASR.Model = *payload.ASR.Model
		}
	}

	if payload.Transcript != nil && payload.Transcript.TrailingSpace != nil {
		cfg.Transcript.TrailingSpace = *payload.Transcript.TrailingSpace
	}

	if payload.Indicator != nil {
		if payload.Indicator.Enable != nil {
			cfg.Indicator.Enable = *payload.Indicator.Enable
		}
		if payload.Indicator.Backend != nil {
			cfg.Indicator.Backend = strings.TrimSpace(*payload.Indicator.Backend)
		}
		if payload.Indicator.DesktopAppName != nil {
			cfg.Indicator.DesktopAppName = strings.TrimSpace(*payload.Indicator.DesktopAppName)
		}
		if payload.Indicator.SoundEnable != nil {
			cfg.Indicator.SoundEnable = *payload.Indicator.SoundEnable
		}
		if payload.Indicator.Height != nil {
			cfg.Indicator.Height = *payload.Indicator.Height
		}
		if payload.Indicator.ErrorTimeoutMS != nil {
			cfg.Indicator.ErrorTimeoutMS = *payload.Indicator.ErrorTimeoutMS
		}
	}

	if payload.ClipboardCmd != nil {
		raw := *payload.ClipboardCmd
		argv, err := parseArgv(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid clipboard_cmd: %w", err)
		}
		cfg.Clipboard = CommandConfig{Raw: raw, Argv: argv}
	}

	if payload.PasteCmd != nil {
		raw := *payload.PasteCmd
		argv, err := parseArgv(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid paste_cmd: %w", err)
		}
		cfg.PasteCmd = CommandConfig{Raw: raw, Argv: argv}
	}

	if payload.Vocab != nil {
		if payload.Vocab.Global != nil {
			cfg.Vocab.GlobalSets = cfg.Vocab.GlobalSets[:0]
			for _, name := range *payload.Vocab.Global {
				name = strings.TrimSpace(name)
				if name == "" {
					continue
				}
				cfg.Vocab.GlobalSets = append(cfg.Vocab.GlobalSets, name)
			}
		}
		if payload.Vocab.MaxPhrases != nil {
			cfg.Vocab.MaxPhrases = *payload.Vocab.MaxPhrases
		}
		if payload.Vocab.Sets != nil {
			if cfg.Vocab.Sets == nil {
				cfg.Vocab.Sets = make(map[string]VocabSet)
			}
			for name, set := range payload.Vocab.Sets {
				trimmedName := strings.TrimSpace(name)
				if trimmedName == "" {
					return nil, fmt.Errorf("vocab.sets contains an empty set name")
				}

				phrases := make([]string, 0, len(set.Phrases))
				phrases = append(phrases, set.Phrases...)

				entry := VocabSet{Name: trimmedName, Phrases: phrases}
				if set.Boost != nil {
					entry.Boost = *set.Boost
				}
				cfg.Vocab.Sets[trimmedName] = entry
			}
		}
	}

	if payload.Debug != nil {
		if payload.Debug.AudioDump != nil {
			cfg.Debug.EnableAudioDump = *payload.Debug.AudioDump
		}
		if payload.Debug.GRPCDump != nil {
			cfg.Debug.EnableGRPCDump = *payload.Debug.GRPCDump
		}
	}

	return warnings, nil
}

func normalizeJSONC(content string) (string, error) {
	withoutComments, err := stripJSONCComments(content)
	if err != nil {
		return "", err
	}
	return stripJSONCTrailingCommas(withoutComments), nil
}

func stripJSONCComments(content string) (string, error) {
	var out strings.Builder
	out.Grow(len(content))

	inString := false
	escape := false
	lineComment := false
	blockComment := false

	for i := 0; i < len(content); i++ {
		ch := content[i]

		if lineComment {
			if ch == '\n' {
				lineComment = false
				out.WriteByte(ch)
				continue
			}
			if ch == '\r' {
				lineComment = false
				out.WriteByte(ch)
				continue
			}
			out.WriteByte(' ')
			continue
		}

		if blockComment {
			if ch == '*' && i+1 < len(content) && content[i+1] == '/' {
				blockComment = false
				out.WriteString("  ")
				i++
				continue
			}
			if ch == '\n' || ch == '\r' || ch == '\t' {
				out.WriteByte(ch)
			} else {
				out.WriteByte(' ')
			}
			continue
		}

		if inString {
			out.WriteByte(ch)
			if escape {
				escape = false
				continue
			}
			if ch == '\\' {
				escape = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}

		if ch == '"' {
			inString = true
			out.WriteByte(ch)
			continue
		}

		if ch == '/' && i+1 < len(content) {
			next := content[i+1]
			if next == '/' {
				lineComment = true
				out.WriteString("  ")
				i++
				continue
			}
			if next == '*' {
				blockComment = true
				out.WriteString("  ")
				i++
				continue
			}
		}

		out.WriteByte(ch)
	}

	if blockComment {
		return "", fmt.Errorf("unterminated block comment in JSONC")
	}

	return out.String(), nil
}

func stripJSONCTrailingCommas(content string) string {
	var out strings.Builder
	out.Grow(len(content))

	inString := false
	escape := false

	for i := 0; i < len(content); i++ {
		ch := content[i]

		if inString {
			out.WriteByte(ch)
			if escape {
				escape = false
				continue
			}
			if ch == '\\' {
				escape = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}

		if ch == '"' {
			inString = true
			out.WriteByte(ch)
			continue
		}

		if ch == ',' {
			j := i + 1
			for j < len(content) && isJSONWhitespace(content[j]) {
				j++
			}
			if j < len(content) && (content[j] == '}' || content[j] == ']') {
				continue
			}
		}

		out.WriteByte(ch)
	}

	return out.String()
}

func isJSONWhitespace(ch byte) bool {
	switch ch {
	case ' ', '\n', '\r', '\t':
		return true
	default:
		return false
	}
}

func ensureSingleJSONValue(decoder *json.Decoder) error {
	var extra struct{}
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return fmt.Errorf("multiple JSON values are not allowed")
	}
	return err
}

func wrapJSONDecodeError(content string, err error) error {
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		line, col := offsetToLineCol(content, syntaxErr.Offset)
		return fmt.Errorf("line %d column %d: %w", line, col, err)
	}

	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		line, col := offsetToLineCol(content, typeErr.Offset)
		return fmt.Errorf("line %d column %d: %w", line, col, err)
	}

	return err
}

func offsetToLineCol(content string, offset int64) (int, int) {
	if offset <= 0 {
		return 1, 1
	}

	limit := int(offset)
	if limit > len(content) {
		limit = len(content)
	}

	line := 1
	col := 1
	for i := 0; i < limit-1; i++ {
		if content[i] == '\n' {
			line++
			col = 1
			continue
		}
		col++
	}
	return line, col
}
