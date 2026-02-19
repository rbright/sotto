package session

import (
	"context"
	"errors"
	"time"
)

var (
	ErrPipelineUnavailable = errors.New("audio capture and ASR pipeline not implemented")
	ErrEmptyTranscript     = errors.New("no speech recognized; check microphone input or mute state")
)

type StopResult struct {
	Transcript    string
	AudioDevice   string
	BytesCaptured int64
	GRPCLatency   time.Duration
}

type Transcriber interface {
	Start(context.Context) error
	StopAndTranscribe(context.Context) (StopResult, error)
	Cancel(context.Context) error
}

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
