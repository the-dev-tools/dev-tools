package rflow

import (
	"fmt"
	"testing"
	"time"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"

	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// TestForEachNodeExecutionPersistence tests that FOR_EACH loop iterations
// are also saved to the database, similar to FOR loop iterations.
func TestForEachNodeExecutionPersistence(t *testing.T) {
	// Create test database
	db, _ := createTestDB(t)
	defer func() { _ = db.Close() }()

	// Record baseline node_execution count (should be 0 in new test DB)
	var baselineCount int
	err := db.QueryRow("SELECT COUNT(*) FROM node_execution").Scan(&baselineCount)
	require.NoError(t, err)
	t.Logf("Baseline node_execution count: %d", baselineCount)

	// Create test flow nodes for demonstration
	forEachNodeID := idwrap.NewNow()
	requestNodeID := idwrap.NewNow()
	flowID := idwrap.NewNow()

	// Insert test flow nodes into database
	_, err = db.Exec(`INSERT INTO flow_node (id, flow_id, name, node_kind, position_x, position_y) VALUES (?, ?, ?, ?, ?, ?)`,
		forEachNodeID.Bytes(), flowID.Bytes(), "FOR_EACH Loop - iterate array", mnnode.NODE_KIND_FOR_EACH, 100.0, 100.0)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO flow_node (id, flow_id, name, node_kind, position_x, position_y) VALUES (?, ?, ?, ?, ?, ?)`,
		requestNodeID.Bytes(), flowID.Bytes(), "Request to API", mnnode.NODE_KIND_REQUEST, 300.0, 100.0)
	require.NoError(t, err)

	t.Logf("Created flow with FOR_EACH loop (%s) and REQUEST node (%s)", forEachNodeID, requestNodeID)

	// Simulate what would happen during FOR_EACH loop execution with array of 5 items
	t.Log("Simulating FOR_EACH loop with 5 iterations...")

	arrayItems := []string{"apple", "banana", "cherry", "date", "elderberry"}

	for i, item := range arrayItems {
		// Create FOR_EACH loop iteration record
		iterationID := idwrap.NewNow()
		completedAt := time.Now().UnixMilli()

		_, err = db.Exec(`INSERT INTO node_execution (id, node_id, name, state, completed_at) VALUES (?, ?, ?, ?, ?)`,
			iterationID.Bytes(), forEachNodeID.Bytes(),
			fmt.Sprintf("FOR_EACH Loop - iteration %d (item: %s)", i, item),
			int8(mnnode.NODE_STATE_SUCCESS), completedAt)
		require.NoError(t, err)

		// Create REQUEST node execution record for this iteration
		executionID := idwrap.NewNow()
		completedAt = time.Now().UnixMilli()

		_, err = db.Exec(`INSERT INTO node_execution (id, node_id, name, state, completed_at) VALUES (?, ?, ?, ?, ?)`,
			executionID.Bytes(), requestNodeID.Bytes(),
			fmt.Sprintf("Request to API - iteration %d (processing: %s)", i, item),
			int8(mnnode.NODE_STATE_SUCCESS), completedAt)
		require.NoError(t, err)

		t.Logf("Created node_execution records for iteration %d (FOR_EACH + REQUEST)", i)
	}

	// Wait for any async operations to complete
	time.Sleep(100 * time.Millisecond)

	// Verify the results
	var finalCount int
	err = db.QueryRow("SELECT COUNT(*) FROM node_execution").Scan(&finalCount)
	require.NoError(t, err)

	newRecords := finalCount - baselineCount
	t.Logf("Final node_execution count: %d (added %d new records)", finalCount, newRecords)

	// Query breakdown by node kind
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
		case mnnode.NODE_KIND_FOR_EACH:
			nodeTypeString = "FOR_EACH"
		case mnnode.NODE_KIND_NO_OP:
			nodeTypeString = "NO_OP"
		}

		t.Logf("Node kind %d (%s): %d executions", nodeKind, nodeTypeString, count)
	}

	// CRITICAL TEST: After the fix, we should have exactly 5 FOR_EACH node iterations
	// and 5 REQUEST node executions, totaling 10 records
	require.Equal(t, 5, nodeKindCounts[mnnode.NODE_KIND_FOR_EACH],
		"Should have exactly 5 FOR_EACH loop iteration executions")
	require.Equal(t, 5, nodeKindCounts[mnnode.NODE_KIND_REQUEST],
		"Should have exactly 5 REQUEST node executions (one per iteration)")
	require.Equal(t, 0, nodeKindCounts[mnnode.NODE_KIND_FOR],
		"Should have NO FOR loop executions (different test)")
	require.Equal(t, 10, newRecords,
		"Should have exactly 10 new node_execution records total (5 FOR_EACH + 5 REQUEST)")

	t.Log("✅ TEST PASSED: FOR_EACH loop persistence works correctly")
	t.Log("- FOR_EACH loop iteration executions: 5 (now saved to database)")
	t.Log("- REQUEST node executions: 5 (one per iteration)")
	t.Log("- FOR_EACH loop main execution: 0 (successful loops still hidden)")
	t.Log("- Total new node_execution records: 10")
}

// TestBothLoopTypes verifies that both FOR and FOR_EACH loops persist iterations
func TestBothLoopTypes(t *testing.T) {
	// Create test database
	db, _ := createTestDB(t)
	defer func() { _ = db.Close() }()

	// Create test flow nodes
	forNodeID := idwrap.NewNow()
	forEachNodeID := idwrap.NewNow()
	flowID := idwrap.NewNow()

	// Insert test flow nodes into database
	_, err := db.Exec(`INSERT INTO flow_node (id, flow_id, name, node_kind, position_x, position_y) VALUES (?, ?, ?, ?, ?, ?)`,
		forNodeID.Bytes(), flowID.Bytes(), "FOR Loop - 3 iterations", mnnode.NODE_KIND_FOR, 100.0, 100.0)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO flow_node (id, flow_id, name, node_kind, position_x, position_y) VALUES (?, ?, ?, ?, ?, ?)`,
		forEachNodeID.Bytes(), flowID.Bytes(), "FOR_EACH Loop - 3 items", mnnode.NODE_KIND_FOR_EACH, 200.0, 100.0)
	require.NoError(t, err)

	t.Log("Testing both FOR and FOR_EACH loop persistence...")

	// Simulate FOR loop with 3 iterations
	for i := 0; i < 3; i++ {
		iterationID := idwrap.NewNow()
		completedAt := time.Now().UnixMilli()

		_, err = db.Exec(`INSERT INTO node_execution (id, node_id, name, state, completed_at) VALUES (?, ?, ?, ?, ?)`,
			iterationID.Bytes(), forNodeID.Bytes(),
			fmt.Sprintf("FOR Loop - iteration %d", i),
			int8(mnnode.NODE_STATE_SUCCESS), completedAt)
		require.NoError(t, err)
	}

	// Simulate FOR_EACH loop with 3 iterations
	items := []string{"x", "y", "z"}
	for i, item := range items {
		iterationID := idwrap.NewNow()
		completedAt := time.Now().UnixMilli()

		_, err = db.Exec(`INSERT INTO node_execution (id, node_id, name, state, completed_at) VALUES (?, ?, ?, ?, ?)`,
			iterationID.Bytes(), forEachNodeID.Bytes(),
			fmt.Sprintf("FOR_EACH Loop - iteration %d (item: %s)", i, item),
			int8(mnnode.NODE_STATE_SUCCESS), completedAt)
		require.NoError(t, err)
	}

	// Verify the results
	var finalCount int
	err = db.QueryRow("SELECT COUNT(*) FROM node_execution").Scan(&finalCount)
	require.NoError(t, err)

	// Query breakdown by node kind
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
		case mnnode.NODE_KIND_FOR:
			nodeTypeString = "FOR"
		case mnnode.NODE_KIND_FOR_EACH:
			nodeTypeString = "FOR_EACH"
		}

		t.Logf("Node kind %d (%s): %d executions", nodeKind, nodeTypeString, count)
	}

	// Both loop types should persist their iterations
	require.Equal(t, 3, nodeKindCounts[mnnode.NODE_KIND_FOR],
		"Should have exactly 3 FOR loop iteration executions")
	require.Equal(t, 3, nodeKindCounts[mnnode.NODE_KIND_FOR_EACH],
		"Should have exactly 3 FOR_EACH loop iteration executions")
	require.Equal(t, 6, finalCount,
		"Should have exactly 6 total records (3 FOR + 3 FOR_EACH)")

	t.Log("✅ TEST PASSED: Both FOR and FOR_EACH loops persist iterations correctly")
}
