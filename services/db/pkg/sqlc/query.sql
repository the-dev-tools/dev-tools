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
  prev,
  next
FROM
  item_api
WHERE
  collection_id = ? AND
  version_parent_id is NULL;

-- name: GetItemApiOwnerID :one
SELECT
  c.owner_id
FROM
  collections c
  INNER JOIN item_api i ON c.id = i.collection_id
WHERE
  i.id = ?
LIMIT
  1;

-- name: CreateItemApi :exec
INSERT INTO
  item_api (id, collection_id, folder_id, name, url, method, version_parent_id, prev, next)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: CreateItemApiBulk :exec
INSERT INTO
  item_api (id, collection_id, folder_id, name, url, method, version_parent_id, prev, next)
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

-- name: UpdateItemApi :exec
UPDATE item_api
SET
  folder_id = ?,
  name = ?,
  url = ?,
  method = ?
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
  version_parent_id = NULL;

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

-- name: GetItemFolderOwnerID :one
SELECT
  c.owner_id
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
  owner_id,
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
  owner_id,
  name
FROM
  collections
WHERE
  id = ?;

-- name: GetCollectionOwnerID :one
SELECT
  owner_id
FROM
  collections
WHERE
  id = ?
LIMIT
  1;

-- name: GetCollectionByOwnerID :many
SELECT
  id,
  owner_id,
  name
FROM
  collections
WHERE
  owner_id = ?;

-- name: CreateCollection :exec
INSERT INTO
  collections (id, owner_id, name)
VALUES
  (?, ?, ?);

-- name: UpdateCollection :exec
UPDATE collections
SET
  owner_id = ?,
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
  updated
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
  updated
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
  updated
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
  updated
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
  workspaces (id, name, updated)
VALUES
  (?, ?, ?);

-- name: UpdateWorkspace :exec
UPDATE workspaces
SET
  name = ?
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

--
-- ResultAPI
--
-- name: GetResultApi :one
SELECT
  *
FROM
  result_api
WHERE
  id = ?
LIMIT
  1;

-- name: GetResultApiByTriggerBy :many
SELECT
  *
FROM
  result_api
WHERE
  trigger_by = ?;

-- name: GetResultApiByTriggerByAndTriggerType :many
SELECT
  *
FROM
  result_api
WHERE
  trigger_by = ?
  AND trigger_type = ?;

-- name: CreateResultApi :exec
INSERT INTO
  result_api (
    id,
    trigger_type,
    trigger_by,
    name,
    status,
    time,
    duration,
    http_resp
  )
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateResultApi :exec
UPDATE result_api
SET
  name = ?,
  status = ?,
  time = ?,
  duration = ?,
  http_resp = ?
WHERE
  id = ?;

-- name: DeleteResultApi :exec
DELETE FROM result_api
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
  header_key,
  enable,
  description,
  value
FROM
  example_header
WHERE
  example_id = ?;

-- name: SetHeaderEnable :exec
UPDATE example_header
    SET
  enable = ?
WHERE
  id = ?;

-- name: CreateHeader :exec
INSERT INTO
  example_header (id, example_id, header_key, enable, description, value)
VALUES
  (?, ?, ?, ?, ?, ?);

-- name: CreateHeaderBulk :exec
INSERT INTO
  example_header (id, example_id, header_key, enable, description, value)
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
  (?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?);

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
  query_key,
  enable,
  description,
  value
FROM
  example_query
WHERE
  example_id = ?;

-- name: CreateQuery :exec
INSERT INTO
  example_query (id, example_id, query_key, enable, description, value)
VALUES
  (?, ?, ?, ?, ?, ?);

-- name: CreateQueryBulk :exec
INSERT INTO
  example_query (id, example_id, query_key, enable, description, value)
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

-- name: SetQueryEnable :exec
UPDATE example_query
SET
  enable = ?
WHERE
  id = ?;

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

/*
*
* body_form
*
*/

-- name: GetBodyForm :one
SELECT
  id,
  example_id,
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
  body_key,
  enable,
  description,
  value
FROM
    example_body_form
WHERE
    example_id = ?;

--
-- BodyForm
--

-- name: CreateBodyForm :exec
INSERT INTO
  example_body_form (id, example_id, body_key, enable, description, value)
VALUES
  (?, ?, ?, ?, ?, ?);

-- name: CreateBodyFormBulk :exec
INSERT INTO
  example_body_form (id, example_id, body_key, enable, description, value)
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
  body_key,
  enable,
  description,
  value
FROM
  example_body_urlencoded
WHERE
  example_id = ?;

-- name: CreateBodyUrlEncoded :exec
INSERT INTO
  example_body_urlencoded (id, example_id, body_key, enable, description, value)
VALUES
    (?, ?, ?, ?, ?, ?);

-- name: CreateBodyUrlEncodedBulk :exec
INSERT INTO
  example_body_urlencoded (id, example_id, body_key, enable, description, value)
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
  active,
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
  active,
  type,
  name,
  description
FROM
  environment
WHERE
  workspace_id = ?;

-- name: GetActiveEnvironmentsByWorkspaceID :one
SELECT
  id,
  workspace_id,
  active,
  type,
  name,
  description
FROM
  environment
WHERE
  workspace_id = ? AND active = true
LIMIT 1;

-- name: CreateEnvironment :exec
INSERT INTO
  environment (id, workspace_id, active, type, name, description)
VALUES
  (?, ?, ?, ?, ?, ?);

-- name: UpdateEnvironment :exec
UPDATE environment
SET
    active = ?,
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

-- name: GetExampleRespsByExampleID :one
SELECT
    *
FROM
  example_resp
WHERE
  example_id = ?
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
  type,
  path,
  value,
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
  type,
  path,
  value,
  enable,
  prev,
  next
FROM
  assertion
WHERE
  example_id = ?;

-- name: CreateAssert :exec
INSERT INTO
  assertion (id, example_id, type, path, value, enable, prev, next)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateAssert :exec
UPDATE assertion
SET
  type = ?,
  path = ?,
  value = ?,
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
  source_handle
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
  source_handle
FROM
  flow_edge
WHERE
  flow_id = ?;

-- name: CreateFlowEdge :exec
INSERT INTO
  flow_edge (id, flow_id, source_id, target_id, source_handle)
VALUES
  (?, ?, ?, ?, ?);

-- name: UpdateFlowEdge :exec
UPDATE flow_edge
SET
  source_id = ?,
  target_id = ?,
  source_handle = ?
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
  condition_path,
  condition_type,
  value
FROM
  flow_node_for
WHERE
  flow_node_id = ?
LIMIT 1;

-- name: CreateFlowNodeFor :exec
INSERT INTO
  flow_node_for (flow_node_id, iter_count, error_handling, condition_path, condition_type, value)
VALUES
  (?, ?, ?, ?, ?, ?);

-- name: UpdateFlowNodeFor :exec
UPDATE flow_node_for
SET
  iter_count = ?,
  error_handling = ?,
  condition_path = ?,
  condition_type = ?,
  value = ?
WHERE
  flow_node_id = ?;

-- name: DeleteFlowNodeFor :exec
DELETE FROM flow_node_for
WHERE
  flow_node_id = ?;

-- name: GetFlowNodeForEach :one
SELECT
  flow_node_id,
  iter_path,
  error_handling,
  condition_path,
  condition_type,
  value
FROM
  flow_node_for_each
WHERE
  flow_node_id = ?
LIMIT 1;

-- name: CreateFlowNodeForEach :exec
INSERT INTO
  flow_node_for_each (flow_node_id, iter_path, error_handling, condition_path, condition_type, value)
VALUES
  (?, ?, ?, ?, ?, ?);

-- name: UpdateFlowNodeForEach :exec
UPDATE flow_node_for_each
SET
  iter_path = ?,
  error_handling = ?,
  condition_path = ?,
  condition_type = ?,
  value = ?
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
  delta_example_id
FROM
  flow_node_request
WHERE
  flow_node_id = ?
LIMIT 1;

-- name: CreateFlowNodeRequest :exec
INSERT INTO
  flow_node_request (flow_node_id, endpoint_id, example_id, delta_example_id)
VALUES
  (?, ?, ?, ?);

-- name: UpdateFlowNodeRequest :exec
UPDATE flow_node_request
SET
  endpoint_id = ?,
  example_id = ?,
  delta_example_id = ?
WHERE
  flow_node_id = ?;

-- name: DeleteFlowNodeRequest :exec
DELETE FROM flow_node_request
WHERE
  flow_node_id = ?;

-- name: GetFlowNodeIf :one
SELECT
  flow_node_id,
  condition_type,
  path,
  value
FROM
  flow_node_if
WHERE
  flow_node_id = ?
LIMIT 1;

-- name: CreateFlowNodeIf :exec
INSERT INTO
  flow_node_if (flow_node_id, condition_type, path, value)
VALUES
  (?, ?, ?, ?);

-- name: UpdateFlowNodeIf :exec
UPDATE flow_node_if
SET
  condition_type = ?,
  path = ?,
  value = ?
WHERE
  flow_node_id = ?;

-- name: DeleteFlowNodeIf :exec
DELETE FROM flow_node_if
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
