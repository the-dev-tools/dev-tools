import type { Client } from '@libsql/client';

export async function initDatabase(rawDb: Client): Promise<void> {
  // BetterAuth-managed tables (user, account, session, verification, jwks).
  // We create them manually because BetterAuth's getMigrations only supports
  // the Kysely adapter, and we use Drizzle with @libsql/client.
  await rawDb.execute(`
    CREATE TABLE IF NOT EXISTS user (
      id            TEXT PRIMARY KEY NOT NULL,
      email         TEXT UNIQUE NOT NULL,
      emailVerified INTEGER NOT NULL DEFAULT 0,
      name          TEXT NOT NULL DEFAULT '',
      image         TEXT,
      createdAt     INTEGER NOT NULL DEFAULT (unixepoch()),
      updatedAt     INTEGER NOT NULL DEFAULT (unixepoch())
    )
  `);

  await rawDb.execute(`
    CREATE TABLE IF NOT EXISTS account (
      id                    TEXT PRIMARY KEY NOT NULL,
      userId                TEXT NOT NULL,
      providerId            TEXT NOT NULL,
      accountId             TEXT NOT NULL,
      password              TEXT,
      accessToken           TEXT,
      refreshToken          TEXT,
      accessTokenExpiresAt  INTEGER,
      scope                 TEXT,
      idToken               TEXT,
      createdAt             INTEGER NOT NULL DEFAULT (unixepoch()),
      updatedAt             INTEGER NOT NULL DEFAULT (unixepoch()),
      FOREIGN KEY (userId) REFERENCES user(id) ON DELETE CASCADE
    )
  `);

  await rawDb.execute(`
    CREATE TABLE IF NOT EXISTS session (
      id        TEXT PRIMARY KEY NOT NULL,
      userId    TEXT NOT NULL,
      token     TEXT UNIQUE NOT NULL,
      expiresAt INTEGER NOT NULL,
      ipAddress TEXT,
      userAgent TEXT,
      createdAt INTEGER NOT NULL DEFAULT (unixepoch()),
      updatedAt INTEGER NOT NULL DEFAULT (unixepoch()),
      FOREIGN KEY (userId) REFERENCES user(id) ON DELETE CASCADE
    )
  `);

  await rawDb.execute(`
    CREATE TABLE IF NOT EXISTS verification (
      id         TEXT PRIMARY KEY NOT NULL,
      identifier TEXT NOT NULL,
      value      TEXT NOT NULL,
      expiresAt  INTEGER NOT NULL,
      createdAt  INTEGER NOT NULL DEFAULT (unixepoch()),
      updatedAt  INTEGER NOT NULL DEFAULT (unixepoch())
    )
  `);

  // JWKS table for BetterAuth's jwt() plugin (RS256 key pairs)
  await rawDb.execute(`
    CREATE TABLE IF NOT EXISTS jwks (
      id         TEXT PRIMARY KEY NOT NULL,
      publicKey  TEXT NOT NULL,
      privateKey TEXT NOT NULL,
      createdAt  INTEGER NOT NULL DEFAULT (unixepoch())
    )
  `);

  // Indexes for BetterAuth tables
  await rawDb.execute(`CREATE INDEX IF NOT EXISTS idx_user_email ON user(email)`);
  await rawDb.execute(`CREATE INDEX IF NOT EXISTS idx_account_providerId ON account(providerId, accountId)`);
}
