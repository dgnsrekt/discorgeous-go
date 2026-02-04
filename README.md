# discorgeous-go

A self-hosted Discord voice TTS bot written in Go. Accepts speech jobs via HTTP API and plays synthesized audio in a configured voice channel using local Piper TTS.

## Features

- HTTP API for enqueueing speech jobs
- Local Piper TTS (no external services)
- Auto-join/leave voice channel
- Job queue with TTL, deduplication, and interrupt support
- Pluggable TTS engine architecture

## Requirements

- Docker and Docker Compose
- Discord bot token with voice permissions
- Piper voice model (`.onnx` file)

## Quick Start

### 1. Create a Discord Bot

1. Go to [Discord Developer Portal](https://discord.com/developers/applications)
2. Create a new application
3. Go to Bot settings and create a bot
4. Copy the bot token
5. Enable these Privileged Gateway Intents:
   - Server Members Intent
   - Message Content Intent
6. Go to OAuth2 > URL Generator:
   - Select `bot` scope
   - Select permissions: `Connect`, `Speak`, `Use Voice Activity`
7. Use the generated URL to invite the bot to your server

### 2. Get Guild and Channel IDs

1. Enable Developer Mode in Discord (User Settings > Advanced > Developer Mode)
2. Right-click your server name and copy the Guild ID
3. Right-click the voice channel and copy the Channel ID

### 3. Download a Piper Voice Model

```bash
mkdir -p models
# Download a voice model (example: en_US-lessac-medium)
wget -O models/en_US-lessac-medium.onnx \
  https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_US/lessac/medium/en_US-lessac-medium.onnx
wget -O models/en_US-lessac-medium.onnx.json \
  https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_US/lessac/medium/en_US-lessac-medium.onnx.json
```

### 4. Configure Environment

```bash
cp .env.example .env
# Edit .env with your values
```

**Important: PIPER_MODEL configuration**

The `PIPER_MODEL` path is set in **docker-compose.yml** (not `.env`) as the single source of truth:
```yaml
environment:
  - PIPER_MODEL=/app/models/en_US-amy-medium.onnx
```

Update this line to match your downloaded model filename. The `.env` file's `PIPER_MODEL` value (if present) will be overridden by docker-compose.yml.

### 5. Run with Docker Compose

```bash
docker compose up -d
```

## HTTP API

### Health Check

```bash
curl http://localhost:8080/v1/healthz
```

Response:
```json
{"status": "ok"}
```

### Enqueue Speech

```bash
curl -X POST http://localhost:8080/v1/speak \
  -H "Authorization: Bearer your_secret_bearer_token_here" \
  -H "Content-Type: application/json" \
  -d '{"text": "Hello, world!"}'
```

Response:
```json
{"job_id": "abc123", "message": "job enqueued"}
```

#### Request Body

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `text` | string | Yes | Text to synthesize (max 1000 chars by default) |
| `voice` | string | No | Voice/speaker ID (uses default if omitted) |
| `interrupt` | boolean | No | Cancel current playback and clear queue |
| `ttl_ms` | integer | No | Job time-to-live in milliseconds |
| `dedupe_key` | string | No | Deduplication key to prevent duplicate jobs |

#### Response Codes

| Code | Description |
|------|-------------|
| 200 | Job enqueued successfully |
| 400 | Invalid request (missing text, text too long, etc.) |
| 401 | Missing or invalid bearer token |
| 409 | Duplicate job (same dedupe_key already in queue) |
| 503 | Queue full |

### Examples

**Interrupt current playback:**
```bash
curl -X POST http://localhost:8080/v1/speak \
  -H "Authorization: Bearer $BEARER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"text": "Urgent message!", "interrupt": true}'
```

**With TTL (expires after 10 seconds):**
```bash
curl -X POST http://localhost:8080/v1/speak \
  -H "Authorization: Bearer $BEARER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"text": "Time-sensitive message", "ttl_ms": 10000}'
```

**With deduplication:**
```bash
curl -X POST http://localhost:8080/v1/speak \
  -H "Authorization: Bearer $BEARER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"text": "This will only queue once", "dedupe_key": "unique-key-123"}'
```

## Ntfy Relay Sidecar (Optional)

The ntfy relay is an optional sidecar that subscribes to [ntfy](https://ntfy.sh) topics and forwards messages to Discorgeous for speech synthesis. This allows you to trigger TTS announcements from anywhere by publishing to an ntfy topic.

### Setup

1. **Configure environment variables** in your `.env`:

```bash
# Required for relay
NTFY_TOPICS=my-alerts,my-notifications
DISCORGEOUS_BEARER_TOKEN=your_secret_bearer_token_here

# Optional settings
NTFY_SERVER=https://ntfy.sh           # Default: https://ntfy.sh
NTFY_PREFIX=[Alert]                   # Prefix added to all messages
NTFY_INTERRUPT=false                  # Interrupt current speech
NTFY_DEDUPE_WINDOW=5s                 # Prevent duplicate messages
NTFY_MAX_TEXT_LENGTH=1000             # Truncate long messages
```

2. **Start with the relay profile**:

```bash
docker compose --profile relay up -d
```

### Publishing Messages

Send a message to your ntfy topic and it will be spoken in Discord:

```bash
# Simple message
curl -d "Server backup complete" https://ntfy.sh/my-alerts

# With title (spoken as "title: message")
curl -H "Title: Deployment" -d "Production deploy finished" https://ntfy.sh/my-alerts

# Using ntfy CLI
ntfy publish my-alerts "Hello from ntfy"
```

### Relay Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `NTFY_SERVER` | `https://ntfy.sh` | Ntfy server URL |
| `NTFY_TOPICS` | (required) | Comma-separated list of topics to subscribe |
| `DISCORGEOUS_API_URL` | `http://discorgeous:8080` | Discorgeous API URL (auto-configured in Docker) |
| `DISCORGEOUS_BEARER_TOKEN` | (required) | Bearer token (must match `BEARER_TOKEN`) |
| `NTFY_PREFIX` | (none) | Prefix added to all spoken messages |
| `NTFY_INTERRUPT` | `false` | Interrupt current playback for new messages |
| `NTFY_DEDUPE_WINDOW` | `0s` | Window for deduplicating identical messages |
| `NTFY_MAX_TEXT_LENGTH` | `1000` | Maximum text length before truncation |

## Configuration

All configuration is via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `DISCORD_TOKEN` | (required) | Discord bot token |
| `GUILD_ID` | (required) | Discord guild/server ID |
| `DEFAULT_VOICE_CHANNEL_ID` | (required) | Voice channel ID to join |
| `HTTP_PORT` | `8080` | HTTP server port |
| `BEARER_TOKEN` | (optional) | Bearer token for API auth |
| `PIPER_PATH` | `piper` | Path to piper binary |
| `PIPER_MODEL` | (required) | Path to piper model file |
| `DEFAULT_VOICE` | `default` | Default voice/speaker ID |
| `AUTO_LEAVE_IDLE` | `5m` | Leave voice after idle duration |
| `MAX_TEXT_LENGTH` | `1000` | Maximum text length per request |
| `QUEUE_CAPACITY` | `100` | Maximum queue size |
| `DEFAULT_TTL` | `30s` | Default job TTL |
| `LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `LOG_FORMAT` | `text` | Log format (text, json) |

## Development

### Prerequisites

- **Go 1.25+** (required for module compatibility)
- ffmpeg (for audio format conversion)
- piper binary (for TTS synthesis)
- Opus development libraries:
  - macOS: `brew install opus pkg-config`
  - Debian/Ubuntu: `apt-get install libopus-dev pkg-config`
  - Fedora: `dnf install opus-devel pkgconf-pkg-config`

### Go Toolchain

This project requires Go 1.25 or later. The `go.mod` file specifies `go 1.25.6`.

### Dependency Notes

**discordgo pseudo-version pin**: The `go.mod` pins discordgo to a specific commit (`v0.29.1-0.20251229161010-9f6aa8159fc6`) rather than a tagged release. This is required because:
- Discord updated their voice encryption protocol
- The official tagged releases don't yet include the encryption compatibility fix
- To update later, check for a new release that includes voice encryption support, then run `go get github.com/bwmarrin/discordgo@<new-version>`

### Build

```bash
go build -o discorgeous ./cmd/discorgeous
```

### Test

```bash
go test ./...
go vet ./...
```

### Run Locally

```bash
export DISCORD_TOKEN=your_token
export GUILD_ID=your_guild_id
export DEFAULT_VOICE_CHANNEL_ID=your_channel_id
export PIPER_MODEL=/path/to/model.onnx
export BEARER_TOKEN=your_secret

./discorgeous
```

## Architecture

```
HTTP/Discord command
        │
        ▼
    Enqueue Job
        │
        ▼
  Queue (bounded, FIFO)
        │
        ▼
  Playback Worker
        │
        ├──► TTS Engine (Piper) ──► WAV audio
        │
        ├──► Audio Converter (ffmpeg) ──► Discord PCM (48kHz stereo)
        │
        └──► Voice Manager (discordgo) ──► Opus encode ──► Discord Voice
```

## License

MIT
