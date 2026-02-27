// Package riva implements the Riva gRPC streaming client adapter.
package riva

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	asrpb "github.com/rbright/sotto/proto/gen/go/riva/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// SpeechPhrase is one vocabulary boost phrase in request-ready form.
type SpeechPhrase struct {
	Phrase string
	Boost  float32
}

// StreamConfig controls stream initialization and recognition behavior.
type StreamConfig struct {
	Endpoint              string
	LanguageCode          string
	Model                 string
	AutomaticPunctuation  bool
	SpeechPhrases         []SpeechPhrase
	DialTimeout           time.Duration
	DebugResponseSinkJSON io.Writer
}

// Stream wraps one active Riva StreamingRecognize RPC lifecycle.
type Stream struct {
	conn   *grpc.ClientConn
	stream asrpb.RivaSpeechRecognition_StreamingRecognizeClient

	cancel context.CancelFunc

	recvDone chan struct{}

	mu                        sync.Mutex
	segments                  []string // committed transcript segments (final results and sealed interim chains)
	lastInterim               string
	lastInterimAge            int
	lastInterimStability      float32
	lastInterimAudioProcessed float32
	recvErr                   error
	closedSend                bool
	debugSinkJSON             io.Writer
}

// DialStream establishes a stream, sends config, and starts the receive loop.
func DialStream(ctx context.Context, cfg StreamConfig) (*Stream, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		return nil, errors.New("riva endpoint is empty")
	}
	if cfg.DialTimeout <= 0 {
		cfg.DialTimeout = 3 * time.Second
	}
	if strings.TrimSpace(cfg.LanguageCode) == "" {
		cfg.LanguageCode = "en-US"
	}

	conn, err := grpc.NewClient(
		endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("dial riva grpc %q: %w", endpoint, err)
	}

	readyCtx, cancel := context.WithTimeout(ctx, cfg.DialTimeout)
	defer cancel()
	conn.Connect()
	if err := waitForReady(readyCtx, conn); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("wait for riva grpc readiness: %w", err)
	}

	streamCtx, streamCancel := context.WithCancel(ctx)
	client := asrpb.NewRivaSpeechRecognitionClient(conn)
	stream, err := openRecognizeWithTimeout(streamCtx, cfg.DialTimeout, func() (asrpb.RivaSpeechRecognition_StreamingRecognizeClient, error) {
		return client.StreamingRecognize(streamCtx)
	})
	if err != nil {
		streamCancel()
		_ = conn.Close()
		return nil, fmt.Errorf("open streaming recognizer: %w", err)
	}

	req := &asrpb.StreamingRecognizeRequest{
		StreamingRequest: &asrpb.StreamingRecognizeRequest_StreamingConfig{
			StreamingConfig: &asrpb.StreamingRecognitionConfig{
				Config: &asrpb.RecognitionConfig{
					Encoding:                   asrpb.AudioEncoding_LINEAR_PCM,
					SampleRateHertz:            16000,
					LanguageCode:               cfg.LanguageCode,
					EnableAutomaticPunctuation: cfg.AutomaticPunctuation,
					AudioChannelCount:          1,
					Model:                      strings.TrimSpace(cfg.Model),
				},
				InterimResults: true,
			},
		},
	}

	for _, phrase := range cfg.SpeechPhrases {
		phraseText := strings.TrimSpace(phrase.Phrase)
		if phraseText == "" {
			continue
		}
		req.GetStreamingConfig().GetConfig().SpeechContexts = append(
			req.GetStreamingConfig().GetConfig().SpeechContexts,
			&asrpb.SpeechContext{Phrases: []string{phraseText}, Boost: phrase.Boost},
		)
	}

	if err := runWithTimeout(streamCtx, cfg.DialTimeout, func() error {
		return stream.Send(req)
	}); err != nil {
		streamCancel()
		_ = conn.Close()
		return nil, fmt.Errorf("send initial streaming config: %w", err)
	}

	s := &Stream{
		conn:          conn,
		stream:        stream,
		cancel:        streamCancel,
		recvDone:      make(chan struct{}),
		debugSinkJSON: cfg.DebugResponseSinkJSON,
	}
	go s.recvLoop()
	return s, nil
}

// SendAudio sends one chunk of PCM audio over the active stream.
func (s *Stream) SendAudio(chunk []byte) error {
	if len(chunk) == 0 {
		return nil
	}

	s.mu.Lock()
	closed := s.closedSend
	recvErr := s.recvErr
	s.mu.Unlock()

	if closed {
		return errors.New("stream already closed for sending")
	}
	if recvErr != nil {
		return fmt.Errorf("stream receive loop failed: %w", recvErr)
	}

	return s.stream.Send(&asrpb.StreamingRecognizeRequest{
		StreamingRequest: &asrpb.StreamingRecognizeRequest_AudioContent{AudioContent: chunk},
	})
}

// CloseAndCollect closes send-side audio and returns merged transcript segments.
func (s *Stream) CloseAndCollect(ctx context.Context) ([]string, time.Duration, error) {
	closedAt := time.Now()

	s.mu.Lock()
	if !s.closedSend {
		s.closedSend = true
		_ = s.stream.CloseSend()
	}
	s.mu.Unlock()

	select {
	case <-s.recvDone:
	case <-ctx.Done():
		if s.cancel != nil {
			s.cancel()
		}
		_ = s.conn.Close()
		return nil, 0, ctx.Err()
	}
	latency := time.Since(closedAt)

	s.mu.Lock()
	defer s.mu.Unlock()
	defer func() {
		if s.cancel != nil {
			s.cancel()
		}
		_ = s.conn.Close()
	}()

	if s.recvErr != nil {
		return nil, latency, s.recvErr
	}

	segments := collectSegments(s.segments, s.lastInterim)
	return segments, latency, nil
}

// Cancel aborts stream processing and closes the underlying grpc connection.
func (s *Stream) Cancel() error {
	s.mu.Lock()
	if !s.closedSend {
		s.closedSend = true
		_ = s.stream.CloseSend()
	}
	s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
	return s.conn.Close()
}
