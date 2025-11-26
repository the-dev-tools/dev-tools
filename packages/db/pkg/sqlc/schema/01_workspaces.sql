-- WORK SPACES
CREATE TABLE workspaces (
  id BLOB NOT NULL PRIMARY KEY,
  name TEXT NOT NULL,
  updated BIGINT NOT NULL DEFAULT (unixepoch()),
  collection_count INT NOT NULL DEFAULT 0,
  flow_count INT NOT NULL DEFAULT 0,
  active_env BLOB,
  global_env BLOB,
  prev BLOB,
  next BLOB,
  FOREIGN KEY (prev) REFERENCES workspaces (id) ON DELETE SET NULL,
  FOREIGN KEY (next) REFERENCES workspaces (id) ON DELETE SET NULL
);

CREATE INDEX workspaces_idx1 ON workspaces (
  name,
  active_env
);

-- WORKSPACE USERS
CREATE TABLE workspaces_users (
  id BLOB NOT NULL PRIMARY KEY,
  workspace_id BLOB NOT NULL,
  user_id BLOB NOT NULL,
  role INT8 NOT NULL DEFAULT 1,
  CHECK (length (id) == 16),
  CHECK (role IN (1, 2, 3)),
  UNIQUE (workspace_id, user_id),
  FOREIGN KEY (workspace_id) REFERENCES workspaces (id) ON DELETE CASCADE,
  FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

CREATE INDEX workspaces_users_idx1 ON workspaces_users (
  workspace_id,
  user_id,
  role
);
