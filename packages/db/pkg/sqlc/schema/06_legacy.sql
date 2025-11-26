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
