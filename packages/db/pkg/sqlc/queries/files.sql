--
-- File System
--

-- name: GetFile :one
-- Get a single file by ID
SELECT id, workspace_id, folder_id, content_id, content_kind, name, display_order, updated_at
FROM files
WHERE id = ?;

-- name: CreateFile :exec
-- Create a new file
INSERT INTO files (id, workspace_id, folder_id, content_id, content_kind, name, display_order, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateFile :exec
-- Update an existing file
UPDATE files 
SET workspace_id = ?, folder_id = ?, content_id = ?, content_kind = ?, name = ?, display_order = ?, updated_at = ?
WHERE id = ?;

-- name: DeleteFile :exec
-- Delete a file by ID
DELETE FROM files WHERE id = ?;

-- name: GetFilesByWorkspaceID :many
-- Get all files in a workspace (unordered)
SELECT id, workspace_id, folder_id, content_id, content_kind, name, display_order, updated_at
FROM files
WHERE workspace_id = ?;

-- name: GetFilesByWorkspaceIDOrdered :many
-- Get all files in a workspace ordered by display_order
SELECT id, workspace_id, folder_id, content_id, content_kind, name, display_order, updated_at
FROM files
WHERE workspace_id = ?
ORDER BY display_order, id;

-- name: GetFilesByFolderID :many
-- Get all files directly under a folder (unordered)
SELECT id, workspace_id, folder_id, content_id, content_kind, name, display_order, updated_at
FROM files
WHERE folder_id = ?;

-- name: GetFilesByFolderIDOrdered :many
-- Get all files directly under a folder ordered by display_order
SELECT id, workspace_id, folder_id, content_id, content_kind, name, display_order, updated_at
FROM files
WHERE folder_id = ?
ORDER BY display_order, id;

-- name: GetRootFilesByWorkspaceID :many
-- Get root-level files (no parent folder) in a workspace ordered by display_order
SELECT id, workspace_id, folder_id, content_id, content_kind, name, display_order, updated_at
FROM files
WHERE workspace_id = ? AND folder_id IS NULL
ORDER BY display_order, id;

-- name: GetFileWorkspaceID :one
-- Get the workspace_id for a file
SELECT workspace_id 
FROM files
WHERE id = ?;

-- name: GetFileWithContent :one
-- Get a file with its content (two-query pattern for union types)
SELECT id, workspace_id, folder_id, content_id, content_kind, name, display_order, updated_at
FROM files
WHERE id = ?;

-- name: GetFolderContent :one
-- Get folder content by content_id (for union type resolution)
SELECT id, name
FROM item_folder
WHERE id = ?;

-- name: GetAPIContent :one
-- Get API content by content_id (for union type resolution)
SELECT id, name, url, method
FROM item_api
WHERE id = ?;

-- name: GetFlowContent :one
-- Get flow content by content_id (for union type resolution)
SELECT id, name, duration
FROM flow
WHERE id = ?;
