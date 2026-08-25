package main

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nietaki/grr-fyi/internal/db"
	"github.com/nietaki/grr-fyi/internal/env"
	"github.com/nietaki/grr-fyi/internal/logging"
	"github.com/nietaki/grr-fyi/internal/replication"
	"github.com/nietaki/grr-fyi/internal/server"
	"github.com/urfave/cli/v3"
)

//go:embed APP_VERSION.txt
var appVersion string

func main() {
	app := &cli.Command{
		Name:    "grr-fyi",
		Usage:   "grr.fyi URL shortener",
		Version: strings.TrimSpace(appVersion),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "log-level",
				Value:   "info",
				Usage:   "Log level (debug, info, warn, error)",
				Sources: cli.EnvVars("LOG_LEVEL"),
			},
		},
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			env.Load()
			logging.Init(ctx, cmd.String("log-level") == "debug")
			return ctx, nil
		},
		Action: runServe,
		Commands: []*cli.Command{
			{
				Name:   "serve",
				Usage:  "Start the HTTP server",
				Action: runServe,
			},
			{
				Name:   "migrate",
				Usage:  "Run database migrations",
				Action: runMigrate,
			},
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		slog.Error("error", "error", err)
		os.Exit(1)
	}
}

func runServe(ctx context.Context, cmd *cli.Command) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	slog.Info("starting application", "cwd", dir)

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	cfg := env.Get()

	store, err := replication.Start(ctx, cfg)
	if err != nil {
		return fmt.Errorf("start replication: %w", err)
	}
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

func runMigrate(ctx context.Context, cmd *cli.Command) error {
	cfg := env.Get()

	conn, err := db.Open(ctx, cfg)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer conn.Close()

	if err := db.Migrate(ctx, conn); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	fmt.Println("Migrations complete")
	return nil
}
