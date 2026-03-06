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
	"github.com/nietaki/epstein-file-review/internal/server"
)

func indexFiles(ctx context.Context, indexingDone chan any) {
	defer close(indexingDone)
	file, err := os.Open("all_files.txt")
	if err != nil {
		panic(err)
	}
	scanner := bufio.NewScanner(file)
	fileCount := 0
	for scanner.Scan() {
		if ctx.Err() != nil {
			println("Stopping file indexing")
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			line = strings.TrimPrefix(line, "/")
			filedb.AddDocument(line)
			fileCount++

			if fileCount%1000 == 0 {
				println("Indexed ", fileCount, " files")
			}
		}
	}

	// print file count
	println("File count: ", fileCount)
}

func main() {
	// print current working directory
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	println("Current working directory: ", dir)
	// read the `all_files.txt` file and split into non-empty lines
	filedb.Init()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	indexingDone := make(chan any)
	go indexFiles(ctx, indexingDone)

	cfg := env.Load()
	server.Start(ctx, cfg)

	<-ctx.Done()
	println("Shutting down server...")

	_, _ = <-indexingDone

	println("file indexing shut down peacefully")
}
