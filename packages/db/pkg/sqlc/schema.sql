-- USERS
CREATE TABLE users (
  id BLOB NOT NULL PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  password_hash BLOB,
  provider_type INT8 NOT NULL DEFAULT 0,
  provider_id TEXT,
  status INT8 NOT NULL DEFAULT 0,
  UNIQUE (provider_type, provider_id)
);

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

-- COLLECTIONS
CREATE TABLE collections (
  id BLOB NOT NULL PRIMARY KEY,
  workspace_id BLOB NOT NULL,
  name TEXT NOT NULL,
  prev BLOB,
  next BLOB,
  UNIQUE (prev, next, workspace_id),
  FOREIGN KEY (workspace_id) REFERENCES workspaces (id) ON DELETE CASCADE
);

CREATE INDEX collection_idx1 ON collections (workspace_id);

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
  UNIQUE (prev, next, parent_id),
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

  -- delta endpoint parent (null for regular endpoints, references parent for delta endpoints)
  delta_parent_id BLOB DEFAULT NULL,

  -- visibility
  hidden BOOLEAN NOT NULL DEFAULT FALSE,

  -- ordering
  prev BLOB,
  next BLOB,
  FOREIGN KEY (collection_id) REFERENCES collections (id) ON DELETE CASCADE,
  FOREIGN KEY (folder_id) REFERENCES item_folder (id) ON DELETE CASCADE,
  FOREIGN KEY (version_parent_id) REFERENCES item_api (id) ON DELETE CASCADE,
  FOREIGN KEY (delta_parent_id) REFERENCES item_api (id) ON DELETE CASCADE
);

CREATE INDEX item_api_idx1 ON item_api (
  collection_id,
  folder_id,
  version_parent_id,
  delta_parent_id
);

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
  FOREIGN KEY (example_id) REFERENCES item_api_example (id) ON DELETE CASCADE
);

CREATE INDEX item_api_example_resp_idx1 ON example_resp (
  example_id
);

--
-- Overlay tables for URL-encoded bodies (Delta Overlay Option A)
--

-- Ordered overlay sequence mixing origin-backed refs and delta-only refs
-- ref_kind: 1 = origin-ref (references example_body_urlencoded.id), 2 = delta-ref (references delta_urlenc_delta.id)
CREATE TABLE delta_urlenc_order (
  example_id BLOB NOT NULL,
  ref_kind TINYINT NOT NULL,
  ref_id BLOB NOT NULL,
  rank TEXT NOT NULL,
  revision BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (example_id, ref_kind, ref_id),
  FOREIGN KEY (example_id) REFERENCES item_api_example (id) ON DELETE CASCADE
);

CREATE INDEX delta_urlenc_order_idx1 ON delta_urlenc_order (
  example_id,
  rank
);

-- State (patch + tombstone) per origin-backed item within a delta example
-- Nullable columns act as field overrides. suppressed=true hides the origin from the delta view.
CREATE TABLE delta_urlenc_state (
  example_id BLOB NOT NULL,
  origin_id BLOB NOT NULL,
  suppressed BOOLEAN NOT NULL DEFAULT FALSE,
  body_key TEXT NULL,
  value TEXT NULL,
  description TEXT NULL,
  enabled BOOLEAN NULL,
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),
  PRIMARY KEY (example_id, origin_id),
  FOREIGN KEY (example_id) REFERENCES item_api_example (id) ON DELETE CASCADE,
  FOREIGN KEY (origin_id) REFERENCES example_body_urlencoded (id) ON DELETE CASCADE
);

CREATE INDEX delta_urlenc_state_idx1 ON delta_urlenc_state (
  example_id,
  origin_id
);

CREATE INDEX delta_urlenc_state_supp_idx ON delta_urlenc_state (
  example_id
) WHERE suppressed = TRUE;

-- Delta-only rows (no origin). Values are stored typed for query clarity and future indexing.
CREATE TABLE delta_urlenc_delta (
  example_id BLOB NOT NULL,
  id BLOB NOT NULL PRIMARY KEY,
  body_key TEXT NOT NULL DEFAULT '',
  value TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),
  FOREIGN KEY (example_id) REFERENCES item_api_example (id) ON DELETE CASCADE
);

CREATE INDEX delta_urlenc_delta_idx1 ON delta_urlenc_delta (
  example_id,
  id
);

--
-- Overlay tables for Headers
--
CREATE TABLE delta_header_order (
  example_id BLOB NOT NULL,
  ref_kind TINYINT NOT NULL,
  ref_id BLOB NOT NULL,
  rank TEXT NOT NULL,
  revision BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (example_id, ref_kind, ref_id),
  FOREIGN KEY (example_id) REFERENCES item_api_example (id) ON DELETE CASCADE
);
CREATE INDEX delta_header_order_idx1 ON delta_header_order (example_id, rank);

CREATE TABLE delta_header_state (
  example_id BLOB NOT NULL,
  origin_id BLOB NOT NULL,
  suppressed BOOLEAN NOT NULL DEFAULT FALSE,
  header_key TEXT NULL,
  value TEXT NULL,
  description TEXT NULL,
  enabled BOOLEAN NULL,
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),
  PRIMARY KEY (example_id, origin_id),
  FOREIGN KEY (example_id) REFERENCES item_api_example (id) ON DELETE CASCADE,
  FOREIGN KEY (origin_id) REFERENCES example_header (id) ON DELETE CASCADE
);
CREATE INDEX delta_header_state_idx1 ON delta_header_state (example_id, origin_id);
CREATE INDEX delta_header_state_supp_idx ON delta_header_state (example_id) WHERE suppressed = TRUE;

CREATE TABLE delta_header_delta (
  example_id BLOB NOT NULL,
  id BLOB NOT NULL PRIMARY KEY,
  header_key TEXT NOT NULL DEFAULT '',
  value TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),
  FOREIGN KEY (example_id) REFERENCES item_api_example (id) ON DELETE CASCADE
);
CREATE INDEX delta_header_delta_idx1 ON delta_header_delta (example_id, id);

-- Overlay tables for Queries
CREATE TABLE delta_query_order (
  example_id BLOB NOT NULL,
  ref_kind TINYINT NOT NULL,
  ref_id BLOB NOT NULL,
  rank TEXT NOT NULL,
  revision BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (example_id, ref_kind, ref_id),
  FOREIGN KEY (example_id) REFERENCES item_api_example (id) ON DELETE CASCADE
);
CREATE INDEX delta_query_order_idx1 ON delta_query_order (example_id, rank);

CREATE TABLE delta_query_state (
  example_id BLOB NOT NULL,
  origin_id BLOB NOT NULL,
  suppressed BOOLEAN NOT NULL DEFAULT FALSE,
  query_key TEXT NULL,
  value TEXT NULL,
  description TEXT NULL,
  enabled BOOLEAN NULL,
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),
  PRIMARY KEY (example_id, origin_id),
  FOREIGN KEY (example_id) REFERENCES item_api_example (id) ON DELETE CASCADE,
  FOREIGN KEY (origin_id) REFERENCES example_query (id) ON DELETE CASCADE
);
CREATE INDEX delta_query_state_idx1 ON delta_query_state (example_id, origin_id);
CREATE INDEX delta_query_state_supp_idx ON delta_query_state (example_id) WHERE suppressed = TRUE;

CREATE TABLE delta_query_delta (
  example_id BLOB NOT NULL,
  id BLOB NOT NULL PRIMARY KEY,
  query_key TEXT NOT NULL DEFAULT '',
  value TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),
  FOREIGN KEY (example_id) REFERENCES item_api_example (id) ON DELETE CASCADE
);
CREATE INDEX delta_query_delta_idx1 ON delta_query_delta (example_id, id);

-- Overlay tables for Form Bodies
CREATE TABLE delta_form_order (
  example_id BLOB NOT NULL,
  ref_kind TINYINT NOT NULL,
  ref_id BLOB NOT NULL,
  rank TEXT NOT NULL,
  revision BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (example_id, ref_kind, ref_id),
  FOREIGN KEY (example_id) REFERENCES item_api_example (id) ON DELETE CASCADE
);
CREATE INDEX delta_form_order_idx1 ON delta_form_order (example_id, rank);

CREATE TABLE delta_form_state (
  example_id BLOB NOT NULL,
  origin_id BLOB NOT NULL,
  suppressed BOOLEAN NOT NULL DEFAULT FALSE,
  body_key TEXT NULL,
  value TEXT NULL,
  description TEXT NULL,
  enabled BOOLEAN NULL,
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),
  PRIMARY KEY (example_id, origin_id),
  FOREIGN KEY (example_id) REFERENCES item_api_example (id) ON DELETE CASCADE,
  FOREIGN KEY (origin_id) REFERENCES example_body_form (id) ON DELETE CASCADE
);
CREATE INDEX delta_form_state_idx1 ON delta_form_state (example_id, origin_id);
CREATE INDEX delta_form_state_supp_idx ON delta_form_state (example_id) WHERE suppressed = TRUE;

CREATE TABLE delta_form_delta (
  example_id BLOB NOT NULL,
  id BLOB NOT NULL PRIMARY KEY,
  body_key TEXT NOT NULL DEFAULT '',
  value TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),
  FOREIGN KEY (example_id) REFERENCES item_api_example (id) ON DELETE CASCADE
);
CREATE INDEX delta_form_delta_idx1 ON delta_form_delta (example_id, id);

CREATE TABLE example_header (
  id BLOB NOT NULL PRIMARY KEY,
  example_id BLOB NOT NULL,
  delta_parent_id BLOB DEFAULT NULL,
  header_key TEXT NOT NULL,
  enable BOOLEAN NOT NULL DEFAULT TRUE,
  description TEXT NOT NULL,
  value TEXT NOT NULL,
  prev BLOB,
  next BLOB,
  FOREIGN KEY (example_id) REFERENCES item_api_example (id) ON DELETE CASCADE,
  FOREIGN KEY (delta_parent_id) REFERENCES example_header (id) ON DELETE CASCADE,
  FOREIGN KEY (prev) REFERENCES example_header (id) ON DELETE SET NULL,
  FOREIGN KEY (next) REFERENCES example_header (id) ON DELETE SET NULL
);

CREATE INDEX example_header_idx1 ON example_header (
  example_id,
  header_key,
  delta_parent_id
);

-- Linked list indexes for header ordering optimization
CREATE INDEX example_header_linked_list_idx ON example_header (
  example_id,
  prev,
  next
);

CREATE INDEX example_header_prev_idx ON example_header (
  prev
);

CREATE INDEX example_header_next_idx ON example_header (
  next
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
  delta_parent_id,
  query_key
);

CREATE TABLE example_body_form (
  id BLOB NOT NULL PRIMARY KEY,
  example_id BLOB NOT NULL,
  delta_parent_id BLOB DEFAULT NULL,
  body_key TEXT NOT NULL,
  enable BOOLEAN NOT NULL DEFAULT TRUE,
  description TEXT NOT NULL,
  value TEXT NOT NULL,
  FOREIGN KEY (example_id) REFERENCES item_api_example (id) ON DELETE CASCADE,
  FOREIGN KEY (delta_parent_id) REFERENCES example_body_form (id) ON DELETE CASCADE
);

CREATE INDEX example_body_form_idx1 ON example_body_form (
  example_id,
  delta_parent_id,
  body_key
);

CREATE TABLE example_body_urlencoded (
  id BLOB NOT NULL PRIMARY KEY,
  example_id BLOB NOT NULL,
  delta_parent_id BLOB DEFAULT NULL,
  body_key TEXT NOT NULL,
  enable BOOLEAN NOT NULL DEFAULT TRUE,
  description TEXT NOT NULL,
  value TEXT NOT NULL,
  -- Linked list pointers for stable ordering within an example
  prev BLOB DEFAULT NULL,
  next BLOB DEFAULT NULL,
  FOREIGN KEY (example_id) REFERENCES item_api_example (id) ON DELETE CASCADE,
  FOREIGN KEY (delta_parent_id) REFERENCES example_body_urlencoded (id) ON DELETE CASCADE
);

CREATE INDEX example_body_urlencoded_idx1 ON example_body_urlencoded (
  example_id,
  delta_parent_id,
  body_key
);

-- Linked list indexes for URL-encoded ordering optimization
CREATE INDEX example_body_urlencoded_linked_list_idx ON example_body_urlencoded (
  example_id,
  prev,
  next
);
CREATE INDEX example_body_urlencoded_prev_idx ON example_body_urlencoded (
  example_id,
  prev
);
CREATE INDEX example_body_urlencoded_next_idx ON example_body_urlencoded (
  example_id,
  next
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
  type INT8 NOT NULL,
  name TEXT NOT NULL,
  description TEXT NOT NULL,
  prev BLOB,
  next BLOB,
  UNIQUE (prev, next, workspace_id),
  FOREIGN KEY (workspace_id) REFERENCES workspaces (id) ON DELETE CASCADE
);

CREATE INDEX environment_idx1 ON environment (workspace_id, type, name);

-- Performance indexes for environment ordering operations
CREATE INDEX environment_ordering ON environment (workspace_id, prev, next);
CREATE INDEX environment_workspace_lookup ON environment (id, workspace_id);

CREATE TABLE variable (
    id BLOB NOT NULL PRIMARY KEY,
    env_id BLOB NOT NULL,
    var_key TEXT NOT NULL,
    value TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    description TEXT NOT NULL,
    prev BLOB,
    next BLOB,
    UNIQUE (env_id, var_key),
    UNIQUE (prev, next, env_id),
    FOREIGN KEY (env_id) REFERENCES environment(id) ON DELETE CASCADE
);

CREATE INDEX variable_idx1 ON variable (env_id, var_key);

-- Performance indexes for variable ordering operations
CREATE INDEX variable_ordering ON variable (env_id, prev, next);

CREATE TABLE assertion (
  id BLOB NOT NULL PRIMARY KEY,
  example_id BLOB NOT NULL,
  delta_parent_id BLOB DEFAULT NULL,
  expression TEXT NOT NULL,
  enable BOOLEAN NOT NULL DEFAULT TRUE,
  prev BLOB,
  next BLOB,
  FOREIGN KEY (example_id) REFERENCES item_api_example (id) ON DELETE CASCADE,
  FOREIGN KEY (delta_parent_id) REFERENCES assertion (id) ON DELETE CASCADE
);

CREATE INDEX assertion_idx1 ON assertion (
  example_id,
  expression
);

CREATE TABLE assertion_result (
  id BLOB NOT NULL PRIMARY KEY,
  response_id BLOB NOT NULL,
  assertion_id BLOB NOT NULL,
  result BOOLEAN NOT NULL,
  FOREIGN KEY (response_id) REFERENCES example_resp (id) ON DELETE CASCADE,
  FOREIGN KEY (assertion_id) REFERENCES assertion (id) ON DELETE CASCADE
);

CREATE INDEX assertion_result_idx1 ON assertion_result (
  response_id,
  assertion_id
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

CREATE INDEX tag_idx1 ON tag (workspace_id);

CREATE TABLE flow_tag (
  id BLOB NOT NULL PRIMARY KEY,
  flow_id BLOB NOT NULL,
  tag_id BLOB NOT NULL,
  FOREIGN KEY (flow_id) REFERENCES flow (id) ON DELETE CASCADE,
  FOREIGN KEY (tag_id) REFERENCES tag (id) ON DELETE CASCADE
);

CREATE INDEX flow_tag_idx1 ON flow_tag (flow_id, tag_id);

CREATE TABLE flow_node (
  id BLOB NOT NULL PRIMARY KEY,
  flow_id BLOB NOT NULL,
  name TEXT NOT NULL,
  node_kind INT NOT NULL,
  position_x REAL NOT NULL,
  position_y REAL NOT NULL,
  FOREIGN KEY (flow_id) REFERENCES flow (id) ON DELETE CASCADE
);

CREATE INDEX flow_node_idx1 ON flow_node (flow_id);

CREATE TABLE flow_edge (
  id BLOB NOT NULL PRIMARY KEY,
  flow_id BLOB NOT NULL,
  source_id BLOB NOT NULL,
  target_id BLOB NOT NULL,
  source_handle INT NOT NULL,
  edge_kind INT NOT NULL DEFAULT 0,
  FOREIGN KEY (flow_id) REFERENCES flow (id) ON DELETE CASCADE,
  FOREIGN KEY (source_id) REFERENCES flow_node (id) ON DELETE CASCADE,
  FOREIGN KEY (target_id) REFERENCES flow_node (id) ON DELETE CASCADE
);

CREATE INDEX flow_edge_idx1 ON flow_edge (flow_id, source_id, target_id);


-- TODO: move conditions to new condition table
CREATE TABLE flow_node_for (
  flow_node_id BLOB NOT NULL PRIMARY KEY,
  iter_count BIGINT NOT NULL,
  error_handling INT8 NOT NULL,
  expression TEXT NOT NULL,
  FOREIGN KEY (flow_node_id) REFERENCES flow_node (id) ON DELETE CASCADE
);

-- TODO: move conditions to new condition table
CREATE TABLE flow_node_for_each (
  flow_node_id BLOB NOT NULL PRIMARY KEY,
  iter_expression TEXT NOT NULL,
  error_handling INT8 NOT NULL,
  expression TEXT NOT NULL,
  FOREIGN KEY (flow_node_id) REFERENCES flow_node (id) ON DELETE CASCADE
);

CREATE TABLE flow_node_request (
  flow_node_id BLOB NOT NULL PRIMARY KEY,
  endpoint_id BLOB,
  example_id BLOB,
  delta_example_id BLOB,
  delta_endpoint_id BLOB,
  has_request_config BOOLEAN NOT NULL DEFAULT FALSE,
  FOREIGN KEY (flow_node_id) REFERENCES flow_node (id) ON DELETE CASCADE,
  FOREIGN KEY (endpoint_id) REFERENCES item_api (id) ON DELETE SET NULL,
  FOREIGN KEY (example_id) REFERENCES item_api_example (id) ON DELETE SET NULL,
  FOREIGN KEY (delta_example_id) REFERENCES item_api_example (id) ON DELETE SET NULL,
  FOREIGN KEY (delta_endpoint_id) REFERENCES item_api (id) ON DELETE SET NULL
);

-- TODO: move conditions to new condition table
CREATE TABLE flow_node_condition (
  flow_node_id BLOB NOT NULL PRIMARY KEY,
  expression TEXT NOT NULL,
  FOREIGN KEY (flow_node_id) REFERENCES flow_node (id) ON DELETE CASCADE
);

CREATE TABLE flow_node_noop (
  flow_node_id BLOB NOT NULL PRIMARY KEY,
  node_type TINYINT NOT NULL,
  FOREIGN KEY (flow_node_id) REFERENCES flow_node (id) ON DELETE CASCADE
);

CREATE TABLE flow_node_js (
  flow_node_id BLOB NOT NULL PRIMARY KEY,
  code BLOB NOT NULL,
  code_compress_type INT8 NOT NULL,
  FOREIGN KEY (flow_node_id) REFERENCES flow_node (id) ON DELETE CASCADE
);

CREATE TABLE migration (
  id BLOB NOT NULL PRIMARY KEY,
  version INT NOT NULL,
  description TEXT NOT NULL,
  apply_at BIGINT NOT NULL
);

CREATE TABLE flow_variable (
  id BLOB NOT NULL PRIMARY KEY,
  flow_id BLOB NOT NULL,
  key TEXT NOT NULL,
  value TEXT NOT NULL,
  enabled BOOL NOT NULL,
  description TEXT NOT NULL,
  prev BLOB,
  next BLOB,
  UNIQUE (flow_id, key),
  UNIQUE (prev, next, flow_id),
  FOREIGN KEY (flow_id) REFERENCES flow (id) ON DELETE CASCADE
);

-- Performance indexes for flow variable ordering operations
CREATE INDEX flow_variable_ordering ON flow_variable (flow_id, prev, next);

CREATE TABLE node_execution (
  id BLOB NOT NULL PRIMARY KEY,
  node_id BLOB NOT NULL,
  name TEXT NOT NULL,
  state INT8 NOT NULL,
  error TEXT,
  -- Keep existing compression fields as-is
  input_data BLOB,  -- Compressed JSON
  input_data_compress_type INT8 NOT NULL DEFAULT 0,
  output_data BLOB, -- Compressed JSON
  output_data_compress_type INT8 NOT NULL DEFAULT 0,
  -- Add new fields
  response_id BLOB, -- Response ID for REQUEST nodes (NULL for non-request nodes)
  completed_at BIGINT, -- Unix timestamp in milliseconds
  FOREIGN KEY (node_id) REFERENCES flow_node (id) ON DELETE CASCADE
);

CREATE INDEX node_execution_idx1 ON node_execution (node_id);
CREATE INDEX node_execution_idx2 ON node_execution (completed_at DESC);
CREATE INDEX node_execution_idx3 ON node_execution (state);

/*
 * UNIFIED COLLECTION ITEMS TABLE
 * Enables mixed folder/endpoint ordering with drag-and-drop functionality
 */
CREATE TABLE collection_items (
  id BLOB NOT NULL PRIMARY KEY,
  collection_id BLOB NOT NULL,
  parent_folder_id BLOB,
  item_type INT8 NOT NULL,                         -- 0 = folder, 1 = endpoint
  folder_id BLOB,
  endpoint_id BLOB,
  name TEXT NOT NULL,
  prev_id BLOB,
  next_id BLOB,
  CHECK (item_type IN (0, 1)),
  CHECK (
    (item_type = 0 AND folder_id IS NOT NULL AND endpoint_id IS NULL) OR
    (item_type = 1 AND folder_id IS NULL AND endpoint_id IS NOT NULL)
  ),
  FOREIGN KEY (collection_id) REFERENCES collections (id) ON DELETE CASCADE,
  FOREIGN KEY (parent_folder_id) REFERENCES collection_items (id) ON DELETE CASCADE,
  FOREIGN KEY (folder_id) REFERENCES item_folder (id) ON DELETE CASCADE,
  FOREIGN KEY (endpoint_id) REFERENCES item_api (id) ON DELETE CASCADE,
  UNIQUE (prev_id, next_id, parent_folder_id, collection_id)
);

CREATE INDEX collection_items_idx1 ON collection_items (
  collection_id, 
  parent_folder_id, 
  item_type
);

CREATE INDEX collection_items_idx2 ON collection_items (
  folder_id
) WHERE folder_id IS NOT NULL;

CREATE INDEX collection_items_idx3 ON collection_items (
  endpoint_id  
) WHERE endpoint_id IS NOT NULL;

CREATE INDEX collection_items_idx4 ON collection_items (
  prev_id, 
  next_id
);

CREATE INDEX collection_items_idx5 ON collection_items (
  collection_id,
  name
);

-- Performance indexes for cross-collection move operations
-- Optimize workspace validation queries for collections
CREATE INDEX collections_workspace_lookup ON collections (id, workspace_id);

-- Optimize cross-collection validation queries for collection items  
CREATE INDEX collection_items_workspace_lookup ON collection_items (id, collection_id);

-- Optimize user access validation for cross-collection moves
CREATE INDEX workspaces_users_collection_access ON workspaces_users (user_id, workspace_id);

-- Optimize collection-workspace JOIN operations in cross-collection queries
CREATE INDEX collections_workspace_id_lookup ON collections (workspace_id, id);

-- Optimize legacy table updates for cross-collection moves
CREATE INDEX item_api_collection_update ON item_api (id, collection_id);
CREATE INDEX item_folder_collection_update ON item_folder (id, collection_id);
