package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/migrate"
)

const MigrationAddWorkspaceOrgIDID = "01KHY2ESCS4SAB6R749SPQ02T2"

const MigrationAddWorkspaceOrgIDChecksum = "sha256:add-workspace-org-id-v1"

func init() {
	if err := migrate.Register(migrate.Migration{
		ID:          MigrationAddWorkspaceOrgIDID,
		Checksum:    MigrationAddWorkspaceOrgIDChecksum,
		Description: "Add organization_id column to workspaces table",
		Apply:       applyAddWorkspaceOrgID,
		Validate:    validateAddWorkspaceOrgID,
	}); err != nil {
		panic("failed to register workspace org id migration: " + err.Error())
	}
}

func applyAddWorkspaceOrgID(ctx context.Context, tx *sql.Tx) error {
	var count int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM pragma_table_info('workspaces')
		WHERE name = 'organization_id'
	`).Scan(&count); err != nil {
		return fmt.Errorf("check organization_id column: %w", err)
	}
	if count > 0 {
		return nil
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE workspaces ADD COLUMN organization_id BLOB`); err != nil {
		return fmt.Errorf("add organization_id column: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS workspaces_org_idx ON workspaces (organization_id)`); err != nil {
		return fmt.Errorf("create workspaces_org_idx index: %w", err)
	}

	return nil
}

func validateAddWorkspaceOrgID(ctx context.Context, db *sql.DB) error {
	var count int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM pragma_table_info('workspaces')
		WHERE name = 'organization_id'
	`).Scan(&count); err != nil {
		return fmt.Errorf("check organization_id column: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("organization_id column not found on workspaces table")
	}
	return nil
}
