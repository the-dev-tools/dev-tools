-- USERS
CREATE TABLE users (
  id BLOB NOT NULL PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  password_hash BLOB,
  provider_type INT8 NOT NULL DEFAULT 0,
  provider_id TEXT,
  status INT8 NOT NULL DEFAULT 0,
  CHECK (length(id) == 16),
  CHECK (status IN (0, 1, 2, 3)),
  CHECK (provider_type IN (0, 1, 2, 3)),
  UNIQUE (provider_type, provider_id)
);

-- WORK SPACES 
CREATE TABLE workspaces (
  id BLOB NOT NULL PRIMARY KEY,
  name TEXT NOT NULL,
  updated TIMESTAMP NOT NULL DEFAULT (unixepoch ()),
  CHECK (length(id) == 16)
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

-- RESULT API
CREATE TABLE result_api (
  id BLOB NOT NULL PRIMARY KEY,
  trigger_type TINYINT,
  trigger_by BLOB,
  name TEXT NOT NULL,
  status TEXT NOT NULL,
  time TIMESTAMP NOT NULL DEFAULT (unixepoch ()),
  duration BIGINT NOT NULL,
  http_resp BLOB
);

CREATE INDEX Idx4 ON result_api (trigger_by, trigger_type);

-- COLLECTIONS
CREATE TABLE collections (
  id BLOB NOT NULL PRIMARY KEY,
  owner_id BLOB NOT NULL,
  name TEXT NOT NULL,
  FOREIGN KEY (owner_id) REFERENCES workspaces (id) ON DELETE CASCADE
);

CREATE INDEX Idx1 ON collections (owner_id);

/*
 *
 * ITEM FOLDER
 *
 */
-- ITEM FOLDER BASE TABLE
CREATE TABLE item_folder (
  id BLOB NOT NULL PRIMARY KEY,
  collection_id BLOB NOT NULL,
  parent_id BLOB,
  name TEXT NOT NULL,
  prev BLOB,
  next BLOB,
  FOREIGN KEY (collection_id) REFERENCES collections (id) ON DELETE CASCADE,
  FOREIGN KEY (parent_id) REFERENCES item_folder (id) ON DELETE CASCADE
);

CREATE INDEX Idx3 ON item_folder (collection_id, parent_id);

/*
 * 
 * ITEM API
 *
 */
-- ITEM API
CREATE TABLE item_api (
  id BLOB NOT NULL PRIMARY KEY,
  collection_id BLOB NOT NULL,
  parent_id BLOB,
  name TEXT NOT NULL,
  url TEXT NOT NULL,
  method TEXT NOT NULL,
  prev BLOB,
  next BLOB,
  FOREIGN KEY (collection_id) REFERENCES collections (id) ON DELETE CASCADE,
  FOREIGN KEY (parent_id) REFERENCES item_folder (id) ON DELETE CASCADE
);

CREATE INDEX Idx2 ON item_api (collection_id, parent_id);

/*
 *
 * ITEM API EXAMPLE
 *
 */
CREATE TABLE item_api_example (
  id BLOB NOT NULL PRIMARY KEY,
  item_api_id BLOB NOT NULL,
  collection_id BLOB NOT NULL,
  parent_example_id BLOB,
  is_default BOOLEAN NOT NULL DEFAULT FALSE,
  body_type INT8 NOT NULL DEFAULT 0,
  name TEXT NOT NULL,
  prev BLOB,
  next BLOB,
  UNIQUE (prev, next),
  FOREIGN KEY (parent_example_id) REFERENCES item_api_example (id) ON DELETE CASCADE,
  FOREIGN KEY (item_api_id) REFERENCES item_api (id) ON DELETE CASCADE,
  FOREIGN KEY (collection_id) REFERENCES collections (id) ON DELETE CASCADE
);

CREATE INDEX item_api_example_idx1 ON item_api_example (
  item_api_id,
  collection_id,
  is_default,
  parent_example_id
);

CREATE TABLE example_header (
  id BLOB NOT NULL PRIMARY KEY,
  example_id BLOB NOT NULL,
  header_key TEXT NOT NULL,
  enable BOOLEAN NOT NULL DEFAULT TRUE,
  description TEXT NOT NULL,
  value TEXT NOT NULL,
  FOREIGN KEY (example_id) REFERENCES item_api_example (id) ON DELETE CASCADE
);

CREATE INDEX example_header_idx1 ON example_header (
  example_id,
  header_key
);

CREATE TABLE example_query (
  id BLOB NOT NULL PRIMARY KEY,
  example_id BLOB NOT NULL,
  query_key TEXT NOT NULL,
  enable BOOLEAN NOT NULL DEFAULT TRUE,
  description TEXT NOT NULL,
  value TEXT NOT NULL,
  FOREIGN KEY (example_id) REFERENCES item_api_example (id) ON DELETE CASCADE
);

CREATE INDEX example_query_idx1 ON example_query (
  example_id,
  query_key
);

CREATE TABLE example_body_form (
  id BLOB NOT NULL PRIMARY KEY,
  example_id BLOB NOT NULL,
  body_key TEXT NOT NULL,
  enable BOOLEAN NOT NULL DEFAULT TRUE,
  description TEXT NOT NULL,
  value TEXT NOT NULL,
  FOREIGN KEY (example_id) REFERENCES item_api_example (id) ON DELETE CASCADE
);

CREATE INDEX example_body_form_idx1 ON example_body_form (
  example_id,
  body_key
);

CREATE TABLE example_body_urlencoded (
  id BLOB NOT NULL PRIMARY KEY,
  example_id BLOB NOT NULL,
  body_key TEXT NOT NULL,
  enable BOOLEAN NOT NULL DEFAULT TRUE,
  description TEXT NOT NULL,
  value TEXT NOT NULL,
  FOREIGN KEY (example_id) REFERENCES item_api_example (id) ON DELETE CASCADE
);

CREATE INDEX example_body_urlencoded_idx1 ON example_body_urlencoded (
  example_id,
  body_key
);

CREATE TABLE example_body_raw (
  id BLOB NOT NULL PRIMARY KEY,
  example_id BLOB NOT NULL,
  visualize_mode INT8 NOT NULL,
  compress_type INT8 NOT NULL,
  data BLOB,
  UNIQUE (example_id),
  FOREIGN KEY (example_id) REFERENCES item_api_example (id) ON DELETE CASCADE
);

CREATE TABLE environment (
  id BLOB NOT NULL PRIMARY KEY,
  workspace_id BLOB NOT NULL,
  is_default BOOLEAN NOT NULL DEFAULT FALSE,
  name TEXT NOT NULL,
  FOREIGN KEY (workspace_id) REFERENCES workspaces (id) ON DELETE CASCADE
);

CREATE TABLE variable (
    id BLOB NOT NULL PRIMARY KEY,
    env_id BLOB NOT NULL,
    var_key TEXT NOT NULL,
    value TEXt NOT NULL,
    UNIQUE (env_id, var_key),
    FOREIGN KEY (env_id) REFERENCES environment(id) ON DELETE CASCADE
)
