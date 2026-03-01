package transcript

import (
	"strings"
	"unicode"
)

type abbreviationBoundaryClass uint8

const (
	abbreviationBoundaryNonTerminal abbreviationBoundaryClass = iota
	abbreviationBoundaryAmbiguous
)

type periodBoundaryReason string

const (
	periodBoundaryReasonDefault               periodBoundaryReason = "default"
	periodBoundaryReasonEmbeddedToken         periodBoundaryReason = "embedded-token"
	periodBoundaryReasonDecimal               periodBoundaryReason = "decimal"
	periodBoundaryReasonInitialism            periodBoundaryReason = "initialism"
	periodBoundaryReasonInitialismBoundary    periodBoundaryReason = "initialism-boundary"
	periodBoundaryReasonKnownAbbreviation     periodBoundaryReason = "known-abbreviation"
	periodBoundaryReasonAmbiguousConservative periodBoundaryReason = "ambiguous-abbreviation-conservative"
	periodBoundaryReasonAmbiguousBoundary     periodBoundaryReason = "ambiguous-abbreviation-boundary"
)

var (
	// lowercaseSentenceAbbreviations should stay lowercase even at sentence starts.
	lowercaseSentenceAbbreviations = map[string]struct{}{
		"e.g": {},
		"etc": {},
		"i.e": {},
		"vs":  {},
	}

	// sentenceBoundaryAbbreviationClasses classifies tokens that frequently appear
	// before a non-terminal period.
	sentenceBoundaryAbbreviationClasses = map[string]abbreviationBoundaryClass{
		// Latin/editorial abbreviations.
		"e.g": abbreviationBoundaryNonTerminal,
		"i.e": abbreviationBoundaryNonTerminal,
		"cf":  abbreviationBoundaryNonTerminal,
		"etc": abbreviationBoundaryAmbiguous,
		"vs":  abbreviationBoundaryAmbiguous,

		// Titles/honorifics.
		"dr":   abbreviationBoundaryNonTerminal,
		"mr":   abbreviationBoundaryNonTerminal,
		"mrs":  abbreviationBoundaryNonTerminal,
		"ms":   abbreviationBoundaryNonTerminal,
		"prof": abbreviationBoundaryNonTerminal,
		"sr":   abbreviationBoundaryNonTerminal,
		"jr":   abbreviationBoundaryNonTerminal,

		// Reference markers.
		"ch":  abbreviationBoundaryNonTerminal,
		"eq":  abbreviationBoundaryNonTerminal,
		"fig": abbreviationBoundaryNonTerminal,
		"ref": abbreviationBoundaryNonTerminal,
		"sec": abbreviationBoundaryNonTerminal,

		// Units/time abbreviations frequently used in dictation.
		"hr":   abbreviationBoundaryNonTerminal,
		"hrs":  abbreviationBoundaryNonTerminal,
		"lb":   abbreviationBoundaryNonTerminal,
		"lbs":  abbreviationBoundaryNonTerminal,
		"min":  abbreviationBoundaryNonTerminal,
		"mins": abbreviationBoundaryNonTerminal,
		"oz":   abbreviationBoundaryNonTerminal,
		"tbsp": abbreviationBoundaryNonTerminal,
		"tsp":  abbreviationBoundaryNonTerminal,
	}

	// lowercaseBoundaryPromoters captures lowercase words that strongly indicate
	// a sentence boundary after ambiguous abbreviations/initialisms in ASR text.
	// Keep this list intentionally narrow to avoid false positives like
	// `etc. and` or `u.s. and`.
	lowercaseBoundaryPromoters = map[string]struct{}{
		"finally":   {},
		"however":   {},
		"meanwhile": {},
		"next":      {},
		"then":      {},
		"therefore": {},
	}

	lowercasePronounBoundaryPromoters = map[string]struct{}{
		"he":   {},
		"i":    {},
		"it":   {},
		"she":  {},
		"they": {},
		"we":   {},
		"you":  {},
	}

	locativePrepositions = map[string]struct{}{
		"across":     {},
		"around":     {},
		"at":         {},
		"from":       {},
		"in":         {},
		"inside":     {},
		"near":       {},
		"outside":    {},
		"through":    {},
		"throughout": {},
		"to":         {},
		"within":     {},
	}
)

func isSentenceBoundaryPeriod(runes []rune, idx int) bool {
	boundary, _ := classifyPeriodBoundary(runes, idx)
	return boundary
}

func classifyPeriodBoundary(runes []rune, idx int) (bool, periodBoundaryReason) {
	if idx < 0 || idx >= len(runes) || runes[idx] != '.' {
		return false, periodBoundaryReasonDefault
	}

	if isDecimalPeriod(runes, idx) {
		return false, periodBoundaryReasonDecimal
	}

	if isEmbeddedPeriodToken(runes, idx) {
		return false, periodBoundaryReasonEmbeddedToken
	}

	token := strings.ToLower(sentenceTokenBeforePeriod(runes, idx))
	if token == "" {
		return true, periodBoundaryReasonDefault
	}
	if class, ok := sentenceBoundaryAbbreviationClasses[token]; ok {
		switch class {
		case abbreviationBoundaryNonTerminal:
			return false, periodBoundaryReasonKnownAbbreviation
		case abbreviationBoundaryAmbiguous:
			if shouldTreatAbbreviationAsBoundary(runes, idx, token) {
				return true, periodBoundaryReasonAmbiguousBoundary
			}
			return false, periodBoundaryReasonAmbiguousConservative
		}
	}

	if looksLikeInitialismToken(token) {
		if shouldTreatAbbreviationAsBoundary(runes, idx, token) {
			return true, periodBoundaryReasonInitialismBoundary
		}
		return false, periodBoundaryReasonInitialism
	}

	return true, periodBoundaryReasonDefault
}

func isDecimalPeriod(runes []rune, idx int) bool {
	if idx <= 0 || idx+1 >= len(runes) {
		return false
	}
	return unicode.IsDigit(runes[idx-1]) && unicode.IsDigit(runes[idx+1])
}

func isEmbeddedPeriodToken(runes []rune, idx int) bool {
	if idx+1 >= len(runes) {
		return false
	}

	next := runes[idx+1]
	return unicode.IsLetter(next) || unicode.IsDigit(next) || next == '.'
}

func shouldTreatAbbreviationAsBoundary(runes []rune, idx int, token string) bool {
	nextWordStart := nextSentenceWordStart(runes, idx+1)
	if nextWordStart < 0 {
		return true
	}
	if unicode.IsUpper(runes[nextWordStart]) {
		return true
	}

	nextWord := strings.ToLower(sentenceWordFromIndex(runes, nextWordStart))
	if isLowercaseBoundaryPromoter(nextWord) {
		return true
	}
	if !isLowercasePronounBoundaryPromoter(nextWord) {
		return false
	}
	if looksLikeInitialismToken(token) && isLikelyLocativeInitialismContinuation(runes, idx) {
		return false
	}
	return true
}

func sentenceWordFromIndex(runes []rune, idx int) string {
	if idx < 0 || idx >= len(runes) {
		return ""
	}

	end := idx
	for end < len(runes) {
		if unicode.IsLetter(runes[end]) {
			end++
			continue
		}
		break
	}

	return string(runes[idx:end])
}

func nextSentenceWordStart(runes []rune, start int) int {
	for i := start; i < len(runes); i++ {
		r := runes[i]
		switch {
		case unicode.IsSpace(r):
			continue
		case isSentencePrefixRune(r):
			continue
		case unicode.IsLetter(r):
			return i
		default:
			return -1
		}
	}
	return -1
}

func isLowercaseSentenceAbbreviation(token string) bool {
	_, ok := lowercaseSentenceAbbreviations[token]
	return ok
}

func isLowercaseBoundaryPromoter(word string) bool {
	_, ok := lowercaseBoundaryPromoters[word]
	return ok
}

func isLowercasePronounBoundaryPromoter(word string) bool {
	_, ok := lowercasePronounBoundaryPromoters[word]
	return ok
}

func isLikelyLocativeInitialismContinuation(runes []rune, idx int) bool {
	tokenStart := sentenceTokenStart(runes, idx)
	if tokenStart < 0 {
		return false
	}

	prevWord, prevStart := previousWordBeforeIndex(runes, tokenStart)
	if prevWord == "" {
		return false
	}
	if isLocativePreposition(prevWord) {
		return isSentenceLeadingWord(runes, prevStart)
	}

	if !isArticleWord(prevWord) || prevStart <= 0 {
		return false
	}

	prepositionWord, prepositionStart := previousWordBeforeIndex(runes, prevStart)
	if !isLocativePreposition(prepositionWord) {
		return false
	}
	return isSentenceLeadingWord(runes, prepositionStart)
}

func sentenceTokenStart(runes []rune, idx int) int {
	if idx <= 0 || idx >= len(runes) {
		return -1
	}

	start := idx - 1
	for start >= 0 {
		if r := runes[start]; unicode.IsLetter(r) || r == '.' {
			start--
			continue
		}
		break
	}

	return start + 1
}

func previousWordBeforeIndex(runes []rune, idx int) (string, int) {
	if idx <= 0 || idx > len(runes) {
		return "", -1
	}

	i := idx - 1
	for i >= 0 && !unicode.IsLetter(runes[i]) {
		i--
	}
	if i < 0 {
		return "", -1
	}

	end := i + 1
	for i >= 0 && unicode.IsLetter(runes[i]) {
		i--
	}
	start := i + 1
	return strings.ToLower(string(runes[start:end])), start
}

func isLocativePreposition(word string) bool {
	_, ok := locativePrepositions[word]
	return ok
}

func isArticleWord(word string) bool {
	switch word {
	case "a", "an", "the":
		return true
	default:
		return false
	}
}

func isSentenceLeadingWord(runes []rune, wordStart int) bool {
	if wordStart <= 0 {
		return true
	}

	i := wordStart - 1
	for i >= 0 {
		r := runes[i]
		switch {
		case unicode.IsSpace(r):
			i--
			continue
		case isSentencePrefixRune(r):
			i--
			continue
		}
		break
	}

	if i < 0 {
		return true
	}
	switch runes[i] {
	case '.', '!', '?':
		return true
	default:
		return false
	}
}

func sentenceTokenBeforePeriod(runes []rune, idx int) string {
	if idx <= 0 || idx >= len(runes) {
		return ""
	}

	start := idx - 1
	for start >= 0 {
		if r := runes[start]; unicode.IsLetter(r) || r == '.' {
			start--
			continue
		}
		break
	}

	return strings.Trim(string(runes[start+1:idx]), ".")
}

func looksLikeInitialismToken(token string) bool {
	if !strings.ContainsRune(token, '.') {
		return false
	}

	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return false
	}

	for _, part := range parts {
		runes := []rune(part)
		if len(runes) != 1 || !unicode.IsLetter(runes[0]) {
			return false
		}
	}

	return true
}
