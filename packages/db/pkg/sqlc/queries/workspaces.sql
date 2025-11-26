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
  global_env
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
  global_env
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
  prev,
  next
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
  prev,
  next
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
  workspaces (id, name, updated, collection_count, flow_count, active_env, global_env, prev, next)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateWorkspace :exec
UPDATE workspaces
SET
  name = ?,
  collection_count = ?,
  flow_count = ?,
  updated = ?,
  active_env = ?
WHERE
  id = ?;

-- name: UpdateWorkspaceUpdatedTime :exec
UPDATE workspaces
SET
  updated = ?
WHERE
  id = ?;

-- name: DeleteWorkspace :exec
DELETE FROM workspaces
WHERE
  id = ?;

-- name: GetWorkspacesByUserIDOrdered :many
-- Uses WITH RECURSIVE CTE to traverse linked list from head to tail for user-scoped ordering
-- Each user has their own workspace ordering maintained via workspaces_users table
WITH RECURSIVE ordered_workspaces AS (
  -- Base case: Find the head (prev IS NULL) for this user
  SELECT
    w.id,
    w.name,
    w.updated,
    w.collection_count,
    w.flow_count,
    w.active_env,
    w.global_env,
    w.prev,
    w.next,
    0 as position
  FROM
    workspaces w
  INNER JOIN workspaces_users wu ON w.id = wu.workspace_id
  WHERE
    wu.user_id = ? AND
    w.prev IS NULL
  
  UNION ALL
  
  -- Recursive case: Follow the next pointers
  SELECT
    w.id,
    w.name,
    w.updated,
    w.collection_count,
    w.flow_count,
    w.active_env,
    w.global_env,
    w.prev,
    w.next,
    ow.position + 1
  FROM
    workspaces w
  INNER JOIN workspaces_users wu ON w.id = wu.workspace_id
  INNER JOIN ordered_workspaces ow ON w.prev = ow.id
  WHERE
    wu.user_id = ?
)
SELECT
  ow.id,
  ow.name,
  ow.updated,
  ow.collection_count,
  ow.flow_count,
  ow.active_env,
  ow.global_env,
  ow.prev,
  ow.next,
  ow.position
FROM
  ordered_workspaces ow
ORDER BY
  ow.position;

-- name: GetAllWorkspacesByUserID :many
-- Returns ALL workspaces for a user, including isolated ones (prev=NULL, next=NULL)
-- Unlike GetWorkspacesByUserIDOrdered, this query finds workspaces regardless of linked-list state
-- Essential for finding new workspaces that haven't been linked yet
SELECT
  w.id,
  w.name,
  w.updated,
  w.collection_count,
  w.flow_count,
  w.active_env,
  w.global_env,
  w.prev,
  w.next
FROM
  workspaces w
INNER JOIN workspaces_users wu ON w.id = wu.workspace_id
WHERE
  wu.user_id = ?
ORDER BY
  w.updated DESC;

-- name: UpdateWorkspaceOrder :exec
-- Update the prev/next pointers for a single workspace with user validation
-- Used for moving workspaces within the user's linked list
UPDATE workspaces
SET
  prev = ?,
  next = ?
WHERE
  workspaces.id = ? AND
  workspaces.id IN (
    SELECT wu.workspace_id 
    FROM workspaces_users wu
    WHERE wu.user_id = ?
  );

-- name: UpdateWorkspacePrev :exec
-- Update only the prev pointer for a workspace with user validation (used in deletion)
UPDATE workspaces
SET
  prev = ?
WHERE
  workspaces.id = ? AND
  workspaces.id IN (
    SELECT wu.workspace_id 
    FROM workspaces_users wu
    WHERE wu.user_id = ?
  );

-- name: UpdateWorkspaceNext :exec
-- Update only the next pointer for a workspace with user validation (used in deletion)
UPDATE workspaces
SET
  next = ?
WHERE
  workspaces.id = ? AND
  workspaces.id IN (
    SELECT wu.workspace_id 
    FROM workspaces_users wu
    WHERE wu.user_id = ?
  );

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
