-- name: Get :one
SELECT
    id,
    email,
    password_hash,
    platform_type,
    platform_id
FROM users WHERE id = ? LIMIT 1;


-- name: GetByPlatformIDandType :one
SELECT
    id,
    email,
    password_hash,
    platform_type,
    platform_id
FROM users WHERE platform_id = ? AND platform_type = ? LIMIT 1;

-- name: Create :one
INSERT INTO users 
(id, email, password_hash, platform_type, platform_id)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: Update :exec
UPDATE users 
SET email = ?,
password_hash = ?
WHERE id = ?;

-- name: Delete :exec
DELETE FROM users 
WHERE id = ?;
