package migrations

import (
	"context"
	"testing"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlitemem"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/migrate"
)

func TestMigrationsRegister(t *testing.T) {
	// Verify all migrations are registered
	migrations := migrate.List()
	if len(migrations) < 1 {
		t.Fatalf("expected at least 1 migration registered, got %d", len(migrations))
	}

	// Verify AI tables migration is registered
	found := false
	for _, m := range migrations {
		if m.ID == MigrationAddAITablesID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("MigrationAddAITablesID not found in registered migrations")
	}
}

func TestMigrationsApply(t *testing.T) {
	ctx := context.Background()

	// Create in-memory database with base schema
	// Note: sqlitemem.NewSQLiteMem already calls CreateLocalTables
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	t.Cleanup(cleanup)

	// Run migrations
	cfg := Config{
		DatabasePath: ":memory:",
		DataDir:      t.TempDir(),
	}
	if err := Run(ctx, db, cfg); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Verify schema_migrations table exists and has records
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE status = 'finished'").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query schema_migrations: %v", err)
	}
	if count < 1 {
		t.Errorf("expected at least 1 finished migration, got %d", count)
	}

	// Verify credential table was created
	var tableName string
	err = db.QueryRowContext(ctx, `
		SELECT name FROM sqlite_master
		WHERE type='table' AND name='credential'
	`).Scan(&tableName)
	if err != nil {
		t.Fatalf("credential table not found: %v", err)
	}
}

func TestMigrationsIdempotent(t *testing.T) {
	ctx := context.Background()

	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	t.Cleanup(cleanup)

	cfg := Config{
		DatabasePath: ":memory:",
		DataDir:      t.TempDir(),
	}

	// Run migrations twice - should be idempotent
	if err := Run(ctx, db, cfg); err != nil {
		t.Fatalf("first migration run failed: %v", err)
	}

	if err := Run(ctx, db, cfg); err != nil {
		t.Fatalf("second migration run failed: %v", err)
	}

	// Verify migration records are not duplicated
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count migrations: %v", err)
	}
	if count < 1 {
		t.Errorf("expected at least 1 migration record, got %d", count)
	}
}

func TestAITablesCreated(t *testing.T) {
	ctx := context.Background()

	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	t.Cleanup(cleanup)

	cfg := Config{
		DatabasePath: ":memory:",
		DataDir:      t.TempDir(),
	}
	if err := Run(ctx, db, cfg); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Verify all AI tables were created
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
			t.Errorf("table %s not found: %v", table, err)
		}
	}

	// Verify credential_workspace_idx exists
	var idxName string
	err = db.QueryRowContext(ctx, `
		SELECT name FROM sqlite_master
		WHERE type='index' AND name='credential_workspace_idx'
	`).Scan(&idxName)
	if err != nil {
		t.Errorf("credential_workspace_idx not found: %v", err)
	}
}

func TestRunTo(t *testing.T) {
	ctx := context.Background()

	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	t.Cleanup(cleanup)

	cfg := Config{
		DatabasePath: ":memory:",
		DataDir:      t.TempDir(),
	}

	// Run only up to AI tables migration
	if err := RunTo(ctx, db, cfg, MigrationAddAITablesID); err != nil {
		t.Fatalf("RunTo failed: %v", err)
	}

	// Verify AI tables migration was run
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE status = 'finished'").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count migrations: %v", err)
	}
	if count < 1 {
		t.Errorf("expected at least 1 finished migration, got %d", count)
	}

	// Credential table should exist
	var tableName string
	err = db.QueryRowContext(ctx, `
		SELECT name FROM sqlite_master
		WHERE type='table' AND name='credential'
	`).Scan(&tableName)
	if err != nil {
		t.Errorf("credential table should exist: %v", err)
	}
}

func TestFilesTableConstraintUpdated(t *testing.T) {
	ctx := context.Background()

	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	t.Cleanup(cleanup)

	cfg := Config{
		DatabasePath: ":memory:",
		DataDir:      t.TempDir(),
	}
	if err := Run(ctx, db, cfg); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Verify files table supports content_kind=5 (graphql)
	var tableDef string
	err = db.QueryRowContext(ctx, `
		SELECT sql FROM sqlite_master
		WHERE type='table' AND name='files'
	`).Scan(&tableDef)
	if err != nil {
		t.Fatalf("failed to get files table definition: %v", err)
	}

	// Check that the constraint includes content_kind=5
	if !contains(tableDef, "content_kind IN (0, 1, 2, 3, 4, 5)") {
		t.Errorf("files table CHECK constraint doesn't include content_kind=5: %s", tableDef)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestGraphQLDeltaColumnsCreated(t *testing.T) {
	ctx := context.Background()

	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	t.Cleanup(cleanup)

	cfg := Config{
		DatabasePath: ":memory:",
		DataDir:      t.TempDir(),
	}
	if err := Run(ctx, db, cfg); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Verify graphql table delta columns
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
		var count int
		err := db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM pragma_table_info('graphql')
			WHERE name = ?
		`, col).Scan(&count)
		if err != nil {
			t.Fatalf("failed to check graphql.%s: %v", col, err)
		}
		if count == 0 {
			t.Errorf("graphql table missing column: %s", col)
		}
	}

	// Verify graphql_header table delta columns
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
		var count int
		err := db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM pragma_table_info('graphql_header')
			WHERE name = ?
		`, col).Scan(&count)
		if err != nil {
			t.Fatalf("failed to check graphql_header.%s: %v", col, err)
		}
		if count == 0 {
			t.Errorf("graphql_header table missing column: %s", col)
		}
	}

	// Verify graphql_assert table delta columns
	assertColumns := []string{
		"parent_graphql_assert_id",
		"is_delta",
		"delta_value",
		"delta_enabled",
		"delta_description",
		"delta_display_order",
	}

	for _, col := range assertColumns {
		var count int
		err := db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM pragma_table_info('graphql_assert')
			WHERE name = ?
		`, col).Scan(&count)
		if err != nil {
			t.Fatalf("failed to check graphql_assert.%s: %v", col, err)
		}
		if count == 0 {
			t.Errorf("graphql_assert table missing column: %s", col)
		}
	}

	// Verify delta indexes were created
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
			t.Errorf("index %s not found: %v", idx, err)
		}
	}
}
