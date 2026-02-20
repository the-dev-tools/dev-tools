-- BetterAuth Organization tables
-- IDs are 16-byte ULIDs stored as BLOB
-- Timestamps are INTEGER (Unix seconds)
-- role/status are TEXT (BetterAuth sends strings)

CREATE TABLE auth_organization (
  id BLOB NOT NULL PRIMARY KEY,
  name TEXT NOT NULL,
  slug TEXT UNIQUE,
  logo TEXT,
  metadata TEXT,
  created_at INTEGER NOT NULL,
  CHECK (length(id) = 16)
);

CREATE TABLE auth_member (
  id BLOB NOT NULL PRIMARY KEY,
  user_id BLOB NOT NULL,
  organization_id BLOB NOT NULL,
  role TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  CHECK (length(id) = 16),
  FOREIGN KEY (user_id) REFERENCES auth_user (id) ON DELETE CASCADE,
  FOREIGN KEY (organization_id) REFERENCES auth_organization (id) ON DELETE CASCADE
);

CREATE TABLE auth_invitation (
  id BLOB NOT NULL PRIMARY KEY,
  email TEXT NOT NULL,
  inviter_id BLOB NOT NULL,
  organization_id BLOB NOT NULL,
  role TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  expires_at INTEGER NOT NULL,
  CHECK (length(id) = 16),
  FOREIGN KEY (inviter_id) REFERENCES auth_user (id) ON DELETE CASCADE,
  FOREIGN KEY (organization_id) REFERENCES auth_organization (id) ON DELETE CASCADE
);
