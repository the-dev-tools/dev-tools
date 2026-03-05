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
const MigrationAddGraphQLTablesChecksum = "sha256:add-graphql-tables-v2"

func init() {
	if err := migrate.Register(migrate.Migration{
		ID:             MigrationAddGraphQLTablesID,
		Checksum:       MigrationAddGraphQLTablesChecksum,
		Description:    "Add GraphQL tables with delta support, assertions, and response history",
		Apply:          applyGraphQLTables,
		Validate:       validateGraphQLTables,
		RequiresBackup: true,
	}); err != nil {
		panic("failed to register GraphQL tables migration: " + err.Error())
	}
}

func applyGraphQLTables(ctx context.Context, tx *sql.Tx) error {
	// 1. Create graphql table (with delta columns inline)
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

			-- Delta system
			parent_graphql_id BLOB DEFAULT NULL,
			is_delta BOOLEAN NOT NULL DEFAULT FALSE,
			is_snapshot BOOLEAN NOT NULL DEFAULT FALSE,
			delta_name TEXT NULL,
			delta_url TEXT NULL,
			delta_query TEXT NULL,
			delta_variables TEXT NULL,
			delta_description TEXT NULL,

			FOREIGN KEY (workspace_id) REFERENCES workspaces (id) ON DELETE CASCADE,
			FOREIGN KEY (folder_id) REFERENCES files (id) ON DELETE SET NULL
		)
	`); err != nil {
		return err
	}

	graphqlIndexes := []string{
		`CREATE INDEX IF NOT EXISTS graphql_workspace_idx ON graphql (workspace_id)`,
		`CREATE INDEX IF NOT EXISTS graphql_folder_idx ON graphql (folder_id) WHERE folder_id IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS graphql_parent_delta_idx ON graphql (parent_graphql_id, is_delta)`,
		`CREATE INDEX IF NOT EXISTS graphql_delta_resolution_idx ON graphql (parent_graphql_id, is_delta, updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS graphql_active_streaming_idx ON graphql (workspace_id, updated_at DESC) WHERE is_delta = FALSE`,
	}
	for _, idx := range graphqlIndexes {
		if _, err := tx.ExecContext(ctx, idx); err != nil {
			return fmt.Errorf("create graphql index: %w", err)
		}
	}

	// 2. Create graphql_version table
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS graphql_version (
			id BLOB NOT NULL PRIMARY KEY,
			graphql_id BLOB NOT NULL,
			version_name TEXT NOT NULL,
			version_description TEXT NOT NULL DEFAULT '',
			is_active BOOLEAN NOT NULL DEFAULT FALSE,
			created_at BIGINT NOT NULL DEFAULT (unixepoch()),
			created_by BLOB,

			FOREIGN KEY (graphql_id) REFERENCES graphql (id) ON DELETE CASCADE,
			FOREIGN KEY (created_by) REFERENCES users (id) ON DELETE SET NULL,
			CHECK (version_name != '')
		)
	`); err != nil {
		return err
	}

	versionIndexes := []string{
		`CREATE INDEX IF NOT EXISTS graphql_version_graphql_idx ON graphql_version (graphql_id)`,
		`CREATE INDEX IF NOT EXISTS graphql_version_active_idx ON graphql_version (is_active) WHERE is_active = TRUE`,
		`CREATE INDEX IF NOT EXISTS graphql_version_created_by_idx ON graphql_version (created_by)`,
	}
	for _, idx := range versionIndexes {
		if _, err := tx.ExecContext(ctx, idx); err != nil {
			return fmt.Errorf("create graphql_version index: %w", err)
		}
	}

	// 3. Create graphql_header table (with delta columns inline)
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

			-- Delta system
			parent_graphql_header_id BLOB DEFAULT NULL,
			is_delta BOOLEAN NOT NULL DEFAULT FALSE,
			delta_header_key TEXT NULL,
			delta_header_value TEXT NULL,
			delta_description TEXT NULL,
			delta_enabled BOOLEAN NULL,
			delta_display_order REAL NULL,

			FOREIGN KEY (graphql_id) REFERENCES graphql (id) ON DELETE CASCADE
		)
	`); err != nil {
		return err
	}

	headerIndexes := []string{
		`CREATE INDEX IF NOT EXISTS graphql_header_graphql_idx ON graphql_header (graphql_id)`,
		`CREATE INDEX IF NOT EXISTS graphql_header_order_idx ON graphql_header (graphql_id, display_order)`,
		`CREATE INDEX IF NOT EXISTS graphql_header_parent_delta_idx ON graphql_header (parent_graphql_header_id, is_delta)`,
		`CREATE INDEX IF NOT EXISTS graphql_header_delta_streaming_idx ON graphql_header (parent_graphql_header_id, is_delta, updated_at DESC)`,
	}
	for _, idx := range headerIndexes {
		if _, err := tx.ExecContext(ctx, idx); err != nil {
			return fmt.Errorf("create graphql_header index: %w", err)
		}
	}

	// 4. Create graphql_assert table (with delta columns inline)
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS graphql_assert (
			id BLOB NOT NULL PRIMARY KEY,
			graphql_id BLOB NOT NULL,
			value TEXT NOT NULL,
			enabled BOOLEAN NOT NULL DEFAULT TRUE,
			description TEXT NOT NULL DEFAULT '',
			display_order REAL NOT NULL DEFAULT 0,
			created_at BIGINT NOT NULL DEFAULT (unixepoch()),
			updated_at BIGINT NOT NULL DEFAULT (unixepoch()),

			-- Delta system
			parent_graphql_assert_id BLOB DEFAULT NULL,
			is_delta BOOLEAN NOT NULL DEFAULT FALSE,
			delta_value TEXT NULL,
			delta_enabled BOOLEAN NULL,
			delta_description TEXT NULL,
			delta_display_order REAL NULL,

			FOREIGN KEY (graphql_id) REFERENCES graphql (id) ON DELETE CASCADE
		)
	`); err != nil {
		return err
	}

	assertIndexes := []string{
		`CREATE INDEX IF NOT EXISTS graphql_assert_graphql_idx ON graphql_assert (graphql_id)`,
		`CREATE INDEX IF NOT EXISTS graphql_assert_order_idx ON graphql_assert (graphql_id, display_order)`,
		`CREATE INDEX IF NOT EXISTS graphql_assert_parent_delta_idx ON graphql_assert (parent_graphql_assert_id, is_delta)`,
		`CREATE INDEX IF NOT EXISTS graphql_assert_delta_streaming_idx ON graphql_assert (parent_graphql_assert_id, is_delta, updated_at DESC)`,
	}
	for _, idx := range assertIndexes {
		if _, err := tx.ExecContext(ctx, idx); err != nil {
			return fmt.Errorf("create graphql_assert index: %w", err)
		}
	}

	// 5. Create graphql_response table
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

	responseIndexes := []string{
		`CREATE INDEX IF NOT EXISTS graphql_response_graphql_idx ON graphql_response (graphql_id)`,
		`CREATE INDEX IF NOT EXISTS graphql_response_time_idx ON graphql_response (graphql_id, time DESC)`,
	}
	for _, idx := range responseIndexes {
		if _, err := tx.ExecContext(ctx, idx); err != nil {
			return fmt.Errorf("create graphql_response index: %w", err)
		}
	}

	// 6. Create graphql_response_header table
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

	// 7. Create graphql_response_assert table
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS graphql_response_assert (
			id BLOB NOT NULL PRIMARY KEY,
			response_id BLOB NOT NULL,
			value TEXT NOT NULL,
			success BOOLEAN NOT NULL,
			created_at BIGINT NOT NULL DEFAULT (unixepoch()),

			FOREIGN KEY (response_id) REFERENCES graphql_response (id) ON DELETE CASCADE
		)
	`); err != nil {
		return err
	}

	responseAssertIndexes := []string{
		`CREATE INDEX IF NOT EXISTS graphql_response_assert_response_idx ON graphql_response_assert (response_id)`,
		`CREATE INDEX IF NOT EXISTS graphql_response_assert_success_idx ON graphql_response_assert (response_id, success)`,
	}
	for _, idx := range responseAssertIndexes {
		if _, err := tx.ExecContext(ctx, idx); err != nil {
			return fmt.Errorf("create graphql_response_assert index: %w", err)
		}
	}

	// 8. Create flow_node_graphql table
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS flow_node_graphql (
			flow_node_id BLOB NOT NULL PRIMARY KEY,
			graphql_id BLOB NOT NULL,
			FOREIGN KEY (graphql_id) REFERENCES graphql (id) ON DELETE CASCADE
		)
	`); err != nil {
		return err
	}

	// 9. Add graphql_response_id column to node_execution table
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

	// 10. Update files table CHECK constraint to allow content_kind = 5 (graphql)
	if err := updateFilesCheckConstraint(ctx, tx); err != nil {
		return fmt.Errorf("update files check constraint: %w", err)
	}

	return nil
}

// updateFilesCheckConstraint recreates the files table with GraphQL content_kind support.
func updateFilesCheckConstraint(ctx context.Context, tx *sql.Tx) error {
	var tableSql string
	err := tx.QueryRowContext(ctx, `
		SELECT sql FROM sqlite_master WHERE type='table' AND name='files'
	`).Scan(&tableSql)
	if err != nil {
		return fmt.Errorf("read files table schema: %w", err)
	}
	if strings.Contains(tableSql, "4, 5)") {
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

func validateGraphQLTables(ctx context.Context, db *sql.DB) error {
	tables := []string{
		"graphql",
		"graphql_version",
		"graphql_header",
		"graphql_assert",
		"graphql_response",
		"graphql_response_header",
		"graphql_response_assert",
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
		"graphql_parent_delta_idx",
		"graphql_delta_resolution_idx",
		"graphql_active_streaming_idx",
		"graphql_version_graphql_idx",
		"graphql_header_graphql_idx",
		"graphql_header_order_idx",
		"graphql_header_parent_delta_idx",
		"graphql_header_delta_streaming_idx",
		"graphql_assert_graphql_idx",
		"graphql_assert_order_idx",
		"graphql_assert_parent_delta_idx",
		"graphql_assert_delta_streaming_idx",
		"graphql_response_graphql_idx",
		"graphql_response_time_idx",
		"graphql_response_header_response_idx",
		"graphql_response_assert_response_idx",
		"graphql_response_assert_success_idx",
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

	// Verify delta columns exist
	deltaColumns := map[string][]string{
		"graphql":        {"parent_graphql_id", "is_delta", "is_snapshot", "delta_name", "delta_url", "delta_query", "delta_variables", "delta_description"},
		"graphql_header": {"parent_graphql_header_id", "is_delta", "delta_header_key", "delta_header_value", "delta_description", "delta_enabled", "delta_display_order"},
		"graphql_assert": {"parent_graphql_assert_id", "is_delta", "delta_value", "delta_enabled", "delta_description", "delta_display_order"},
	}

	for table, cols := range deltaColumns {
		for _, col := range cols {
			var colCount int
			err := db.QueryRowContext(ctx, `
				SELECT COUNT(*) FROM pragma_table_info(?)
				WHERE name = ?
			`, table, col).Scan(&colCount)
			if err != nil {
				return fmt.Errorf("check %s.%s: %w", table, col, err)
			}
			if colCount == 0 {
				return fmt.Errorf("column %s.%s not found", table, col)
			}
		}
	}

	return nil
}
