# Build stage
FROM golang:1.26-bookworm AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o /api ./cmd/api

# Runtime stage
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /api .

EXPOSE 8080

ENV GIN_MODE=release
ENV PORT=8080

CMD ["/app/api"]
