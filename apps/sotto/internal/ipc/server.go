package ipc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
)

// Handler processes one IPC command request.
type Handler interface {
	Handle(context.Context, Request) Response
}

// HandlerFunc adapts a function to the Handler interface.
type HandlerFunc func(context.Context, Request) Response

func (f HandlerFunc) Handle(ctx context.Context, req Request) Response {
	return f(ctx, req)
}

// Serve accepts unix-socket clients until context cancellation or listener close.
func Serve(ctx context.Context, listener net.Listener, handler Handler) error {
	var wg sync.WaitGroup

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) || ctx.Err() != nil {
				wg.Wait()
				return nil
			}
			return fmt.Errorf("accept IPC connection: %w", err)
		}

		wg.Add(1)
		go func(c net.Conn) {
			defer wg.Done()
			defer c.Close()

			reader := bufio.NewReader(c)
			line, err := reader.ReadBytes('\n')
			if err != nil {
				_ = json.NewEncoder(c).Encode(Response{OK: false, Error: fmt.Sprintf("read request: %v", err)})
				return
			}

			var req Request
			if err := json.Unmarshal(line, &req); err != nil {
				_ = json.NewEncoder(c).Encode(Response{OK: false, Error: fmt.Sprintf("decode request: %v", err)})
				return
			}

			resp := handler.Handle(ctx, req)
			_ = json.NewEncoder(c).Encode(resp)
		}(conn)
	}
}
