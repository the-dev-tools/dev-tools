-- name: GetFlow :one
SELECT
  id,
  workspace_id,
  version_parent_id,
  name,
  duration,
  running
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
  running
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
  duration,
  running
FROM
  flow
WHERE
  version_parent_id is ?;

-- name: CreateFlow :exec
INSERT INTO
  flow (id, workspace_id, version_parent_id, name, duration, running)
VALUES
  (?, ?, ?, ?, ?, ?);

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

-- name: CleanupOrphanedFlowNodeNoop :exec
DELETE FROM flow_node_noop WHERE flow_node_id NOT IN (SELECT id FROM flow_node);

-- name: CleanupOrphanedFlowNodeJs :exec
DELETE FROM flow_node_js WHERE flow_node_id NOT IN (SELECT id FROM flow_node);

-- name: CleanupOrphanedFlowEdges :exec
DELETE FROM flow_edge WHERE source_id NOT IN (SELECT id FROM flow_node) OR target_id NOT IN (SELECT id FROM flow_node);

-- name: CleanupOrphanedNodeExecutions :exec
DELETE FROM node_execution WHERE node_id NOT IN (SELECT id FROM flow_node);
