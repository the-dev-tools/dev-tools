-- name: GetFlow :one
SELECT
  id,
  workspace_id,
  version_parent_id,
  name,
  duration,
  running,
  node_id_mapping
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
  duration,
  running,
  node_id_mapping
FROM
  flow
WHERE
  workspace_id = ? AND
  version_parent_id is NULL;

-- name: GetAllFlowsByWorkspaceID :many
-- Returns all flows including versions for TanStack DB sync
SELECT
  id,
  workspace_id,
  version_parent_id,
  name,
  duration,
  running,
  node_id_mapping
FROM
  flow
WHERE
  workspace_id = ?;

-- name: GetFlowsByVersionParentID :many
SELECT
  id,
  workspace_id,
  version_parent_id,
  name,
  duration,
  running,
  node_id_mapping
FROM
  flow
WHERE
  version_parent_id is ?;

-- name: CreateFlow :exec
INSERT INTO
  flow (id, workspace_id, version_parent_id, name, duration, running, node_id_mapping)
VALUES
  (?, ?, ?, ?, ?, ?, ?);

-- name: CreateFlowsBulk :exec
INSERT INTO
  flow (id, workspace_id, version_parent_id, name, duration, running, node_id_mapping)
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

-- name: UpdateFlow :exec
UPDATE flow
SET
  name = ?,
  duration = ?,
  running = ?
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
  position_y,
  state
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
  position_y,
  state
FROM
  flow_node
WHERE
  flow_id = ?;

-- name: GetFlowNodesByFlowIDs :many
-- Batch query for cascade collection - fetches all nodes for multiple flows
SELECT id, flow_id
FROM flow_node
WHERE flow_id IN (sqlc.slice('flow_ids'));

-- name: CreateFlowNode :exec
INSERT INTO
  flow_node (id, flow_id, name, node_kind, position_x, position_y, state)
VALUES
  (?, ?, ?, ?, ?, ?, 0);

-- name: CreateFlowNodeWithState :exec
INSERT INTO
  flow_node (id, flow_id, name, node_kind, position_x, position_y, state)
VALUES
  (?, ?, ?, ?, ?, ?, ?);

-- name: CreateFlowNodesBulk :exec
INSERT INTO
  flow_node (id, flow_id, name, node_kind, position_x, position_y, state)
VALUES
  (?, ?, ?, ?, ?, ?, 0),
  (?, ?, ?, ?, ?, ?, 0),
  (?, ?, ?, ?, ?, ?, 0),
  (?, ?, ?, ?, ?, ?, 0),
  (?, ?, ?, ?, ?, ?, 0),
  (?, ?, ?, ?, ?, ?, 0),
  (?, ?, ?, ?, ?, ?, 0),
  (?, ?, ?, ?, ?, ?, 0),
  (?, ?, ?, ?, ?, ?, 0),
  (?, ?, ?, ?, ?, ?, 0);

-- name: UpdateFlowNode :exec
UPDATE flow_node
SET
  name = ?,
  position_x = ?,
  position_y = ?
WHERE
  id = ?;

-- name: UpdateFlowNodeState :exec
UPDATE flow_node
SET
  state = ?
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
  state
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
  state
FROM
  flow_edge
WHERE
  flow_id = ?;

-- name: GetFlowEdgesByFlowIDs :many
-- Batch query for cascade collection - fetches all edges for multiple flows
SELECT id, flow_id
FROM flow_edge
WHERE flow_id IN (sqlc.slice('flow_ids'));

-- name: CreateFlowEdge :exec
INSERT INTO
  flow_edge (id, flow_id, source_id, target_id, source_handle, state)
VALUES
  (?, ?, ?, ?, ?, 0);

-- name: UpdateFlowEdge :exec
UPDATE flow_edge
SET
  source_id = ?,
  target_id = ?,
  source_handle = ?
WHERE
  id = ?;

-- name: UpdateFlowEdgeState :exec
UPDATE flow_edge
SET
  state = ?
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

-- name: GetFlowNodeHTTP :one
SELECT
  flow_node_id,
  http_id,
  delta_http_id
FROM
  flow_node_http
WHERE
  flow_node_id = ?
LIMIT 1;

-- name: CreateFlowNodeHTTP :exec
INSERT INTO
  flow_node_http (
    flow_node_id,
    http_id,
    delta_http_id
  )
VALUES
  (?, ?, ?);

-- name: UpdateFlowNodeHTTP :exec
INSERT INTO flow_node_http (
    flow_node_id,
    http_id,
    delta_http_id
)
VALUES
    (?, ?, ?)
ON CONFLICT(flow_node_id) DO UPDATE SET
    http_id = excluded.http_id,
    delta_http_id = excluded.delta_http_id;

-- name: DeleteFlowNodeHTTP :exec
DELETE FROM flow_node_http
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
  display_order
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
  display_order
FROM
  flow_variable
WHERE
  flow_id = ?;

-- name: GetFlowVariablesByFlowIDs :many
-- Batch query for cascade collection - fetches all variables for multiple flows
SELECT id, flow_id
FROM flow_variable
WHERE flow_id IN (sqlc.slice('flow_ids'));

-- name: CreateFlowVariable :exec
INSERT INTO
  flow_variable (id, flow_id, key, value, enabled, description, display_order)
VALUES
  (?, ?, ?, ?, ?, ?, ?);

-- name: CreateFlowVariableBulk :exec
INSERT INTO
  flow_variable (id, flow_id, key, value, enabled, description, display_order)
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
SELECT
  id,
  flow_id,
  key,
  value,
  enabled,
  description,
  display_order
FROM
  flow_variable
WHERE
  flow_id = ?
ORDER BY
  display_order;

-- name: UpdateFlowVariableOrder :exec
UPDATE flow_variable
SET
  display_order = ?
WHERE
  id = ?;

-- Node Execution
-- name: GetNodeExecution :one
SELECT * FROM node_execution
WHERE id = ?;

-- name: ListNodeExecutions :many
SELECT * FROM node_execution
WHERE node_id = ?
ORDER BY completed_at DESC, id DESC
LIMIT ? OFFSET ?;

-- name: ListNodeExecutionsByState :many
SELECT * FROM node_execution
WHERE node_id = ? AND state = ?
ORDER BY completed_at DESC, id DESC
LIMIT ? OFFSET ?;

-- name: ListNodeExecutionsByFlowRun :many
SELECT ne.* FROM node_execution ne
JOIN flow_node fn ON ne.node_id = fn.id
WHERE fn.flow_id = ?
ORDER BY ne.completed_at DESC, ne.id DESC;

-- name: CreateNodeExecution :one
INSERT INTO node_execution (
  id, node_id, name, state, error, input_data, input_data_compress_type,
  output_data, output_data_compress_type, http_response_id, completed_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateNodeExecution :one
UPDATE node_execution
SET state = ?, error = ?, output_data = ?, 
    output_data_compress_type = ?, http_response_id = ?, completed_at = ?
WHERE id = ?
RETURNING *;

-- name: UpsertNodeExecution :one
INSERT INTO node_execution (
  id, node_id, name, state, error, input_data, input_data_compress_type,
  output_data, output_data_compress_type, http_response_id, completed_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  state = excluded.state,
  error = excluded.error, 
  input_data = excluded.input_data,
  input_data_compress_type = excluded.input_data_compress_type,
  output_data = excluded.output_data,
  output_data_compress_type = excluded.output_data_compress_type,
  http_response_id = excluded.http_response_id,
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

-- Cleanup queries for orphaned flow_node sub-table records
-- (used when FK constraints are removed for flexible insert ordering)

-- name: CleanupOrphanedFlowNodeFor :exec
DELETE FROM flow_node_for WHERE flow_node_id NOT IN (SELECT id FROM flow_node);

-- name: CleanupOrphanedFlowNodeForEach :exec
DELETE FROM flow_node_for_each WHERE flow_node_id NOT IN (SELECT id FROM flow_node);

-- name: CleanupOrphanedFlowNodeHttp :exec
DELETE FROM flow_node_http WHERE flow_node_id NOT IN (SELECT id FROM flow_node);

-- name: CleanupOrphanedFlowNodeCondition :exec
DELETE FROM flow_node_condition WHERE flow_node_id NOT IN (SELECT id FROM flow_node);

-- name: CleanupOrphanedFlowNodeJs :exec
DELETE FROM flow_node_js WHERE flow_node_id NOT IN (SELECT id FROM flow_node);

-- name: CleanupOrphanedFlowEdges :exec
DELETE FROM flow_edge WHERE source_id NOT IN (SELECT id FROM flow_node) OR target_id NOT IN (SELECT id FROM flow_node);

-- name: CleanupOrphanedNodeExecutions :exec
DELETE FROM node_execution WHERE node_id NOT IN (SELECT id FROM flow_node);

-- name: GetLatestVersionByParentID :one
SELECT
  id,
  workspace_id,
  version_parent_id,
  name,
  duration,
  running,
  node_id_mapping
FROM
  flow
WHERE
  version_parent_id = ?
ORDER BY id DESC
LIMIT 1;

-- name: UpdateFlowNodeIDMapping :exec
UPDATE flow
SET node_id_mapping = ?
WHERE id = ?;

-- name: UpdateNodeExecutionNodeID :exec
UPDATE node_execution
SET node_id = ?
WHERE id = ?;
