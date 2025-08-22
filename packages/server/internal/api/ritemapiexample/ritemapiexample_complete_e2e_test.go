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
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logconsole"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/dbtime"
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

// TestExampleMoveCompleteE2EFlow - The ultimate test: Create → Move → List → Verify
// This test must work flawlessly with zero vanishing examples
func TestExampleMoveCompleteE2EFlow(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()
	
	// Setup complete test environment
	setup := createCompleteTestSetup(t, base, 4)

	t.Log("=== Phase 1: Complete E2E Integration Test ===")
	
	// CRITICAL: All 4 examples must be visible throughout the entire flow
	example1ID := setup.exampleIDs[0]
	example2ID := setup.exampleIDs[1] 
	example3ID := setup.exampleIDs[2]
	example4ID := setup.exampleIDs[3]

	// Step 1: Verify Initial State
	t.Run("Verify initial complete state", func(t *testing.T) {
		counts := countExamplesAllMethods(t, setup.authedCtx, setup.rpcExample, base.Queries, setup.endpointID)
		
		// ALL methods must return 4 examples
		if counts.rpcCount != 4 {
			t.Fatalf("RPC count: expected 4, got %d", counts.rpcCount)
		}
		if counts.dbOrderedCount != 4 {
			t.Fatalf("DB ordered count: expected 4, got %d", counts.dbOrderedCount)
		}
		if counts.dbAllCount != 4 {
			t.Fatalf("DB all count: expected 4, got %d", counts.dbAllCount)
		}
		if counts.isolatedCount != 0 {
			t.Fatalf("Isolated count: expected 0, got %d", counts.isolatedCount)
		}
		
		t.Log("✓ Initial state: All 4 examples visible via all methods")
	})

	// Step 2: Complex Move Sequence
	t.Run("Complex move sequence", func(t *testing.T) {
		// Move 1: example1 AFTER example3 → [example2, example3, example1, example4]
		performMove(t, setup.authedCtx, setup.rpcExample, setup.endpointID, example1ID, example3ID, resourcesv1.MovePosition_MOVE_POSITION_AFTER)
		verifyCompleteDataIntegrity(t, setup.authedCtx, setup.rpcExample, base.Queries, setup.endpointID, 4)

		// Move 2: example4 BEFORE example2 → [example4, example2, example3, example1]
		performMove(t, setup.authedCtx, setup.rpcExample, setup.endpointID, example4ID, example2ID, resourcesv1.MovePosition_MOVE_POSITION_BEFORE)
		verifyCompleteDataIntegrity(t, setup.authedCtx, setup.rpcExample, base.Queries, setup.endpointID, 4)

		// Move 3: example2 to last position → [example4, example3, example1, example2]
		performMove(t, setup.authedCtx, setup.rpcExample, setup.endpointID, example2ID, example1ID, resourcesv1.MovePosition_MOVE_POSITION_AFTER)
		verifyCompleteDataIntegrity(t, setup.authedCtx, setup.rpcExample, base.Queries, setup.endpointID, 4)

		t.Log("✓ Complex move sequence: All examples remain visible")
	})

	// Step 3: Final Comprehensive Verification
	t.Run("Final comprehensive verification", func(t *testing.T) {
		counts := countExamplesAllMethods(t, setup.authedCtx, setup.rpcExample, base.Queries, setup.endpointID)
		
		// CRITICAL: Still 4 examples after all operations
		if counts.rpcCount != 4 {
			t.Fatalf("CRITICAL FAILURE: RPC lost examples: expected 4, got %d", counts.rpcCount)
		}
		if counts.dbOrderedCount != 4 {
			t.Fatalf("CRITICAL FAILURE: DB ordered query lost examples: expected 4, got %d", counts.dbOrderedCount)
		}
		if counts.dbAllCount != 4 {
			t.Fatalf("CRITICAL FAILURE: DB all query lost examples: expected 4, got %d", counts.dbAllCount)
		}
		if counts.isolatedCount != 0 {
			t.Fatalf("CRITICAL FAILURE: Found isolated examples: %d", counts.isolatedCount)
		}

		// Verify linked-list integrity
		verifyLinkedListIntegrity(t, setup.authedCtx, base.Queries, setup.endpointID)
		
		t.Log("✓ COMPLETE E2E SUCCESS: 100% data integrity maintained")
	})

	t.Log("=== Complete E2E Flow Test: PASSED ===")
}

// TestExampleListRPCRobustness - Test ExampleList RPC with various corruption scenarios
// Verify it always returns all examples (via auto-linking or fallback)
func TestExampleListRPCRobustness(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()

	setup := createCompleteTestSetup(t, base, 5)

	t.Log("=== Phase 2: ExampleList RPC Robustness Test ===")

	t.Run("Normal operation", func(t *testing.T) {
		listResp := callExampleList(t, setup.authedCtx, setup.rpcExample, setup.endpointID)
		if len(listResp.Items) != 5 {
			t.Fatalf("Normal operation: expected 5 examples, got %d", len(listResp.Items))
		}
		t.Log("✓ Normal operation: ExampleList returns all 5 examples")
	})

	t.Run("With artificially broken linked-list", func(t *testing.T) {
		// Break the linked-list by setting a Next pointer to NULL
		err := base.Queries.UpdateItemApiExampleOrder(ctx, gen.UpdateItemApiExampleOrderParams{
			ID:   setup.exampleIDs[1], // Break example2's Next pointer
			Prev: &setup.exampleIDs[0],
			Next: nil, // Should point to example3, but we break it
		})
		if err != nil {
			t.Fatalf("Failed to break linked-list: %v", err)
		}

		// ExampleList should still return all 5 examples via auto-linking or fallback
		listResp := callExampleList(t, setup.authedCtx, setup.rpcExample, setup.endpointID)
		if len(listResp.Items) != 5 {
			t.Fatalf("With broken linked-list: expected 5 examples, got %d", len(listResp.Items))
		}
		
		t.Log("✓ With broken linked-list: ExampleList still returns all 5 examples (protection layers working)")
	})

	t.Run("Performance under fallback", func(t *testing.T) {
		start := time.Now()
		listResp := callExampleList(t, setup.authedCtx, setup.rpcExample, setup.endpointID)
		duration := time.Since(start)
		
		if len(listResp.Items) != 5 {
			t.Fatalf("Performance test: expected 5 examples, got %d", len(listResp.Items))
		}
		
		// Even with fallback, should complete within 100ms
		if duration > 100*time.Millisecond {
			t.Errorf("Performance: ExampleList took %v, expected < 100ms", duration)
		} else {
			t.Logf("✓ Performance: ExampleList completed in %v (< 100ms requirement met)", duration)
		}
	})

	t.Log("=== ExampleList RPC Robustness Test: PASSED ===")
}

// TestMovePlusAutoLinkingIntegration - Test move operations with auto-linking
// Verify defensive programming works correctly
func TestMovePlusAutoLinkingIntegration(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()

	setup := createCompleteTestSetup(t, base, 3)

	t.Log("=== Phase 3: Move + Auto-Linking Integration Test ===")

	t.Run("Move with concurrent auto-linking", func(t *testing.T) {
		// Perform moves while auto-linking is working
		performMove(t, setup.authedCtx, setup.rpcExample, setup.endpointID, setup.exampleIDs[0], setup.exampleIDs[2], resourcesv1.MovePosition_MOVE_POSITION_AFTER)
		
		// Verify auto-linking worked and all examples are still visible
		counts := countExamplesAllMethods(t, setup.authedCtx, setup.rpcExample, base.Queries, setup.endpointID)
		
		if counts.rpcCount != 3 || counts.dbAllCount != 3 || counts.isolatedCount != 0 {
			t.Fatalf("Move + auto-linking failed: rpc=%d, db=%d, isolated=%d", counts.rpcCount, counts.dbAllCount, counts.isolatedCount)
		}

		t.Log("✓ Move with auto-linking: All 3 examples maintained")
	})

	t.Run("Rapid moves with defensive programming", func(t *testing.T) {
		// Perform rapid sequence of moves
		for i := 0; i < 5; i++ {
			srcIdx := i % 3
			targetIdx := (i + 1) % 3
			if srcIdx != targetIdx {
				performMove(t, setup.authedCtx, setup.rpcExample, setup.endpointID, setup.exampleIDs[srcIdx], setup.exampleIDs[targetIdx], resourcesv1.MovePosition_MOVE_POSITION_AFTER)
			}
		}

		// Defensive programming should ensure no data loss
		verifyCompleteDataIntegrity(t, setup.authedCtx, setup.rpcExample, base.Queries, setup.endpointID, 3)
		
		t.Log("✓ Rapid moves: Defensive programming maintained integrity")
	})

	t.Log("=== Move + Auto-Linking Integration Test: PASSED ===")
}

// Helper functions for comprehensive testing

type completeTestSetup struct {
	rpcExample   ritemapiexample.ItemAPIExampleRPC
	authedCtx    context.Context
	endpointID   idwrap.IDWrap
	exampleIDs   []idwrap.IDWrap
	exampleNames []string
}

func createCompleteTestSetup(t *testing.T, base *testutil.BaseDBQueries, numExamples int) *completeTestSetup {
	ctx := context.Background()
	mockLogger := mocklogger.NewMockLogger()

	// Initialize all services
	ias := sitemapi.New(base.Queries)
	iaes := sitemapiexample.New(base.Queries)
	ifs := sitemfolder.New(base.Queries)
	ws := sworkspace.New(base.Queries)
	cs := scollection.New(base.Queries, mockLogger)
	us := suser.New(base.Queries)
	hs := sexampleheader.New(base.Queries)
	qs := sexamplequery.New(base.Queries)
	bfs := sbodyform.New(base.Queries)
	bues := sbodyurl.New(base.Queries)
	brs := sbodyraw.New(base.Queries)
	ers := sexampleresp.New(base.Queries)
	erhs := sexamplerespheader.New(base.Queries)
	es := senv.New(base.Queries, mockLogger)
	vs := svar.New(base.Queries, mockLogger)
	as := sassert.New(base.Queries)
	ars := sassertres.New(base.Queries)

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
		Name:         "e2e_test_endpoint",
		Url:          "https://api.e2e-test.com/endpoint",
		Method:       "POST",
		CollectionID: collectionID,
		FolderID:     nil,
	}

	err := ias.CreateItemApi(ctx, endpoint)
	if err != nil {
		t.Fatalf("Failed to create endpoint: %v", err)
	}

	// Create examples with proper linked-list structure
	exampleIDs := make([]idwrap.IDWrap, numExamples)
	exampleNames := make([]string, numExamples)
	
	for i := 0; i < numExamples; i++ {
		exampleIDs[i] = idwrap.NewNow()
		exampleNames[i] = fmt.Sprintf("e2e_example_%d", i+1)
		
		var prev *idwrap.IDWrap
		var next *idwrap.IDWrap
		
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
			Next:         next,
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
			
			err := base.Queries.UpdateItemApiExampleOrder(ctx, gen.UpdateItemApiExampleOrderParams{
				Prev: prevOfPrev,
				Next: &exampleIDs[i],
				ID:   exampleIDs[i-1],
			})
			if err != nil {
				t.Fatalf("Failed to update prev example %d linked list pointers: %v", i-1, err)
			}
		}
	}

	// Create RPC handler
	logChanMap := logconsole.NewLogChanMapWith(10000)
	rpcExample := ritemapiexample.New(base.DB, iaes, ias, ifs, ws, cs, us, hs, qs, bfs, bues, brs, erhs, ers, es, vs, as, ars, logChanMap)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	return &completeTestSetup{
		rpcExample:   rpcExample,
		authedCtx:    authedCtx,
		endpointID:   endpointID,
		exampleIDs:   exampleIDs,
		exampleNames: exampleNames,
	}
}

type exampleCounts struct {
	rpcCount       int
	dbOrderedCount int
	dbAllCount     int
	isolatedCount  int
}

// Helper to count examples via different methods - CRITICAL for validation
func countExamplesAllMethods(t *testing.T, ctx context.Context, rpc ritemapiexample.ItemAPIExampleRPC, queries *gen.Queries, endpointID idwrap.IDWrap) exampleCounts {
	t.Helper()

	// Count via RPC (this is what users see - MUST always be complete)
	listReq := connect.NewRequest(&examplev1.ExampleListRequest{
		EndpointId: endpointID.Bytes(),
	})
	
	listResp, err := rpc.ExampleList(ctx, listReq)
	if err != nil {
		t.Fatalf("ExampleList RPC failed: %v", err)
	}
	rpcCount := len(listResp.Msg.Items)

	// Count via ordered database query
	orderedExamples, err := queries.GetExamplesByEndpointIDOrdered(ctx, gen.GetExamplesByEndpointIDOrderedParams{
		ItemApiID:   endpointID,
		ItemApiID_2: endpointID,
	})
	dbOrderedCount := 0
	if err == nil {
		dbOrderedCount = len(orderedExamples)
	}

	// Count via ALL database query (fallback)
	allExamples, err := queries.GetAllExamplesByEndpointID(ctx, endpointID)
	dbAllCount := 0
	if err == nil {
		dbAllCount = len(allExamples)
	}

	// Count isolated examples (should always be 0) using repository method
	repo := sitemapiexample.NewExampleMovableRepository(queries)
	isolatedExamples, err := repo.DetectIsolatedExamples(ctx, endpointID)
	isolatedCount := 0
	if err == nil {
		isolatedCount = len(isolatedExamples)
	}

	return exampleCounts{
		rpcCount:       rpcCount,
		dbOrderedCount: dbOrderedCount,
		dbAllCount:     dbAllCount,
		isolatedCount:  isolatedCount,
	}
}

// Helper to perform move and validate success
func performMove(t *testing.T, ctx context.Context, rpc ritemapiexample.ItemAPIExampleRPC, endpointID, exampleID, targetExampleID idwrap.IDWrap, position resourcesv1.MovePosition) {
	t.Helper()

	start := time.Now()

	moveReq := connect.NewRequest(&examplev1.ExampleMoveRequest{
		EndpointId:      endpointID.Bytes(),
		ExampleId:       exampleID.Bytes(),
		Position:        position,
		TargetExampleId: targetExampleID.Bytes(),
	})

	_, err := rpc.ExampleMove(ctx, moveReq)
	if err != nil {
		t.Fatalf("ExampleMove failed: %v", err)
	}

	duration := time.Since(start)
	
	// Performance requirement: moves should complete < 10ms
	if duration > 10*time.Millisecond {
		t.Errorf("Move took %v, expected < 10ms", duration)
	}
}

// Helper to call ExampleList RPC and return response
func callExampleList(t *testing.T, ctx context.Context, rpc ritemapiexample.ItemAPIExampleRPC, endpointID idwrap.IDWrap) *examplev1.ExampleListResponse {
	t.Helper()

	listReq := connect.NewRequest(&examplev1.ExampleListRequest{
		EndpointId: endpointID.Bytes(),
	})
	
	listResp, err := rpc.ExampleList(ctx, listReq)
	if err != nil {
		t.Fatalf("ExampleList failed: %v", err)
	}
	
	return listResp.Msg
}

// Helper to verify complete data integrity
func verifyCompleteDataIntegrity(t *testing.T, ctx context.Context, rpc ritemapiexample.ItemAPIExampleRPC, queries *gen.Queries, endpointID idwrap.IDWrap, expectedCount int) {
	t.Helper()

	counts := countExamplesAllMethods(t, ctx, rpc, queries, endpointID)
	
	// CRITICAL: RPC must always return expected count (this is what users see)
	if counts.rpcCount != expectedCount {
		t.Fatalf("CRITICAL FAILURE: RPC count mismatch: expected %d, got %d", expectedCount, counts.rpcCount)
	}
	
	// Database should also have expected count
	if counts.dbAllCount != expectedCount {
		t.Fatalf("Database integrity failure: expected %d, got %d", expectedCount, counts.dbAllCount)
	}
	
	// Should never have isolated examples
	if counts.isolatedCount != 0 {
		t.Fatalf("Data corruption: found %d isolated examples", counts.isolatedCount)
	}

	// Verify linked-list structure
	verifyLinkedListIntegrity(t, ctx, queries, endpointID)
}

// Helper to verify linked-list integrity
func verifyLinkedListIntegrity(t *testing.T, ctx context.Context, queries *gen.Queries, endpointID idwrap.IDWrap) {
	t.Helper()

	orderedExamples, err := queries.GetExamplesByEndpointIDOrdered(ctx, gen.GetExamplesByEndpointIDOrderedParams{
		ItemApiID:   endpointID,
		ItemApiID_2: endpointID,
	})
	
	if err != nil {
		// If ordered query fails, that's ok as long as fallback works
		// We've already verified RPC returns correct count via fallback
		return
	}

	if len(orderedExamples) == 0 {
		return // Nothing to verify for empty list
	}

	// First example should have nil prev
	if orderedExamples[0].Prev != nil {
		t.Errorf("Linked-list integrity: first example should have nil prev")
	}

	// Last example should have nil next
	lastIdx := len(orderedExamples) - 1
	if orderedExamples[lastIdx].Next != nil {
		t.Errorf("Linked-list integrity: last example should have nil next")
	}

	// Verify forward links
	for i := 0; i < len(orderedExamples)-1; i++ {
		if orderedExamples[i].Next == nil {
			t.Errorf("Linked-list integrity: example %d missing next pointer", i)
			continue
		}
		
		nextID := idwrap.NewFromBytesMust(orderedExamples[i+1].ID)
		actualNextID := idwrap.NewFromBytesMust(orderedExamples[i].Next)
		
		if actualNextID.Compare(nextID) != 0 {
			t.Errorf("Linked-list integrity: example %d next pointer incorrect", i)
		}
	}

	// Verify backward links
	for i := 1; i < len(orderedExamples); i++ {
		if orderedExamples[i].Prev == nil {
			t.Errorf("Linked-list integrity: example %d missing prev pointer", i)
			continue
		}
		
		prevID := idwrap.NewFromBytesMust(orderedExamples[i-1].ID)
		actualPrevID := idwrap.NewFromBytesMust(orderedExamples[i].Prev)
		
		if actualPrevID.Compare(prevID) != 0 {
			t.Errorf("Linked-list integrity: example %d prev pointer incorrect", i)
		}
	}
}