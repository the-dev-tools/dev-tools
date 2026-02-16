// =============================================================================
// User Queries (read-only — BetterAuth handles creation)
// =============================================================================

export const userQueries = {
  /** Get user by ID */
  getById: `SELECT * FROM user WHERE id = ?`,
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

  /** Get all accounts */
  getAll: `SELECT * FROM account`,

  /** Get account by provider and account ID */
  getByProvider: `SELECT * FROM account WHERE providerId = ? AND userId = ?`,
} as const;
