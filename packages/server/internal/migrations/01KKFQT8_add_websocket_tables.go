package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/migrate"
)

// MigrationAddWebSocketTablesID is the ULID for the WebSocket tables migration.
const MigrationAddWebSocketTablesID = "01KKFQT81KV5MX8H9MNTPCWDV9"

// MigrationAddWebSocketTablesChecksum is a stable hash of this migration.
const MigrationAddWebSocketTablesChecksum = "sha256:add-websocket-tables-v1"

func init() {
	if err := migrate.Register(migrate.Migration{
		ID:             MigrationAddWebSocketTablesID,
		Checksum:       MigrationAddWebSocketTablesChecksum,
		Description:    "Add WebSocket tables for connections, headers, and flow nodes",
		Apply:          applyWebSocketTables,
		Validate:       validateWebSocketTables,
		RequiresBackup: true,
	}); err != nil {
		panic("failed to register WebSocket tables migration: " + err.Error())
	}
}

func applyWebSocketTables(ctx context.Context, tx *sql.Tx) error {
	// 1. Create websocket table
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS websocket (
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
		)
	`); err != nil {
		return err
	}

	wsIndexes := []string{
		`CREATE INDEX IF NOT EXISTS websocket_workspace_idx ON websocket (workspace_id)`,
		`CREATE INDEX IF NOT EXISTS websocket_folder_idx ON websocket (folder_id) WHERE folder_id IS NOT NULL`,
	}
	for _, idx := range wsIndexes {
		if _, err := tx.ExecContext(ctx, idx); err != nil {
			return fmt.Errorf("create websocket index: %w", err)
		}
	}

	// 2. Create websocket_header table
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS websocket_header (
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
		)
	`); err != nil {
		return err
	}

	headerIndexes := []string{
		`CREATE INDEX IF NOT EXISTS websocket_header_ws_idx ON websocket_header (websocket_id)`,
		`CREATE INDEX IF NOT EXISTS websocket_header_order_idx ON websocket_header (websocket_id, display_order)`,
	}
	for _, idx := range headerIndexes {
		if _, err := tx.ExecContext(ctx, idx); err != nil {
			return fmt.Errorf("create websocket_header index: %w", err)
		}
	}

	// 3. Create flow_node_ws_connection table
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS flow_node_ws_connection (
			flow_node_id BLOB NOT NULL PRIMARY KEY,
			websocket_id BLOB,
			FOREIGN KEY (websocket_id) REFERENCES websocket (id) ON DELETE SET NULL
		)
	`); err != nil {
		return err
	}

	// 4. Create flow_node_ws_send table
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS flow_node_ws_send (
			flow_node_id BLOB NOT NULL PRIMARY KEY,
			ws_connection_node_name TEXT NOT NULL DEFAULT '',
			message TEXT NOT NULL DEFAULT ''
		)
	`); err != nil {
		return err
	}

	// 5. Update files table CHECK constraint to allow content_kind = 6 (websocket)
	if err := updateFilesCheckConstraintWebSocket(ctx, tx); err != nil {
		return fmt.Errorf("update files check constraint: %w", err)
	}

	return nil
}

// updateFilesCheckConstraintWebSocket recreates the files table with WebSocket content_kind support.
func updateFilesCheckConstraintWebSocket(ctx context.Context, tx *sql.Tx) error {
	var tableSql string
	err := tx.QueryRowContext(ctx, `
		SELECT sql FROM sqlite_master WHERE type='table' AND name='files'
	`).Scan(&tableSql)
	if err != nil {
		return fmt.Errorf("read files table schema: %w", err)
	}
	if strings.Contains(tableSql, "5, 6)") {
		return nil
	}

	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE files_new (
			id BLOB NOT NULL PRIMARY KEY,
			workspace_id BLOB NOT NULL,
			parent_id BLOB,
			content_id BLOB,
			content_kind INT8 NOT NULL DEFAULT 0,
			name TEXT NOT NULL,
			display_order REAL NOT NULL DEFAULT 0,
			path_hash TEXT,
			updated_at BIGINT NOT NULL DEFAULT (unixepoch()),
			CHECK (length (id) == 16),
			CHECK (content_kind IN (0, 1, 2, 3, 4, 5, 6)),
			CHECK (
				(content_kind = 0 AND content_id IS NOT NULL) OR
				(content_kind = 1 AND content_id IS NOT NULL) OR
				(content_kind = 2 AND content_id IS NOT NULL) OR
				(content_kind = 3 AND content_id IS NOT NULL) OR
				(content_kind = 4 AND content_id IS NOT NULL) OR
				(content_kind = 5 AND content_id IS NOT NULL) OR
				(content_kind = 6 AND content_id IS NOT NULL) OR
				(content_id IS NULL)
			),
			FOREIGN KEY (workspace_id) REFERENCES workspaces (id) ON DELETE CASCADE,
			FOREIGN KEY (parent_id) REFERENCES files (id) ON DELETE SET NULL
		)
	`); err != nil {
		return fmt.Errorf("create files_new: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO files_new SELECT * FROM files`); err != nil {
		return fmt.Errorf("copy files data: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DROP TABLE files`); err != nil {
		return fmt.Errorf("drop old files: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE files_new RENAME TO files`); err != nil {
		return fmt.Errorf("rename files_new: %w", err)
	}

	indexes := []string{
		`CREATE INDEX files_workspace_idx ON files (workspace_id)`,
		`CREATE UNIQUE INDEX files_path_hash_idx ON files (workspace_id, path_hash) WHERE path_hash IS NOT NULL`,
		`CREATE INDEX files_hierarchy_idx ON files (workspace_id, parent_id, display_order)`,
		`CREATE INDEX files_content_lookup_idx ON files (content_kind, content_id) WHERE content_id IS NOT NULL`,
		`CREATE INDEX files_parent_lookup_idx ON files (parent_id, display_order) WHERE parent_id IS NOT NULL`,
		`CREATE INDEX files_name_search_idx ON files (workspace_id, name)`,
		`CREATE INDEX files_kind_filter_idx ON files (workspace_id, content_kind)`,
		`CREATE INDEX files_workspace_hierarchy_idx ON files (workspace_id, parent_id, content_kind, display_order)`,
	}
	for _, idx := range indexes {
		if _, err := tx.ExecContext(ctx, idx); err != nil {
			return fmt.Errorf("recreate index: %w", err)
		}
	}

	return nil
}

func validateWebSocketTables(ctx context.Context, db *sql.DB) error {
	tables := []string{
		"websocket",
		"websocket_header",
		"flow_node_ws_connection",
		"flow_node_ws_send",
	}

	for _, table := range tables {
		var name string
		err := db.QueryRowContext(ctx, `
			SELECT name FROM sqlite_master
			WHERE type='table' AND name=?
		`, table).Scan(&name)
		if err != nil {
			return fmt.Errorf("table %s not found: %w", table, err)
		}
	}

	indexes := []string{
		"websocket_workspace_idx",
		"websocket_folder_idx",
		"websocket_header_ws_idx",
		"websocket_header_order_idx",
	}

	for _, idx := range indexes {
		var name string
		err := db.QueryRowContext(ctx, `
			SELECT name FROM sqlite_master
			WHERE type='index' AND name=?
		`, idx).Scan(&name)
		if err != nil {
			return fmt.Errorf("index %s not found: %w", idx, err)
		}
	}

	return nil
}
