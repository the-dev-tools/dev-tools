import type { Client as RpcClient } from '@connectrpc/connect';
import type { Client as DbClient } from '@libsql/client';
import type { AddressInfo } from 'node:net';
import { Code, ConnectError, createClient } from '@connectrpc/connect';
import { connectNodeAdapter, createConnectTransport } from '@connectrpc/connect-node';
import id128 from 'id128';

const { Ulid } = id128;
import { createServer, type Server } from 'node:http';
import { afterAll, beforeAll, describe, expect, it } from 'vitest';

import { AuthProvider } from '@the-dev-tools/spec/buf/api/auth_common/v1/auth_common_pb';
import { AuthInternalService } from '@the-dev-tools/spec/buf/api/auth_internal/v1/auth_internal_pb';

import { createTestService } from './test-utils.js';

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
    server.listen(0, '127.0.0.1', () => {
      resolve();
    });
  });

  const { port } = server.address() as AddressInfo;

  const transport = createConnectTransport({
    baseUrl: `http://127.0.0.1:${port}`,
    httpVersion: '1.1',
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

describe('RPC: createUserWithPassword', () => {
  it('creates user and credential account, persists to SQLite', async () => {
    const res = await client.createUserWithPassword({
      email: 'rpc-alice@test.com',
      name: 'Alice',
      password: 'secret123',
    });

    expect(res.user).toBeDefined();
    expect(res.user!.email).toBe('rpc-alice@test.com');
    expect(res.user!.name).toBe('Alice');
    expect(res.user!.id).toBeTruthy();
    expect(res.sessionToken).toBeTruthy();

    // Convert binary ULID to string for SQLite queries
    const userIdStr = Ulid.construct(res.user!.id).toCanonical();

    // Verify directly in SQLite
    const userRows = await rawDb.execute({
      args: [userIdStr],
      sql: 'SELECT * FROM user WHERE id = ?',
    });
    expect(userRows.rows).toHaveLength(1);
    expect(userRows.rows[0].email).toBe('rpc-alice@test.com');

    const accountRows = await rawDb.execute({
      args: [userIdStr, 'credential'],
      sql: 'SELECT * FROM account WHERE userId = ? AND providerId = ?',
    });
    expect(accountRows.rows).toHaveLength(1);
    expect(accountRows.rows[0].password).not.toBe('secret123');
    expect((accountRows.rows[0].password as string).length).toBeGreaterThan(10);
  });

  it('returns AlreadyExists over the wire', async () => {
    await client.createUserWithPassword({
      email: 'rpc-dup@test.com',
      name: 'First',
      password: 'password-first',
    });

    try {
      await client.createUserWithPassword({
        email: 'rpc-dup@test.com',
        name: 'Second',
        password: 'password-second',
      });
      expect.fail('should have thrown');
    } catch (e) {
      expect(e).toBeInstanceOf(ConnectError);
      expect((e as ConnectError).code).toBe(Code.AlreadyExists);
    }
  });
});

describe('RPC: full auth flow', () => {
  it('signup → verify → getToken', async () => {
    // 1. Sign up with password
    const signup = await client.createUserWithPassword({
      email: 'rpc-flow@test.com',
      name: 'Flow',
      password: 'mypassword',
    });
    expect(signup.user!.email).toBe('rpc-flow@test.com');
    expect(signup.sessionToken).toBeTruthy();

    const signupUserIdStr = Ulid.construct(signup.user!.id).toCanonical();

    // Verify user + credential account exist in SQLite
    const userRows = await rawDb.execute({
      args: [signupUserIdStr],
      sql: 'SELECT * FROM user WHERE id = ?',
    });
    expect(userRows.rows).toHaveLength(1);

    const accountRows = await rawDb.execute({
      args: [signupUserIdStr, 'credential'],
      sql: 'SELECT * FROM account WHERE userId = ? AND providerId = ?',
    });
    expect(accountRows.rows).toHaveLength(1);
    expect(accountRows.rows[0].password).not.toBe('mypassword');

    // 2. Verify credentials
    const verify = await client.verifyCredentials({
      email: 'rpc-flow@test.com',
      password: 'mypassword',
    });
    expect(verify.valid).toBe(true);
    // Compare binary ULIDs via canonical string
    expect(Ulid.construct(verify.user!.id).toCanonical()).toBe(signupUserIdStr);
    expect(verify.sessionToken).toBeTruthy();

    // 3. Get JWT access token using session token
    const token = await client.token({
      sessionToken: verify.sessionToken,
    });
    expect(token.accessToken).toBeTruthy();

    // Verify it's a valid JWT
    const parts = token.accessToken.split('.');
    expect(parts.length).toBe(3);

    // Decode and check claims — JWT sub is string ULID
    const payload = JSON.parse(atob(parts[1])) as { email: string; sub: string };
    expect(payload.sub).toBe(signupUserIdStr);
    expect(payload.email).toBe('rpc-flow@test.com');

    // Verify session exists in SQLite (BetterAuth internal)
    const sessionRows = await rawDb.execute({
      args: [signupUserIdStr],
      sql: 'SELECT * FROM session WHERE userId = ?',
    });
    expect(sessionRows.rows.length).toBeGreaterThanOrEqual(1);
  });
});

describe('RPC: accounts', () => {
  it('creates OAuth account and persists to SQLite', async () => {
    const user = await client.createUserWithPassword({
      email: 'rpc-accounts@test.com',
      name: 'Accounts',
      password: 'password-accounts',
    });

    await client.createAccount({
      provider: AuthProvider.GOOGLE,
      providerAccountId: 'google-456',
      userId: user.user!.id,
    });

    // Verify via RPC
    const res = await client.accountsByUserId({
      userId: user.user!.id,
    });

    const googleAccount = res.accounts.find((a) => a.provider === AuthProvider.GOOGLE);
    expect(googleAccount).toBeDefined();
    expect(googleAccount!.providerAccountId).toBe('google-456');

    // Verify directly in SQLite
    const userIdStr = Ulid.construct(user.user!.id).toCanonical();
    const rows = await rawDb.execute({
      args: [userIdStr, 'google'],
      sql: 'SELECT * FROM account WHERE userId = ? AND providerId = ?',
    });
    expect(rows.rows).toHaveLength(1);
    expect(rows.rows[0].accountId).toBe('google-456');
    expect(rows.rows[0].password).toBeNull();
  });
});

describe('RPC: OAuth', () => {
  it('returns OAuth URL for Google', async () => {
    const res = await client.oAuthUrl({
      callbackUrl: 'https://app.test/callback',
      provider: AuthProvider.GOOGLE,
    });

    expect(res.url).toContain('accounts.google.com');
    expect(res.state).toBeTruthy();
  });

  it('rejects unsupported provider', async () => {
    try {
      await client.oAuthUrl({
        callbackUrl: 'https://app.test/callback',
        provider: AuthProvider.EMAIL,
      });
      expect.fail('should have thrown');
    } catch (e) {
      expect(e).toBeInstanceOf(ConnectError);
      expect((e as ConnectError).code).toBe(Code.InvalidArgument);
    }
  });
});

describe('RPC: token', () => {
  it('rejects invalid session token', async () => {
    try {
      await client.token({
        sessionToken: 'invalid-token',
      });
      expect.fail('should have thrown');
    } catch (e) {
      expect(e).toBeInstanceOf(ConnectError);
      expect((e as ConnectError).code).toBe(Code.Unauthenticated);
    }
  });
});
