--
-- ItemApi
--
-- name: GetItemApi :one
SELECT
  id,
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

-- name: GetItemApisByIDs :many
SELECT
  id,
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
  id IN (sqlc.slice('ids'));


-- name: GetItemApiByFolderIDAndNextID :one
SELECT
  id,
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
  folder_id = ?
LIMIT
  1;

-- name: GetItemsApiByFolderID :many
SELECT
  id,
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
  folder_id = ? AND
  version_parent_id is NULL AND
  delta_parent_id is NULL AND
  hidden = FALSE;

-- name: GetAllItemsApiByFolderID :many
SELECT
  id,
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
  folder_id = ? AND
  version_parent_id is NULL AND
  delta_parent_id is NULL;



-- name: CreateItemApi :exec
INSERT INTO
  item_api (id, folder_id, name, url, method, version_parent_id, delta_parent_id, hidden, prev, next)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: CreateItemApiBulk :exec
INSERT INTO
  item_api (id, folder_id, name, url, method, version_parent_id, delta_parent_id, hidden, prev, next)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

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

-- name: GetItemApiByFolderIDAndURLAndMethod :one
SELECT
  id,
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
  folder_id = ? AND
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

-- name: GetItemApiExamplesByIDs :many
SELECT
    id,
    item_api_id,
    is_default,
    body_type,
    name,
    version_parent_id,
    prev,
    next
FROM
  item_api_example
WHERE
  id IN (sqlc.slice('ids'));

-- name: GetItemExampleByNextIDAndItemApiID :one
SELECT
    id,
    item_api_id,
    is_default,
    body_type,
    name,
    version_parent_id,
    prev,
    next
FROM
  item_api_example
WHERE
  next = ? AND
  prev = ?
LIMIT
  1;

-- name: GetItemApiExamples :many
SELECT
    id,
    item_api_id,
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

-- name: GetItemApiExamplesByItemApiID :many
SELECT
    id,
    item_api_id,
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

-- name: GetItemApiExampleByVersionParentID :many
SELECT
    id,
    item_api_id,
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
    is_default,
    body_type,
    name,
    version_parent_id,
    prev,
    next
  )
VALUES
    (?, ?, ?, ?, ?, ?, ?, ?);

-- name: CreateItemApiExampleBulk :exec
INSERT INTO
  item_api_example (
    id,
    item_api_id,
    is_default,
    body_type,
    name,
    version_parent_id,
    prev,
    next
  )
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
