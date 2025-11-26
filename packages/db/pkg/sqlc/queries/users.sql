--
-- Users
--
-- name: GetUser :one
SELECT
  id,
  email,
  password_hash,
  provider_type,
  provider_id
FROM
  users
WHERE
  id = ?
LIMIT
  1;

-- name: GetUserByEmail :one
SELECT
  id,
  email,
  password_hash,
  provider_type,
  provider_id
FROM
  users
WHERE
  email = ?
LIMIT
  1;

-- name: GetUserByEmailAndProviderType :one
SELECT
  id,
  email,
  password_hash,
  provider_type,
  provider_id
FROM
  users
WHERE
  email = ?
  AND provider_type = ?
LIMIT
  1;

-- name: GetUserByProviderIDandType :one
SELECT
  id,
  email,
  password_hash,
  provider_type,
  provider_id
FROM
  users
WHERE
  provider_id = ?
  AND provider_type = ?
LIMIT
  1;

-- name: CreateUser :exec
INSERT INTO
  users (
    id,
    email,
    password_hash,
    provider_type,
    provider_id
  )
VALUES
  (?, ?, ?, ?, ?);

-- name: UpdateUser :exec
UPDATE users
SET
  email = ?,
  password_hash = ?
WHERE
  id = ?;

-- name: DeleteUser :exec
DELETE FROM users
WHERE
  id = ?;
