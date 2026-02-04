# syntax=docker/dockerfile:1

# Build stage: compile the Go binary
FROM golang:1.23-bookworm AS builder

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary with optimizations
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o /discorgeous ./cmd/discorgeous

# Runtime stage: minimal image with runtime dependencies
FROM debian:bookworm-slim

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
        amd64) PIPER_ARCH="amd64" ;; \
        arm64) PIPER_ARCH="arm64" ;; \
        *) echo "Unsupported architecture: ${TARGETARCH}" && exit 1 ;; \
    esac && \
    wget -q "https://github.com/rhasspy/piper/releases/download/${PIPER_VERSION}/piper_linux_${PIPER_ARCH}.tar.gz" -O /tmp/piper.tar.gz && \
    tar -xzf /tmp/piper.tar.gz -C /app && \
    rm /tmp/piper.tar.gz && \
    chmod +x /app/piper/piper

# Create models directory (users will mount their models here)
RUN mkdir -p /app/models && chown discorgeous:discorgeous /app/models

# Copy binary from builder
COPY --from=builder /discorgeous /app/discorgeous

# Set ownership
RUN chown discorgeous:discorgeous /app/discorgeous

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
