package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/migrate"
)

// MigrationAddGraphQLDeltaID is the ULID for the GraphQL delta system migration.
const MigrationAddGraphQLDeltaID = "01KHEX5HB7REY2NXDPCYFS6S02"

// MigrationAddGraphQLDeltaChecksum is a stable hash of this migration.
const MigrationAddGraphQLDeltaChecksum = "sha256:add-graphql-delta-v1"

func init() {
	if err := migrate.Register(migrate.Migration{
		ID:             MigrationAddGraphQLDeltaID,
		Checksum:       MigrationAddGraphQLDeltaChecksum,
		Description:    "Add delta/variant support to GraphQL tables for flow node overrides",
		Apply:          applyGraphQLDelta,
		Validate:       validateGraphQLDelta,
		RequiresBackup: true,
	}); err != nil {
		panic("failed to register GraphQL delta migration: " + err.Error())
	}
}

// applyGraphQLDelta adds delta system fields to GraphQL tables.
func applyGraphQLDelta(ctx context.Context, tx *sql.Tx) error {
	// 1. Add delta system fields to graphql table
	graphqlColumns := []struct {
		name    string
		sqlType string
	}{
		{"parent_graphql_id", "BLOB DEFAULT NULL"},
		{"is_delta", "BOOLEAN NOT NULL DEFAULT FALSE"},
		{"is_snapshot", "BOOLEAN NOT NULL DEFAULT FALSE"},
		{"delta_name", "TEXT NULL"},
		{"delta_url", "TEXT NULL"},
		{"delta_query", "TEXT NULL"},
		{"delta_variables", "TEXT NULL"},
		{"delta_description", "TEXT NULL"},
	}

	for _, col := range graphqlColumns {
		if err := addColumnIfNotExists(ctx, tx, "graphql", col.name, col.sqlType); err != nil {
			return fmt.Errorf("add graphql.%s: %w", col.name, err)
		}
	}

	// 2. Add indexes for graphql delta resolution and performance
	graphqlIndexes := []string{
		`CREATE INDEX IF NOT EXISTS graphql_parent_delta_idx ON graphql (parent_graphql_id, is_delta)`,
		`CREATE INDEX IF NOT EXISTS graphql_delta_resolution_idx ON graphql (parent_graphql_id, is_delta, updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS graphql_active_streaming_idx ON graphql (workspace_id, updated_at DESC) WHERE is_delta = FALSE`,
	}

	for _, idx := range graphqlIndexes {
		if _, err := tx.ExecContext(ctx, idx); err != nil {
			return fmt.Errorf("create graphql index: %w", err)
		}
	}

	// 3. Add delta system fields to graphql_header table
	headerColumns := []struct {
		name    string
		sqlType string
	}{
		{"parent_graphql_header_id", "BLOB DEFAULT NULL"},
		{"is_delta", "BOOLEAN NOT NULL DEFAULT FALSE"},
		{"delta_header_key", "TEXT NULL"},
		{"delta_header_value", "TEXT NULL"},
		{"delta_description", "TEXT NULL"},
		{"delta_enabled", "BOOLEAN NULL"},
		{"delta_display_order", "REAL NULL"},
	}

	for _, col := range headerColumns {
		if err := addColumnIfNotExists(ctx, tx, "graphql_header", col.name, col.sqlType); err != nil {
			return fmt.Errorf("add graphql_header.%s: %w", col.name, err)
		}
	}

	// 4. Add indexes for graphql_header delta support
	headerIndexes := []string{
		`CREATE INDEX IF NOT EXISTS graphql_header_parent_delta_idx ON graphql_header (parent_graphql_header_id, is_delta)`,
		`CREATE INDEX IF NOT EXISTS graphql_header_delta_streaming_idx ON graphql_header (parent_graphql_header_id, is_delta, updated_at DESC)`,
	}

	for _, idx := range headerIndexes {
		if _, err := tx.ExecContext(ctx, idx); err != nil {
			return fmt.Errorf("create graphql_header index: %w", err)
		}
	}

	// 5. Add delta system fields to graphql_assert table
	assertColumns := []struct {
		name    string
		sqlType string
	}{
		{"parent_graphql_assert_id", "BLOB DEFAULT NULL"},
		{"is_delta", "BOOLEAN NOT NULL DEFAULT FALSE"},
		{"delta_value", "TEXT NULL"},
		{"delta_enabled", "BOOLEAN NULL"},
		{"delta_description", "TEXT NULL"},
		{"delta_display_order", "REAL NULL"},
	}

	for _, col := range assertColumns {
		if err := addColumnIfNotExists(ctx, tx, "graphql_assert", col.name, col.sqlType); err != nil {
			return fmt.Errorf("add graphql_assert.%s: %w", col.name, err)
		}
	}

	// 6. Add indexes for graphql_assert delta support
	assertIndexes := []string{
		`CREATE INDEX IF NOT EXISTS graphql_assert_parent_delta_idx ON graphql_assert (parent_graphql_assert_id, is_delta)`,
		`CREATE INDEX IF NOT EXISTS graphql_assert_delta_streaming_idx ON graphql_assert (parent_graphql_assert_id, is_delta, updated_at DESC)`,
	}

	for _, idx := range assertIndexes {
		if _, err := tx.ExecContext(ctx, idx); err != nil {
			return fmt.Errorf("create graphql_assert index: %w", err)
		}
	}

	return nil
}

// addColumnIfNotExists adds a column to a table if it doesn't already exist.
func addColumnIfNotExists(ctx context.Context, tx *sql.Tx, table, column, sqlType string) error {
	var colCount int
	err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM pragma_table_info(?)
		WHERE name = ?
	`, table, column).Scan(&colCount)
	if err != nil {
		return fmt.Errorf("check column existence: %w", err)
	}
	if colCount > 0 {
		return nil // Column already exists
	}

	query := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, sqlType)
	if _, err := tx.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("alter table: %w", err)
	}
	return nil
}

// validateGraphQLDelta verifies all delta columns and indexes were created successfully.
func validateGraphQLDelta(ctx context.Context, db *sql.DB) error {
	// Verify graphql table columns
	graphqlColumns := []string{
		"parent_graphql_id",
		"is_delta",
		"is_snapshot",
		"delta_name",
		"delta_url",
		"delta_query",
		"delta_variables",
		"delta_description",
	}

	for _, col := range graphqlColumns {
		if err := verifyColumnExists(ctx, db, "graphql", col); err != nil {
			return err
		}
	}

	// Verify graphql_header table columns
	headerColumns := []string{
		"parent_graphql_header_id",
		"is_delta",
		"delta_header_key",
		"delta_header_value",
		"delta_description",
		"delta_enabled",
		"delta_display_order",
	}

	for _, col := range headerColumns {
		if err := verifyColumnExists(ctx, db, "graphql_header", col); err != nil {
			return err
		}
	}

	// Verify graphql_assert table columns
	assertColumns := []string{
		"parent_graphql_assert_id",
		"is_delta",
		"delta_value",
		"delta_enabled",
		"delta_description",
		"delta_display_order",
	}

	for _, col := range assertColumns {
		if err := verifyColumnExists(ctx, db, "graphql_assert", col); err != nil {
			return err
		}
	}

	// Verify indexes
	indexes := []string{
		"graphql_parent_delta_idx",
		"graphql_delta_resolution_idx",
		"graphql_active_streaming_idx",
		"graphql_header_parent_delta_idx",
		"graphql_header_delta_streaming_idx",
		"graphql_assert_parent_delta_idx",
		"graphql_assert_delta_streaming_idx",
	}

	for _, idx := range indexes {
		var name string
		err := db.QueryRowContext(ctx, `
			SELECT name FROM sqlite_master
			WHERE type='index' AND name=?
		`, idx).Scan(&name)
		if err != nil {
			return fmt.Errorf("index %s not found: %w", idx, err)
		}
	}

	return nil
}

// verifyColumnExists checks if a column exists in a table.
func verifyColumnExists(ctx context.Context, db *sql.DB, table, column string) error {
	var colCount int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM pragma_table_info(?)
		WHERE name = ?
	`, table, column).Scan(&colCount)
	if err != nil {
		return fmt.Errorf("check %s.%s: %w", table, column, err)
	}
	if colCount == 0 {
		return fmt.Errorf("column %s.%s not found", table, column)
	}
	return nil
}
