package indicator

import (
	"os"
	"strings"
)

type locale string

const (
	localeEnglish locale = "en"
)

type messages struct {
	recording  string
	processing string
	errorText  string
}

func indicatorMessagesFromEnv() messages {
	return indicatorMessages(resolveLocale(os.Getenv("LANG")))
}

func resolveLocale(raw string) locale {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if strings.HasPrefix(raw, "en") {
		return localeEnglish
	}
	return localeEnglish
}

func indicatorMessages(tag locale) messages {
	switch tag {
	case localeEnglish:
		fallthrough
	default:
		return messages{
			recording:  "Recording…",
			processing: "Transcribing…",
			errorText:  "Speech recognition error",
		}
	}
}
