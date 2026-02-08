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
const MigrationAddWorkspaceSyncChecksum = "sha256:add-workspace-sync-v2"

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

// columnExists checks if a column exists on a table using pragma_table_info.
func columnExists(ctx context.Context, tx *sql.Tx, table, column string) (bool, error) {
	var count int
	err := tx.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name = '%s'`, table, column),
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// applyWorkspaceSync adds folder sync columns to the workspaces table.
// Idempotent: skips columns that already exist (e.g. from fresh schema).
func applyWorkspaceSync(ctx context.Context, tx *sql.Tx) error {
	columns := []struct {
		name string
		ddl  string
	}{
		{"sync_path", `ALTER TABLE workspaces ADD COLUMN sync_path TEXT`},
		{"sync_format", `ALTER TABLE workspaces ADD COLUMN sync_format TEXT`},
		{"sync_enabled", `ALTER TABLE workspaces ADD COLUMN sync_enabled BOOLEAN NOT NULL DEFAULT 0`},
	}

	for _, col := range columns {
		exists, err := columnExists(ctx, tx, "workspaces", col.name)
		if err != nil {
			return fmt.Errorf("check %s column: %w", col.name, err)
		}
		if exists {
			continue
		}
		if _, err := tx.ExecContext(ctx, col.ddl); err != nil {
			return fmt.Errorf("add %s column: %w", col.name, err)
		}
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
