package transcript

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	pronounIContractionPattern = regexp.MustCompile(`\bi['’](?:m|d|ll|ve|re|s)\b`)
	pronounIWordPattern        = regexp.MustCompile(`\bi\b`)
)

func capitalizeStandalonePronounI(text string) string {
	matches := pronounIWordPattern.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		return text
	}

	var out strings.Builder
	out.Grow(len(text))

	last := 0
	for _, match := range matches {
		start, end := match[0], match[1]
		out.WriteString(text[last:start])
		if shouldSkipPronounICapitalization(text, start, end) {
			out.WriteString(text[start:end])
		} else {
			out.WriteString("I")
		}
		last = end
	}

	out.WriteString(text[last:])
	return out.String()
}

func shouldSkipPronounICapitalization(text string, start int, end int) bool {
	if end+1 < len(text) && text[end] == '.' {
		nextRune, _ := utf8.DecodeRuneInString(text[end+1:])
		if unicode.IsLetter(nextRune) {
			return true
		}
	}

	if start > 1 && text[start-1] == '.' && end < len(text) && text[end] == '.' {
		prevRune, _ := utf8.DecodeLastRuneInString(text[:start-1])
		if unicode.IsLetter(prevRune) {
			return true
		}
	}

	return false
}
