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
  name
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
  collections (id, workspace_id, name)
VALUES
  (?, ?, ?);

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
  global_env
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
      workspace_id = ?
      AND user_id = ?
    LIMIT
      1
  )
LIMIT
  1;

-- name: CreateWorkspace :exec
INSERT INTO
  workspaces (id, name, updated, collection_count, flow_count, active_env, global_env)
VALUES
  (?, ?, ?, ?, ?, ?, ?);

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
  value
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
  value
FROM
  example_header
WHERE
  example_id = ?;

-- name: GetHeaderByDeltaParentID :one
SELECT
  id,
  example_id,
  delta_parent_id,
  header_key,
  enable,
  description,
  value
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
  example_header (id, example_id, delta_parent_id, header_key, enable, description, value)
VALUES
  (?, ?, ?, ?, ?, ?, ?);

-- name: CreateHeaderBulk :exec
INSERT INTO
  example_header (id, example_id, delta_parent_id, header_key, enable, description, value)
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
  (?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?);

-- name: UpdateHeader :exec
UPDATE example_header
SET
  header_key = ?,
  enable = ?,
  description = ?,
  value = ?
WHERE
  id = ?;

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
  description
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
  description
FROM
  environment
WHERE
  workspace_id = ?;

-- name: CreateEnvironment :exec
INSERT INTO
  environment (id, workspace_id, type, name, description)
VALUES
  (?, ?, ?, ?, ?);

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
  description
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
  description
FROM
  variable
WHERE
  env_id = ?;

-- name: CreateVariable :exec
INSERT INTO
  variable (id, env_id, var_key, value, enabled, description)
VALUES
  (?, ?, ?, ?, ?, ?);

-- name: CreateVariableBulk :exec
INSERT INTO
  variable (id, env_id, var_key, value, enabled, description)
VALUES
  (?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?);

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
  description
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
  description
FROM
  flow_variable
WHERE
  flow_id = ?;

-- name: CreateFlowVariable :exec
INSERT INTO
  flow_variable (id, flow_id, key, value, enabled, description)
VALUES
  (?, ?, ?, ?, ?, ?);

-- name: CreateFlowVariableBulk :exec
INSERT INTO
  flow_variable (id, flow_id, key, value, enabled, description)
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

-- name: GetNodeExecutionsByNodeID :many
SELECT * FROM node_execution WHERE node_id = ? ORDER BY completed_at DESC;

-- name: GetLatestNodeExecutionByNodeID :one
SELECT * FROM node_execution WHERE node_id = ? ORDER BY id DESC LIMIT 1;
