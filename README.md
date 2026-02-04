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

- Go 1.23+
- ffmpeg
- piper binary
- Opus development libraries (`libopus-dev`)

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
