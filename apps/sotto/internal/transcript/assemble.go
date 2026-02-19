package transcript

import "strings"

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
