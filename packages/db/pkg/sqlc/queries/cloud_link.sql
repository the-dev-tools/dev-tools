--
-- Cloud links
--

-- name: GetCloudLink :one
SELECT
  file_id, workspace_id, kind, api_url, cloud_workspace_slug, repo_slug, path,
  base_sha, local_dir, dirty, source_text, clean_yaml, updated_at
FROM cloud_link
WHERE file_id = ?
LIMIT 1;

-- name: GetCloudLinksByWorkspaceID :many
SELECT
  file_id, workspace_id, kind, api_url, cloud_workspace_slug, repo_slug, path,
  base_sha, local_dir, dirty, source_text, clean_yaml, updated_at
FROM cloud_link
WHERE workspace_id = ?
ORDER BY api_url, cloud_workspace_slug, repo_slug, path;

-- name: GetCloudLinkByTarget :one
SELECT
  file_id, workspace_id, kind, api_url, cloud_workspace_slug, repo_slug, path,
  base_sha, local_dir, dirty, source_text, clean_yaml, updated_at
FROM cloud_link
WHERE workspace_id = ? AND api_url = ? AND cloud_workspace_slug = ? AND repo_slug = ? AND path = ?
LIMIT 1;

-- name: GetCloudLinksByRepo :many
SELECT
  file_id, workspace_id, kind, api_url, cloud_workspace_slug, repo_slug, path,
  base_sha, local_dir, dirty, source_text, clean_yaml, updated_at
FROM cloud_link
WHERE workspace_id = ? AND api_url = ? AND cloud_workspace_slug = ? AND repo_slug = ?
ORDER BY path;

-- name: CreateCloudLink :exec
INSERT INTO cloud_link (
  file_id, workspace_id, kind, api_url, cloud_workspace_slug, repo_slug, path,
  base_sha, local_dir, dirty, source_text, clean_yaml, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateCloudLinkContent :exec
UPDATE cloud_link
SET base_sha = ?, dirty = ?, source_text = ?, clean_yaml = ?, updated_at = ?
WHERE file_id = ?;

-- name: UpdateCloudLinkLocalDir :exec
UPDATE cloud_link
SET local_dir = ?, updated_at = ?
WHERE file_id = ?;

-- name: DeleteCloudLink :exec
DELETE FROM cloud_link
WHERE file_id = ?;
