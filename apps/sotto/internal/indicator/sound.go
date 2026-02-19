package indicator

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jfreymuth/pulse"
	"github.com/rbright/sotto/internal/config"
)

type cueKind int

const (
	cueStart cueKind = iota + 1
	cueStop
	cueComplete
	cueCancel
)

const cueSampleRate = 16000

type toneSpec struct {
	frequencyHz float64
	duration    time.Duration
	volume      float64
}

var (
	startCuePCM = synthesizeCue([]toneSpec{
		{frequencyHz: 880, duration: 70 * time.Millisecond, volume: 0.18},
		{frequencyHz: 1175, duration: 70 * time.Millisecond, volume: 0.18},
	})
	stopCuePCM = synthesizeCue([]toneSpec{
		{frequencyHz: 620, duration: 120 * time.Millisecond, volume: 0.18},
	})
	completeCuePCM = synthesizeCue([]toneSpec{
		{frequencyHz: 740, duration: 65 * time.Millisecond, volume: 0.18},
		{frequencyHz: 988, duration: 90 * time.Millisecond, volume: 0.18},
	})
	cancelCuePCM = synthesizeCue([]toneSpec{
		{frequencyHz: 480, duration: 75 * time.Millisecond, volume: 0.18},
		{frequencyHz: 360, duration: 90 * time.Millisecond, volume: 0.18},
	})
)

func emitCue(kind cueKind, cfg config.IndicatorConfig) error {
	if path := cuePath(kind, cfg); path != "" {
		if err := playCueFile(path); err == nil {
			return nil
		}
	}

	samples := cueSamples(kind)
	if len(samples) == 0 {
		return nil
	}

	return playSynthCue(samples)
}

func cuePath(kind cueKind, cfg config.IndicatorConfig) string {
	var raw string
	switch kind {
	case cueStart:
		raw = cfg.SoundStartFile
	case cueStop:
		raw = cfg.SoundStopFile
	case cueComplete:
		raw = cfg.SoundCompleteFile
	case cueCancel:
		raw = cfg.SoundCancelFile
	default:
		return ""
	}
	return expandUserPath(raw)
}

func expandUserPath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if raw == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return raw
		}
		return home
	}
	if !strings.HasPrefix(raw, "~/") {
		return raw
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return raw
	}
	return filepath.Join(home, strings.TrimPrefix(raw, "~/"))
}

func playCueFile(path string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("stat cue file %q: %w", path, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "pw-play", "--media-role", "Notification", path)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("play cue file %q: %w", path, err)
	}
	return nil
}

func playSynthCue(samples []int16) error {
	client, err := pulse.NewClient(
		pulse.ClientApplicationName("sotto"),
		pulse.ClientApplicationIconName("audio-input-microphone"),
	)
	if err != nil {
		return fmt.Errorf("connect pulse server: %w", err)
	}
	defer client.Close()

	cursor := 0
	reader := pulse.Int16Reader(func(buf []int16) (int, error) {
		if cursor >= len(samples) {
			return 0, pulse.EndOfData
		}

		n := copy(buf, samples[cursor:])
		cursor += n
		if cursor >= len(samples) {
			return n, pulse.EndOfData
		}
		return n, nil
	})

	stream, err := client.NewPlayback(
		reader,
		pulse.PlaybackMono,
		pulse.PlaybackSampleRate(cueSampleRate),
		pulse.PlaybackLatency(0.02),
		pulse.PlaybackMediaName("sotto indicator cue"),
	)
	if err != nil {
		return fmt.Errorf("create pulse playback stream: %w", err)
	}
	defer stream.Close()

	stream.Start()
	stream.Drain()
	if err := stream.Error(); err != nil {
		return fmt.Errorf("play cue stream: %w", err)
	}

	return nil
}

func cueSamples(kind cueKind) []int16 {
	switch kind {
	case cueStart:
		return startCuePCM
	case cueStop:
		return stopCuePCM
	case cueComplete:
		return completeCuePCM
	case cueCancel:
		return cancelCuePCM
	default:
		return nil
	}
}

func synthesizeCue(parts []toneSpec) []int16 {
	if len(parts) == 0 {
		return nil
	}
	gapSamples := samplesForDuration(22 * time.Millisecond)
	total := 0
	for i, part := range parts {
		total += samplesForDuration(part.duration)
		if i < len(parts)-1 {
			total += gapSamples
		}
	}

	pcm := make([]int16, 0, total)
	for i, part := range parts {
		pcm = append(pcm, synthesizeTone(part)...)
		if i < len(parts)-1 && gapSamples > 0 {
			pcm = append(pcm, make([]int16, gapSamples)...)
		}
	}

	return pcm
}

func synthesizeTone(spec toneSpec) []int16 {
	n := samplesForDuration(spec.duration)
	if n <= 0 || spec.frequencyHz <= 0 || spec.volume <= 0 {
		return nil
	}

	attackRelease := n / 10
	maxRamp := cueSampleRate / 200 // 5ms
	if attackRelease > maxRamp {
		attackRelease = maxRamp
	}
	if attackRelease < 1 {
		attackRelease = 1
	}

	pcm := make([]int16, n)
	for i := 0; i < n; i++ {
		envelope := 1.0
		if i < attackRelease {
			envelope = float64(i) / float64(attackRelease)
		}
		releaseIndex := n - i - 1
		if releaseIndex < attackRelease {
			release := float64(releaseIndex) / float64(attackRelease)
			if release < envelope {
				envelope = release
			}
		}
		t := float64(i) / cueSampleRate
		sample := math.Sin(2 * math.Pi * spec.frequencyHz * t)
		pcm[i] = int16(math.Round(sample * spec.volume * envelope * 32767))
	}

	return pcm
}

func samplesForDuration(d time.Duration) int {
	if d <= 0 {
		return 0
	}
	return int(math.Round(d.Seconds() * cueSampleRate))
}
