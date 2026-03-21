package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
)

var (
	defaultLogger *slog.Logger
)

func Init(ctx context.Context, verbose bool) {
	var handler slog.Handler
	var writer io.Writer = os.Stdout

	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}

	if verbose {
		opts.Level = slog.LevelDebug
	}

	if os.Getenv("LOG_FORMAT") == "json" {
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
