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
	go install github.com/mvrahden/go-test/cmd/gotest@latest

.PHONY: goimports
goimports:
	@echo "Running goimports..."
	goimports -l -w ./main.go ./internal/

.PHONY: coverage
coverage:
	@echo "Running tests with coverage..."
	gotest -coverprofile=coverage/coverage.out ./internal/...
	go tool cover -func=coverage/coverage.out

coverage-html:
	@echo "Running tests with coverage..."
	gotest -coverprofile=coverage/coverage.out ./internal/...
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
	gotest ./...

.PHONY: build
build:
	go build

.PHONY: run
run: build
	go run main.go

.PHONY: clean
clean:
	rm ./grr-fyi || true
	rm -f $(BUILD_DIR)/grr-fyi* || true

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

