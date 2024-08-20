-- name: Get :one
SELECT id, owner_id, name
FROM collections
WHERE id = ?
LIMIT 1;


-- name: GetByPlatformIDandType :many
SELECT id, owner_id, name
FROM collections
WHERE id = ?;


-- name: GetOwnerID :one
SELECT owner_id
FROM collections
WHERE id = ? LIMIT 1;

-- name: GetByOwnerID :many
SELECT id, owner_id, name
FROM collections
WHERE owner_id = ?;


-- name: Create :one
INSERT INTO collections (id, owner_id, name)
VALUES (?, ?, ?) RETURNING id, owner_id, name;

-- name: Update :exec
UPDATE collections 
SET owner_id = ?, name = ?
WHERE id = ?;

-- name: Delete :exec
DELETE FROM collections 
WHERE id = ?;
