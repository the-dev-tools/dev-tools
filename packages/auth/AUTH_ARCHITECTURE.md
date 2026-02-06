# Auth Architecture Spec

## Overview

The auth system is a two-process architecture: a **Go auth-service** (public-facing RPC gateway) and a **Node.js BetterAuth service** (internal auth engine). BetterAuth owns the auth database exclusively — the Go service never touches it.

The core idea: **BetterAuth is the auth engine, not a helper.** It handles users, passwords, OAuth, sessions, and account linking. We build a thin custom layer on top for JWT issuance (so the Go main server can validate tokens locally) and refresh token rotation.

```
                        Public Network                    Internal Network
                    ┌──────────────────┐             ┌──────────────────────────────┐
                    │                  │             │                              │
Client ─────────►  │  Go auth-service │ ──ConnRPC──►│  Node.js BetterAuth          │
  (browser/CLI)    │  :8081           │             │  :50051                      │
                    │                  │             │                              │
                    │  - JWT validate  │             │  ┌────────────────────────┐  │
                    │  - RPC gateway   │             │  │ BetterAuth Engine      │  │
                    │  - Interceptor   │             │  │  - Email/Password      │  │
                    │                  │             │  │  - OAuth (Google,      │  │
                    │                  │             │  │    GitHub, Microsoft)  │  │
                    │                  │             │  │  - Account linking     │  │
                    └──────────────────┘             │  │  - User management    │  │
                                                     │  │  - Session management │  │
Client ─────────►  Go main server :8080              │  └────────┬───────────────┘  │
                    │  - JWT validate (shared secret) │           │                  │
                    │  - Business logic               │  ┌────────▼───────────────┐  │
                    └─────────────────────────────────│  │ Auth Database          │  │
                                                     │  │ (SQLite / LibSQL)      │  │
                                                     │  │ Drizzle ORM adapter    │  │
                                                     │  └────────────────────────┘  │
                                                     └──────────────────────────────┘
```

### Key Principles

1. **BetterAuth is the auth engine.** It manages users, accounts, sessions, passwords, and OAuth. We call `auth.api.*` — not raw SQL — for these operations.
2. **Shared JWT secret.** Both Go servers validate JWTs using the same `JWT_SECRET`. The main server never calls the auth-service — it validates tokens locally.
3. **Custom JWT layer on top.** We issue our own JWTs (for the Go main server) and manage refresh token rotation. BetterAuth sessions exist internally but aren't exposed to clients.
4. **Plugin-driven extensibility.** New auth capabilities (2FA, passkeys, SSO, organizations) are added by enabling BetterAuth plugins — not by writing new code.

---

## What BetterAuth Provides

### Core (built-in, zero config)

| Capability | BetterAuth API | What it handles |
|---|---|---|
| Email/password signup | `auth.api.signUpEmail()` | bcrypt hashing, user + account creation, session |
| Email/password signin | `auth.api.signInEmail()` | bcrypt verification, session creation |
| Social OAuth | `auth.api.signInSocial()` | Full OAuth dance: redirect URL, code exchange, user create/link |
| Account linking | `auth.api.linkSocialAccount()` | Link multiple providers to one user |
| User management | `auth.api.getSession()` | User lookup by session |
| List accounts | `auth.api.listAccounts()` | Query linked providers for a user |
| Email verification | `auth.api.sendVerificationEmail()` | Token generation, verification flow |
| Password reset | `auth.api.forgetPassword()` / `resetPassword()` | Token-based password reset |
| Session management | Automatic | DB-backed sessions with expiry, refresh |
| Schema management | Automatic | Creates/migrates `user`, `account`, `session`, `verification` tables |

### Social Providers (one-line config each)

```typescript
socialProviders: {
  google:    { clientId, clientSecret },  // Google OAuth 2.0
  github:    { clientId, clientSecret },  // GitHub OAuth
  microsoft: { clientId, clientSecret },  // Microsoft / Azure AD
  apple:     { clientId, clientSecret },  // Apple Sign-In
  discord:   { clientId, clientSecret },  // Discord OAuth
  // ... 20+ providers supported
}
```

### Plugins (add capability with one line)

| Plugin | Import | What it adds |
|---|---|---|
| `twoFactor()` | `better-auth/plugins` | TOTP, SMS codes, backup codes |
| `passkey()` | `@better-auth/passkey` | WebAuthn/FIDO2 passwordless login |
| `magicLink()` | `better-auth/plugins` | Email magic link authentication |
| `emailOTP()` | `better-auth/plugins` | Email one-time password |
| `phoneNumber()` | `better-auth/plugins` | SMS-based authentication |
| `username()` | `better-auth/plugins` | Username + password (alt to email) |
| `anonymous()` | `better-auth/plugins` | Guest sessions, convert to full user later |
| `admin()` | `better-auth/plugins` | User management, impersonation, roles/permissions |
| `organization()` | `better-auth/plugins` | Multi-tenancy, teams, member roles, invitations |
| `sso()` | `better-auth/plugins` | Enterprise SAML + OIDC SSO, per-org providers |
| `apiKey()` | `better-auth/plugins` | API key generation + validation |
| `multiSession()` | `better-auth/plugins` | Switch between multiple logged-in accounts |
| `oneTap()` | `better-auth/plugins` | Google One Tap sign-in |
| `oidcProvider()` | `better-auth/plugins` | Turn your app into an OIDC identity provider |

**Adding a plugin is config-only — no new service code needed:**
```typescript
import { twoFactor, organization } from "better-auth/plugins";

const auth = betterAuth({
  // ...existing config
  plugins: [twoFactor(), organization()],
});
```

### Plugin Roadmap

Phase 1 (MVP):
- Core email/password + OAuth (Google, GitHub) — **current scope**

Phase 2 (Post-launch):
- `twoFactor()` — TOTP + backup codes
- `passkey()` — WebAuthn passwordless

Phase 3 (Enterprise):
- `organization()` — teams, workspaces, member roles
- `sso()` — SAML/OIDC for enterprise customers
- `admin()` — user management dashboard

---

## Components

### 1. Go Auth-Service (`packages/auth/auth-service`)

**Role:** Public-facing ConnectRPC gateway on port 8081.

**Responsibilities:**
- JWT validation via `AuthInterceptor` (middleware)
- Request routing: translates public `AuthService` RPCs into internal `AuthInternalService` RPCs
- Client info extraction (IP, User-Agent from headers)
- No database access — purely a proxy + JWT validator

**Public RPC Contract** (from `packages/spec/api/auth.tsp`):

| Method | Auth | Description |
|---|:---:|---|
| `SignUp` | No | Create account with email/password |
| `SignIn` | No | Authenticate with email/password |
| `SignOut` | Yes | Revoke refresh token |
| `GetOAuthUrl` | No | Get OAuth provider authorization URL |
| `HandleOAuthCallback` | No | Exchange OAuth code for tokens |
| `RefreshToken` | No | Rotate refresh token, get new access token |
| `GetMe` | Yes | Get current user profile |
| `GetLinkedAccounts` | Yes | List linked OAuth/credential accounts |

### 2. Node.js BetterAuth Service (`packages/auth/betterauth`)

**Role:** Internal auth engine on port 50051. Not publicly accessible.

**Two layers:**
1. **BetterAuth engine** — handles users, passwords, OAuth, sessions, account linking
2. **Thin ConnectRPC adapter** — translates internal RPC calls into `auth.api.*` calls + custom JWT/refresh token logic

**Internal RPC Contract** (from `packages/spec/api/auth-internal.tsp`):

| Method | Uses BetterAuth | Custom Logic | Description |
|---|:---:|:---:|---|
| `CreateUser` | `auth.api.signUpEmail()` | | Create user (OAuth onboarding, no password) |
| `CreateUserWithPassword` | `auth.api.signUpEmail()` | | BetterAuth handles bcrypt + user/account insert |
| `VerifyCredentials` | `auth.api.signInEmail()` | | BetterAuth verifies bcrypt hash |
| `GetUser` | `auth.api.getSession()` | | User lookup via BetterAuth |
| `CreateTokens` | | JWT + refresh token | Sign JWT with `jose`, store refresh token |
| `RefreshTokens` | | JWT + refresh token | Rotate refresh token, issue new JWT |
| `RevokeRefreshToken` | | SQL delete | Delete refresh token (sign out) |
| `GetOAuthUrl` | Social provider config | | Build OAuth authorization URL |
| `ExchangeOAuthCode` | `auth.api.signInSocial()` | JWT + refresh token | BetterAuth does OAuth dance, we issue JWT |
| `GetAccountsByUserId` | `auth.api.listAccounts()` | | List linked accounts via BetterAuth |
| `CreateAccount` | `auth.api.linkSocialAccount()` | | Link a new provider to existing user |

---

## Database

### Ownership

BetterAuth exclusively owns the auth database. The Go auth-service has zero database access. BetterAuth manages schema creation and migrations automatically.

### Storage Modes

The `@libsql/client` package transparently supports all three modes. The Node.js service selects the mode based on the `DATABASE_URL` environment variable:

| Mode | `DATABASE_URL` Value | Use Case |
|---|---|---|
| **In-memory** | `:memory:` | Unit tests, CI |
| **File SQLite** | `file:auth.db` | Desktop app, local dev, CLI |
| **Remote LibSQL** | `libsql://your-db.turso.io` | SaaS production (Turso) |

Remote mode also requires `DATABASE_AUTH_TOKEN` for Turso authentication.

### Database Adapter: Drizzle + @libsql/client

BetterAuth connects to the database through the Drizzle adapter, which wraps `@libsql/client`:

```typescript
import { betterAuth } from "better-auth";
import { drizzleAdapter } from "better-auth/adapters/drizzle";
import { drizzle } from "drizzle-orm/libsql";
import { createClient } from "@libsql/client";

const client = createClient({
  url: process.env.DATABASE_URL || "file:auth.db",
  authToken: process.env.DATABASE_AUTH_TOKEN,
});
const db = drizzle(client);

const auth = betterAuth({
  database: drizzleAdapter(db, { provider: "sqlite" }),
  // ...
});
```

This single code path works for all three modes (`:memory:`, `file:`, `libsql://`). The `@libsql/client` is also available directly for custom queries (refresh token table).

### Schema

**BetterAuth-managed tables** (auto-created, auto-migrated):

```sql
-- user: Core user identity
CREATE TABLE user (
  id            TEXT PRIMARY KEY,
  email         TEXT UNIQUE NOT NULL,
  emailVerified INTEGER NOT NULL DEFAULT 0,
  name          TEXT NOT NULL DEFAULT '',
  image         TEXT,
  createdAt     INTEGER NOT NULL DEFAULT (unixepoch()),
  updatedAt     INTEGER NOT NULL DEFAULT (unixepoch())
);

-- account: Linked auth providers (credential, Google, GitHub, Microsoft, ...)
CREATE TABLE account (
  id                    TEXT PRIMARY KEY,
  userId                TEXT NOT NULL REFERENCES user(id) ON DELETE CASCADE,
  providerId            TEXT NOT NULL,
  accountId             TEXT NOT NULL,
  password              TEXT,             -- bcrypt hash (credential only)
  accessToken           TEXT,             -- OAuth access token
  refreshToken          TEXT,             -- OAuth refresh token (provider's)
  accessTokenExpiresAt  INTEGER,
  scope                 TEXT,
  idToken               TEXT,
  createdAt             INTEGER NOT NULL DEFAULT (unixepoch()),
  updatedAt             INTEGER NOT NULL DEFAULT (unixepoch())
);

-- session: BetterAuth's internal sessions
CREATE TABLE session (
  id        TEXT PRIMARY KEY,
  userId    TEXT NOT NULL REFERENCES user(id) ON DELETE CASCADE,
  token     TEXT UNIQUE NOT NULL,
  expiresAt INTEGER NOT NULL,
  ipAddress TEXT,
  userAgent TEXT,
  createdAt INTEGER NOT NULL DEFAULT (unixepoch()),
  updatedAt INTEGER NOT NULL DEFAULT (unixepoch())
);

-- verification: Email verification / password reset tokens
CREATE TABLE verification (
  id         TEXT PRIMARY KEY,
  identifier TEXT NOT NULL,
  value      TEXT NOT NULL,
  expiresAt  INTEGER NOT NULL,
  createdAt  INTEGER NOT NULL DEFAULT (unixepoch()),
  updatedAt  INTEGER NOT NULL DEFAULT (unixepoch())
);
```

**Custom table** (managed by our code, not BetterAuth):

```sql
-- refresh_token: Our JWT refresh token rotation
-- Separate from BetterAuth's session table
CREATE TABLE refresh_token (
  id        TEXT PRIMARY KEY,
  userId    TEXT NOT NULL REFERENCES user(id) ON DELETE CASCADE,
  token     TEXT UNIQUE NOT NULL,
  expiresAt INTEGER NOT NULL,
  ipAddress TEXT,
  userAgent TEXT,
  createdAt INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE INDEX idx_refresh_token_token   ON refresh_token(token);
CREATE INDEX idx_refresh_token_userId  ON refresh_token(userId);
```

Note: BetterAuth plugins add their own tables automatically (e.g., `twoFactor()` adds `twoFactor` table, `organization()` adds `organization`, `member`, `invitation` tables). No manual schema work needed.

### Why Two Token Systems?

BetterAuth has its own session system (`session` table + cookies). We don't expose it to clients because:

1. **The Go main server needs stateless JWT validation.** It validates tokens locally using `JWT_SECRET` — no network call. BetterAuth sessions require a DB lookup.
2. **Stateless access tokens.** Our JWTs contain `{sub, email, name, iat, exp}` — the Go main server extracts user identity from the payload directly.
3. **Refresh token rotation.** Single-use refresh tokens with rotation for security.

BetterAuth sessions are still created internally (it requires them for `signInEmail`/`signUpEmail`) but are an implementation detail. The externally-visible token system is our JWT + refresh token pair.

---

## BetterAuth Configuration

```typescript
import { betterAuth } from "better-auth";
import { drizzleAdapter } from "better-auth/adapters/drizzle";
import { drizzle } from "drizzle-orm/libsql";
import { createClient } from "@libsql/client";

// --- Database ---
const client = createClient({
  url: process.env.DATABASE_URL || "file:auth.db",
  authToken: process.env.DATABASE_AUTH_TOKEN,
});
const db = drizzle(client);

// --- BetterAuth instance ---
export const auth = betterAuth({
  database: drizzleAdapter(db, { provider: "sqlite" }),
  secret: process.env.AUTH_SECRET,

  emailAndPassword: {
    enabled: true,
    requireEmailVerification: false, // Phase 1: skip email verify
  },

  socialProviders: {
    ...(process.env.GOOGLE_CLIENT_ID && {
      google: {
        clientId: process.env.GOOGLE_CLIENT_ID,
        clientSecret: process.env.GOOGLE_CLIENT_SECRET!,
      },
    }),
    ...(process.env.GITHUB_CLIENT_ID && {
      github: {
        clientId: process.env.GITHUB_CLIENT_ID,
        clientSecret: process.env.GITHUB_CLIENT_SECRET!,
      },
    }),
    ...(process.env.MICROSOFT_CLIENT_ID && {
      microsoft: {
        clientId: process.env.MICROSOFT_CLIENT_ID,
        clientSecret: process.env.MICROSOFT_CLIENT_SECRET!,
      },
    }),
  },

  // Future plugins — uncomment when ready:
  // plugins: [
  //   twoFactor(),    // Phase 2: TOTP + backup codes
  //   passkey(),      // Phase 2: WebAuthn
  //   organization(), // Phase 3: teams + multi-tenancy
  //   sso(),          // Phase 3: enterprise SAML/OIDC
  // ],
});
```

---

## Request Flows

### SignUp (email/password)

```
Client -> Go:SignUp(email, password, name)
  Go -> Node:CreateUserWithPassword(email, password, name)
    Node -> auth.api.signUpEmail({ body: { email, password, name } })
      BetterAuth: bcrypt hash, INSERT user, INSERT account(credential), CREATE session
    Node <- BetterAuth returns { user, session }
  Go <- Node returns { user, account }
  Go -> Node:CreateTokens(userId, email, name)
    Node: sign JWT with jose, INSERT refresh_token
  Go <- Node returns { accessToken, refreshToken }
Go <- returns { user, accessToken, refreshToken }
```

### SignIn (email/password)

```
Client -> Go:SignIn(email, password)
  Go -> Node:VerifyCredentials(email, password)
    Node -> auth.api.signInEmail({ body: { email, password } })
      BetterAuth: lookup user, verify bcrypt hash, CREATE session
    Node <- BetterAuth returns { user, session } or throws error
  Go <- Node returns { valid, user, account }
  if valid:
    Go -> Node:CreateTokens(userId, email, name)
    Go <- returns { user, accessToken, refreshToken }
  else:
    Go <- returns CodeUnauthenticated
```

### OAuth (e.g., Google Sign-In)

```
Client -> Go:GetOAuthUrl(provider=GOOGLE, callbackUrl)
  Go -> Node:GetOAuthUrl(provider, callbackUrl)
    Node: read BetterAuth social provider config, build authorization URL
  Go <- returns { url, state }

Client redirects to Google, user authorizes, Google redirects back with code

Client -> Go:HandleOAuthCallback(provider=GOOGLE, code, state)
  Go -> Node:ExchangeOAuthCode(provider, code, state)
    Node -> auth.api.signInSocial({ ... })
      BetterAuth: exchange code with Google, get user profile,
                  find-or-create user, link account, CREATE session
    Node <- BetterAuth returns { user, session, isNewUser }
    Node -> CreateTokens(userId, email, name)  -- our custom JWT
  Go <- returns { user, accessToken, refreshToken, isNewUser }
```

### RefreshToken

```
Client -> Go:RefreshToken(refreshToken)
  Go -> Node:RefreshTokens(refreshToken)
    Node: SELECT refresh_token JOIN user WHERE token=? AND not expired
    Node: DELETE old token (rotation)
    Node: sign new JWT, INSERT new refresh_token
  Go <- returns { accessToken, refreshToken }
```

---

## Method Implementation Map

### BetterAuth-powered methods

**`CreateUserWithPassword`**
```
1. auth.api.signUpEmail({ body: { email, password, name } })
   → BetterAuth hashes password, creates user + credential account + session
2. Extract user data from BetterAuth response
3. Return { user, account }
```

**`VerifyCredentials`**
```
1. auth.api.signInEmail({ body: { email, password } })
   → BetterAuth verifies bcrypt hash, returns session + user
2. If success: return { valid: true, user, account }
   If auth error: return { valid: false }
```

**`CreateUser`** (no password — OAuth onboarding)
```
1. auth.api.signUpEmail({ body: { email, password: randomToken, name } })
   → BetterAuth creates user. Password is a random throwaway (user will OAuth).
   OR: Direct SQL INSERT into user table if BetterAuth doesn't support no-password signup.
2. Return { user }
```

**`GetOAuthUrl`**
```
1. Read social provider config from BetterAuth instance
2. Build OAuth authorization URL with client_id, redirect_uri, scope, state
3. Return { url, state }
```

**`ExchangeOAuthCode`**
```
1. auth.api.signInSocial() or BetterAuth's OAuth callback handler
   → Exchanges code with provider, gets user profile, finds-or-creates user + account
2. Generate our custom JWT + refresh token via CreateTokens logic
3. Return { user, accessToken, refreshToken, isNewUser }
```

**`GetAccountsByUserId`**
```
1. auth.api.listAccounts() or direct SELECT from account table
2. Map providerId strings to AuthProvider enum
3. Check if credential account has password → hasPassword
4. Return { accounts[], hasPassword }
```

**`CreateAccount`** (link new provider)
```
1. auth.api.linkSocialAccount() for OAuth providers
   OR: For credential accounts — hash password, INSERT into account table
2. Return { account }
```

### Custom methods (JWT + refresh token)

**`GetUser`**
```
1. SELECT from user table WHERE id = ?
   (or auth.api.getSession() if we have a BetterAuth session)
2. Return { user } or { user: undefined }
```

**`CreateTokens`**
```
1. Sign JWT with jose: { sub: userId, email, name, iat, exp } using JWT_SECRET
2. Generate random refresh token (64 chars, crypto-random)
3. INSERT into refresh_token table
4. Return { accessToken, refreshToken }
```

**`RefreshTokens`**
```
1. SELECT refresh_token JOIN user WHERE token = ? AND expiresAt > now
2. If not found → throw Unauthenticated
3. DELETE old refresh token (rotation — single use)
4. Sign new JWT
5. INSERT new refresh token
6. Return { accessToken, refreshToken, user }
```

**`RevokeRefreshToken`**
```
1. DELETE FROM refresh_token WHERE token = ?
2. Return { success: rowsAffected > 0 }
```

---

## Environment Variables

| Variable | Required | Default | Description |
|---|:---:|---|---|
| `DATABASE_URL` | No | `file:auth.db` | Auth DB URL. `:memory:`, `file:path`, or `libsql://host` |
| `DATABASE_AUTH_TOKEN` | For Turso | — | Turso authentication token |
| `JWT_SECRET` | **Yes** | — | Shared secret for JWT signing/validation (both Go servers) |
| `AUTH_SECRET` | **Yes** | — | BetterAuth's internal encryption secret |
| `BETTERAUTH_PORT` | No | `50051` | Internal ConnectRPC server port |
| `BETTERAUTH_URL` | No | `http://localhost:50051` | Go auth-service connects here |
| `AUTH_SERVICE_ADDR` | No | `:8081` | Go auth-service listen address |
| `GOOGLE_CLIENT_ID` | No | — | Google OAuth client ID |
| `GOOGLE_CLIENT_SECRET` | No | — | Google OAuth client secret |
| `GITHUB_CLIENT_ID` | No | — | GitHub OAuth client ID |
| `GITHUB_CLIENT_SECRET` | No | — | GitHub OAuth client secret |
| `MICROSOFT_CLIENT_ID` | No | — | Microsoft OAuth client ID |
| `MICROSOFT_CLIENT_SECRET` | No | — | Microsoft OAuth client secret |

---

## Dependencies

### Node.js BetterAuth Service (`packages/auth/betterauth`)

| Package | Purpose |
|---|---|
| `better-auth` | Auth engine (passwords, OAuth, users, sessions, plugins) |
| `better-auth/adapters/drizzle` | Database adapter for BetterAuth |
| `drizzle-orm` | ORM layer (used only as BetterAuth's adapter) |
| `@libsql/client` | SQLite/LibSQL/Turso driver (all modes) |
| `@connectrpc/connect` + `connect-node` | Internal RPC server |
| `@the-dev-tools/spec` | Generated protobuf types |
| `jose` | JWT signing (HS256) for our custom tokens |

### Go Auth-Service (`packages/auth/auth-service`)

| Package | Purpose |
|---|---|
| `connectrpc.com/connect` | RPC framework |
| `github.com/golang-jwt/jwt/v5` | JWT validation |
| Generated ConnectRPC client | Talks to BetterAuth Node.js service |

---

## Testing Strategy

### Node.js Tests

Use `:memory:` SQLite via `@libsql/client` + Drizzle for isolated, fast tests.

```typescript
// test-utils.ts
import { createClient } from "@libsql/client";
import { drizzle } from "drizzle-orm/libsql";
import { betterAuth } from "better-auth";
import { drizzleAdapter } from "better-auth/adapters/drizzle";

export async function createTestAuth() {
  const client = createClient({ url: ":memory:" });
  const db = drizzle(client);

  const auth = betterAuth({
    database: drizzleAdapter(db, { provider: "sqlite" }),
    secret: "test-secret",
    emailAndPassword: { enabled: true },
    // Enable social providers with test credentials for OAuth tests
  });

  // Create service with auth instance + raw client for refresh_token queries
  const service = createInternalAuthService({ auth, rawDb: client, jwtSecret, config });
  return { auth, client, service };
}
```

Each test gets its own in-memory database (mirrors the Go `sqlitemem.NewSQLiteMem` pattern).

**Test categories:**
- **BetterAuth operations:** signUpEmail/signInEmail round-trip through the library
- **Custom token operations:** JWT creation, refresh rotation, revocation
- **Account operations:** getAccountsByUserId, createAccount
- **Integration flows:** full sign-up → sign-in → tokens → refresh → revoke

### Go Tests

Existing Go tests use mock `AuthInternalServiceClient` — they test the Go handler logic in isolation without a real BetterAuth backend. These remain unchanged.

---

## Deployment Modes

### Local / Desktop

```env
DATABASE_URL=file:auth.db
AUTH_SECRET=local-dev-secret
JWT_SECRET=local-dev-jwt-secret
```

Both Go servers and Node.js BetterAuth run as local processes. SQLite file on disk.

### SaaS / Production

```env
DATABASE_URL=libsql://auth-db-your-org.turso.io
DATABASE_AUTH_TOKEN=eyJ...
AUTH_SECRET=<random-32-bytes>
JWT_SECRET=<random-32-bytes>
GOOGLE_CLIENT_ID=...
GOOGLE_CLIENT_SECRET=...
GITHUB_CLIENT_ID=...
GITHUB_CLIENT_SECRET=...
```

Auth database on Turso. Go servers and BetterAuth deployed as separate containers. Internal network between Go auth-service and BetterAuth Node.js.

### Testing / CI

```env
DATABASE_URL=:memory:
AUTH_SECRET=test-secret
JWT_SECRET=test-jwt-secret
```

In-memory SQLite. Each test creates a fresh database. No persistence.

---

## TypeSpec Contracts

### Public API (`packages/spec/api/auth.tsp`)

Exposed to clients. Served by Go auth-service on `:8081`.

```
AuthService:
  SignUp(email, password, name)                → { user, accessToken, refreshToken }
  SignIn(email, password)                      → { user, accessToken, refreshToken }
  SignOut(refreshToken)                        → { success }
  GetOAuthUrl(provider, callbackUrl, state?)   → { url, state }
  HandleOAuthCallback(provider, code, state?)  → { user, accessToken, refreshToken, isNewUser }
  RefreshToken(refreshToken)                   → { accessToken, refreshToken }
  GetMe()                                      → { user }
  GetLinkedAccounts()                          → { accounts[], hasPassword }
```

### Internal API (`packages/spec/api/auth-internal.tsp`)

Internal only. Served by Node.js BetterAuth on `:50051`. Called only by Go auth-service.

```
AuthInternalService:
  CreateUser(email, name, image?)                                        → { user }
  CreateUserWithPassword(email, name, password)                          → { user, account }
  VerifyCredentials(email, password)                                     → { valid, user?, account? }
  GetUser(userId)                                                        → { user? }
  CreateTokens(userId, email, name, ipAddress?, userAgent?)              → { accessToken, refreshToken }
  RefreshTokens(refreshToken)                                            → { accessToken, refreshToken, user }
  RevokeRefreshToken(refreshToken)                                       → { success }
  GetOAuthUrl(provider, callbackUrl, state?)                             → { url, state }
  ExchangeOAuthCode(provider, code, state?, ipAddress?, userAgent?)      → { user, accessToken, refreshToken, isNewUser }
  GetAccountsByUserId(userId)                                            → { accounts[], hasPassword }
  CreateAccount(userId, provider, providerAccountId, password?, ...)     → { account }
```

---

## File Structure

```
packages/
  auth/
    AUTH_ARCHITECTURE.md            ← This file

    auth-service/                   ← Go auth-service (public RPC gateway)
      cmd/auth-service/main.go      ← Entry point
      pkg/
        client/betterauth.go        ← ConnectRPC client to BetterAuth
        handler/
          auth.go                   ← AuthService handlers + JWT interceptor
          auth_test.go              ← Tests with mock BetterAuth client

    betterauth/                     ← Node.js BetterAuth service (internal)
      src/
        index.ts                    ← Entry point, HTTP server, env config
        auth.ts                     ← BetterAuth instance + Drizzle adapter
        service.ts                  ← AuthInternalService (ConnectRPC ↔ auth.api.*)
        refresh-token.ts            ← Custom refresh token queries (only custom table)
        test-utils.ts               ← Test helper (in-memory DB + BetterAuth instance)
        service.test.ts             ← Service tests
      vitest.config.ts
      package.json
      project.json
      tsconfig.json

  spec/
    api/
      auth.tsp                      ← Public auth API (TypeSpec)
      auth-internal.tsp             ← Internal auth API (TypeSpec)
```

---

## Security Considerations

1. **JWT_SECRET must be strong.** Minimum 32 bytes of randomness. Shared between Go main server, Go auth-service, and Node.js BetterAuth.
2. **AUTH_SECRET is BetterAuth-internal.** Used for encrypting BetterAuth sessions/tokens. Does not need to be shared with Go.
3. **Internal service not publicly accessible.** BetterAuth on `:50051` should only be reachable from the Go auth-service (internal network / localhost).
4. **Refresh token rotation.** Each refresh token is single-use. Using a token deletes it and creates a new one.
5. **Generic error messages.** The Go auth-service returns "invalid credentials" for both wrong-email and wrong-password (no user enumeration).
6. **Password hashing.** Handled entirely by BetterAuth (bcrypt, configurable rounds). We never see or store plaintext passwords.
7. **OAuth state parameter.** CSRF protection on OAuth flows. State is generated server-side and validated on callback.
8. **Plugin security.** BetterAuth plugins (2FA, passkeys, SSO) follow security best practices out of the box — TOTP with backup codes, WebAuthn challenge-response, SAML signature validation.
