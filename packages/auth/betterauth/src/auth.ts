import type { Client } from '@libsql/client';
import { betterAuth } from 'better-auth';
import { drizzleAdapter } from 'better-auth/adapters/drizzle';
import { bearer, jwt } from 'better-auth/plugins';
import { sql } from 'drizzle-orm';
import { drizzle } from 'drizzle-orm/libsql';
import { integer, sqliteTable, text } from 'drizzle-orm/sqlite-core';

// =============================================================================
// Drizzle Schema (required by BetterAuth's Drizzle adapter)
// =============================================================================

const user = sqliteTable('user', {
  createdAt: integer('createdAt')
    .notNull()
    .default(sql`(unixepoch())`),
  email: text('email').unique().notNull(),
  emailVerified: integer('emailVerified').notNull().default(0),
  id: text('id').primaryKey().notNull(),
  image: text('image'),
  name: text('name').notNull().default(''),
  updatedAt: integer('updatedAt')
    .notNull()
    .default(sql`(unixepoch())`),
});

const account = sqliteTable('account', {
  accessToken: text('accessToken'),
  accessTokenExpiresAt: integer('accessTokenExpiresAt'),
  accountId: text('accountId').notNull(),
  createdAt: integer('createdAt')
    .notNull()
    .default(sql`(unixepoch())`),
  id: text('id').primaryKey().notNull(),
  idToken: text('idToken'),
  password: text('password'),
  providerId: text('providerId').notNull(),
  refreshToken: text('refreshToken'),
  scope: text('scope'),
  updatedAt: integer('updatedAt')
    .notNull()
    .default(sql`(unixepoch())`),
  userId: text('userId')
    .notNull()
    .references(() => user.id, { onDelete: 'cascade' }),
});

const session = sqliteTable('session', {
  createdAt: integer('createdAt')
    .notNull()
    .default(sql`(unixepoch())`),
  expiresAt: integer('expiresAt').notNull(),
  id: text('id').primaryKey().notNull(),
  ipAddress: text('ipAddress'),
  token: text('token').unique().notNull(),
  updatedAt: integer('updatedAt')
    .notNull()
    .default(sql`(unixepoch())`),
  userAgent: text('userAgent'),
  userId: text('userId')
    .notNull()
    .references(() => user.id, { onDelete: 'cascade' }),
});

const verification = sqliteTable('verification', {
  createdAt: integer('createdAt')
    .notNull()
    .default(sql`(unixepoch())`),
  expiresAt: integer('expiresAt').notNull(),
  id: text('id').primaryKey().notNull(),
  identifier: text('identifier').notNull(),
  updatedAt: integer('updatedAt')
    .notNull()
    .default(sql`(unixepoch())`),
  value: text('value').notNull(),
});

const jwks = sqliteTable('jwks', {
  createdAt: integer('createdAt')
    .notNull()
    .default(sql`(unixepoch())`),
  id: text('id').primaryKey().notNull(),
  privateKey: text('privateKey').notNull(),
  publicKey: text('publicKey').notNull(),
});

const schema = { account, jwks, session, user, verification };

// =============================================================================
// BetterAuth Factory
// =============================================================================

export interface AuthConfig {
  baseURL?: string;
  oauth?: {
    github?: { clientId: string; clientSecret: string };
    google?: { clientId: string; clientSecret: string };
    microsoft?: { clientId: string; clientSecret: string };
  };
  secret: string;
}

export function createAuth(client: Client, config: AuthConfig) {
  const db = drizzle(client);
  return betterAuth({
    baseURL: config.baseURL ?? 'http://localhost:50051',
    database: drizzleAdapter(db, { provider: 'sqlite', schema }),
    emailAndPassword: { enabled: true, requireEmailVerification: false },
    plugins: [
      bearer(),
      jwt({
        jwks: {
          keyPairConfig: { alg: 'RS256' },
        },
        jwt: {
          definePayload: ({ user }) => ({
            email: user.email,
            name: user.name,
            sub: user.id,
          }),
        },
      }),
    ],
    secret: config.secret,
    session: {
      expiresIn: 60 * 60 * 24 * 7, // 7 days
      updateAge: 60 * 60 * 24, // update session every day
    },
    socialProviders: {
      ...(config.oauth?.google && { google: config.oauth.google }),
      ...(config.oauth?.github && { github: config.oauth.github }),
      ...(config.oauth?.microsoft && { microsoft: config.oauth.microsoft }),
    },
  });
}

export type Auth = ReturnType<typeof createAuth>;
