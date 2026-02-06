package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/migrate"
)

// MigrationAddExternalIDID is the ULID for the external_id migration.
const MigrationAddExternalIDID = "01KGTFDMC0A8NKA2ER2K6YFQCY"

// MigrationAddExternalIDChecksum is a stable hash of this migration.
const MigrationAddExternalIDChecksum = "sha256:add-external-id-v1"

func init() {
	if err := migrate.Register(migrate.Migration{
		ID:          MigrationAddExternalIDID,
		Checksum:    MigrationAddExternalIDChecksum,
		Description: "Add external_id column to users table for BetterAuth user mapping",
		Apply:       applyAddExternalID,
		Validate:    validateAddExternalID,
	}); err != nil {
		panic("failed to register external_id migration: " + err.Error())
	}
}

// applyAddExternalID adds the external_id column to the users table.
// This column maps BetterAuth user IDs to internal ULID-based user IDs.
func applyAddExternalID(ctx context.Context, tx *sql.Tx) error {
	// Check if column already exists (schema may already include it)
	var count int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM pragma_table_info('users')
		WHERE name = 'external_id'
	`).Scan(&count); err != nil {
		return fmt.Errorf("check external_id column: %w", err)
	}
	if count > 0 {
		return nil // Column already exists
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE users ADD COLUMN external_id TEXT UNIQUE`); err != nil {
		return err
	}
	return nil
}

// validateAddExternalID verifies the external_id column was added successfully.
func validateAddExternalID(ctx context.Context, db *sql.DB) error {
	var count int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM pragma_table_info('users')
		WHERE name = 'external_id'
	`).Scan(&count)
	if err != nil {
		return fmt.Errorf("check external_id column: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("external_id column not found on users table")
	}
	return nil
}
