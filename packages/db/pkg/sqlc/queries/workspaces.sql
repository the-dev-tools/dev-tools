--
-- Workspaces
--
-- name: GetWorkspace :one
SELECT
  id,
  name,
  updated,
  collection_count,
  flow_count,
  active_env,
  global_env,
  display_order,
  sync_path,
  sync_format,
  sync_enabled
FROM
  workspaces
WHERE
  id = ?
LIMIT
  1;

-- name: GetWorkspaceByUserID :one
SELECT
  id,
  name,
  updated,
  collection_count,
  flow_count,
  active_env,
  global_env,
  display_order,
  sync_path,
  sync_format,
  sync_enabled
FROM
  workspaces
WHERE
  id = (
    SELECT
      workspace_id
    FROM
      workspaces_users
    WHERE
      user_id = ?
    LIMIT
      1
  )
LIMIT
  1;

-- name: GetWorkspacesByUserID :many
SELECT
  id,
  name,
  updated,
  collection_count,
  flow_count,
  active_env,
  global_env,
  display_order,
  sync_path,
  sync_format,
  sync_enabled
FROM
  workspaces
WHERE
  id IN (
    SELECT
      workspace_id
    FROM
      workspaces_users
    WHERE
      user_id = ?
  );

-- name: GetWorkspaceByUserIDandWorkspaceID :one
SELECT
  id,
  name,
  updated,
  collection_count,
  flow_count,
  active_env,
  global_env,
  display_order,
  sync_path,
  sync_format,
  sync_enabled
FROM
  workspaces
WHERE
  id = (
    SELECT
      workspace_id
    FROM
      workspaces_users
    WHERE
      workspace_id = ?
      AND user_id = ?
    LIMIT
      1
  )
LIMIT
  1;

-- name: CreateWorkspace :exec
INSERT INTO
  workspaces (id, name, updated, collection_count, flow_count, active_env, global_env, display_order, sync_path, sync_format, sync_enabled)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateWorkspace :exec
UPDATE workspaces
SET
  name = ?,
  collection_count = ?,
  flow_count = ?,
  updated = ?,
  active_env = ?,
  display_order = ?
WHERE
  id = ?;

-- name: UpdateWorkspaceUpdatedTime :exec
UPDATE workspaces
SET
  updated = ?
WHERE
  id = ?;

-- name: UpdateWorkspaceSync :exec
UPDATE workspaces
SET
  sync_path = ?,
  sync_format = ?,
  sync_enabled = ?,
  updated = ?
WHERE
  id = ?;

-- name: DeleteWorkspace :exec
DELETE FROM workspaces
WHERE
  id = ?;

-- name: GetWorkspacesByUserIDOrdered :many
SELECT
  w.id,
  w.name,
  w.updated,
  w.collection_count,
  w.flow_count,
  w.active_env,
  w.global_env,
  w.display_order,
  w.sync_path,
  w.sync_format,
  w.sync_enabled
FROM
  workspaces w
INNER JOIN workspaces_users wu ON w.id = wu.workspace_id
WHERE
  wu.user_id = ?
ORDER BY
  w.display_order ASC;

-- name: GetAllWorkspacesByUserID :many
-- Returns ALL workspaces for a user
SELECT
  w.id,
  w.name,
  w.updated,
  w.collection_count,
  w.flow_count,
  w.active_env,
  w.global_env,
  w.display_order,
  w.sync_path,
  w.sync_format,
  w.sync_enabled
FROM
  workspaces w
INNER JOIN workspaces_users wu ON w.id = wu.workspace_id
WHERE
  wu.user_id = ?
ORDER BY
  w.updated DESC;

-- name: GetSyncedWorkspaces :many
-- Returns all workspaces with sync enabled
SELECT
  id,
  name,
  updated,
  collection_count,
  flow_count,
  active_env,
  global_env,
  display_order,
  sync_path,
  sync_format,
  sync_enabled
FROM
  workspaces
WHERE
  sync_enabled = 1;

--
-- WorkspaceUsers
--
-- name: CheckIFWorkspaceUserExists :one
SELECT
  cast(
  EXISTS (
    SELECT
      1
    FROM
      workspaces_users
    WHERE
      workspace_id = ?
      AND user_id = ?
    LIMIT
      1
) AS boolean
);

-- name: GetWorkspaceUser :one
SELECT
  id,
  workspace_id,
  user_id,
  role
FROM
  workspaces_users
WHERE
  id = ?
LIMIT
  1;

-- name: GetWorkspaceUserByUserID :many
SELECT
  id,
  workspace_id,
  user_id,
  role
FROM
  workspaces_users
WHERE
  user_id = ?;

-- name: GetWorkspaceUserByWorkspaceID :many
SELECT
  id,
  workspace_id,
  user_id,
  role
FROM
  workspaces_users
WHERE
  workspace_id = ?;

-- name: GetWorkspaceUserByWorkspaceIDAndUserID :one
SELECT
  id,
  workspace_id,
  user_id,
  role
FROM
  workspaces_users
WHERE
  workspace_id = ?
  AND user_id = ?
LIMIT
  1;

-- name: CreateWorkspaceUser :exec
INSERT INTO
  workspaces_users (id, workspace_id, user_id, role)
VALUES
  (?, ?, ?, ?);

-- name: UpdateWorkspaceUser :exec
UPDATE workspaces_users
SET
  workspace_id = ?,
  user_id = ?,
  role = ?
WHERE
  id = ?;

-- name: DeleteWorkspaceUser :exec
DELETE FROM workspaces_users
WHERE
  id = ?;
