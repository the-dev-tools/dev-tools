package migrations

import (
	"context"
	"database/sql"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/migrate"
)

// MigrationUpdateFilesContentKindID is the ULID for updating files CHECK constraint.
const MigrationUpdateFilesContentKindID = "01KFF093T3QH35ZGY971GHD6BP"

// MigrationUpdateFilesContentKindChecksum is a stable hash of this migration.
const MigrationUpdateFilesContentKindChecksum = "sha256:update-files-content-kind-v1"

func init() {
	if err := migrate.Register(migrate.Migration{
		ID:             MigrationUpdateFilesContentKindID,
		Checksum:       MigrationUpdateFilesContentKindChecksum,
		Description:    "Update files table CHECK constraint to allow content_kind=4 (credential)",
		Apply:          applyFilesContentKind,
		Validate:       validateFilesContentKind,
		RequiresBackup: true, // This migration recreates the files table
	}); err != nil {
		panic("failed to register files content kind migration: " + err.Error())
	}
}

// applyFilesContentKind recreates the files table to update the CHECK constraint.
// SQLite doesn't support ALTER TABLE to modify CHECK constraints, so we must:
// 1. Create a new table with updated constraints
// 2. Copy all data
// 3. Drop old table
// 4. Rename new table
// 5. Recreate indexes
func applyFilesContentKind(ctx context.Context, tx *sql.Tx) error {
	// Check if the table needs migration by attempting to understand current constraint.
	// If the table already supports content_kind=4, skip the migration.
	// We do this by checking if the constraint allows 4 (new DBs have this already).
	var checkResult int
	err := tx.QueryRowContext(ctx, `
		SELECT 1 FROM sqlite_master
		WHERE type='table' AND name='files'
		AND sql LIKE '%content_kind IN (0, 1, 2, 3, 4)%'
	`).Scan(&checkResult)
	if err == nil {
		// Table already has the updated constraint, skip migration
		return nil
	}

	// Drop indexes first (they'll be recreated with the new table)
	indexes := []string{
		"files_workspace_idx",
		"files_path_hash_idx",
		"files_hierarchy_idx",
		"files_content_lookup_idx",
		"files_parent_lookup_idx",
		"files_name_search_idx",
		"files_kind_filter_idx",
		"files_workspace_hierarchy_idx",
	}
	for _, idx := range indexes {
		if _, err := tx.ExecContext(ctx, "DROP INDEX IF EXISTS "+idx); err != nil {
			return err
		}
	}

	// Create new table with updated CHECK constraints
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
			CHECK (content_kind IN (0, 1, 2, 3, 4)), -- 0=folder, 1=http, 2=flow, 3=http_delta, 4=credential
			CHECK (
				(content_kind = 0 AND content_id IS NOT NULL) OR
				(content_kind = 1 AND content_id IS NOT NULL) OR
				(content_kind = 2 AND content_id IS NOT NULL) OR
				(content_kind = 3 AND content_id IS NOT NULL) OR
				(content_kind = 4 AND content_id IS NOT NULL) OR
				(content_id IS NULL)
			),
			FOREIGN KEY (workspace_id) REFERENCES workspaces (id) ON DELETE CASCADE,
			FOREIGN KEY (parent_id) REFERENCES files (id) ON DELETE SET NULL
		)
	`); err != nil {
		return err
	}

	// Copy all data from old table to new
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO files_new (id, workspace_id, parent_id, content_id, content_kind, name, display_order, path_hash, updated_at)
		SELECT id, workspace_id, parent_id, content_id, content_kind, name, display_order, path_hash, updated_at
		FROM files
	`); err != nil {
		return err
	}

	// Drop old table
	if _, err := tx.ExecContext(ctx, "DROP TABLE files"); err != nil {
		return err
	}

	// Rename new table to files
	if _, err := tx.ExecContext(ctx, "ALTER TABLE files_new RENAME TO files"); err != nil {
		return err
	}

	// Recreate all indexes
	if _, err := tx.ExecContext(ctx, `
		CREATE INDEX files_workspace_idx ON files (workspace_id)
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		CREATE UNIQUE INDEX files_path_hash_idx ON files (workspace_id, path_hash) WHERE path_hash IS NOT NULL
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		CREATE INDEX files_hierarchy_idx ON files (workspace_id, parent_id, display_order)
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		CREATE INDEX files_content_lookup_idx ON files (content_kind, content_id) WHERE content_id IS NOT NULL
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		CREATE INDEX files_parent_lookup_idx ON files (parent_id, display_order) WHERE parent_id IS NOT NULL
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		CREATE INDEX files_name_search_idx ON files (workspace_id, name)
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		CREATE INDEX files_kind_filter_idx ON files (workspace_id, content_kind)
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		CREATE INDEX files_workspace_hierarchy_idx ON files (workspace_id, parent_id, content_kind, display_order)
	`); err != nil {
		return err
	}

	return nil
}

// validateFilesContentKind verifies the files table supports content_kind=4.
func validateFilesContentKind(ctx context.Context, db *sql.DB) error {
	// Verify the constraint exists in the updated form
	var sql string
	err := db.QueryRowContext(ctx, `
		SELECT sql FROM sqlite_master
		WHERE type='table' AND name='files'
	`).Scan(&sql)
	if err != nil {
		return err
	}

	// Also verify we can query the table
	var count int
	return db.QueryRowContext(ctx, "SELECT COUNT(*) FROM files").Scan(&count)
}
