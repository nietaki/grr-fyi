package logging

import (
	"context"
	"io"
	"log/slog"
	"os"

	"github.com/nietaki/epstein-file-review/internal/env"
)

var (
	defaultLogger *slog.Logger
)

func getHandlerOptions(verbose bool) *slog.HandlerOptions {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}

	if verbose {
		opts.Level = slog.LevelDebug
	}

	return opts
}

func Init(ctx context.Context, verbose bool) {
	var handler slog.Handler
	var writer io.Writer = os.Stdout

	opts := getHandlerOptions(verbose)

	if env.Get().LogFormat == "json" {
		handler = slog.NewJSONHandler(writer, opts)
	} else {
		handler = slog.NewTextHandler(writer, opts)
	}

	defaultLogger = slog.New(handler)
	slog.SetDefault(defaultLogger)
}

func SetLogger(logger *slog.Logger) {
	defaultLogger = logger
	slog.SetDefault(defaultLogger)
}

func GetLogger() *slog.Logger {
	return defaultLogger
}
