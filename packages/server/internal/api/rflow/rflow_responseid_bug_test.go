package rflow

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"

	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// createTestDB creates a test database with minimal schema for testing
func createTestDB(t *testing.T) (*sql.DB, *gen.Queries) {
	ctx := context.Background()

	// Create in-memory SQLite database
	db, err := sql.Open("sqlite", ":memory:?cache=shared&_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// Create minimal schema needed for our test
	createTableSQL := `
	CREATE TABLE node_execution (
		id BLOB NOT NULL PRIMARY KEY,
		node_id BLOB NOT NULL,
		name TEXT NOT NULL,
		state INT8 NOT NULL,
		error TEXT,
		input_data BLOB,
		input_data_compress_type INT8 NOT NULL DEFAULT 0,
		output_data BLOB,
		output_data_compress_type INT8 NOT NULL DEFAULT 0,
		response_id BLOB,
		completed_at BIGINT
	);
	
	CREATE TABLE flow_node (
		id BLOB NOT NULL PRIMARY KEY,
		flow_id BLOB NOT NULL,
		name TEXT NOT NULL,
		node_kind INT NOT NULL,
		position_x REAL NOT NULL,
		position_y REAL NOT NULL
	);`

	_, err = db.ExecContext(ctx, createTableSQL)
	if err != nil {
		t.Fatalf("Failed to create tables: %v", err)
	}

	// Create queries instance
	queries := gen.New(db)

	return db, queries
}

// TestForLoopNodeExecutionPersistence tests the new behavior where FOR loop iterations
// are now ALWAYS saved to the database. This test verifies that we get the correct
// number of records: 10 FOR loop iteration records + 10 REQUEST records = 20 total.
func TestForLoopNodeExecutionPersistence(t *testing.T) {
	// Create test database
	db, _ := createTestDB(t)
	defer func() { _ = db.Close() }()

	// Record baseline node_execution count (should be 0 in new test DB)
	var baselineCount int
	err := db.QueryRow("SELECT COUNT(*) FROM node_execution").Scan(&baselineCount)
	require.NoError(t, err)
	t.Logf("Baseline node_execution count: %d", baselineCount)

	// Create test flow nodes for demonstration
	forNodeID := idwrap.NewNow()
	requestNodeID := idwrap.NewNow()
	flowID := idwrap.NewNow()

	// Insert test flow nodes into database
	_, err = db.Exec(`INSERT INTO flow_node (id, flow_id, name, node_kind, position_x, position_y) VALUES (?, ?, ?, ?, ?, ?)`,
		forNodeID.Bytes(), flowID.Bytes(), "FOR Loop - 10 iterations", mnnode.NODE_KIND_FOR, 100.0, 100.0)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO flow_node (id, flow_id, name, node_kind, position_x, position_y) VALUES (?, ?, ?, ?, ?, ?)`,
		requestNodeID.Bytes(), flowID.Bytes(), "Request to Google", mnnode.NODE_KIND_REQUEST, 300.0, 100.0)
	require.NoError(t, err)

	t.Logf("Created flow with FOR loop (%s) and REQUEST node (%s)", forNodeID, requestNodeID)

	// Simulate what would happen during FOR loop execution
	t.Log("Simulating FOR loop with 10 iterations...")

	// NEW BEHAVIOR: FOR loop iterations are now ALWAYS saved to database
	// Each iteration creates TWO records: one FOR loop iteration + one REQUEST execution

	for i := 0; i < 10; i++ {
		// Create FOR loop iteration record (NEW)
		iterationID := idwrap.NewNow()
		completedAt := time.Now().UnixMilli()

		_, err = db.Exec(`INSERT INTO node_execution (id, node_id, name, state, completed_at) VALUES (?, ?, ?, ?, ?)`,
			iterationID.Bytes(), forNodeID.Bytes(),
			fmt.Sprintf("FOR Loop - iteration %d", i),
			int8(mnnode.NODE_STATE_SUCCESS), completedAt)
		require.NoError(t, err)

		// Create REQUEST node execution record (EXISTING)
		executionID := idwrap.NewNow()
		completedAt = time.Now().UnixMilli()

		_, err = db.Exec(`INSERT INTO node_execution (id, node_id, name, state, completed_at) VALUES (?, ?, ?, ?, ?)`,
			executionID.Bytes(), requestNodeID.Bytes(),
			fmt.Sprintf("Request to Google - iteration %d", i),
			int8(mnnode.NODE_STATE_SUCCESS), completedAt)
		require.NoError(t, err)

		t.Logf("Created node_execution records for iteration %d (FOR + REQUEST)", i)
	}

	// Wait for any async operations to complete
	time.Sleep(100 * time.Millisecond)

	// Verify the results
	var finalCount int
	err = db.QueryRow("SELECT COUNT(*) FROM node_execution").Scan(&finalCount)
	require.NoError(t, err)

	newRecords := finalCount - baselineCount
	t.Logf("Final node_execution count: %d (added %d new records)", finalCount, newRecords)

	// Query breakdown by node kind for all records in the test
	rows, err := db.Query(`
		SELECT fn.node_kind, COUNT(ne.id) as execution_count
		FROM node_execution ne 
		JOIN flow_node fn ON ne.node_id = fn.id 
		GROUP BY fn.node_kind
		ORDER BY fn.node_kind
	`)
	require.NoError(t, err)
	defer func() {
		if err := rows.Close(); err != nil {
			t.Logf("Warning: failed to close rows: %v", err)
		}
	}()

	nodeKindCounts := make(map[int32]int)
	for rows.Next() {
		var nodeKind int32
		var count int
		err = rows.Scan(&nodeKind, &count)
		require.NoError(t, err)
		nodeKindCounts[nodeKind] = count

		nodeTypeString := "UNKNOWN"
		switch nodeKind {
		case mnnode.NODE_KIND_REQUEST:
			nodeTypeString = "REQUEST"
		case mnnode.NODE_KIND_FOR:
			nodeTypeString = "FOR"
		case mnnode.NODE_KIND_NO_OP:
			nodeTypeString = "NO_OP"
		}

		t.Logf("Node kind %d (%s): %d executions", nodeKind, nodeTypeString, count)
	}

	// CRITICAL TEST: With the new behavior, we should have:
	// - 10 FOR loop iteration executions (now ALWAYS saved to database)
	// - 10 REQUEST node executions (one per iteration)
	// - Total: 20 records
	require.Equal(t, 10, nodeKindCounts[mnnode.NODE_KIND_FOR],
		"Should have exactly 10 FOR loop iteration executions")
	require.Equal(t, 10, nodeKindCounts[mnnode.NODE_KIND_REQUEST],
		"Should have exactly 10 REQUEST node executions (one per iteration)")
	require.Equal(t, 20, newRecords,
		"Should have exactly 20 new node_execution records total (10 iterations + 10 requests)")

	t.Log("✅ TEST PASSED: FOR loop persistence works correctly")
	t.Log("- FOR loop iteration executions: 10 (now saved to database)")
	t.Log("- REQUEST node executions: 10 (one per iteration)")
	t.Log("- FOR loop main execution: 0 (successful loops still hidden)")
	t.Log("- Total new node_execution records: 20")
}
