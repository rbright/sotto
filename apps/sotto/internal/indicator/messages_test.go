package indicator

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveLocaleDefaultsToEnglish(t *testing.T) {
	require.Equal(t, localeEnglish, resolveLocale("en_US.UTF-8"))
	require.Equal(t, localeEnglish, resolveLocale("fr_FR.UTF-8"))
}

func TestIndicatorMessagesEnglish(t *testing.T) {
	msg := indicatorMessages(localeEnglish)
	require.Equal(t, "Recording…", msg.recording)
	require.Equal(t, "Transcribing…", msg.processing)
	require.Equal(t, "Speech recognition error", msg.errorText)
}
