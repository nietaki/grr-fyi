SHELL := /bin/bash

export TIMESTAMP=$(shell date +"%s")
export pwd=$(shell pwd)
# architectures list
export BUILD_DIR=$(pwd)/build
export ARCHITECTURES=(amd64 arm64 386)
export OPERATING_SYSTEMS=(linux darwin)

.PHONY: all
all: check test

.PHONY: install
install:
	# if the mise command is found, run it
	@which mise >/dev/null 2>&1 && mise install || true
	@echo "Installing dependencies..."
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/mgechev/revive@latest
	go install honnef.co/go/tools/cmd/staticcheck@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest

.PHONY: check
check:
	@echo "Running goimports..."
	goimports -l -w .

	@echo "Running go vet..."
	go vet ./...

	@echo "Running staticcheck..."
	staticcheck ./...
	revive -config revive_config.toml ./...

	# echo "Running govulncheck..."
	# govulncheck ./...

.PHONY: test
test:
	@echo "Running tests..."
	go test -v ./...

.PHONY: build
build:
	go build

.PHONY: run
run: build
	go run main.go

.PHONY: clean
clean:
	rm ./epstein-file-review || true
	rm -f $(BUILD_DIR)/epstein-file-review* || true

.PHONY: build-all
build-all:
	GOOS=linux GOARCH=amd64 go build -o "$(BUILD_DIR)/epstein-file-review_linux_amd64"
	GOOS=linux GOARCH=arm64 go build -o "$(BUILD_DIR)/epstein-file-review_linux_arm64"
	GOOS=darwin GOARCH=amd64 go build -o "$(BUILD_DIR)/epstein-file-review_darwin_amd64"
	GOOS=darwin GOARCH=arm64 go build -o "$(BUILD_DIR)/epstein-file-review_darwin_arm64"

.PHONY: build-docker
build-docker:
	docker buildx build --platform linux/arm64,linux/amd64 --tag registry.hoplon.net/nietaki/epstein-file-review:latest .

.PHONY: push-docker
push-docker:
	docker buildx build --platform linux/arm64,linux/amd64 --tag registry.hoplon.net/nietaki/epstein-file-review:latest --push .
