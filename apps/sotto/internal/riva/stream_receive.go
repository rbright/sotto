package riva

import (
	"encoding/json"
	"errors"
	"io"

	asrpb "github.com/rbright/sotto/proto/gen/go/riva/proto"
)

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

		// Keep only the latest interim hypothesis. Riva can reset interim text
		// boundaries between updates; pre-committing the prior interim here can
		// introduce duplicated or stale leading segments in the final transcript.
		s.lastInterim = transcript
	}
}
