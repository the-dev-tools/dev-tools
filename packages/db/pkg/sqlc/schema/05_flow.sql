CREATE TABLE flow (
  id BLOB NOT NULL PRIMARY KEY,
  workspace_id BLOB NOT NULL,
  version_parent_id BLOB DEFAULT NULL,
  name TEXT NOT NULL,
  duration INT NOT NULL DEFAULT 0,
  running BOOLEAN NOT NULL DEFAULT FALSE,
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
  FOREIGN KEY (flow_id) REFERENCES flow (id) ON DELETE CASCADE
);

CREATE INDEX flow_edge_idx1 ON flow_edge (flow_id, source_id, target_id);


-- TODO: move conditions to new condition table
CREATE TABLE flow_node_for (
  flow_node_id BLOB NOT NULL PRIMARY KEY,
  iter_count BIGINT NOT NULL,
  error_handling INT8 NOT NULL,
  expression TEXT NOT NULL
);

-- TODO: move conditions to new condition table
CREATE TABLE flow_node_for_each (
  flow_node_id BLOB NOT NULL PRIMARY KEY,
  iter_expression TEXT NOT NULL,
  error_handling INT8 NOT NULL,
  expression TEXT NOT NULL
);

CREATE TABLE flow_node_http (
  flow_node_id BLOB NOT NULL PRIMARY KEY,
  http_id BLOB NOT NULL,
  delta_http_id BLOB,
  FOREIGN KEY (http_id) REFERENCES http (id) ON DELETE CASCADE,
  FOREIGN KEY (delta_http_id) REFERENCES http (id) ON DELETE SET NULL
);

-- TODO: move conditions to new condition table
CREATE TABLE flow_node_condition (
  flow_node_id BLOB NOT NULL PRIMARY KEY,
  expression TEXT NOT NULL
);

CREATE TABLE flow_node_noop (
  flow_node_id BLOB NOT NULL PRIMARY KEY,
  node_type TINYINT NOT NULL
);

CREATE TABLE flow_node_js (
  flow_node_id BLOB NOT NULL PRIMARY KEY,
  code BLOB NOT NULL,
  code_compress_type INT8 NOT NULL
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
  FOREIGN KEY (http_response_id) REFERENCES http_response (id) ON DELETE SET NULL
);

CREATE INDEX node_execution_idx1 ON node_execution (node_id);
CREATE INDEX node_execution_idx2 ON node_execution (completed_at DESC);
CREATE INDEX node_execution_idx3 ON node_execution (state);
