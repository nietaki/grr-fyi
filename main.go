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
			println("Stopping file indexing")
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
				println("Indexed ", fileCount, " files")
			}
		}
	}

	// print file count
	println("File count: ", fileCount)

	// err = fmt.Errorf("test error from file indexing")
}

func indexFiles(ctx context.Context) chan error {
	indexingResult := make(chan error)
	go doIndexFiles(ctx, indexingResult)
	return indexingResult
}

func main() {
	// print current working directory
	dir, err := os.Getwd()
	if err != nil {
		panic(err) // this one is ok
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	println("Current working directory: ", dir)
	// read the `all_files.txt` file and split into non-empty lines
	err = filedb.Init(ctx)
	if err != nil {
		stop()
	}

	indexingResultChan := indexFiles(ctx)
	go func() {
		err := <-indexingResultChan
		close(indexingResultChan)
		if err != nil {
			println("Error indexing files: ", err.Error())
			stop()
		}
	}()

	cfg := env.Load()
	server.Start(ctx, cfg)

	<-ctx.Done()
	println("Shutting down server...")

	_, _ = <-indexingResultChan

	println("file indexing shut down peacefully")
}
