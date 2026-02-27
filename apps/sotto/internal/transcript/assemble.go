// Package transcript assembles and normalizes recognized ASR segments.
package transcript

import (
	"regexp"
	"strings"
	"unicode"
)

// Options controls transcript assembly formatting behavior.
type Options struct {
	TrailingSpace       bool
	CapitalizeSentences bool
}

var (
	pronounIContractionPattern = regexp.MustCompile(`\bi['’](?:m|d|ll|ve|re|s)\b`)
	pronounIWordPattern        = regexp.MustCompile(`\bi\b`)
)

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
	return pronounIWordPattern.ReplaceAllString(text, "I")
}

func capitalizeSentenceStarts(text string) string {
	var out strings.Builder
	out.Grow(len(text))

	capitalizeNext := true
	for _, r := range text {
		if capitalizeNext && unicode.IsLetter(r) {
			r = unicode.ToUpper(r)
			capitalizeNext = false
		} else if unicode.IsLetter(r) {
			capitalizeNext = false
		}

		out.WriteRune(r)

		switch r {
		case '.', '!', '?':
			capitalizeNext = true
		}
	}

	return out.String()
}
