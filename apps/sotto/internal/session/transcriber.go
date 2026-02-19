package session

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrPipelineUnavailable indicates runtime transcriber wiring is missing.
	ErrPipelineUnavailable = errors.New("audio capture and ASR pipeline not implemented")
	// ErrEmptyTranscript indicates stop completed but no usable speech was recognized.
	ErrEmptyTranscript = errors.New("no speech recognized; check microphone input or mute state")
)

// StopResult is the transcriber output consumed by the session controller.
type StopResult struct {
	Transcript    string
	AudioDevice   string
	BytesCaptured int64
	GRPCLatency   time.Duration
}

// Transcriber abstracts capture/ASR operations needed by session orchestration.
type Transcriber interface {
	Start(context.Context) error
	StopAndTranscribe(context.Context) (StopResult, error)
	Cancel(context.Context) error
}

// PlaceholderTranscriber is a no-op placeholder used in tests/fallback wiring.
type PlaceholderTranscriber struct{}

func (PlaceholderTranscriber) Start(context.Context) error {
	return nil
}

func (PlaceholderTranscriber) StopAndTranscribe(context.Context) (StopResult, error) {
	return StopResult{}, ErrPipelineUnavailable
}

func (PlaceholderTranscriber) Cancel(context.Context) error {
	return nil
}
