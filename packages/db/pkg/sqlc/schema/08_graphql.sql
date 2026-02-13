/*
 *
 * GRAPHQL SYSTEM
 * GraphQL request support - simpler than HTTP (no delta system)
 *
 */

-- Core GraphQL request table
CREATE TABLE graphql (
  id BLOB NOT NULL PRIMARY KEY,
  workspace_id BLOB NOT NULL,
  folder_id BLOB,
  name TEXT NOT NULL,
  url TEXT NOT NULL,
  query TEXT NOT NULL DEFAULT '',
  variables TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  last_run_at BIGINT NULL,
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),

  FOREIGN KEY (workspace_id) REFERENCES workspaces (id) ON DELETE CASCADE,
  FOREIGN KEY (folder_id) REFERENCES files (id) ON DELETE SET NULL
);

CREATE INDEX graphql_workspace_idx ON graphql (workspace_id);
CREATE INDEX graphql_folder_idx ON graphql (folder_id) WHERE folder_id IS NOT NULL;

-- GraphQL request headers
CREATE TABLE graphql_header (
  id BLOB NOT NULL PRIMARY KEY,
  graphql_id BLOB NOT NULL,
  header_key TEXT NOT NULL,
  header_value TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  display_order REAL NOT NULL DEFAULT 0,
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),

  FOREIGN KEY (graphql_id) REFERENCES graphql (id) ON DELETE CASCADE
);

CREATE INDEX graphql_header_graphql_idx ON graphql_header (graphql_id);
CREATE INDEX graphql_header_order_idx ON graphql_header (graphql_id, display_order);

-- GraphQL response (read-only)
CREATE TABLE graphql_response (
  id BLOB NOT NULL PRIMARY KEY,
  graphql_id BLOB NOT NULL,
  status INT32 NOT NULL,
  body BLOB,
  time DATETIME NOT NULL,
  duration INT32 NOT NULL,
  size INT32 NOT NULL,
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),

  FOREIGN KEY (graphql_id) REFERENCES graphql (id) ON DELETE CASCADE
);

CREATE INDEX graphql_response_graphql_idx ON graphql_response (graphql_id);
CREATE INDEX graphql_response_time_idx ON graphql_response (graphql_id, time DESC);

-- GraphQL response headers (read-only)
CREATE TABLE graphql_response_header (
  id BLOB NOT NULL PRIMARY KEY,
  response_id BLOB NOT NULL,
  key TEXT NOT NULL,
  value TEXT NOT NULL,
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),

  FOREIGN KEY (response_id) REFERENCES graphql_response (id) ON DELETE CASCADE
);

CREATE INDEX graphql_response_header_response_idx ON graphql_response_header (response_id);
