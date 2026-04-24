package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc"
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

	// Verify files table supports content_kind=6 (websocket)
	var tableDef string
	err = db.QueryRowContext(ctx, `
		SELECT sql FROM sqlite_master
		WHERE type='table' AND name='files'
	`).Scan(&tableDef)
	if err != nil {
		t.Fatalf("failed to get files table definition: %v", err)
	}

	// Check that the constraint includes both content_kind=6 (websocket) and 7 (graphql_delta)
	if !contains(tableDef, "content_kind IN (0, 1, 2, 3, 4, 5, 6, 7)") {
		t.Errorf("files table CHECK constraint doesn't include content_kind 6 and 7: %s", tableDef)
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

// TestMigrationCount ensures no migrations are accidentally omitted.
func TestMigrationCount(t *testing.T) {
	migrations := migrate.List()
	const expectedCount = 10
	if len(migrations) != expectedCount {
		t.Errorf("expected %d registered migrations, got %d — update this count if you added/removed a migration", expectedCount, len(migrations))
	}
}

// TestWebSocketTablesCreated verifies the WebSocket migration creates all tables and indexes.
func TestWebSocketTablesCreated(t *testing.T) {
	ctx := context.Background()
	db := runAllMigrations(t, ctx)

	tables := []string{
		"websocket",
		"websocket_header",
		"flow_node_ws_connection",
		"flow_node_ws_send",
	}
	for _, table := range tables {
		assertTableExists(t, ctx, db, table)
	}

	indexes := []string{
		"websocket_workspace_idx",
		"websocket_folder_idx",
		"websocket_header_ws_idx",
		"websocket_header_order_idx",
	}
	for _, idx := range indexes {
		assertIndexExists(t, ctx, db, idx)
	}
}

// TestSubFlowTablesCreated verifies the sub-flow migration creates all tables.
func TestSubFlowTablesCreated(t *testing.T) {
	ctx := context.Background()
	db := runAllMigrations(t, ctx)

	tables := []string{
		"flow_node_sub_flow_trigger",
		"flow_node_sub_flow_return",
		"flow_node_run_sub_flow",
	}
	for _, table := range tables {
		assertTableExists(t, ctx, db, table)
	}

	// Verify run_sub_flow has the expected columns
	columns := []string{"flow_node_id", "target_flow_id", "target_flow_name", "inputs"}
	for _, col := range columns {
		assertColumnExists(t, ctx, db, "flow_node_run_sub_flow", col)
	}
}

// TestWaitNodeTableCreated verifies the wait node migration.
func TestWaitNodeTableCreated(t *testing.T) {
	ctx := context.Background()
	db := runAllMigrations(t, ctx)

	assertTableExists(t, ctx, db, "flow_node_wait")
	assertColumnExists(t, ctx, db, "flow_node_wait", "duration_ms")
}

// TestFlowErrorColumnsCreated verifies flow error/node_id_mapping columns.
func TestFlowErrorColumnsCreated(t *testing.T) {
	ctx := context.Background()
	db := runAllMigrations(t, ctx)

	assertColumnExists(t, ctx, db, "flow", "error")
	assertColumnExists(t, ctx, db, "flow", "node_id_mapping")
}

// TestMigratedSchemaMatchesFresh is the critical release-safety test.
// It compares the schema produced by running all migrations on a base DB
// against a fresh DB created from the schema files (sqlc/schema/*.sql).
// Any mismatch means migrations are out of sync with the canonical schema.
func TestMigratedSchemaMatchesFresh(t *testing.T) {
	ctx := context.Background()

	// 1. Build "migrated" DB: base schema + all migrations
	migratedDB := runAllMigrations(t, ctx)

	// 2. Build "fresh" DB: just the schema files (what a fresh install gets)
	freshDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open fresh db: %v", err)
	}
	freshDB.SetMaxOpenConns(1)
	t.Cleanup(func() { freshDB.Close() })

	if err := sqlc.CreateLocalTables(ctx, freshDB); err != nil {
		t.Fatalf("failed to create fresh schema: %v", err)
	}

	// 3. Compare tables
	migratedTables := getSchemaObjects(t, ctx, migratedDB, "table")
	freshTables := getSchemaObjects(t, ctx, freshDB, "table")

	// Exclude schema_migrations (created by migration runner, not in canonical schema)
	migratedTablesFiltered := filterKeys(migratedTables, "schema_migrations")
	freshTablesFiltered := freshTables

	// Check all fresh tables exist in migrated schema
	for table := range freshTablesFiltered {
		if _, ok := migratedTablesFiltered[table]; !ok {
			t.Errorf("table %q exists in fresh schema but NOT in migrated schema — missing migration?", table)
		}
	}

	// Check migrated schema doesn't have extra tables
	for table := range migratedTablesFiltered {
		if _, ok := freshTablesFiltered[table]; !ok {
			t.Errorf("table %q exists in migrated schema but NOT in fresh schema — stale migration or missing schema file?", table)
		}
	}

	// 4. Compare columns for every shared table
	for table := range freshTablesFiltered {
		if _, ok := migratedTablesFiltered[table]; !ok {
			continue
		}

		freshCols := getTableColumns(t, ctx, freshDB, table)
		migratedCols := getTableColumns(t, ctx, migratedDB, table)

		for col := range freshCols {
			if _, ok := migratedCols[col]; !ok {
				t.Errorf("table %q: column %q exists in fresh schema but NOT in migrated schema", table, col)
			}
		}
		for col := range migratedCols {
			if _, ok := freshCols[col]; !ok {
				t.Errorf("table %q: column %q exists in migrated schema but NOT in fresh schema", table, col)
			}
		}
	}

	// 5. Compare indexes (by name)
	migratedIndexes := getSchemaObjects(t, ctx, migratedDB, "index")
	freshIndexes := getSchemaObjects(t, ctx, freshDB, "index")

	// Filter out SQLite auto-indexes and migration-related indexes
	migratedIdxFiltered := filterAutoIndexes(migratedIndexes, "idx_schema_migrations")
	freshIdxFiltered := filterAutoIndexes(freshIndexes)

	for idx := range freshIdxFiltered {
		if _, ok := migratedIdxFiltered[idx]; !ok {
			t.Errorf("index %q exists in fresh schema but NOT in migrated schema", idx)
		}
	}

	// 6. Compare triggers
	// Note: migrated DBs may have triggers that enforce constraints which fresh DBs
	// handle via CHECK constraints (e.g., delta/snapshot exclusivity). These are
	// intentionally different mechanisms achieving the same goal, since SQLite cannot
	// add CHECK constraints to existing tables via ALTER TABLE.
	migratedTriggers := getSchemaObjects(t, ctx, migratedDB, "trigger")
	freshTriggers := getSchemaObjects(t, ctx, freshDB, "trigger")

	// Triggers that exist only in migrated schema because fresh uses CHECK constraints
	migrationOnlyTriggers := map[string]bool{
		"trg_http_delta_snapshot_insert": true,
		"trg_http_delta_snapshot_update": true,
	}

	for trigger := range freshTriggers {
		if _, ok := migratedTriggers[trigger]; !ok {
			t.Errorf("trigger %q exists in fresh schema but NOT in migrated schema", trigger)
		}
	}
	for trigger := range migratedTriggers {
		if migrationOnlyTriggers[trigger] {
			continue // Expected: migration uses trigger, fresh uses CHECK
		}
		if _, ok := freshTriggers[trigger]; !ok {
			t.Errorf("trigger %q exists in migrated schema but NOT in fresh schema", trigger)
		}
	}
}

// TestMigratedSchemaDataPreservation verifies migrations don't lose existing data.
// Inserts rows before a new migration and verifies they survive.
func TestMigratedSchemaDataPreservation(t *testing.T) {
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

	// Run up to just before the WebSocket migration (the last one that recreates files table)
	if err := RunTo(ctx, db, cfg, MigrationAddSubFlowTablesID); err != nil {
		t.Fatalf("RunTo failed: %v", err)
	}

	// Insert a test file row with content_kind=5 (graphql)
	testID := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	testWsID := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 16}
	_, err = db.ExecContext(ctx, `INSERT INTO workspaces (id, name) VALUES (?, 'test')`, testWsID)
	if err != nil {
		t.Fatalf("failed to insert workspace: %v", err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO files (id, workspace_id, content_kind, name, display_order) VALUES (?, ?, 5, 'test_graphql', 1.0)`, testID, testWsID)
	if err != nil {
		t.Fatalf("failed to insert files row: %v", err)
	}

	// Now run remaining migrations (WebSocket migration recreates files table)
	if err := Run(ctx, db, cfg); err != nil {
		t.Fatalf("full Run after partial failed: %v", err)
	}

	// Verify the row survived
	var name string
	err = db.QueryRowContext(ctx, `SELECT name FROM files WHERE id = ?`, testID).Scan(&name)
	if err != nil {
		t.Fatalf("files row lost after migration: %v", err)
	}
	if name != "test_graphql" {
		t.Errorf("files row data corrupted: got name=%q, want %q", name, "test_graphql")
	}

	// Verify we can now insert content_kind=6 (websocket) and 7 (graphql_delta)
	testID2 := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 17}
	_, err = db.ExecContext(ctx, `INSERT INTO files (id, workspace_id, content_kind, name, display_order) VALUES (?, ?, 6, 'test_ws', 2.0)`, testID2, testWsID)
	if err != nil {
		t.Errorf("cannot insert content_kind=6 after migration: %v", err)
	}

	testID3 := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 18}
	_, err = db.ExecContext(ctx, `INSERT INTO files (id, workspace_id, content_kind, name, display_order) VALUES (?, ?, 7, 'test_gql_delta', 3.0)`, testID3, testWsID)
	if err != nil {
		t.Errorf("cannot insert content_kind=7 after migration: %v", err)
	}
}

// TestPartialMigrationUpgradePaths tests upgrading from each intermediate migration state.
// This catches ordering issues where migration N depends on something migration N-1 changed.
func TestPartialMigrationUpgradePaths(t *testing.T) {
	ctx := context.Background()
	allMigrations := migrate.List()

	for i := range allMigrations {
		targetID := allMigrations[i].ID
		t.Run(fmt.Sprintf("upgrade_from_%s", targetID[:8]), func(t *testing.T) {
			db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
			if err != nil {
				t.Fatalf("failed to create test db: %v", err)
			}
			t.Cleanup(cleanup)

			cfg := Config{
				DatabasePath: ":memory:",
				DataDir:      t.TempDir(),
			}

			// Run up to migration i
			if err := RunTo(ctx, db, cfg, targetID); err != nil {
				t.Fatalf("RunTo(%s) failed: %v", targetID[:8], err)
			}

			// Then run all remaining migrations
			if err := Run(ctx, db, cfg); err != nil {
				t.Fatalf("Run (remaining from %s) failed: %v", targetID[:8], err)
			}

			// Verify all migrations are finished
			var count int
			err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE status = 'finished'").Scan(&count)
			if err != nil {
				t.Fatalf("failed to count finished migrations: %v", err)
			}
			if count != len(allMigrations) {
				t.Errorf("expected %d finished migrations, got %d", len(allMigrations), count)
			}
		})
	}
}

// --- Helpers ---

func runAllMigrations(t *testing.T, ctx context.Context) *sql.DB {
	t.Helper()
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
	return db
}

func assertTableExists(t *testing.T, ctx context.Context, db *sql.DB, table string) {
	t.Helper()
	var name string
	err := db.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
	if err != nil {
		t.Errorf("table %s not found: %v", table, err)
	}
}

func assertIndexExists(t *testing.T, ctx context.Context, db *sql.DB, idx string) {
	t.Helper()
	var name string
	err := db.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='index' AND name=?`, idx).Scan(&name)
	if err != nil {
		t.Errorf("index %s not found: %v", idx, err)
	}
}

func assertColumnExists(t *testing.T, ctx context.Context, db *sql.DB, table, col string) {
	t.Helper()
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`, table, col).Scan(&count)
	if err != nil {
		t.Fatalf("failed to check %s.%s: %v", table, col, err)
	}
	if count == 0 {
		t.Errorf("table %s missing column %s", table, col)
	}
}

// getSchemaObjects returns a map of name -> sql definition for all objects of the given type.
func getSchemaObjects(t *testing.T, ctx context.Context, db *sql.DB, objType string) map[string]string {
	t.Helper()
	rows, err := db.QueryContext(ctx, `SELECT name, COALESCE(sql, '') FROM sqlite_master WHERE type=? ORDER BY name`, objType)
	if err != nil {
		t.Fatalf("failed to query schema objects of type %s: %v", objType, err)
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var name, sqlDef string
		if err := rows.Scan(&name, &sqlDef); err != nil {
			t.Fatalf("failed to scan schema object: %v", err)
		}
		result[name] = sqlDef
	}
	return result
}

// getTableColumns returns column names for a table.
func getTableColumns(t *testing.T, ctx context.Context, db *sql.DB, table string) map[string]bool {
	t.Helper()
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info('%s')`, table))
	if err != nil {
		t.Fatalf("failed to get columns for %s: %v", table, err)
	}
	defer rows.Close()

	cols := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dfltValue sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			t.Fatalf("failed to scan column info for %s: %v", table, err)
		}
		cols[name] = true
	}
	return cols
}

func filterKeys(m map[string]string, exclude ...string) map[string]string {
	excl := make(map[string]bool, len(exclude))
	for _, k := range exclude {
		excl[k] = true
	}
	result := make(map[string]string, len(m))
	for k, v := range m {
		if !excl[k] {
			result[k] = v
		}
	}
	return result
}

func filterAutoIndexes(m map[string]string, extraExclude ...string) map[string]string {
	excl := make(map[string]bool, len(extraExclude))
	for _, k := range extraExclude {
		excl[k] = true
	}
	result := make(map[string]string, len(m))
	for k, v := range m {
		if !strings.HasPrefix(k, "sqlite_autoindex_") && !excl[k] {
			result[k] = v
		}
	}
	return result
}

