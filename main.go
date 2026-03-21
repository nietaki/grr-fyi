package main

import (
	"bufio"
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/nietaki/epstein-file-review/internal/env"
	"github.com/nietaki/epstein-file-review/internal/filedb"
	"github.com/nietaki/epstein-file-review/internal/logging"
	"github.com/nietaki/epstein-file-review/internal/server"
)

func doIndexFiles(ctx context.Context, indexingResult chan error) {
	var err error
	file, err := os.Open("all_files.txt")
	defer func() { indexingResult <- err }()
	if err != nil {
		return
	}

	scanner := bufio.NewScanner(file)
	fileCount := 0
	for scanner.Scan() {
		if ctx.Err() != nil {
			slog.Info("stopping file indexing")
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			line = strings.TrimPrefix(line, "/")
			err = filedb.AddDocument(ctx, line)
			if err != nil {
				return
			}
			fileCount++

			if fileCount%1000 == 0 {
				slog.Info("file indexing progress", "count", fileCount)
			}
		}
	}

	slog.Info("file indexing complete", "total_files", fileCount)

	// err = fmt.Errorf("test error from file indexing")
}

func indexFiles(ctx context.Context) chan error {
	indexingResult := make(chan error)
	go doIndexFiles(ctx, indexingResult)
	return indexingResult
}

func main() {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	logging.Init(context.Background(), os.Getenv("LOG_LEVEL") == "debug")

	slog.Info("starting application", "cwd", dir)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	err = filedb.Init(ctx)
	if err != nil {
		slog.Error("failed to initialize filedb", "error", err)
		stop()
	}

	indexingResultChan := indexFiles(ctx)
	go func() {
		err := <-indexingResultChan
		close(indexingResultChan)
		if err != nil {
			slog.Error("error indexing files", "error", err)
			stop()
		}
	}()

	cfg := env.Load()
	server.Start(ctx, cfg)

	<-ctx.Done()
	slog.Info("shutting down server...")

	<-indexingResultChan

	slog.Info("file indexing shut down peacefully")
}
