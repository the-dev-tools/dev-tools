package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/migrate"
)

// MigrationEnforceDeltaSnapshotExclusivityID is the ULID for the delta/snapshot mutual exclusivity migration.
const MigrationEnforceDeltaSnapshotExclusivityID = "01KH1AZ5P9V9SD3XTXM9W1RNK7"

// MigrationEnforceDeltaSnapshotExclusivityChecksum is a stable hash of this migration.
const MigrationEnforceDeltaSnapshotExclusivityChecksum = "sha256:enforce-delta-snapshot-exclusivity-v1"

func init() {
	if err := migrate.Register(migrate.Migration{
		ID:             MigrationEnforceDeltaSnapshotExclusivityID,
		Checksum:       MigrationEnforceDeltaSnapshotExclusivityChecksum,
		Description:    "Enforce mutual exclusivity between is_delta and is_snapshot via triggers",
		Apply:          applyDeltaSnapshotExclusivity,
		Validate:       validateDeltaSnapshotExclusivity,
		RequiresBackup: false,
	}); err != nil {
		panic("failed to register delta/snapshot exclusivity migration: " + err.Error())
	}
}

func applyDeltaSnapshotExclusivity(ctx context.Context, tx *sql.Tx) error {
	// Verify no existing rows violate the constraint before adding triggers.
	var violationCount int
	err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM http
		WHERE is_delta = TRUE AND is_snapshot = TRUE
	`).Scan(&violationCount)
	if err != nil {
		return fmt.Errorf("check delta/snapshot violations: %w", err)
	}
	if violationCount > 0 {
		return fmt.Errorf("found %d http rows with both is_delta=TRUE and is_snapshot=TRUE", violationCount)
	}

	// SQLite cannot add CHECK constraints to existing tables, so we use
	// BEFORE INSERT/UPDATE triggers to enforce mutual exclusivity at runtime.
	// The CHECK constraint in the DDL schema (04_http.sql) covers fresh databases.
	if _, err := tx.ExecContext(ctx, `
		CREATE TRIGGER IF NOT EXISTS trg_http_delta_snapshot_insert
		BEFORE INSERT ON http
		FOR EACH ROW
		WHEN NEW.is_delta = TRUE AND NEW.is_snapshot = TRUE
		BEGIN
			SELECT RAISE(ABORT, 'http record cannot be both a delta and a snapshot');
		END
	`); err != nil {
		return fmt.Errorf("create insert trigger: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		CREATE TRIGGER IF NOT EXISTS trg_http_delta_snapshot_update
		BEFORE UPDATE ON http
		FOR EACH ROW
		WHEN NEW.is_delta = TRUE AND NEW.is_snapshot = TRUE
		BEGIN
			SELECT RAISE(ABORT, 'http record cannot be both a delta and a snapshot');
		END
	`); err != nil {
		return fmt.Errorf("create update trigger: %w", err)
	}

	return nil
}

func validateDeltaSnapshotExclusivity(ctx context.Context, db *sql.DB) error {
	// Verify both triggers exist.
	for _, name := range []string{
		"trg_http_delta_snapshot_insert",
		"trg_http_delta_snapshot_update",
	} {
		var count int
		err := db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM sqlite_master
			WHERE type='trigger' AND name=?
		`, name).Scan(&count)
		if err != nil {
			return fmt.Errorf("validate trigger %s: %w", name, err)
		}
		if count == 0 {
			return fmt.Errorf("trigger %s not found", name)
		}
	}

	// Verify no rows violate the invariant.
	var violationCount int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM http
		WHERE is_delta = TRUE AND is_snapshot = TRUE
	`).Scan(&violationCount)
	if err != nil {
		return fmt.Errorf("validate delta/snapshot exclusivity: %w", err)
	}
	if violationCount > 0 {
		return fmt.Errorf("found %d rows violating delta/snapshot exclusivity", violationCount)
	}

	return nil
}
