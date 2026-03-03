package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/migrate"
)

const MigrationAddFlowErrorID = "01KJSYH5EHQ1MEG2R4H3N3WK71"

const MigrationAddFlowErrorChecksum = "sha256:add-flow-error-and-node-id-mapping-v1"

func init() {
	if err := migrate.Register(migrate.Migration{
		ID:             MigrationAddFlowErrorID,
		Checksum:       MigrationAddFlowErrorChecksum,
		Description:    "Add error and node_id_mapping columns to flow table",
		Apply:          applyFlowError,
		Validate:       validateFlowError,
		RequiresBackup: false,
	}); err != nil {
		panic("failed to register flow error migration: " + err.Error())
	}
}

func applyFlowError(ctx context.Context, tx *sql.Tx) error {
	columns := []struct {
		name string
		ddl  string
	}{
		{"error", `ALTER TABLE flow ADD COLUMN error TEXT DEFAULT NULL`},
		{"node_id_mapping", `ALTER TABLE flow ADD COLUMN node_id_mapping BLOB DEFAULT NULL`},
	}

	for _, col := range columns {
		var count int
		err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM pragma_table_info('flow')
			WHERE name = ?
		`, col.name).Scan(&count)
		if err != nil {
			return fmt.Errorf("check %s column: %w", col.name, err)
		}
		if count == 0 {
			if _, err := tx.ExecContext(ctx, col.ddl); err != nil {
				return fmt.Errorf("add %s column: %w", col.name, err)
			}
		}
	}
	return nil
}

func validateFlowError(ctx context.Context, db *sql.DB) error {
	for _, col := range []string{"error", "node_id_mapping"} {
		var count int
		err := db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM pragma_table_info('flow')
			WHERE name = ?
		`, col).Scan(&count)
		if err != nil {
			return fmt.Errorf("validate %s column: %w", col, err)
		}
		if count == 0 {
			return fmt.Errorf("%s column not found on flow table", col)
		}
	}
	return nil
}
