--
-- This file is the source of truth for saas application's schema
--

-- 
-- ItemApi 
--

-- name: GetItemApi :one
SELECT id, collection_id, parent_id, name, url, method, headers, query, body
FROM item_api
WHERE id = ? LIMIT 1;

-- name: GetItemsApiByCollectionID :many
SELECT id, collection_id, parent_id, name, url, method, headers, query, body
FROM item_api
WHERE collection_id = ?;

-- name: GetItemApiOwnerID :one
SELECT c.owner_id FROM collections c
INNER JOIN item_api i ON c.id = i.collection_id
WHERE i.id = ? LIMIT 1;

-- name: CreateItemApi :one
INSERT INTO item_api (id, collection_id, parent_id, name, url, method, headers, query, body)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING NULL; 

-- name: UpdateItemApi :exec
UPDATE item_api
SET collection_id = ?, parent_id = ?, name = ?, url = ?, method = ?, headers = ?, query = ?, body = ?
WHERE id = ?;

-- name: DeleteItemApi :exec
DELETE FROM item_api
WHERE id = ?;

--
-- ItemFolder
--

-- name: GetItemFolder :one
SELECT id, name, parent_id, collection_id
FROM item_folder
WHERE id = ? LIMIT 1;

-- name: GetItemFolderByCollectionID :many
SELECT id, name, parent_id, collection_id
FROM item_folder
WHERE collection_id = ?;

-- name: GetItemFolderOwnerID :one
SELECT c.owner_id FROM collections c
INNER JOIN item_folder i ON c.id = i.collection_id
WHERE i.id = ? LIMIT 1;

-- name: CreateItemFolder :exec
INSERT INTO item_folder (id, name, parent_id, collection_id)
VALUES (?, ?, ?, ?);

-- name: UpdateItemFolder :exec
UPDATE item_folder
SET name = ?
WHERE id = ?;

-- name: DeleteItemFolder :exec
DELETE FROM item_folder
WHERE id = ?;

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
FROM users WHERE id = ? LIMIT 1;

-- name: GetUserByEmail :one
SELECT
    id,
    email,
    password_hash,
    provider_type,
    provider_id
FROM users WHERE email = ? LIMIT 1;

-- name: GetUserByEmailAndProviderType :one
SELECT
        id,
        email,
        password_hash,
        provider_type,
        provider_id
FROM users WHERE email = ? AND provider_type = ? LIMIT 1;

-- name: GetUserByProviderIDandType :one
SELECT
    id,
    email,
    password_hash,
    provider_type,
    provider_id
FROM users WHERE provider_id = ? AND provider_type = ? LIMIT 1;

-- name: CreateUser :one
INSERT INTO users 
(id, email, password_hash, provider_type, provider_id)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateUser :exec
UPDATE users 
SET email = ?,
password_hash = ?
WHERE id = ?;

-- name: DeleteUser :exec
DELETE FROM users 
WHERE id = ?;


-- 
-- Collections
--

-- name: GetCollection :one
SELECT id, owner_id, name
FROM collections
WHERE id = ?
LIMIT 1;


-- name: GetCollectionByPlatformIDandType :many
SELECT id, owner_id, name
FROM collections
WHERE id = ?;


-- name: GetCollectionOwnerID :one
SELECT owner_id
FROM collections
WHERE id = ? LIMIT 1;

-- name: GetCollectionByOwnerID :many
SELECT id, owner_id, name
FROM collections
WHERE owner_id = ?;


-- name: CreateCollection :one
INSERT INTO collections (id, owner_id, name)
VALUES (?, ?, ?) RETURNING id, owner_id, name;

-- name: UpdateCollection :exec
UPDATE collections 
SET owner_id = ?, name = ?
WHERE id = ?;

-- name: DeleteCollection :exec
DELETE FROM collections 
WHERE id = ?;


--
-- Workspaces
--

-- name: GetWorkspace :one
SELECT id, name
FROM workspaces
WHERE id = ? LIMIT 1;

-- name: GetWorkspaceByUserID :one
SELECT id, name FROM workspaces WHERE id = (SELECT workspace_id FROM workspaces_users WHERE user_id = ? LIMIT 1) LIMIT 1;

-- name: GetWorkspacesByUserID :many
SELECT id, name FROM workspaces WHERE id IN (SELECT workspace_id FROM workspaces_users WHERE user_id = ?);

-- name: GetWorkspaceByUserIDandWorkspaceID :one
SELECT id, name FROM workspaces WHERE id = (SELECT workspace_id FROM workspaces_users WHERE workspace_id = ? AND user_id = ? LIMIT 1) LIMIT 1;

-- name: CreateWorkspace :exec
INSERT INTO workspaces (id, name)
VALUES (?, ?);

-- name: UpdateWorkspace :exec
UPDATE workspaces
SET name = ?
WHERE id = ?;

-- name: DeleteWorkspace :exec
DELETE FROM workspaces
WHERE id = ?;

-- 
-- WorkspaceUsers
--

-- name: GetWorkspaceUser :one
SELECT id, workspace_id, user_id FROM workspaces_users
WHERE id = ? LIMIT 1;

-- name: GetWorkspaceUserByUserID :one
SELECT id, workspace_id, user_id FROM workspaces_users
WHERE user_id = ? LIMIT 1;

-- name: GetWorkspaceUserByWorkspaceID :one
SELECT id, workspace_id, user_id FROM workspaces_users
WHERE workspace_id = ? LIMIT 1;

-- name: CreateWorkspaceUser :exec
INSERT INTO workspaces_users (id, workspace_id, user_id)
VALUES (?, ?, ?);

-- name: UpdateWorkspaceUser :exec
UPDATE workspaces_users
SET workspace_id = ?, user_id = ?
WHERE id = ?;

-- name: DeleteWorkspaceUser :exec
DELETE FROM workspaces_users
WHERE id = ?;
