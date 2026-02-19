package riva

import "strings"

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

	prevWords := strings.Fields(previous)
	currWords := strings.Fields(current)
	common := commonPrefixWords(prevWords, currWords)
	shorter := len(prevWords)
	if len(currWords) < shorter {
		shorter = len(currWords)
	}
	if shorter == 0 {
		return true
	}
	return common*2 >= shorter
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

// cleanSegment normalizes transcript whitespace.
func cleanSegment(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	return strings.Join(strings.Fields(raw), " ")
}
