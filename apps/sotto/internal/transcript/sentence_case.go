package transcript

import (
	"strings"
	"unicode"
)

func capitalizeSentenceStarts(text string) string {
	runes := []rune(text)

	var out strings.Builder
	out.Grow(len(text))

	capitalizeStart := true
	pendingBoundary := false
	sawWhitespaceAfterBoundary := false

	for i, r := range runes {
		if capitalizeStart && unicode.IsLetter(r) {
			if shouldCapitalizeWordAt(runes, i) {
				r = unicode.ToUpper(r)
			}
			capitalizeStart = false
			pendingBoundary = false
			sawWhitespaceAfterBoundary = false
		} else if pendingBoundary {
			switch {
			case unicode.IsSpace(r):
				sawWhitespaceAfterBoundary = true
			case unicode.IsLetter(r):
				if sawWhitespaceAfterBoundary && shouldCapitalizeWordAt(runes, i) {
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
		case '.':
			if isSentenceBoundaryPeriod(runes, i) {
				pendingBoundary = true
				sawWhitespaceAfterBoundary = false
			} else {
				pendingBoundary = false
				sawWhitespaceAfterBoundary = false
			}
		case '!', '?':
			pendingBoundary = true
			sawWhitespaceAfterBoundary = false
		}
	}

	return out.String()
}

func shouldCapitalizeWordAt(runes []rune, idx int) bool {
	token := strings.ToLower(strings.Trim(wordTokenFromIndex(runes, idx), "."))
	if token == "" {
		return true
	}
	return !isLowercaseSentenceAbbreviation(token)
}

func wordTokenFromIndex(runes []rune, idx int) string {
	if idx < 0 || idx >= len(runes) {
		return ""
	}

	end := idx
	for end < len(runes) {
		r := runes[end]
		if unicode.IsLetter(r) || r == '.' {
			end++
			continue
		}
		break
	}

	return string(runes[idx:end])
}

func isSentencePrefixRune(r rune) bool {
	switch r {
	case ')', ']', '}', '\'', '"', '’', '”':
		return true
	default:
		return false
	}
}
