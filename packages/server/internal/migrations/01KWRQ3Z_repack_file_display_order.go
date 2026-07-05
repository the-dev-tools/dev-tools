package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/migrate"
)

// MigrationRepackFileDisplayOrderID is the ULID for the file display_order repack migration.
const MigrationRepackFileDisplayOrderID = "01KWRQ3ZC05MK9VPB0ZMQZCZ4G"

// MigrationRepackFileDisplayOrderChecksum is a stable hash of this migration.
const MigrationRepackFileDisplayOrderChecksum = "sha256:repack-file-display-order-v1"

func init() {
	if err := migrate.Register(migrate.Migration{
		ID:             MigrationRepackFileDisplayOrderID,
		Checksum:       MigrationRepackFileDisplayOrderChecksum,
		Description:    "Repack files.display_order to sequential values, fixing float32 MAX overflow",
		Apply:          applyRepackFileDisplayOrder,
		Validate:       validateRepackFileDisplayOrder,
		RequiresBackup: true, // Rewrites data in-place across the whole files table
	}); err != nil {
		panic("failed to register file display_order repack migration: " + err.Error())
	}
}

// The client used to generate append orders as the midpoint between the last
// order and float32 MAX, converging to float32 MAX after a few dozen inserts.
// Rows pegged at MAX overflow the float32 wire type, sort inconsistently, and
// hide every file in the workspace (issue #44). Rewrite each sibling group
// (workspace_id, parent_id) to sequential integers starting at 0, preserving
// the current relative order with id (creation time) as the tie-breaker.
func applyRepackFileDisplayOrder(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `
		UPDATE files SET display_order = (
			SELECT rn - 1 FROM (
				SELECT id, ROW_NUMBER() OVER (
					PARTITION BY workspace_id, parent_id
					ORDER BY display_order, id
				) AS rn
				FROM files
			) ranked
			WHERE ranked.id = files.id
		)
	`); err != nil {
		return fmt.Errorf("repack files.display_order: %w", err)
	}
	return nil
}

func validateRepackFileDisplayOrder(ctx context.Context, db *sql.DB) error {
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM files WHERE display_order >= 1e30 OR display_order != display_order`).Scan(&count)
	if err != nil {
		return fmt.Errorf("validate files.display_order: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("%d files still have pathological display_order values", count)
	}
	return nil
}
