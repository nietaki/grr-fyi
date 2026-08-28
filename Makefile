SHELL := /bin/bash

export TIMESTAMP=$(shell date +"%s")
export pwd=$(shell pwd)
# architectures list
export BUILD_DIR=$(pwd)/build
export ARCHITECTURES=(amd64 arm64 386)
export OPERATING_SYSTEMS=(linux darwin)
export APP_VERSION=$(shell cat APP_VERSION.txt)
export CHART_VERSION=$(shell cat CHART_VERSION.txt)

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

	go install golang.org/x/tools/gopls@latest

.PHONY: goimports
goimports:
	@echo "Running goimports..."
	goimports -l -w ./main.go ./internal/

.PHONY: coverage
coverage:
	@echo "Running tests with coverage..."
	go test -coverprofile=coverage/coverage.out ./internal/...
	go tool cover -func=coverage/coverage.out

coverage-html:
	@echo "Running tests with coverage..."
	go test -coverprofile=coverage/coverage.out ./internal/...
	go tool cover -html=coverage/coverage.out

.PHONY: vet
vet:
	@echo "Running go vet..."
	go vet ./...

.PHONY: staticcheck
staticcheck:
	@echo "Running go staticcheck..."
	staticcheck ./...

.PHONY: govulncheck
govulncheck:
	@echo "Running go govulncheck..."
	govulncheck internal/...

.PHONY: check
check: goimports coverage vet staticcheck
	@echo "all checks passed!"

.PHONY: test
test:
	@echo "Running tests..."
	go test ./...

.PHONY: build
build:
	go build

.PHONY: run
run: build
	go run main.go

.PHONY: run-pprof
run-pprof: build
	PPROF_ENABLED=true SITE_URL=http://localhost:30666/ go run main.go

.PHONY: profile-cpu
profile-cpu:
	@echo "Collecting 30s CPU profile..."
	@curl -s -o /tmp/cpu.prof "http://localhost:30666/debug/pprof/profile?seconds=30"
	@echo "Opening flamegraph..."
	@go tool pprof -http=:30669 /tmp/cpu.prof

.PHONY: profile-heap
profile-heap:
	@echo "Collecting heap profile..."
	@curl -s -o /tmp/heap.prof "http://localhost:30666/debug/pprof/heap"
	@echo "Opening heap profile..."
	@go tool pprof -http=:30669 /tmp/heap.prof

.PHONY: profile-goroutine
profile-goroutine:
	@echo "Goroutine count:"
	@curl -s "http://localhost:30666/debug/pprof/goroutine?debug=1" | grep "^goroutine" | wc -l
	@echo ""
	@echo "First 20 goroutines:"
	@curl -s "http://localhost:30666/debug/pprof/goroutine?debug=1" | head -50

.PHONY: pprof-shell
pprof-shell:
	go tool pprof http://localhost:30666/debug/pprof/profile?seconds=30

.PHONY: clean
clean:
	rm ./grr-fyi || true
	rm -f $(BUILD_DIR)/grr-fyi* || true

.PHONY: remove-litestream-cache
remove-litestream-cache:
	@echo "Removing Litestream cache..."
	@find ./litestream-cache -mindepth 1 -not -name .gitignore -exec rm -rf {} + 2>/dev/null || true

.PHONY: remove-db
remove-db:
	@echo "Removing database files..."
	rm -f ./db/filedb.sqlite ./db/filedb.sqlite-shm ./db/filedb.sqlite-wal

.PHONY: build-all
build-all:
	GOOS=linux GOARCH=amd64 go build -o "$(BUILD_DIR)/grr-fyi_linux_amd64"
	GOOS=linux GOARCH=arm64 go build -o "$(BUILD_DIR)/grr-fyi_linux_arm64"
	GOOS=darwin GOARCH=amd64 go build -o "$(BUILD_DIR)/grr-fyi_darwin_amd64"
	GOOS=darwin GOARCH=arm64 go build -o "$(BUILD_DIR)/grr-fyi_darwin_arm64"

.PHONY: docker-build
docker-build:
	echo "DEPRECATED: use 'make docker-push' instead"
	exit 1
	docker buildx build --platform linux/arm64,linux/amd64 --tag registry.hoplon.net/nietaki/grr-fyi:latest .

export BUILDX_BUILDER ?= multiarch

.PHONY: docker-builder
docker-builder:
	docker buildx inspect $(BUILDX_BUILDER) >/dev/null 2>&1 || docker buildx create --name $(BUILDX_BUILDER) --driver docker-container --bootstrap
	docker buildx use $(BUILDX_BUILDER)

.PHONY: docker-push
docker-push: docker-builder
	docker buildx build --platform linux/arm64,linux/amd64 --tag registry.hoplon.net/nietaki/grr-fyi:latest --tag registry.hoplon.net/nietaki/grr-fyi:$(APP_VERSION) --push .

build/grr-fyi-chart-$(CHART_VERSION).tgz:
# build/grr-fyi-chart-$(CHART_VERSION).tgz:
	echo "packaging the chart, version $(CHART_VERSION)"
	helm package grr-fyi-chart --app-version $(APP_VERSION) --version $(CHART_VERSION) --destination $(BUILD_DIR)

.PHONY: helm-push
helm-push: build/grr-fyi-chart-$(CHART_VERSION).tgz
	helm push build/grr-fyi-chart-$(CHART_VERSION).tgz oci://registry.hoplon.net/helm-charts

.PHONY: bump-versions
bump-versions:
	@bash scripts/bump-versions.sh

.PHONY: push-all
push-all: docker-push helm-push

.PHONY: push-new
push-new: bump-versions
	$(MAKE) push-all

