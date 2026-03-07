/*
 *
 * WEBSOCKET SYSTEM
 * WebSocket connection support for flows and standalone testing
 *
 */

-- Core WebSocket connection definition
CREATE TABLE websocket (
  id BLOB NOT NULL PRIMARY KEY,
  workspace_id BLOB NOT NULL,
  folder_id BLOB,
  name TEXT NOT NULL,
  url TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  last_run_at BIGINT NULL,
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),

  FOREIGN KEY (workspace_id) REFERENCES workspaces (id) ON DELETE CASCADE,
  FOREIGN KEY (folder_id) REFERENCES files (id) ON DELETE SET NULL
);

CREATE INDEX websocket_workspace_idx ON websocket (workspace_id);
CREATE INDEX websocket_folder_idx ON websocket (folder_id) WHERE folder_id IS NOT NULL;

-- WebSocket connection headers (sent during handshake)
CREATE TABLE websocket_header (
  id BLOB NOT NULL PRIMARY KEY,
  websocket_id BLOB NOT NULL,
  header_key TEXT NOT NULL,
  header_value TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  display_order REAL NOT NULL DEFAULT 0,
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),

  FOREIGN KEY (websocket_id) REFERENCES websocket (id) ON DELETE CASCADE
);

CREATE INDEX websocket_header_ws_idx ON websocket_header (websocket_id);
CREATE INDEX websocket_header_order_idx ON websocket_header (websocket_id, display_order);

-- Flow node: WebSocket Connection (entry/listener node)
CREATE TABLE flow_node_ws_connection (
  flow_node_id BLOB NOT NULL PRIMARY KEY,
  websocket_id BLOB,
  FOREIGN KEY (websocket_id) REFERENCES websocket (id) ON DELETE SET NULL
);

-- Flow node: WebSocket Send (action node)
CREATE TABLE flow_node_ws_send (
  flow_node_id BLOB NOT NULL PRIMARY KEY,
  ws_connection_node_name TEXT NOT NULL DEFAULT '',
  message TEXT NOT NULL DEFAULT ''
);
