// Package main provides the sotto CLI process entrypoint.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rbright/sotto/internal/app"
)

// main wires process signal handling to the application runner.
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	exitCode := app.Execute(ctx, os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}
