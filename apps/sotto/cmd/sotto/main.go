package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rbright/sotto/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	exitCode := app.Execute(ctx, os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}
