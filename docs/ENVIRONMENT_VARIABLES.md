# Environment Variables

Consolidated reference of all environment variables used across DevTools services.

---

## Main Server (`packages/server`)

The business logic server handling workspaces, flows, HTTP requests, files, and credentials.

| Variable | Required | Default | Description |
|---|:---:|---|---|
| `PORT` | No | `8080` | TCP listen port (only used when `SERVER_MODE=tcp`) |
| `HMAC_SECRET` | **Yes** | — | HMAC secret for cryptographic operations |
| `AUTH_MODE` | No | `local` | Authentication mode: `local` (desktop/CLI, dummy user) or `betterauth` (SaaS, JWKS JWT validation) |
| `JWKS_URL` | When `betterauth` | — | URL to fetch JWKS public keys for JWT validation. Auto-derived from `BETTERAUTH_URL` + `/api/auth/jwks` if not set |
| `BETTERAUTH_URL` | When `betterauth` | — | Base URL of BetterAuth service. Used to derive `JWKS_URL` if `JWKS_URL` is not set |
| `DB_MODE` | **Yes** | — | Database mode (currently only `LOCAL` is supported) |
| `DB_NAME` | **Yes** | — | Database file name (without `.db` extension) |
| `DB_PATH` | **Yes** | — | Directory path where the database file is stored |
| `DB_ENCRYPTION_KEY` | **Yes** | — | Encryption key for the SQLite database |
| `LOG_LEVEL` | No | `ERROR` | Log verbosity: `DEBUG`, `INFO`, `WARNING`, `ERROR` |
| `SERVER_MODE` | No | `uds` | Listener mode: `uds` (Unix Domain Socket) or `tcp` |
| `SERVER_SOCKET_PATH` | No | `/tmp/the-dev-tools/server.socket` | Custom Unix socket path (only used when `SERVER_MODE=uds`) |
| `WORKER_MODE` | No | `uds` | Worker-js connection mode: `uds` or `tcp` |
| `WORKER_SOCKET_PATH` | No | `/tmp/the-dev-tools/worker-js.socket` | Worker-js Unix socket path (only used when `WORKER_MODE=uds`) |
| `WORKER_URL` | No | `http://localhost:9090` | Worker-js URL (only used when `WORKER_MODE=tcp`) |

### Dev-only (build tag: `dev`)

| Variable | Required | Default | Description |
|---|:---:|---|---|
| `DEVTOOLS_REPLAY_DIR` | No | `/tmp/devtools-replay` | Directory for mutation event replay JSONL files |

---

## Auth Service (`packages/auth/auth-service`)

Stateless Go proxy that forwards auth RPCs to BetterAuth.

| Variable | Required | Default | Description |
|---|:---:|---|---|
| `BETTERAUTH_URL` | No | `http://localhost:50051` | BetterAuth internal service URL |
| `JWKS_URL` | No | `BETTERAUTH_URL` + `/api/auth/jwks` | JWKS endpoint for JWT validation |
| `AUTH_SERVICE_ADDR` | No | `:8081` | Listen address (host:port) |

---

## BetterAuth (`packages/auth/betterauth`)

Internal Node.js auth engine (BetterAuth + Effect-TS). Owns the Auth DB.

| Variable | Required | Default | Description |
|---|:---:|---|---|
| `BETTERAUTH_PORT` | No | `50051` | Internal RPC listen port |
| `BETTERAUTH_URL` | No | `http://localhost:{BETTERAUTH_PORT}` | Public base URL (used for OAuth callbacks, JWKS endpoint) |
| `DATABASE_URL` | No | `file:auth.db` | Auth database URL (SQLite file path or Turso URL) |
| `DATABASE_AUTH_TOKEN` | For Turso | — | Turso authentication token |
| `AUTH_SECRET` | **Yes** | `development-auth-secret-change-in-production` | BetterAuth internal encryption secret. **Must be changed in production** |
| `GOOGLE_CLIENT_ID` | No | — | Google OAuth client ID |
| `GOOGLE_CLIENT_SECRET` | No | — | Google OAuth client secret |
| `GITHUB_CLIENT_ID` | No | — | GitHub OAuth client ID |
| `GITHUB_CLIENT_SECRET` | No | — | GitHub OAuth client secret |
| `MICROSOFT_CLIENT_ID` | No | — | Microsoft OAuth client ID |
| `MICROSOFT_CLIENT_SECRET` | No | — | Microsoft OAuth client secret |

---

## CLI (`apps/cli`)

| Variable | Required | Default | Description |
|---|:---:|---|---|
| `LOG_LEVEL` | No | `ERROR` | Log verbosity: `DEBUG`, `INFO`, `WARNING`, `ERROR` |

---

## Testing & Integration

These variables are only used when running integration tests. They are never required for normal development.

### AI Integration Tests

Build tag: `ai_integration` | Guard: `RUN_AI_INTEGRATION_TESTS=true`

| Variable | Description |
|---|---|
| `RUN_AI_INTEGRATION_TESTS` | Set to `true` to enable AI integration tests |
| `OPENAI_API_KEY` | OpenAI API key |
| `OPENAI_BASE_URL` | OpenAI base URL (custom endpoint) |
| `OPENAI_MODEL` | OpenAI model override |
| `ANTHROPIC_API_KEY` | Anthropic API key |
| `ANTHROPIC_BASE_URL` | Anthropic base URL (custom endpoint) |
| `ANTHROPIC_MODEL` | Anthropic model override |
| `ANTHROPIC_AUTH_TOKEN` | Anthropic auth token (custom provider tests) |
| `GEMINI_API_KEY` | Google Gemini API key |
| `MINIMAX_API_KEY` | Minimax API key (custom provider tests) |

### E2E / Flow Tests

| Variable | Description |
|---|---|
| `SKIP_E2E` | Set to `true` to skip end-to-end JS flow tests |
| `MONOREPO_ROOT` | Override monorepo root path for test discovery |
