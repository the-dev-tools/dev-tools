package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/migrate"
)

// MigrationAddAITablesID is the ULID for the AI tables migration.
const MigrationAddAITablesID = "01KFF093T2EFF5NQH0GKHGP1QN"

// MigrationAddAITablesChecksum is a stable hash of this migration.
const MigrationAddAITablesChecksum = "sha256:add-ai-tables-v1"

func init() {
	if err := migrate.Register(migrate.Migration{
		ID:             MigrationAddAITablesID,
		Checksum:       MigrationAddAITablesChecksum,
		Description:    "Add credential and AI node tables, update files table for credential content_kind",
		Apply:          applyAITables,
		Validate:       validateAITables,
		RequiresBackup: true, // Files table recreation requires backup
	}); err != nil {
		panic("failed to register AI tables migration: " + err.Error())
	}
}

// applyAITables creates all the AI-related tables and updates files table:
// - credential (base credential storage)
// - credential_openai, credential_gemini, credential_anthropic (provider-specific)
// - flow_node_ai (AI agent node)
// - flow_node_ai_provider (LLM provider configuration node)
// - flow_node_memory (conversation memory node)
// - updates files table CHECK constraint for content_kind=4 (credential)
func applyAITables(ctx context.Context, tx *sql.Tx) error {
	// 1. Create credential table
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

	// 2. Create credential_openai table
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

	// 3. Create credential_gemini table
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

	// 4. Create credential_anthropic table
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

	// 5. Create flow_node_ai table (AI agent node) - no FK to match other node tables
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS flow_node_ai (
			flow_node_id BLOB NOT NULL PRIMARY KEY,
			prompt TEXT NOT NULL,
			max_iterations INT NOT NULL DEFAULT 5
		)
	`); err != nil {
		return err
	}

	// 6. Create flow_node_ai_provider table (LLM provider configuration node) - no FK
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS flow_node_ai_provider (
			flow_node_id BLOB NOT NULL PRIMARY KEY,
			credential_id BLOB, -- Optional: NULL means no credential set yet
			model INT8 NOT NULL, -- AiModel enum
			temperature REAL, -- Optional: 0.0-2.0, NULL means use provider default
			max_tokens INT -- Optional: max output tokens, NULL means use provider default
		)
	`); err != nil {
		return err
	}

	// 7. Create flow_node_memory table (conversation memory node)
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS flow_node_memory (
			flow_node_id BLOB NOT NULL PRIMARY KEY,
			memory_type INT8 NOT NULL, -- AiMemoryType enum: 0 = WindowBuffer
			window_size INT NOT NULL -- For WindowBuffer: number of messages to retain
		)
	`); err != nil {
		return err
	}

	// 8. Update files table CHECK constraint for content_kind=4 (credential)
	if err := updateFilesTableConstraint(ctx, tx); err != nil {
		return err
	}

	return nil
}

// updateFilesTableConstraint recreates the files table to add content_kind=4 support.
// SQLite doesn't support ALTER TABLE to modify CHECK constraints.
func updateFilesTableConstraint(ctx context.Context, tx *sql.Tx) error {
	// Check if the table already supports content_kind=4
	// Use flexible pattern to handle SQLite formatting variations (whitespace)
	var count int
	err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sqlite_master
		WHERE type='table' AND name='files'
		AND sql LIKE '%content_kind%IN%(%4%)%'
	`).Scan(&count)
	if err != nil {
		return fmt.Errorf("check files constraint: %w", err)
	}
	if count > 0 {
		// Table already has the updated constraint, skip
		return nil
	}

	// Drop indexes first (they'll be recreated with the new table)
	indexes := []string{
		"files_workspace_idx",
		"files_path_hash_idx",
		"files_hierarchy_idx",
		"files_content_lookup_idx",
		"files_parent_lookup_idx",
		"files_name_search_idx",
		"files_kind_filter_idx",
		"files_workspace_hierarchy_idx",
	}
	for _, idx := range indexes {
		if _, err := tx.ExecContext(ctx, "DROP INDEX IF EXISTS "+idx); err != nil {
			return err
		}
	}

	// Create new table with updated CHECK constraints
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE files_new (
			id BLOB NOT NULL PRIMARY KEY,
			workspace_id BLOB NOT NULL,
			parent_id BLOB,
			content_id BLOB,
			content_kind INT8 NOT NULL DEFAULT 0,
			name TEXT NOT NULL,
			display_order REAL NOT NULL DEFAULT 0,
			path_hash TEXT,
			updated_at BIGINT NOT NULL DEFAULT (unixepoch()),
			CHECK (length (id) == 16),
			CHECK (content_kind IN (0, 1, 2, 3, 4)), -- 0=folder, 1=http, 2=flow, 3=http_delta, 4=credential
			CHECK (
				(content_kind = 0 AND content_id IS NOT NULL) OR
				(content_kind = 1 AND content_id IS NOT NULL) OR
				(content_kind = 2 AND content_id IS NOT NULL) OR
				(content_kind = 3 AND content_id IS NOT NULL) OR
				(content_kind = 4 AND content_id IS NOT NULL) OR
				(content_id IS NULL)
			),
			FOREIGN KEY (workspace_id) REFERENCES workspaces (id) ON DELETE CASCADE,
			FOREIGN KEY (parent_id) REFERENCES files (id) ON DELETE SET NULL
		)
	`); err != nil {
		return err
	}

	// Copy all data from old table to new
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO files_new (id, workspace_id, parent_id, content_id, content_kind, name, display_order, path_hash, updated_at)
		SELECT id, workspace_id, parent_id, content_id, content_kind, name, display_order, path_hash, updated_at
		FROM files
	`); err != nil {
		return err
	}

	// Drop old table
	if _, err := tx.ExecContext(ctx, "DROP TABLE files"); err != nil {
		return err
	}

	// Rename new table to files
	if _, err := tx.ExecContext(ctx, "ALTER TABLE files_new RENAME TO files"); err != nil {
		return err
	}

	// Recreate all indexes
	indexSQL := []string{
		`CREATE INDEX files_workspace_idx ON files (workspace_id)`,
		`CREATE UNIQUE INDEX files_path_hash_idx ON files (workspace_id, path_hash) WHERE path_hash IS NOT NULL`,
		`CREATE INDEX files_hierarchy_idx ON files (workspace_id, parent_id, display_order)`,
		`CREATE INDEX files_content_lookup_idx ON files (content_kind, content_id) WHERE content_id IS NOT NULL`,
		`CREATE INDEX files_parent_lookup_idx ON files (parent_id, display_order) WHERE parent_id IS NOT NULL`,
		`CREATE INDEX files_name_search_idx ON files (workspace_id, name)`,
		`CREATE INDEX files_kind_filter_idx ON files (workspace_id, content_kind)`,
		`CREATE INDEX files_workspace_hierarchy_idx ON files (workspace_id, parent_id, content_kind, display_order)`,
	}
	for _, sql := range indexSQL {
		if _, err := tx.ExecContext(ctx, sql); err != nil {
			return err
		}
	}

	return nil
}

// validateAITables verifies all tables and indexes were created successfully.
func validateAITables(ctx context.Context, db *sql.DB) error {
	// Verify all AI tables exist
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
			return fmt.Errorf("table %s not found: %w", table, err)
		}
	}

	// Verify credential index exists
	var idxName string
	err := db.QueryRowContext(ctx, `
		SELECT name FROM sqlite_master
		WHERE type='index' AND name='credential_workspace_idx'
	`).Scan(&idxName)
	if err != nil {
		return fmt.Errorf("credential_workspace_idx not found: %w", err)
	}

	// Verify files table has updated constraint for content_kind=4
	// Use flexible pattern to handle SQLite formatting variations
	var count int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sqlite_master
		WHERE type='table' AND name='files'
		AND sql LIKE '%content_kind%IN%(%4%)%'
	`).Scan(&count)
	if err != nil {
		return fmt.Errorf("check files constraint: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("files table constraint not updated for content_kind=4")
	}

	// Verify files table indexes exist
	filesIndexes := []string{
		"files_workspace_idx",
		"files_path_hash_idx",
		"files_hierarchy_idx",
		"files_content_lookup_idx",
		"files_parent_lookup_idx",
		"files_name_search_idx",
		"files_kind_filter_idx",
		"files_workspace_hierarchy_idx",
	}

	for _, idx := range filesIndexes {
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
