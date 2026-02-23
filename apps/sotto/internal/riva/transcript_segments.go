package riva

import "strings"

const stableInterimBoundaryThreshold = 0.85

// collectSegments appends a valid trailing interim segment when needed.
func collectSegments(committedSegments []string, lastInterim string) []string {
	segments := append([]string(nil), committedSegments...)
	if interim := cleanSegment(lastInterim); interim != "" {
		segments = appendSegment(segments, interim)
	}
	return segments
}

// appendSegment merges continuation segments to avoid duplicate transcript growth.
func appendSegment(segments []string, transcript string) []string {
	transcript = cleanSegment(transcript)
	if transcript == "" {
		return segments
	}
	if len(segments) == 0 {
		return append(segments, transcript)
	}

	last := cleanSegment(segments[len(segments)-1])
	switch {
	case transcript == last:
		return segments
	case strings.HasPrefix(transcript, last):
		segments[len(segments)-1] = transcript
		return segments
	case strings.HasPrefix(last, transcript):
		return segments
	default:
		return append(segments, transcript)
	}
}

// isInterimContinuation decides whether an interim update extends prior speech.
func isInterimContinuation(previous string, current string) bool {
	previous = cleanSegment(previous)
	current = cleanSegment(current)
	if previous == "" || current == "" {
		return true
	}
	if previous == current {
		return true
	}
	if strings.HasPrefix(current, previous) || strings.HasPrefix(previous, current) {
		return true
	}
	if strings.HasSuffix(current, previous) || strings.HasSuffix(previous, current) {
		return true
	}

	prevWords := strings.Fields(previous)
	currWords := strings.Fields(current)
	shorter := len(prevWords)
	if len(currWords) < shorter {
		shorter = len(currWords)
	}
	if shorter == 0 {
		return true
	}

	commonPrefix := commonPrefixWords(prevWords, currWords)
	if commonPrefix*2 >= shorter {
		return true
	}

	commonSuffix := commonSuffixWords(prevWords, currWords)
	if shorter >= 3 && commonSuffix*2 >= shorter {
		return true
	}

	return false
}

// shouldCommitPriorInterimOnDivergence decides whether to preserve prior interim
// text when a new interim hypothesis diverges.
func shouldCommitPriorInterimOnDivergence(previous string, previousStability float32, current string) bool {
	previous = cleanSegment(previous)
	current = cleanSegment(current)
	if previous == "" || current == "" {
		return false
	}
	if isInterimContinuation(previous, current) {
		return false
	}
	if previousStability >= stableInterimBoundaryThreshold {
		return true
	}
	return endsWithSentencePunctuation(previous)
}

// endsWithSentencePunctuation reports whether transcript looks sentence-complete.
func endsWithSentencePunctuation(transcript string) bool {
	transcript = strings.TrimSpace(transcript)
	if transcript == "" {
		return false
	}
	switch transcript[len(transcript)-1] {
	case '.', '!', '?':
		return true
	default:
		return false
	}
}

// commonPrefixWords counts shared leading words across two slices.
func commonPrefixWords(left []string, right []string) int {
	limit := len(left)
	if len(right) < limit {
		limit = len(right)
	}
	count := 0
	for i := 0; i < limit; i++ {
		if left[i] != right[i] {
			break
		}
		count++
	}
	return count
}

// commonSuffixWords counts shared trailing words across two slices.
func commonSuffixWords(left []string, right []string) int {
	li := len(left) - 1
	ri := len(right) - 1
	count := 0
	for li >= 0 && ri >= 0 {
		if left[li] != right[ri] {
			break
		}
		count++
		li--
		ri--
	}
	return count
}

// cleanSegment normalizes transcript whitespace.
func cleanSegment(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	return strings.Join(strings.Fields(raw), " ")
}
