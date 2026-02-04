# syntax=docker/dockerfile:1

# Build stage: compile the Go binary
FROM golang:1.25-bookworm AS builder

WORKDIR /app

# Build dependencies (CGO)
RUN apt-get update && apt-get install -y --no-install-recommends \
    pkg-config \
    libopus-dev \
    && rm -rf /var/lib/apt/lists/*

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the main binary with optimizations (requires CGO for opus)
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o /discorgeous ./cmd/discorgeous

# Build the ntfy-relay binary (pure Go, no CGO needed)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /ntfy-relay ./cmd/ntfy-relay

# Runtime stage: minimal image with runtime dependencies
FROM debian:bookworm-slim AS discorgeous

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    ffmpeg \
    libopus0 \
    wget \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user for security
RUN useradd -r -s /bin/false discorgeous

# Set up piper directory
WORKDIR /app

# Download and install piper
ARG PIPER_VERSION=2023.11.14-2
ARG TARGETARCH
RUN case "${TARGETARCH}" in \
        amd64) PIPER_ARCH="x86_64" ;; \
        arm64) PIPER_ARCH="aarch64" ;; \
        *) echo "Unsupported architecture: ${TARGETARCH}" && exit 1 ;; \
    esac && \
    wget -q "https://github.com/rhasspy/piper/releases/download/${PIPER_VERSION}/piper_linux_${PIPER_ARCH}.tar.gz" -O /tmp/piper.tar.gz && \
    tar -xzf /tmp/piper.tar.gz -C /app && \
    rm /tmp/piper.tar.gz && \
    chmod +x /app/piper/piper

# Create models directory (users will mount their models here)
RUN mkdir -p /app/models && chown discorgeous:discorgeous /app/models

# Copy binaries from builder
COPY --from=builder /discorgeous /app/discorgeous
COPY --from=builder /ntfy-relay /app/ntfy-relay

# Set ownership
RUN chown discorgeous:discorgeous /app/discorgeous /app/ntfy-relay

# Switch to non-root user
USER discorgeous

# Set piper path in environment
ENV PIPER_PATH=/app/piper/piper

# Expose HTTP port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/v1/healthz || exit 1

# Run the application
ENTRYPOINT ["/app/discorgeous"]

# =============================================================================
# Relay runtime stage: minimal image for ntfy-relay (no piper, ffmpeg, models)
# =============================================================================
FROM debian:bookworm-slim AS relay

# Install only essential runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user for security
RUN useradd -r -s /bin/false relay

WORKDIR /app

# Copy only the relay binary from builder
COPY --from=builder /ntfy-relay /app/ntfy-relay

# Set ownership
RUN chown relay:relay /app/ntfy-relay

# Switch to non-root user
USER relay

# No ports exposed (relay is outbound-only client)

# Run the relay
ENTRYPOINT ["/app/ntfy-relay"]
