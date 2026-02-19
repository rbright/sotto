package indicator

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/rbright/sotto/internal/config"
	"github.com/stretchr/testify/require"
)

func TestCueSamplesPresent(t *testing.T) {
	require.NotEmpty(t, cueSamples(cueStart))
	require.NotEmpty(t, cueSamples(cueStop))
	require.NotEmpty(t, cueSamples(cueComplete))
	require.NotEmpty(t, cueSamples(cueCancel))
}

func TestSynthesizeToneDuration(t *testing.T) {
	got := synthesizeTone(toneSpec{frequencyHz: 440, duration: 100 * time.Millisecond, volume: 0.2})
	want := samplesForDuration(100 * time.Millisecond)
	require.Len(t, got, want)
}

func TestSynthesizeToneInvalidSpecReturnsEmpty(t *testing.T) {
	require.Empty(t, synthesizeTone(toneSpec{frequencyHz: 0, duration: 100 * time.Millisecond, volume: 0.2}))
	require.Empty(t, synthesizeTone(toneSpec{frequencyHz: 440, duration: 0, volume: 0.2}))
	require.Empty(t, synthesizeTone(toneSpec{frequencyHz: 440, duration: 100 * time.Millisecond, volume: 0}))
}

func TestCuePathMapping(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := config.IndicatorConfig{
		SoundStartFile:    "~/start.wav",
		SoundStopFile:     "/tmp/stop.wav",
		SoundCompleteFile: "/tmp/complete.wav",
		SoundCancelFile:   "/tmp/cancel.wav",
	}

	require.Equal(t, filepath.Join(home, "start.wav"), cuePath(cueStart, cfg))
	require.Equal(t, "/tmp/stop.wav", cuePath(cueStop, cfg))
	require.Equal(t, "/tmp/complete.wav", cuePath(cueComplete, cfg))
	require.Equal(t, "/tmp/cancel.wav", cuePath(cueCancel, cfg))
}

func TestExpandUserPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	require.Equal(t, home, expandUserPath("~"))
	require.Equal(t, filepath.Join(home, "Downloads", "sound.wav"), expandUserPath("~/Downloads/sound.wav"))
	require.Equal(t, "/tmp/sound.wav", expandUserPath("/tmp/sound.wav"))
	require.Empty(t, expandUserPath("   "))
}

func TestSamplesForDuration(t *testing.T) {
	require.Equal(t, 0, samplesForDuration(0))
	require.Greater(t, samplesForDuration(25*time.Millisecond), 0)
}
