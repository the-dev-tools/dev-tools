package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/migrate"
)

// MigrationAddAIChecksTablesID is the ULID for the AI checks storage migration.
const MigrationAddAIChecksTablesID = "01M4JSNG9KCGZ5WKVGHVFXX919"

// MigrationAddAIChecksTablesChecksum is a stable hash of this migration.
const MigrationAddAIChecksTablesChecksum = "sha256:add-ai-checks-tables-v1"

// aiChecksTables are the tables that keep a workflow's stream: and expect: settings, so a
// workspace imported from YAML or HAR exports them again.
var aiChecksTables = []struct{ name, ddl string }{
	{"http_stream", `
		CREATE TABLE IF NOT EXISTS http_stream (
			http_id BLOB NOT NULL PRIMARY KEY,
			preset TEXT NOT NULL,
			timeout_ms BIGINT NOT NULL DEFAULT 0,
			FOREIGN KEY (http_id) REFERENCES http (id) ON DELETE CASCADE
		)`},
	{"flow_node_expect", `
		CREATE TABLE IF NOT EXISTS flow_node_expect (
			flow_node_id BLOB NOT NULL PRIMARY KEY,
			expect TEXT NOT NULL,
			FOREIGN KEY (flow_node_id) REFERENCES flow_node (id) ON DELETE CASCADE
		)`},
	{"flow_ai_checks", `
		CREATE TABLE IF NOT EXISTS flow_ai_checks (
			flow_id BLOB NOT NULL PRIMARY KEY,
			settings TEXT NOT NULL,
			FOREIGN KEY (flow_id) REFERENCES flow (id) ON DELETE CASCADE
		)`},
}

func init() {
	if err := migrate.Register(migrate.Migration{
		ID:             MigrationAddAIChecksTablesID,
		Checksum:       MigrationAddAIChecksTablesChecksum,
		Description:    "Add http_stream, flow_node_expect and flow_ai_checks tables for stream: and expect:",
		Apply:          applyAIChecksTables,
		Validate:       validateAIChecksTables,
		RequiresBackup: false,
	}); err != nil {
		panic("failed to register AI checks tables migration: " + err.Error())
	}
}

func applyAIChecksTables(ctx context.Context, tx *sql.Tx) error {
	for _, t := range aiChecksTables {
		if _, err := tx.ExecContext(ctx, t.ddl); err != nil {
			return fmt.Errorf("create %s table: %w", t.name, err)
		}
	}
	return nil
}

func validateAIChecksTables(ctx context.Context, db *sql.DB) error {
	for _, t := range aiChecksTables {
		var name string
		if err := db.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name=?`, t.name).Scan(&name); err != nil {
			return fmt.Errorf("%s table not found: %w", t.name, err)
		}
	}
	return nil
}
