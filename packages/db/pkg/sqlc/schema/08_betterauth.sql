-- BetterAuth tables
-- IDs are 16-byte ULIDs stored as BLOB (sent by BetterAuth)
-- Timestamps are INTEGER (Unix seconds)
-- Booleans are INTEGER (0/1)

CREATE TABLE auth_user (
  id BLOB NOT NULL PRIMARY KEY,
  name TEXT NOT NULL,
  email TEXT NOT NULL UNIQUE,
  email_verified INTEGER NOT NULL DEFAULT 0,
  image TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CHECK (length(id) = 16)
);

CREATE TABLE auth_session (
  id BLOB NOT NULL PRIMARY KEY,
  user_id BLOB NOT NULL,
  token TEXT NOT NULL UNIQUE,
  expires_at INTEGER NOT NULL,
  ip_address TEXT,
  user_agent TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CHECK (length(id) = 16),
  FOREIGN KEY (user_id) REFERENCES auth_user (id) ON DELETE CASCADE
);

CREATE TABLE auth_account (
  id BLOB NOT NULL PRIMARY KEY,
  user_id BLOB NOT NULL,
  account_id TEXT NOT NULL,
  provider_id TEXT NOT NULL,
  access_token TEXT,
  refresh_token TEXT,
  access_token_expires_at INTEGER,
  refresh_token_expires_at INTEGER,
  scope TEXT,
  id_token TEXT,
  password TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CHECK (length(id) = 16),
  FOREIGN KEY (user_id) REFERENCES auth_user (id) ON DELETE CASCADE
);

CREATE TABLE auth_verification (
  id BLOB NOT NULL PRIMARY KEY,
  identifier TEXT NOT NULL,
  value TEXT NOT NULL,
  expires_at INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CHECK (length(id) = 16)
);

CREATE TABLE auth_jwks (
  id BLOB NOT NULL PRIMARY KEY,
  public_key TEXT NOT NULL,
  private_key TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  expires_at INTEGER,
  CHECK (length(id) = 16)
);
