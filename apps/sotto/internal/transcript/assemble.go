// Package transcript assembles and normalizes recognized ASR segments.
package transcript

import "strings"

// Options controls transcript assembly formatting behavior.
type Options struct {
	TrailingSpace       bool
	CapitalizeSentences bool
}

// Assemble joins final ASR segments and applies configured normalization.
func Assemble(finalSegments []string, opts Options) string {
	if len(finalSegments) == 0 {
		return ""
	}

	joined := strings.Join(finalSegments, " ")
	normalized := strings.Join(strings.Fields(joined), " ")
	if normalized == "" {
		return ""
	}

	if opts.CapitalizeSentences {
		normalized = capitalizeSentences(normalized)
	}

	if opts.TrailingSpace {
		return normalized + " "
	}
	return normalized
}

func capitalizeSentences(text string) string {
	text = capitalizeSentenceStarts(text)
	text = pronounIContractionPattern.ReplaceAllStringFunc(text, func(match string) string {
		return "I" + match[1:]
	})
	return capitalizeStandalonePronounI(text)
}
