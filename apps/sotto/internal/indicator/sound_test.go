package indicator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCueSamplesPresent(t *testing.T) {
	require.NotEmpty(t, cueSamples(cueStart))
	require.NotEmpty(t, cueSamples(cueStop))
	require.NotEmpty(t, cueSamples(cueComplete))
	require.NotEmpty(t, cueSamples(cueCancel))
}

func TestCueEmbeddedWAVPresent(t *testing.T) {
	require.NotEmpty(t, cueWAV(cueStart))
	require.NotEmpty(t, cueWAV(cueStop))
	require.NotEmpty(t, cueWAV(cueComplete))
	require.NotEmpty(t, cueWAV(cueCancel))
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

func TestSamplesForDuration(t *testing.T) {
	require.Equal(t, 0, samplesForDuration(0))
	require.Greater(t, samplesForDuration(25*time.Millisecond), 0)
}

func TestEmitCueRespectsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := emitCue(ctx, cueStart)
	require.Error(t, err)
	require.True(t, errors.Is(err, context.Canceled))
}
