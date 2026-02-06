import type { ServiceImpl } from '@connectrpc/connect';
import type { Client } from '@libsql/client';
import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import { AuthInternalService } from '@the-dev-tools/spec/buf/api/auth_internal/v1/auth_internal_pb';
import {
  type Account,
  AccountSchema,
  AuthProvider,
  type User,
  UserSchema,
} from '@the-dev-tools/spec/buf/api/auth_internal/v1/auth_internal_pb';

import type { Auth } from './auth.js';
import { accountQueries, userQueries } from './queries.js';

// =============================================================================
// Types
// =============================================================================

export interface ServiceDeps {
  auth: Auth;
  rawDb: Client;
}

// =============================================================================
// Provider Mapping
// =============================================================================

const providerToString: Partial<Record<number, string>> = {
  [AuthProvider.EMAIL]: 'credential',
  [AuthProvider.GIT_HUB]: 'github',
  [AuthProvider.GOOGLE]: 'google',
  [AuthProvider.MICROSOFT]: 'microsoft',
  [AuthProvider.UNSPECIFIED]: 'unspecified',
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
  if (typeof value === 'string') return value;
  if (value < 10000000000) {
    return new Date(value * 1000).toISOString();
  }
  return new Date(value).toISOString();
}

function generateId(): string {
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
  let result = '';
  for (let i = 0; i < 32; i++) {
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

export function createInternalAuthService(deps: ServiceDeps): ServiceImpl<typeof AuthInternalService> {
  const { auth, rawDb } = deps;

  return {
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

        // Extract session token from BetterAuth's signUp response
        const sessionToken = result.token ?? '';

        // Fetch the credential account that BetterAuth created
        const accountResult = await rawDb.execute({
          args: ['credential', user.id],
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
          sessionToken,
          user: betterAuthUserToProto(user),
        };
      } catch (error) {
        if (error instanceof Error && error.message.includes('already')) {
          throw new ConnectError('A user with this email already exists', Code.AlreadyExists);
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

        // Extract session token from BetterAuth's signIn response
        const sessionToken = result.token ?? '';

        // Fetch the credential account for the response
        const accountResult = await rawDb.execute({
          args: ['credential', user.id],
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
          sessionToken,
          user: betterAuthUserToProto(user),
          valid: true,
        };
      } catch {
        // BetterAuth throws on invalid credentials
        return { valid: false };
      }
    },

    async getToken(req) {
      // Use BetterAuth's jwt() plugin to get a signed JWT access token.
      // The bearer() plugin lets us authenticate via Authorization header with the session token.
      try {
        const response = await auth.handler(
          new Request(`${(auth.options as { baseURL?: string }).baseURL ?? 'http://localhost:50051'}/api/auth/token`, {
            headers: {
              authorization: `Bearer ${req.sessionToken}`,
            },
            method: 'GET',
          }),
        );

        if (!response.ok) {
          throw new ConnectError('Failed to get token: session invalid or expired', Code.Unauthenticated);
        }

        const data = (await response.json()) as { token?: string };
        if (!data.token) {
          throw new ConnectError('BetterAuth did not return a JWT token', Code.Internal);
        }

        return { accessToken: data.token };
      } catch (error) {
        if (error instanceof ConnectError) throw error;
        throw new ConnectError(
          `Failed to get token: ${error instanceof Error ? error.message : String(error)}`,
          Code.Unauthenticated,
        );
      }
    },

    async getOAuthUrl(req) {
      const providerName = providerToString[req.provider];

      if (!providerName || providerName === 'unspecified' || providerName === 'credential') {
        throw new ConnectError(`Unsupported OAuth provider: ${providerName}`, Code.InvalidArgument);
      }

      try {
        const rawResult: unknown = await auth.api.signInSocial({
          body: {
            callbackURL: req.callbackUrl,
            disableRedirect: true,
            provider: providerName,
          },
        } as never);

        let oauthUrl: string;
        if (rawResult instanceof Response) {
          const data = (await rawResult.json()) as { url?: string };
          oauthUrl = data.url ?? '';
        } else {
          oauthUrl = (rawResult as { url: string }).url;
        }

        if (!oauthUrl) {
          throw new ConnectError('BetterAuth did not return an OAuth URL', Code.Internal);
        }

        const parsedUrl = new URL(oauthUrl);
        const state = parsedUrl.searchParams.get('state') ?? '';

        return { state, url: oauthUrl };
      } catch (error) {
        if (error instanceof ConnectError) throw error;
        const msg = error instanceof Error ? error.message : String(error);
        if (msg.includes('not found') || msg.includes('provider')) {
          throw new ConnectError(
            `${providerName.charAt(0).toUpperCase() + providerName.slice(1)} OAuth is not configured`,
            Code.FailedPrecondition,
          );
        }
        throw new ConnectError(`Failed to generate OAuth URL: ${msg}`, Code.Internal);
      }
    },

    async exchangeOAuthCode(req) {
      const providerName = providerToString[req.provider];

      if (!providerName || providerName === 'unspecified' || providerName === 'credential') {
        throw new ConnectError(`Unsupported OAuth provider: ${providerName}`, Code.InvalidArgument);
      }

      // Construct synthetic callback request for BetterAuth's handler
      const baseURL = (auth.options as { baseURL?: string }).baseURL ?? 'http://localhost:50051';
      const callbackUrl = new URL(`${baseURL}/api/auth/callback/${providerName}`);
      callbackUrl.searchParams.set('code', req.code);
      if (req.state) callbackUrl.searchParams.set('state', req.state);

      const response = await auth.handler(new Request(callbackUrl.toString()));

      // Extract session cookie from BetterAuth's response
      const cookies = response.headers.getSetCookie();
      const sessionCookie = cookies.find((c: string) => c.includes('better-auth.session_token='));

      if (!sessionCookie) {
        throw new ConnectError('OAuth code exchange failed', Code.Unauthenticated);
      }

      // Parse the session token value
      const tokenMatch = /(?:__Secure-)?better-auth\.session_token=([^;]+)/.exec(sessionCookie);
      if (!tokenMatch?.[1]) {
        throw new ConnectError('OAuth code exchange failed: no session token', Code.Unauthenticated);
      }

      const sessionToken = tokenMatch[1];

      // Retrieve user info from the session using bearer auth
      const sessionResult = (await auth.api.getSession({
        headers: new Headers({
          authorization: `Bearer ${sessionToken}`,
        }),
      } as never)) as null | {
        session: { userId: string };
        user: {
          createdAt: Date | number | string;
          email: string;
          emailVerified: boolean;
          id: string;
          image?: null | string;
          name: string;
          updatedAt: Date | number | string;
        };
      };

      if (!sessionResult) {
        throw new ConnectError('Failed to retrieve session after OAuth', Code.Internal);
      }

      const user = sessionResult.user;

      // Detect new user: if created within last 60 seconds, likely just created
      const now = Math.floor(Date.now() / 1000);
      const createdAtTs =
        user.createdAt instanceof Date
          ? Math.floor(user.createdAt.getTime() / 1000)
          : typeof user.createdAt === 'string'
            ? Math.floor(new Date(user.createdAt).getTime() / 1000)
            : user.createdAt;
      const isNewUser = now - createdAtTs < 60;

      return {
        isNewUser,
        sessionToken,
        user: betterAuthUserToProto(user),
      };
    },

    async getAccountsByUserId(req) {
      const result = await rawDb.execute({
        args: [req.userId],
        sql: accountQueries.getByUserId,
      });

      let hasPassword = false;
      const accounts: Account[] = [];

      for (const row of result.rows) {
        const providerId = row.providerId as string;
        const provider = stringToProvider[providerId] ?? AuthProvider.UNSPECIFIED;

        if (providerId === 'credential' && row.password) {
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
      const id = generateId();
      const now = Math.floor(Date.now() / 1000);
      const providerStr = providerToString[req.provider] ?? 'unspecified';

      if (providerStr === 'credential' && req.password) {
        const { hashPassword } = await import('better-auth/crypto');
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
