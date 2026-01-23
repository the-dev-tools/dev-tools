package migrations

import (
	"context"
	"database/sql"
	"strings"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/migrate"
)

// MigrationFixAINodeFKID is the ULID for removing FK constraints from AI node tables.
const MigrationFixAINodeFKID = "01KFF095A1B2C3D4E5F6G7H8J9"

// MigrationFixAINodeFKChecksum is a stable hash of this migration.
const MigrationFixAINodeFKChecksum = "sha256:fix-ai-node-fk-v1"

func init() {
	if err := migrate.Register(migrate.Migration{
		ID:             MigrationFixAINodeFKID,
		Checksum:       MigrationFixAINodeFKChecksum,
		Description:    "Remove FK constraints from flow_node_ai and flow_node_ai_provider tables",
		Apply:          applyFixAINodeFK,
		Validate:       validateFixAINodeFK,
		RequiresBackup: true, // This migration recreates tables
	}); err != nil {
		panic("failed to register AI node FK fix migration: " + err.Error())
	}
}

// applyFixAINodeFK removes FK constraints from AI node tables to match
// the pattern of other node type tables (flow_node_for, flow_node_js, etc.)
func applyFixAINodeFK(ctx context.Context, tx *sql.Tx) error {
	// Fix flow_node_ai table
	if err := fixFlowNodeAI(ctx, tx); err != nil {
		return err
	}

	// Fix flow_node_ai_provider table
	if err := fixFlowNodeAIProvider(ctx, tx); err != nil {
		return err
	}

	return nil
}

func fixFlowNodeAI(ctx context.Context, tx *sql.Tx) error {
	// Check if table has FK constraint
	var tableDef string
	err := tx.QueryRowContext(ctx, `
		SELECT sql FROM sqlite_master
		WHERE type='table' AND name='flow_node_ai'
	`).Scan(&tableDef)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil // Table doesn't exist
		}
		return err
	}

	// If no FOREIGN KEY, skip
	if !strings.Contains(tableDef, "FOREIGN KEY") {
		return nil
	}

	// Recreate table without FK
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE flow_node_ai_new (
			flow_node_id BLOB NOT NULL PRIMARY KEY,
			prompt TEXT NOT NULL,
			max_iterations INT NOT NULL DEFAULT 5
		)
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO flow_node_ai_new (flow_node_id, prompt, max_iterations)
		SELECT flow_node_id, prompt, max_iterations FROM flow_node_ai
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, "DROP TABLE flow_node_ai"); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, "ALTER TABLE flow_node_ai_new RENAME TO flow_node_ai"); err != nil {
		return err
	}

	return nil
}

func fixFlowNodeAIProvider(ctx context.Context, tx *sql.Tx) error {
	// Check if table has FK constraint
	var tableDef string
	err := tx.QueryRowContext(ctx, `
		SELECT sql FROM sqlite_master
		WHERE type='table' AND name='flow_node_ai_provider'
	`).Scan(&tableDef)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil // Table doesn't exist
		}
		return err
	}

	// If no FOREIGN KEY, skip
	if !strings.Contains(tableDef, "FOREIGN KEY") {
		return nil
	}

	// Recreate table without FK
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE flow_node_ai_provider_new (
			flow_node_id BLOB NOT NULL PRIMARY KEY,
			credential_id BLOB,
			model INT8 NOT NULL,
			temperature REAL,
			max_tokens INT
		)
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO flow_node_ai_provider_new (flow_node_id, credential_id, model, temperature, max_tokens)
		SELECT flow_node_id, credential_id, model, temperature, max_tokens FROM flow_node_ai_provider
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, "DROP TABLE flow_node_ai_provider"); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, "ALTER TABLE flow_node_ai_provider_new RENAME TO flow_node_ai_provider"); err != nil {
		return err
	}

	return nil
}

// validateFixAINodeFK verifies the tables exist and can be queried.
func validateFixAINodeFK(ctx context.Context, db *sql.DB) error {
	tables := []string{"flow_node_ai", "flow_node_ai_provider"}
	for _, table := range tables {
		var count int
		err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count)
		if err != nil && err != sql.ErrNoRows {
			// Table might not exist yet, which is fine
			var name string
			checkErr := db.QueryRowContext(ctx, `
				SELECT name FROM sqlite_master WHERE type='table' AND name=?
			`, table).Scan(&name)
			if checkErr == sql.ErrNoRows {
				continue // Table doesn't exist, that's okay
			}
			return err
		}
	}
	return nil
}
