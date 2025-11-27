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
  parent_id BLOB,
  content_id BLOB,
  content_kind INT8 NOT NULL DEFAULT 0,
  name TEXT NOT NULL,
  display_order REAL NOT NULL DEFAULT 0,
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),
  CHECK (length (id) == 16),
  CHECK (content_kind IN (0, 1, 2, 3)), -- 0 = item_folder, 1 = item_api, 2 = flow, 3 = http_delta
  CHECK (
    (content_kind = 0 AND content_id IS NOT NULL) OR
    (content_kind = 1 AND content_id IS NOT NULL) OR
    (content_kind = 2 AND content_id IS NOT NULL) OR
    (content_kind = 3 AND content_id IS NOT NULL) OR
    (content_id IS NULL)
  ),
  FOREIGN KEY (workspace_id) REFERENCES workspaces (id) ON DELETE CASCADE,
  FOREIGN KEY (parent_id) REFERENCES files (id) ON DELETE SET NULL
);

-- Performance indexes for file system operations
CREATE INDEX files_workspace_idx ON files (workspace_id);

CREATE INDEX files_hierarchy_idx ON files (
  workspace_id,
  parent_id,
  display_order
);

CREATE INDEX files_content_lookup_idx ON files (
  content_kind,
  content_id
) WHERE content_id IS NOT NULL;

CREATE INDEX files_parent_lookup_idx ON files (
  parent_id,
  display_order
) WHERE parent_id IS NOT NULL;

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
  parent_id,
  content_kind,
  display_order
);
