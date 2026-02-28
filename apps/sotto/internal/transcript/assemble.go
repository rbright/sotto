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

	capitalizeStart := true
	pendingBoundary := false
	sawWhitespaceAfterBoundary := false

	for _, r := range text {
		if capitalizeStart && unicode.IsLetter(r) {
			r = unicode.ToUpper(r)
			capitalizeStart = false
		} else if pendingBoundary {
			switch {
			case unicode.IsSpace(r):
				sawWhitespaceAfterBoundary = true
			case unicode.IsLetter(r):
				if sawWhitespaceAfterBoundary {
					r = unicode.ToUpper(r)
				}
				pendingBoundary = false
				sawWhitespaceAfterBoundary = false
			case unicode.IsDigit(r):
				pendingBoundary = false
				sawWhitespaceAfterBoundary = false
			case isSentencePrefixRune(r):
				// Keep waiting for a letter. This supports punctuation like: . "quote"
			default:
				if !sawWhitespaceAfterBoundary {
					pendingBoundary = false
					sawWhitespaceAfterBoundary = false
				}
			}
		}

		out.WriteRune(r)

		switch r {
		case '.', '!', '?':
			pendingBoundary = true
			sawWhitespaceAfterBoundary = false
		}
	}

	return out.String()
}

func isSentencePrefixRune(r rune) bool {
	switch r {
	case ')', ']', '}', '\'', '"', '’', '”':
		return true
	default:
		return false
	}
}
