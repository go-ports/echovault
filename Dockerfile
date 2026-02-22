# Build stage — requires CGO for go-sqlite3 (FTS5) and sqlite-vec
FROM golang:1.26-bookworm AS builder

ARG VERSION=dev

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" \
    go build \
    -ldflags "-X github.com/go-ports/echovault/internal/buildinfo.Version=${VERSION}" \
    -o /bin/memory ./cmd/memory

# Runtime stage — same distro as builder to ensure glibc compatibility
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /bin/memory /usr/local/bin/memory

# Default memory home inside the container; mount a host path here for persistence.
ENV MEMORY_HOME=/root/.memory

VOLUME ["/root/.memory"]

ENTRYPOINT ["memory", "mcp"]
