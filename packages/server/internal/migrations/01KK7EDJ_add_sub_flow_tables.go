package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/migrate"
)

const MigrationAddSubFlowTablesID = "01KK7EDJV31GSEJFTN11H5WGFH"

const MigrationAddSubFlowTablesChecksum = "sha256:add-sub-flow-tables-v1"

func init() {
	if err := migrate.Register(migrate.Migration{
		ID:             MigrationAddSubFlowTablesID,
		Checksum:       MigrationAddSubFlowTablesChecksum,
		Description:    "Add sub-flow node tables (trigger, return, run)",
		Apply:          applySubFlowTables,
		Validate:       validateSubFlowTables,
		RequiresBackup: false,
	}); err != nil {
		panic("failed to register sub-flow tables migration: " + err.Error())
	}
}

func applySubFlowTables(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS flow_node_sub_flow_trigger (
			flow_node_id BLOB NOT NULL PRIMARY KEY,
			params BLOB NOT NULL DEFAULT '[]'
		)
	`); err != nil {
		return fmt.Errorf("create flow_node_sub_flow_trigger table: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS flow_node_sub_flow_return (
			flow_node_id BLOB NOT NULL PRIMARY KEY,
			outputs BLOB NOT NULL DEFAULT '[]'
		)
	`); err != nil {
		return fmt.Errorf("create flow_node_sub_flow_return table: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS flow_node_run_sub_flow (
			flow_node_id BLOB NOT NULL PRIMARY KEY,
			target_flow_id BLOB,
			target_flow_name TEXT NOT NULL DEFAULT '',
			inputs BLOB NOT NULL DEFAULT '[]',
			FOREIGN KEY (target_flow_id) REFERENCES flow (id) ON DELETE SET NULL
		)
	`); err != nil {
		return fmt.Errorf("create flow_node_run_sub_flow table: %w", err)
	}

	return nil
}

func validateSubFlowTables(ctx context.Context, db *sql.DB) error {
	tables := []string{
		"flow_node_sub_flow_trigger",
		"flow_node_sub_flow_return",
		"flow_node_run_sub_flow",
	}
	for _, table := range tables {
		var name string
		err := db.QueryRowContext(ctx, `
			SELECT name FROM sqlite_master
			WHERE type='table' AND name=?
		`, table).Scan(&name)
		if err != nil {
			return fmt.Errorf("%s table not found: %w", table, err)
		}
	}
	return nil
}
