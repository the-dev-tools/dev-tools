--
-- This file is the source of truth for application's db schema
--
--
-- ItemApi
--
-- name: GetItemApi :one
SELECT
  id,
  collection_id,
  folder_id,
  name,
  url,
  method,
  version_parent_id,
  delta_parent_id,
  hidden,
  prev,
  next
FROM
  item_api
WHERE
  id = ?
LIMIT
  1;


-- name: GetItemApiByCollectionIDAndNextIDAndParentID :one
SELECT
  id,
  collection_id,
  folder_id,
  name,
  url,
  method,
  version_parent_id,
  delta_parent_id,
  hidden,
  prev,
  next
FROM
  item_api
WHERE
  next = ? AND
  folder_id = ? AND
  collection_id = ?
LIMIT
  1;

-- name: GetItemsApiByCollectionID :many
SELECT
  id,
  collection_id,
  folder_id,
  name,
  url,
  method,
  version_parent_id,
  delta_parent_id,
  hidden,
  prev,
  next
FROM
  item_api
WHERE
  collection_id = ? AND
  version_parent_id is NULL AND
  delta_parent_id is NULL AND
  hidden = FALSE;

-- name: GetAllItemsApiByCollectionID :many
SELECT
  id,
  collection_id,
  folder_id,
  name,
  url,
  method,
  version_parent_id,
  delta_parent_id,
  hidden,
  prev,
  next
FROM
  item_api
WHERE
  collection_id = ? AND
  version_parent_id is NULL AND
  delta_parent_id is NULL;

-- name: GetItemApiWorkspaceID :one
SELECT
  c.workspace_id
FROM
  collections c
  INNER JOIN item_api i ON c.id = i.collection_id
WHERE
  i.id = ?
LIMIT
  1;

-- name: CreateItemApi :exec
INSERT INTO
  item_api (id, collection_id, folder_id, name, url, method, version_parent_id, delta_parent_id, hidden, prev, next)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: CreateItemApiBulk :exec
INSERT INTO
  item_api (id, collection_id, folder_id, name, url, method, version_parent_id, delta_parent_id, hidden, prev, next)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateItemApi :exec
UPDATE item_api
SET
  folder_id = ?,
  name = ?,
  url = ?,
  method = ?,
  hidden = ?
WHERE
  id = ?;

-- name: UpdateItemApiOrder :exec
UPDATE item_api
SET
  next = ?,
  prev = ?
WHERE
  id = ?;

-- name: DeleteItemApi :exec
DELETE FROM item_api
WHERE
  id = ?;

-- name: GetItemApiByCollectionIDAndURLAndMethod :one
SELECT
  id,
  collection_id,
  folder_id,
  name,
  url,
  method,
  version_parent_id,
  delta_parent_id,
  hidden,
  prev,
  next
FROM
  item_api
WHERE
  collection_id = ? AND
  url = ? AND
  method = ? AND
  version_parent_id is NULL AND
  delta_parent_id is NULL
LIMIT
  1;

--
-- Item Api Example
--
-- name: GetItemApiExample :one
SELECT
    id,
    item_api_id,
    collection_id,
    is_default,
    body_type,
    name,
    version_parent_id,
    prev,
    next
FROM
  item_api_example
WHERE
  id = ?
LIMIT
  1;

-- name: GetItemExampleByCollectionIDAndNextIDAndItemApiID :one
SELECT
    id,
    item_api_id,
    collection_id,
    is_default,
    body_type,
    name,
    version_parent_id,
    prev,
    next
FROM
  item_api_example
WHERE
  collection_id = ? AND
  next = ? AND
  prev = ?
LIMIT
  1;

-- name: GetItemApiExamples :many
SELECT
    id,
    item_api_id,
    collection_id,
    is_default,
    body_type,
    name,
    version_parent_id,
    prev,
    next
FROM
  item_api_example
WHERE
  item_api_id = ? AND
  is_default is false AND
  version_parent_id is NULL;

-- name: GetItemApiExamplesWithDefaults :many
SELECT
    id,
    item_api_id,
    collection_id,
    is_default,
    body_type,
    name,
    version_parent_id,
    prev,
    next
FROM
  item_api_example
WHERE
  item_api_id = ? AND
  version_parent_id is NULL;

-- name: GetItemApiExampleDefault :one
SELECT
    id,
    item_api_id,
    collection_id,
    is_default,
    body_type,
    name,
    version_parent_id,
    prev,
    next
FROM
  item_api_example
WHERE
  item_api_id = ?
  AND is_default = true
LIMIT
  1;

-- name: GetItemApiExampleByCollectionID :many
SELECT
    id,
    item_api_id,
    collection_id,
    is_default,
    body_type,
    name,
    version_parent_id,
    prev,
    next
FROM
  item_api_example
WHERE
  collection_id = ? AND
  version_parent_id is NULL;

-- name: GetItemApiExampleByVersionParentID :many
SELECT
    id,
    item_api_id,
    collection_id,
    is_default,
    body_type,
    name,
    version_parent_id,
    prev,
    next
FROM
  item_api_example
WHERE
  version_parent_id = ?;

-- name: CreateItemApiExample :exec
INSERT INTO
  item_api_example (
    id,
    item_api_id,
    collection_id,
    is_default,
    body_type,
    name,
    version_parent_id,
    prev,
    next
  )
VALUES
    (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: CreateItemApiExampleBulk :exec
INSERT INTO
  item_api_example (
    id,
    item_api_id,
    collection_id,
    is_default,
    body_type,
    name,
    version_parent_id,
    prev,
    next
  )
VALUES
    (?, ?, ?, ?, ?, ?, ?, ?, ?),
    (?, ?, ?, ?, ?, ?, ?, ?, ?),
    (?, ?, ?, ?, ?, ?, ?, ?, ?),
    (?, ?, ?, ?, ?, ?, ?, ?, ?),
    (?, ?, ?, ?, ?, ?, ?, ?, ?),
    (?, ?, ?, ?, ?, ?, ?, ?, ?),
    (?, ?, ?, ?, ?, ?, ?, ?, ?),
    (?, ?, ?, ?, ?, ?, ?, ?, ?),
    (?, ?, ?, ?, ?, ?, ?, ?, ?),
    (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateItemApiExample :exec
UPDATE item_api_example
SET
  name = ?,
  body_type = ?
WHERE
  id = ?;

-- name: UpdateItemApiExampleOrder :exec
UPDATE item_api_example
SET
  prev = ?,
  next = ?
WHERE
  id = ?;


-- name: DeleteItemApiExample :exec
DELETE FROM item_api_example
WHERE
  id = ?;

-- name: GetExamplesByEndpointIDOrdered :many
-- Uses WITH RECURSIVE CTE to traverse linked list from head to tail for endpoint-scoped ordering
-- Examples are scoped to specific endpoints via item_api_id column
-- Excludes default examples (is_default = FALSE) to match other example queries
WITH RECURSIVE ordered_examples AS (
  -- Base case: Find the head (prev IS NULL) for this endpoint
  SELECT
    e.id,
    e.item_api_id,
    e.collection_id,
    e.is_default,
    e.body_type,
    e.name,
    e.version_parent_id,
    e.prev,
    e.next,
    0 as position
  FROM
    item_api_example e
  WHERE
    e.item_api_id = ? AND
    e.version_parent_id IS NULL AND
    e.is_default = FALSE AND
    e.prev IS NULL
  
  UNION ALL
  
  -- Recursive case: Follow the next pointers
  SELECT
    e.id,
    e.item_api_id,
    e.collection_id,
    e.is_default,
    e.body_type,
    e.name,
    e.version_parent_id,
    e.prev,
    e.next,
    oe.position + 1
  FROM
    item_api_example e
  INNER JOIN ordered_examples oe ON e.prev = oe.id
  WHERE
    e.item_api_id = ? AND
    e.version_parent_id IS NULL AND
    e.is_default = FALSE
)
SELECT
  oe.id,
  oe.item_api_id,
  oe.collection_id,
  oe.is_default,
  oe.body_type,
  oe.name,
  oe.version_parent_id,
  oe.prev,
  oe.next,
  oe.position
FROM
  ordered_examples oe
ORDER BY
  oe.position;

-- name: GetAllExamplesByEndpointID :many
-- Returns ALL examples for an endpoint, including isolated ones (prev=NULL, next=NULL)
-- Unlike GetExamplesByEndpointIDOrdered, this query finds examples regardless of linked-list state
-- Essential for finding examples that became isolated during failed move operations
SELECT
  id,
  item_api_id,
  collection_id,
  is_default,
  body_type,
  name,
  version_parent_id,
  prev,
  next
FROM
  item_api_example
WHERE
  item_api_id = ? AND
  version_parent_id IS NULL AND
  is_default = FALSE
ORDER BY
  name ASC;

-- name: UpdateExampleOrder :exec
-- Update the prev/next pointers for a single example with endpoint validation
-- Used for moving examples within the endpoint's linked list
UPDATE item_api_example
SET
  prev = ?,
  next = ?
WHERE
  id = ? AND
  item_api_id = ?;

-- name: UpdateExamplePrev :exec
-- Update only the prev pointer for an example with endpoint validation (used in deletion)
UPDATE item_api_example
SET
  prev = ?
WHERE
  id = ? AND
  item_api_id = ?;

-- name: UpdateExampleNext :exec
-- Update only the next pointer for an example with endpoint validation (used in deletion)
UPDATE item_api_example
SET
  next = ?
WHERE
  id = ? AND
  item_api_id = ?;

--
-- ItemFolder
--


-- name: GetItemFolder :one
SELECT
  id,
  collection_id,
  parent_id,
  name,
  prev,
  next
FROM
  item_folder
WHERE
  id = ?
LIMIT
  1;

-- name: GetItemFoldersByCollectionID :many
SELECT
  id,
  collection_id,
  parent_id,
  name,
  prev,
  next
FROM
  item_folder
WHERE
  collection_id = ?;


-- name: GetItemFolderByCollectionIDAndNextIDAndParentID :one
SELECT
  id,
  collection_id,
  parent_id,
  name,
  prev,
  next
FROM
  item_folder
WHERE
  next = ? AND
  parent_id = ? AND
  collection_id = ?
LIMIT
  1;

-- name: GetItemFolderWorkspaceID :one
SELECT
  c.workspace_id
FROM
  collections c
  INNER JOIN item_folder i ON c.id = i.collection_id
WHERE
  i.id = ?
LIMIT
  1;

-- name: CreateItemFolder :exec
INSERT INTO
    item_folder (id, name, parent_id, collection_id, prev, next)
VALUES
    (?, ?, ?, ?, ?, ?);

-- name: CreateItemFolderBulk :exec
INSERT INTO
    item_folder (id, name, parent_id, collection_id, prev, next)
VALUES
  (?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?);

-- name: UpdateItemFolder :exec
UPDATE item_folder
SET
  name = ?,
  parent_id = ?
WHERE
  id = ?;

-- name: UpdateItemFolderOrder :exec
UPDATE item_folder
SET
  prev = ?,
  next = ?
WHERE
  id = ?;

-- name: DeleteItemFolder :exec
DELETE FROM item_folder
WHERE
  id = ?;

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

--
-- Collections
--
-- name: GetCollection :one
SELECT
  id,
  workspace_id,
  name,
  prev,
  next
FROM
  collections
WHERE
  id = ?
LIMIT
  1;

-- name: GetCollectionByPlatformIDandType :many
SELECT
  id,
  workspace_id,
  name
FROM
  collections
WHERE
  id = ?;

-- name: GetCollectionWorkspaceID :one
SELECT
  workspace_id
FROM
  collections
WHERE
  id = ?
LIMIT
  1;

-- name: GetCollectionByWorkspaceID :many
SELECT
  id,
  workspace_id,
  name
FROM
  collections
WHERE
  workspace_id = ?;

-- name: GetCollectionByWorkspaceIDAndName :one
SELECT
  id,
  workspace_id,
  name
FROM
  collections
WHERE
  workspace_id = ? AND
  name = ?
LIMIT
  1;

-- name: CreateCollection :exec
INSERT INTO
  collections (id, workspace_id, name, prev, next)
VALUES
  (?, ?, ?, ?, ?);

-- name: UpdateCollection :exec
UPDATE collections
SET
  workspace_id = ?,
  name = ?
WHERE
  id = ?;

-- name: DeleteCollection :exec
DELETE FROM collections
WHERE
  id = ?;

-- name: GetCollectionsInOrder :many
-- Uses WITH RECURSIVE CTE to traverse linked list from head to tail
-- Requires index on (workspace_id, prev) for optimal performance
WITH RECURSIVE ordered_collections AS (
  -- Base case: Find the head (prev IS NULL)
  SELECT
    c.id,
    c.workspace_id,
    c.name,
    c.prev,
    c.next,
    0 as position
  FROM
    collections c
  WHERE
    c.workspace_id = ? AND
    c.prev IS NULL
  
  UNION ALL
  
  -- Recursive case: Follow the next pointers
  SELECT
    c.id,
    c.workspace_id,
    c.name,
    c.prev,
    c.next,
    oc.position + 1
  FROM
    collections c
  INNER JOIN ordered_collections oc ON c.prev = oc.id
  WHERE
    c.workspace_id = ?
)
SELECT
  oc.id,
  oc.workspace_id,
  oc.name,
  oc.prev,
  oc.next,
  oc.position
FROM
  ordered_collections oc
ORDER BY
  oc.position;

-- name: GetCollectionByPrevNext :one
-- Find collection by its prev/next references for position-based operations
SELECT
  id,
  workspace_id,
  name,
  prev,
  next
FROM
  collections
WHERE
  workspace_id = ? AND
  prev = ? AND
  next = ?
LIMIT
  1;

-- name: UpdateCollectionOrder :exec
-- Update the prev/next pointers for a single collection
-- Used for moving collections within the linked list
UPDATE collections
SET
  prev = ?,
  next = ?
WHERE
  id = ? AND
  workspace_id = ?;

-- name: UpdateCollectionPositions :exec
-- Batch update positions for multiple collections (for efficient reordering)
-- This query updates prev/next based on the position parameter
-- Usage: Call with arrays of (id, prev_id, next_id, workspace_id) for each collection
-- Note: In practice, this would be used with multiple individual UPDATE statements
-- since SQLite doesn't support arrays directly in prepared statements
UPDATE collections
SET
  prev = ?,
  next = ?
WHERE
  id = ? AND
  workspace_id = ?;

-- name: GetCollectionMaxPosition :one
-- Get the last collection in the list (tail) for a workspace
-- Used when appending new collections to the end of the list
SELECT
  id,
  workspace_id,
  name,
  prev,
  next
FROM
  collections
WHERE
  workspace_id = ? AND
  next IS NULL
LIMIT
  1;

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

/*
*
* Header
*
*/

-- name: GetHeader :one
SELECT
  id,
  example_id,
  delta_parent_id,
  header_key,
  enable,
  description,
  value,
  prev,
  next
FROM
  example_header
WHERE
  id = ?
LIMIT 1;

-- name: GetHeadersByExampleID :many
SELECT
  id,
  example_id,
  delta_parent_id,
  header_key,
  enable,
  description,
  value,
  prev,
  next
FROM
  example_header
WHERE
  example_id = ?;

-- name: GetHeadersByExampleIDOrdered :many
-- Uses WITH RECURSIVE CTE to traverse linked list from head to tail for example-scoped ordering
-- Headers are scoped to specific examples via example_id column
WITH RECURSIVE ordered_headers AS (
  -- Base case: Find the head (prev IS NULL) for this example
  SELECT
    h.id,
    h.example_id,
    h.delta_parent_id,
    h.header_key,
    h.enable,
    h.description,
    h.value,
    h.prev,
    h.next,
    0 as position
  FROM
    example_header h
  WHERE
    h.example_id = ? AND
    h.prev IS NULL
  
  UNION ALL
  
  -- Recursive case: Follow the next pointers
  SELECT
    h.id,
    h.example_id,
    h.delta_parent_id,
    h.header_key,
    h.enable,
    h.description,
    h.value,
    h.prev,
    h.next,
    oh.position + 1
  FROM
    example_header h
  INNER JOIN ordered_headers oh ON h.prev = oh.id
  WHERE
    h.example_id = ?
)
SELECT
  oh.id,
  oh.example_id,
  oh.delta_parent_id,
  oh.header_key,
  oh.enable,
  oh.description,
  oh.value,
  oh.prev,
  oh.next,
  oh.position
FROM
  ordered_headers oh
ORDER BY
  oh.position;

-- name: GetAllHeadersByExampleID :many
-- Returns ALL headers for an example, including isolated ones (prev=NULL, next=NULL)
-- Unlike GetHeadersByExampleIDOrdered, this query finds headers regardless of linked-list state
-- Essential for finding headers that became isolated during failed move operations
SELECT
  id,
  example_id,
  delta_parent_id,
  header_key,
  enable,
  description,
  value,
  prev,
  next
FROM
  example_header
WHERE
  example_id = ?
ORDER BY
  header_key ASC;

-- name: GetHeaderByDeltaParentID :one
SELECT
  id,
  example_id,
  delta_parent_id,
  header_key,
  enable,
  description,
  value,
  prev,
  next
FROM
  example_header
WHERE
  delta_parent_id = ?
LIMIT 1;

-- name: SetHeaderEnable :exec
UPDATE example_header
    SET
  enable = ?
WHERE
  id = ?;

-- name: CreateHeader :exec
INSERT INTO
  example_header (id, example_id, delta_parent_id, header_key, enable, description, value, prev, next)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: CreateHeaderBulk :exec
INSERT INTO
  example_header (id, example_id, delta_parent_id, header_key, enable, description, value, prev, next)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateHeader :exec
UPDATE example_header
SET
  header_key = ?,
  enable = ?,
  description = ?,
  value = ?
WHERE
  id = ?;

-- name: UpdateHeaderOrder :exec
-- Update the prev/next pointers for a single header with example validation
-- Used for moving headers within the example's linked list
UPDATE example_header
SET
  prev = ?,
  next = ?
WHERE
  id = ? AND
  example_id = ?;

-- name: UpdateHeaderPrev :exec
-- Update only the prev pointer for a header with example validation (used in deletion)
UPDATE example_header
SET
  prev = ?
WHERE
  id = ? AND
  example_id = ?;

-- name: UpdateHeaderNext :exec
-- Update only the next pointer for a header with example validation (used in deletion)
UPDATE example_header
SET
  next = ?
WHERE
  id = ? AND
  example_id = ?;

-- name: GetHeaderTail :one
-- Get the last header in the list (tail) for an example
-- Used when appending new headers to the end of the list
SELECT
  id,
  example_id,
  delta_parent_id,
  header_key,
  enable,
  description,
  value,
  prev,
  next
FROM
  example_header
WHERE
  example_id = ? AND
  next IS NULL
LIMIT
  1;

-- name: DeleteHeader :exec
DELETE FROM example_header
WHERE
  id = ?;

/*
*
* Query
*
*/

-- name: GetQuery :one
SELECT
  id,
  example_id,
  delta_parent_id,
  query_key,
  enable,
  description,
  value
FROM
  example_query
WHERE
  id = ?
LIMIT 1;

-- name: GetQueriesByExampleID :many
SELECT
  id,
  example_id,
  delta_parent_id,
  query_key,
  enable,
  description,
  value
FROM
  example_query
WHERE
  example_id = ?;

-- name: GetQueryByDeltaParentID :one
SELECT
  id,
  example_id,
  delta_parent_id,
  query_key,
  enable,
  description,
  value
FROM
  example_query
WHERE
  delta_parent_id = ?
lIMIT 1;

-- name: CreateQuery :exec
INSERT INTO
  example_query (id, example_id, delta_parent_id, query_key, enable, description, value)
VALUES
  (?, ?, ?, ?, ?, ?, ?);

-- name: CreateQueryBulk :exec
INSERT INTO
  example_query (id, example_id, delta_parent_id, query_key, enable, description, value)
VALUES
  (?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?);

-- name: UpdateQuery :exec
UPDATE example_query
SET
  query_key = ?,
  enable = ?,
  description = ?,
  value = ?
WHERE
  id = ?;

-- name: DeleteQuery :exec
DELETE FROM example_query
WHERE
  id = ?;

-- name: SetQueryEnable :exec
UPDATE example_query
SET
  enable = ?
WHERE
  id = ?;

/*
*
* body_form
*
*/

-- name: GetBodyForm :one
SELECT
  id,
  example_id,
  delta_parent_id,
  body_key,
  enable,
  description,
  value
FROM
    example_body_form
WHERE
    id = ?
LIMIT 1;

-- name: GetBodyFormsByExampleID :many
SELECT
  id,
  example_id,
  delta_parent_id,
  body_key,
  enable,
  description,
  value
FROM
    example_body_form
WHERE
    example_id = ?;

-- name: GetBodyFormsByDeltaParentID :many
SELECT
  id,
  example_id,
  delta_parent_id,
  body_key,
  enable,
  description,
  value
FROM
    example_body_form
WHERE
    delta_parent_id = ?;

--
-- BodyForm
--

-- name: CreateBodyForm :exec
INSERT INTO
  example_body_form (id, example_id, delta_parent_id, body_key, enable, description, value)
VALUES
  (?, ?, ?, ?, ?, ?, ?);

-- name: CreateBodyFormBulk :exec
INSERT INTO
  example_body_form (id, example_id, delta_parent_id, body_key, enable, description, value)
VALUES
  (?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?);

-- name: UpdateBodyForm :exec
UPDATE example_body_form
SET
  body_key = ?,
  enable = ?,
  description = ?,
  value = ?
WHERE
  id = ?;

-- name: SetBodyFormEnable :exec
UPDATE example_body_form
SET
  enable = ?
WHERE
  id = ?;

-- name: DeleteBodyForm :exec
DELETE FROM example_body_form
WHERE
  id = ?;

--
-- Body Url Encoded
--

-- name: GetBodyUrlEncoded :one
SELECT
  id,
  example_id,
  delta_parent_id,
  body_key,
  enable,
  description,
  value
FROM
  example_body_urlencoded
WHERE
  id = ?
LIMIT 1;

-- name: GetBodyUrlEncodedsByExampleID :many
SELECT
  id,
  example_id,
  delta_parent_id,
  body_key,
  enable,
  description,
  value
FROM
  example_body_urlencoded
WHERE
  example_id = ?;

-- name: GetBodyUrlEncodedsByDeltaParentID :many
SELECT
  id,
  example_id,
  delta_parent_id,
  body_key,
  enable,
  description,
  value
FROM
  example_body_urlencoded
WHERE
  delta_parent_id = ?;

-- name: CreateBodyUrlEncoded :exec
INSERT INTO
  example_body_urlencoded (id, example_id, delta_parent_id, body_key, enable, description, value)
VALUES
    (?, ?, ?, ?, ?, ?, ?);

-- name: CreateBodyUrlEncodedBulk :exec
INSERT INTO
  example_body_urlencoded (id, example_id, delta_parent_id, body_key, enable, description, value)
VALUES
    (?, ?, ?, ?, ?, ?, ?),
    (?, ?, ?, ?, ?, ?, ?),
    (?, ?, ?, ?, ?, ?, ?),
    (?, ?, ?, ?, ?, ?, ?),
    (?, ?, ?, ?, ?, ?, ?),
    (?, ?, ?, ?, ?, ?, ?),
    (?, ?, ?, ?, ?, ?, ?),
    (?, ?, ?, ?, ?, ?, ?),
    (?, ?, ?, ?, ?, ?, ?),
    (?, ?, ?, ?, ?, ?, ?);

-- name: UpdateBodyUrlEncoded :exec
UPDATE example_body_urlencoded
    SET
      body_key = ?,
      enable = ?,
      description = ?,
      value = ?
    WHERE
      id = ?;

-- name: DeleteBodyURLEncoded :exec
DELETE FROM example_body_urlencoded
WHERE
  id = ?;

/*
* Body Raw
*/

-- name: GetBodyRaw :one
SELECT
  id,
  example_id,
  visualize_mode,
  compress_type,
  data
FROM
  example_body_raw
WHERE
  id = ?
LIMIT 1;

-- name: GetBodyRawsByExampleID :one
SELECT
  id,
  example_id,
  visualize_mode,
  compress_type,
  data
FROM
  example_body_raw
WHERE
  example_id = ?
LIMIT 1;

-- name: CreateBodyRaw :exec
INSERT INTO
  example_body_raw (id, example_id, visualize_mode, compress_type, data)
VALUES
  (?, ?, ?, ?, ?);

-- name: CreateBodyRawBulk :exec
INSERT INTO
  example_body_raw (id, example_id, visualize_mode, compress_type, data)
VALUES
  (?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?);


-- name: UpdateVisualizeMode :exec
UPDATE example_body_raw
SET visualize_mode = ?
WHERE
  id = ?;

-- name: UpdateBodyRawData :exec
UPDATE example_body_raw
SET
  compress_type = ?,
  data = ?
WHERE
  id = ?;


-- name: DeleteBodyRaw :exec
DELETE FROM example_body_raw
WHERE
  id = ?;


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
  prev,
  next
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
  prev,
  next
FROM
  environment
WHERE
  workspace_id = ?;

-- name: CreateEnvironment :exec
INSERT INTO
  environment (id, workspace_id, type, name, description, prev, next)
VALUES
  (?, ?, ?, ?, ?, ?, ?);

-- name: UpdateEnvironment :exec
UPDATE environment
SET
    name = ?,
    description = ?
WHERE
    id = ?;

-- name: DeleteEnvironment :exec
DELETE FROM environment
WHERE
  id = ?;

-- name: GetEnvironmentsByWorkspaceIDOrdered :many
-- Uses WITH RECURSIVE CTE to traverse linked list from head to tail
-- Requires index on (workspace_id, prev) for optimal performance
WITH RECURSIVE ordered_environments AS (
  -- Base case: Find the head (prev IS NULL)
  SELECT
    e.id,
    e.workspace_id,
    e.type,
    e.name,
    e.description,
    e.prev,
    e.next,
    0 as position
  FROM
    environment e
  WHERE
    e.workspace_id = ? AND
    e.prev IS NULL
  
  UNION ALL
  
  -- Recursive case: Follow the next pointers
  SELECT
    e.id,
    e.workspace_id,
    e.type,
    e.name,
    e.description,
    e.prev,
    e.next,
    oe.position + 1
  FROM
    environment e
  INNER JOIN ordered_environments oe ON e.prev = oe.id
  WHERE
    e.workspace_id = ?
)
SELECT
  oe.id,
  oe.workspace_id,
  oe.type,
  oe.name,
  oe.description,
  oe.prev,
  oe.next,
  oe.position
FROM
  ordered_environments oe
ORDER BY
  oe.position;

-- name: GetEnvironmentWorkspaceID :one
-- Get workspace ID for environment (validation for move operations)
SELECT
  workspace_id
FROM
  environment
WHERE
  id = ?
LIMIT
  1;

-- name: UpdateEnvironmentOrder :exec
-- Update the prev/next pointers for a single environment
-- Used for moving environments within the linked list
UPDATE environment
SET
  prev = ?,
  next = ?
WHERE
  id = ? AND
  workspace_id = ?;

-- name: GetEnvironmentMaxPosition :one
-- Get the last environment in the list (tail) for a workspace
-- Used when appending new environments to the end of the list
SELECT
  id,
  workspace_id,
  type,
  name,
  description,
  prev,
  next
FROM
  environment
WHERE
  workspace_id = ? AND
  next IS NULL
LIMIT
  1;

-- name: GetEnvironmentByPrevNext :one
-- Find environment by its prev/next references for position-based operations
SELECT
  id,
  workspace_id,
  type,
  name,
  description,
  prev,
  next
FROM
  environment
WHERE
  workspace_id = ? AND
  prev = ? AND
  next = ?
LIMIT
  1;

-- name: UpdateEnvironmentNext :exec
-- Update only the next pointer for an environment (used in deletion)
UPDATE environment
SET
  next = ?
WHERE
  id = ? AND
  workspace_id = ?;

-- name: UpdateEnvironmentPrev :exec
-- Update only the prev pointer for an environment (used in deletion)
UPDATE environment
SET
  prev = ?
WHERE
  id = ? AND
  workspace_id = ?;

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
  prev,
  next
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
  prev,
  next
FROM
  variable
WHERE
  env_id = ?;

-- name: GetVariablesByEnvironmentIDOrdered :many
-- Uses WITH RECURSIVE CTE to traverse linked list from head to tail
-- Requires index on (env_id, prev) for optimal performance
WITH RECURSIVE ordered_variables AS (
  -- Base case: Find the head (prev IS NULL)
  SELECT
    v.id,
    v.env_id,
    v.var_key,
    v.value,
    v.enabled,
    v.description,
    v.prev,
    v.next,
    0 as position
  FROM
    variable v
  WHERE
    v.env_id = ? AND
    v.prev IS NULL
  
  UNION ALL
  
  -- Recursive case: Follow the next pointers
  SELECT
    v.id,
    v.env_id,
    v.var_key,
    v.value,
    v.enabled,
    v.description,
    v.prev,
    v.next,
    ov.position + 1
  FROM
    variable v
  INNER JOIN ordered_variables ov ON v.prev = ov.id
  WHERE
    v.env_id = ?
)
SELECT
  ov.id,
  ov.env_id,
  ov.var_key,
  ov.value,
  ov.enabled,
  ov.description,
  ov.prev,
  ov.next,
  ov.position
FROM
  ordered_variables ov
ORDER BY
  ov.position;

-- name: CreateVariable :exec
INSERT INTO
  variable (id, env_id, var_key, value, enabled, description, prev, next)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?);

-- name: CreateVariableBulk :exec
INSERT INTO
  variable (id, env_id, var_key, value, enabled, description, prev, next)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateVariable :exec
UPDATE variable
SET
  var_key = ?,
  value = ?,
  enabled = ?,
  description = ?
WHERE
  id = ?;

-- name: DeleteVariable :exec
DELETE FROM variable
WHERE
  id = ?;

-- name: UpdateVariableOrder :exec
-- Update the prev/next pointers for a single variable
-- Used for moving variables within the linked list
UPDATE variable
SET
  prev = ?,
  next = ?
WHERE
  id = ? AND
  env_id = ?;

-- name: UpdateVariablePrev :exec
-- Update only the prev pointer for a variable (used in deletion)
UPDATE variable
SET
  prev = ?
WHERE
  id = ? AND
  env_id = ?;

-- name: UpdateVariableNext :exec
-- Update only the next pointer for a variable (used in deletion)
UPDATE variable
SET
  next = ?
WHERE
  id = ? AND
  env_id = ?;

-- name: GetExampleResp :one
SELECT
    *
FROM
  example_resp
WHERE
  id = ?
LIMIT 1;

-- name: GetExampleRespByExampleID :one
SELECT
    *
FROM
  example_resp
WHERE
  example_id = ?
LIMIT 1;

-- name: GetExampleRespByExampleIDLatest :one
SELECT
    *
FROM
  example_resp
WHERE
  example_id = ?
ORDER BY id DESC
LIMIT 1;

-- name: CreateExampleResp :exec
INSERT INTO
  example_resp (id, example_id, status, body, body_compress_type, duration)
VALUES
  (?, ?, ?, ?, ?, ?);

-- name: UpdateExampleResp :exec
UPDATE example_resp
SET
  status = ?,
  body = ?,
  body_compress_type = ?,
  duration = ?
WHERE
  id = ?;

-- name: DeleteExampleResp :exec
DELETE FROM example_resp
WHERE
  id = ?;

/*
* example_resp_header
*/

-- name: GetExampleRespHeader :one
SELECT
  id,
  example_resp_id,
  header_key,
  value
FROM
  example_resp_header
WHERE
  id = ?
LIMIT 1;


-- name: GetExampleRespHeadersByRespID :many
SELECT
  id,
  example_resp_id,
  header_key,
  value
FROM
  example_resp_header
WHERE
  example_resp_id = ?;

-- name: CreateExampleRespHeader :exec
INSERT INTO
  example_resp_header (id, example_resp_id, header_key, value)
VALUES
  (?, ?, ?, ?);

-- name: CreateExampleRespHeaderBulk :exec
INSERT INTO
  example_resp_header (id, example_resp_id, header_key, value)
VALUES
  (?, ?, ?, ?),
  (?, ?, ?, ?),
  (?, ?, ?, ?),
  (?, ?, ?, ?),
  (?, ?, ?, ?);

-- name: UpdateExampleRespHeader :exec
UPDATE example_resp_header
SET
  header_key = ?,
  value = ?
WHERE
  id = ?;

-- name: DeleteExampleRespHeader :exec
DELETE FROM example_resp_header
WHERE
  id = ?;


/*
* INFO: Asserts
*/

-- name: GetAssert :one
SELECT
  id,
  example_id,
  delta_parent_id,
  expression,
  enable,
  prev,
  next
FROM
  assertion
WHERE
  id = ?
LIMIT 1;

-- name: GetAssertsByExampleID :many
SELECT
  id,
  example_id,
  delta_parent_id,
  expression,
  enable,
  prev,
  next
FROM
  assertion
WHERE
  example_id = ?;

-- name: CreateAssert :exec
INSERT INTO
  assertion (id, example_id, delta_parent_id, expression, enable, prev, next)
VALUES
  (?, ?, ?, ?, ?, ?, ?);

-- name: UpdateAssert :exec
UPDATE assertion
SET
  delta_parent_id = ?,
  expression = ?,
  enable = ?
WHERE
  id = ?;

-- name: DeleteAssert :exec
DELETE FROM assertion
WHERE
  id = ?;

/*
* INFO: assert_result
*/

-- name: GetAssertResult :one
SELECT
  id,
  response_id,
  assertion_id,
  result
FROM
  assertion_result
WHERE
  id = ?
LIMIT 1;

-- name: GetAssertResultsByAssertID :many
SELECT
  id,
  response_id,
  assertion_id,
  result
FROM
  assertion_result
WHERE
  assertion_id = ?;

-- name: GetAssertResultsByResponseID :many
SELECT
  id,
  response_id,
  assertion_id,
  result
FROM
  assertion_result
WHERE
  response_id = ?;

-- name: CreateAssertResult :exec
INSERT INTO
  assertion_result (id, response_id, assertion_id, result)
VALUES
  (?, ?, ?, ?);

-- name: CreateAssertBulk :exec
INSERT INTO
  assertion_result (id, response_id, assertion_id, result)
VALUES
  (?, ?, ?, ?),
  (?, ?, ?, ?),
  (?, ?, ?, ?),
  (?, ?, ?, ?),
  (?, ?, ?, ?);

-- name: UpdateAssertResult :exec
UPDATE assertion_result
SET
  result = ?
WHERE
  id = ?;

-- name: DeleteAssertResult :exec
DELETE FROM assertion_result
WHERE
  id = ?;

-- name: GetFlow :one
SELECT
  id,
  workspace_id,
  version_parent_id,
  name
FROM
  flow
WHERE
  id = ?
LIMIT 1;

-- name: GetFlowsByWorkspaceID :many
SELECT
  id,
  workspace_id,
  version_parent_id,
  name
FROM
  flow
WHERE
  workspace_id = ? AND
  version_parent_id is NULL;

-- name: GetFlowsByVersionParentID :many
SELECT
  id,
  workspace_id,
  version_parent_id,
  name
FROM
  flow
WHERE
  version_parent_id is ?;

-- name: CreateFlow :exec
INSERT INTO
  flow (id, workspace_id, version_parent_id, name)
VALUES
  (?, ?, ?, ?);

-- name: UpdateFlow :exec
UPDATE flow
SET
  name = ?
WHERE
  id = ?;

-- name: DeleteFlow :exec
DELETE FROM flow
WHERE
  id = ?;

-- name: GetTag :one
SELECT
  id,
  workspace_id,
  name,
  color
FROM
  tag
WHERE
  id = ?
LIMIT 1;

-- name: GetTagsByWorkspaceID :many
SELECT
  id,
  workspace_id,
  name,
  color
FROM
  tag
WHERE
  workspace_id = ?;

-- name: CreateTag :exec
INSERT INTO
  tag (id, workspace_id, name, color)
VALUES
  (?, ?, ?, ?);

-- name: UpdateTag :exec
UPDATE tag
SET
  name = ?,
  color = ?
WHERE
  id = ?;

-- name: DeleteTag :exec
DELETE FROM tag
WHERE
  id = ?;

-- name: GetFlowTag :one
SELECT
  id,
  flow_id,
  tag_id
FROM flow_tag
WHERE id = ?
LIMIT 1;

-- name: GetFlowTagsByFlowID :many
SELECT
  id,
  flow_id,
  tag_id
FROM
  flow_tag
WHERE
  flow_id = ?;

-- name: GetFlowTagsByTagID :many
SELECT
  id,
  flow_id,
  tag_id
FROM
  flow_tag
WHERE
  tag_id = ?;

-- name: CreateFlowTag :exec
INSERT INTO
  flow_tag (id, flow_id, tag_id)
VALUES
  (?, ?, ?);

-- name: DeleteFlowTag :exec
DELETE FROM flow_tag
WHERE
  id = ?;

-- name: GetFlowNode :one
SELECT
  id,
  flow_id,
  name,
  node_kind,
  position_x,
  position_y
FROM
  flow_node
WHERE
  id = ?
LIMIT 1;

-- name: GetFlowNodesByFlowID :many
SELECT
  id,
  flow_id,
  name,
  node_kind,
  position_x,
  position_y
FROM
  flow_node
WHERE
  flow_id = ?;

-- name: CreateFlowNode :exec
INSERT INTO
  flow_node (id, flow_id, name, node_kind, position_x, position_y)
VALUES
  (?, ?, ?, ?, ?, ?);

-- name: UpdateFlowNode :exec
UPDATE flow_node
SET
  name = ?,
  position_x = ?,
  position_y = ?
WHERE
  id = ?;

-- name: DeleteFlowNode :exec
DELETE FROM flow_node
WHERE
  id = ?;

-- name: GetFlowEdge :one
SELECT
  id,
  flow_id,
  source_id,
  target_id,
  source_handle,
  edge_kind
FROM
  flow_edge
WHERE
  id = ?
LIMIT 1;

-- name: GetFlowEdgesByFlowID :many
SELECT
  id,
  flow_id,
  source_id,
  target_id,
  source_handle,
  edge_kind
FROM
  flow_edge
WHERE
  flow_id = ?;

-- name: CreateFlowEdge :exec
INSERT INTO
  flow_edge (id, flow_id, source_id, target_id, source_handle, edge_kind)
VALUES
  (?, ?, ?, ?, ?, ?);

-- name: UpdateFlowEdge :exec
UPDATE flow_edge
SET
  source_id = ?,
  target_id = ?,
  source_handle = ?,
  edge_kind = ?
WHERE
  id = ?;

-- name: DeleteFlowEdge :exec
DELETE FROM
  flow_edge
WHERE
  id = ?;

-- name: GetFlowNodeFor :one
SELECT
  flow_node_id,
  iter_count,
  error_handling,
  expression
FROM
  flow_node_for
WHERE
  flow_node_id = ?
LIMIT 1;

-- name: CreateFlowNodeFor :exec
INSERT INTO
  flow_node_for (flow_node_id, iter_count, error_handling, expression)
VALUES
  (?, ?, ?, ?);

-- name: UpdateFlowNodeFor :exec
UPDATE flow_node_for
SET
  iter_count = ?,
  error_handling = ?,
  expression = ?
WHERE
  flow_node_id = ?;

-- name: DeleteFlowNodeFor :exec
DELETE FROM flow_node_for
WHERE
  flow_node_id = ?;

-- name: GetFlowNodeForEach :one
SELECT
  flow_node_id,
  iter_expression,
  error_handling,
  expression
FROM
  flow_node_for_each
WHERE
  flow_node_id = ?
LIMIT 1;

-- name: CreateFlowNodeForEach :exec
INSERT INTO
  flow_node_for_each (flow_node_id, iter_expression, error_handling, expression)
VALUES
  (?, ?, ?, ?);

-- name: UpdateFlowNodeForEach :exec
UPDATE flow_node_for_each
SET
  iter_expression = ?,
  error_handling = ?,
  expression = ?
WHERE
  flow_node_id = ?;

-- name: DeleteFlowNodeForEach :exec
DELETE FROM flow_node_for_each
WHERE
  flow_node_id = ?;

-- name: GetFlowNodeRequest :one
SELECT
  flow_node_id,
  endpoint_id,
  example_id,
  delta_example_id,
  delta_endpoint_id
FROM
  flow_node_request
WHERE
  flow_node_id = ?
LIMIT 1;

-- name: CreateFlowNodeRequest :exec
INSERT INTO
  flow_node_request (flow_node_id, endpoint_id, example_id, delta_example_id, delta_endpoint_id)
VALUES
  (?, ?, ?, ?, ?);

-- name: UpdateFlowNodeRequest :exec
UPDATE flow_node_request
SET
  endpoint_id = ?,
  example_id = ?,
  delta_example_id = ?,
  delta_endpoint_id = ?
WHERE
  flow_node_id = ?;

-- name: DeleteFlowNodeRequest :exec
DELETE FROM flow_node_request
WHERE
  flow_node_id = ?;

-- name: GetFlowNodeCondition :one
SELECT
  flow_node_id,
  expression
FROM
  flow_node_condition
WHERE
  flow_node_id = ?
LIMIT 1;

-- name: CreateFlowNodeCondition :exec
INSERT INTO
  flow_node_condition (flow_node_id, expression)
VALUES
  (?, ?);

-- name: UpdateFlowNodeCondition :exec
UPDATE flow_node_condition
SET
  expression = ?
WHERE
  flow_node_id = ?;

-- name: DeleteFlowNodeCondition :exec
DELETE FROM flow_node_condition
WHERE
  flow_node_id = ?;


-- name: GetFlowNodeNoop :one
SELECT
  flow_node_id,
  node_type
FROM
  flow_node_noop
where
  flow_node_id = ?
LIMIT 1;

-- name: CreateFlowNodeNoop :exec
INSERT INTO
  flow_node_noop (flow_node_id, node_type)
VALUES
  (?, ?);

-- name: DeleteFlowNodeNoop :exec
DELETE from flow_node_noop
WHERE
  flow_node_id = ?;

-- name: GetFlowNodeJs :one
SELECT
  flow_node_id,
  code,
  code_compress_type
FROM
  flow_node_js
WHERE
  flow_node_id = ?
LIMIT 1;

-- name: CreateFlowNodeJs :exec
INSERT INTO
  flow_node_js (flow_node_id, code, code_compress_type)
VALUES
  (?, ?, ?);

-- name: UpdateFlowNodeJs :exec
UPDATE flow_node_js
SET
  code = ?,
  code_compress_type = ?
WHERE
  flow_node_id = ?;

-- name: DeleteFlowNodeJs :exec
DELETE FROM flow_node_js
WHERE
  flow_node_id = ?;

-- name: GetMigration :one
SELECT
  id,
  version,
  description,
  apply_at
FROM
  migration
WHERE
  id = ?
LIMIT 1;

-- name: GetMigrations :many
SELECT
  id,
  version,
  description,
  apply_at
FROM
  migration;

-- name: CreateMigration :exec
INSERT INTO
  migration (id, version, description, apply_at)
VALUES
  (?, ?, ?, ?);

-- name: DeleteMigration :exec
DELETE FROM migration
WHERE
  id = ?;

-- name: GetFlowVariable :one
SELECT
  id,
  flow_id,
  key,
  value,
  enabled,
  description,
  prev,
  next
FROM
  flow_variable
WHERE
  id = ?
LIMIT 1;

-- name: GetFlowVariablesByFlowID :many
SELECT
  id,
  flow_id,
  key,
  value,
  enabled,
  description,
  prev,
  next
FROM
  flow_variable
WHERE
  flow_id = ?;

-- name: CreateFlowVariable :exec
INSERT INTO
  flow_variable (id, flow_id, key, value, enabled, description, prev, next)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?);

-- name: CreateFlowVariableBulk :exec
INSERT INTO
  flow_variable (id, flow_id, key, value, enabled, description, prev, next)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateFlowVariable :exec
UPDATE flow_variable
SET
  key = ?,
  value = ?,
  enabled = ?,
  description = ?
WHERE
  id = ?;

-- name: DeleteFlowVariable :exec
DELETE FROM flow_variable
WHERE
  id = ?;

-- name: GetFlowVariablesByFlowIDOrdered :many
-- Uses WITH RECURSIVE CTE to traverse linked list from head to tail
-- Requires index on (flow_id, prev) for optimal performance
WITH RECURSIVE ordered_flow_variables AS (
  -- Base case: Find the head (prev IS NULL)
  SELECT
    fv.id,
    fv.flow_id,
    fv.key,
    fv.value,
    fv.enabled,
    fv.description,
    fv.prev,
    fv.next,
    0 as position
  FROM
    flow_variable fv
  WHERE
    fv.flow_id = ? AND
    fv.prev IS NULL
  
  UNION ALL
  
  -- Recursive case: Follow the next pointers
  SELECT
    fv.id,
    fv.flow_id,
    fv.key,
    fv.value,
    fv.enabled,
    fv.description,
    fv.prev,
    fv.next,
    ofv.position + 1
  FROM
    flow_variable fv
  INNER JOIN ordered_flow_variables ofv ON fv.prev = ofv.id
  WHERE
    fv.flow_id = ?
)
SELECT
  ofv.id,
  ofv.flow_id,
  ofv.key,
  ofv.value,
  ofv.enabled,
  ofv.description,
  ofv.prev,
  ofv.next,
  ofv.position
FROM
  ordered_flow_variables ofv
ORDER BY
  ofv.position;

-- name: UpdateFlowVariableOrder :exec
-- Update the prev/next pointers for a single flow variable
-- Used for moving flow variables within the linked list
UPDATE flow_variable
SET
  prev = ?,
  next = ?
WHERE
  id = ? AND
  flow_id = ?;

-- name: UpdateFlowVariablePrev :exec
-- Update only the prev pointer for a flow variable (used in deletion)
UPDATE flow_variable
SET
  prev = ?
WHERE
  id = ? AND
  flow_id = ?;

-- name: UpdateFlowVariableNext :exec
-- Update only the next pointer for a flow variable (used in deletion)
UPDATE flow_variable
SET
  next = ?
WHERE
  id = ? AND
  flow_id = ?;

-- Node Execution
-- name: GetNodeExecution :one
SELECT * FROM node_execution
WHERE id = ?;

-- name: ListNodeExecutions :many
SELECT * FROM node_execution
WHERE node_id = ?
ORDER BY completed_at DESC
LIMIT ? OFFSET ?;

-- name: ListNodeExecutionsByState :many
SELECT * FROM node_execution
WHERE node_id = ? AND state = ?
ORDER BY completed_at DESC
LIMIT ? OFFSET ?;

-- name: ListNodeExecutionsByFlowRun :many
SELECT ne.* FROM node_execution ne
JOIN flow_node fn ON ne.node_id = fn.id
WHERE fn.flow_id = ?
ORDER BY ne.completed_at DESC;

-- name: CreateNodeExecution :one
INSERT INTO node_execution (
  id, node_id, name, state, error, input_data, input_data_compress_type,
  output_data, output_data_compress_type, response_id, completed_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateNodeExecution :one
UPDATE node_execution
SET state = ?, error = ?, output_data = ?, 
    output_data_compress_type = ?, response_id = ?, completed_at = ?
WHERE id = ?
RETURNING *;

-- name: UpsertNodeExecution :one
INSERT INTO node_execution (
  id, node_id, name, state, error, input_data, input_data_compress_type,
  output_data, output_data_compress_type, response_id, completed_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  state = excluded.state,
  error = excluded.error, 
  output_data = excluded.output_data,
  output_data_compress_type = excluded.output_data_compress_type,
  response_id = excluded.response_id,
  completed_at = excluded.completed_at
RETURNING *;

-- name: GetNodeExecutionsByNodeID :many
SELECT * FROM node_execution WHERE node_id = ? ORDER BY id DESC;

-- name: GetLatestNodeExecutionByNodeID :one
SELECT * FROM node_execution WHERE node_id = ? ORDER BY id DESC LIMIT 1;

-- name: DeleteNodeExecutionsByNodeID :exec
DELETE FROM node_execution WHERE node_id = ?;

-- name: DeleteNodeExecutionsByNodeIDs :exec
DELETE FROM node_execution WHERE node_id IN (sqlc.slice('node_ids'));

-- 
-- Collection Items Unified Table Queries
-- These queries handle both folders and endpoints through a unified interface
--

-- name: GetCollectionItem :one
SELECT
  id,
  collection_id,
  parent_folder_id,
  item_type,
  folder_id,
  endpoint_id,
  name,
  prev_id,
  next_id
FROM
  collection_items
WHERE
  id = ?
LIMIT
  1;

-- name: GetCollectionItemsInOrder :many
-- Uses WITH RECURSIVE CTE to traverse linked list from head to tail
-- Returns items in correct order for a collection/parent folder
WITH RECURSIVE ordered_items AS (
  -- Base case: Find the head (prev_id IS NULL)
  SELECT
    ci.id,
    ci.collection_id,
    ci.parent_folder_id,
    ci.item_type,
    ci.folder_id,
    ci.endpoint_id,
    ci.name,
    ci.prev_id,
    ci.next_id,
    0 as position
  FROM
    collection_items ci
  WHERE
    ci.collection_id = ? AND
    ci.parent_folder_id IS ? AND
    ci.prev_id IS NULL
  
  UNION ALL
  
  -- Recursive case: Follow the next_id pointers
  SELECT
    ci.id,
    ci.collection_id,
    ci.parent_folder_id,
    ci.item_type,
    ci.folder_id,
    ci.endpoint_id,
    ci.name,
    ci.prev_id,
    ci.next_id,
    oi.position + 1
  FROM
    collection_items ci
  INNER JOIN ordered_items oi ON ci.prev_id = oi.id
  WHERE
    ci.collection_id = ?
)
SELECT
  oi.id,
  oi.collection_id,
  oi.parent_folder_id,
  oi.item_type,
  oi.folder_id,
  oi.endpoint_id,
  oi.name,
  oi.prev_id,
  oi.next_id,
  oi.position
FROM
  ordered_items oi
ORDER BY
  oi.position;

-- name: GetCollectionItemsByCollectionID :many
SELECT
  id,
  collection_id,
  parent_folder_id,
  item_type,
  folder_id,
  endpoint_id,
  name,
  prev_id,
  next_id
FROM
  collection_items
WHERE
  collection_id = ?;

-- name: GetCollectionItemsByParentFolderID :many
SELECT
  id,
  collection_id,
  parent_folder_id,
  item_type,
  folder_id,
  endpoint_id,
  name,
  prev_id,
  next_id
FROM
  collection_items
WHERE
  collection_id = ? AND
  parent_folder_id = ?;

-- name: GetCollectionItemTail :one
-- Get the last item in the list (tail) for a collection/parent folder
-- Used when appending new items to the end of the list
SELECT
  id,
  collection_id,
  parent_folder_id,
  item_type,
  folder_id,
  endpoint_id,
  name,
  prev_id,
  next_id
FROM
  collection_items
WHERE
  collection_id = ? AND
  parent_folder_id IS ? AND
  next_id IS NULL
LIMIT
  1;

-- name: InsertCollectionItem :exec
INSERT INTO
  collection_items (
    id,
    collection_id,
    parent_folder_id,
    item_type,
    folder_id,
    endpoint_id,
    name,
    prev_id,
    next_id
  )
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateCollectionItemOrder :exec
-- Update the prev_id/next_id pointers for a single collection item
-- Used for moving items within the linked list
UPDATE collection_items
SET
  prev_id = ?,
  next_id = ?
WHERE
  id = ?;

-- name: UpdateCollectionItemParent :exec
-- Move an item to a different parent folder while maintaining linked list integrity
UPDATE collection_items
SET
  parent_folder_id = ?,
  prev_id = ?,
  next_id = ?
WHERE
  id = ?;

-- name: DeleteCollectionItem :exec
DELETE FROM collection_items
WHERE
  id = ?;

-- name: GetCollectionItemsByType :many
-- Get items filtered by type (0 = folder, 1 = endpoint)
SELECT
  id,
  collection_id,
  parent_folder_id,
  item_type,
  folder_id,
  endpoint_id,
  name,
  prev_id,
  next_id
FROM
  collection_items
WHERE
  collection_id = ? AND
  parent_folder_id IS ? AND
  item_type = ?;

-- name: GetCollectionItemByFolderID :one
-- Get collection item by folder_id (for legacy ID compatibility)
SELECT
  id,
  collection_id,
  parent_folder_id,
  item_type,
  folder_id,
  endpoint_id,
  name,
  prev_id,
  next_id
FROM
  collection_items
WHERE
  folder_id = ?
LIMIT
  1;

-- name: GetCollectionItemByEndpointID :one
-- Get collection item by endpoint_id (for legacy ID compatibility)  
SELECT
  id,
  collection_id,
  parent_folder_id,
  item_type,
  folder_id,
  endpoint_id,
  name,
  prev_id,
  next_id
FROM
  collection_items
WHERE
  endpoint_id = ?
LIMIT
  1;

-- name: UpdateCollectionItemParentFolder :exec
-- Update only the parent_folder_id for cross-folder moves
UPDATE collection_items
SET
  parent_folder_id = ?
WHERE
  id = ?;

-- 
-- Cross-Collection Move Support Queries
-- These queries support moving collection items between different collections
--

-- name: ValidateCollectionsInSameWorkspace :one
-- Validate that two collections are in the same workspace
SELECT 
  c1.workspace_id = c2.workspace_id AS same_workspace,
  c1.workspace_id AS source_workspace_id,
  c2.workspace_id AS target_workspace_id
FROM collections c1, collections c2 
WHERE c1.id = ? AND c2.id = ?;

-- name: GetCollectionWorkspaceByItemId :one
-- Get the workspace_id for a collection that contains a specific collection item
SELECT c.workspace_id 
FROM collections c
JOIN collection_items ci ON ci.collection_id = c.id
WHERE ci.id = ?;

-- name: UpdateCollectionItemCollectionId :exec
-- Update the collection_id and parent_folder_id for cross-collection moves
UPDATE collection_items 
SET collection_id = ?, parent_folder_id = ?
WHERE id = ?;

-- name: GetCollectionItemsInOrderForCollection :many
-- Get all collection items in order for a specific collection (used for cross-collection validation)
SELECT id, collection_id, parent_folder_id, item_type, folder_id, endpoint_id, name, prev_id, next_id
FROM collection_items 
WHERE collection_id = ? AND parent_folder_id IS NULL
ORDER BY CASE WHEN prev_id IS NULL THEN 0 ELSE 1 END, id;

-- name: UpdateItemApiCollectionId :exec
-- Update legacy item_api table collection_id for cross-collection moves
UPDATE item_api
SET collection_id = ?
WHERE id = ?;

-- name: UpdateItemFolderCollectionId :exec
-- Update legacy item_folder table collection_id for cross-collection moves  
UPDATE item_folder
SET collection_id = ?
WHERE id = ?;
