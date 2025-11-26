--
-- ItemFolder
--


-- name: GetItemFolder :one
SELECT
  id,
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

-- name: GetItemFoldersByParentID :many
SELECT
  id,
  parent_id,
  name,
  prev,
  next
FROM
  item_folder
WHERE
  parent_id = ?;


-- name: GetItemFolderByParentIDAndNextID :one
SELECT
  id,
  parent_id,
  name,
  prev,
  next
FROM
  item_folder
WHERE
  next = ? AND
  parent_id = ?
LIMIT
  1;



-- name: CreateItemFolder :exec
INSERT INTO
    item_folder (id, name, parent_id, prev, next)
VALUES
    (?, ?, ?, ?, ?);

-- name: CreateItemFolderBulk :exec
INSERT INTO
    item_folder (id, name, parent_id, prev, next)
VALUES
  (?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?);

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
