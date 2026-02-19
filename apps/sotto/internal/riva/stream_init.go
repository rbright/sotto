package riva

import (
	"context"
	"fmt"
	"time"

	asrpb "github.com/rbright/sotto/proto/gen/go/riva/proto"
)

type openResult struct {
	stream asrpb.RivaSpeechRecognition_StreamingRecognizeClient
	err    error
}

// openRecognizeWithTimeout bounds stream-open latency when backend RPCs stall.
func openRecognizeWithTimeout(
	ctx context.Context,
	timeout time.Duration,
	open func() (asrpb.RivaSpeechRecognition_StreamingRecognizeClient, error),
) (asrpb.RivaSpeechRecognition_StreamingRecognizeClient, error) {
	if timeout <= 0 {
		return open()
	}

	resultCh := make(chan openResult, 1)
	go func() {
		stream, err := open()
		resultCh <- openResult{stream: stream, err: err}
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return nil, fmt.Errorf("timed out after %s", timeout)
	case result := <-resultCh:
		return result.stream, result.err
	}
}

// runWithTimeout bounds one blocking stream operation (for example initial Send).
func runWithTimeout(ctx context.Context, timeout time.Duration, call func() error) error {
	if timeout <= 0 {
		return call()
	}

	resultCh := make(chan error, 1)
	go func() {
		resultCh <- call()
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return fmt.Errorf("timed out after %s", timeout)
	case err := <-resultCh:
		return err
	}
}
