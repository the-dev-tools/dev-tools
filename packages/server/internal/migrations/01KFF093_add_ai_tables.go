package migrations

import (
	"context"
	"database/sql"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/migrate"
)

// MigrationAddAITablesID is the ULID for the AI tables migration.
const MigrationAddAITablesID = "01KFF093T2EFF5NQH0GKHGP1QN"

// MigrationAddAITablesChecksum is a stable hash of this migration.
const MigrationAddAITablesChecksum = "sha256:add-ai-tables-v1"

func init() {
	if err := migrate.Register(migrate.Migration{
		ID:          MigrationAddAITablesID,
		Checksum:    MigrationAddAITablesChecksum,
		Description: "Add credential and AI node tables for AI agent feature",
		Apply:       applyAITables,
		Validate:    validateAITables,
	}); err != nil {
		panic("failed to register AI tables migration: " + err.Error())
	}
}

// applyAITables creates all the AI-related tables:
// - credential (base credential storage)
// - credential_openai, credential_gemini, credential_anthropic (provider-specific)
// - flow_node_ai (AI agent node)
// - flow_node_ai_provider (LLM provider configuration node)
// - flow_node_memory (conversation memory node)
func applyAITables(ctx context.Context, tx *sql.Tx) error {
	// Create credential table
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS credential (
			id BLOB NOT NULL PRIMARY KEY,
			workspace_id BLOB NOT NULL,
			name TEXT NOT NULL,
			kind INT8 NOT NULL, -- 0 = OpenAI, 1 = Gemini, 2 = Anthropic
			FOREIGN KEY (workspace_id) REFERENCES workspaces (id) ON DELETE CASCADE
		)
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS credential_workspace_idx ON credential (workspace_id)
	`); err != nil {
		return err
	}

	// Create credential_openai table
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS credential_openai (
			credential_id BLOB NOT NULL PRIMARY KEY,
			token BLOB NOT NULL, -- Encrypted or plaintext depending on encryption_type
			base_url TEXT,
			encryption_type INT8 NOT NULL DEFAULT 0, -- 0=None, 1=XChaCha20-Poly1305, 2=AES-256-GCM
			FOREIGN KEY (credential_id) REFERENCES credential (id) ON DELETE CASCADE
		)
	`); err != nil {
		return err
	}

	// Create credential_gemini table
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS credential_gemini (
			credential_id BLOB NOT NULL PRIMARY KEY,
			api_key BLOB NOT NULL, -- Encrypted or plaintext depending on encryption_type
			base_url TEXT,
			encryption_type INT8 NOT NULL DEFAULT 0, -- 0=None, 1=XChaCha20-Poly1305, 2=AES-256-GCM
			FOREIGN KEY (credential_id) REFERENCES credential (id) ON DELETE CASCADE
		)
	`); err != nil {
		return err
	}

	// Create credential_anthropic table
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS credential_anthropic (
			credential_id BLOB NOT NULL PRIMARY KEY,
			api_key BLOB NOT NULL, -- Encrypted or plaintext depending on encryption_type
			base_url TEXT,
			encryption_type INT8 NOT NULL DEFAULT 0, -- 0=None, 1=XChaCha20-Poly1305, 2=AES-256-GCM
			FOREIGN KEY (credential_id) REFERENCES credential (id) ON DELETE CASCADE
		)
	`); err != nil {
		return err
	}

	// Create flow_node_ai table (AI agent node)
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS flow_node_ai (
			flow_node_id BLOB NOT NULL PRIMARY KEY,
			prompt TEXT NOT NULL,
			max_iterations INT NOT NULL DEFAULT 5,
			FOREIGN KEY (flow_node_id) REFERENCES flow_node (id) ON DELETE CASCADE
		)
	`); err != nil {
		return err
	}

	// Create flow_node_ai_provider table (LLM provider configuration node)
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS flow_node_ai_provider (
			flow_node_id BLOB NOT NULL PRIMARY KEY,
			credential_id BLOB NOT NULL,
			model INT8 NOT NULL, -- AiModel enum
			temperature REAL, -- Optional: 0.0-2.0, NULL means use provider default
			max_tokens INT, -- Optional: max output tokens, NULL means use provider default
			FOREIGN KEY (flow_node_id) REFERENCES flow_node (id) ON DELETE CASCADE,
			FOREIGN KEY (credential_id) REFERENCES credential (id) ON DELETE CASCADE
		)
	`); err != nil {
		return err
	}

	// Create flow_node_memory table (conversation memory node)
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS flow_node_memory (
			flow_node_id BLOB NOT NULL PRIMARY KEY,
			memory_type INT8 NOT NULL, -- AiMemoryType enum: 0 = WindowBuffer
			window_size INT NOT NULL -- For WindowBuffer: number of messages to retain
		)
	`); err != nil {
		return err
	}

	return nil
}

// validateAITables verifies all tables were created successfully.
func validateAITables(ctx context.Context, db *sql.DB) error {
	tables := []string{
		"credential",
		"credential_openai",
		"credential_gemini",
		"credential_anthropic",
		"flow_node_ai",
		"flow_node_ai_provider",
		"flow_node_memory",
	}

	for _, table := range tables {
		var name string
		err := db.QueryRowContext(ctx, `
			SELECT name FROM sqlite_master
			WHERE type='table' AND name=?
		`, table).Scan(&name)
		if err != nil {
			return err
		}
	}

	return nil
}
