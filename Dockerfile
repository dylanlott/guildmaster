# Multi-stage Dockerfile for Guildmaster server
# Builds the Go binary from ./cmd/server and produces a small runtime image

############################
# Builder image
############################
FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git ca-certificates tzdata
WORKDIR /src

# Cache modules
COPY go.mod go.sum ./
RUN go mod download

# Copy full source
COPY . .

# Build the server binary. Disable CGO for a static binary.
WORKDIR /src/cmd/server
ENV CGO_ENABLED=0
RUN go build -o /out/guildmaster .

############################
# Final image
############################
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app

# Copy binary and static assets
COPY --from=builder /out/guildmaster /app/guildmaster
COPY assets /app/assets

EXPOSE 8080

# Minimal, non-root user
RUN addgroup -S app && adduser -S app -G app
USER app

ENTRYPOINT ["/app/guildmaster"]
