package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/nietaki/grr-fyi/internal/env"
	"github.com/nietaki/grr-fyi/internal/logging"
	"github.com/nietaki/grr-fyi/internal/server"
)

func main() {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	logging.Init(context.Background(), os.Getenv("LOG_LEVEL") == "debug")

	slog.Info("starting application", "cwd", dir)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	cfg := env.Load()
	server.Start(ctx, cfg)

	<-ctx.Done()
	slog.Info("shutting down server...")
}
