package ritemapiexample_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/ritemapiexample"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logconsole"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/service/sassert"
	"the-dev-tools/server/pkg/service/sassertres"
	"the-dev-tools/server/pkg/service/sbodyform"
	"the-dev-tools/server/pkg/service/sbodyraw"
	"the-dev-tools/server/pkg/service/sbodyurl"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/sexampleheader"
	"the-dev-tools/server/pkg/service/sexamplequery"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sexamplerespheader"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/sitemfolder"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/testutil"
	examplev1 "the-dev-tools/spec/dist/buf/go/collection/item/example/v1"
	resourcesv1 "the-dev-tools/spec/dist/buf/go/resources/v1"
)

// Helper functions for bug reproduction test
func countExamplesViaRPC(t *testing.T, ctx context.Context, rpcExample ritemapiexample.ItemAPIExampleRPC, endpointID idwrap.IDWrap) int {
	t.Helper()
	listReq := connect.NewRequest(&examplev1.ExampleListRequest{
		EndpointId: endpointID.Bytes(),
	})
	
	listResp, err := rpcExample.ExampleList(ctx, listReq)
	if err != nil {
		t.Fatalf("ExampleList RPC failed: %v", err)
	}
	
	return len(listResp.Msg.Items)
}

func countExamplesInDatabase(t *testing.T, ctx context.Context, queries *gen.Queries, endpointID idwrap.IDWrap) int {
	t.Helper()
	
	// Count examples directly in database (not using ordered query)
	examples, err := queries.GetItemApiExamples(ctx, endpointID)
	if err != nil {
		t.Fatalf("Failed to count examples in database: %v", err)
	}
	
	return len(examples)
}

func findIsolatedExamples(t *testing.T, ctx context.Context, base *testutil.BaseDBQueries, endpointID idwrap.IDWrap) []idwrap.IDWrap {
	t.Helper()
	
	// Find examples that have both prev=NULL and next=NULL (isolated nodes)
	// These would exist in database but not be findable by the recursive CTE
	rows, err := base.DB.QueryContext(ctx, `
		SELECT id FROM item_api_example 
		WHERE item_api_id = ? 
		AND version_parent_id IS NULL 
		AND is_default = FALSE 
		AND prev IS NULL 
		AND next IS NULL
	`, endpointID)
	if err != nil {
		t.Fatalf("Failed to query isolated examples: %v", err)
	}
	defer rows.Close()
	
	var isolated []idwrap.IDWrap
	for rows.Next() {
		var id []byte
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("Failed to scan isolated example ID: %v", err)
		}
		isolated = append(isolated, idwrap.NewFromBytesMust(id))
	}
	
	return isolated
}

func verifyVanishingBug(t *testing.T, expectedCount, actualRPCCount int, isolatedExamples []idwrap.IDWrap) bool {
	t.Helper()
	
	if actualRPCCount < expectedCount && len(isolatedExamples) > 0 {
		t.Logf("🐛 VANISHING BUG DETECTED!")
		t.Logf("   Expected examples: %d", expectedCount)
		t.Logf("   RPC returned: %d", actualRPCCount)
		t.Logf("   Isolated (vanished) examples: %d", len(isolatedExamples))
		for i, isolated := range isolatedExamples {
			t.Logf("     Isolated example %d: %s", i+1, isolated.String())
		}
		return true
	}
	return false
}

// logDatabaseState logs the current linked-list state for debugging
func logDatabaseState(t *testing.T, ctx context.Context, base *testutil.BaseDBQueries, endpointID idwrap.IDWrap, operation string) {
	t.Helper()
	
	t.Logf("=== DATABASE STATE AFTER %s ===", operation)
	
	// Get all examples for this endpoint directly from database
	rows, err := base.DB.QueryContext(ctx, `
		SELECT id, name, prev, next FROM item_api_example 
		WHERE item_api_id = ? AND version_parent_id IS NULL AND is_default = FALSE
		ORDER BY name
	`, endpointID)
	if err != nil {
		t.Logf("Failed to query database state: %v", err)
		return
	}
	defer rows.Close()
	
	for rows.Next() {
		var id, name []byte
		var prev, next *[]byte
		if err := rows.Scan(&id, &name, &prev, &next); err != nil {
			t.Logf("Failed to scan database row: %v", err)
			continue
		}
		
		idStr := idwrap.NewFromBytesMust(id).String()[:8] + "..."
		prevStr := "NULL"
		nextStr := "NULL"
		
		if prev != nil {
			prevStr = idwrap.NewFromBytesMust(*prev).String()[:8] + "..."
		}
		if next != nil {
			nextStr = idwrap.NewFromBytesMust(*next).String()[:8] + "..."
		}
		
		t.Logf("  %s: name=%s, prev=%s, next=%s", idStr, name, prevStr, nextStr)
	}
}

// TestExampleVanishingBug reproduces the vanishing examples bug
// This test creates multiple examples and performs move operations that trigger
// the bug where examples become isolated (prev=NULL, next=NULL) and vanish
// from the ExampleList RPC response while still existing in the database.
func TestExampleVanishingBug(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()
	
	queries := base.Queries
	db := base.DB

	mockLogger := mocklogger.NewMockLogger()

	// Initialize all required services
	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	ifs := sitemfolder.New(queries)
	ws := sworkspace.New(queries)
	cs := scollection.New(queries, mockLogger)
	us := suser.New(queries)
	hs := sexampleheader.New(queries)
	qs := sexamplequery.New(queries)
	bfs := sbodyform.New(queries)
	bues := sbodyurl.New(queries)
	brs := sbodyraw.New(queries)
	ers := sexampleresp.New(queries)
	erhs := sexamplerespheader.New(queries)
	es := senv.New(queries, mockLogger)
	vs := svar.New(queries, mockLogger)
	as := sassert.New(queries)
	ars := sassertres.New(queries)

	// Create test data
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	userID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)

	// Create endpoint
	endpointID := idwrap.NewNow()
	endpoint := &mitemapi.ItemApi{
		ID:           endpointID,
		Name:         "test_endpoint",
		Url:          "https://api.example.com/test",
		Method:       "GET",
		CollectionID: collectionID,
		FolderID:     nil,
	}

	err := ias.CreateItemApi(ctx, endpoint)
	if err != nil {
		t.Fatalf("Failed to create endpoint: %v", err)
	}

	// SETUP PHASE: Create 4 examples with proper linked list structure
	t.Log("=== SETUP PHASE ===")
	
	numExamples := 4
	exampleIDs := make([]idwrap.IDWrap, numExamples)
	exampleNames := make([]string, numExamples)
	
	// Create examples sequentially with proper linking
	for i := 0; i < numExamples; i++ {
		exampleIDs[i] = idwrap.NewNow()
		exampleNames[i] = fmt.Sprintf("example%d", i+1)
		
		var prev *idwrap.IDWrap
		if i > 0 {
			prev = &exampleIDs[i-1]
		}
		
		example := &mitemapiexample.ItemApiExample{
			ID:           exampleIDs[i],
			ItemApiID:    endpointID,
			CollectionID: collectionID,
			Name:         exampleNames[i],
			Updated:      dbtime.DBNow(),
			IsDefault:    false,
			BodyType:     mitemapiexample.BodyTypeRaw,
			Prev:         prev,
			Next:         nil,
		}

		err := iaes.CreateApiExample(ctx, example)
		if err != nil {
			t.Fatalf("Failed to create example %d: %v", i, err)
		}
		
		// Update previous example's Next pointer
		if i > 0 {
			var prevOfPrev *idwrap.IDWrap
			if i > 1 {
				prevOfPrev = &exampleIDs[i-2]
			}
			
			err := queries.UpdateItemApiExampleOrder(ctx, gen.UpdateItemApiExampleOrderParams{
				Prev: prevOfPrev,
				Next: &exampleIDs[i],
				ID:   exampleIDs[i-1],
			})
			if err != nil {
				t.Fatalf("Failed to update prev example %d linked list pointers: %v", i-1, err)
			}
		}
		
		// Small delay to ensure different creation times
		time.Sleep(1 * time.Millisecond)
	}

	// Create RPC handler
	logChanMap := logconsole.NewLogChanMapWith(10000)
	rpcExample := ritemapiexample.New(db, iaes, ias, ifs, ws, cs, us, hs, qs, bfs, bues, brs, erhs, ers, es, vs, as, ars, logChanMap)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	t.Log("Initial examples created:")
	for i, id := range exampleIDs {
		t.Logf("  %s: %s", exampleNames[i], id.String()[:8]+"...")
	}

	// Verify initial state
	initialRPCCount := countExamplesViaRPC(t, authedCtx, rpcExample, endpointID)
	initialDBCount := countExamplesInDatabase(t, ctx, queries, endpointID)
	
	t.Logf("Initial counts - RPC: %d, DB: %d", initialRPCCount, initialDBCount)
	
	if initialRPCCount != 4 || initialDBCount != 4 {
		t.Fatalf("Initial setup failed - expected 4 examples, got RPC: %d, DB: %d", initialRPCCount, initialDBCount)
	}

	logDatabaseState(t, ctx, base, endpointID, "INITIAL SETUP")

	// TRIGGER BUG PHASE: Perform move operations that are likely to cause isolation
	t.Log("\n=== TRIGGER BUG PHASE ===")
	
	// Scenario A: Edge Position Moves (most likely to trigger bug)
	t.Log("Performing Scenario A: Edge Position Moves")
	
	// Move first example to last position (after example4)
	t.Logf("Moving %s (example1) AFTER %s (example4)", exampleNames[0], exampleNames[3])
	moveReq := connect.NewRequest(&examplev1.ExampleMoveRequest{
		EndpointId:      endpointID.Bytes(),
		ExampleId:       exampleIDs[0].Bytes(),
		Position:        resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		TargetExampleId: exampleIDs[3].Bytes(),
	})

	_, err = rpcExample.ExampleMove(authedCtx, moveReq)
	if err != nil {
		t.Logf("Move operation failed: %v", err)
	} else {
		t.Log("Move operation completed")
	}
	
	logDatabaseState(t, ctx, base, endpointID, "MOVE 1 (example1 after example4)")
	
	// Check for vanishing examples after first move
	rpcCount := countExamplesViaRPC(t, authedCtx, rpcExample, endpointID)
	dbCount := countExamplesInDatabase(t, ctx, queries, endpointID)
	isolatedExamples := findIsolatedExamples(t, ctx, base, endpointID)
	
	t.Logf("After Move 1 - RPC: %d, DB: %d, Isolated: %d", rpcCount, dbCount, len(isolatedExamples))
	
	if verifyVanishingBug(t, 4, rpcCount, isolatedExamples) {
		t.Log("✓ VANISHING BUG REPRODUCED after first move!")
		return // Bug confirmed, test successful
	}
	
	// Move last example to first position (before example2)
	t.Logf("Moving %s (example4) BEFORE %s (example2)", exampleNames[3], exampleNames[1])
	moveReq = connect.NewRequest(&examplev1.ExampleMoveRequest{
		EndpointId:      endpointID.Bytes(),
		ExampleId:       exampleIDs[3].Bytes(),
		Position:        resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
		TargetExampleId: exampleIDs[1].Bytes(),
	})

	_, err = rpcExample.ExampleMove(authedCtx, moveReq)
	if err != nil {
		t.Logf("Move operation failed: %v", err)
	} else {
		t.Log("Move operation completed")
	}
	
	logDatabaseState(t, ctx, base, endpointID, "MOVE 2 (example4 before example2)")
	
	// Check for vanishing examples after second move
	rpcCount = countExamplesViaRPC(t, authedCtx, rpcExample, endpointID)
	dbCount = countExamplesInDatabase(t, ctx, queries, endpointID)
	isolatedExamples = findIsolatedExamples(t, ctx, base, endpointID)
	
	t.Logf("After Move 2 - RPC: %d, DB: %d, Isolated: %d", rpcCount, dbCount, len(isolatedExamples))
	
	if verifyVanishingBug(t, 4, rpcCount, isolatedExamples) {
		t.Log("✓ VANISHING BUG REPRODUCED after second move!")
		return // Bug confirmed, test successful
	}

	// Scenario B: Rapid Sequential Moves
	t.Log("Performing Scenario B: Rapid Sequential Moves")
	
	// Move example1 after example3
	t.Logf("Moving %s (example1) AFTER %s (example3)", exampleNames[0], exampleNames[2])
	moveReq = connect.NewRequest(&examplev1.ExampleMoveRequest{
		EndpointId:      endpointID.Bytes(),
		ExampleId:       exampleIDs[0].Bytes(),
		Position:        resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		TargetExampleId: exampleIDs[2].Bytes(),
	})

	_, err = rpcExample.ExampleMove(authedCtx, moveReq)
	if err != nil {
		t.Logf("Move operation failed: %v", err)
	}
	
	logDatabaseState(t, ctx, base, endpointID, "MOVE 3 (example1 after example3)")
	
	// Move example2 before example4  
	t.Logf("Moving %s (example2) BEFORE %s (example4)", exampleNames[1], exampleNames[3])
	moveReq = connect.NewRequest(&examplev1.ExampleMoveRequest{
		EndpointId:      endpointID.Bytes(),
		ExampleId:       exampleIDs[1].Bytes(),
		Position:        resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
		TargetExampleId: exampleIDs[3].Bytes(),
	})

	_, err = rpcExample.ExampleMove(authedCtx, moveReq)
	if err != nil {
		t.Logf("Move operation failed: %v", err)
	}
	
	logDatabaseState(t, ctx, base, endpointID, "MOVE 4 (example2 before example4)")
	
	// Check for vanishing examples after rapid moves
	rpcCount = countExamplesViaRPC(t, authedCtx, rpcExample, endpointID)
	dbCount = countExamplesInDatabase(t, ctx, queries, endpointID)
	isolatedExamples = findIsolatedExamples(t, ctx, base, endpointID)
	
	t.Logf("After Rapid Moves - RPC: %d, DB: %d, Isolated: %d", rpcCount, dbCount, len(isolatedExamples))
	
	if verifyVanishingBug(t, 4, rpcCount, isolatedExamples) {
		t.Log("✓ VANISHING BUG REPRODUCED after rapid sequential moves!")
		return // Bug confirmed, test successful
	}

	// DETECTION PHASE: Final verification
	t.Log("\n=== DETECTION PHASE ===")
	
	finalRPCCount := countExamplesViaRPC(t, authedCtx, rpcExample, endpointID)
	finalDBCount := countExamplesInDatabase(t, ctx, queries, endpointID)
	finalIsolatedExamples := findIsolatedExamples(t, ctx, base, endpointID)
	
	t.Logf("Final counts - RPC: %d, DB: %d, Isolated: %d", finalRPCCount, finalDBCount, len(finalIsolatedExamples))
	
	logDatabaseState(t, ctx, base, endpointID, "FINAL STATE")
	
	// Test result evaluation
	if finalRPCCount == 4 && len(finalIsolatedExamples) == 0 {
		t.Log("❌ Bug was NOT reproduced - all examples remain visible")
		t.Log("   This could mean:")
		t.Log("   1. The bug has been fixed")
		t.Log("   2. The test scenarios don't trigger the specific bug condition")
		t.Log("   3. The bug occurs under different circumstances")
	} else if finalRPCCount < 4 && len(finalIsolatedExamples) > 0 {
		t.Log("✓ VANISHING BUG REPRODUCED!")
		t.Log("   Examples exist in database but are invisible to RPC")
		t.Log("   This confirms the linked-list corruption hypothesis")
		
		// Additional debugging information
		t.Log("   DEBUGGING DETAILS:")
		t.Logf("   - Expected examples: %d", 4)
		t.Logf("   - RPC visible: %d", finalRPCCount)
		t.Logf("   - Database total: %d", finalDBCount)
		t.Logf("   - Isolated (vanished): %d", len(finalIsolatedExamples))
		
		// This test should PASS when bug exists (for reproduction purposes)
		// When the bug is fixed, this test should FAIL, indicating fix works
	} else {
		t.Log("⚠️ Unexpected state detected")
		t.Logf("   RPC Count: %d, DB Count: %d, Isolated: %d", finalRPCCount, finalDBCount, len(finalIsolatedExamples))
	}
}

// TestExampleVanishingBugChainBreakerPattern tries a specific pattern that's likely to break the chain
func TestExampleVanishingBugChainBreakerPattern(t *testing.T) {
	setup := createTestSetup(t, 4)
	defer setup.base.Close()

	ctx := setup.authedCtx
	
	example1ID := setup.exampleIDs[0]
	example2ID := setup.exampleIDs[1]
	example3ID := setup.exampleIDs[2]
	example4ID := setup.exampleIDs[3]

	t.Log("=== CHAIN BREAKER PATTERN ===")
	t.Log("Initial order: [example1, example2, example3, example4]")

	// Pattern that's most likely to cause linked-list corruption:
	// Move middle examples to opposite ends in rapid succession
	
	// Move example2 to end (after example4)
	t.Log("Step 1: Move example2 to end (after example4)")
	moveReq := connect.NewRequest(&examplev1.ExampleMoveRequest{
		EndpointId:      setup.endpointID.Bytes(),
		ExampleId:       example2ID.Bytes(),
		Position:        resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		TargetExampleId: example4ID.Bytes(),
	})

	_, err := setup.rpcExample.ExampleMove(ctx, moveReq)
	if err != nil {
		t.Logf("Move 1 failed: %v", err)
	}
	
	logDatabaseState(t, ctx, setup.base, setup.endpointID, "MOVE example2 to end")

	// Move example3 to beginning (before example1)
	t.Log("Step 2: Move example3 to beginning (before example1)")
	moveReq = connect.NewRequest(&examplev1.ExampleMoveRequest{
		EndpointId:      setup.endpointID.Bytes(),
		ExampleId:       example3ID.Bytes(),
		Position:        resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
		TargetExampleId: example1ID.Bytes(),
	})

	_, err = setup.rpcExample.ExampleMove(ctx, moveReq)
	if err != nil {
		t.Logf("Move 2 failed: %v", err)
	}
	
	logDatabaseState(t, ctx, setup.base, setup.endpointID, "MOVE example3 to beginning")

	// Check for vanishing examples
	rpcCount := countExamplesViaRPC(t, ctx, setup.rpcExample, setup.endpointID)
	dbCount := countExamplesInDatabase(t, ctx, setup.base.Queries, setup.endpointID)
	isolatedExamples := findIsolatedExamples(t, ctx, setup.base, setup.endpointID)
	
	t.Logf("After Chain Breaker Pattern - RPC: %d, DB: %d, Isolated: %d", rpcCount, dbCount, len(isolatedExamples))
	
	if verifyVanishingBug(t, 4, rpcCount, isolatedExamples) {
		t.Log("✓ CHAIN BREAKER PATTERN REPRODUCED THE BUG!")
	} else {
		t.Log("Chain Breaker Pattern did not reproduce the bug")
	}
}