package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/migrate"
)

// MigrationAddUserAuthColumnsID is the ULID for the user auth columns migration.
const MigrationAddUserAuthColumnsID = "01KGTFDMC0A8NKA2ER2K6YFQCY"

// MigrationAddUserAuthColumnsChecksum is a stable hash of this migration.
const MigrationAddUserAuthColumnsChecksum = "sha256:add-user-auth-columns-v1"

func init() {
	if err := migrate.Register(migrate.Migration{
		ID:          MigrationAddUserAuthColumnsID,
		Checksum:    MigrationAddUserAuthColumnsChecksum,
		Description: "Add external_id, name, and image columns to users table for BetterAuth",
		Apply:       applyAddUserAuthColumns,
		Validate:    validateAddUserAuthColumns,
	}); err != nil {
		panic("failed to register user auth columns migration: " + err.Error())
	}
}

// applyAddUserAuthColumns adds external_id, name, and image columns to the users table.
func applyAddUserAuthColumns(ctx context.Context, tx *sql.Tx) error {
	columns := []struct {
		name string
		ddl  string
	}{
		{"external_id", "ALTER TABLE users ADD COLUMN external_id TEXT"},
		{"name", "ALTER TABLE users ADD COLUMN name TEXT NOT NULL DEFAULT ''"},
		{"image", "ALTER TABLE users ADD COLUMN image TEXT"},
	}

	for _, col := range columns {
		var count int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM pragma_table_info('users')
			WHERE name = ?
		`, col.name).Scan(&count); err != nil {
			return fmt.Errorf("check %s column: %w", col.name, err)
		}
		if count > 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx, col.ddl); err != nil {
			return fmt.Errorf("add %s column: %w", col.name, err)
		}
	}

	// Create unique index on external_id
	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS idx_users_external_id ON users(external_id)`); err != nil {
		return fmt.Errorf("create external_id index: %w", err)
	}

	return nil
}

// validateAddUserAuthColumns verifies that all columns were added successfully.
func validateAddUserAuthColumns(ctx context.Context, db *sql.DB) error {
	for _, col := range []string{"external_id", "name", "image"} {
		var count int
		err := db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM pragma_table_info('users')
			WHERE name = ?
		`, col).Scan(&count)
		if err != nil {
			return fmt.Errorf("check %s column: %w", col, err)
		}
		if count == 0 {
			return fmt.Errorf("%s column not found on users table", col)
		}
	}
	return nil
}
