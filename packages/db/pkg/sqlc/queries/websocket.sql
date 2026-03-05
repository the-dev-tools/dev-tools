--
-- WebSocket Core Queries
--

-- name: GetWebSocket :one
SELECT
  id, workspace_id, folder_id, name, url,
  description, last_run_at, created_at, updated_at
FROM websocket
WHERE id = ? LIMIT 1;

-- name: GetWebSocketsByWorkspaceID :many
SELECT
  id, workspace_id, folder_id, name, url,
  description, last_run_at, created_at, updated_at
FROM websocket
WHERE workspace_id = ?
ORDER BY updated_at DESC;

-- name: GetWebSocketWorkspaceID :one
SELECT workspace_id
FROM websocket
WHERE id = ?
LIMIT 1;

-- name: CreateWebSocket :exec
INSERT INTO websocket (
  id, workspace_id, folder_id, name, url,
  description, last_run_at, created_at, updated_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateWebSocket :exec
UPDATE websocket
SET
  name = ?,
  url = ?,
  description = ?,
  last_run_at = COALESCE(?, last_run_at),
  updated_at = unixepoch()
WHERE id = ?;

-- name: DeleteWebSocket :exec
DELETE FROM websocket
WHERE id = ?;

--
-- WebSocket Header Queries
--

-- name: GetWebSocketHeaders :many
SELECT
  id, websocket_id, header_key, header_value, description,
  enabled, display_order, created_at, updated_at
FROM websocket_header
WHERE websocket_id = ?
ORDER BY display_order;

-- name: GetWebSocketHeaderByID :one
SELECT
  id, websocket_id, header_key, header_value, description,
  enabled, display_order, created_at, updated_at
FROM websocket_header
WHERE id = ? LIMIT 1;

-- name: CreateWebSocketHeader :exec
INSERT INTO websocket_header (
  id, websocket_id, header_key, header_value, description,
  enabled, display_order, created_at, updated_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateWebSocketHeader :exec
UPDATE websocket_header
SET
  header_key = ?,
  header_value = ?,
  description = ?,
  enabled = ?,
  display_order = ?,
  updated_at = unixepoch()
WHERE id = ?;

-- name: DeleteWebSocketHeader :exec
DELETE FROM websocket_header
WHERE id = ?;

-- name: DeleteWebSocketHeadersByWebSocketID :exec
DELETE FROM websocket_header
WHERE websocket_id = ?;

--
-- Flow Node WebSocket Queries
--

-- name: GetFlowNodeWsConnection :one
SELECT
  flow_node_id,
  websocket_id
FROM flow_node_ws_connection
WHERE flow_node_id = ?
LIMIT 1;

-- name: CreateFlowNodeWsConnection :exec
INSERT INTO flow_node_ws_connection (flow_node_id, websocket_id) VALUES (?, ?);

-- name: UpdateFlowNodeWsConnection :exec
INSERT INTO flow_node_ws_connection (flow_node_id, websocket_id) VALUES (?, ?)
ON CONFLICT(flow_node_id) DO UPDATE SET
  websocket_id = excluded.websocket_id;

-- name: DeleteFlowNodeWsConnection :exec
DELETE FROM flow_node_ws_connection WHERE flow_node_id = ?;

-- name: GetFlowNodeWsSend :one
SELECT
  flow_node_id,
  ws_connection_node_name,
  message
FROM flow_node_ws_send
WHERE flow_node_id = ?
LIMIT 1;

-- name: CreateFlowNodeWsSend :exec
INSERT INTO flow_node_ws_send (flow_node_id, ws_connection_node_name, message) VALUES (?, ?, ?);

-- name: UpdateFlowNodeWsSend :exec
INSERT INTO flow_node_ws_send (flow_node_id, ws_connection_node_name, message) VALUES (?, ?, ?)
ON CONFLICT(flow_node_id) DO UPDATE SET
  ws_connection_node_name = excluded.ws_connection_node_name,
  message = excluded.message;

-- name: DeleteFlowNodeWsSend :exec
DELETE FROM flow_node_ws_send WHERE flow_node_id = ?;
