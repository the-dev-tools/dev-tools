-- name: CreateHttpVersion :exec
INSERT INTO http_version (
  id, http_id, version_name, version_description, is_active, created_at, created_by
)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetHttpVersionsByHttpID :many
SELECT id, http_id, version_name, version_description, is_active, created_at, created_by
FROM http_version
WHERE http_id = ?
ORDER BY created_at DESC;