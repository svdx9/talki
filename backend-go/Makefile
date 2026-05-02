# Backend Makefile for Conjugate.cc

# Tooling configuration
TOOLS_DIR := tools
TOOLS_BIN := $(abspath $(TOOLS_DIR)/bin)

# Versions
AIR_VERSION := v1.63.0
GOFUMPT_VERSION := latest
GOLANGCI_LINT_VERSION := v2.8.0
MIGRATE_VERSION := v4.19.1
OAPI_CODEGEN_VERSION := v2.5.1

# Binaries
GOLANGCI_LINT := $(TOOLS_BIN)/golangci-lint
GOFUMPT := $(TOOLS_BIN)/gofumpt
AIR := $(TOOLS_BIN)/air
MIGRATE := $(TOOLS_BIN)/migrate
OAPI_CODEGEN := $(TOOLS_BIN)/oapi-codegen

# Build variables
GIT_SHA := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X 'main.GitSHA=$(GIT_SHA)' -X 'main.BuildTime=$(BUILD_TIME)'

.PHONY: help
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  tools-install   Install required build tools to $(TOOLS_BIN)"
	@echo "  generate        Generate Go code from OpenAPI specification"
	@echo "  test            Run tests"
	@echo "  lint            Run linter"
	@echo "  build           Build server binary"
	@echo "  debug-build     Build server binary with debug symbols"
	@echo "  format          Format source code"
	@echo "  clean           Clean build artifacts and tools"

.PHONY: tools-dir
tools-dir:
	mkdir -p $(TOOLS_BIN)

.PHONY: tools-install
tools-install: tools-dir $(GOLANGCI_LINT) $(GOFUMPT) $(AIR) $(MIGRATE) $(OAPI_CODEGEN)

$(GOLANGCI_LINT):
	GOBIN=$(TOOLS_BIN) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

$(GOFUMPT):
	GOBIN=$(TOOLS_BIN) go install mvdan.cc/gofumpt@$(GOFUMPT_VERSION)

$(AIR):
	GOBIN=$(TOOLS_BIN) go install github.com/air-verse/air@$(AIR_VERSION)

$(MIGRATE):
	GOBIN=$(TOOLS_BIN) go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@$(MIGRATE_VERSION)

$(OAPI_CODEGEN):
	GOBIN=$(TOOLS_BIN) go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION)

.PHONY: generate
generate: tools-install
	$(OAPI_CODEGEN) -config ../docs/schema/v1/config.yaml ../docs/schema/v1/api.yaml

.PHONY: test
test:
	go test ./...

.PHONY: lint
lint: tools-install
	$(GOLANGCI_LINT) run ./...

.PHONY: build
build:
	mkdir -p bin/
	go build -ldflags "$(LDFLAGS)" -o bin/server ./cmd/server/main.go

.PHONY: debug-build
debug-build:
	mkdir -p bin/
	go build -gcflags="all=-N -l" -ldflags "$(LDFLAGS)" -o bin/server-debug ./cmd/server/main.go

.PHONY: format
format: tools-install
	$(GOFUMPT) -l -w .

.PHONY: clean
clean:
	rm -rf bin/ $(TOOLS_BIN)

.PHONY: dev
dev: $(AIR)
	$(AIR) -c .air.toml
