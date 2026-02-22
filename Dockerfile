# Build stage — requires CGO for go-sqlite3 (FTS5) and sqlite-vec
FROM golang:1.26.0-bookworm AS builder

ARG VERSION=dev

WORKDIR /build

RUN apt-get update && apt-get install -y --no-install-recommends \
    libsqlite3-dev \
    && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" \
    go build \
    -ldflags "-X github.com/go-ports/echovault/internal/buildinfo.Version=${VERSION}" \
    -o /bin/memory ./cmd/memory

# Runtime stage — same distro as builder to ensure glibc compatibility
FROM debian:bookworm-slim

# Required by MCP Registry: associate this image with the published server name
LABEL io.modelcontextprotocol.server.name="io.github.go-ports/echovault"

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Create a non-root user and application data directory
RUN useradd -m -u 1000 -s /usr/sbin/nologin memory \
    && mkdir -p /app/.memory \
    && chown -R memory:memory /app

WORKDIR /app

COPY --from=builder /bin/memory /usr/local/bin/memory

# Default memory home inside the container; mount a host path here for persistence.
ENV MEMORY_HOME=/app/.memory

VOLUME ["/app/.memory"]

# Run the application as a non-root user
USER memory
ENTRYPOINT ["memory", "mcp"]
