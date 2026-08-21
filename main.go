package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nietaki/grr-fyi/internal/db"
	"github.com/nietaki/grr-fyi/internal/env"
	"github.com/nietaki/grr-fyi/internal/logging"
	"github.com/nietaki/grr-fyi/internal/replication"
	"github.com/nietaki/grr-fyi/internal/server"
)

func main() {
	if err := run(); err != nil {
		slog.Error("error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	logging.Init(context.Background(), os.Getenv("LOG_LEVEL") == "debug")

	slog.Info("starting application", "cwd", dir)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	cfg := env.Load()

	// Start litestream replication (restores the database from the replica if
	// the local file is missing) before opening the application connection.
	store, err := replication.Start(ctx, cfg)
	if err != nil {
		return fmt.Errorf("start replication: %w", err)
	}
	// Register before the connection close so it runs after it (LIFO). Use a
	// fresh context so the final sync is not cut short by the cancelled ctx.
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := store.Close(shutdownCtx); err != nil {
			slog.Error("failed to close litestream store", "error", err)
		}
	}()

	conn, err := db.Open(ctx, cfg)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer conn.Close()

	if err := db.Migrate(ctx, conn); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	server.Start(ctx, cfg)

	<-ctx.Done()
	slog.Info("shutting down server...")
	return nil
}
