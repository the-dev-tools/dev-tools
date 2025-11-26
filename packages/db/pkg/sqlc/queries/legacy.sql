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
