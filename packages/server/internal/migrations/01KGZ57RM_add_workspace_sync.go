package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/migrate"
)

// MigrationAddWorkspaceSyncID is the ULID for the workspace sync migration.
const MigrationAddWorkspaceSyncID = "01KGZ57RM25ANJQA21JQGJ6D2M"

// MigrationAddWorkspaceSyncChecksum is a stable hash of this migration.
const MigrationAddWorkspaceSyncChecksum = "sha256:add-workspace-sync-v1"

func init() {
	if err := migrate.Register(migrate.Migration{
		ID:             MigrationAddWorkspaceSyncID,
		Checksum:       MigrationAddWorkspaceSyncChecksum,
		Description:    "Add sync_path, sync_format, sync_enabled columns to workspaces table",
		Apply:          applyWorkspaceSync,
		Validate:       validateWorkspaceSync,
		RequiresBackup: false,
	}); err != nil {
		panic("failed to register workspace sync migration: " + err.Error())
	}
}

// applyWorkspaceSync adds folder sync columns to the workspaces table.
func applyWorkspaceSync(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `ALTER TABLE workspaces ADD COLUMN sync_path TEXT`); err != nil {
		return fmt.Errorf("add sync_path column: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE workspaces ADD COLUMN sync_format TEXT`); err != nil {
		return fmt.Errorf("add sync_format column: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE workspaces ADD COLUMN sync_enabled BOOLEAN NOT NULL DEFAULT 0`); err != nil {
		return fmt.Errorf("add sync_enabled column: %w", err)
	}

	return nil
}

// validateWorkspaceSync verifies that sync columns exist on the workspaces table.
func validateWorkspaceSync(ctx context.Context, db *sql.DB) error {
	columns := []string{"sync_path", "sync_format", "sync_enabled"}
	for _, col := range columns {
		var cid int
		var name, ctype string
		var notnull int
		var dfltValue *string
		var pk int
		err := db.QueryRowContext(ctx,
			fmt.Sprintf(`SELECT cid, name, type, "notnull", dflt_value, pk FROM pragma_table_info('workspaces') WHERE name = '%s'`, col),
		).Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk)
		if err != nil {
			return fmt.Errorf("column %s not found in workspaces table: %w", col, err)
		}
	}
	return nil
}
