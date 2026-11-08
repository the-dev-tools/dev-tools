/*
 *
 * CLOUD LINK
 *
 */
-- A Files row that mirrors something in Stresseur Cloud: a repo (root folder)
-- or one of the repo's flow files (folder under the repo folder that holds the
-- flows imported from that file). Linked rows stay out of the Files tree.
CREATE TABLE cloud_link (
  file_id BLOB NOT NULL PRIMARY KEY,
  workspace_id BLOB NOT NULL,
  kind TEXT NOT NULL, -- 'repo' or 'flow_file'
  api_url TEXT NOT NULL,
  cloud_workspace_slug TEXT NOT NULL,
  repo_slug TEXT NOT NULL,
  path TEXT NOT NULL DEFAULT '', -- repo-relative flow file path, '' for a repo link
  base_sha TEXT NOT NULL DEFAULT '', -- commit the local copy is based on
  local_dir TEXT NOT NULL DEFAULT '', -- linked local clone (repo links)
  dirty BOOLEAN NOT NULL DEFAULT FALSE, -- edited since fetched or last saved
  source_text TEXT NOT NULL DEFAULT '', -- flow file text as fetched or last saved
  clean_yaml TEXT NOT NULL DEFAULT '', -- export of the imported flows right after fetch or save
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),
  CHECK (length (file_id) == 16),
  CHECK (kind IN ('repo', 'flow_file')),
  CHECK ((kind = 'repo' AND path = '') OR (kind = 'flow_file' AND path != '')),
  FOREIGN KEY (workspace_id) REFERENCES workspaces (id) ON DELETE CASCADE,
  FOREIGN KEY (file_id) REFERENCES files (id) ON DELETE CASCADE
);

-- One link per repo and per flow file path in a workspace
CREATE UNIQUE INDEX cloud_link_target_idx ON cloud_link (
  workspace_id,
  api_url,
  cloud_workspace_slug,
  repo_slug,
  path
);
