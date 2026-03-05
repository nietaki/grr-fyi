package main

import (
	"bufio"
	"os"
	"strings"

	"github.com/nietaki/epstein-file-review/internal/env"
	"github.com/nietaki/epstein-file-review/internal/filedb"
	"github.com/nietaki/epstein-file-review/internal/server"
)

func indexFiles() {
	file, err := os.Open("all_files.txt")
	if err != nil {
		panic(err)
	}
	scanner := bufio.NewScanner(file)
	fileCount := 0
	for scanner.Scan() {
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

	go indexFiles()

	// foo := 1
	cfg := env.Load()
	server.Start(cfg)
}
