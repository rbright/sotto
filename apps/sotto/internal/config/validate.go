package config

import (
	"fmt"
	"sort"
	"strings"
)

// Validate enforces config invariants and returns non-fatal warnings.
func Validate(cfg Config) ([]Warning, error) {
	warnings := make([]Warning, 0)

	if strings.TrimSpace(cfg.RivaGRPC) == "" {
		return nil, fmt.Errorf("riva_grpc must not be empty")
	}
	if strings.TrimSpace(cfg.RivaHTTP) == "" {
		return nil, fmt.Errorf("riva_http must not be empty")
	}
	if strings.TrimSpace(cfg.RivaHealthPath) == "" {
		return nil, fmt.Errorf("riva_health_path must not be empty")
	}
	if !strings.HasPrefix(strings.TrimSpace(cfg.RivaHealthPath), "/") {
		return nil, fmt.Errorf("riva_health_path must start with '/'")
	}
	if strings.TrimSpace(cfg.ASR.LanguageCode) == "" {
		return nil, fmt.Errorf("asr.language_code must not be empty")
	}
	backend := strings.ToLower(strings.TrimSpace(cfg.Indicator.Backend))
	if backend == "" {
		return nil, fmt.Errorf("indicator.backend must not be empty")
	}
	if backend != "hypr" && backend != "desktop" {
		return nil, fmt.Errorf("indicator.backend must be one of: hypr, desktop")
	}
	if backend == "desktop" && strings.TrimSpace(cfg.Indicator.DesktopAppName) == "" {
		return nil, fmt.Errorf("indicator.desktop_app_name must not be empty when indicator.backend=desktop")
	}
	if cfg.Indicator.Height <= 0 {
		return nil, fmt.Errorf("indicator.height must be > 0")
	}
	if cfg.Indicator.ErrorTimeoutMS < 0 {
		return nil, fmt.Errorf("indicator.error_timeout_ms must be >= 0")
	}
	if cfg.Vocab.MaxPhrases <= 0 {
		return nil, fmt.Errorf("vocab.max_phrases must be > 0")
	}
	if len(cfg.Clipboard.Argv) == 0 {
		return nil, fmt.Errorf("clipboard_cmd must not be empty")
	}

	if cfg.Paste.Enable && cfg.PasteCmd.Raw != "" && len(cfg.PasteCmd.Argv) == 0 {
		return nil, fmt.Errorf("paste_cmd is configured but empty")
	}
	if cfg.Paste.Enable && len(cfg.PasteCmd.Argv) == 0 && strings.TrimSpace(cfg.Paste.Shortcut) == "" {
		return nil, fmt.Errorf("paste.shortcut must not be empty when paste.enable=true and paste_cmd is unset")
	}

	_, vocabWarnings, err := BuildSpeechPhrases(cfg)
	if err != nil {
		return nil, err
	}
	warnings = append(warnings, vocabWarnings...)

	return warnings, nil
}

// BuildSpeechPhrases merges enabled vocab sets into deterministic ASR phrase payloads.
func BuildSpeechPhrases(cfg Config) ([]SpeechPhrase, []Warning, error) {
	enabledSets := cfg.Vocab.GlobalSets
	if len(enabledSets) == 0 {
		return nil, nil, nil
	}

	type candidate struct {
		boost float64
		from  string
	}

	warnings := make([]Warning, 0)
	selected := make(map[string]candidate)

	for _, name := range enabledSets {
		set, ok := cfg.Vocab.Sets[name]
		if !ok {
			return nil, nil, fmt.Errorf("vocab.global references unknown set %q", name)
		}
		for _, phrase := range set.Phrases {
			phrase = strings.TrimSpace(phrase)
			if phrase == "" {
				continue
			}
			if existing, exists := selected[phrase]; exists {
				if set.Boost > existing.boost {
					warnings = append(warnings, Warning{Message: fmt.Sprintf("phrase %q present in %q and %q; using higher boost %.2f", phrase, existing.from, name, set.Boost)})
					selected[phrase] = candidate{boost: set.Boost, from: name}
				}
				continue
			}
			selected[phrase] = candidate{boost: set.Boost, from: name}
		}
	}

	if len(selected) > cfg.Vocab.MaxPhrases {
		return nil, nil, fmt.Errorf("vocabulary phrase count %d exceeds vocab.max_phrases=%d", len(selected), cfg.Vocab.MaxPhrases)
	}

	phrases := make([]SpeechPhrase, 0, len(selected))
	for phrase, c := range selected {
		phrases = append(phrases, SpeechPhrase{Phrase: phrase, Boost: float32(c.boost)})
	}

	sort.Slice(phrases, func(i, j int) bool {
		if phrases[i].Phrase == phrases[j].Phrase {
			return phrases[i].Boost < phrases[j].Boost
		}
		return phrases[i].Phrase < phrases[j].Phrase
	})

	return phrases, warnings, nil
}
