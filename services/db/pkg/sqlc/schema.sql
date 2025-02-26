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
  updated BIGINT NOT NULL DEFAULT (unixepoch()),
  collection_count INT NOT NULL DEFAULT 0,
  flow_count INT NOT NULL DEFAULT 0,
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

CREATE INDEX result_api_idx1 ON result_api (trigger_by, trigger_type);

-- COLLECTIONS
CREATE TABLE collections (
  id BLOB NOT NULL PRIMARY KEY,
  owner_id BLOB NOT NULL,
  name TEXT NOT NULL,
  FOREIGN KEY (owner_id) REFERENCES workspaces (id) ON DELETE CASCADE
);

CREATE INDEX collection_idx1 ON collections (owner_id);

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
  UNIQUE (prev, next),
  FOREIGN KEY (collection_id) REFERENCES collections (id) ON DELETE CASCADE,
  FOREIGN KEY (parent_id) REFERENCES item_folder (id) ON DELETE CASCADE
);

CREATE INDEX item_folder_idx1 ON item_folder (collection_id, parent_id);

/*
 *
 * ITEM API
 *
 */
-- ITEM API
CREATE TABLE item_api (
  id BLOB NOT NULL PRIMARY KEY,
  collection_id BLOB NOT NULL,
  folder_id BLOB,
  name TEXT NOT NULL,
  url TEXT NOT NULL,
  method TEXT NOT NULL,

  -- versioning
  version_parent_id BLOB DEFAULT NULL,

  -- ordering
  prev BLOB,
  next BLOB,
  UNIQUE (prev, next),
  FOREIGN KEY (collection_id) REFERENCES collections (id) ON DELETE CASCADE,
  FOREIGN KEY (folder_id) REFERENCES item_folder (id) ON DELETE CASCADE,
  FOREIGN KEY (version_parent_id) REFERENCES item_api (id) ON DELETE CASCADE
);

CREATE INDEX item_api_idx1 ON item_api (collection_id, folder_id, version_parent_id);

/*
 *
 * ITEM API EXAMPLE
 *
 */
CREATE TABLE item_api_example (
  id BLOB NOT NULL PRIMARY KEY,
  item_api_id BLOB NOT NULL,
  collection_id BLOB NOT NULL,
  is_default BOOLEAN NOT NULL DEFAULT FALSE,
  body_type INT8 NOT NULL DEFAULT 0,
  name TEXT NOT NULL,

  -- versioning
  version_parent_id BLOB DEFAULT NULL,

  -- ordering
  prev BLOB,
  next BLOB,
  UNIQUE (prev, next),
  FOREIGN KEY (item_api_id) REFERENCES item_api (id) ON DELETE CASCADE,
  FOREIGN KEY (collection_id) REFERENCES collections (id) ON DELETE CASCADE,
  FOREIGN KEY (version_parent_id) REFERENCES item_api_example (id) ON DELETE CASCADE
);

CREATE INDEX item_api_example_idx1 ON item_api_example (
  item_api_id,
  collection_id,
  is_default,
  version_parent_id
);

CREATE TABLE example_resp (
  id BLOB NOT NULL PRIMARY KEY,
  example_id BLOB NOT NULL,
  status TINYINT NOT NULL DEFAULT 200,
  body BLOB,
  body_compress_type INT8 NOT NULL DEFAULT FALSE,
  duration INT NOT NULL,
  UNIQUE (example_id),
  FOREIGN KEY (example_id) REFERENCES item_api_example (id) ON DELETE CASCADE
);

CREATE INDEX item_api_example_resp_idx1 ON example_resp (
  example_id
);

CREATE TABLE example_header (
  id BLOB NOT NULL PRIMARY KEY,
  example_id BLOB NOT NULL,
  delta_parent_id BLOB DEFAULT NULL,
  header_key TEXT NOT NULL,
  enable BOOLEAN NOT NULL DEFAULT TRUE,
  description TEXT NOT NULL,
  value TEXT NOT NULL,
  FOREIGN KEY (example_id) REFERENCES item_api_example (id) ON DELETE CASCADE,
  FOREIGN KEY (delta_parent_id) REFERENCES example_header (id) ON DELETE CASCADE
);

CREATE INDEX example_header_idx1 ON example_header (
  example_id,
  header_key
);

CREATE TABLE example_resp_header (
  id BLOB NOT NULL PRIMARY KEY,
  example_resp_id BLOB NOT NULL,
  header_key TEXT NOT NULL,
  value TEXT NOT NULL,
  FOREIGN KEY (example_resp_id) REFERENCES example_resp (id) ON DELETE CASCADE
);

CREATE INDEX example_header_resp_idx1 ON example_resp_header (
  example_resp_id,
  header_key
);

CREATE TABLE example_query (
  id BLOB NOT NULL PRIMARY KEY,
  example_id BLOB NOT NULL,
  delta_parent_id BLOB DEFAULT NULL,
  query_key TEXT NOT NULL,
  enable BOOLEAN NOT NULL DEFAULT TRUE,
  description TEXT NOT NULL,
  value TEXT NOT NULL,
  FOREIGN KEY (example_id) REFERENCES item_api_example (id) ON DELETE CASCADE,
  FOREIGN KEY (delta_parent_id) REFERENCES example_query (id) ON DELETE CASCADE
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

-- TODO: env shouldn't active field it should be in workspace
CREATE TABLE environment (
  id BLOB NOT NULL PRIMARY KEY,
  workspace_id BLOB NOT NULL,
  active BOOLEAN NOT NULL DEFAULT FALSE,
  type INT8 NOT NULL,
  name TEXT NOT NULL,
  description TEXT NOT NULL,
  FOREIGN KEY (workspace_id) REFERENCES workspaces (id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX unique_active_environment_idx1
ON environment (workspace_id)
WHERE active = true;


CREATE TABLE variable (
    id BLOB NOT NULL PRIMARY KEY,
    env_id BLOB NOT NULL,
    var_key TEXT NOT NULL,
    value TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    description TEXT NOT NULL,
    UNIQUE (env_id, var_key),
    FOREIGN KEY (env_id) REFERENCES environment(id) ON DELETE CASCADE
);

CREATE TABLE assertion (
  id BLOB NOT NULL PRIMARY KEY,
  example_id BLOB NOT NULL,
  delta_parent_id BLOB DEFAULT NULL,
  type INT8 NOT NULL,
  path TEXT NOT NULL,
  value TEXT NOT NULL,
  enable BOOLEAN NOT NULL DEFAULT TRUE,
  prev BLOB,
  next BLOB,
  FOREIGN KEY (example_id) REFERENCES item_api_example (id) ON DELETE CASCADE,
  FOREIGN KEY (delta_parent_id) REFERENCES assertion (id) ON DELETE CASCADE
);

CREATE TABLE assertion_result (
  id BLOB NOT NULL PRIMARY KEY,
  response_id BLOB NOT NULL,
  assertion_id BLOB NOT NULL,
  result BOOLEAN NOT NULL,
  FOREIGN KEY (response_id) REFERENCES example_resp (id) ON DELETE CASCADE,
  FOREIGN KEY (assertion_id) REFERENCES assertion (id) ON DELETE CASCADE
);

CREATE TABLE flow (
  id BLOB NOT NULL PRIMARY KEY,
  workspace_id BLOB NOT NULL,
  version_parent_id BLOB DEFAULT NULL,
  name TEXT NOT NULL,
  FOREIGN KEY (workspace_id) REFERENCES workspaces (id) ON DELETE CASCADE,
  FOREIGN KEY (version_parent_id) REFERENCES flow (id) ON DELETE CASCADE
);

CREATE index flow_idx1 ON flow (workspace_id, workspace_id, version_parent_id);

CREATE TABLE tag (
  id BLOB NOT NULL PRIMARY KEY,
  workspace_id BLOB NOT NULL,
  name TEXT NOT NULL,
  color INT8 NOT NULL,
  FOREIGN KEY (workspace_id) REFERENCES workspaces (id) ON DELETE CASCADE
);

CREATE TABLE flow_tag (
  id BLOB NOT NULL PRIMARY KEY,
  flow_id BLOB NOT NULL,
  tag_id BLOB NOT NULL,
  FOREIGN KEY (flow_id) REFERENCES flow (id) ON DELETE CASCADE,
  FOREIGN KEY (tag_id) REFERENCES tag (id) ON DELETE CASCADE
);

CREATE TABLE flow_node (
  id BLOB NOT NULL PRIMARY KEY,
  flow_id BLOB NOT NULL,
  name TEXT NOT NULL,
  node_kind INT NOT NULL,
  position_x REAL NOT NULL,
  position_y REAL NOT NULL,
  FOREIGN KEY (flow_id) REFERENCES flow (id) ON DELETE CASCADE
);

CREATE TABLE flow_edge (
  id BLOB NOT NULL PRIMARY KEY,
  flow_id BLOB NOT NULL,
  source_id BLOB NOT NULL,
  target_id BLOB NOT NULL,
  source_handle INT NOT NULL,
  FOREIGN KEY (flow_id) REFERENCES flow (id) ON DELETE CASCADE,
  FOREIGN KEY (source_id) REFERENCES flow_node (id) ON DELETE CASCADE,
  FOREIGN KEY (target_id) REFERENCES flow_node (id) ON DELETE CASCADE
);


-- TODO: move conditions to new condition table
CREATE TABLE flow_node_for (
  flow_node_id BLOB NOT NULL PRIMARY KEY,
  iter_count BIGINT NOT NULL,
  error_handling INT8 NOT NULL,
  condition_path TEXT NOT NULL,
  condition_type INT8 NOT NULL,
  value text NOT NULL,
  FOREIGN KEY (flow_node_id) REFERENCES flow_node (id) ON DELETE CASCADE
);

-- TODO: move conditions to new condition table
CREATE TABLE flow_node_for_each (
  flow_node_id BLOB NOT NULL PRIMARY KEY,
  iter_path TEXT NOT NULL,
  error_handling INT8 NOT NULL,
  condition_path TEXT NOT NULL,
  condition_type INT8 NOT NULL,
  value text NOT NULL,
  FOREIGN KEY (flow_node_id) REFERENCES flow_node (id) ON DELETE CASCADE
);

CREATE TABLE flow_node_request (
  flow_node_id BLOB NOT NULL PRIMARY KEY,
  endpoint_id BLOB,
  example_id BLOB,
  delta_example_id BLOB,
  FOREIGN KEY (flow_node_id) REFERENCES flow_node (id) ON DELETE CASCADE,
  FOREIGN KEY (endpoint_id) REFERENCES item_api (id) ON DELETE SET NULL,
  FOREIGN KEY (example_id) REFERENCES item_api_example (id) ON DELETE SET NULL,
  FOREIGN KEY (delta_example_id) REFERENCES item_api_example (id) ON DELETE SET NULL
);

-- TODO: move conditions to new condition table
CREATE TABLE flow_node_if (
  flow_node_id BLOB NOT NULL PRIMARY KEY,
  condition_type INT8 NOT NULL,
  path TEXT NOT NULL,
  value text NOT NULL,
  FOREIGN KEY (flow_node_id) REFERENCES flow_node (id) ON DELETE CASCADE
);

CREATE TABLE flow_node_noop (
  flow_node_id BLOB NOT NULL PRIMARY KEY,
  node_type TINYINT NOT NULL,
  FOREIGN KEY (flow_node_id) REFERENCES flow_node (id) ON DELETE CASCADE
);

CREATE TABLE migration (
  id BLOB NOT NULL PRIMARY KEY,
  version INT NOT NULL,
  description TEXT NOT NULL,
  apply_at BIGINT NOT NULL
);
