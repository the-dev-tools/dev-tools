package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/migrate"
)

// MigrationAddGraphQLTablesID is the ULID for the GraphQL tables migration.
const MigrationAddGraphQLTablesID = "01KHDYWX1KV5MX8H9MNTPCWDV9"

// MigrationAddGraphQLTablesChecksum is a stable hash of this migration.
const MigrationAddGraphQLTablesChecksum = "sha256:add-graphql-tables-v1"

func init() {
	if err := migrate.Register(migrate.Migration{
		ID:             MigrationAddGraphQLTablesID,
		Checksum:       MigrationAddGraphQLTablesChecksum,
		Description:    "Add GraphQL request, header, response, and response header tables",
		Apply:          applyGraphQLTables,
		Validate:       validateGraphQLTables,
		RequiresBackup: true,
	}); err != nil {
		panic("failed to register GraphQL tables migration: " + err.Error())
	}
}

// applyGraphQLTables creates all GraphQL-related tables:
// - graphql (core request)
// - graphql_header (request headers)
// - graphql_response (cached response)
// - graphql_response_header (response headers)
func applyGraphQLTables(ctx context.Context, tx *sql.Tx) error {
	// 1. Create graphql table
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS graphql (
			id BLOB NOT NULL PRIMARY KEY,
			workspace_id BLOB NOT NULL,
			folder_id BLOB,
			name TEXT NOT NULL,
			url TEXT NOT NULL,
			query TEXT NOT NULL DEFAULT '',
			variables TEXT NOT NULL DEFAULT '',
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

	if _, err := tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS graphql_workspace_idx ON graphql (workspace_id)
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS graphql_folder_idx ON graphql (folder_id) WHERE folder_id IS NOT NULL
	`); err != nil {
		return err
	}

	// 2. Create graphql_header table
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS graphql_header (
			id BLOB NOT NULL PRIMARY KEY,
			graphql_id BLOB NOT NULL,
			header_key TEXT NOT NULL,
			header_value TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			enabled BOOLEAN NOT NULL DEFAULT TRUE,
			display_order REAL NOT NULL DEFAULT 0,
			created_at BIGINT NOT NULL DEFAULT (unixepoch()),
			updated_at BIGINT NOT NULL DEFAULT (unixepoch()),

			FOREIGN KEY (graphql_id) REFERENCES graphql (id) ON DELETE CASCADE
		)
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS graphql_header_graphql_idx ON graphql_header (graphql_id)
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS graphql_header_order_idx ON graphql_header (graphql_id, display_order)
	`); err != nil {
		return err
	}

	// 3. Create graphql_response table
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS graphql_response (
			id BLOB NOT NULL PRIMARY KEY,
			graphql_id BLOB NOT NULL,
			status INT32 NOT NULL,
			body BLOB,
			time DATETIME NOT NULL,
			duration INT32 NOT NULL,
			size INT32 NOT NULL,
			created_at BIGINT NOT NULL DEFAULT (unixepoch()),

			FOREIGN KEY (graphql_id) REFERENCES graphql (id) ON DELETE CASCADE
		)
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS graphql_response_graphql_idx ON graphql_response (graphql_id)
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS graphql_response_time_idx ON graphql_response (graphql_id, time DESC)
	`); err != nil {
		return err
	}

	// 4. Create graphql_response_header table
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS graphql_response_header (
			id BLOB NOT NULL PRIMARY KEY,
			response_id BLOB NOT NULL,
			key TEXT NOT NULL,
			value TEXT NOT NULL,
			created_at BIGINT NOT NULL DEFAULT (unixepoch()),

			FOREIGN KEY (response_id) REFERENCES graphql_response (id) ON DELETE CASCADE
		)
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS graphql_response_header_response_idx ON graphql_response_header (response_id)
	`); err != nil {
		return err
	}

	// 5. Create flow_node_graphql table (links flow nodes to GraphQL requests)
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS flow_node_graphql (
			flow_node_id BLOB NOT NULL PRIMARY KEY,
			graphql_id BLOB NOT NULL,
			FOREIGN KEY (graphql_id) REFERENCES graphql (id) ON DELETE CASCADE
		)
	`); err != nil {
		return err
	}

	// 6. Add graphql_response_id column to node_execution table
	// Check if column already exists before adding
	var colCount int
	err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM pragma_table_info('node_execution')
		WHERE name = 'graphql_response_id'
	`).Scan(&colCount)
	if err != nil {
		return fmt.Errorf("check node_execution column: %w", err)
	}
	if colCount == 0 {
		if _, err := tx.ExecContext(ctx, `
			ALTER TABLE node_execution ADD COLUMN graphql_response_id BLOB
				REFERENCES graphql_response (id) ON DELETE SET NULL
		`); err != nil {
			return err
		}
	}

	// 7. Update files table CHECK constraint to allow content_kind = 5 (graphql)
	// SQLite requires table recreation to modify CHECK constraints
	if err := updateFilesCheckConstraint(ctx, tx); err != nil {
		return fmt.Errorf("update files check constraint: %w", err)
	}

	return nil
}

// updateFilesCheckConstraint recreates the files table with GraphQL content_kind support.
func updateFilesCheckConstraint(ctx context.Context, tx *sql.Tx) error {
	// Check if already updated (content_kind = 5 works)
	// We detect by checking the table SQL for "5"
	var tableSql string
	err := tx.QueryRowContext(ctx, `
		SELECT sql FROM sqlite_master WHERE type='table' AND name='files'
	`).Scan(&tableSql)
	if err != nil {
		return fmt.Errorf("read files table schema: %w", err)
	}
	// If the constraint already includes 5, skip
	if strings.Contains(tableSql, "4, 5)") {
		return nil
	}

	// Recreate table with updated CHECK constraints
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
			CHECK (content_kind IN (0, 1, 2, 3, 4, 5)),
			CHECK (
				(content_kind = 0 AND content_id IS NOT NULL) OR
				(content_kind = 1 AND content_id IS NOT NULL) OR
				(content_kind = 2 AND content_id IS NOT NULL) OR
				(content_kind = 3 AND content_id IS NOT NULL) OR
				(content_kind = 4 AND content_id IS NOT NULL) OR
				(content_kind = 5 AND content_id IS NOT NULL) OR
				(content_id IS NULL)
			),
			FOREIGN KEY (workspace_id) REFERENCES workspaces (id) ON DELETE CASCADE,
			FOREIGN KEY (parent_id) REFERENCES files (id) ON DELETE SET NULL
		)
	`); err != nil {
		return fmt.Errorf("create files_new: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO files_new SELECT * FROM files
	`); err != nil {
		return fmt.Errorf("copy files data: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DROP TABLE files`); err != nil {
		return fmt.Errorf("drop old files: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE files_new RENAME TO files`); err != nil {
		return fmt.Errorf("rename files_new: %w", err)
	}

	// Recreate indexes
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


// validateGraphQLTables verifies all tables and indexes were created successfully.
func validateGraphQLTables(ctx context.Context, db *sql.DB) error {
	tables := []string{
		"graphql",
		"graphql_header",
		"graphql_response",
		"graphql_response_header",
		"flow_node_graphql",
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
		"graphql_workspace_idx",
		"graphql_folder_idx",
		"graphql_header_graphql_idx",
		"graphql_header_order_idx",
		"graphql_response_graphql_idx",
		"graphql_response_time_idx",
		"graphql_response_header_response_idx",
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
