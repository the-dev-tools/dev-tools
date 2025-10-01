package migrate

import (
	"context"
	"database/sql"
	"fmt"
)

// RunCheckpoint performs PRAGMA wal_checkpoint(TRUNCATE).
func RunCheckpoint(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("migrate: db handle nil for checkpoint")
	}
	if _, err := db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return fmt.Errorf("migrate: wal checkpoint: %w", err)
	}
	return nil
}
