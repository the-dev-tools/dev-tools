--
-- This file is the source of truth for application's db schema
--
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

--
-- Overlay (Delta) queries: URL-encoded
--
-- Order
-- name: DeltaUrlencOrderCount :one
SELECT COUNT(*) FROM delta_urlenc_order WHERE example_id = ?;

-- name: DeltaUrlencOrderSelectAsc :many
SELECT ref_kind, ref_id, rank, revision FROM delta_urlenc_order WHERE example_id = ? ORDER BY rank ASC;

-- name: DeltaUrlencOrderLastRank :one
SELECT rank FROM delta_urlenc_order WHERE example_id = ? ORDER BY rank DESC LIMIT 1;

SELECT COALESCE(MAX(revision), 0) FROM delta_urlenc_order WHERE example_id = ?;

-- name: DeltaUrlencOrderInsertIgnore :exec
INSERT OR IGNORE INTO delta_urlenc_order(example_id, ref_kind, ref_id, rank, revision) VALUES (?,?,?,?,?);

-- name: DeltaUrlencOrderUpsert :exec
INSERT INTO delta_urlenc_order(example_id, ref_kind, ref_id, rank, revision)
VALUES (?,?,?,?,?)
ON CONFLICT(example_id, ref_kind, ref_id) DO UPDATE SET rank=excluded.rank, revision=excluded.revision;

-- name: DeltaUrlencOrderDeleteByRef :exec
DELETE FROM delta_urlenc_order WHERE example_id = ? AND ref_id = ?;

-- name: DeltaUrlencOrderExists :one
SELECT 1 FROM delta_urlenc_order WHERE example_id = ? AND ref_kind = ? AND ref_id = ?;

-- Delta rows
-- name: DeltaUrlencDeltaInsert :exec
INSERT INTO delta_urlenc_delta(example_id, id, body_key, value, description, enabled) VALUES (?,?,?,?,?, ?);

-- name: DeltaUrlencDeltaUpdate :exec
UPDATE delta_urlenc_delta SET body_key = ?, value = ?, description = ?, enabled = ?, updated_at = unixepoch() WHERE example_id = ? AND id = ?;

-- name: DeltaUrlencDeltaExists :one
SELECT 1 FROM delta_urlenc_delta WHERE example_id = ? AND id = ?;

-- name: DeltaUrlencDeltaGet :one
SELECT body_key, value, description, enabled FROM delta_urlenc_delta WHERE example_id = ? AND id = ?;

-- name: DeltaUrlencDeltaDelete :exec
DELETE FROM delta_urlenc_delta WHERE example_id = ? AND id = ?;

-- State
-- name: DeltaUrlencStateUpsert :exec
INSERT INTO delta_urlenc_state(example_id, origin_id, suppressed, body_key, value, description, enabled, updated_at)
VALUES (?,?,?,?,?,?,?, unixepoch())
ON CONFLICT(example_id, origin_id) DO UPDATE SET
  suppressed=excluded.suppressed,
  body_key=COALESCE(excluded.body_key, delta_urlenc_state.body_key),
  value=COALESCE(excluded.value, delta_urlenc_state.value),
  description=COALESCE(excluded.description, delta_urlenc_state.description),
  enabled=COALESCE(excluded.enabled, delta_urlenc_state.enabled),
  updated_at=unixepoch();

-- name: DeltaUrlencStateGet :one
SELECT suppressed, body_key, value, description, enabled FROM delta_urlenc_state WHERE example_id = ? AND origin_id = ?;

-- name: DeltaUrlencStateClearOverrides :exec
UPDATE delta_urlenc_state SET body_key=NULL, value=NULL, description=NULL, enabled=NULL, updated_at = unixepoch() WHERE example_id = ? AND origin_id = ?;

-- name: DeltaUrlencStateSuppress :exec
INSERT INTO delta_urlenc_state(example_id, origin_id, suppressed, updated_at)
VALUES (?,?, TRUE, unixepoch())
ON CONFLICT(example_id, origin_id) DO UPDATE SET suppressed=TRUE, updated_at=unixepoch();

-- name: DeltaUrlencStateUnsuppress :exec
UPDATE delta_urlenc_state SET suppressed = FALSE, updated_at = unixepoch() WHERE example_id = ? AND origin_id = ?;

-- name: DeltaUrlencResolveExampleByDeltaID :one
SELECT example_id FROM delta_urlenc_delta WHERE id = ? LIMIT 1;

-- name: DeltaUrlencResolveExampleByOrderRefID :one
SELECT example_id FROM delta_urlenc_order WHERE ref_id = ? LIMIT 1;

--
-- Overlay (Delta) queries: Headers
--
-- Order
-- name: DeltaHeaderOrderCount :one
SELECT COUNT(*) FROM delta_header_order WHERE example_id = ?;
-- name: DeltaHeaderOrderSelectAsc :many
SELECT ref_kind, ref_id, rank, revision FROM delta_header_order WHERE example_id = ? ORDER BY rank ASC;
-- name: DeltaHeaderOrderLastRank :one
SELECT rank FROM delta_header_order WHERE example_id = ? ORDER BY rank DESC LIMIT 1;
SELECT COALESCE(MAX(revision), 0) FROM delta_header_order WHERE example_id = ?;
-- name: DeltaHeaderOrderInsertIgnore :exec
INSERT OR IGNORE INTO delta_header_order(example_id, ref_kind, ref_id, rank, revision) VALUES (?,?,?,?,?);
-- name: DeltaHeaderOrderUpsert :exec
INSERT INTO delta_header_order(example_id, ref_kind, ref_id, rank, revision)
VALUES (?,?,?,?,?)
ON CONFLICT(example_id, ref_kind, ref_id) DO UPDATE SET rank=excluded.rank, revision=excluded.revision;
-- name: DeltaHeaderOrderDeleteByRef :exec
DELETE FROM delta_header_order WHERE example_id = ? AND ref_id = ?;
-- name: DeltaHeaderOrderExists :one
SELECT 1 FROM delta_header_order WHERE example_id = ? AND ref_kind = ? AND ref_id = ?;

-- Delta rows
-- name: DeltaHeaderDeltaInsert :exec
INSERT INTO delta_header_delta(example_id, id, header_key, value, description, enabled) VALUES (?,?,?,?,?, ?);
-- name: DeltaHeaderDeltaUpdate :exec
UPDATE delta_header_delta SET header_key = ?, value = ?, description = ?, enabled = ?, updated_at = unixepoch() WHERE example_id = ? AND id = ?;
-- name: DeltaHeaderDeltaGet :one
SELECT header_key, value, description, enabled FROM delta_header_delta WHERE example_id = ? AND id = ?;
-- name: DeltaHeaderDeltaExists :one
SELECT 1 FROM delta_header_delta WHERE example_id = ? AND id = ?;
-- name: DeltaHeaderDeltaDelete :exec
DELETE FROM delta_header_delta WHERE example_id = ? AND id = ?;

-- State
-- name: DeltaHeaderStateUpsert :exec
INSERT INTO delta_header_state(example_id, origin_id, suppressed, header_key, value, description, enabled, updated_at)
VALUES (?,?,?,?,?,?,?, unixepoch())
ON CONFLICT(example_id, origin_id) DO UPDATE SET
  suppressed=excluded.suppressed,
  header_key=COALESCE(excluded.header_key, delta_header_state.header_key),
  value=COALESCE(excluded.value, delta_header_state.value),
  description=COALESCE(excluded.description, delta_header_state.description),
  enabled=COALESCE(excluded.enabled, delta_header_state.enabled),
  updated_at=unixepoch();
-- name: DeltaHeaderStateGet :one
SELECT suppressed, header_key, value, description, enabled FROM delta_header_state WHERE example_id = ? AND origin_id = ?;
-- name: DeltaHeaderStateClearOverrides :exec
UPDATE delta_header_state SET header_key=NULL, value=NULL, description=NULL, enabled=NULL, updated_at = unixepoch() WHERE example_id = ? AND origin_id = ?;
-- name: DeltaHeaderStateSuppress :exec
INSERT INTO delta_header_state(example_id, origin_id, suppressed, updated_at)
VALUES (?,?, TRUE, unixepoch())
ON CONFLICT(example_id, origin_id) DO UPDATE SET suppressed=TRUE, updated_at=unixepoch();
-- name: DeltaHeaderStateUnsuppress :exec
UPDATE delta_header_state SET suppressed = FALSE, updated_at = unixepoch() WHERE example_id = ? AND origin_id = ?;
-- name: DeltaHeaderResolveExampleByDeltaID :one
SELECT example_id FROM delta_header_delta WHERE id = ? LIMIT 1;
-- name: DeltaHeaderResolveExampleByOrderRefID :one
SELECT example_id FROM delta_header_order WHERE ref_id = ? LIMIT 1;

--
-- Overlay (Delta) queries: Queries
--
-- Order
-- name: DeltaQueryOrderCount :one
SELECT COUNT(*) FROM delta_query_order WHERE example_id = ?;
-- name: DeltaQueryOrderSelectAsc :many
SELECT ref_kind, ref_id, rank, revision FROM delta_query_order WHERE example_id = ? ORDER BY rank ASC;
-- name: DeltaQueryOrderLastRank :one
SELECT rank FROM delta_query_order WHERE example_id = ? ORDER BY rank DESC LIMIT 1;
SELECT COALESCE(MAX(revision), 0) FROM delta_query_order WHERE example_id = ?;
-- name: DeltaQueryOrderInsertIgnore :exec
INSERT OR IGNORE INTO delta_query_order(example_id, ref_kind, ref_id, rank, revision) VALUES (?,?,?,?,?);
-- name: DeltaQueryOrderUpsert :exec
INSERT INTO delta_query_order(example_id, ref_kind, ref_id, rank, revision)
VALUES (?,?,?,?,?)
ON CONFLICT(example_id, ref_kind, ref_id) DO UPDATE SET rank=excluded.rank, revision=excluded.revision;
-- name: DeltaQueryOrderDeleteByRef :exec
DELETE FROM delta_query_order WHERE example_id = ? AND ref_id = ?;
-- name: DeltaQueryOrderExists :one
SELECT 1 FROM delta_query_order WHERE example_id = ? AND ref_kind = ? AND ref_id = ?;

-- Delta rows
-- name: DeltaQueryDeltaInsert :exec
INSERT INTO delta_query_delta(example_id, id, query_key, value, description, enabled) VALUES (?,?,?,?,?, ?);
-- name: DeltaQueryDeltaUpdate :exec
UPDATE delta_query_delta SET query_key = ?, value = ?, description = ?, enabled = ?, updated_at = unixepoch() WHERE example_id = ? AND id = ?;
-- name: DeltaQueryDeltaGet :one
SELECT query_key, value, description, enabled FROM delta_query_delta WHERE example_id = ? AND id = ?;
-- name: DeltaQueryDeltaExists :one
SELECT 1 FROM delta_query_delta WHERE example_id = ? AND id = ?;
-- name: DeltaQueryDeltaDelete :exec
DELETE FROM delta_query_delta WHERE example_id = ? AND id = ?;

-- State
-- name: DeltaQueryStateUpsert :exec
INSERT INTO delta_query_state(example_id, origin_id, suppressed, query_key, value, description, enabled, updated_at)
VALUES (?,?,?,?,?,?,?, unixepoch())
ON CONFLICT(example_id, origin_id) DO UPDATE SET
  suppressed=excluded.suppressed,
  query_key=COALESCE(excluded.query_key, delta_query_state.query_key),
  value=COALESCE(excluded.value, delta_query_state.value),
  description=COALESCE(excluded.description, delta_query_state.description),
  enabled=COALESCE(excluded.enabled, delta_query_state.enabled),
  updated_at=unixepoch();
-- name: DeltaQueryStateGet :one
SELECT suppressed, query_key, value, description, enabled FROM delta_query_state WHERE example_id = ? AND origin_id = ?;
-- name: DeltaQueryStateClearOverrides :exec
UPDATE delta_query_state SET query_key=NULL, value=NULL, description=NULL, enabled=NULL, updated_at = unixepoch() WHERE example_id = ? AND origin_id = ?;
-- name: DeltaQueryStateSuppress :exec
INSERT INTO delta_query_state(example_id, origin_id, suppressed, updated_at)
VALUES (?,?, TRUE, unixepoch())
ON CONFLICT(example_id, origin_id) DO UPDATE SET suppressed=TRUE, updated_at=unixepoch();
-- name: DeltaQueryStateUnsuppress :exec
UPDATE delta_query_state SET suppressed = FALSE, updated_at = unixepoch() WHERE example_id = ? AND origin_id = ?;
-- name: DeltaQueryResolveExampleByDeltaID :one
SELECT example_id FROM delta_query_delta WHERE id = ? LIMIT 1;
-- name: DeltaQueryResolveExampleByOrderRefID :one
SELECT example_id FROM delta_query_order WHERE ref_id = ? LIMIT 1;

--
-- Overlay (Delta) queries: Forms
--
-- Order
-- name: DeltaFormOrderCount :one
SELECT COUNT(*) FROM delta_form_order WHERE example_id = ?;
-- name: DeltaFormOrderSelectAsc :many
SELECT ref_kind, ref_id, rank, revision FROM delta_form_order WHERE example_id = ? ORDER BY rank ASC;
-- name: DeltaFormOrderLastRank :one
SELECT rank FROM delta_form_order WHERE example_id = ? ORDER BY rank DESC LIMIT 1;
SELECT COALESCE(MAX(revision), 0) FROM delta_form_order WHERE example_id = ?;
-- name: DeltaFormOrderInsertIgnore :exec
INSERT OR IGNORE INTO delta_form_order(example_id, ref_kind, ref_id, rank, revision) VALUES (?,?,?,?,?);
-- name: DeltaFormOrderUpsert :exec
INSERT INTO delta_form_order(example_id, ref_kind, ref_id, rank, revision)
VALUES (?,?,?,?,?)
ON CONFLICT(example_id, ref_kind, ref_id) DO UPDATE SET rank=excluded.rank, revision=excluded.revision;
-- name: DeltaFormOrderDeleteByRef :exec
DELETE FROM delta_form_order WHERE example_id = ? AND ref_id = ?;
-- name: DeltaFormOrderExists :one
SELECT 1 FROM delta_form_order WHERE example_id = ? AND ref_kind = ? AND ref_id = ?;

-- Delta rows
-- name: DeltaFormDeltaInsert :exec
INSERT INTO delta_form_delta(example_id, id, body_key, value, description, enabled) VALUES (?,?,?,?,?, ?);
-- name: DeltaFormDeltaUpdate :exec
UPDATE delta_form_delta SET body_key = ?, value = ?, description = ?, enabled = ?, updated_at = unixepoch() WHERE example_id = ? AND id = ?;
-- name: DeltaFormDeltaGet :one
SELECT body_key, value, description, enabled FROM delta_form_delta WHERE example_id = ? AND id = ?;
-- name: DeltaFormDeltaExists :one
SELECT 1 FROM delta_form_delta WHERE example_id = ? AND id = ?;
-- name: DeltaFormDeltaDelete :exec
DELETE FROM delta_form_delta WHERE example_id = ? AND id = ?;

-- State
-- name: DeltaFormStateUpsert :exec
INSERT INTO delta_form_state(example_id, origin_id, suppressed, body_key, value, description, enabled, updated_at)
VALUES (?,?,?,?,?,?,?, unixepoch())
ON CONFLICT(example_id, origin_id) DO UPDATE SET
  suppressed=excluded.suppressed,
  body_key=COALESCE(excluded.body_key, delta_form_state.body_key),
  value=COALESCE(excluded.value, delta_form_state.value),
  description=COALESCE(excluded.description, delta_form_state.description),
  enabled=COALESCE(excluded.enabled, delta_form_state.enabled),
  updated_at=unixepoch();
-- name: DeltaFormStateGet :one
SELECT suppressed, body_key, value, description, enabled FROM delta_form_state WHERE example_id = ? AND origin_id = ?;
-- name: DeltaFormStateClearOverrides :exec
UPDATE delta_form_state SET body_key=NULL, value=NULL, description=NULL, enabled=NULL, updated_at = unixepoch() WHERE example_id = ? AND origin_id = ?;
-- name: DeltaFormStateSuppress :exec
INSERT INTO delta_form_state(example_id, origin_id, suppressed, updated_at)
VALUES (?,?, TRUE, unixepoch())
ON CONFLICT(example_id, origin_id) DO UPDATE SET suppressed=TRUE, updated_at=unixepoch();
-- name: DeltaFormStateUnsuppress :exec
UPDATE delta_form_state SET suppressed = FALSE, updated_at = unixepoch() WHERE example_id = ? AND origin_id = ?;
-- name: DeltaFormResolveExampleByDeltaID :one
SELECT example_id FROM delta_form_delta WHERE id = ? LIMIT 1;
-- name: DeltaFormResolveExampleByOrderRefID :one
SELECT example_id FROM delta_form_order WHERE ref_id = ? LIMIT 1;

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

-- name: GetHeadersByExampleIDs :many
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
  example_id IN (sqlc.slice('example_ids'))
ORDER BY example_id;

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

-- name: GetQueriesByExampleIDs :many
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
  example_id IN (sqlc.slice('example_ids'))
ORDER BY example_id;

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

/*
 *
 * HTTP STREAMING OPTIMIZATION QUERIES
 * High-performance queries for Phase 2a HTTP streaming implementation
 *
 */

-- HTTP Snapshot Queries for Streaming
-- name: GetHTTPSnapshotPage :many
-- High-performance paginated snapshot query for initial data load
-- Uses optimized streaming indexes for fast workspace-scoped access
SELECT 
  h.id,
  h.workspace_id,
  h.folder_id,
  h.name,
  h.url,
  h.method,
  h.description,
  h.parent_http_id,
  h.is_delta,
  h.delta_name,
  h.delta_url,
  h.delta_method,
  h.delta_description,
  h.created_at,
  h.updated_at
FROM http h
WHERE h.workspace_id = ? 
  AND h.is_delta = FALSE
  AND h.updated_at <= ?
ORDER BY h.updated_at DESC, h.id
LIMIT ?;

-- name: GetHTTPSnapshotCount :one
-- Count query for pagination progress tracking
SELECT COUNT(*) as total_count
FROM http h
WHERE h.workspace_id = ? 
  AND h.is_delta = FALSE
  AND h.updated_at <= ?;

-- HTTP Incremental Streaming Queries
-- name: GetHTTPIncrementalUpdates :many
-- Real-time streaming query for changes since last update
-- Optimized with streaming indexes for minimal latency
SELECT 
  h.id,
  h.workspace_id,
  h.folder_id,
  h.name,
  h.url,
  h.method,
  h.description,
  h.parent_http_id,
  h.is_delta,
  h.delta_name,
  h.delta_url,
  h.delta_method,
  h.delta_description,
  h.created_at,
  h.updated_at
FROM http h
WHERE h.workspace_id = ? 
  AND h.updated_at > ?
  AND h.updated_at <= ?
ORDER BY h.updated_at ASC, h.id;

-- name: GetHTTPDeltasSince :many
-- Delta-specific streaming query for conflict resolution
-- Uses delta resolution index for optimal performance
SELECT 
  h.id,
  h.workspace_id,
  h.folder_id,
  h.name,
  h.url,
  h.method,
  h.description,
  h.parent_http_id,
  h.is_delta,
  h.delta_name,
  h.delta_url,
  h.delta_method,
  h.delta_description,
  h.created_at,
  h.updated_at
FROM http h
WHERE h.parent_http_id IN (sqlc.slice('parent_ids'))
  AND h.is_delta = TRUE
  AND h.updated_at > ?
  AND h.updated_at <= ?
ORDER BY h.parent_http_id, h.updated_at ASC;

-- HTTP Delta Resolution Queries
-- name: ResolveHTTPWithDeltas :one
-- CTE-optimized query to resolve HTTP record with all applicable deltas
-- Single query for complete delta resolution with minimal joins
WITH RECURSIVE delta_chain AS (
  -- Base case: Start with the parent HTTP record
  SELECT 
    h.id,
    h.workspace_id,
    h.folder_id,
    h.name,
    h.url,
    h.method,
    h.description,
    h.parent_http_id,
    h.is_delta,
    h.delta_name,
    h.delta_url,
    h.delta_method,
    h.delta_description,
    h.created_at,
    h.updated_at,
    0 as delta_level
  FROM http h
  WHERE h.id = ? AND h.is_delta = FALSE
  
  UNION ALL
  
  -- Recursive case: Apply deltas in chronological order
  SELECT 
    h.id,
    h.workspace_id,
    h.folder_id,
    COALESCE(h.delta_name, dc.name, dc.name) as name,
    COALESCE(h.delta_url, dc.url, dc.url) as url,
    COALESCE(h.delta_method, dc.method, dc.method) as method,
    COALESCE(h.delta_description, dc.description, dc.description) as description,
    h.parent_http_id,
    h.is_delta,
    h.delta_name,
    h.delta_url,
    h.delta_method,
    h.delta_description,
    h.created_at,
    h.updated_at,
    dc.delta_level + 1
  FROM http h
  INNER JOIN delta_chain dc ON h.parent_http_id = dc.id
  WHERE h.is_delta = TRUE
    AND h.updated_at <= ?
)
SELECT 
  id,
  workspace_id,
  folder_id,
  name,
  url,
  method,
  description,
  parent_http_id,
  is_delta,
  delta_name,
  delta_url,
  delta_method,
  delta_description,
  created_at,
  updated_at
FROM delta_chain
ORDER BY delta_level DESC
LIMIT 1;

-- HTTP Child Record Streaming Queries
-- name: GetHTTPHeadersStreaming :many
-- Optimized headers query for streaming with enabled filter
SELECT 
  hh.id,
  hh.http_id,
  hh.header_key,
  hh.header_value,
  hh.description,
  hh.enabled,
  hh.parent_header_id,
  hh.is_delta,
  hh.delta_header_key,
  hh.delta_header_value,
  hh.delta_description,
  hh.delta_enabled,
  hh.created_at,
  hh.updated_at
FROM http_header hh
WHERE hh.http_id IN (sqlc.slice('http_ids'))
  AND hh.enabled = TRUE
  AND hh.updated_at <= ?
ORDER BY hh.http_id, hh.updated_at DESC;

-- name: GetHTTPSearchParamsStreaming :many
-- Optimized search parameters query for streaming
SELECT 
  hsp.id,
  hsp.http_id,
  hsp.key,
  hsp.value,
  hsp.description,
  hsp.enabled,
  hsp.parent_http_search_param_id,
  hsp.is_delta,
  hsp.delta_key,
  hsp.delta_value,
  hsp.delta_description,
  hsp.delta_enabled,
  hsp.created_at,
  hsp.updated_at
FROM http_search_param hsp
WHERE hsp.http_id IN (sqlc.slice('http_ids'))
  AND hsp.enabled = TRUE
  AND hsp.updated_at <= ?
ORDER BY hsp.http_id, hsp.updated_at DESC;

-- name: GetHTTPBodyFormStreaming :many
-- Optimized form body query for streaming
SELECT 
  hbf.id,
  hbf.http_id,
  hbf.key,
  hbf.value,
  hbf.description,
  hbf.enabled,
  hbf.parent_http_body_form_id,
  hbf.is_delta,
  hbf.delta_key,
  hbf.delta_value,
  hbf.delta_description,
  hbf.delta_enabled,
  hbf.created_at,
  hbf.updated_at
FROM http_body_form hbf
WHERE hbf.http_id IN (sqlc.slice('http_ids'))
  AND hbf.enabled = TRUE
  AND hbf.updated_at <= ?
ORDER BY hbf.http_id, hbf.updated_at DESC;

-- HTTP Batch Operations for Streaming
-- name: GetHTTPBatchForStreaming :many
-- Batch query for processing multiple HTTP records efficiently
-- Optimized for high-throughput streaming operations
SELECT 
  h.id,
  h.workspace_id,
  h.folder_id,
  h.name,
  h.url,
  h.method,
  h.description,
  h.parent_http_id,
  h.is_delta,
  h.delta_name,
  h.delta_url,
  h.delta_method,
  h.delta_description,
  h.created_at,
  h.updated_at
FROM http h
WHERE h.id IN (sqlc.slice('http_ids'))
  AND h.updated_at <= ?
ORDER BY h.updated_at DESC;

-- HTTP Performance Monitoring Queries
-- name: GetHTTPStreamingMetrics :one
-- Performance metrics query for monitoring streaming operations
SELECT 
  COUNT(*) as total_http_records,
  COUNT(CASE WHEN is_delta = FALSE THEN 1 END) as base_records,
  COUNT(CASE WHEN is_delta = TRUE THEN 1 END) as delta_records,
  MAX(updated_at) as latest_update,
  MIN(updated_at) as earliest_update,
  COUNT(CASE WHEN updated_at > ? THEN 1 END) as recent_changes
FROM http 
WHERE workspace_id = ?;

-- name: GetHTTPWorkspaceActivity :many
-- Activity monitoring query for workspace streaming health
SELECT 
  DATE(updated_at, 'unixepoch') as activity_date,
  COUNT(*) as changes_count,
  COUNT(CASE WHEN is_delta = TRUE THEN 1 END) as delta_count,
  COUNT(CASE WHEN is_delta = FALSE THEN 1 END) as base_count
FROM http
WHERE workspace_id = ?
  AND updated_at >= ?
GROUP BY DATE(updated_at, 'unixepoch')
ORDER BY activity_date DESC
LIMIT 30;

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

-- name: GetBodyFormsByExampleIDs :many
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
    example_id IN (sqlc.slice('example_ids'))
ORDER BY example_id;

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

-- name: GetBodyUrlEncodedLinks :one
SELECT
  example_id,
  prev,
  next
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
  value,
  prev,
  next
FROM
  example_body_urlencoded
WHERE
  example_id = ?;

-- name: GetBodyUrlEncodedsByExampleIDs :many
SELECT
  id,
  example_id,
  delta_parent_id,
  body_key,
  enable,
  description,
  value,
  prev,
  next
FROM
  example_body_urlencoded
WHERE
  example_id IN (sqlc.slice('example_ids'))
ORDER BY example_id;

-- name: GetBodyUrlEncodedsByExampleIDOrdered :many
-- Uses WITH RECURSIVE CTE to traverse linked list from head to tail for example-scoped ordering
-- URL-encoded body fields are scoped to specific examples via example_id column
WITH RECURSIVE ordered_urlenc AS (
  -- Base case: Find the head (prev IS NULL) for this example
  SELECT
    b.id,
    b.example_id,
    b.delta_parent_id,
    b.body_key,
    b.enable,
    b.description,
    b.value,
    b.prev,
    b.next,
    0 as position
  FROM
    example_body_urlencoded b
  WHERE
    b.example_id = ? AND
    b.prev IS NULL

  UNION ALL

  -- Recursive case: Follow the next pointers
  SELECT
    b.id,
    b.example_id,
    b.delta_parent_id,
    b.body_key,
    b.enable,
    b.description,
    b.value,
    b.prev,
    b.next,
    ou.position + 1
  FROM
    example_body_urlencoded b
  INNER JOIN ordered_urlenc ou ON b.prev = ou.id
  WHERE
    b.example_id = ?
)
SELECT
  ou.id,
  ou.example_id,
  ou.delta_parent_id,
  ou.body_key,
  ou.enable,
  ou.description,
  ou.value,
  ou.prev,
  ou.next,
  ou.position
FROM
  ordered_urlenc ou
ORDER BY
  ou.position;

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

-- name: UpdateBodyUrlEncodedOrder :exec
-- Update the prev/next pointers for a single URL-encoded body with example validation
-- Used for moving items within the example's linked list
UPDATE example_body_urlencoded
SET
  prev = ?,
  next = ?
WHERE
  id = ? AND
  example_id = ?;

-- name: UpdateBodyUrlEncodedPrev :exec
-- Update only the prev pointer for a URL-encoded body with example validation (used in deletion or moves)
UPDATE example_body_urlencoded
SET
  prev = ?
WHERE
  id = ? AND
  example_id = ?;

-- name: UpdateBodyUrlEncodedNext :exec
-- Update only the next pointer for a URL-encoded body with example validation (used in deletion or moves)
UPDATE example_body_urlencoded
SET
  next = ?
WHERE
  id = ? AND
  example_id = ?;

-- name: GetBodyUrlEncodedTail :one
-- Get the last URL-encoded pair in the list (tail) for an example
-- Used when appending new pairs to the end of the list
SELECT
  id,
  example_id,
  delta_parent_id,
  body_key,
  enable,
  description,
  value,
  prev,
  next
FROM
  example_body_urlencoded
WHERE
  example_id = ? AND
  next IS NULL
LIMIT
  1;

-- name: GetBodyUrlEncodedTailExcludingID :one
SELECT
  id,
  example_id,
  delta_parent_id,
  body_key,
  enable,
  description,
  value,
  prev,
  next
FROM
  example_body_urlencoded
WHERE
  example_id = ? AND
  id <> ? AND
  next IS NULL
LIMIT
  1;

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

-- name: GetBodyRawsByExampleIDs :many
SELECT
  id,
  example_id,
  visualize_mode,
  compress_type,
  data
FROM
  example_body_raw
WHERE
  example_id IN (sqlc.slice('example_ids'))
ORDER BY example_id;

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
  display_order
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
  display_order
FROM
  environment
WHERE
  workspace_id = ?
ORDER BY
  display_order;

-- name: CreateEnvironment :exec
INSERT INTO
  environment (id, workspace_id, type, name, description, display_order)
VALUES
  (?, ?, ?, ?, ?, ?);

-- name: UpdateEnvironment :exec
UPDATE environment
SET
    type = ?,
    name = ?,
    description = ?,
    display_order = ?
WHERE
    id = ?;

-- name: DeleteEnvironment :exec
DELETE FROM environment
WHERE
  id = ?;

-- name: GetEnvironmentsByWorkspaceIDOrdered :many
SELECT
  id,
  workspace_id,
  type,
  name,
  description,
  display_order
FROM
  environment
WHERE
  workspace_id = ?
ORDER BY
  display_order;

-- name: GetEnvironmentWorkspaceID :one
SELECT
  workspace_id
FROM
  environment
WHERE
  id = ?
LIMIT
  1;

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
  display_order
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
  display_order
FROM
  variable
WHERE
  env_id = ?
ORDER BY
  display_order;

-- name: GetVariablesByEnvironmentIDOrdered :many
SELECT
  id,
  env_id,
  var_key,
  value,
  enabled,
  description,
  display_order
FROM
  variable
WHERE
  env_id = ?
ORDER BY
  display_order;

-- name: CreateVariable :exec
INSERT INTO
  variable (id, env_id, var_key, value, enabled, description, display_order)
VALUES
  (?, ?, ?, ?, ?, ?, ?);

-- name: CreateVariableBulk :exec
INSERT INTO
  variable (id, env_id, var_key, value, enabled, description, display_order)
VALUES
  (?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?);

-- name: UpdateVariable :exec
UPDATE variable
SET
  var_key = ?,
  value = ?,
  enabled = ?,
  description = ?,
  display_order = ?
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

-- name: GetExampleRespByExampleIDsLatest :many
SELECT
  id,
  example_id,
  status,
  body,
  body_compress_type,
  duration
FROM (
  SELECT
    er.*, 
    ROW_NUMBER() OVER (PARTITION BY er.example_id ORDER BY er.id DESC) AS rn
  FROM
    example_resp er
  WHERE
    er.example_id IN (sqlc.slice('example_ids'))
)
WHERE
  rn = 1;

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

-- name: GetExampleRespHeadersByRespIDs :many
SELECT
  id,
  example_resp_id,
  header_key,
  value
FROM
  example_resp_header
WHERE
  example_resp_id IN (sqlc.slice('resp_ids'))
ORDER BY example_resp_id;

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

-- name: GetAssertsByExampleIDs :many
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
  example_id IN (sqlc.slice('example_ids'))
ORDER BY example_id;

-- name: GetAssertsByDeltaParent :many
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
  delta_parent_id = ?;

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

-- name: GetAssertsByExampleIDOrdered :many
-- Uses WITH RECURSIVE CTE to traverse linked list from head to tail for example-scoped ordering
-- Assertions are scoped to specific examples via example_id column
WITH RECURSIVE ordered_asserts AS (
  -- Base case: Find the head (prev IS NULL) for this example
  SELECT
    a.id,
    a.example_id,
    a.delta_parent_id,
    a.expression,
    a.enable,
    a.prev,
    a.next,
    0 as position
  FROM
    assertion a
  WHERE
    a.example_id = ? AND
    a.prev IS NULL
  
  UNION ALL
  
  -- Recursive case: Follow the next pointers
  SELECT
    a.id,
    a.example_id,
    a.delta_parent_id,
    a.expression,
    a.enable,
    a.prev,
    a.next,
    oa.position + 1
  FROM
    assertion a
  INNER JOIN ordered_asserts oa ON a.prev = oa.id
  WHERE
    a.example_id = ?
)
SELECT
  oa.id,
  oa.example_id,
  oa.delta_parent_id,
  oa.expression,
  oa.enable,
  oa.prev,
  oa.next,
  oa.position
FROM
  ordered_asserts oa
ORDER BY
  oa.position;

-- name: UpdateAssertOrder :exec
-- Update the prev/next pointers for a single assertion with example validation
-- Used for moving assertions within the example's linked list
UPDATE assertion
SET
  prev = ?,
  next = ?
WHERE
  id = ? AND
  example_id = ?;

-- name: UpdateAssertPrev :exec
-- Update only the prev pointer for an assertion with example validation (used in deletion)
UPDATE assertion
SET
  prev = ?
WHERE
  id = ? AND
  example_id = ?;

-- name: UpdateAssertNext :exec
-- Update only the next pointer for an assertion with example validation (used in deletion)
UPDATE assertion
SET
  next = ?
WHERE
  id = ? AND
  example_id = ?;

-- name: GetAssertTail :one
-- Get the last assertion in the list (tail) for an example
-- Used when appending new assertions to the end of the list
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
  example_id = ? AND
  next IS NULL
LIMIT
  1;

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
  name,
  duration
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
  name,
  duration
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
  name,
  duration
FROM
  flow
WHERE
  version_parent_id is ?;

-- name: CreateFlow :exec
INSERT INTO
  flow (id, workspace_id, version_parent_id, name, duration)
VALUES
  (?, ?, ?, ?, ?);

-- name: UpdateFlow :exec
UPDATE flow
SET
  name = ?,
  duration = ?
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
  delta_endpoint_id,
  has_request_config
FROM
  flow_node_request
WHERE
  flow_node_id = ?
LIMIT 1;

-- name: CreateFlowNodeRequest :exec
INSERT INTO
  flow_node_request (
    flow_node_id,
    endpoint_id,
    example_id,
    delta_example_id,
    delta_endpoint_id,
    has_request_config
  )
VALUES
  (?, ?, ?, ?, ?, ?);

-- name: UpdateFlowNodeRequest :exec
UPDATE flow_node_request
SET
  endpoint_id = ?,
  example_id = ?,
  delta_example_id = ?,
  delta_endpoint_id = ?,
  has_request_config = ?
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
  input_data = excluded.input_data,
  input_data_compress_type = excluded.input_data_compress_type,
  output_data = excluded.output_data,
  output_data_compress_type = excluded.output_data_compress_type,
  response_id = excluded.response_id,
  completed_at = excluded.completed_at
RETURNING *;

-- name: GetNodeExecutionsByNodeID :many
SELECT *
FROM node_execution
WHERE node_id = ? AND completed_at IS NOT NULL
ORDER BY completed_at DESC, id DESC;

-- name: GetLatestNodeExecutionByNodeID :one
SELECT *
FROM node_execution
WHERE node_id = ? AND completed_at IS NOT NULL
ORDER BY completed_at DESC, id DESC
LIMIT 1;

-- name: DeleteNodeExecutionsByNodeID :exec
DELETE FROM node_execution WHERE node_id = ?;

-- name: DeleteNodeExecutionsByNodeIDs :exec
DELETE FROM node_execution WHERE node_id IN (sqlc.slice('node_ids'));

-- 






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

/*
 *
 * HTTP SYSTEM QUERIES
 * Single-table approach with delta fields for Phase 1 HTTP implementation
 *
 */

--
-- HTTP Core Queries
--

-- name: GetHTTP :one
SELECT
  id,
  workspace_id,
  folder_id,
  name,
  url,
  method,
  description,
  parent_http_id,
  is_delta,
  delta_name,
  delta_url,
  delta_method,
  delta_description,
  created_at,
  updated_at
FROM http
WHERE id = ?
LIMIT 1;

-- name: GetHTTPsByWorkspaceID :many
SELECT
  id,
  workspace_id,
  folder_id,
  name,
  url,
  method,
  description,
  parent_http_id,
  is_delta,
  delta_name,
  delta_url,
  delta_method,
  delta_description,
  created_at,
  updated_at
FROM http
WHERE workspace_id = ? AND is_delta = FALSE
ORDER BY updated_at DESC;

-- name: GetHTTPsByFolderID :many
SELECT
  id,
  workspace_id,
  folder_id,
  name,
  url,
  method,
  description,
  parent_http_id,
  is_delta,
  delta_name,
  delta_url,
  delta_method,
  delta_description,
  created_at,
  updated_at
FROM http
WHERE folder_id = ? AND is_delta = FALSE
ORDER BY updated_at DESC;

-- name: GetHTTPsByIDs :many
SELECT
  id,
  workspace_id,
  folder_id,
  name,
  url,
  method,
  description,
  parent_http_id,
  is_delta,
  delta_name,
  delta_url,
  delta_method,
  delta_description,
  created_at,
  updated_at
FROM http
WHERE id IN (sqlc.slice('ids'));

-- name: GetHTTPWorkspaceID :one
SELECT workspace_id
FROM http
WHERE id = ?
LIMIT 1;

-- name: CreateHTTP :exec
INSERT INTO http (
  id, workspace_id, folder_id, name, url, method, description,
  parent_http_id, is_delta, delta_name, delta_url, delta_method, delta_description,
  created_at, updated_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateHTTP :exec
UPDATE http
SET
  folder_id = ?,
  name = ?,
  url = ?,
  method = ?,
  description = ?,
  updated_at = unixepoch()
WHERE id = ?;

-- name: UpdateHTTPDelta :exec
UPDATE http
SET
  delta_name = ?,
  delta_url = ?,
  delta_method = ?,
  delta_description = ?,
  updated_at = unixepoch()
WHERE id = ?;

-- name: DeleteHTTP :exec
DELETE FROM http
WHERE id = ?;

-- name: GetHTTPDeltasByParentID :many
SELECT
  id,
  workspace_id,
  folder_id,
  name,
  url,
  method,
  description,
  parent_http_id,
  is_delta,
  delta_name,
  delta_url,
  delta_method,
  delta_description,
  created_at,
  updated_at
FROM http
WHERE parent_http_id = ? AND is_delta = TRUE
ORDER BY created_at DESC;

--
-- HTTP Search Parameter Queries
--

-- name: GetHTTPSearchParams :many
SELECT
  id,
  http_id,
  key,
  value,
  description,
  enabled,
  parent_http_search_param_id,
  is_delta,
  delta_key,
  delta_value,
  delta_description,
  delta_enabled,
  "order",
  created_at,
  updated_at
FROM http_search_param
WHERE http_id = ? AND is_delta = FALSE
ORDER BY "order";

-- name: GetHTTPSearchParamsByIDs :many
SELECT
  id,
  http_id,
  key,
  value,
  description,
  enabled,
  parent_http_search_param_id,
  is_delta,
  delta_key,
  delta_value,
  delta_description,
  delta_enabled,
  "order",
  created_at,
  updated_at
FROM http_search_param
WHERE id IN (sqlc.slice('ids'));

-- name: CreateHTTPSearchParam :exec
INSERT INTO http_search_param (
  id, http_id, key, value, description, enabled, "order",
  parent_http_search_param_id, is_delta, delta_key, delta_value,
  delta_description, delta_enabled, created_at, updated_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateHTTPSearchParam :exec
UPDATE http_search_param
SET
  key = ?,
  value = ?,
  description = ?,
  enabled = ?,
  updated_at = unixepoch()
WHERE id = ?;

-- name: UpdateHTTPSearchParamDelta :exec
UPDATE http_search_param
SET
  delta_key = ?,
  delta_value = ?,
  delta_description = ?,
  delta_enabled = ?,
  updated_at = unixepoch()
WHERE id = ?;

-- name: UpdateHTTPSearchParamOrder :exec
UPDATE http_search_param
SET "order" = ?
WHERE id = ? AND http_id = ?;

-- name: DeleteHTTPSearchParam :exec
DELETE FROM http_search_param
WHERE id = ?;

--
-- HTTP Header Queries
--

-- name: GetHTTPHeaders :many
SELECT
  id,
  http_id,
  header_key,
  header_value,
  description,
  enabled,
  parent_header_id,
  is_delta,
  delta_header_key,
  delta_header_value,
  delta_description,
  delta_enabled,
  prev,
  next,
  created_at,
  updated_at
FROM http_header
WHERE http_id = ? AND is_delta = FALSE
ORDER BY created_at ASC;

-- name: GetHTTPHeadersByIDs :many
SELECT
  id,
  http_id,
  header_key,
  header_value,
  description,
  enabled,
  parent_header_id,
  is_delta,
  delta_header_key,
  delta_header_value,
  delta_description,
  delta_enabled,
  prev,
  next,
  created_at,
  updated_at
FROM http_header
WHERE id IN (sqlc.slice('ids'));

-- name: CreateHTTPHeader :exec
INSERT INTO http_header (
  id, http_id, header_key, header_value, description, enabled,
  parent_header_id, is_delta, delta_header_key, delta_header_value,
  delta_description, delta_enabled, prev, next, created_at, updated_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateHTTPHeader :exec
UPDATE http_header
SET
  header_key = ?,
  header_value = ?,
  description = ?,
  enabled = ?,
  updated_at = unixepoch()
WHERE id = ?;

-- name: UpdateHTTPHeaderDelta :exec
UPDATE http_header
SET
  delta_header_key = ?,
  delta_header_value = ?,
  delta_description = ?,
  delta_enabled = ?,
  updated_at = unixepoch()
WHERE id = ?;

-- name: UpdateHTTPHeaderOrder :exec
UPDATE http_header
SET prev = ?, next = ?
WHERE id = ? AND http_id = ?;

-- name: DeleteHTTPHeader :exec
DELETE FROM http_header
WHERE id = ?;

--
-- HTTP Body Form Queries
--

-- name: GetHTTPBodyForms :many
SELECT
  id,
  http_id,
  key,
  value,
  description,
  enabled,
  parent_http_body_form_id,
  is_delta,
  delta_key,
  delta_value,
  delta_description,
  delta_enabled,
  "order",
  created_at,
  updated_at
FROM http_body_form
WHERE http_id = ? AND is_delta = FALSE
ORDER BY "order";

-- name: GetHTTPBodyFormsByIDs :many
SELECT
  id,
  http_id,
  key,
  value,
  description,
  enabled,
  parent_http_body_form_id,
  is_delta,
  delta_key,
  delta_value,
  delta_description,
  delta_enabled,
  "order",
  created_at,
  updated_at
FROM http_body_form
WHERE id IN (sqlc.slice('ids'));

-- name: CreateHTTPBodyForm :exec
INSERT INTO http_body_form (
  id, http_id, key, value, description, enabled, "order",
  parent_http_body_form_id, is_delta, delta_key, delta_value,
  delta_description, delta_enabled, created_at, updated_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateHTTPBodyForm :exec
UPDATE http_body_form
SET
  key = ?,
  value = ?,
  description = ?,
  enabled = ?,
  updated_at = unixepoch()
WHERE id = ?;

-- name: UpdateHTTPBodyFormDelta :exec
UPDATE http_body_form
SET
  delta_key = ?,
  delta_value = ?,
  delta_description = ?,
  delta_enabled = ?,
  updated_at = unixepoch()
WHERE id = ?;

-- name: UpdateHTTPBodyFormOrder :exec
UPDATE http_body_form
SET "order" = ?
WHERE id = ? AND http_id = ?;

-- name: DeleteHTTPBodyForm :exec
DELETE FROM http_body_form WHERE id = ?;

--
-- HTTP Body URL-Encoded Queries (TypeSpec-compliant)
--

-- name: GetHTTPBodyUrlEncoded :one
SELECT
  id,
  http_id,
  key,
  value,
  enabled,
  description,
  "order",
  parent_http_body_urlencoded_id,
  is_delta,
  delta_key,
  delta_value,
  delta_enabled,
  delta_description,
  delta_order,
  created_at,
  updated_at
FROM http_body_urlencoded
WHERE id = ?
LIMIT 1;

-- name: GetHTTPBodyUrlEncodedByHttpID :many
SELECT
  id,
  http_id,
  key,
  value,
  enabled,
  description,
  "order",
  parent_http_body_urlencoded_id,
  is_delta,
  delta_key,
  delta_value,
  delta_enabled,
  delta_description,
  delta_order,
  created_at,
  updated_at
FROM http_body_urlencoded
WHERE http_id = ? AND is_delta = FALSE
ORDER BY "order";

-- name: GetHTTPBodyUrlEncodedsByIDs :many
SELECT
  id,
  http_id,
  key,
  value,
  enabled,
  description,
  "order",
  parent_http_body_urlencoded_id,
  is_delta,
  delta_key,
  delta_value,
  delta_enabled,
  delta_description,
  delta_order,
  created_at,
  updated_at
FROM http_body_urlencoded
WHERE id IN (sqlc.slice('ids'));

-- name: CreateHTTPBodyUrlEncoded :exec
INSERT INTO http_body_urlencoded (
  id, http_id, key, value, enabled, description, "order",
  parent_http_body_urlencoded_id, is_delta, delta_key, delta_value,
  delta_enabled, delta_description, delta_order, created_at, updated_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: CreateHTTPBodyUrlEncodedBulk :exec
INSERT INTO http_body_urlencoded (
  id, http_id, key, value, enabled, description, "order",
  parent_http_body_urlencoded_id, is_delta, delta_key, delta_value,
  delta_enabled, delta_description, delta_order, created_at, updated_at
)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateHTTPBodyUrlEncoded :exec
UPDATE http_body_urlencoded
SET
  key = ?,
  value = ?,
  enabled = ?,
  description = ?,
  "order" = ?,
  delta_key = ?,
  delta_value = ?,
  delta_enabled = ?,
  delta_description = ?,
  delta_order = ?,
  updated_at = ?
WHERE id = ?;

-- name: UpdateHTTPBodyUrlEncodedDelta :exec
UPDATE http_body_urlencoded
SET
  delta_key = ?,
  delta_value = ?,
  delta_enabled = ?,
  delta_description = ?,
  delta_order = ?,
  updated_at = unixepoch()
WHERE id = ?;

-- name: DeleteHTTPBodyUrlEncoded :exec
DELETE FROM http_body_urlencoded WHERE id = ?;

--
-- HTTP Assert Queries (TypeSpec-compliant)
--

-- name: GetHTTPAssert :one
SELECT
  id,
  http_id,
  key,
  value,
  enabled,
  description,
  "order",
  parent_http_assert_id,
  is_delta,
  delta_key,
  delta_value,
  delta_enabled,
  delta_description,
  delta_order,
  created_at,
  updated_at
FROM http_assert
WHERE id = ?
LIMIT 1;

-- name: GetHTTPAssertsByHttpID :many
SELECT
  id,
  http_id,
  key,
  value,
  enabled,
  description,
  "order",
  parent_http_assert_id,
  is_delta,
  delta_key,
  delta_value,
  delta_enabled,
  delta_description,
  delta_order,
  created_at,
  updated_at
FROM http_assert
WHERE http_id = ? AND is_delta = FALSE
ORDER BY "order";

-- name: GetHTTPAssertsByIDs :many
SELECT
  id,
  http_id,
  key,
  value,
  enabled,
  description,
  "order",
  parent_http_assert_id,
  is_delta,
  delta_key,
  delta_value,
  delta_enabled,
  delta_description,
  delta_order,
  created_at,
  updated_at
FROM http_assert
WHERE id IN (sqlc.slice('ids'));

-- name: CreateHTTPAssert :exec
INSERT INTO http_assert (
  id, http_id, key, value, enabled, description, "order",
  parent_http_assert_id, is_delta, delta_key, delta_value,
  delta_enabled, delta_description, delta_order, created_at, updated_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: CreateHTTPAssertBulk :exec
INSERT INTO http_assert (
  id, http_id, key, value, enabled, description, "order",
  parent_http_assert_id, is_delta, delta_key, delta_value,
  delta_enabled, delta_description, delta_order, created_at, updated_at
)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateHTTPAssert :exec
UPDATE http_assert
SET
  key = ?,
  value = ?,
  enabled = ?,
  description = ?,
  "order" = ?,
  delta_key = ?,
  delta_value = ?,
  delta_enabled = ?,
  delta_description = ?,
  delta_order = ?,
  updated_at = ?
WHERE id = ?;

-- name: UpdateHTTPAssertDelta :exec
UPDATE http_assert
SET
  delta_key = ?,
  delta_value = ?,
  delta_enabled = ?,
  delta_description = ?,
  delta_order = ?,
  updated_at = unixepoch()
WHERE id = ?;

-- name: DeleteHTTPAssert :exec
DELETE FROM http_assert WHERE id = ?;

--
-- HTTP Response Queries (TypeSpec-compliant)
--
-- HTTP Response Queries (TypeSpec-compliant)
--

-- name: GetHTTPResponse :one
SELECT
  id,
  http_id,
  status,
  body,
  time,
  duration,
  size,
  created_at
FROM http_response
WHERE id = ?
LIMIT 1;

-- name: GetHTTPResponsesByHttpID :many
SELECT
  id,
  http_id,
  status,
  body,
  time,
  duration,
  size,
  created_at
FROM http_response
WHERE http_id = ?
ORDER BY time DESC;

-- name: GetHTTPResponsesByIDs :many
SELECT
  id,
  http_id,
  status,
  body,
  time,
  duration,
  size,
  created_at
FROM http_response
WHERE id IN (sqlc.slice('ids'))
ORDER BY time DESC;

-- name: CreateHTTPResponse :exec
INSERT INTO http_response (
  id, http_id, status, body, time, duration, size, created_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: CreateHTTPResponseBulk :exec
INSERT INTO http_response (
  id, http_id, status, body, time, duration, size, created_at
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

-- name: UpdateHTTPResponse :exec
UPDATE http_response
SET
  status = ?,
  body = ?,
  time = ?,
  duration = ?,
  size = ?
WHERE id = ?;

-- name: DeleteHTTPResponse :exec
DELETE FROM http_response WHERE id = ?;

--
-- HTTP Response Header Queries (TypeSpec-compliant)
--

-- name: GetHTTPResponseHeader :one
SELECT
  id,
  http_id,
  key,
  value,
  created_at
FROM http_response_header
WHERE id = ?
LIMIT 1;

-- name: GetHTTPResponseHeadersByHttpID :many
SELECT
  id,
  http_id,
  key,
  value,
  created_at
FROM http_response_header
WHERE http_id = ?
ORDER BY key;

-- name: GetHTTPResponseHeadersByIDs :many
SELECT
  id,
  http_id,
  key,
  value,
  created_at
FROM http_response_header
WHERE id IN (sqlc.slice('ids'))
ORDER BY http_id, key;

-- name: CreateHTTPResponseHeader :exec
INSERT INTO http_response_header (
  id, http_id, key, value, created_at
)
VALUES (?, ?, ?, ?, ?);

-- name: CreateHTTPResponseHeaderBulk :exec
INSERT INTO http_response_header (
  id, http_id, key, value, created_at
)
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

-- name: UpdateHTTPResponseHeader :exec
UPDATE http_response_header
SET
  key = ?,
  value = ?
WHERE id = ?;

-- name: DeleteHTTPResponseHeader :exec
DELETE FROM http_response_header WHERE id = ?;

--
-- HTTP Response Assert Queries (TypeSpec-compliant)
--

-- name: GetHTTPResponseAssert :one
SELECT
  id,
  http_id,
  value,
  success,
  created_at
FROM http_response_assert
WHERE id = ?
LIMIT 1;

-- name: GetHTTPResponseAssertsByHttpID :many
SELECT
  id,
  http_id,
  value,
  success,
  created_at
FROM http_response_assert
WHERE http_id = ?
ORDER BY created_at DESC;

-- name: GetHTTPResponseAssertsByIDs :many
SELECT
  id,
  http_id,
  value,
  success,
  created_at
FROM http_response_assert
WHERE id IN (sqlc.slice('ids'))
ORDER BY created_at DESC;

-- name: CreateHTTPResponseAssert :exec
INSERT INTO http_response_assert (
  id, http_id, value, success, created_at
)
VALUES (?, ?, ?, ?, ?);

-- name: CreateHTTPResponseAssertBulk :exec
INSERT INTO http_response_assert (
  id, http_id, value, success, created_at
)
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

-- name: UpdateHTTPResponseAssert :exec
UPDATE http_response_assert
SET
  value = ?,
  success = ?
WHERE id = ?;

-- name: DeleteHTTPResponseAssert :exec
DELETE FROM http_response_assert WHERE id = ?;
