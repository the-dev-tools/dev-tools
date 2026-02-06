import type { ServiceImpl } from '@connectrpc/connect';
import { Code, ConnectError } from '@connectrpc/connect';
import { beforeEach, describe, expect, it } from 'vitest';

import { AuthInternalService, AuthProvider } from '@the-dev-tools/spec/buf/api/auth_internal/v1/auth_internal_pb';

import { createTestService } from './test-utils.js';

type Service = ServiceImpl<typeof AuthInternalService>;

let service: Service;

beforeEach(async () => {
  const t = await createTestService();
  service = t.service;
});

// =============================================================================
// createUserWithPassword
// =============================================================================

describe('createUserWithPassword', () => {
  it('creates user and credential account', async () => {
    const res = await service.createUserWithPassword(
      { email: 'alice@test.com', name: 'Alice', password: 'secret123' } as never,
      {} as never,
    );

    expect(res.user).toBeDefined();
    expect(res.account).toBeDefined();
    expect(res.user!.email).toBe('alice@test.com');
    expect(res.account!.provider).toBe(AuthProvider.EMAIL);
    expect(res.account!.providerAccountId).toBe(res.user!.id);
  });

  it('returns a session token', async () => {
    const res = await service.createUserWithPassword(
      { email: 'alice@test.com', name: 'Alice', password: 'secret123' } as never,
      {} as never,
    );

    expect(res.sessionToken).toBeTruthy();
  });

  it('stores hashed password, not plaintext', async () => {
    const res = await service.createUserWithPassword(
      { email: 'alice@test.com', name: 'Alice', password: 'secret123' } as never,
      {} as never,
    );

    expect(res.account!.passwordHash).toBeDefined();
    expect(res.account!.passwordHash).not.toBe('secret123');
  });

  it('password is verifiable via round-trip', async () => {
    await service.createUserWithPassword(
      { email: 'alice@test.com', name: 'Alice', password: 'secret123' } as never,
      {} as never,
    );

    const verify = await service.verifyCredentials(
      { email: 'alice@test.com', password: 'secret123' } as never,
      {} as never,
    );
    expect(verify.valid).toBe(true);
  });

  it('returns AlreadyExists for duplicate email', async () => {
    await service.createUserWithPassword(
      { email: 'alice@test.com', name: 'Alice', password: 'secret123' } as never,
      {} as never,
    );

    try {
      await service.createUserWithPassword(
        {
          email: 'alice@test.com',
          name: 'Alice2',
          password: 'other-password',
        } as never,
        {} as never,
      );
      expect.fail('should have thrown');
    } catch (e) {
      expect(e).toBeInstanceOf(ConnectError);
      expect((e as ConnectError).code).toBe(Code.AlreadyExists);
    }
  });
});

// =============================================================================
// verifyCredentials
// =============================================================================

describe('verifyCredentials', () => {
  beforeEach(async () => {
    await service.createUserWithPassword(
      { email: 'alice@test.com', name: 'Alice', password: 'correct-password' } as never,
      {} as never,
    );
  });

  it('returns valid=true with user, account, and session token for correct password', async () => {
    const res = await service.verifyCredentials(
      { email: 'alice@test.com', password: 'correct-password' } as never,
      {} as never,
    );

    expect(res.valid).toBe(true);
    expect(res.user).toBeDefined();
    expect(res.user!.email).toBe('alice@test.com');
    expect(res.account).toBeDefined();
    expect(res.account!.provider).toBe(AuthProvider.EMAIL);
    expect(res.sessionToken).toBeTruthy();
  });

  it('returns valid=false for wrong password', async () => {
    const res = await service.verifyCredentials(
      { email: 'alice@test.com', password: 'wrong-password' } as never,
      {} as never,
    );

    expect(res.valid).toBe(false);
    expect(res.user).toBeUndefined();
  });

  it('returns valid=false for nonexistent user', async () => {
    const res = await service.verifyCredentials(
      { email: 'nobody@test.com', password: 'any-password' } as never,
      {} as never,
    );

    expect(res.valid).toBe(false);
  });
});

// =============================================================================
// getToken
// =============================================================================

describe('getToken', () => {
  it('returns a JWT access token for valid session', async () => {
    const signup = await service.createUserWithPassword(
      { email: 'alice@test.com', name: 'Alice', password: 'secret123' } as never,
      {} as never,
    );

    const res = await service.getToken({ sessionToken: signup.sessionToken } as never, {} as never);

    expect(res.accessToken).toBeTruthy();
    // JWT format: header.payload.signature
    const parts = res.accessToken!.split('.');
    expect(parts.length).toBe(3);
  });

  it('throws Unauthenticated for invalid session token', async () => {
    try {
      await service.getToken({ sessionToken: 'bogus-session-token' } as never, {} as never);
      expect.fail('should have thrown');
    } catch (e) {
      expect(e).toBeInstanceOf(ConnectError);
      expect((e as ConnectError).code).toBe(Code.Unauthenticated);
    }
  });
});

// =============================================================================
// getOAuthUrl
// =============================================================================

describe('getOAuthUrl', () => {
  it('returns Google OAuth URL', async () => {
    const res = await service.getOAuthUrl(
      {
        callbackUrl: 'https://app.test/callback',
        provider: AuthProvider.GOOGLE,
      } as never,
      {} as never,
    );

    expect(res.url).toContain('accounts.google.com');
    expect(res.url).toContain('test-google-id');
    expect(res.state).toBeTruthy();
  });

  it('returns GitHub OAuth URL', async () => {
    const res = await service.getOAuthUrl(
      {
        callbackUrl: 'https://app.test/callback',
        provider: AuthProvider.GIT_HUB,
      } as never,
      {} as never,
    );

    expect(res.url).toContain('github.com');
    expect(res.url).toContain('test-github-id');
  });

  it('generates state automatically via BetterAuth', async () => {
    const res = await service.getOAuthUrl(
      {
        callbackUrl: 'https://app.test/callback',
        provider: AuthProvider.GOOGLE,
      } as never,
      {} as never,
    );

    expect(res.state).toBeTruthy();
    expect(res.state!.length).toBeGreaterThan(0);
  });

  it('throws for unconfigured provider', async () => {
    const t = await createTestService({
      authConfig: { oauth: undefined },
    });

    await expect(
      t.service.getOAuthUrl(
        {
          callbackUrl: 'https://app.test/callback',
          provider: AuthProvider.GOOGLE,
        } as never,
        {} as never,
      ),
    ).rejects.toThrow(ConnectError);
  });
});

// =============================================================================
// exchangeOAuthCode
// =============================================================================

describe('exchangeOAuthCode', () => {
  it('throws InvalidArgument for credential provider', async () => {
    try {
      await service.exchangeOAuthCode(
        {
          code: 'auth-code',
          provider: AuthProvider.EMAIL,
          state: 'some-state',
        } as never,
        {} as never,
      );
      expect.fail('should have thrown');
    } catch (e) {
      expect(e).toBeInstanceOf(ConnectError);
      expect((e as ConnectError).code).toBe(Code.InvalidArgument);
    }
  });

  it('throws Unauthenticated for invalid state/code', async () => {
    try {
      await service.exchangeOAuthCode(
        {
          code: 'bogus-code',
          provider: AuthProvider.GOOGLE,
          state: 'bogus-state',
        } as never,
        {} as never,
      );
      expect.fail('should have thrown');
    } catch (e) {
      expect(e).toBeInstanceOf(ConnectError);
      expect((e as ConnectError).code).toBe(Code.Unauthenticated);
    }
  });
});

// =============================================================================
// getAccountsByUserId
// =============================================================================

describe('getAccountsByUserId', () => {
  it('returns credential account with hasPassword=true', async () => {
    const created = await service.createUserWithPassword(
      { email: 'alice@test.com', name: 'Alice', password: 'password123' } as never,
      {} as never,
    );

    const res = await service.getAccountsByUserId({ userId: created.user!.id } as never, {} as never);

    expect(res.accounts!.length).toBeGreaterThanOrEqual(1);
    expect(res.hasPassword).toBe(true);

    const credentialAccount = res.accounts!.find((a) => a.provider === AuthProvider.EMAIL);
    expect(credentialAccount).toBeDefined();
  });

  it('returns multiple accounts', async () => {
    const created = await service.createUserWithPassword(
      { email: 'alice@test.com', name: 'Alice', password: 'password123' } as never,
      {} as never,
    );

    await service.createAccount(
      {
        provider: AuthProvider.GOOGLE,
        providerAccountId: 'google-123',
        userId: created.user!.id,
      } as never,
      {} as never,
    );

    const res = await service.getAccountsByUserId({ userId: created.user!.id } as never, {} as never);

    expect(res.accounts!.length).toBeGreaterThanOrEqual(2);
    expect(res.hasPassword).toBe(true);
  });
});

// =============================================================================
// createAccount
// =============================================================================

describe('createAccount', () => {
  it('creates credential account with password', async () => {
    const created = await service.createUserWithPassword(
      { email: 'alice@test.com', name: 'Alice', password: 'password123' } as never,
      {} as never,
    );

    const res = await service.createAccount(
      {
        password: 'secretpass',
        provider: AuthProvider.EMAIL,
        providerAccountId: 'alice@test.com',
        userId: created.user!.id,
      } as never,
      {} as never,
    );

    expect(res.account).toBeDefined();
    expect(res.account!.provider).toBe(AuthProvider.EMAIL);
    expect(res.account!.passwordHash).toBeDefined();
    expect(res.account!.passwordHash).not.toBe('secretpass');
    expect(res.account!.passwordHash!.length).toBeGreaterThan(10);
  });

  it('creates OAuth account without password', async () => {
    const created = await service.createUserWithPassword(
      { email: 'alice@test.com', name: 'Alice', password: 'password123' } as never,
      {} as never,
    );

    const res = await service.createAccount(
      {
        accessToken: 'gtoken',
        provider: AuthProvider.GOOGLE,
        providerAccountId: 'google-123',
        userId: created.user!.id,
      } as never,
      {} as never,
    );

    expect(res.account).toBeDefined();
    expect(res.account!.provider).toBe(AuthProvider.GOOGLE);
    expect(res.account!.passwordHash).toBeUndefined();
    expect(res.account!.accessToken).toBe('gtoken');
  });
});

// =============================================================================
// Integration: full auth flow
// =============================================================================

describe('integration: full auth flow', () => {
  it('sign up → verify → get token', async () => {
    // 1. Sign up
    const signup = await service.createUserWithPassword(
      { email: 'alice@test.com', name: 'Alice', password: 'mypassword' } as never,
      {} as never,
    );
    expect(signup.user!.email).toBe('alice@test.com');
    expect(signup.sessionToken).toBeTruthy();

    // 2. Verify credentials
    const verify = await service.verifyCredentials(
      { email: 'alice@test.com', password: 'mypassword' } as never,
      {} as never,
    );
    expect(verify.valid).toBe(true);
    expect(verify.sessionToken).toBeTruthy();

    // 3. Get JWT access token using session token
    const token = await service.getToken({ sessionToken: verify.sessionToken } as never, {} as never);
    expect(token.accessToken).toBeTruthy();

    // Verify it's a valid JWT (3 parts separated by dots)
    const parts = token.accessToken!.split('.');
    expect(parts.length).toBe(3);

    // Decode payload and verify claims
    const payload = JSON.parse(atob(parts[1]));
    expect(payload.sub).toBe(signup.user!.id);
    expect(payload.email).toBe('alice@test.com');
    expect(payload.name).toBe('Alice');
  });
});
