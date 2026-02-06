import type { ServiceImpl } from "@connectrpc/connect";
import type { Client } from "@libsql/client";
import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { SignJWT } from "jose";

import { AuthInternalService } from "@the-dev-tools/spec/buf/api/auth_internal/v1/auth_internal_pb";
import {
  type Account,
  AccountSchema,
  AuthProvider,
  type User,
  UserSchema,
} from "@the-dev-tools/spec/buf/api/auth_internal/v1/auth_internal_pb";

import type { Auth } from "./auth.js";
import {
  accountQueries,
  refreshTokenQueries,
  userQueries,
} from "./queries.js";

// =============================================================================
// Types
// =============================================================================

export interface ServiceConfig {
  jwt: { accessTokenExpiry: number };
  refreshToken: { expiry: number };
}

export interface ServiceDeps {
  auth: Auth;
  config: ServiceConfig;
  jwtSecret: Uint8Array;
  rawDb: Client;
}

// =============================================================================
// Provider Mapping
// =============================================================================

const providerToString: Partial<Record<number, string>> = {
  [AuthProvider.EMAIL]: "credential",
  [AuthProvider.GIT_HUB]: "github",
  [AuthProvider.GOOGLE]: "google",
  [AuthProvider.MICROSOFT]: "microsoft",
  [AuthProvider.UNSPECIFIED]: "unspecified",
};

const stringToProvider: Record<string, AuthProvider> = {
  credential: AuthProvider.EMAIL,
  github: AuthProvider.GIT_HUB,
  google: AuthProvider.GOOGLE,
  microsoft: AuthProvider.MICROSOFT,
};

// =============================================================================
// Helpers
// =============================================================================

function toISOString(value: null | number | string): string {
  if (value === null) return new Date().toISOString();
  if (typeof value === "string") return value;
  if (value < 10000000000) {
    return new Date(value * 1000).toISOString();
  }
  return new Date(value).toISOString();
}

function generateId(): string {
  const chars =
    "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";
  let result = "";
  for (let i = 0; i < 32; i++) {
    result += chars.charAt(Math.floor(Math.random() * chars.length));
  }
  return result;
}

function generateRefreshToken(): string {
  const chars =
    "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";
  let result = "";
  for (let i = 0; i < 64; i++) {
    result += chars.charAt(Math.floor(Math.random() * chars.length));
  }
  return result;
}

function createUserProto(data: {
  createdAt: string;
  email: string;
  emailVerified: boolean;
  id: string;
  image?: string;
  name: string;
  updatedAt: string;
}): User {
  return create(UserSchema, data);
}

function createAccountProto(data: {
  accessToken?: string;
  accessTokenExpiresAt?: string;
  createdAt: string;
  id: string;
  idToken?: string;
  oauthRefreshToken?: string;
  passwordHash?: string;
  provider: AuthProvider;
  providerAccountId: string;
  scope?: string;
  updatedAt: string;
  userId: string;
}): Account {
  return create(AccountSchema, data);
}

/**
 * Convert a BetterAuth user object to our proto User.
 * BetterAuth returns dates as Date objects or epoch integers depending on adapter.
 */
function betterAuthUserToProto(user: {
  createdAt: Date | number | string;
  email: string;
  emailVerified: boolean | number;
  id: string;
  image?: null | string;
  name: string;
  updatedAt: Date | number | string;
}): User {
  return createUserProto({
    createdAt: toISOString(user.createdAt instanceof Date ? user.createdAt.getTime() : user.createdAt),
    email: user.email,
    emailVerified: Boolean(user.emailVerified),
    id: user.id,
    image: user.image ?? undefined,
    name: user.name,
    updatedAt: toISOString(user.updatedAt instanceof Date ? user.updatedAt.getTime() : user.updatedAt),
  });
}

// =============================================================================
// Factory
// =============================================================================

export function createInternalAuthService(
  deps: ServiceDeps,
): ServiceImpl<typeof AuthInternalService> {
  const { auth, config, jwtSecret, rawDb } = deps;

  async function generateAccessToken(
    userId: string,
    email: string,
    name: string,
  ): Promise<string> {
    const now = Math.floor(Date.now() / 1000);
    const exp = now + config.jwt.accessTokenExpiry;

    return new SignJWT({ email, name })
      .setProtectedHeader({ alg: "HS256" })
      .setSubject(userId)
      .setIssuedAt(now)
      .setExpirationTime(exp)
      .sign(jwtSecret);
  }

  return {
    async createUser(req) {
      // BetterAuth requires a password for signUpEmail, so we generate a
      // random throwaway for OAuth-first users who won't use password login.
      const throwawayPassword = crypto.randomUUID();

      try {
        const result = await auth.api.signUpEmail({
          body: {
            email: req.email,
            name: req.name,
            password: throwawayPassword,
          },
        });

        return {
          user: betterAuthUserToProto(result.user),
        };
      } catch (error) {
        if (error instanceof Error && error.message.includes("already")) {
          throw new ConnectError(
            "A user with this email already exists",
            Code.AlreadyExists,
          );
        }
        throw error;
      }
    },

    async createUserWithPassword(req) {
      try {
        const result = await auth.api.signUpEmail({
          body: {
            email: req.email,
            name: req.name,
            password: req.password,
          },
        });

        const user = result.user;

        // Fetch the credential account that BetterAuth created
        // BetterAuth uses userId as the accountId for credential accounts
        const accountResult = await rawDb.execute({
          args: ["credential", user.id],
          sql: accountQueries.getByProvider,
        });

        let account: Account | undefined;
        if (accountResult.rows.length > 0) {
          const row = accountResult.rows[0];
          account = createAccountProto({
            createdAt: toISOString(row.createdAt as number),
            id: row.id as string,
            passwordHash: (row.password as null | string) ?? undefined,
            provider: AuthProvider.EMAIL,
            providerAccountId: row.accountId as string,
            updatedAt: toISOString(row.updatedAt as number),
            userId: user.id,
          });
        }

        return {
          account,
          user: betterAuthUserToProto(user),
        };
      } catch (error) {
        if (error instanceof Error && error.message.includes("already")) {
          throw new ConnectError(
            "A user with this email already exists",
            Code.AlreadyExists,
          );
        }
        throw error;
      }
    },

    async verifyCredentials(req) {
      try {
        const result = await auth.api.signInEmail({
          body: {
            email: req.email,
            password: req.password,
          },
        });

        const user = result.user;

        // Fetch the credential account for the response
        // BetterAuth uses userId as the accountId for credential accounts
        const accountResult = await rawDb.execute({
          args: ["credential", user.id],
          sql: accountQueries.getByProvider,
        });

        let account: Account | undefined;
        if (accountResult.rows.length > 0) {
          const row = accountResult.rows[0];
          account = createAccountProto({
            createdAt: toISOString(row.createdAt as number),
            id: row.id as string,
            passwordHash: (row.password as null | string) ?? undefined,
            provider: AuthProvider.EMAIL,
            providerAccountId: row.accountId as string,
            updatedAt: toISOString(row.updatedAt as number),
            userId: user.id,
          });
        }

        return {
          account,
          user: betterAuthUserToProto(user),
          valid: true,
        };
      } catch {
        // BetterAuth throws on invalid credentials
        return { valid: false };
      }
    },

    async getUser(req) {
      // Simple read — no BetterAuth API for direct ID lookup without session
      const result = await rawDb.execute({
        args: [req.userId],
        sql: userQueries.getById,
      });

      if (result.rows.length === 0) {
        return { user: undefined };
      }

      const row = result.rows[0];
      return {
        user: createUserProto({
          createdAt: toISOString(row.createdAt as number),
          email: row.email as string,
          emailVerified: Boolean(row.emailVerified),
          id: row.id as string,
          image: (row.image as null | string) ?? undefined,
          name: (row.name as null | string) ?? "",
          updatedAt: toISOString(row.updatedAt as number),
        }),
      };
    },

    async createTokens(req) {
      const accessToken = await generateAccessToken(
        req.userId,
        req.email,
        req.name,
      );

      const refreshTokenId = generateId();
      const refreshToken = generateRefreshToken();
      const now = Math.floor(Date.now() / 1000);
      const refreshExpiresAt = now + config.refreshToken.expiry;

      await rawDb.execute({
        args: [
          refreshTokenId,
          req.userId,
          refreshToken,
          refreshExpiresAt,
          req.ipAddress ?? null,
          req.userAgent ?? null,
          now,
        ],
        sql: refreshTokenQueries.insert,
      });

      return { accessToken, refreshToken };
    },

    async refreshTokens(req) {
      const now = Math.floor(Date.now() / 1000);

      const result = await rawDb.execute({
        args: [req.refreshToken, now],
        sql: refreshTokenQueries.getWithUser,
      });

      if (result.rows.length === 0) {
        throw new ConnectError(
          "Invalid or expired refresh token",
          Code.Unauthenticated,
        );
      }

      const row = result.rows[0];
      const userId = row.userId as string;
      const email = row.email as string;
      const name = (row.name as null | string) ?? "";

      await rawDb.execute({
        args: [req.refreshToken],
        sql: refreshTokenQueries.deleteByToken,
      });

      const accessToken = await generateAccessToken(userId, email, name);

      const newRefreshTokenId = generateId();
      const newRefreshToken = generateRefreshToken();
      const refreshExpiresAt = now + config.refreshToken.expiry;

      await rawDb.execute({
        args: [newRefreshTokenId, userId, newRefreshToken, refreshExpiresAt, now],
        sql: refreshTokenQueries.insertMinimal,
      });

      return {
        accessToken,
        refreshToken: newRefreshToken,
        user: createUserProto({
          createdAt: toISOString(row.userCreatedAt as number),
          email,
          emailVerified: Boolean(row.emailVerified),
          id: userId,
          image: (row.image as null | string) ?? undefined,
          name,
          updatedAt: toISOString(row.userUpdatedAt as number),
        }),
      };
    },

    async revokeRefreshToken(req) {
      const result = await rawDb.execute({
        args: [req.refreshToken],
        sql: refreshTokenQueries.deleteByToken,
      });

      return { success: result.rowsAffected > 0 };
    },

    async getOAuthUrl(req) {
      const providerName = providerToString[req.provider];

      if (
        !providerName ||
        providerName === "unspecified" ||
        providerName === "credential"
      ) {
        throw new ConnectError(
          `Unsupported OAuth provider: ${providerName}`,
          Code.InvalidArgument,
        );
      }

      try {
        // BetterAuth's signInSocial with disableRedirect returns { url, redirect }
        // but TypeScript types it as Response in some overloads.
        const rawResult: unknown = await auth.api.signInSocial({
          body: {
            callbackURL: req.callbackUrl,
            disableRedirect: true,
            provider: providerName,
          },
        } as never);

        // Handle both Response and parsed object return types
        let oauthUrl: string;
        if (rawResult instanceof Response) {
          const data = (await rawResult.json()) as { url?: string };
          oauthUrl = data.url ?? "";
        } else {
          oauthUrl = (rawResult as { url: string }).url;
        }

        if (!oauthUrl) {
          throw new ConnectError(
            "BetterAuth did not return an OAuth URL",
            Code.Internal,
          );
        }

        // Extract state from the generated OAuth URL
        const parsedUrl = new URL(oauthUrl);
        const state = parsedUrl.searchParams.get("state") ?? "";

        return { state, url: oauthUrl };
      } catch (error) {
        if (error instanceof ConnectError) throw error;
        const msg = error instanceof Error ? error.message : String(error);
        if (msg.includes("not found") || msg.includes("provider")) {
          throw new ConnectError(
            `${providerName.charAt(0).toUpperCase() + providerName.slice(1)} OAuth is not configured`,
            Code.FailedPrecondition,
          );
        }
        throw new ConnectError(
          `Failed to generate OAuth URL: ${msg}`,
          Code.Internal,
        );
      }
    },

    async exchangeOAuthCode(req) {
      const providerName = providerToString[req.provider];

      if (
        !providerName ||
        providerName === "unspecified" ||
        providerName === "credential"
      ) {
        throw new ConnectError(
          `Unsupported OAuth provider: ${providerName}`,
          Code.InvalidArgument,
        );
      }

      // Construct synthetic callback request for BetterAuth's handler
      const baseURL = (auth.options as { baseURL?: string }).baseURL ?? "http://localhost:50051";
      const callbackUrl = new URL(`${baseURL}/api/auth/callback/${providerName}`);
      callbackUrl.searchParams.set("code", req.code);
      if (req.state) callbackUrl.searchParams.set("state", req.state);

      const response = await auth.handler(new Request(callbackUrl.toString()));

      // Extract session cookie from BetterAuth's response
      const cookies = response.headers.getSetCookie();
      const sessionCookie = cookies.find(
        (c: string) => c.includes("better-auth.session_token="),
      );

      if (!sessionCookie) {
        throw new ConnectError(
          "OAuth code exchange failed",
          Code.Unauthenticated,
        );
      }

      // Parse the session token value
      const tokenMatch = /(?:__Secure-)?better-auth\.session_token=([^;]+)/.exec(sessionCookie);
      if (!tokenMatch?.[1]) {
        throw new ConnectError(
          "OAuth code exchange failed: no session token",
          Code.Unauthenticated,
        );
      }

      // Retrieve user info from the session
      const sessionResult = await auth.api.getSession({
        headers: new Headers({
          cookie: `better-auth.session_token=${tokenMatch[1]}`,
        }),
      } as never) as null | { session: { userId: string }; user: { createdAt: Date | number | string; email: string; emailVerified: boolean; id: string; image?: null | string; name: string; updatedAt: Date | number | string } };

      if (!sessionResult) {
        throw new ConnectError(
          "Failed to retrieve session after OAuth",
          Code.Internal,
        );
      }

      const user = sessionResult.user;

      // Detect new user: if created within last 60 seconds, likely just created
      const now = Math.floor(Date.now() / 1000);
      const createdAtTs = user.createdAt instanceof Date
        ? Math.floor(user.createdAt.getTime() / 1000)
        : typeof user.createdAt === "string"
          ? Math.floor(new Date(user.createdAt).getTime() / 1000)
          : user.createdAt;
      const isNewUser = (now - createdAtTs) < 60;

      // Issue our custom JWT + refresh token
      const accessToken = await generateAccessToken(
        user.id,
        user.email,
        user.name,
      );

      const refreshTokenId = generateId();
      const refreshToken = generateRefreshToken();
      const refreshExpiresAt = now + config.refreshToken.expiry;

      await rawDb.execute({
        args: [
          refreshTokenId,
          user.id,
          refreshToken,
          refreshExpiresAt,
          req.ipAddress ?? null,
          req.userAgent ?? null,
          now,
        ],
        sql: refreshTokenQueries.insert,
      });

      return {
        accessToken,
        isNewUser,
        refreshToken,
        user: betterAuthUserToProto(user),
      };
    },

    async getAccountsByUserId(req) {
      // Direct read — BetterAuth's listAccounts requires session headers
      const result = await rawDb.execute({
        args: [req.userId],
        sql: accountQueries.getByUserId,
      });

      let hasPassword = false;
      const accounts: Account[] = [];

      for (const row of result.rows) {
        const providerId = row.providerId as string;
        const provider = stringToProvider[providerId] ?? AuthProvider.UNSPECIFIED;

        if (providerId === "credential" && row.password) {
          hasPassword = true;
        }

        accounts.push(
          createAccountProto({
            accessToken: (row.accessToken as null | string) ?? undefined,
            accessTokenExpiresAt: row.accessTokenExpiresAt
              ? toISOString(row.accessTokenExpiresAt as number)
              : undefined,
            createdAt: toISOString(row.createdAt as number),
            id: row.id as string,
            idToken: (row.idToken as null | string) ?? undefined,
            oauthRefreshToken: (row.refreshToken as null | string) ?? undefined,
            passwordHash: (row.password as null | string) ?? undefined,
            provider,
            providerAccountId: row.accountId as string,
            scope: (row.scope as null | string) ?? undefined,
            updatedAt: toISOString(row.updatedAt as number),
            userId: row.userId as string,
          }),
        );
      }

      return { accounts, hasPassword };
    },

    async createAccount(req) {
      // For OAuth accounts, use raw SQL insert
      const id = generateId();
      const now = Math.floor(Date.now() / 1000);
      const providerStr = providerToString[req.provider] ?? "unspecified";

      // For credential accounts with password, use BetterAuth to hash
      // For OAuth accounts, insert directly
      if (providerStr === "credential" && req.password) {
        // Use BetterAuth's password hashing (consistent with signUpEmail)
        const { hashPassword } = await import("better-auth/crypto");
        const passwordHash = await hashPassword(req.password);

        await rawDb.execute({
          args: [
            id,
            req.userId,
            providerStr,
            req.providerAccountId,
            passwordHash,
            null,
            null,
            null,
            null,
            null,
            now,
            now,
          ],
          sql: accountQueries.insert,
        });

        return {
          account: createAccountProto({
            createdAt: toISOString(now),
            id,
            passwordHash,
            provider: req.provider,
            providerAccountId: req.providerAccountId,
            updatedAt: toISOString(now),
            userId: req.userId,
          }),
        };
      }

      await rawDb.execute({
        args: [
          id,
          req.userId,
          providerStr,
          req.providerAccountId,
          null,
          req.accessToken ?? null,
          req.oauthRefreshToken ?? null,
          req.accessTokenExpiresAt ?? null,
          req.scope ?? null,
          req.idToken ?? null,
          now,
          now,
        ],
        sql: accountQueries.insert,
      });

      return {
        account: createAccountProto({
          accessToken: req.accessToken,
          accessTokenExpiresAt: req.accessTokenExpiresAt,
          createdAt: toISOString(now),
          id,
          idToken: req.idToken,
          oauthRefreshToken: req.oauthRefreshToken,
          provider: req.provider,
          providerAccountId: req.providerAccountId,
          scope: req.scope,
          updatedAt: toISOString(now),
          userId: req.userId,
        }),
      };
    },
  };
}
