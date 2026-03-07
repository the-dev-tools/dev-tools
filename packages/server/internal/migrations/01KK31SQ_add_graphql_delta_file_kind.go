package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/migrate"
)

// MigrationAddGraphQLDeltaFileKindID is the ULID for the GraphQL delta file kind migration.
const MigrationAddGraphQLDeltaFileKindID = "01KK31SQD0GFP92RZ3XMN5V6BQ"

// MigrationAddGraphQLDeltaFileKindChecksum is a stable hash of this migration.
const MigrationAddGraphQLDeltaFileKindChecksum = "sha256:add-graphql-delta-file-kind-v1"

func init() {
	if err := migrate.Register(migrate.Migration{
		ID:             MigrationAddGraphQLDeltaFileKindID,
		Checksum:       MigrationAddGraphQLDeltaFileKindChecksum,
		Description:    "Add GraphQL delta content_kind (7) to files table CHECK constraint",
		Apply:          applyGraphQLDeltaFileKind,
		Validate:       validateGraphQLDeltaFileKind,
		RequiresBackup: true,
	}); err != nil {
		panic("failed to register GraphQL delta file kind migration: " + err.Error())
	}
}

func applyGraphQLDeltaFileKind(ctx context.Context, tx *sql.Tx) error {
	return updateFilesCheckConstraintGraphQLDelta(ctx, tx)
}

// updateFilesCheckConstraintGraphQLDelta recreates the files table with GraphQL delta content_kind support.
func updateFilesCheckConstraintGraphQLDelta(ctx context.Context, tx *sql.Tx) error {
	var tableSql string
	err := tx.QueryRowContext(ctx, `
		SELECT sql FROM sqlite_master WHERE type='table' AND name='files'
	`).Scan(&tableSql)
	if err != nil {
		return fmt.Errorf("read files table schema: %w", err)
	}
	if strings.Contains(tableSql, "6, 7)") {
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
			CHECK (content_kind IN (0, 1, 2, 3, 4, 5, 6, 7)),
			CHECK (
				(content_kind = 0 AND content_id IS NOT NULL) OR
				(content_kind = 1 AND content_id IS NOT NULL) OR
				(content_kind = 2 AND content_id IS NOT NULL) OR
				(content_kind = 3 AND content_id IS NOT NULL) OR
				(content_kind = 4 AND content_id IS NOT NULL) OR
				(content_kind = 5 AND content_id IS NOT NULL) OR
				(content_kind = 6 AND content_id IS NOT NULL) OR
				(content_kind = 7 AND content_id IS NOT NULL) OR
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

func validateGraphQLDeltaFileKind(ctx context.Context, db *sql.DB) error {
	var tableSql string
	err := db.QueryRowContext(ctx, `
		SELECT sql FROM sqlite_master WHERE type='table' AND name='files'
	`).Scan(&tableSql)
	if err != nil {
		return fmt.Errorf("read files table schema: %w", err)
	}
	if !strings.Contains(tableSql, "6, 7)") {
		return fmt.Errorf("files table CHECK constraint does not include content_kind 7 (graphql_delta)")
	}
	return nil
}
