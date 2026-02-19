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

func TestRecordResponseTracksInterimThenFinal(t *testing.T) {
	s := &Stream{}

	s.recordResponse(&asrpb.StreamingRecognizeResponse{
		Results: []*asrpb.StreamingRecognitionResult{{
			IsFinal:      false,
			Alternatives: []*asrpb.SpeechRecognitionAlternative{{Transcript: "hello wor"}},
		}},
	})

	require.Equal(t, "hello wor", s.lastInterim)
	require.Empty(t, s.segments)

	s.recordResponse(&asrpb.StreamingRecognizeResponse{
		Results: []*asrpb.StreamingRecognitionResult{{
			IsFinal:      true,
			Alternatives: []*asrpb.SpeechRecognitionAlternative{{Transcript: "hello world"}},
		}},
	})

	require.Empty(t, s.lastInterim)
	require.Equal(t, []string{"hello world"}, s.segments)
}

func TestRecordResponseCommitsInterimAcrossPauseLikeReset(t *testing.T) {
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

	segments := collectSegments(s.segments, s.lastInterim)
	require.Equal(t, []string{"first phrase", "second phrase"}, segments)
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

func TestCleanSegmentAndInterimContinuation(t *testing.T) {
	require.Equal(t, "hello world", cleanSegment("  hello\n world  "))
	require.Empty(t, cleanSegment("   \n\t"))

	require.True(t, isInterimContinuation("hello", "hello world"))
	require.True(t, isInterimContinuation("hello world", "hello"))
	require.False(t, isInterimContinuation("first phrase", "second phrase"))
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
