// =============================================================================
// Custom Schema DDL (refresh_token only — BetterAuth manages all other tables)
// =============================================================================

export const schema = {
  createRefreshTokenTable: `
    CREATE TABLE IF NOT EXISTS refresh_token (
      id TEXT PRIMARY KEY NOT NULL,
      userId TEXT NOT NULL,
      token TEXT UNIQUE NOT NULL,
      expiresAt INTEGER NOT NULL,
      ipAddress TEXT,
      userAgent TEXT,
      createdAt INTEGER NOT NULL DEFAULT (unixepoch()),
      FOREIGN KEY (userId) REFERENCES user(id) ON DELETE CASCADE
    )
  `,
} as const;

// =============================================================================
// Indexes
// =============================================================================

export const indexes = {
  refreshTokenByToken: `CREATE INDEX IF NOT EXISTS idx_refresh_token_token ON refresh_token(token)`,
  refreshTokenByUserId: `CREATE INDEX IF NOT EXISTS idx_refresh_token_userId ON refresh_token(userId)`,
} as const;

// =============================================================================
// User Queries (read-only — BetterAuth handles creation)
// =============================================================================

export const userQueries = {
  /** Get user by ID */
  getById: `SELECT * FROM user WHERE id = ?`,
} as const;

// =============================================================================
// Refresh Token Queries
// =============================================================================

export const refreshTokenQueries = {
  /** Insert a new refresh token */
  insert: `
    INSERT INTO refresh_token (id, userId, token, expiresAt, ipAddress, userAgent, createdAt)
    VALUES (?, ?, ?, ?, ?, ?, ?)
  `,

  /** Get refresh token with user data (for validation) */
  getWithUser: `
    SELECT
      rt.*,
      u.email,
      u.name,
      u.image,
      u.emailVerified,
      u.createdAt as userCreatedAt,
      u.updatedAt as userUpdatedAt
    FROM refresh_token rt
    JOIN user u ON rt.userId = u.id
    WHERE rt.token = ? AND rt.expiresAt > ?
  `,

  /** Delete refresh token by token value */
  deleteByToken: `DELETE FROM refresh_token WHERE token = ?`,

  /** Delete all refresh tokens for a user */
  deleteByUserId: `DELETE FROM refresh_token WHERE userId = ?`,

  /** Insert refresh token (without ip/user agent) */
  insertMinimal: `
    INSERT INTO refresh_token (id, userId, token, expiresAt, createdAt)
    VALUES (?, ?, ?, ?, ?)
  `,
} as const;

// =============================================================================
// Account Queries (read-only + OAuth inserts — BetterAuth handles credential accounts)
// =============================================================================

export const accountQueries = {
  /** Insert a new account (OAuth providers only — credential accounts via BetterAuth) */
  insert: `
    INSERT INTO account (id, userId, providerId, accountId, password, accessToken, refreshToken, accessTokenExpiresAt, scope, idToken, createdAt, updatedAt)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
  `,

  /** Get accounts by user ID */
  getByUserId: `SELECT * FROM account WHERE userId = ?`,

  /** Get account by provider and account ID */
  getByProvider: `SELECT * FROM account WHERE providerId = ? AND accountId = ?`,
} as const;
