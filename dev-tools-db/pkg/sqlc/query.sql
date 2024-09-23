--
-- This file is the source of truth for saas application's schema
--
-- 
-- ItemApi 
--
-- name: GetItemApi :one
SELECT
  id,
  collection_id,
  parent_id,
  name,
  url,
  method,
  prev,
  next
FROM
  item_api
WHERE
  id = ?
LIMIT
  1;

-- name: GetItemsApiByCollectionID :many
SELECT
  id,
  collection_id,
  parent_id,
  name,
  url,
  method,
  prev,
  next
FROM
  item_api
WHERE
  collection_id = ?;

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
  item_api (id, collection_id, parent_id, name, url, method, prev, next)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?);

-- name: CreateItemApiBulk :exec
INSERT INTO
  item_api (id, collection_id, parent_id, name, url, method, prev, next)
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

-- name: UpdateItemApi :exec
UPDATE item_api
SET
  collection_id = ?,
  parent_id = ?,
  name = ?,
  url = ?,
  method = ?
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
  parent_example_id,
  is_default,
  body_type,
  name,
  prev,
  next
FROM
  item_api_example
WHERE
  id = ?
LIMIT
  1;

-- name: GetItemApiExamples :many
SELECT
  id,
  item_api_id,
  collection_id,
  parent_example_id,
  is_default,
  body_type,
  name,
  prev,
  next
FROM
  item_api_example
WHERE
  item_api_id = ?
  AND is_default = false;

-- name: GetItemApiExampleDefault :one
SELECT
  id,
  item_api_id,
  collection_id,
  parent_example_id,
  is_default,
  body_type,
  name,
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
  parent_example_id,
  is_default,
  body_type,
  name,
  prev,
  next
FROM
  item_api_example
WHERE
  collection_id = ?;

-- name: CreateItemApiExample :exec
INSERT INTO
  item_api_example (
    id,
    item_api_id,
    collection_id,
    parent_example_id,
    is_default,
    body_type,
    name,
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
    parent_example_id,
    is_default,
    body_type,
    name,
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

-- name: DeleteItemApiExample :exec
DELETE FROM item_api_example
WHERE
  id = ?;

--
-- ItemFolder
--


-- name: GetItemFolder :one
SELECT
    *
FROM
  item_folder
WHERE
  id = ?
LIMIT
  1;

-- name: GetItemFoldersByCollectionID :many
SELECT
    *
FROM
  item_folder
WHERE
  collection_id = ?;

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

-- name: CreateUser :one
INSERT INTO
  users (
    id,
    email,
    password_hash,
    provider_type,
    provider_id
  )
VALUES
  (?, ?, ?, ?, ?) RETURNING *;

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
  is_default,
  name
FROM 
  environment
WHERE
  id = ?
LIMIT 1;

-- name: GetEnvironmentsByWorkspaceID :many
SELECT
  id,
  workspace_id,
  is_default,
  name
FROM 
  environment
WHERE
  workspace_id = ?;

-- CreateEnvironment :exec
INSERT INTO
  environment (id, workspace_id, is_default, name)
VALUES
  (?, ?, ?, ?);

-- name: UpdateEnvironment :exec
UPDATE environment
SET
    is_default = ?
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
  value
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
  value
FROM
  variable
WHERE
  env_id = ?;

-- name: CreateVariable :exec
INSERT INTO
  variable (id, env_id, var_key, value)
VALUES
  (?, ?, ?, ?);

-- name: CreateVariableBulk :exec
INSERT INTO
  variable (id, env_id, var_key, value)
VALUES
  (?, ?, ?, ?),
  (?, ?, ?, ?),
  (?, ?, ?, ?),
  (?, ?, ?, ?),
  (?, ?, ?, ?);

-- name: UpdateVariable :exec
UPDATE variable
SET
  var_key = ?,
  value = ?
WHERE
  id = ?;

-- name: DeleteVariable :exec
DELETE FROM variable
WHERE
  id = ?;
