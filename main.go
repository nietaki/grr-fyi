package main

import (
	"bufio"
	"context"
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
			logging.Info("stopping file indexing")
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
				logging.Info("file indexing progress", "count", fileCount)
			}
		}
	}

	logging.Info("file indexing complete", "total_files", fileCount)

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

	logging.Info("starting application", "cwd", dir)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	err = filedb.Init(ctx)
	if err != nil {
		logging.Error("failed to initialize filedb", "error", err)
		stop()
	}

	indexingResultChan := indexFiles(ctx)
	go func() {
		err := <-indexingResultChan
		close(indexingResultChan)
		if err != nil {
			logging.Error("error indexing files", "error", err)
			stop()
		}
	}()

	cfg := env.Load()
	server.Start(ctx, cfg)

	<-ctx.Done()
	logging.Info("shutting down server...")

	<-indexingResultChan

	logging.Info("file indexing shut down peacefully")
}
