import type { Client as RpcClient } from "@connectrpc/connect";
import { Code, ConnectError, createClient } from "@connectrpc/connect";
import {
  connectNodeAdapter,
  createConnectTransport,
} from "@connectrpc/connect-node";
import type { Client as DbClient } from "@libsql/client";
import { type Server, createServer } from "node:http";
import type { AddressInfo } from "node:net";
import { afterAll, beforeAll, describe, expect, it } from "vitest";

import {
  AuthInternalService,
  AuthProvider,
} from "@the-dev-tools/spec/buf/api/auth_internal/v1/auth_internal_pb";

import { createTestService } from "./test-utils.js";

// =============================================================================
// Server + Client setup
// =============================================================================

let server: Server;
let client: RpcClient<typeof AuthInternalService>;
let rawDb: DbClient;

beforeAll(async () => {
  const testService = await createTestService();
  rawDb = testService.rawDb;
  const { service } = testService;

  const handler = connectNodeAdapter({
    routes: (router) => {
      router.service(AuthInternalService, service);
    },
  });

  server = createServer(handler);

  await new Promise<void>((resolve) => {
    server.listen(0, "127.0.0.1", () => {
      resolve();
    });
  });

  const { port } = server.address() as AddressInfo;

  const transport = createConnectTransport({
    baseUrl: `http://127.0.0.1:${port}`,
    httpVersion: "1.1",
  });

  client = createClient(AuthInternalService, transport);
});

afterAll(async () => {
  await new Promise<void>((resolve) => {
    server.close(() => {
      resolve();
    });
  });
});

// =============================================================================
// RPC Integration Tests
// =============================================================================

describe("RPC: createUser", () => {
  it("creates a user via HTTP and persists to SQLite", async () => {
    const res = await client.createUser({
      email: "rpc-alice@test.com",
      name: "Alice",
    });

    expect(res.user).toBeDefined();
    expect(res.user!.email).toBe("rpc-alice@test.com");
    expect(res.user!.name).toBe("Alice");
    expect(res.user!.id).toBeTruthy();

    // Verify directly in SQLite
    const rows = await rawDb.execute({
      args: [res.user!.id],
      sql: "SELECT * FROM user WHERE id = ?",
    });
    expect(rows.rows).toHaveLength(1);
    expect(rows.rows[0].email).toBe("rpc-alice@test.com");
    expect(rows.rows[0].name).toBe("Alice");
  });

  it("returns AlreadyExists over the wire", async () => {
    await client.createUser({
      email: "rpc-dup@test.com",
      name: "First",
    });

    try {
      await client.createUser({
        email: "rpc-dup@test.com",
        name: "Second",
      });
      expect.fail("should have thrown");
    } catch (e) {
      expect(e).toBeInstanceOf(ConnectError);
      expect((e as ConnectError).code).toBe(Code.AlreadyExists);
    }
  });
});

describe("RPC: full auth flow", () => {
  it("signup → verify → tokens → refresh → revoke", async () => {
    // 1. Sign up with password
    const signup = await client.createUserWithPassword({
      email: "rpc-flow@test.com",
      name: "Flow",
      password: "mypassword",
    });
    expect(signup.user!.email).toBe("rpc-flow@test.com");

    // Verify user + credential account exist in SQLite
    const userRows = await rawDb.execute({
      args: [signup.user!.id],
      sql: "SELECT * FROM user WHERE id = ?",
    });
    expect(userRows.rows).toHaveLength(1);

    const accountRows = await rawDb.execute({
      args: [signup.user!.id, "credential"],
      sql: "SELECT * FROM account WHERE userId = ? AND providerId = ?",
    });
    expect(accountRows.rows).toHaveLength(1);
    // Password must be hashed, not plaintext
    expect(accountRows.rows[0].password).not.toBe("mypassword");
    expect((accountRows.rows[0].password as string).length).toBeGreaterThan(10);

    // 2. Verify credentials
    const verify = await client.verifyCredentials({
      email: "rpc-flow@test.com",
      password: "mypassword",
    });
    expect(verify.valid).toBe(true);
    expect(verify.user!.id).toBe(signup.user!.id);

    // 3. Create tokens
    const tokens = await client.createTokens({
      email: "rpc-flow@test.com",
      name: "Flow",
      userId: signup.user!.id,
    });
    expect(tokens.accessToken).toBeTruthy();
    expect(tokens.refreshToken).toBeTruthy();

    // Verify refresh token row exists in SQLite
    const rtRows = await rawDb.execute({
      args: [signup.user!.id],
      sql: "SELECT * FROM refresh_token WHERE userId = ?",
    });
    expect(rtRows.rows).toHaveLength(1);
    expect(rtRows.rows[0].token).toBe(tokens.refreshToken);

    // 4. Refresh tokens — old token deleted, new one created
    const refreshed = await client.refreshTokens({
      refreshToken: tokens.refreshToken,
    });
    expect(refreshed.accessToken).toBeTruthy();
    expect(refreshed.refreshToken).not.toBe(tokens.refreshToken);

    // Verify old token gone, new token present in SQLite
    const rtAfterRefresh = await rawDb.execute({
      args: [signup.user!.id],
      sql: "SELECT * FROM refresh_token WHERE userId = ?",
    });
    expect(rtAfterRefresh.rows).toHaveLength(1);
    expect(rtAfterRefresh.rows[0].token).toBe(refreshed.refreshToken);

    // 5. Revoke
    const revoked = await client.revokeRefreshToken({
      refreshToken: refreshed.refreshToken,
    });
    expect(revoked.success).toBe(true);

    // Verify refresh token table is empty for this user
    const rtAfterRevoke = await rawDb.execute({
      args: [signup.user!.id],
      sql: "SELECT * FROM refresh_token WHERE userId = ?",
    });
    expect(rtAfterRevoke.rows).toHaveLength(0);

    // 6. Refresh should fail now
    try {
      await client.refreshTokens({
        refreshToken: refreshed.refreshToken,
      });
      expect.fail("should have thrown");
    } catch (e) {
      expect(e).toBeInstanceOf(ConnectError);
      expect((e as ConnectError).code).toBe(Code.Unauthenticated);
    }
  });
});

describe("RPC: getUser", () => {
  it("returns user by ID", async () => {
    const created = await client.createUser({
      email: "rpc-getuser@test.com",
      name: "GetMe",
    });

    const res = await client.getUser({ userId: created.user!.id });
    expect(res.user).toBeDefined();
    expect(res.user!.email).toBe("rpc-getuser@test.com");
  });

  it("returns empty for nonexistent user", async () => {
    const res = await client.getUser({ userId: "does-not-exist" });
    expect(res.user).toBeUndefined();
  });
});

describe("RPC: accounts", () => {
  it("creates OAuth account and persists to SQLite", async () => {
    const user = await client.createUser({
      email: "rpc-accounts@test.com",
      name: "Accounts",
    });

    await client.createAccount({
      provider: AuthProvider.GOOGLE,
      providerAccountId: "google-456",
      userId: user.user!.id,
    });

    // Verify via RPC
    const res = await client.getAccountsByUserId({
      userId: user.user!.id,
    });

    const googleAccount = res.accounts.find(
      (a) => a.provider === AuthProvider.GOOGLE,
    );
    expect(googleAccount).toBeDefined();
    expect(googleAccount!.providerAccountId).toBe("google-456");

    // Verify directly in SQLite
    const rows = await rawDb.execute({
      args: [user.user!.id, "google"],
      sql: "SELECT * FROM account WHERE userId = ? AND providerId = ?",
    });
    expect(rows.rows).toHaveLength(1);
    expect(rows.rows[0].accountId).toBe("google-456");
    expect(rows.rows[0].password).toBeNull();
  });
});

describe("RPC: OAuth", () => {
  it("returns OAuth URL for Google", async () => {
    const res = await client.getOAuthUrl({
      callbackUrl: "https://app.test/callback",
      provider: AuthProvider.GOOGLE,
    });

    expect(res.url).toContain("accounts.google.com");
    expect(res.state).toBeTruthy();
  });

  it("rejects unsupported provider", async () => {
    try {
      await client.getOAuthUrl({
        callbackUrl: "https://app.test/callback",
        provider: AuthProvider.EMAIL,
      });
      expect.fail("should have thrown");
    } catch (e) {
      expect(e).toBeInstanceOf(ConnectError);
      expect((e as ConnectError).code).toBe(Code.InvalidArgument);
    }
  });
});
