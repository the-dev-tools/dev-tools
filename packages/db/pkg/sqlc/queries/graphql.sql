--
-- GraphQL Core Queries
--

-- name: GetGraphQL :one
SELECT
  id, workspace_id, folder_id, name, url, query, variables,
  description, last_run_at, created_at, updated_at
FROM graphql
WHERE id = ? LIMIT 1;

-- name: GetGraphQLsByWorkspaceID :many
SELECT
  id, workspace_id, folder_id, name, url, query, variables,
  description, last_run_at, created_at, updated_at
FROM graphql
WHERE workspace_id = ?
ORDER BY updated_at DESC;

-- name: GetGraphQLWorkspaceID :one
SELECT workspace_id
FROM graphql
WHERE id = ?
LIMIT 1;

-- name: CreateGraphQL :exec
INSERT INTO graphql (
  id, workspace_id, folder_id, name, url, query, variables,
  description, last_run_at, created_at, updated_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateGraphQL :exec
UPDATE graphql
SET
  name = ?,
  url = ?,
  query = ?,
  variables = ?,
  description = ?,
  last_run_at = COALESCE(?, last_run_at),
  updated_at = unixepoch()
WHERE id = ?;

-- name: DeleteGraphQL :exec
DELETE FROM graphql
WHERE id = ?;

--
-- GraphQL Header Queries
--

-- name: GetGraphQLHeaders :many
SELECT
  id, graphql_id, header_key, header_value, description,
  enabled, display_order, created_at, updated_at
FROM graphql_header
WHERE graphql_id = ?
ORDER BY display_order;

-- name: GetGraphQLHeadersByIDs :many
SELECT
  id, graphql_id, header_key, header_value, description,
  enabled, display_order, created_at, updated_at
FROM graphql_header
WHERE id IN (sqlc.slice('ids'));

-- name: CreateGraphQLHeader :exec
INSERT INTO graphql_header (
  id, graphql_id, header_key, header_value, description,
  enabled, display_order, created_at, updated_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateGraphQLHeader :exec
UPDATE graphql_header
SET
  header_key = ?,
  header_value = ?,
  description = ?,
  enabled = ?,
  display_order = ?,
  updated_at = unixepoch()
WHERE id = ?;

-- name: DeleteGraphQLHeader :exec
DELETE FROM graphql_header
WHERE id = ?;

--
-- GraphQL Response Queries
--

-- name: GetGraphQLResponse :one
SELECT
  id, graphql_id, status, body, time, duration, size, created_at
FROM graphql_response
WHERE id = ? LIMIT 1;

-- name: GetGraphQLResponsesByGraphQLID :many
SELECT
  id, graphql_id, status, body, time, duration, size, created_at
FROM graphql_response
WHERE graphql_id = ?
ORDER BY time DESC;

-- name: GetGraphQLResponsesByWorkspaceID :many
SELECT
  gr.id, gr.graphql_id, gr.status, gr.body, gr.time,
  gr.duration, gr.size, gr.created_at
FROM graphql_response gr
INNER JOIN graphql g ON gr.graphql_id = g.id
WHERE g.workspace_id = ?
ORDER BY gr.time DESC;

-- name: CreateGraphQLResponse :exec
INSERT INTO graphql_response (
  id, graphql_id, status, body, time, duration, size, created_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: DeleteGraphQLResponse :exec
DELETE FROM graphql_response WHERE id = ?;

--
-- GraphQL Response Header Queries
--

-- name: GetGraphQLResponseHeadersByResponseID :many
SELECT
  id, response_id, key, value, created_at
FROM graphql_response_header
WHERE response_id = ?
ORDER BY key;

-- name: GetGraphQLResponseHeadersByWorkspaceID :many
SELECT
  grh.id, grh.response_id, grh.key, grh.value, grh.created_at
FROM graphql_response_header grh
INNER JOIN graphql_response gr ON grh.response_id = gr.id
INNER JOIN graphql g ON gr.graphql_id = g.id
WHERE g.workspace_id = ?
ORDER BY gr.time DESC, grh.key;

-- name: CreateGraphQLResponseHeader :exec
INSERT INTO graphql_response_header (
  id, response_id, key, value, created_at
)
VALUES (?, ?, ?, ?, ?);

-- name: CreateGraphQLResponseHeaderBulk :exec
INSERT INTO graphql_response_header (
  id, response_id, key, value, created_at
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

-- name: DeleteGraphQLResponseHeader :exec
DELETE FROM graphql_response_header WHERE id = ?;
