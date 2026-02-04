# PRD-3: Ntfy → Discorgeous relay (optional sidecar)

## Summary
Add an **optional** ntfy subscriber that watches one or more configured ntfy topics and forwards received messages into the existing Discorgeous HTTP enqueue API (`POST /v1/speak`).

This should run as a **separate container** (sidecar) in `docker-compose.yml`, so Discorgeous remains focused on Discord voice + queue/tts, and the relay remains focused on ntfy subscription + forwarding.

## Goals
- Provide a `discorgeous-ntfy-relay` process that:
  - Subscribes to ntfy JSON stream for configured topics.
  - For each incoming message, calls Discorgeous `POST /v1/speak` with bearer auth.
  - Reconnects automatically on stream disconnect.
  - Supports basic filtering / formatting and optional interrupt/dedupe behavior.
- Add a `docker compose` service `ntfy-relay` that runs alongside `discorgeous`.
- Keep everything self-host friendly and secret-safe (`.env` remains uncommitted).

## Non-goals
- No inbound HTTP endpoints on the relay (it is an outbound client only).
- No per-message voice-channel selection (Discorgeous remains one-guild + default channel).
- No persistence/guaranteed delivery (best-effort forwarding; ntfy itself buffers).

## Proposed UX
### Minimal docker-compose usage
- User sets env vars (in their local `.env`, not committed):
  - `NTFY_SERVER` (default `https://ntfy.sh`)
  - `NTFY_TOPICS` (required; comma-separated)
  - `DISCORGOUS_API_URL` (default `http://discorgeous:8080`)
  - `DISCORGOUS_BEARER_TOKEN` (must match `BEARER_TOKEN` used by Discorgeous)
  - Optional:
    - `NTFY_PREFIX` (string to prefix spoken text)
    - `NTFY_INTERRUPT` (`true|false`, default false)
    - `NTFY_DEDUPE_WINDOW` (duration; if set, use dedupe_key to prevent repeats)
    - `NTFY_MAX_TEXT_LENGTH` (cap before forwarding; default match discorgeous max)

### Mapping to Discorgeous API
Forwarded request body:
```json
{
  "text": "<formatted text>",
  "interrupt": false,
  "ttl_ms": 0,
  "dedupe_key": "..."
}
```

## Implementation Plan

### Task 1 — Relay client implementation (new cmd)
- Add `cmd/ntfy-relay/main.go` (or similar) implementing:
  - Config struct + env parsing (reuse patterns from `internal/config`, but keep relay simple).
  - Subscribe to ntfy JSON stream endpoint: `GET {NTFY_SERVER}/{topic}/json` (or streaming variant) and decode JSON objects.
  - For each message:
    - Extract text fields (title/message) safely.
    - Apply prefix/formatting.
    - Enforce max length.
    - POST to Discorgeous `/v1/speak` with `Authorization: Bearer <token>`.
  - Reconnect loop with backoff and context cancellation.
  - Structured logs.

### Task 2 — Add Docker support for relay
- Update `Dockerfile` to build a second binary for relay.
- Update `docker-compose.yml`:
  - Add `ntfy-relay` service using the same image (or a small second image) and run the relay binary.
  - `depends_on: discorgeous` and use internal network URL `http://discorgeous:8080`.
  - Ensure no model files are required in relay.

### Task 3 — Documentation
- Update `README.md`:
  - Add section: “ntfy relay sidecar”.
  - Provide `.env` example entries (in `.env.example`, safe defaults, no secrets).
  - Include an example ntfy publish command and expected behavior.

### Task 4 — Tests
- Unit tests for:
  - config parsing (env → config)
  - dedupe logic (if implemented)
  - formatting + max length trimming
- Use `httptest.Server` to validate outgoing requests to Discorgeous.

## Acceptance Checklist
- [ ] `go test ./...` passes
- [ ] `go vet ./...` passes
- [ ] `gofmt -l .` is clean
- [ ] `docker compose build` succeeds
- [ ] With `docker compose up`, publishing a message to the configured ntfy topic results in a `POST /v1/speak` to Discorgeous (verified by logs and/or hearing audio in Discord)

## Test Plan (manual)
1. Configure `.env` locally with `NTFY_TOPICS=<topic>` and matching bearer token.
2. `docker compose up -d --build`
3. Publish:
   - `curl -d "hello from ntfy" "${NTFY_SERVER:-https://ntfy.sh}/$NTFY_TOPICS"`
4. Observe relay logs show receive + forwarded request.
5. Observe discorgeous logs show speak job enqueued and played.
