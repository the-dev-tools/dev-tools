--
-- GraphQL Core Queries
--

-- name: GetGraphQL :one
SELECT
  id, workspace_id, folder_id, name, url, query, variables,
  description, last_run_at, created_at, updated_at,
  parent_graphql_id, is_delta, is_snapshot,
  delta_name, delta_url, delta_query, delta_variables, delta_description
FROM graphql
WHERE id = ? LIMIT 1;

-- name: GetGraphQLsByWorkspaceID :many
SELECT
  id, workspace_id, folder_id, name, url, query, variables,
  description, last_run_at, created_at, updated_at,
  parent_graphql_id, is_delta, is_snapshot,
  delta_name, delta_url, delta_query, delta_variables, delta_description
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
  description, last_run_at, created_at, updated_at,
  parent_graphql_id, is_delta, is_snapshot,
  delta_name, delta_url, delta_query, delta_variables, delta_description
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

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

-- name: UpdateGraphQLDelta :exec
UPDATE graphql
SET
  delta_name = ?,
  delta_url = ?,
  delta_query = ?,
  delta_variables = ?,
  delta_description = ?,
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
  enabled, display_order, created_at, updated_at,
  parent_graphql_header_id, is_delta,
  delta_header_key, delta_header_value, delta_description, delta_enabled, delta_display_order
FROM graphql_header
WHERE graphql_id = ?
ORDER BY display_order;

-- name: GetGraphQLHeadersByIDs :many
SELECT
  id, graphql_id, header_key, header_value, description,
  enabled, display_order, created_at, updated_at,
  parent_graphql_header_id, is_delta,
  delta_header_key, delta_header_value, delta_description, delta_enabled, delta_display_order
FROM graphql_header
WHERE id IN (sqlc.slice('ids'));

-- name: CreateGraphQLHeader :exec
INSERT INTO graphql_header (
  id, graphql_id, header_key, header_value, description,
  enabled, display_order, created_at, updated_at,
  parent_graphql_header_id, is_delta,
  delta_header_key, delta_header_value, delta_description, delta_enabled, delta_display_order
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

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

--
-- GraphQL Delta Queries
--

-- name: GetGraphQLDeltasByWorkspaceID :many
SELECT
  id, workspace_id, folder_id, name, url, query, variables,
  description, last_run_at, created_at, updated_at,
  parent_graphql_id, is_delta, is_snapshot,
  delta_name, delta_url, delta_query, delta_variables, delta_description
FROM graphql
WHERE workspace_id = ? AND is_delta = TRUE
ORDER BY updated_at DESC;

-- name: GetGraphQLDeltasByParentID :many
SELECT
  id, workspace_id, folder_id, name, url, query, variables,
  description, last_run_at, created_at, updated_at,
  parent_graphql_id, is_delta, is_snapshot,
  delta_name, delta_url, delta_query, delta_variables, delta_description
FROM graphql
WHERE parent_graphql_id = ? AND is_delta = TRUE
ORDER BY updated_at DESC;

--
-- GraphQL Header Delta Queries
--

-- name: GetGraphQLHeaderDeltasByWorkspaceID :many
SELECT
  h.id, h.graphql_id, h.header_key, h.header_value, h.description,
  h.enabled, h.display_order, h.created_at, h.updated_at,
  h.parent_graphql_header_id, h.is_delta,
  h.delta_header_key, h.delta_header_value, h.delta_description, h.delta_enabled, h.delta_display_order
FROM graphql_header h
JOIN graphql g ON h.graphql_id = g.id
WHERE g.workspace_id = ? AND h.is_delta = TRUE
ORDER BY h.updated_at DESC;

-- name: GetGraphQLHeaderDeltasByParentID :many
SELECT
  id, graphql_id, header_key, header_value, description,
  enabled, display_order, created_at, updated_at,
  parent_graphql_header_id, is_delta,
  delta_header_key, delta_header_value, delta_description, delta_enabled, delta_display_order
FROM graphql_header
WHERE parent_graphql_header_id = ? AND is_delta = TRUE
ORDER BY display_order;

--
-- GraphQL Assert Queries
--

-- name: GetGraphQLAssert :one
SELECT
  id,
  graphql_id,
  value,
  enabled,
  description,
  display_order,
  parent_graphql_assert_id,
  is_delta,
  delta_value,
  delta_enabled,
  delta_description,
  delta_display_order,
  created_at,
  updated_at
FROM graphql_assert
WHERE id = ?
LIMIT 1;

-- name: GetGraphQLAssertsByGraphQLID :many
SELECT
  id,
  graphql_id,
  value,
  enabled,
  description,
  display_order,
  parent_graphql_assert_id,
  is_delta,
  delta_value,
  delta_enabled,
  delta_description,
  delta_display_order,
  created_at,
  updated_at
FROM graphql_assert
WHERE graphql_id = ?
ORDER BY display_order;

-- name: GetGraphQLAssertsByIDs :many
SELECT
  id,
  graphql_id,
  value,
  enabled,
  description,
  display_order,
  parent_graphql_assert_id,
  is_delta,
  delta_value,
  delta_enabled,
  delta_description,
  delta_display_order,
  created_at,
  updated_at
FROM graphql_assert
WHERE id IN (sqlc.slice('ids'));

-- name: CreateGraphQLAssert :exec
INSERT INTO graphql_assert (
  id,
  graphql_id,
  value,
  enabled,
  description,
  display_order,
  parent_graphql_assert_id,
  is_delta,
  delta_value,
  delta_enabled,
  delta_description,
  delta_display_order,
  created_at,
  updated_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateGraphQLAssert :exec
UPDATE graphql_assert
SET
  value = ?,
  enabled = ?,
  description = ?,
  display_order = ?,
  updated_at = ?
WHERE id = ?;

-- name: UpdateGraphQLAssertDelta :exec
UPDATE graphql_assert
SET
  delta_value = ?,
  delta_enabled = ?,
  delta_description = ?,
  delta_display_order = ?,
  updated_at = ?
WHERE id = ?;

-- name: DeleteGraphQLAssert :exec
DELETE FROM graphql_assert
WHERE id = ?;

-- name: GetGraphQLAssertDeltasByWorkspaceID :many
SELECT
  ga.id,
  ga.graphql_id,
  ga.value,
  ga.enabled,
  ga.description,
  ga.display_order,
  ga.parent_graphql_assert_id,
  ga.is_delta,
  ga.delta_value,
  ga.delta_enabled,
  ga.delta_description,
  ga.delta_display_order,
  ga.created_at,
  ga.updated_at
FROM graphql_assert ga
INNER JOIN graphql g ON ga.graphql_id = g.id
WHERE g.workspace_id = ? AND ga.is_delta = TRUE
ORDER BY ga.display_order;

-- name: GetGraphQLAssertDeltasByParentID :many
SELECT
  id,
  graphql_id,
  value,
  enabled,
  description,
  display_order,
  parent_graphql_assert_id,
  is_delta,
  delta_value,
  delta_enabled,
  delta_description,
  delta_display_order,
  created_at,
  updated_at
FROM graphql_assert
WHERE parent_graphql_assert_id = ? AND is_delta = TRUE
ORDER BY display_order;

--
-- GraphQL Version Queries
--

-- name: CreateGraphQLVersion :exec
INSERT INTO graphql_version (
  id, graphql_id, version_name, version_description, is_active, created_at, created_by
)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetGraphQLVersionsByGraphQLID :many
SELECT id, graphql_id, version_name, version_description, is_active, created_at, created_by
FROM graphql_version
WHERE graphql_id = ?
ORDER BY created_at DESC;

--
-- GraphQL Response Assert Queries
--

-- name: CreateGraphQLResponseAssert :exec
INSERT INTO graphql_response_assert (
  id, response_id, value, success, created_at
)
VALUES (?, ?, ?, ?, ?);

-- name: GetGraphQLResponseAssertsByResponseID :many
SELECT id, response_id, value, success, created_at
FROM graphql_response_assert
WHERE response_id = ?
ORDER BY created_at;

-- name: GetGraphQLResponseAssertsByWorkspaceID :many
SELECT
  gra.id,
  gra.response_id,
  gra.value,
  gra.success,
  gra.created_at
FROM graphql_response_assert gra
INNER JOIN graphql_response gr ON gra.response_id = gr.id
INNER JOIN graphql g ON gr.graphql_id = g.id
WHERE g.workspace_id = ?;
