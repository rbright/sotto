package transcript

import "strings"

// Assemble joins final ASR segments and applies whitespace/trailing-space normalization.
func Assemble(finalSegments []string, trailingSpace bool) string {
	if len(finalSegments) == 0 {
		return ""
	}

	joined := strings.Join(finalSegments, " ")
	normalized := strings.Join(strings.Fields(joined), " ")
	if normalized == "" {
		return ""
	}

	if trailingSpace {
		return normalized + " "
	}
	return normalized
}
