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



/*
 *
 * ITEM FOLDER
 *
 */
-- ITEM FOLDER BASE TABLE
CREATE TABLE item_folder (
  id BLOB NOT NULL PRIMARY KEY,
  parent_id BLOB,
  name TEXT NOT NULL,
  prev BLOB,
  next BLOB,
  UNIQUE (prev, next, parent_id),
  FOREIGN KEY (parent_id) REFERENCES item_folder (id) ON DELETE CASCADE
);

CREATE INDEX item_folder_idx1 ON item_folder (parent_id);

/*
 *
 * ITEM API
 *
 */
-- ITEM API
CREATE TABLE item_api (
  id BLOB NOT NULL PRIMARY KEY,
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
  FOREIGN KEY (folder_id) REFERENCES item_folder (id) ON DELETE CASCADE,
  FOREIGN KEY (version_parent_id) REFERENCES item_api (id) ON DELETE CASCADE,
  FOREIGN KEY (delta_parent_id) REFERENCES item_api (id) ON DELETE CASCADE
);

CREATE INDEX item_api_idx1 ON item_api (
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
  is_default BOOLEAN NOT NULL DEFAULT FALSE,
  body_type INT8 NOT NULL DEFAULT 0,
  name TEXT NOT NULL,

  -- versioning
  version_parent_id BLOB DEFAULT NULL,

  -- ordering
  prev BLOB,
  next BLOB,
  FOREIGN KEY (item_api_id) REFERENCES item_api (id) ON DELETE CASCADE,
  FOREIGN KEY (version_parent_id) REFERENCES item_api_example (id) ON DELETE CASCADE
);

CREATE INDEX item_api_example_idx1 ON item_api_example (
  item_api_id,
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
  display_order REAL NOT NULL DEFAULT 0,
  FOREIGN KEY (workspace_id) REFERENCES workspaces (id) ON DELETE CASCADE
);

CREATE INDEX environment_idx1 ON environment (workspace_id, type, name);
CREATE INDEX environment_workspace_lookup ON environment (id, workspace_id);
CREATE INDEX environment_order_idx ON environment (workspace_id, display_order);

CREATE TABLE variable (
    id BLOB NOT NULL PRIMARY KEY,
    env_id BLOB NOT NULL,
    var_key TEXT NOT NULL,
    value TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    description TEXT NOT NULL,
    display_order REAL NOT NULL DEFAULT 0,
    UNIQUE (env_id, var_key),
    FOREIGN KEY (env_id) REFERENCES environment(id) ON DELETE CASCADE
);

CREATE INDEX variable_idx1 ON variable (env_id, var_key);
CREATE INDEX variable_order_idx ON variable (env_id, display_order);

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
  duration INT NOT NULL DEFAULT 0,
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

CREATE TABLE flow_node_http (
  flow_node_id BLOB NOT NULL PRIMARY KEY,
  http_id BLOB NOT NULL,
  FOREIGN KEY (flow_node_id) REFERENCES flow_node (id) ON DELETE CASCADE,
  FOREIGN KEY (http_id) REFERENCES http (id) ON DELETE CASCADE
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
  http_response_id BLOB, -- Response ID for HTTP request nodes (NULL for non-request nodes)
  completed_at BIGINT, -- Unix timestamp in milliseconds
  FOREIGN KEY (node_id) REFERENCES flow_node (id) ON DELETE CASCADE,
  FOREIGN KEY (http_response_id) REFERENCES http_response (id) ON DELETE SET NULL
);

CREATE INDEX node_execution_idx1 ON node_execution (node_id);
CREATE INDEX node_execution_idx2 ON node_execution (completed_at DESC);
CREATE INDEX node_execution_idx3 ON node_execution (state);



/*
 *
 * FILES
 *
 */
-- FILES TABLE
-- Union type approach using content_id + content_kind for flexible file system
CREATE TABLE files (
  id BLOB NOT NULL PRIMARY KEY,
  workspace_id BLOB NOT NULL,
  folder_id BLOB,
  content_id BLOB,
  content_kind INT8 NOT NULL DEFAULT 0,
  name TEXT NOT NULL,
  display_order REAL NOT NULL DEFAULT 0,
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),
  CHECK (length (id) == 16),
  CHECK (content_kind IN (0, 1, 2)), -- 0 = item_folder, 1 = item_api, 2 = flow
  CHECK (
    (content_kind = 0 AND content_id IS NOT NULL) OR
    (content_kind = 1 AND content_id IS NOT NULL) OR
    (content_kind = 2 AND content_id IS NOT NULL) OR
    (content_id IS NULL)
  ),
  FOREIGN KEY (workspace_id) REFERENCES workspaces (id) ON DELETE CASCADE,
  FOREIGN KEY (folder_id) REFERENCES files (id) ON DELETE SET NULL
);

-- Performance indexes for file system operations
CREATE INDEX files_workspace_idx ON files (workspace_id);

CREATE INDEX files_hierarchy_idx ON files (
  workspace_id,
  folder_id,
  display_order
);

CREATE INDEX files_content_lookup_idx ON files (
  content_kind,
  content_id
) WHERE content_id IS NOT NULL;

CREATE INDEX files_parent_lookup_idx ON files (
  folder_id,
  display_order
) WHERE folder_id IS NOT NULL;

CREATE INDEX files_name_search_idx ON files (
  workspace_id,
  name
);

CREATE INDEX files_kind_filter_idx ON files (
  workspace_id,
  content_kind
);

-- Composite index for common file system queries
CREATE INDEX files_workspace_hierarchy_idx ON files (
  workspace_id,
  folder_id,
  content_kind,
  display_order
);

/*
 *
 * UNIFIED HTTP SYSTEM
 * Single-table approach with delta fields for Phase 1 HTTP implementation
 *
 */

-- Core HTTP table with workspace/folder relationships
CREATE TABLE http (
  id BLOB NOT NULL PRIMARY KEY,
  workspace_id BLOB NOT NULL,
  folder_id BLOB,
  name TEXT NOT NULL,
  url TEXT NOT NULL,
  method TEXT NOT NULL,
  body_kind INT8 NOT NULL DEFAULT 2,
  description TEXT NOT NULL DEFAULT '',
  
  -- Delta system fields
  parent_http_id BLOB DEFAULT NULL,        -- Parent HTTP for delta records
  is_delta BOOLEAN NOT NULL DEFAULT FALSE, -- TRUE for delta records
  
  -- Delta override fields (NULL means "no change" for delta records)
  delta_name TEXT NULL,
  delta_url TEXT NULL,
  delta_method TEXT NULL,
  delta_body_kind INT8 NULL,
  delta_description TEXT NULL,
  
  -- Metadata
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),
  
  -- Foreign keys
  FOREIGN KEY (workspace_id) REFERENCES workspaces (id) ON DELETE CASCADE,
  FOREIGN KEY (folder_id) REFERENCES files (id) ON DELETE SET NULL,
  FOREIGN KEY (parent_http_id) REFERENCES http (id) ON DELETE CASCADE,
  
  -- Constraints
  CHECK (is_delta = FALSE OR parent_http_id IS NOT NULL) -- Delta records must have a parent
);

-- Performance indexes for HTTP table
CREATE INDEX http_workspace_idx ON http (workspace_id);
CREATE INDEX http_folder_idx ON http (folder_id) WHERE folder_id IS NOT NULL;
CREATE INDEX http_parent_delta_idx ON http (parent_http_id, is_delta);
CREATE INDEX http_workspace_name_idx ON http (workspace_id, name);
CREATE INDEX http_method_idx ON http (method);

-- HTTP search parameters (query strings)
CREATE TABLE http_search_param (
  id BLOB NOT NULL PRIMARY KEY,
  http_id BLOB NOT NULL,
  key TEXT NOT NULL,
  value TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  description TEXT NOT NULL DEFAULT '',
  "order" REAL NOT NULL DEFAULT 0,
  
  -- Delta relationship fields
  parent_http_search_param_id BLOB,
  is_delta BOOLEAN NOT NULL DEFAULT FALSE,
  
  -- Delta fields (NULL means "no change" for delta records)
  delta_key TEXT,
  delta_value TEXT,
  delta_enabled BOOLEAN,
  delta_description TEXT,
  delta_order REAL,
  
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),
  
  FOREIGN KEY (http_id) REFERENCES http (id) ON DELETE CASCADE,
  FOREIGN KEY (parent_http_search_param_id) REFERENCES http_search_param (id) ON DELETE CASCADE
);

-- Performance indexes
CREATE INDEX http_search_param_http_idx ON http_search_param (http_id);
CREATE INDEX http_search_param_key_idx ON http_search_param (http_id, key);
CREATE INDEX http_search_param_order_idx ON http_search_param (http_id, "order");
CREATE INDEX http_search_param_delta_idx ON http_search_param (parent_http_search_param_id) WHERE is_delta = TRUE;

-- HTTP headers
CREATE TABLE http_header (
  id BLOB NOT NULL PRIMARY KEY,
  http_id BLOB NOT NULL,
  header_key TEXT NOT NULL,
  header_value TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  
  -- Delta system fields
  parent_header_id BLOB DEFAULT NULL,
  is_delta BOOLEAN NOT NULL DEFAULT FALSE,
  
  -- Delta override fields
  delta_header_key TEXT NULL,
  delta_header_value TEXT NULL,
  delta_description TEXT NULL,
  delta_enabled BOOLEAN NULL,
  
  -- Ordering
  prev BLOB DEFAULT NULL,
  next BLOB DEFAULT NULL,
  
  -- Metadata
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),
  
  -- Foreign keys
  FOREIGN KEY (http_id) REFERENCES http (id) ON DELETE CASCADE,
  FOREIGN KEY (parent_header_id) REFERENCES http_header (id) ON DELETE CASCADE,
  FOREIGN KEY (prev) REFERENCES http_header (id) ON DELETE SET NULL,
  FOREIGN KEY (next) REFERENCES http_header (id) ON DELETE SET NULL,
  
  -- Constraints
  CHECK (is_delta = FALSE OR parent_header_id IS NOT NULL)
);

-- Indexes for headers
CREATE INDEX http_header_http_idx ON http_header (http_id);
CREATE INDEX http_header_parent_delta_idx ON http_header (parent_header_id, is_delta);
CREATE INDEX http_header_ordering_idx ON http_header (http_id, prev, next);
CREATE INDEX http_header_key_idx ON http_header (header_key);

-- Streaming performance indexes for headers
CREATE INDEX http_header_streaming_idx ON http_header (http_id, enabled, updated_at DESC) WHERE enabled = TRUE;
CREATE INDEX http_header_delta_streaming_idx ON http_header (parent_header_id, is_delta, updated_at DESC);

-- HTTP body form data
CREATE TABLE http_body_form (
  id BLOB NOT NULL PRIMARY KEY,
  http_id BLOB NOT NULL,
  key TEXT NOT NULL,
  value TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  description TEXT NOT NULL DEFAULT '',
  "order" REAL NOT NULL DEFAULT 0,
  
  -- Delta relationship fields
  parent_http_body_form_id BLOB,
  is_delta BOOLEAN NOT NULL DEFAULT FALSE,
  
  -- Delta fields (NULL means "no change" for delta records)
  delta_key TEXT,
  delta_value TEXT,
  delta_enabled BOOLEAN,
  delta_description TEXT,
  delta_order REAL,
  
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),
  
  FOREIGN KEY (http_id) REFERENCES http (id) ON DELETE CASCADE,
  FOREIGN KEY (parent_http_body_form_id) REFERENCES http_body_form (id) ON DELETE CASCADE
);

-- Performance indexes
CREATE INDEX http_body_form_http_idx ON http_body_form (http_id);
CREATE INDEX http_body_form_key_idx ON http_body_form (http_id, key);
CREATE INDEX http_body_form_order_idx ON http_body_form (http_id, "order");
CREATE INDEX http_body_form_delta_idx ON http_body_form (parent_http_body_form_id) WHERE is_delta = TRUE;



-- HTTP body raw data
CREATE TABLE http_body_raw (
  id BLOB NOT NULL PRIMARY KEY,
  http_id BLOB NOT NULL,
  raw_data BLOB,
  content_type TEXT NOT NULL DEFAULT '',
  compression_type INT8 NOT NULL DEFAULT 0, -- 0 = none, 1 = gzip, etc.
  
  -- Delta system fields
  parent_body_raw_id BLOB DEFAULT NULL,
  is_delta BOOLEAN NOT NULL DEFAULT FALSE,
  
  -- Delta override fields
  delta_raw_data BLOB NULL,
  delta_content_type TEXT NULL,
  delta_compression_type INT8 NULL,
  
  -- Metadata
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),
  
  -- Foreign keys
  FOREIGN KEY (http_id) REFERENCES http (id) ON DELETE CASCADE,
  FOREIGN KEY (parent_body_raw_id) REFERENCES http_body_raw (id) ON DELETE CASCADE,
  
  -- Constraints
  CHECK (is_delta = FALSE OR parent_body_raw_id IS NOT NULL),
  UNIQUE (http_id) -- One raw body per HTTP request
);

-- Indexes for raw body
CREATE INDEX http_body_raw_http_idx ON http_body_raw (http_id);
CREATE INDEX http_body_raw_parent_delta_idx ON http_body_raw (parent_body_raw_id, is_delta);

-- Streaming performance indexes for raw body
CREATE INDEX http_body_raw_streaming_idx ON http_body_raw (http_id, updated_at DESC);
CREATE INDEX http_body_raw_delta_streaming_idx ON http_body_raw (parent_body_raw_id, is_delta, updated_at DESC);

-- HTTP version management
CREATE TABLE http_version (
  id BLOB NOT NULL PRIMARY KEY,
  http_id BLOB NOT NULL,
  version_name TEXT NOT NULL,
  version_description TEXT NOT NULL DEFAULT '',
  is_active BOOLEAN NOT NULL DEFAULT FALSE,
  
  -- Metadata
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),
  created_by BLOB, -- User ID who created this version
  
  -- Foreign keys
  FOREIGN KEY (http_id) REFERENCES http (id) ON DELETE CASCADE,
  FOREIGN KEY (created_by) REFERENCES users (id) ON DELETE SET NULL,
  
  -- Constraints
  UNIQUE (http_id, version_name)
);

-- Indexes for versions
CREATE INDEX http_version_http_idx ON http_version (http_id);
CREATE INDEX http_version_active_idx ON http_version (is_active) WHERE is_active = TRUE;
CREATE INDEX http_version_created_by_idx ON http_version (created_by);



-- HTTP assertions (TypeSpec-compliant)
-- Single-table approach with delta fields for HttpAssert entities
CREATE TABLE http_assert (
  id BLOB NOT NULL PRIMARY KEY,
  http_id BLOB NOT NULL,
  key TEXT NOT NULL,
  value TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  description TEXT NOT NULL DEFAULT '',
  "order" REAL NOT NULL DEFAULT 0,
  
  -- Delta relationship fields
  parent_http_assert_id BLOB,
  is_delta BOOLEAN NOT NULL DEFAULT FALSE,
  
  -- Delta fields (NULL means "no change" for delta records)
  delta_key TEXT,
  delta_value TEXT,
  delta_enabled BOOLEAN,
  delta_description TEXT,
  delta_order REAL,
  
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),
  
  FOREIGN KEY (http_id) REFERENCES http (id) ON DELETE CASCADE,
  FOREIGN KEY (parent_http_assert_id) REFERENCES http_assert (id) ON DELETE CASCADE,
  
  -- Constraints
  CHECK (is_delta = FALSE OR parent_http_assert_id IS NOT NULL) -- Delta records must have a parent
);

-- Performance indexes for HttpAssert
CREATE INDEX http_assert_http_idx ON http_assert (http_id);
CREATE INDEX http_assert_key_idx ON http_assert (http_id, key);
CREATE INDEX http_assert_order_idx ON http_assert (http_id, "order");
CREATE INDEX http_assert_delta_idx ON http_assert (parent_http_assert_id) WHERE is_delta = TRUE;

-- Performance indexes for workspace-scoped access patterns
CREATE INDEX http_workspace_access_idx ON http (workspace_id, folder_id, is_delta);
CREATE INDEX http_workspace_streaming_idx ON http (workspace_id, updated_at DESC);

-- Critical streaming performance indexes (Priority 1 optimizations)
-- Optimizes real-time streaming queries and delta resolution
CREATE INDEX http_delta_resolution_idx ON http (parent_http_id, is_delta, updated_at DESC);
CREATE INDEX http_workspace_method_streaming_idx ON http (workspace_id, method, updated_at DESC);

-- Partial indexes for streaming performance (only index non-delta records for streaming)
CREATE INDEX http_active_streaming_idx ON http (workspace_id, updated_at DESC) WHERE is_delta = FALSE;

-- Composite indexes for common query patterns
CREATE INDEX http_workspace_method_idx ON http (workspace_id, method);
CREATE INDEX http_folder_method_idx ON http (folder_id, method) WHERE folder_id IS NOT NULL;

/*
 *
 * HTTP BODY URL-ENCODED (TypeSpec-compliant)
 * Single-table approach with delta fields for HttpBodyUrlEncoded entities
 *
 */

-- HttpBodyUrlEncoded table following TypeSpec specification
CREATE TABLE http_body_urlencoded (
  id BLOB NOT NULL PRIMARY KEY,
  http_id BLOB NOT NULL,
  key TEXT NOT NULL,
  value TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  description TEXT NOT NULL DEFAULT '',
  "order" REAL NOT NULL DEFAULT 0,
  
  -- Delta relationship fields
  parent_http_body_urlencoded_id BLOB,
  is_delta BOOLEAN NOT NULL DEFAULT FALSE,
  
  -- Delta fields (NULL means "no change" for delta records)
  delta_key TEXT,
  delta_value TEXT,
  delta_enabled BOOLEAN,
  delta_description TEXT,
  delta_order REAL,
  
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),
  
  FOREIGN KEY (http_id) REFERENCES http (id) ON DELETE CASCADE,
  FOREIGN KEY (parent_http_body_urlencoded_id) REFERENCES http_body_urlencoded (id) ON DELETE CASCADE,
  
  -- Constraints
  CHECK (is_delta = FALSE OR parent_http_body_urlencoded_id IS NOT NULL) -- Delta records must have a parent
);

-- Performance indexes for HttpBodyUrlEncoded
CREATE INDEX http_body_urlencoded_http_idx ON http_body_urlencoded (http_id);
CREATE INDEX http_body_urlencoded_key_idx ON http_body_urlencoded (http_id, key);
CREATE INDEX http_body_urlencoded_order_idx ON http_body_urlencoded (http_id, "order");
CREATE INDEX http_body_urlencoded_delta_idx ON http_body_urlencoded (parent_http_body_urlencoded_id) WHERE is_delta = TRUE;

/*
 *
 * HTTP RESPONSE (TypeSpec-compliant)
 * Read-only tables following TypeSpec HttpResponse specification
 *
 */

-- HttpResponse table (read-only, no delta fields)
CREATE TABLE http_response (
  id BLOB NOT NULL PRIMARY KEY,
  http_id BLOB NOT NULL,
  status INT32 NOT NULL,
  body BLOB,
  time DATETIME NOT NULL,
  duration INT32 NOT NULL,
  size INT32 NOT NULL,
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),
  
  FOREIGN KEY (http_id) REFERENCES http (id) ON DELETE CASCADE
);

-- HttpResponseHeader table (read-only, no delta fields)
CREATE TABLE http_response_header (
  id BLOB NOT NULL PRIMARY KEY,
  http_id BLOB NOT NULL,
  key TEXT NOT NULL,
  value TEXT NOT NULL,
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),
  
  FOREIGN KEY (http_id) REFERENCES http (id) ON DELETE CASCADE
);

-- Performance indexes for HttpResponse
CREATE INDEX http_response_http_idx ON http_response (http_id);
CREATE INDEX http_response_time_idx ON http_response (http_id, time DESC);

-- Performance indexes for HttpResponseHeader
CREATE INDEX http_response_header_http_idx ON http_response_header (http_id);
CREATE INDEX http_response_header_key_idx ON http_response_header (http_id, key);

-- HttpResponseAssert table (read-only, no delta fields)
CREATE TABLE http_response_assert (
  id BLOB NOT NULL PRIMARY KEY,
  http_id BLOB NOT NULL,
  value TEXT NOT NULL,
  success BOOLEAN NOT NULL,
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),
  
  FOREIGN KEY (http_id) REFERENCES http (id) ON DELETE CASCADE
);

-- Performance indexes for HttpResponseAssert
CREATE INDEX http_response_assert_http_idx ON http_response_assert (http_id);
CREATE INDEX http_response_assert_success_idx ON http_response_assert (http_id, success);
