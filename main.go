package main

import (
	"github.com/nietaki/epstein-file-review/internal/env"
	"github.com/nietaki/epstein-file-review/internal/server"
)

func main() {
	// foo := 1
	cfg := env.Load()
	server.Start(cfg)
}
