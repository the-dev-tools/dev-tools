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
  content_hash TEXT,

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
  last_run_at BIGINT NULL,
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
CREATE INDEX http_content_hash_idx ON http (workspace_id, content_hash) WHERE content_hash IS NOT NULL;

-- HTTP search parameters (query strings)
CREATE TABLE http_search_param (
  id BLOB NOT NULL PRIMARY KEY,
  http_id BLOB NOT NULL,
  key TEXT NOT NULL,
  value TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  description TEXT NOT NULL DEFAULT '',
  display_order REAL NOT NULL DEFAULT 0,

  -- Delta relationship fields
  parent_http_search_param_id BLOB,
  is_delta BOOLEAN NOT NULL DEFAULT FALSE,

  -- Delta fields (NULL means "no change" for delta records)
  delta_key TEXT,
  delta_value TEXT,
  delta_enabled BOOLEAN,
  delta_description TEXT,
  delta_display_order REAL,

  created_at BIGINT NOT NULL DEFAULT (unixepoch()),
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),

  FOREIGN KEY (http_id) REFERENCES http (id) ON DELETE CASCADE,
  FOREIGN KEY (parent_http_search_param_id) REFERENCES http_search_param (id) ON DELETE CASCADE
);

-- Performance indexes
CREATE INDEX http_search_param_http_idx ON http_search_param (http_id);
CREATE INDEX http_search_param_key_idx ON http_search_param (http_id, key);
CREATE INDEX http_search_param_order_idx ON http_search_param (http_id, display_order);
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
  delta_display_order REAL,

  -- Ordering
  display_order REAL NOT NULL DEFAULT 0,

  -- Metadata
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),

  -- Foreign keys
  FOREIGN KEY (http_id) REFERENCES http (id) ON DELETE CASCADE,
  FOREIGN KEY (parent_header_id) REFERENCES http_header (id) ON DELETE CASCADE,

  -- Constraints
  CHECK (is_delta = FALSE OR parent_header_id IS NOT NULL)
);

-- Indexes for headers
CREATE INDEX http_header_http_idx ON http_header (http_id);
CREATE INDEX http_header_parent_delta_idx ON http_header (parent_header_id, is_delta);
CREATE INDEX http_header_order_idx ON http_header (http_id, display_order);
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
  display_order REAL NOT NULL DEFAULT 0,

  -- Delta relationship fields
  parent_http_body_form_id BLOB,
  is_delta BOOLEAN NOT NULL DEFAULT FALSE,

  -- Delta fields (NULL means "no change" for delta records)
  delta_key TEXT,
  delta_value TEXT,
  delta_enabled BOOLEAN,
  delta_description TEXT,
  delta_display_order REAL,

  created_at BIGINT NOT NULL DEFAULT (unixepoch()),
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),

  FOREIGN KEY (http_id) REFERENCES http (id) ON DELETE CASCADE,
  FOREIGN KEY (parent_http_body_form_id) REFERENCES http_body_form (id) ON DELETE CASCADE
);

-- Performance indexes
CREATE INDEX http_body_form_http_idx ON http_body_form (http_id);
CREATE INDEX http_body_form_key_idx ON http_body_form (http_id, key);
CREATE INDEX http_body_form_order_idx ON http_body_form (http_id, display_order);
CREATE INDEX http_body_form_delta_idx ON http_body_form (parent_http_body_form_id) WHERE is_delta = TRUE;



-- HTTP body raw data
CREATE TABLE http_body_raw (
  id BLOB NOT NULL PRIMARY KEY,
  http_id BLOB NOT NULL,
  raw_data BLOB,
  compression_type INT8 NOT NULL DEFAULT 0, -- 0 = none, 1 = gzip, etc.

  -- Delta system fields
  parent_body_raw_id BLOB DEFAULT NULL,
  is_delta BOOLEAN NOT NULL DEFAULT FALSE,

  -- Delta override fields
  delta_raw_data BLOB NULL,
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
  value TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  description TEXT NOT NULL DEFAULT '',
  display_order REAL NOT NULL DEFAULT 0,

  -- Delta relationship fields
  parent_http_assert_id BLOB,
  is_delta BOOLEAN NOT NULL DEFAULT FALSE,

  -- Delta fields (NULL means "no change" for delta records)
  delta_value TEXT,
  delta_enabled BOOLEAN,
  delta_description TEXT,
  delta_display_order REAL,

  created_at BIGINT NOT NULL DEFAULT (unixepoch()),
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),

  FOREIGN KEY (http_id) REFERENCES http (id) ON DELETE CASCADE,
  FOREIGN KEY (parent_http_assert_id) REFERENCES http_assert (id) ON DELETE CASCADE,

  -- Constraints
  CHECK (is_delta = FALSE OR parent_http_assert_id IS NOT NULL) -- Delta records must have a parent
);

-- Performance indexes for HttpAssert
CREATE INDEX http_assert_http_idx ON http_assert (http_id);
CREATE INDEX http_assert_order_idx ON http_assert (http_id, display_order);
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
  display_order REAL NOT NULL DEFAULT 0,

  -- Delta relationship fields
  parent_http_body_urlencoded_id BLOB,
  is_delta BOOLEAN NOT NULL DEFAULT FALSE,

  -- Delta fields (NULL means "no change" for delta records)
  delta_key TEXT,
  delta_value TEXT,
  delta_enabled BOOLEAN,
  delta_description TEXT,
  delta_display_order REAL,

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
CREATE INDEX http_body_urlencoded_order_idx ON http_body_urlencoded (http_id, display_order);
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
  response_id BLOB NOT NULL,
  key TEXT NOT NULL,
  value TEXT NOT NULL,
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),

  FOREIGN KEY (response_id) REFERENCES http_response (id) ON DELETE CASCADE
);

-- Performance indexes for HttpResponse
CREATE INDEX http_response_http_idx ON http_response (http_id);
CREATE INDEX http_response_time_idx ON http_response (http_id, time DESC);

-- Performance indexes for HttpResponseHeader
CREATE INDEX http_response_header_response_idx ON http_response_header (response_id);
CREATE INDEX http_response_header_key_idx ON http_response_header (response_id, key);

-- HttpResponseAssert table (read-only, no delta fields)
CREATE TABLE http_response_assert (
  id BLOB NOT NULL PRIMARY KEY,
  response_id BLOB NOT NULL,
  value TEXT NOT NULL,
  success BOOLEAN NOT NULL,
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),

  FOREIGN KEY (response_id) REFERENCES http_response (id) ON DELETE CASCADE
);

-- Performance indexes for HttpResponseAssert
CREATE INDEX http_response_assert_response_idx ON http_response_assert (response_id);
CREATE INDEX http_response_assert_success_idx ON http_response_assert (response_id, success);
