BINARY  := timeweft
PKG     := ./cmd/timeweft
PREFIX  ?= $(HOME)/.local
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

GOLANGCI_LINT ?= golangci-lint
GORELEASER    ?= goreleaser

.DEFAULT_GOAL := help

.PHONY: help build install uninstall test race cover fmt vet lint tidy check snapshot clean

help: ## Show this help
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z_-]+:.*## / {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build bin/timeweft (static, no cgo)
	CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' -o bin/$(BINARY) $(PKG)

install: build ## Install to $(PREFIX)/bin (default ~/.local/bin)
	install -d $(PREFIX)/bin
	install -m 0755 bin/$(BINARY) $(PREFIX)/bin/$(BINARY)

uninstall: ## Remove $(PREFIX)/bin/timeweft
	rm -f $(PREFIX)/bin/$(BINARY)

test: ## Run tests
	go test ./...

race: ## Tests with race detector (needs cgo; unsupported on 47-bit VMA kernels, e.g. Raspberry Pi)
	CGO_ENABLED=1 go test -race ./...

cover: ## Test coverage summary (coverage.out)
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -n 1

fmt: ## Format code
	gofmt -w .

vet: ## go vet + gofmt check
	go vet ./...
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

lint: vet ## golangci-lint (skipped with a hint if not installed)
	@if command -v $(GOLANGCI_LINT) >/dev/null 2>&1; then $(GOLANGCI_LINT) run; \
	else echo "golangci-lint not installed, skipping (CI runs it)"; fi

tidy: ## Check go.mod/go.sum are tidy
	go mod tidy -diff

check: tidy lint test ## Local pre-push check (CI additionally runs `make race`)

snapshot: ## Local release build into dist/ via goreleaser
	$(GORELEASER) release --snapshot --clean

clean: ## Remove build output
	rm -rf bin dist coverage.out
