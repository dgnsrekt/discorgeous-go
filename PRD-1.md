# PRD: discorgeous-go (Go rewrite) — Self-hosted Discord voice TTS bot

## Goal
Build a self-hosted Discord bot (one guild per running instance) that:
- Accepts speech jobs via a small **HTTP enqueue API**
- Auto-joins a **single configured voice channel**
- Generates audio using **local Piper** (no external TTS)
- Streams speech into Discord voice
- Auto-leaves after an idle timeout

## Non-goals
- Multi-guild support in one process
- Per-request voice channel overrides (always speak in DEFAULT_VOICE_CHANNEL_ID)
- Music playback, soundboards, or arbitrary file streaming
- Full Discord voice protocol implementation from scratch (use a Go library or an external voice node like Lavalink)
- Production-grade auth beyond a static bearer token + bind-to-localhost (good enough for self-host)

## Constraints
- Must run in Docker.
- One guild per instance.
- Default voice channel only.
- Local Piper required; keep TTS pluggable so other engines can be added later.
- Prefer minimal dependencies and a clean, testable architecture.

## Success criteria
- `POST /v1/speak` with bearer auth enqueues a job.
- If not connected, bot auto-joins DEFAULT_VOICE_CHANNEL_ID and speaks the text.
- Playback is serialized (no overlapping speech).
- After queue drains, bot leaves after AUTO_LEAVE_IDLE.
- Works end-to-end locally with a provided dev setup (env vars + docker-compose).

## Design notes
### Core pipeline
HTTP/Discord command -> Enqueue -> (ensure voice connected) -> TTS -> Audio pipeline -> Voice send

### Interfaces
- `TTSEngine` interface with `Synthesize(ctx, req) (AudioResult, error)`
- Initial implementation: `PiperEngine`

### HTTP API
- `POST /v1/speak` (Bearer auth) body:
  - `text` (required)
  - `voice` (optional; default from env)
  - `interrupt` (optional; cancels current playback + flushes queue)
  - `ttl_ms` (optional)
  - `dedupe_key` (optional)
- `GET /v1/healthz`

### Discord UX (minimal)
- Slash command `/say text:<...>` uses the same enqueue path as HTTP.
- Slash command `/voice status` reports connected state + queue depth.

### Voice sending approach
Start with a proven Go Discord library voice sender path. If voice send is too brittle with current Discord voice changes, pivot to Lavalink.

## Implementation tasks

### Task 1: Project scaffold + config
- [x] Create Go module, basic directory layout (`cmd/`, `internal/`)
- [x] Add config loader from env (with sane defaults)
- [x] Add structured logging

### Task 2: HTTP enqueue API
- [x] Implement HTTP server + routes `/v1/speak`, `/v1/healthz`
- [x] Implement bearer auth middleware
- [x] Validate payload: max text length, TTL, queue capacity

### Task 3: Queue + playback worker
- [x] Implement bounded queue and speak job type
- [x] Implement single playback goroutine
- [x] Implement interrupt semantics
- [x] Implement auto-leave idle timer

### Task 4: TTS engine (Piper)
- [x] Implement `TTSEngine` + registry
- [x] Implement `PiperEngine` invoking local `piper` binary
- [x] Decide audio artifact contract (WAV on disk or stream) and implement

### Task 5: Audio pipeline to Discord voice
- [x] Convert Piper output to Discord-ready stream (likely via ffmpeg to 48k stereo PCM)
- [x] Encode/stream to Discord voice using chosen library OR Lavalink
- [x] Basic integration tests (smoke test command + manual runbook)

### Task 6: Docker + docs
- [x] Dockerfile (multi-stage) including runtime deps (piper, ffmpeg, certs)
- [x] docker-compose example + env template
- [x] README: setup, permissions, commands, HTTP API examples

## Test plan
Commands that must pass:
- `go test ./...`
- `go vet ./...`
- `gofmt -w .` (enforced in CI or pre-commit)

Manual smoke test:
- Start container with env vars set.
- `curl POST /v1/speak` and confirm audio plays in configured voice channel.

## Acceptance checklist
- [ ] HTTP enqueue works with bearer auth
- [ ] Auto-join + speak + auto-leave works
- [ ] Piper used locally; TTS interface is pluggable
- [ ] `go test ./...` and `go vet ./...` pass
- [ ] Docker build succeeds
- [ ] README + docker-compose included
- [ ] Changes committed on a branch and pushed
