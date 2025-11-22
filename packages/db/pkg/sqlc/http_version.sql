-- name: CreateHttpVersion :exec
INSERT INTO http_version (
  id, http_id, version_name, version_description, is_active, created_at, created_by
)
VALUES (?, ?, ?, ?, ?, ?, ?);
