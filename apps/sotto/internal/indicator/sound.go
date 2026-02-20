package indicator

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"math"
	"os/exec"
	"time"

	"github.com/jfreymuth/pulse"
)

// cueKind identifies each cue event used by the session lifecycle.
type cueKind int

const (
	cueStart cueKind = iota + 1
	cueStop
	cueComplete
	cueCancel
)

const cueSampleRate = 16000

// toneSpec describes one synthesized cue tone segment.
type toneSpec struct {
	frequencyHz float64
	duration    time.Duration
	volume      float64
}

var (
	//go:embed assets/toggle_on.wav assets/toggle_off.wav assets/complete.wav assets/cancel.wav
	cueAssetFS embed.FS

	startCueWAV    = mustCueWAV("assets/toggle_on.wav")
	stopCueWAV     = mustCueWAV("assets/toggle_off.wav")
	completeCueWAV = mustCueWAV("assets/complete.wav")
	cancelCueWAV   = mustCueWAV("assets/cancel.wav")

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

// emitCue plays an embedded WAV cue when available, then falls back to synthesis.
func emitCue(ctx context.Context, kind cueKind) error {
	if ctx == nil {
		ctx = context.Background()
	}

	if data := cueWAV(kind); len(data) > 0 {
		if err := playCueData(ctx, data); err == nil {
			return nil
		}
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	samples := cueSamples(kind)
	if len(samples) == 0 {
		return nil
	}

	return playSynthCue(samples)
}

func cueWAV(kind cueKind) []byte {
	switch kind {
	case cueStart:
		return startCueWAV
	case cueStop:
		return stopCueWAV
	case cueComplete:
		return completeCueWAV
	case cueCancel:
		return cancelCueWAV
	default:
		return nil
	}
}

func mustCueWAV(path string) []byte {
	data, err := cueAssetFS.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("load embedded cue %q: %v", path, err))
	}
	return data
}

// playCueData plays an embedded WAV payload through pw-play.
func playCueData(ctx context.Context, data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("embedded cue payload is empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	runCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "pw-play", "--media-role", "Notification", "-")
	cmd.Stdin = bytes.NewReader(data)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("play embedded cue: %w", err)
	}
	return nil
}

// playSynthCue streams synthesized PCM samples through Pulse playback.
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

// cueSamples returns the synthesized PCM table for one cue kind.
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

// synthesizeCue concatenates one or more tone segments with short silence gaps.
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

// synthesizeTone creates one windowed sine-wave segment.
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

// samplesForDuration converts a time duration into cue sample count.
func samplesForDuration(d time.Duration) int {
	if d <= 0 {
		return 0
	}
	return int(math.Round(d.Seconds() * cueSampleRate))
}
