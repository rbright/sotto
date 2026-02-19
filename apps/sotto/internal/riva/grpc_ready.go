package riva

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

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
