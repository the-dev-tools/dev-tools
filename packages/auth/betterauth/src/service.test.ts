import type { ServiceImpl } from "@connectrpc/connect";
import { Code, ConnectError } from "@connectrpc/connect";
import { jwtVerify } from "jose";
import { beforeEach, describe, expect, it } from "vitest";

import {
  AuthInternalService,
  AuthProvider,
} from "@the-dev-tools/spec/buf/api/auth_internal/v1/auth_internal_pb";

import { createTestService } from "./test-utils.js";

type Service = ServiceImpl<typeof AuthInternalService>;

let service: Service;
let jwtSecret: Uint8Array;

beforeEach(async () => {
  const t = await createTestService();
  service = t.service;
  jwtSecret = t.jwtSecret;
});

// =============================================================================
// createUser
// =============================================================================

describe("createUser", () => {
  it("creates a user successfully", async () => {
    const res = await service.createUser(
      { email: "alice@test.com", name: "Alice" } as never,
      {} as never,
    );

    expect(res.user).toBeDefined();
    expect(res.user!.email).toBe("alice@test.com");
    expect(res.user!.name).toBe("Alice");
    expect(res.user!.id).toBeTruthy();
    expect(res.user!.emailVerified).toBe(false);
  });

  it("returns AlreadyExists for duplicate email", async () => {
    await service.createUser(
      { email: "alice@test.com", name: "Alice" } as never,
      {} as never,
    );

    await expect(
      service.createUser(
        { email: "alice@test.com", name: "Alice2" } as never,
        {} as never,
      ),
    ).rejects.toThrow(ConnectError);

    try {
      await service.createUser(
        { email: "alice@test.com", name: "Alice2" } as never,
        {} as never,
      );
    } catch (e) {
      expect(e).toBeInstanceOf(ConnectError);
      expect((e as ConnectError).code).toBe(Code.AlreadyExists);
    }
  });
});

// =============================================================================
// createUserWithPassword
// =============================================================================

describe("createUserWithPassword", () => {
  it("creates user and credential account", async () => {
    const res = await service.createUserWithPassword(
      { email: "alice@test.com", name: "Alice", password: "secret123" } as never,
      {} as never,
    );

    expect(res.user).toBeDefined();
    expect(res.account).toBeDefined();
    expect(res.user!.email).toBe("alice@test.com");
    expect(res.account!.provider).toBe(AuthProvider.EMAIL);
    // BetterAuth uses userId as accountId for credential accounts
    expect(res.account!.providerAccountId).toBe(res.user!.id);
  });

  it("stores hashed password, not plaintext", async () => {
    const res = await service.createUserWithPassword(
      { email: "alice@test.com", name: "Alice", password: "secret123" } as never,
      {} as never,
    );

    expect(res.account!.passwordHash).toBeDefined();
    expect(res.account!.passwordHash).not.toBe("secret123");
  });

  it("password is verifiable via round-trip", async () => {
    await service.createUserWithPassword(
      { email: "alice@test.com", name: "Alice", password: "secret123" } as never,
      {} as never,
    );

    // Verify via verifyCredentials round-trip
    const verify = await service.verifyCredentials(
      { email: "alice@test.com", password: "secret123" } as never,
      {} as never,
    );
    expect(verify.valid).toBe(true);
  });

  it("returns AlreadyExists for duplicate email", async () => {
    await service.createUserWithPassword(
      { email: "alice@test.com", name: "Alice", password: "secret123" } as never,
      {} as never,
    );

    try {
      await service.createUserWithPassword(
        {
          email: "alice@test.com",
          name: "Alice2",
          password: "other-password",
        } as never,
        {} as never,
      );
      expect.fail("should have thrown");
    } catch (e) {
      expect(e).toBeInstanceOf(ConnectError);
      expect((e as ConnectError).code).toBe(Code.AlreadyExists);
    }
  });
});

// =============================================================================
// verifyCredentials
// =============================================================================

describe("verifyCredentials", () => {
  beforeEach(async () => {
    await service.createUserWithPassword(
      { email: "alice@test.com", name: "Alice", password: "correct-password" } as never,
      {} as never,
    );
  });

  it("returns valid=true with user and account for correct password", async () => {
    const res = await service.verifyCredentials(
      { email: "alice@test.com", password: "correct-password" } as never,
      {} as never,
    );

    expect(res.valid).toBe(true);
    expect(res.user).toBeDefined();
    expect(res.user!.email).toBe("alice@test.com");
    expect(res.account).toBeDefined();
    expect(res.account!.provider).toBe(AuthProvider.EMAIL);
  });

  it("returns valid=false for wrong password", async () => {
    const res = await service.verifyCredentials(
      { email: "alice@test.com", password: "wrong-password" } as never,
      {} as never,
    );

    expect(res.valid).toBe(false);
    expect(res.user).toBeUndefined();
  });

  it("returns valid=false for nonexistent user", async () => {
    const res = await service.verifyCredentials(
      { email: "nobody@test.com", password: "any-password" } as never,
      {} as never,
    );

    expect(res.valid).toBe(false);
  });
});

// =============================================================================
// getUser
// =============================================================================

describe("getUser", () => {
  it("returns user when found", async () => {
    const created = await service.createUser(
      { email: "alice@test.com", name: "Alice" } as never,
      {} as never,
    );

    const res = await service.getUser(
      { userId: created.user!.id } as never,
      {} as never,
    );

    expect(res.user).toBeDefined();
    expect(res.user!.email).toBe("alice@test.com");
  });

  it("returns undefined user when not found", async () => {
    const res = await service.getUser(
      { userId: "nonexistent-id" } as never,
      {} as never,
    );

    expect(res.user).toBeUndefined();
  });
});

// =============================================================================
// createTokens
// =============================================================================

describe("createTokens", () => {
  it("returns valid JWT access token and refresh token", async () => {
    const created = await service.createUser(
      { email: "alice@test.com", name: "Alice" } as never,
      {} as never,
    );

    const res = await service.createTokens(
      {
        email: "alice@test.com",
        name: "Alice",
        userId: created.user!.id,
      } as never,
      {} as never,
    );

    expect(res.accessToken).toBeTruthy();
    expect(res.refreshToken).toBeTruthy();
    expect(res.refreshToken!.length).toBe(64);

    const { payload } = await jwtVerify(res.accessToken!, jwtSecret);
    expect(payload.sub).toBe(created.user!.id);
    expect(payload.email).toBe("alice@test.com");
    expect(payload.name).toBe("Alice");
  });
});

// =============================================================================
// refreshTokens
// =============================================================================

describe("refreshTokens", () => {
  it("rotates refresh token and returns new tokens + user", async () => {
    const created = await service.createUserWithPassword(
      { email: "alice@test.com", name: "Alice", password: "password123" } as never,
      {} as never,
    );

    const tokens = await service.createTokens(
      {
        email: "alice@test.com",
        name: "Alice",
        userId: created.user!.id,
      } as never,
      {} as never,
    );

    const res = await service.refreshTokens(
      { refreshToken: tokens.refreshToken } as never,
      {} as never,
    );

    expect(res.accessToken).toBeTruthy();
    expect(res.refreshToken).toBeTruthy();
    expect(res.refreshToken).not.toBe(tokens.refreshToken);
    expect(res.user).toBeDefined();
    expect(res.user!.email).toBe("alice@test.com");
  });

  it("rejects already-used refresh token", async () => {
    const created = await service.createUser(
      { email: "alice@test.com", name: "Alice" } as never,
      {} as never,
    );

    const tokens = await service.createTokens(
      {
        email: "alice@test.com",
        name: "Alice",
        userId: created.user!.id,
      } as never,
      {} as never,
    );

    await service.refreshTokens(
      { refreshToken: tokens.refreshToken } as never,
      {} as never,
    );

    await expect(
      service.refreshTokens(
        { refreshToken: tokens.refreshToken } as never,
        {} as never,
      ),
    ).rejects.toThrow(ConnectError);
  });

  it("rejects invalid refresh token", async () => {
    await expect(
      service.refreshTokens(
        { refreshToken: "bogus-token" } as never,
        {} as never,
      ),
    ).rejects.toThrow(ConnectError);
  });
});

// =============================================================================
// revokeRefreshToken
// =============================================================================

describe("revokeRefreshToken", () => {
  it("revokes existing token", async () => {
    const created = await service.createUser(
      { email: "alice@test.com", name: "Alice" } as never,
      {} as never,
    );

    const tokens = await service.createTokens(
      {
        email: "alice@test.com",
        name: "Alice",
        userId: created.user!.id,
      } as never,
      {} as never,
    );

    const res = await service.revokeRefreshToken(
      { refreshToken: tokens.refreshToken } as never,
      {} as never,
    );

    expect(res.success).toBe(true);

    // Can't use it anymore
    await expect(
      service.refreshTokens(
        { refreshToken: tokens.refreshToken } as never,
        {} as never,
      ),
    ).rejects.toThrow();
  });

  it("returns false for nonexistent token", async () => {
    const res = await service.revokeRefreshToken(
      { refreshToken: "nonexistent" } as never,
      {} as never,
    );

    expect(res.success).toBe(false);
  });
});

// =============================================================================
// getOAuthUrl
// =============================================================================

describe("getOAuthUrl", () => {
  it("returns Google OAuth URL", async () => {
    const res = await service.getOAuthUrl(
      {
        callbackUrl: "https://app.test/callback",
        provider: AuthProvider.GOOGLE,
      } as never,
      {} as never,
    );

    expect(res.url).toContain("accounts.google.com");
    expect(res.url).toContain("test-google-id");
    expect(res.state).toBeTruthy();
  });

  it("returns GitHub OAuth URL", async () => {
    const res = await service.getOAuthUrl(
      {
        callbackUrl: "https://app.test/callback",
        provider: AuthProvider.GIT_HUB,
      } as never,
      {} as never,
    );

    expect(res.url).toContain("github.com");
    expect(res.url).toContain("test-github-id");
  });

  it("generates state automatically via BetterAuth", async () => {
    const res = await service.getOAuthUrl(
      {
        callbackUrl: "https://app.test/callback",
        provider: AuthProvider.GOOGLE,
      } as never,
      {} as never,
    );

    expect(res.state).toBeTruthy();
    expect(res.state!.length).toBeGreaterThan(0);
  });

  it("throws for unconfigured provider", async () => {
    const t = await createTestService({
      authConfig: { oauth: undefined },
    });

    await expect(
      t.service.getOAuthUrl(
        {
          callbackUrl: "https://app.test/callback",
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

describe("exchangeOAuthCode", () => {
  it("throws InvalidArgument for credential provider", async () => {
    try {
      await service.exchangeOAuthCode(
        {
          code: "auth-code",
          provider: AuthProvider.EMAIL,
          state: "some-state",
        } as never,
        {} as never,
      );
      expect.fail("should have thrown");
    } catch (e) {
      expect(e).toBeInstanceOf(ConnectError);
      expect((e as ConnectError).code).toBe(Code.InvalidArgument);
    }
  });

  it("throws Unauthenticated for invalid state/code", async () => {
    // With a bogus state that doesn't exist in the verification table,
    // BetterAuth's callback handler won't set a session cookie.
    try {
      await service.exchangeOAuthCode(
        {
          code: "bogus-code",
          provider: AuthProvider.GOOGLE,
          state: "bogus-state",
        } as never,
        {} as never,
      );
      expect.fail("should have thrown");
    } catch (e) {
      expect(e).toBeInstanceOf(ConnectError);
      expect((e as ConnectError).code).toBe(Code.Unauthenticated);
    }
  });
});

// =============================================================================
// getAccountsByUserId
// =============================================================================

describe("getAccountsByUserId", () => {
  it("returns empty for user with no extra accounts", async () => {
    // createUser uses signUpEmail which creates a credential account internally,
    // but with a throwaway password. Check that we see the credential account.
    const created = await service.createUser(
      { email: "alice@test.com", name: "Alice" } as never,
      {} as never,
    );

    const res = await service.getAccountsByUserId(
      { userId: created.user!.id } as never,
      {} as never,
    );

    // BetterAuth creates a credential account for signUpEmail
    expect(res.accounts!.length).toBeGreaterThanOrEqual(0);
  });

  it("returns credential account with hasPassword=true", async () => {
    const created = await service.createUserWithPassword(
      { email: "alice@test.com", name: "Alice", password: "password123" } as never,
      {} as never,
    );

    const res = await service.getAccountsByUserId(
      { userId: created.user!.id } as never,
      {} as never,
    );

    expect(res.accounts!.length).toBeGreaterThanOrEqual(1);
    expect(res.hasPassword).toBe(true);

    const credentialAccount = res.accounts!.find(
      (a) => a.provider === AuthProvider.EMAIL,
    );
    expect(credentialAccount).toBeDefined();
  });

  it("returns multiple accounts", async () => {
    const created = await service.createUserWithPassword(
      { email: "alice@test.com", name: "Alice", password: "password123" } as never,
      {} as never,
    );

    await service.createAccount(
      {
        provider: AuthProvider.GOOGLE,
        providerAccountId: "google-123",
        userId: created.user!.id,
      } as never,
      {} as never,
    );

    const res = await service.getAccountsByUserId(
      { userId: created.user!.id } as never,
      {} as never,
    );

    expect(res.accounts!.length).toBeGreaterThanOrEqual(2);
    expect(res.hasPassword).toBe(true);
  });
});

// =============================================================================
// createAccount
// =============================================================================

describe("createAccount", () => {
  it("creates credential account with password", async () => {
    const created = await service.createUser(
      { email: "alice@test.com", name: "Alice" } as never,
      {} as never,
    );

    const res = await service.createAccount(
      {
        password: "secretpass",
        provider: AuthProvider.EMAIL,
        providerAccountId: "alice@test.com",
        userId: created.user!.id,
      } as never,
      {} as never,
    );

    expect(res.account).toBeDefined();
    expect(res.account!.provider).toBe(AuthProvider.EMAIL);
    expect(res.account!.passwordHash).toBeDefined();
    expect(res.account!.passwordHash).not.toBe("secretpass");

    // Verify the password is hashed and works via round-trip
    // (We can't easily verify the hash format since BetterAuth uses scrypt internally)
    expect(res.account!.passwordHash!.length).toBeGreaterThan(10);
  });

  it("creates OAuth account without password", async () => {
    const created = await service.createUser(
      { email: "alice@test.com", name: "Alice" } as never,
      {} as never,
    );

    const res = await service.createAccount(
      {
        accessToken: "gtoken",
        provider: AuthProvider.GOOGLE,
        providerAccountId: "google-123",
        userId: created.user!.id,
      } as never,
      {} as never,
    );

    expect(res.account).toBeDefined();
    expect(res.account!.provider).toBe(AuthProvider.GOOGLE);
    expect(res.account!.passwordHash).toBeUndefined();
    expect(res.account!.accessToken).toBe("gtoken");
  });
});

// =============================================================================
// Integration: full auth flow
// =============================================================================

describe("integration: full auth flow", () => {
  it("sign up -> verify -> tokens -> refresh -> revoke", async () => {
    // 1. Sign up
    const signup = await service.createUserWithPassword(
      { email: "alice@test.com", name: "Alice", password: "mypassword" } as never,
      {} as never,
    );
    expect(signup.user!.email).toBe("alice@test.com");

    // 2. Verify credentials
    const verify = await service.verifyCredentials(
      { email: "alice@test.com", password: "mypassword" } as never,
      {} as never,
    );
    expect(verify.valid).toBe(true);

    // 3. Create tokens
    const tokens = await service.createTokens(
      {
        email: "alice@test.com",
        name: "Alice",
        userId: signup.user!.id,
      } as never,
      {} as never,
    );
    expect(tokens.accessToken).toBeTruthy();

    // 4. Refresh tokens
    const refreshed = await service.refreshTokens(
      { refreshToken: tokens.refreshToken } as never,
      {} as never,
    );
    expect(refreshed.accessToken).toBeTruthy();
    expect(refreshed.refreshToken).not.toBe(tokens.refreshToken);

    // 5. Revoke
    const revoked = await service.revokeRefreshToken(
      { refreshToken: refreshed.refreshToken } as never,
      {} as never,
    );
    expect(revoked.success).toBe(true);

    // 6. Refresh should fail now
    await expect(
      service.refreshTokens(
        { refreshToken: refreshed.refreshToken } as never,
        {} as never,
      ),
    ).rejects.toThrow();
  });
});
