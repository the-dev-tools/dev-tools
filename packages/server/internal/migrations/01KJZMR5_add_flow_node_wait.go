package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/migrate"
)

const MigrationAddFlowNodeWaitID = "01KJZMR5232X0983HYB7GS2WZ7"

const MigrationAddFlowNodeWaitChecksum = "sha256:add-flow-node-wait-v1"

func init() {
	if err := migrate.Register(migrate.Migration{
		ID:             MigrationAddFlowNodeWaitID,
		Checksum:       MigrationAddFlowNodeWaitChecksum,
		Description:    "Add flow_node_wait table for wait/delay nodes",
		Apply:          applyFlowNodeWait,
		Validate:       validateFlowNodeWait,
		RequiresBackup: false,
	}); err != nil {
		panic("failed to register flow_node_wait migration: " + err.Error())
	}
}

func applyFlowNodeWait(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS flow_node_wait (
			flow_node_id BLOB NOT NULL PRIMARY KEY,
			duration_ms BIGINT NOT NULL
		)
	`); err != nil {
		return fmt.Errorf("create flow_node_wait table: %w", err)
	}
	return nil
}

func validateFlowNodeWait(ctx context.Context, db *sql.DB) error {
	var name string
	err := db.QueryRowContext(ctx, `
		SELECT name FROM sqlite_master
		WHERE type='table' AND name='flow_node_wait'
	`).Scan(&name)
	if err != nil {
		return fmt.Errorf("flow_node_wait table not found: %w", err)
	}
	return nil
}
