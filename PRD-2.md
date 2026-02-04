# PRD: Address review feedback (stability, Docker/Go consistency, voice reliability)

## Goal
Tighten up discorgeous-go based on PR review feedback by:
- Making tests more deterministic (reduce flakiness)
- Improving startup safety messaging and shutdown behavior
- Improving Discord voice connect/reconnect reliability and error reporting
- Ensuring Docker/Go toolchain + build dependencies are consistent and documented
- Minor code-quality cleanup (dedupe helpers/errors)

## Non-goals
- Adding multi-guild support
- Adding per-request channel selection
- Implementing a full command suite / permissions management
- Replacing Discord voice approach with Lavalink (unless needed as a follow-up PRD)

## Constraints
- Work on a new branch (do not directly modify main).
- Keep changes small and reviewable.
- Preserve current end-to-end behavior (HTTP enqueue -> speak in configured VC).
- Do not commit secrets (.env) or voice models.

## Success criteria
- `go test ./...` and `go vet ./...` pass reliably (no time.Sleep-based races)
- Docker build succeeds on Apple Silicon and amd64
- On startup, app logs a clear warning when bearer auth is disabled (empty BEARER_TOKEN)
- Voice connect path uses context deadlines (no indefinite polling) and provides actionable errors
- Clean shutdown closes voice connection if connected
- README documents:
  - required Go version/toolchain choice
  - why discordgo is pinned to a pseudo-version (Discord voice encryption compatibility)
  - docker-compose PIPER_MODEL configuration (single source of truth)

## Design notes
- Prefer deterministic synchronization primitives in tests: channels, WaitGroups, or condition variables.
- For voice readiness: wait on discordgo events/state with a context timeout rather than polling vc.Ready in a tight loop.
- Keep the reconnect strategy minimal: retry connect/playback once (or small bounded retries) with backoff; ensure interrupt/cancel semantics remain correct.

## Implementation tasks

### Task 1: Go/Docker/toolchain consistency
- [x] Decide and enforce a supported Go version for the repo (update go.mod and Dockerfile builder image accordingly).
- [x] Ensure Dockerfile builder includes all CGO deps needed for gopus (pkg-config, libopus-dev).
- [x] Ensure runtime stage includes only runtime deps (ffmpeg, libopus0, ca-certs, etc.).
- [x] Update README "Development" section with required packages and Go version.

### Task 2: Safer config + startup warnings
- [x] If BEARER_TOKEN is empty, log a WARN at startup indicating HTTP auth is disabled.
- [ ] (Optional) Add a config flag to require auth even in dev, if desired.

### Task 3: Deterministic queue/playback tests
- [ ] Remove/replace time.Sleep synchronization in queue tests with deterministic signals.
- [ ] Use timeouts only as test failsafes (e.g., context with deadline) rather than primary synchronization.

### Task 4: Code cleanup (dedupe helpers/errors)
- [ ] Consolidate duplicate WAV little-endian helper functions into a shared internal utility.
- [ ] Consolidate duplicate ErrSynthesisFailed definitions (or rename to be domain-specific).
- [ ] Replace magic numbers with named constants where appropriate.

### Task 5: Voice connect/reconnect + shutdown reliability
- [ ] Replace voice readiness polling with a context-aware wait (no unbounded loops).
- [ ] Improve error propagation/logging for speaking state and voice websocket errors.
- [ ] Add minimal reconnect logic or a bounded retry on voice connect failure.
- [ ] Add graceful shutdown: on Stop(), disconnect from voice if connected.

### Task 6: Documentation updates
- [ ] Add a short note explaining discordgo pseudo-version pin (Discord voice encryption mode compatibility) and how to update later.
- [ ] Ensure docker-compose does not fight `.env` for PIPER_MODEL; pick a single source of truth and document it.

## Test plan
- Commands that must pass:
  - `go test ./...`
  - `go vet ./...`
  - `gofmt -w .` (or verify formatting is clean)
  - `docker compose build`

- Manual smoke test:
  - `docker compose up -d`
  - `curl -X POST http://127.0.0.1:8080/v1/speak ...`
  - confirm bot joins VC, speaks, and auto-leaves after idle

## Acceptance checklist
- [ ] Tests pass reliably
- [ ] Docker build succeeds
- [ ] Startup warnings are clear
- [ ] Voice connection handling is more robust
- [ ] Docs updated
- [ ] Changes committed on a branch and pushed
