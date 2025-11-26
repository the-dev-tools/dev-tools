/*
* Environment
*/

-- name: GetEnvironment :one
SELECT
  id,
  workspace_id,
  type,
  name,
  description,
  display_order
FROM
  environment
WHERE
  id = ?
LIMIT 1;

-- name: GetEnvironmentsByWorkspaceID :many
SELECT
  id,
  workspace_id,
  type,
  name,
  description,
  display_order
FROM
  environment
WHERE
  workspace_id = ?
ORDER BY
  display_order;

-- name: CreateEnvironment :exec
INSERT INTO
  environment (id, workspace_id, type, name, description, display_order)
VALUES
  (?, ?, ?, ?, ?, ?);

-- name: UpdateEnvironment :exec
UPDATE environment
SET
    type = ?,
    name = ?,
    description = ?,
    display_order = ?
WHERE
    id = ?;

-- name: DeleteEnvironment :exec
DELETE FROM environment
WHERE
  id = ?;

-- name: GetEnvironmentsByWorkspaceIDOrdered :many
SELECT
  id,
  workspace_id,
  type,
  name,
  description,
  display_order
FROM
  environment
WHERE
  workspace_id = ?
ORDER BY
  display_order;

-- name: GetEnvironmentWorkspaceID :one
SELECT
  workspace_id
FROM
  environment
WHERE
  id = ?
LIMIT
  1;

/*
* Variables
*/

-- name: GetVariable :one
SELECT
  id,
  env_id,
  var_key,
  value,
  enabled,
  description,
  display_order
FROM
  variable
WHERE
  id = ?
LIMIT 1;

-- name: GetVariablesByEnvironmentID :many
SELECT
  id,
  env_id,
  var_key,
  value,
  enabled,
  description,
  display_order
FROM
  variable
WHERE
  env_id = ?
ORDER BY
  display_order;

-- name: GetVariablesByEnvironmentIDOrdered :many
SELECT
  id,
  env_id,
  var_key,
  value,
  enabled,
  description,
  display_order
FROM
  variable
WHERE
  env_id = ?
ORDER BY
  display_order;

-- name: CreateVariable :exec
INSERT INTO
  variable (id, env_id, var_key, value, enabled, description, display_order)
VALUES
  (?, ?, ?, ?, ?, ?, ?);

-- name: CreateVariableBulk :exec
INSERT INTO
  variable (id, env_id, var_key, value, enabled, description, display_order)
VALUES
  (?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?);

-- name: UpdateVariable :exec
UPDATE variable
SET
  var_key = ?,
  value = ?,
  enabled = ?,
  description = ?,
  display_order = ?
WHERE
  id = ?;

-- name: DeleteVariable :exec
DELETE FROM variable
WHERE
  id = ?;
