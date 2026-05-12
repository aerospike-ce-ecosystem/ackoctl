BINARY  := ackoctl
BIN_DIR := bin
PKG     := ./cmd/ackoctl

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.date=$(DATE)

GO_BUILD := go build -trimpath -ldflags "$(LDFLAGS)"

.PHONY: all
all: build

.PHONY: build
build:
	@mkdir -p $(BIN_DIR)
	$(GO_BUILD) -o $(BIN_DIR)/$(BINARY) $(PKG)

.PHONY: install
install:
	go install -trimpath -ldflags "$(LDFLAGS)" $(PKG)

.PHONY: test
test:
	go test -race ./...

.PHONY: test-coverage
test-coverage:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: fmt
fmt:
	gofmt -s -w .

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: snapshot
snapshot:
	goreleaser release --snapshot --clean

.PHONY: clean
clean:
	rm -rf $(BIN_DIR) dist coverage.out

.PHONY: help
help:
	@awk 'BEGIN {FS = ":.*##"; printf "Usage: make <target>\n\nTargets:\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  %-15s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
