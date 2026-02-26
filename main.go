package main

import (
	"bufio"
	"os"
	"strings"

	"github.com/nietaki/epstein-file-review/internal/env"
	"github.com/nietaki/epstein-file-review/internal/filedb"
	"github.com/nietaki/epstein-file-review/internal/server"
)

func main() {
	// print current working directory
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	println("Current working directory: ", dir)
	// read the `all_files.txt` file and split into non-empty lines

	file, err := os.Open("all_files.txt")
	if err != nil {
		panic(err)
	}
	scanner := bufio.NewScanner(file)
	var files []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			files = append(files, strings.TrimPrefix(line, "/"))
		}
	}

	filedb.StoreFilenames(files)

	// print file count
	println("File count: ", len(files))

	// foo := 1
	cfg := env.Load()
	server.Start(cfg)
}
