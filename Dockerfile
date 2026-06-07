FROM golang:1.26-alpine AS builder
RUN apk add --no-cache git ca-certificates tzdata
WORKDIR /src

# Cache modules
COPY go.mod go.sum ./
RUN go mod download

# Copy full source
COPY . .

WORKDIR /src
RUN go mod tidy

WORKDIR /src/cmd/server
ENV CGO_ENABLED=0
RUN go build -o /out/guildmaster .

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app

COPY --from=builder /out/guildmaster /app/guildmaster
COPY assets /app/assets

ENV PORT=8080 \
    DATABASE_PATH=/data/guildmaster.db \
    SPREADSHEET_ID= \
    SCOREBOARD_API_KEY= \
    GUILDMASTER_ADMIN_KEY=

VOLUME ["/data"]
EXPOSE 8080
ENTRYPOINT ["/app/guildmaster"]
