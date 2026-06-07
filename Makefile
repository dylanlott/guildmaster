# Makefile for the Guildmaster Go project

# Variables
BINARY_NAME=guildmaster
SRC=$(wildcard *.go)
GO_IMAGE=golang:1.26-alpine
GO_BIN=/usr/local/go/bin/go
GO_MIN_PREFIX=go1.26

# Default target
all: build

# Build the binary
build:
	@go_version=$$(go env GOVERSION 2>/dev/null || go version 2>/dev/null | awk '{print $$3}'); \
	case "$$go_version" in \
		$(GO_MIN_PREFIX)* ) \
			go build -o $(BINARY_NAME) $(SRC) ;; \
		* ) \
			echo "Local Go $$go_version is too old for this repo (needs Go 1.26+). Building in Docker..."; \
			docker run --rm -v "$$PWD":/src -w /src --entrypoint /bin/sh $(GO_IMAGE) -lc 'apk add --no-cache git >/dev/null && $(GO_BIN) build -o $(BINARY_NAME) $(SRC)' ; \
			;; \
	esac

# Run the application
run:
	@go_version=$$(go env GOVERSION 2>/dev/null || go version 2>/dev/null | awk '{print $$3}'); \
	case "$$go_version" in \
		$(GO_MIN_PREFIX)* ) \
			go run . ;; \
		* ) \
			echo "Local Go $$go_version is too old for this repo (needs Go 1.26+). Running in Docker..."; \
			docker run --rm -it -v "$$PWD":/src -w /src --entrypoint /bin/sh $(GO_IMAGE) -lc 'apk add --no-cache git >/dev/null && $(GO_BIN) run .' ; \
			;; \
	esac

tui:
	@go_version=$$(go env GOVERSION 2>/dev/null || go version 2>/dev/null | awk '{print $$3}'); \
	case "$$go_version" in \
		$(GO_MIN_PREFIX)* ) \
			go run . -tui ;; \
		* ) \
			echo "Local Go $$go_version is too old for this repo (needs Go 1.26+). TUI run requires a newer local Go toolchain."; \
			exit 1 ;; \
	esac

# Run the http server
server:
	@go_version=$$(go env GOVERSION 2>/dev/null || go version 2>/dev/null | awk '{print $$3}'); \
	case "$$go_version" in \
		$(GO_MIN_PREFIX)* ) \
			go run ./cmd/server ;; \
		* ) \
			echo "Local Go $$go_version is too old for this repo (needs Go 1.26+). Running server in Docker..."; \
			docker run --rm -it -p 8080:8080 -v "$$PWD":/src -w /src --entrypoint /bin/sh $(GO_IMAGE) -lc 'apk add --no-cache git >/dev/null && $(GO_BIN) run ./cmd/server' ; \
			;; \
	esac

# Import games from Google Sheets into SQLite
import:
	@go_version=$$(go env GOVERSION 2>/dev/null || go version 2>/dev/null | awk '{print $$3}'); \
	case "$$go_version" in \
		$(GO_MIN_PREFIX)* ) \
			go run ./cmd/import ;; \
		* ) \
			echo "Local Go $$go_version is too old for this repo (needs Go 1.26+). Running import in Docker..."; \
			docker run --rm -it -v "$$PWD":/src -w /src --entrypoint /bin/sh $(GO_IMAGE) -lc 'apk add --no-cache git >/dev/null && $(GO_BIN) run ./cmd/import' ; \
			;; \
	esac

# Format the code
fmt:
	@go_version=$$(go env GOVERSION 2>/dev/null || go version 2>/dev/null | awk '{print $$3}'); \
	case "$$go_version" in \
		$(GO_MIN_PREFIX)* ) \
			go fmt ./... ;; \
		* ) \
			echo "Local Go $$go_version is too old for this repo (needs Go 1.26+). Formatting in Docker..."; \
			docker run --rm -v "$$PWD":/src -w /src --entrypoint /bin/sh $(GO_IMAGE) -lc 'apk add --no-cache git >/dev/null && $(GO_BIN) fmt ./...' ; \
			;; \
	esac

# Run tests
test:
	@go_version=$$(go env GOVERSION 2>/dev/null || go version 2>/dev/null | awk '{print $$3}'); \
	case "$$go_version" in \
		$(GO_MIN_PREFIX)* ) \
			go test ./... ;; \
		* ) \
			echo "Local Go $$go_version is too old for this repo (needs Go 1.26+). Running tests in Docker..."; \
			docker run --rm -v "$$PWD":/src -w /src --entrypoint /bin/sh $(GO_IMAGE) -lc 'apk add --no-cache git >/dev/null && $(GO_BIN) test ./...' ; \
			;; \
	esac

# Clean up build artifacts
clean:
	rm -f $(BINARY_NAME)

.PHONY: all build run fmt test clean server import tui
