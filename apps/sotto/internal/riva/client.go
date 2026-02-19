package riva

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	asrpb "github.com/rbright/sotto/proto/gen/go/riva/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
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

	recvDone chan struct{}

	mu            sync.Mutex
	segments      []string // committed transcript segments (final and pause-committed interim)
	lastInterim   string
	recvErr       error
	closedSend    bool
	debugSinkJSON io.Writer
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

	client := asrpb.NewRivaSpeechRecognitionClient(conn)
	stream, err := client.StreamingRecognize(ctx)
	if err != nil {
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

	if err := stream.Send(req); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("send initial streaming config: %w", err)
	}

	s := &Stream{
		conn:          conn,
		stream:        stream,
		recvDone:      make(chan struct{}),
		debugSinkJSON: cfg.DebugResponseSinkJSON,
	}
	go s.recvLoop()
	return s, nil
}

// recvLoop continuously receives recognition responses until stream close/error.
func (s *Stream) recvLoop() {
	defer close(s.recvDone)

	for {
		resp, err := s.stream.Recv()
		if err == nil {
			s.recordResponse(resp)
			continue
		}
		if errors.Is(err, io.EOF) {
			return
		}

		s.mu.Lock()
		s.recvErr = err
		s.mu.Unlock()
		return
	}
}

// recordResponse merges final/interim segments into stream state.
func (s *Stream) recordResponse(resp *asrpb.StreamingRecognizeResponse) {
	if sink := s.debugSinkJSON; sink != nil {
		b, err := json.Marshal(resp)
		if err == nil {
			_, _ = sink.Write(append(b, '\n'))
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, result := range resp.GetResults() {
		alternatives := result.GetAlternatives()
		if len(alternatives) == 0 {
			continue
		}
		transcript := cleanSegment(alternatives[0].GetTranscript())
		if transcript == "" {
			continue
		}
		if result.GetIsFinal() {
			s.segments = appendSegment(s.segments, transcript)
			s.lastInterim = ""
			continue
		}

		if s.lastInterim != "" && !isInterimContinuation(s.lastInterim, transcript) {
			s.segments = appendSegment(s.segments, s.lastInterim)
		}
		s.lastInterim = transcript
	}
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
		_ = s.conn.Close()
		return nil, 0, ctx.Err()
	}
	latency := time.Since(closedAt)

	s.mu.Lock()
	defer s.mu.Unlock()
	defer func() { _ = s.conn.Close() }()

	if s.recvErr != nil {
		return nil, latency, s.recvErr
	}

	segments := collectSegments(s.segments, s.lastInterim)
	return segments, latency, nil
}

// collectSegments appends a valid trailing interim segment when needed.
func collectSegments(committedSegments []string, lastInterim string) []string {
	segments := append([]string(nil), committedSegments...)
	if interim := cleanSegment(lastInterim); interim != "" {
		segments = appendSegment(segments, interim)
	}
	return segments
}

// appendSegment merges continuation segments to avoid duplicate transcript growth.
func appendSegment(segments []string, transcript string) []string {
	transcript = cleanSegment(transcript)
	if transcript == "" {
		return segments
	}
	if len(segments) == 0 {
		return append(segments, transcript)
	}

	last := cleanSegment(segments[len(segments)-1])
	switch {
	case transcript == last:
		return segments
	case strings.HasPrefix(transcript, last):
		segments[len(segments)-1] = transcript
		return segments
	case strings.HasPrefix(last, transcript):
		return segments
	default:
		return append(segments, transcript)
	}
}

// isInterimContinuation decides whether an interim update extends prior speech.
func isInterimContinuation(previous string, current string) bool {
	previous = cleanSegment(previous)
	current = cleanSegment(current)
	if previous == "" || current == "" {
		return true
	}
	if previous == current {
		return true
	}
	if strings.HasPrefix(current, previous) || strings.HasPrefix(previous, current) {
		return true
	}

	prevWords := strings.Fields(previous)
	currWords := strings.Fields(current)
	common := commonPrefixWords(prevWords, currWords)
	shorter := len(prevWords)
	if len(currWords) < shorter {
		shorter = len(currWords)
	}
	if shorter == 0 {
		return true
	}
	return common*2 >= shorter
}

// commonPrefixWords counts shared leading words across two slices.
func commonPrefixWords(left []string, right []string) int {
	limit := len(left)
	if len(right) < limit {
		limit = len(right)
	}
	count := 0
	for i := 0; i < limit; i++ {
		if left[i] != right[i] {
			break
		}
		count++
	}
	return count
}

// cleanSegment normalizes transcript whitespace.
func cleanSegment(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	return strings.Join(strings.Fields(raw), " ")
}

// Cancel aborts stream processing and closes the underlying grpc connection.
func (s *Stream) Cancel() error {
	s.mu.Lock()
	if !s.closedSend {
		s.closedSend = true
		_ = s.stream.CloseSend()
	}
	s.mu.Unlock()
	return s.conn.Close()
}

// waitForReady blocks until gRPC connection enters Ready or fails.
func waitForReady(ctx context.Context, conn *grpc.ClientConn) error {
	for {
		state := conn.GetState()
		switch state {
		case connectivity.Ready:
			return nil
		case connectivity.Shutdown:
			return errors.New("grpc connection entered shutdown state")
		}

		if !conn.WaitForStateChange(ctx, state) {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("grpc readiness wait timed out in state %s", state.String())
		}
	}
}
