# Makefile for the Guildmaster Go project

# Variables
BINARY_NAME=guildmaster
SRC=$(wildcard *.go)

# Default target
all: build

# Build the binary
build:
	go build -o $(BINARY_NAME) $(SRC)

# Run the application
run:
	go run .

tui: 
	go run . -tui

# Run the http server
server:
	go run ./cmd/server

# Format the code
fmt:
	go fmt ./...

# Run tests
test:
	go test ./...

# Clean up build artifacts
clean:
	rm -f $(BINARY_NAME)

.PHONY: all build run fmt test clean
