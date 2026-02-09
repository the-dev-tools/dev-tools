package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/migrate"
)

// MigrationAddHttpIsSnapshotID is the ULID for the http is_snapshot column migration.
const MigrationAddHttpIsSnapshotID = "01KGZC9EQCWJN09C2SJQBWRZQM"

// MigrationAddHttpIsSnapshotChecksum is a stable hash of this migration.
const MigrationAddHttpIsSnapshotChecksum = "sha256:add-http-is-snapshot-v2"

func init() {
	if err := migrate.Register(migrate.Migration{
		ID:             MigrationAddHttpIsSnapshotID,
		Checksum:       MigrationAddHttpIsSnapshotChecksum,
		Description:    "Add is_snapshot column to http table for version snapshots",
		Apply:          applyHttpIsSnapshot,
		Validate:       validateHttpIsSnapshot,
		RequiresBackup: false, // Simple ALTER TABLE ADD COLUMN with default, no backup needed
	}); err != nil {
		panic("failed to register http is_snapshot migration: " + err.Error())
	}
}

func applyHttpIsSnapshot(ctx context.Context, tx *sql.Tx) error {
	// Check if column already exists (idempotent)
	var count int
	err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM pragma_table_info('http')
		WHERE name = 'is_snapshot'
	`).Scan(&count)
	if err != nil {
		return fmt.Errorf("check is_snapshot column: %w", err)
	}
	if count == 0 {
		// SQLite supports ALTER TABLE ADD COLUMN with a default value
		if _, err := tx.ExecContext(ctx, `
			ALTER TABLE http ADD COLUMN is_snapshot BOOLEAN NOT NULL DEFAULT FALSE
		`); err != nil {
			return fmt.Errorf("add is_snapshot column: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_http_workspace_snapshot
		ON http(workspace_id, is_snapshot)
	`); err != nil {
		return fmt.Errorf("create is_snapshot index: %w", err)
	}

	return nil
}

func validateHttpIsSnapshot(ctx context.Context, db *sql.DB) error {
	var count int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM pragma_table_info('http')
		WHERE name = 'is_snapshot'
	`).Scan(&count)
	if err != nil {
		return fmt.Errorf("validate is_snapshot column: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("is_snapshot column not found on http table")
	}

	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sqlite_master
		WHERE type='index' AND name='idx_http_workspace_snapshot'
	`).Scan(&count)
	if err != nil {
		return fmt.Errorf("validate is_snapshot index: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("idx_http_workspace_snapshot index not found on http table")
	}
	return nil
}
