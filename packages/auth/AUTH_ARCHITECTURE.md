# Auth Architecture Spec

## Overview

The auth system uses a three-service, two-database architecture with **pure stateless JWT** and **JWKS asymmetric key validation**. No shared secrets between services — the main server validates JWTs using JWKS public keys fetched from BetterAuth.

```
                        Auth DB (auth.db)           Main DB (app.db)
                        ┌──────────────┐            ┌──────────────────┐
                        │ user         │            │ users            │
                        │ account      │            │   + external_id  │
                        │ session *    │            │ workspaces       │
                        │ verification │            │ workspaces_users │
                        │ jwks         │            │ http, flows, ... │
                        └──────┬───────┘            └────────┬─────────┘
                               │                             │
                     * BetterAuth sessions are               │
                       internal — we expose them              │
                       as opaque "refresh tokens"             │
                                                              │
┌────────┐    ┌───────────────────────┐    JWKS      ┌───────┴───────────┐
│ Client ├──► │ Auth-Service :8081    ├─────────────► │ Main Server :8080 │
│        │    │ (Go, pure auth)       │              │ (Go, business)    │
│        │    │                       │              │                   │
│ Stores:│    │ Returns:              │              │ Validates JWT via │
│  JWT   │    │  accessToken  (JWT)   │              │ JWKS public keys  │
│  RT    │    │  refreshToken (opaque)│              │                   │
│        │    │                       │              │ Auto-provisions   │
└────────┘    └───────────┬───────────┘              │ user on first JWT │
                          │ ConnectRPC               └───────────────────┘
              ┌───────────┴───────────┐
              │ BetterAuth :50051     │
              │ (Node.js, internal)   │
              │ jwt() plugin → JWKS   │
              │ bearer() plugin       │
              │ Owns Auth DB          │
              └───────────────────────┘
```

### Key Invariants

1. **No shared secrets.** JWKS asymmetric key validation replaces `JWT_SECRET`. The main server fetches public keys from BetterAuth's `/api/auth/jwks` endpoint.
2. **Separate databases.** Auth DB and Main DB are completely independent — never share a connection.
3. **BetterAuth owns Auth DB.** Tables: `user`, `account`, `session`, `verification`, `jwks`. Auto-created/migrated by BetterAuth.
4. **Main server owns Main DB.** Users table has `external_id` mapping to BetterAuth user IDs.
5. **Auth server is stateless.** No DB of its own — proxies to BetterAuth.
6. **Single-user local mode unchanged.** Uses dummy user for desktop/CLI, no external auth needed.
7. **User auto-provisioning.** First JWT from a new user auto-creates their account in the main DB.

### Token Design

- **`accessToken`**: JWT, short-lived (~15 min), signed by BetterAuth JWKS (RS256). Claims: `{ sub, email, name }`.
- **`refreshToken`**: BetterAuth session token (opaque string), long-lived (7 days). Used to obtain fresh access tokens.
- **Refresh flow**: Client sends refreshToken → auth-service calls BetterAuth `GetToken` → returns new accessToken.
- **Sign out**: Stateless — client discards tokens.

---

## Components

### 1. Go Auth-Service (`packages/auth/auth-service`)

**Role:** Public-facing ConnectRPC gateway on port 8081.

**Responsibilities:**

- JWT validation via JWKS-based `AuthInterceptor`
- Request routing: translates public RPCs into internal BetterAuth RPCs
- Token issuance: calls `GetToken` on BetterAuth to get JWKS-signed JWTs
- No database access — purely a proxy + JWT validator

**Public RPC Contract** (from `packages/spec/api/auth.tsp`):

| Method                | Auth | Description                                 |
| --------------------- | :--: | ------------------------------------------- |
| `SignUp`              |  No  | Create account, returns JWT + session token |
| `SignIn`              |  No  | Authenticate, returns JWT + session token   |
| `SignOut`             |  No  | Stateless — returns success                 |
| `GetOAuthUrl`         |  No  | Get OAuth provider authorization URL        |
| `HandleOAuthCallback` |  No  | Exchange OAuth code for tokens              |
| `RefreshToken`        |  No  | Get fresh JWT using session token           |
| `GetMe`               | Yes  | Get current user from JWT claims            |
| `GetLinkedAccounts`   | Yes  | List linked OAuth/credential accounts       |

### 2. Node.js BetterAuth Service (`packages/auth/betterauth`)

**Role:** Internal auth engine on port 50051. Not publicly accessible.

**Plugins:**

- `jwt()` — JWKS key management, RS256 JWT signing via `/api/auth/token`
- `bearer()` — Enables `Authorization: Bearer <session-token>` for server-to-server calls

**Internal RPC Contract** (from `packages/spec/api/auth-internal.tsp`):

| Method                   |         Uses BetterAuth         | Description                                             |
| ------------------------ | :-----------------------------: | ------------------------------------------------------- |
| `CreateUserWithPassword` |    `auth.api.signUpEmail()`     | Create user + credential account, returns session token |
| `VerifyCredentials`      |    `auth.api.signInEmail()`     | Verify password, returns session token                  |
| `GetToken`               | `auth.handler(/api/auth/token)` | Exchange session token for JWKS-signed JWT              |
| `GetOAuthUrl`            |    `auth.api.signInSocial()`    | Build OAuth URL with `disableRedirect: true`            |
| `ExchangeOAuthCode`      | `auth.handler()` + Bearer auth  | Process OAuth callback, return user + session token     |
| `GetAccountsByUserId`    |           Direct SQL            | List linked accounts                                    |
| `CreateAccount`          |     `hashPassword()` + SQL      | Link new provider account                               |

### 3. Go Main Server (`packages/server`)

**Role:** Business logic server on port 8080.

**Auth behavior depends on `AUTH_MODE`:**

- `local` (default): Single-user mode for desktop/CLI. Uses dummy user (`00000000000000000000000000`). No external auth.
- `betterauth`: Multi-user mode (self-hosted or hosted). Validates JWTs via JWKS public keys. Auto-provisions users.

**User auto-provisioning flow:**

1. JWT arrives with `sub` = BetterAuth user ID (random string)
2. Middleware looks up `users.external_id = sub`
3. Found → return internal ULID
4. Not found → create user with new ULID, `external_id = sub`, `email` from JWT claims
5. ULID stored in context — all downstream handlers unchanged

---

## Databases

### Auth DB (owned by BetterAuth)

**BetterAuth-managed tables:**

```sql
-- user: Core user identity (random string IDs, not ULIDs)
CREATE TABLE user (
  id TEXT PRIMARY KEY, email TEXT UNIQUE NOT NULL,
  emailVerified INTEGER, name TEXT, image TEXT,
  createdAt INTEGER, updatedAt INTEGER
);

-- account: Linked auth providers (credential, Google, etc.)
CREATE TABLE account (
  id TEXT PRIMARY KEY, userId TEXT NOT NULL REFERENCES user(id),
  providerId TEXT NOT NULL, accountId TEXT NOT NULL,
  password TEXT, -- bcrypt hash (credential accounts only)
  accessToken TEXT, refreshToken TEXT, accessTokenExpiresAt INTEGER,
  scope TEXT, idToken TEXT, createdAt INTEGER, updatedAt INTEGER
);

-- session: BetterAuth's internal sessions (exposed as "refresh tokens")
CREATE TABLE session (
  id TEXT PRIMARY KEY, userId TEXT NOT NULL REFERENCES user(id),
  token TEXT UNIQUE NOT NULL, expiresAt INTEGER,
  ipAddress TEXT, userAgent TEXT, createdAt INTEGER, updatedAt INTEGER
);

-- jwks: RSA key pairs for JWT signing (managed by jwt() plugin)
CREATE TABLE jwks (
  id TEXT PRIMARY KEY, publicKey TEXT NOT NULL,
  privateKey TEXT NOT NULL, createdAt INTEGER
);
```

### Main DB (owned by Main Server)

```sql
-- users: Internal user accounts (ULID-based IDs)
CREATE TABLE users (
  id BLOB NOT NULL PRIMARY KEY,    -- 16-byte ULID
  email TEXT NOT NULL UNIQUE,
  password_hash BLOB,
  provider_type INT8 NOT NULL DEFAULT 0,
  provider_id TEXT,
  external_id TEXT UNIQUE,          -- Maps to BetterAuth user.id
  status INT8 NOT NULL DEFAULT 0,
  UNIQUE (provider_type, provider_id)
);
```

The `external_id` column is the bridge: it maps BetterAuth's random string user IDs to internal 16-byte ULIDs. This keeps the main server's ID system independent.

---

## Request Flows

### SignUp (email/password)

```
Client → Go:SignUp(email, password, name)
  Go → Node:CreateUserWithPassword(email, password, name)
    BetterAuth: bcrypt hash, INSERT user + account + session
  Go ← { user, sessionToken }
  Go → Node:GetToken(sessionToken)
    BetterAuth: sign JWT with JWKS private key (RS256)
  Go ← { accessToken }
Go ← { user, accessToken, refreshToken=sessionToken }
```

### SignIn (email/password)

```
Client → Go:SignIn(email, password)
  Go → Node:VerifyCredentials(email, password)
    BetterAuth: verify bcrypt, create session
  Go ← { valid, user, sessionToken }
  Go → Node:GetToken(sessionToken)
  Go ← { accessToken }
Go ← { user, accessToken, refreshToken=sessionToken }
```

### OAuth (e.g., Google)

```
Client → Go:GetOAuthUrl(GOOGLE, callbackUrl)
  Go → Node:GetOAuthUrl(GOOGLE, callbackUrl)
    BetterAuth: signInSocial(disableRedirect: true) → builds URL
  Go ← { url, state }

[User authorizes on Google, redirects back with code]

Client → Go:HandleOAuthCallback(GOOGLE, code, state)
  Go → Node:ExchangeOAuthCode(GOOGLE, code, state)
    BetterAuth: handler(callback request) → exchange code, find/create user
    BetterAuth: getSession(Bearer sessionToken) → retrieve user
  Go ← { user, sessionToken, isNewUser }
  Go → Node:GetToken(sessionToken)
  Go ← { accessToken }
Go ← { user, accessToken, refreshToken=sessionToken, isNewUser }
```

### RefreshToken

```
Client → Go:RefreshToken(refreshToken)
  // refreshToken IS the BetterAuth session token
  Go → Node:GetToken(refreshToken)
    BetterAuth: validate session, sign fresh JWT
  Go ← { accessToken }
Go ← { accessToken, refreshToken (unchanged) }
```

### Authenticated Request to Main Server

```
Client → Main:AnyRPC (Bearer accessToken)
  Middleware: validate JWT via JWKS public keys
  Middleware: extract sub → lookup external_id → get internal ULID
  Middleware: (first time) auto-create user in main DB
  Handler: receives context with internal ULID
```

---

## Environment Variables

### BetterAuth Service

| Variable               | Required  | Default        | Description                           |
| ---------------------- | :-------: | -------------- | ------------------------------------- |
| `DATABASE_URL`         |    No     | `file:auth.db` | Auth DB URL                           |
| `DATABASE_AUTH_TOKEN`  | For Turso | —              | Turso auth token                      |
| `AUTH_SECRET`          |  **Yes**  | —              | BetterAuth internal encryption secret |
| `BETTERAUTH_PORT`      |    No     | `50051`        | Internal RPC port                     |
| `GOOGLE_CLIENT_ID`     |    No     | —              | Google OAuth credentials              |
| `GOOGLE_CLIENT_SECRET` |    No     | —              |                                       |

### Go Auth-Service

| Variable            | Required | Default                        | Description                      |
| ------------------- | :------: | ------------------------------ | -------------------------------- |
| `BETTERAUTH_URL`    |    No    | `http://localhost:50051`       | BetterAuth service URL           |
| `JWKS_URL`          |    No    | `BETTERAUTH_URL/api/auth/jwks` | JWKS endpoint for JWT validation |
| `AUTH_SERVICE_ADDR` |    No    | `:8081`                        | Listen address                   |

### Go Main Server

| Variable         |  Required  | Default | Description                                          |
| ---------------- | :--------: | ------- | ---------------------------------------------------- |
| `AUTH_MODE`      |     No     | `local` | `local` or `betterauth`                              |
| `JWKS_URL`       | betterauth | —       | JWKS endpoint URL (or derived from `BETTERAUTH_URL`) |
| `BETTERAUTH_URL` | betterauth | —       | Used to derive JWKS_URL if not set                   |

---

## BetterAuth Configuration

```typescript
import { bearer, jwt } from 'better-auth/plugins';

export const auth = betterAuth({
  database: drizzleAdapter(db, { provider: 'sqlite' }),
  secret: process.env.AUTH_SECRET,
  emailAndPassword: { enabled: true },
  session: {
    expiresIn: 60 * 60 * 24 * 7, // 7 days
    updateAge: 60 * 60 * 24, // 1 day
  },
  plugins: [
    bearer(), // Enables Authorization: Bearer <session-token>
    jwt({
      jwks: { keyPairConfig: { alg: 'RS256' } },
      jwt: {
        definePayload: ({ user }) => ({
          email: user.email,
          name: user.name,
          sub: user.id,
        }),
      },
    }),
  ],
  socialProviders: {
    /* google */
  },
});
```

---

## Security Considerations

1. **No shared secrets.** JWKS asymmetric keys replace JWT_SECRET. Public keys fetched from BetterAuth, private keys never leave BetterAuth.
2. **AUTH_SECRET is BetterAuth-internal.** Used for session cookie signing. Not shared with Go services.
3. **Internal service not publicly accessible.** BetterAuth on `:50051` should only be reachable from the Go auth-service.
4. **Generic error messages.** "invalid credentials" for both wrong-email and wrong-password (no user enumeration).
5. **Password hashing.** Handled by BetterAuth (bcrypt). Plaintext passwords never stored.
6. **OAuth state parameter.** CSRF protection on OAuth flows.
7. **User auto-provisioning race safety.** UNIQUE constraint on `external_id` + retry logic handles concurrent first-requests.

---

## File Structure

```
packages/
  auth/
    AUTH_ARCHITECTURE.md            ← This file

    auth-service/                   ← Go auth-service (public RPC gateway)
      cmd/auth-service/main.go      ← Entry point (JWKS_URL config)
      pkg/
        client/betterauth.go        ← ConnectRPC client to BetterAuth
        handler/
          auth.go                   ← AuthService handlers + JWKS interceptor
          jwks.go                   ← JWKS fetching + RSA key parsing
          auth_test.go              ← Tests with mock BetterAuth client

    betterauth/                     ← Node.js BetterAuth service (internal)
      src/
        index.ts                    ← Entry point, Effect-TS lifecycle
        auth.ts                     ← BetterAuth instance (jwt, bearer plugins)
        db.ts                       ← Database initialization
        service.ts                  ← AuthInternalService implementation
        queries.ts                  ← SQL queries for account + user tables
        test-utils.ts               ← Test helper (in-memory DB)
        service.test.ts             ← Unit tests
        integration.test.ts         ← RPC integration tests

  server/                           ← Go main server
    internal/
      api/middleware/mwauth/
        mwauth.go                   ← Auth middleware (JWKS + auto-provisioning)
        mwauth_jwks.go              ← JWKS utilities
        mwauth_test.go              ← BetterAuth interceptor tests
      migrations/
        01KGTFDM_add_external_id.go ← Migration: external_id column

  spec/
    api/
      auth.tsp                      ← Public auth API (TypeSpec)
      auth-internal.tsp             ← Internal auth API (TypeSpec)
```
