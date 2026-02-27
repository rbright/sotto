package riva

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	asrpb "github.com/rbright/sotto/proto/gen/go/riva/proto"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCollectSegmentsAppendsTrailingInterim(t *testing.T) {
	got := collectSegments([]string{"hello there"}, "how are you")
	require.Equal(t, []string{"hello there", "how are you"}, got)
}

func TestCollectSegmentsFallsBackToInterim(t *testing.T) {
	got := collectSegments(nil, "  tentative words  ")
	require.Equal(t, []string{"tentative words"}, got)
}

func TestCollectSegmentsMergesTrailingInterimWithCommittedSegments(t *testing.T) {
	got := collectSegments([]string{"hello world"}, "hello world and beyond")
	require.Equal(t, []string{"hello world and beyond"}, got)

	got = collectSegments([]string{"hello world"}, "hello")
	require.Equal(t, []string{"hello world"}, got)
}

func TestRecordResponseTracksInterimThenFinal(t *testing.T) {
	s := &Stream{}

	s.recordResponse(&asrpb.StreamingRecognizeResponse{
		Results: []*asrpb.StreamingRecognitionResult{{
			IsFinal:      false,
			Alternatives: []*asrpb.SpeechRecognitionAlternative{{Transcript: "hello wor"}},
		}},
	})

	require.Equal(t, "hello wor", s.lastInterim)
	require.Equal(t, 1, s.lastInterimAge)
	require.Empty(t, s.segments)

	s.recordResponse(&asrpb.StreamingRecognizeResponse{
		Results: []*asrpb.StreamingRecognitionResult{{
			IsFinal:      true,
			Alternatives: []*asrpb.SpeechRecognitionAlternative{{Transcript: "hello world"}},
		}},
	})

	require.Empty(t, s.lastInterim)
	require.Equal(t, 0, s.lastInterimAge)
	require.Equal(t, []string{"hello world"}, s.segments)
}

func TestRecordResponseReplacesDivergentInterimWithoutPrecommit(t *testing.T) {
	s := &Stream{}

	s.recordResponse(&asrpb.StreamingRecognizeResponse{
		Results: []*asrpb.StreamingRecognitionResult{{
			IsFinal:      false,
			Alternatives: []*asrpb.SpeechRecognitionAlternative{{Transcript: "first phrase"}},
		}},
	})

	s.recordResponse(&asrpb.StreamingRecognizeResponse{
		Results: []*asrpb.StreamingRecognitionResult{{
			IsFinal:      false,
			Alternatives: []*asrpb.SpeechRecognitionAlternative{{Transcript: "second phrase"}},
		}},
	})

	require.Empty(t, s.segments)
	segments := collectSegments(s.segments, s.lastInterim)
	require.Equal(t, []string{"second phrase"}, segments)
}

func TestRecordResponseCommitsStableSingleInterimOnDivergence(t *testing.T) {
	s := &Stream{}

	s.recordResponse(&asrpb.StreamingRecognizeResponse{
		Results: []*asrpb.StreamingRecognitionResult{{
			IsFinal:      false,
			Stability:    0.95,
			Alternatives: []*asrpb.SpeechRecognitionAlternative{{Transcript: "first phrase"}},
		}},
	})

	s.recordResponse(&asrpb.StreamingRecognizeResponse{
		Results: []*asrpb.StreamingRecognitionResult{{
			IsFinal:      false,
			Stability:    0.20,
			Alternatives: []*asrpb.SpeechRecognitionAlternative{{Transcript: "second phrase"}},
		}},
	})

	require.Equal(t, []string{"first phrase"}, s.segments)
	segments := collectSegments(s.segments, s.lastInterim)
	require.Equal(t, []string{"first phrase", "second phrase"}, segments)
}

func TestRecordResponseCommitsOneShotInterimOnAudioAdvance(t *testing.T) {
	s := &Stream{}

	s.recordResponse(&asrpb.StreamingRecognizeResponse{
		Results: []*asrpb.StreamingRecognitionResult{{
			IsFinal:        false,
			AudioProcessed: 1.0,
			Alternatives:   []*asrpb.SpeechRecognitionAlternative{{Transcript: "first phrase has enough words"}},
		}},
	})

	s.recordResponse(&asrpb.StreamingRecognizeResponse{
		Results: []*asrpb.StreamingRecognitionResult{{
			IsFinal:        false,
			AudioProcessed: 2.0,
			Alternatives:   []*asrpb.SpeechRecognitionAlternative{{Transcript: "second phrase continues now"}},
		}},
	})

	require.Equal(t, []string{"first phrase has enough words"}, s.segments)
	segments := collectSegments(s.segments, s.lastInterim)
	require.Equal(t, []string{"first phrase has enough words", "second phrase continues now"}, segments)
}

func TestRecordResponseKeepsOneShotInterimWhenAudioAdvanceIsSmall(t *testing.T) {
	s := &Stream{}

	s.recordResponse(&asrpb.StreamingRecognizeResponse{
		Results: []*asrpb.StreamingRecognitionResult{{
			IsFinal:        false,
			AudioProcessed: 1.0,
			Alternatives:   []*asrpb.SpeechRecognitionAlternative{{Transcript: "first phrase has enough words"}},
		}},
	})

	s.recordResponse(&asrpb.StreamingRecognizeResponse{
		Results: []*asrpb.StreamingRecognitionResult{{
			IsFinal:        false,
			AudioProcessed: 1.2,
			Alternatives:   []*asrpb.SpeechRecognitionAlternative{{Transcript: "second phrase continues now"}},
		}},
	})

	require.Empty(t, s.segments)
	segments := collectSegments(s.segments, s.lastInterim)
	require.Equal(t, []string{"second phrase continues now"}, segments)
}

func TestRecordResponseCommitsInterimChainOnDivergence(t *testing.T) {
	s := &Stream{}

	s.recordResponse(&asrpb.StreamingRecognizeResponse{
		Results: []*asrpb.StreamingRecognitionResult{{
			IsFinal:      false,
			Alternatives: []*asrpb.SpeechRecognitionAlternative{{Transcript: "first phrase"}},
		}},
	})

	s.recordResponse(&asrpb.StreamingRecognizeResponse{
		Results: []*asrpb.StreamingRecognitionResult{{
			IsFinal:      false,
			Alternatives: []*asrpb.SpeechRecognitionAlternative{{Transcript: "first phrase extended"}},
		}},
	})

	s.recordResponse(&asrpb.StreamingRecognizeResponse{
		Results: []*asrpb.StreamingRecognitionResult{{
			IsFinal:      false,
			Alternatives: []*asrpb.SpeechRecognitionAlternative{{Transcript: "second phrase"}},
		}},
	})

	require.Equal(t, []string{"first phrase extended"}, s.segments)
	require.Equal(t, "second phrase", s.lastInterim)
	require.Equal(t, 1, s.lastInterimAge)
	segments := collectSegments(s.segments, s.lastInterim)
	require.Equal(t, []string{"first phrase extended", "second phrase"}, segments)
}

func TestRecordResponseBuildsMultipleSegmentsAcrossLongInterimStream(t *testing.T) {
	s := &Stream{}

	responses := []*asrpb.StreamingRecognizeResponse{
		{Results: []*asrpb.StreamingRecognitionResult{{IsFinal: false, Alternatives: []*asrpb.SpeechRecognitionAlternative{{Transcript: "alpha"}}}}},
		{Results: []*asrpb.StreamingRecognitionResult{{IsFinal: false, Alternatives: []*asrpb.SpeechRecognitionAlternative{{Transcript: "alpha one"}}}}},
		{Results: []*asrpb.StreamingRecognitionResult{{IsFinal: false, Alternatives: []*asrpb.SpeechRecognitionAlternative{{Transcript: "beta"}}}}},
		{Results: []*asrpb.StreamingRecognitionResult{{IsFinal: false, Alternatives: []*asrpb.SpeechRecognitionAlternative{{Transcript: "beta two"}}}}},
		{Results: []*asrpb.StreamingRecognitionResult{{IsFinal: false, Alternatives: []*asrpb.SpeechRecognitionAlternative{{Transcript: "gamma"}}}}},
	}
	for _, resp := range responses {
		s.recordResponse(resp)
	}

	segments := collectSegments(s.segments, s.lastInterim)
	require.Equal(t, []string{"alpha one", "beta two", "gamma"}, segments)
}

func TestRecordResponseDoesNotPrependStaleInterimBeforeFinal(t *testing.T) {
	s := &Stream{}

	s.recordResponse(&asrpb.StreamingRecognizeResponse{
		Results: []*asrpb.StreamingRecognitionResult{{
			IsFinal:      false,
			Alternatives: []*asrpb.SpeechRecognitionAlternative{{Transcript: "stale words"}},
		}},
	})

	s.recordResponse(&asrpb.StreamingRecognizeResponse{
		Results: []*asrpb.StreamingRecognitionResult{{
			IsFinal:      false,
			Alternatives: []*asrpb.SpeechRecognitionAlternative{{Transcript: "hello world"}},
		}},
	})

	s.recordResponse(&asrpb.StreamingRecognizeResponse{
		Results: []*asrpb.StreamingRecognitionResult{{
			IsFinal:      true,
			Alternatives: []*asrpb.SpeechRecognitionAlternative{{Transcript: "hello world"}},
		}},
	})

	segments := collectSegments(s.segments, s.lastInterim)
	require.Equal(t, []string{"hello world"}, segments)
}

func TestRecordResponseTreatsSuffixCorrectionAsContinuation(t *testing.T) {
	s := &Stream{}

	s.recordResponse(&asrpb.StreamingRecognizeResponse{
		Results: []*asrpb.StreamingRecognitionResult{{
			IsFinal:      false,
			Alternatives: []*asrpb.SpeechRecognitionAlternative{{Transcript: "replace reply replied on the review thread with details"}},
		}},
	})

	s.recordResponse(&asrpb.StreamingRecognizeResponse{
		Results: []*asrpb.StreamingRecognitionResult{{
			IsFinal:      false,
			Alternatives: []*asrpb.SpeechRecognitionAlternative{{Transcript: "replied on the review thread with details"}},
		}},
	})

	require.Empty(t, s.segments)
	segments := collectSegments(s.segments, s.lastInterim)
	require.Equal(t, []string{"replied on the review thread with details"}, segments)
}

func TestAppendSegmentDedupAndPrefixMerge(t *testing.T) {
	segments := appendSegment(nil, "hello")
	require.Equal(t, []string{"hello"}, segments)

	segments = appendSegment(segments, "hello")
	require.Equal(t, []string{"hello"}, segments)

	segments = appendSegment(segments, "hello world")
	require.Equal(t, []string{"hello world"}, segments)

	segments = appendSegment(segments, "hello")
	require.Equal(t, []string{"hello world"}, segments)

	segments = appendSegment(segments, "new sentence")
	require.Equal(t, []string{"hello world", "new sentence"}, segments)
}

func TestInterimHelpers(t *testing.T) {
	require.Equal(t, "hello world", cleanSegment("  hello\n world  "))
	require.Empty(t, cleanSegment("   \n\t"))

	continuationCases := []struct {
		name     string
		previous string
		current  string
		want     bool
	}{
		{name: "prefix extension", previous: "hello", current: "hello world", want: true},
		{name: "suffix correction", previous: "replace reply replied on thread", current: "replied on thread", want: true},
		{name: "shared prefix majority", previous: "one two three", current: "one two four", want: true},
		{name: "shared suffix majority", previous: "noise at start hello world", current: "hello world", want: true},
		{name: "divergent phrases", previous: "first phrase", current: "second phrase", want: false},
	}
	for _, tc := range continuationCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, isInterimContinuation(tc.previous, tc.current))
		})
	}

	require.False(t, shouldCommitInterimBoundary("", 5, 0.9, 1.0, 2.0))
	require.False(t, shouldCommitInterimBoundary("first phrase", 1, 0.1, 1.0, 1.2))
	require.True(t, shouldCommitInterimBoundary("first phrase", 2, 0.1, 1.0, 1.1))
	require.True(t, shouldCommitInterimBoundary("first phrase", 1, 0.9, 1.0, 1.1))
	require.True(t, shouldCommitInterimBoundary("done.", 1, 0.0, 1.0, 1.1))
	require.True(t, shouldCommitInterimBoundary("first phrase has enough words", 1, 0.1, 1.0, 2.0))
	require.False(t, shouldCommitInterimBoundary("too short", 1, 0.1, 1.0, 2.0))
}

func TestDialStreamEndToEndWithDebugSinkAndSpeechContexts(t *testing.T) {
	server := &testRivaServer{
		responses: []*asrpb.StreamingRecognizeResponse{
			{Results: []*asrpb.StreamingRecognitionResult{{
				IsFinal:      false,
				Alternatives: []*asrpb.SpeechRecognitionAlternative{{Transcript: "hello wor"}},
			}}},
			{Results: []*asrpb.StreamingRecognitionResult{{
				IsFinal:      true,
				Alternatives: []*asrpb.SpeechRecognitionAlternative{{Transcript: "hello world"}},
			}}},
			{Results: []*asrpb.StreamingRecognitionResult{{
				IsFinal:      false,
				Alternatives: []*asrpb.SpeechRecognitionAlternative{{Transcript: "second phrase"}},
			}}},
		},
	}
	endpoint, shutdown := startTestRivaServer(t, server)
	defer shutdown()

	var debug bytes.Buffer

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := DialStream(ctx, StreamConfig{
		Endpoint:             endpoint,
		LanguageCode:         "en-US",
		Model:                "parakeet",
		AutomaticPunctuation: true,
		SpeechPhrases: []SpeechPhrase{
			{Phrase: "  Sotto  ", Boost: 12},
			{Phrase: "", Boost: 20},
		},
		DialTimeout:           2 * time.Second,
		DebugResponseSinkJSON: &debug,
	})
	require.NoError(t, err)

	require.NoError(t, stream.SendAudio([]byte{1, 2, 3, 4}))
	require.NoError(t, stream.SendAudio(nil)) // no-op path

	segments, latency, err := stream.CloseAndCollect(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"hello world", "second phrase"}, segments)
	require.GreaterOrEqual(t, latency, time.Duration(0))

	require.NotNil(t, server.receivedConfig)
	require.Equal(t, int32(16000), server.receivedConfig.Config.SampleRateHertz)
	require.Equal(t, int32(1), server.receivedConfig.Config.AudioChannelCount)
	require.Equal(t, "en-US", server.receivedConfig.Config.LanguageCode)
	require.Equal(t, "parakeet", server.receivedConfig.Config.Model)
	require.True(t, server.receivedConfig.Config.EnableAutomaticPunctuation)
	require.Len(t, server.receivedConfig.Config.SpeechContexts, 1)
	require.Equal(t, []string{"Sotto"}, server.receivedConfig.Config.SpeechContexts[0].Phrases)
	require.Equal(t, 1, server.audioChunks)

	require.Contains(t, debug.String(), "results")
}

func TestDialStreamEmptyEndpoint(t *testing.T) {
	_, err := DialStream(context.Background(), StreamConfig{Endpoint: "   "})
	require.Error(t, err)
	require.Contains(t, err.Error(), "endpoint is empty")
}

func TestDialStreamReadinessTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := DialStream(ctx, StreamConfig{
		Endpoint:    "127.0.0.1:1",
		DialTimeout: 100 * time.Millisecond,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "readiness")
}

func TestRunWithTimeoutTimesOut(t *testing.T) {
	err := runWithTimeout(context.Background(), 20*time.Millisecond, func() error {
		time.Sleep(120 * time.Millisecond)
		return nil
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "timed out")
}

func TestOpenRecognizeWithTimeoutTimesOut(t *testing.T) {
	_, err := openRecognizeWithTimeout(context.Background(), 20*time.Millisecond, func() (asrpb.RivaSpeechRecognition_StreamingRecognizeClient, error) {
		time.Sleep(120 * time.Millisecond)
		return nil, nil
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "timed out")
}

func TestRunWithTimeoutReturnsCallError(t *testing.T) {
	want := errors.New("boom")
	err := runWithTimeout(context.Background(), time.Second, func() error {
		return want
	})
	require.ErrorIs(t, err, want)
}

func TestCloseAndCollectReturnsServerStreamError(t *testing.T) {
	server := &testRivaServer{streamErr: status.Error(codes.Internal, "boom")}
	endpoint, shutdown := startTestRivaServer(t, server)
	defer shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	stream, err := DialStream(ctx, StreamConfig{Endpoint: endpoint, DialTimeout: time.Second})
	require.NoError(t, err)
	require.NoError(t, stream.SendAudio([]byte{1, 2}))

	_, _, err = stream.CloseAndCollect(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "boom")
}

func TestSendAudioAfterCloseReturnsError(t *testing.T) {
	server := &testRivaServer{}
	endpoint, shutdown := startTestRivaServer(t, server)
	defer shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	stream, err := DialStream(ctx, StreamConfig{Endpoint: endpoint, DialTimeout: time.Second})
	require.NoError(t, err)

	_, _, err = stream.CloseAndCollect(ctx)
	require.NoError(t, err)

	err = stream.SendAudio([]byte{9, 9, 9})
	require.Error(t, err)
	require.Contains(t, err.Error(), "closed")
}

type testRivaServer struct {
	asrpb.UnimplementedRivaSpeechRecognitionServer

	responses []*asrpb.StreamingRecognizeResponse
	streamErr error

	receivedConfig *asrpb.StreamingRecognitionConfig
	audioChunks    int
}

func (s *testRivaServer) StreamingRecognize(stream grpc.BidiStreamingServer[asrpb.StreamingRecognizeRequest, asrpb.StreamingRecognizeResponse]) error {
	for {
		req, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}

		if cfg := req.GetStreamingConfig(); cfg != nil {
			s.receivedConfig = cfg
			continue
		}
		if len(req.GetAudioContent()) > 0 {
			s.audioChunks++
		}
	}

	for _, resp := range s.responses {
		if err := stream.Send(resp); err != nil {
			return err
		}
	}
	if s.streamErr != nil {
		return s.streamErr
	}
	return nil
}

func startTestRivaServer(t *testing.T, srv asrpb.RivaSpeechRecognitionServer) (string, func()) {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	asrpb.RegisterRivaSpeechRecognitionServer(grpcServer, srv)

	go func() {
		_ = grpcServer.Serve(lis)
	}()

	shutdown := func() {
		grpcServer.Stop()
		_ = lis.Close()
	}

	return lis.Addr().String(), shutdown
}
