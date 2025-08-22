package ritemapiexample_test

import (
	"context"
	"fmt"
	"sync"
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

// TestRapidConsecutiveMoves - Stress test: Many rapid moves in sequence
// Verify no examples ever vanish
func TestRapidConsecutiveMoves(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()
	
	setup := createStressTestSetup(t, base, 8)

	t.Log("=== Stress Test: Rapid Consecutive Moves ===")

	t.Run("100 rapid moves maintain integrity", func(t *testing.T) {
		const numMoves = 100
		
		initialCounts := countExamplesAllMethods(t, ctx, setup.rpcExample, base.Queries, setup.endpointID)
		expectedCount := initialCounts.rpcCount
		
		t.Logf("Starting rapid moves test with %d examples", expectedCount)
		
		start := time.Now()
		
		// Perform many rapid moves in sequence
		for i := 0; i < numMoves; i++ {
			srcIdx := i % expectedCount
			targetIdx := (i + 2) % expectedCount
			
			if srcIdx != targetIdx {
				performMove(t, ctx, setup.rpcExample, setup.endpointID, 
					setup.exampleIDs[srcIdx], setup.exampleIDs[targetIdx], 
					resourcesv1.MovePosition_MOVE_POSITION_AFTER)
				
				// Every 10 moves, verify integrity
				if i%10 == 0 {
					counts := countExamplesAllMethods(t, ctx, setup.rpcExample, base.Queries, setup.endpointID)
					if counts.rpcCount != expectedCount {
						t.Fatalf("Move %d: Lost examples! Expected %d, got %d", i, expectedCount, counts.rpcCount)
					}
					if counts.isolatedCount > 0 {
						t.Fatalf("Move %d: Found %d isolated examples", i, counts.isolatedCount)
					}
				}
			}
		}
		
		duration := time.Since(start)
		avgMoveTime := duration / numMoves
		
		// Final comprehensive verification
		finalCounts := countExamplesAllMethods(t, ctx, setup.rpcExample, base.Queries, setup.endpointID)
		
		if finalCounts.rpcCount != expectedCount {
			t.Fatalf("CRITICAL FAILURE: Lost examples after rapid moves! Expected %d, got %d", expectedCount, finalCounts.rpcCount)
		}
		if finalCounts.isolatedCount != 0 {
			t.Fatalf("CRITICAL FAILURE: Found %d isolated examples after rapid moves", finalCounts.isolatedCount)
		}
		
		t.Logf("✓ Rapid moves success: %d moves completed in %v (avg %v per move)", 
			numMoves, duration, avgMoveTime)
		
		// Performance check: average move should be < 10ms
		if avgMoveTime > 10*time.Millisecond {
			t.Errorf("Performance concern: Average move time %v > 10ms", avgMoveTime)
		}
	})

	t.Run("Alternating move patterns", func(t *testing.T) {
		const numCycles = 20
		
		// Alternating pattern: first->last, last->first, middle->edges, etc.
		for cycle := 0; cycle < numCycles; cycle++ {
			numExamples := len(setup.exampleIDs)
			
			switch cycle % 4 {
			case 0:
				// Move first to last position
				performMove(t, ctx, setup.rpcExample, setup.endpointID,
					setup.exampleIDs[0], setup.exampleIDs[numExamples-1],
					resourcesv1.MovePosition_MOVE_POSITION_AFTER)
			case 1:
				// Move last to first position  
				performMove(t, ctx, setup.rpcExample, setup.endpointID,
					setup.exampleIDs[numExamples-1], setup.exampleIDs[0],
					resourcesv1.MovePosition_MOVE_POSITION_BEFORE)
			case 2:
				// Move middle to beginning
				midIdx := numExamples / 2
				performMove(t, ctx, setup.rpcExample, setup.endpointID,
					setup.exampleIDs[midIdx], setup.exampleIDs[0],
					resourcesv1.MovePosition_MOVE_POSITION_BEFORE)
			case 3:
				// Move middle to end
				midIdx := numExamples / 2
				performMove(t, ctx, setup.rpcExample, setup.endpointID,
					setup.exampleIDs[midIdx], setup.exampleIDs[numExamples-1],
					resourcesv1.MovePosition_MOVE_POSITION_AFTER)
			}
			
			// Verify integrity after each cycle
			verifyCompleteDataIntegrity(t, ctx, setup.rpcExample, base.Queries, setup.endpointID, numExamples)
		}
		
		t.Logf("✓ Alternating patterns: %d cycles completed successfully", numCycles)
	})

	t.Log("=== Rapid Consecutive Moves Test: PASSED ===")
}

// TestConcurrentMoveOperations - Test concurrent moves on same endpoint
// Verify database integrity maintained
func TestConcurrentMoveOperations(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()
	
	setup := createStressTestSetup(t, base, 10)

	t.Log("=== Stress Test: Concurrent Move Operations ===")

	t.Run("Concurrent moves maintain database integrity", func(t *testing.T) {
		const numGoroutines = 8
		const movesPerGoroutine = 5
		
		initialCount := len(setup.exampleIDs)
		
		var wg sync.WaitGroup
		errorsCh := make(chan error, numGoroutines)
		
		wg.Add(numGoroutines)
		
		start := time.Now()
		
		// Launch concurrent move operations
		for g := 0; g < numGoroutines; g++ {
			go func(goroutineID int) {
				defer wg.Done()
				
				for m := 0; m < movesPerGoroutine; m++ {
					srcIdx := ((goroutineID * movesPerGoroutine) + m) % initialCount
					targetIdx := ((goroutineID * movesPerGoroutine) + m + 2) % initialCount
					
					if srcIdx != targetIdx {
						moveReq := connect.NewRequest(&examplev1.ExampleMoveRequest{
							EndpointId:      setup.endpointID.Bytes(),
							ExampleId:       setup.exampleIDs[srcIdx].Bytes(),
							Position:        resourcesv1.MovePosition_MOVE_POSITION_AFTER,
							TargetExampleId: setup.exampleIDs[targetIdx].Bytes(),
						})

						_, err := setup.rpcExample.ExampleMove(ctx, moveReq)
						if err != nil {
							errorsCh <- fmt.Errorf("goroutine %d move %d failed: %w", goroutineID, m, err)
							return
						}
					}
				}
			}(g)
		}
		
		// Wait for all goroutines to complete
		wg.Wait()
		close(errorsCh)
		
		duration := time.Since(start)
		
		// Check for errors
		for err := range errorsCh {
			t.Errorf("Concurrent move error: %v", err)
		}
		
		// Verify final integrity
		finalCounts := countExamplesAllMethods(t, ctx, setup.rpcExample, base.Queries, setup.endpointID)
		
		if finalCounts.rpcCount != initialCount {
			t.Fatalf("Concurrent moves corrupted data: expected %d examples, got %d", initialCount, finalCounts.rpcCount)
		}
		if finalCounts.isolatedCount != 0 {
			t.Fatalf("Concurrent moves created %d isolated examples", finalCounts.isolatedCount)
		}
		
		// Verify linked-list integrity
		verifyLinkedListIntegrity(t, ctx, base.Queries, setup.endpointID)
		
		totalMoves := numGoroutines * movesPerGoroutine
		t.Logf("✓ Concurrent moves success: %d concurrent moves in %v", totalMoves, duration)
		
		// Performance check: should handle concurrent load efficiently
		avgTimePerMove := duration / time.Duration(totalMoves)
		if avgTimePerMove > 20*time.Millisecond {
			t.Errorf("Concurrent performance concern: avg %v per move > 20ms", avgTimePerMove)
		}
	})

	t.Run("Concurrent moves with list operations", func(t *testing.T) {
		const numMoveWorkers = 4
		const numListWorkers = 3
		const operationsPerWorker = 10
		
		var wg sync.WaitGroup
		errorsCh := make(chan error, numMoveWorkers+numListWorkers)
		
		// Workers performing moves
		for w := 0; w < numMoveWorkers; w++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				
				for op := 0; op < operationsPerWorker; op++ {
					srcIdx := (workerID + op) % len(setup.exampleIDs)
					targetIdx := (workerID + op + 1) % len(setup.exampleIDs)
					
					if srcIdx != targetIdx {
						moveReq := connect.NewRequest(&examplev1.ExampleMoveRequest{
							EndpointId:      setup.endpointID.Bytes(),
							ExampleId:       setup.exampleIDs[srcIdx].Bytes(),
							Position:        resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
							TargetExampleId: setup.exampleIDs[targetIdx].Bytes(),
						})

						_, err := setup.rpcExample.ExampleMove(ctx, moveReq)
						if err != nil {
							errorsCh <- fmt.Errorf("move worker %d op %d failed: %w", workerID, op, err)
							return
						}
					}
					
					// Brief pause to allow interleaving with list operations
					time.Sleep(1 * time.Millisecond)
				}
			}(w)
		}
		
		// Workers performing list operations
		for w := 0; w < numListWorkers; w++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				
				for op := 0; op < operationsPerWorker; op++ {
					listReq := connect.NewRequest(&examplev1.ExampleListRequest{
						EndpointId: setup.endpointID.Bytes(),
					})
					
					listResp, err := setup.rpcExample.ExampleList(ctx, listReq)
					if err != nil {
						errorsCh <- fmt.Errorf("list worker %d op %d failed: %w", workerID, op, err)
						return
					}
					
					// Verify we always get the expected count
					if len(listResp.Msg.Items) != len(setup.exampleIDs) {
						errorsCh <- fmt.Errorf("list worker %d op %d: expected %d items, got %d", 
							workerID, op, len(setup.exampleIDs), len(listResp.Msg.Items))
						return
					}
					
					time.Sleep(2 * time.Millisecond)
				}
			}(w)
		}
		
		wg.Wait()
		close(errorsCh)
		
		// Check for errors
		for err := range errorsCh {
			t.Errorf("Concurrent operation error: %v", err)
		}
		
		// Final verification
		verifyCompleteDataIntegrity(t, ctx, setup.rpcExample, base.Queries, setup.endpointID, len(setup.exampleIDs))
		
		t.Log("✓ Concurrent moves with list operations: All operations completed successfully")
	})

	t.Log("=== Concurrent Move Operations Test: PASSED ===")
}

// TestEdgeCasePositions - Test all edge positions: first→last, last→first, etc.
// Verify no edge cases cause vanishing
func TestEdgeCasePositions(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()
	
	setup := createStressTestSetup(t, base, 5)

	t.Log("=== Stress Test: Edge Case Positions ===")

	t.Run("First to last position moves", func(t *testing.T) {
		numExamples := len(setup.exampleIDs)
		
		// Move first element to after last element multiple times
		for i := 0; i < 5; i++ {
			performMove(t, ctx, setup.rpcExample, setup.endpointID,
				setup.exampleIDs[0], setup.exampleIDs[numExamples-1],
				resourcesv1.MovePosition_MOVE_POSITION_AFTER)
			
			verifyCompleteDataIntegrity(t, ctx, setup.rpcExample, base.Queries, setup.endpointID, numExamples)
		}
		
		t.Log("✓ First to last position moves completed successfully")
	})

	t.Run("Last to first position moves", func(t *testing.T) {
		numExamples := len(setup.exampleIDs)
		
		// Move last element to before first element multiple times
		for i := 0; i < 5; i++ {
			performMove(t, ctx, setup.rpcExample, setup.endpointID,
				setup.exampleIDs[numExamples-1], setup.exampleIDs[0],
				resourcesv1.MovePosition_MOVE_POSITION_BEFORE)
			
			verifyCompleteDataIntegrity(t, ctx, setup.rpcExample, base.Queries, setup.endpointID, numExamples)
		}
		
		t.Log("✓ Last to first position moves completed successfully")
	})

	t.Run("Adjacent element swaps", func(t *testing.T) {
		numExamples := len(setup.exampleIDs)
		
		// Test swapping adjacent elements
		for i := 0; i < numExamples-1; i++ {
			// Move element i after element i+1
			performMove(t, ctx, setup.rpcExample, setup.endpointID,
				setup.exampleIDs[i], setup.exampleIDs[i+1],
				resourcesv1.MovePosition_MOVE_POSITION_AFTER)
			
			verifyCompleteDataIntegrity(t, ctx, setup.rpcExample, base.Queries, setup.endpointID, numExamples)
			
			// Move it back
			performMove(t, ctx, setup.rpcExample, setup.endpointID,
				setup.exampleIDs[i], setup.exampleIDs[i+1],
				resourcesv1.MovePosition_MOVE_POSITION_BEFORE)
			
			verifyCompleteDataIntegrity(t, ctx, setup.rpcExample, base.Queries, setup.endpointID, numExamples)
		}
		
		t.Log("✓ Adjacent element swaps completed successfully")
	})

	t.Run("Single element movements", func(t *testing.T) {
		numExamples := len(setup.exampleIDs)
		
		// Move single element through all positions
		movingElement := setup.exampleIDs[2] // Middle element
		
		// Move to every possible position
		for targetIdx := 0; targetIdx < numExamples; targetIdx++ {
			if targetIdx == 2 { // Skip moving to self
				continue
			}
			
			position := resourcesv1.MovePosition_MOVE_POSITION_AFTER
			if targetIdx == 0 {
				position = resourcesv1.MovePosition_MOVE_POSITION_BEFORE
			}
			
			performMove(t, ctx, setup.rpcExample, setup.endpointID,
				movingElement, setup.exampleIDs[targetIdx], position)
			
			verifyCompleteDataIntegrity(t, ctx, setup.rpcExample, base.Queries, setup.endpointID, numExamples)
		}
		
		t.Log("✓ Single element movements through all positions completed successfully")
	})

	t.Run("Boundary condition edge cases", func(t *testing.T) {
		numExamples := len(setup.exampleIDs)
		
		// Test various boundary conditions
		testCases := []struct {
			name      string
			srcIdx    int
			targetIdx int
			position  resourcesv1.MovePosition
		}{
			{"Move first before second", 0, 1, resourcesv1.MovePosition_MOVE_POSITION_BEFORE},
			{"Move last after second-to-last", numExamples - 1, numExamples - 2, resourcesv1.MovePosition_MOVE_POSITION_AFTER},
			{"Move second-to-last before last", numExamples - 2, numExamples - 1, resourcesv1.MovePosition_MOVE_POSITION_BEFORE},
			{"Move second after first", 1, 0, resourcesv1.MovePosition_MOVE_POSITION_AFTER},
		}
		
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				performMove(t, ctx, setup.rpcExample, setup.endpointID,
					setup.exampleIDs[tc.srcIdx], setup.exampleIDs[tc.targetIdx], tc.position)
				
				verifyCompleteDataIntegrity(t, ctx, setup.rpcExample, base.Queries, setup.endpointID, numExamples)
			})
		}
		
		t.Log("✓ Boundary condition edge cases completed successfully")
	})

	t.Log("=== Edge Case Positions Test: PASSED ===")
}

// TestLargeEndpointMoving - Test with 100+ examples in endpoint
// Verify performance and correctness scale
func TestLargeEndpointMoving(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()
	
	// Create large endpoint with 150 examples
	setup := createStressTestSetup(t, base, 150)

	t.Log("=== Stress Test: Large Endpoint Moving ===")

	t.Run("Large scale moves maintain performance", func(t *testing.T) {
		numExamples := len(setup.exampleIDs)
		t.Logf("Testing with %d examples", numExamples)
		
		// Test moves across the large dataset
		testMoves := []struct {
			name      string
			srcIdx    int
			targetIdx int
		}{
			{"Beginning to end", 0, numExamples - 1},
			{"End to beginning", numExamples - 1, 0},
			{"Quarter to three-quarter", numExamples / 4, (3 * numExamples) / 4},
			{"Middle to beginning", numExamples / 2, 0},
			{"Middle to end", numExamples / 2, numExamples - 1},
		}
		
		for _, tm := range testMoves {
			t.Run(tm.name, func(t *testing.T) {
				start := time.Now()
				
				performMove(t, ctx, setup.rpcExample, setup.endpointID,
					setup.exampleIDs[tm.srcIdx], setup.exampleIDs[tm.targetIdx],
					resourcesv1.MovePosition_MOVE_POSITION_AFTER)
				
				duration := time.Since(start)
				
				// Performance requirement: even with 150 examples, moves should be < 50ms
				if duration > 50*time.Millisecond {
					t.Errorf("Large scale move performance: %s took %v, expected < 50ms", tm.name, duration)
				} else {
					t.Logf("✓ %s completed in %v", tm.name, duration)
				}
				
				// Verify integrity (but skip full verification for performance)
				counts := countExamplesAllMethods(t, ctx, setup.rpcExample, base.Queries, setup.endpointID)
				if counts.rpcCount != numExamples || counts.isolatedCount != 0 {
					t.Fatalf("Large scale integrity failure: rpc=%d, isolated=%d", counts.rpcCount, counts.isolatedCount)
				}
			})
		}
	})

	t.Run("Large scale list performance", func(t *testing.T) {
		numExamples := len(setup.exampleIDs)
		
		// Test ExampleList performance with large dataset
		start := time.Now()
		
		listResp := callExampleList(t, ctx, setup.rpcExample, setup.endpointID)
		
		duration := time.Since(start)
		
		if len(listResp.Items) != numExamples {
			t.Fatalf("Large scale list: expected %d examples, got %d", numExamples, len(listResp.Items))
		}
		
		// Performance requirement: listing 150 examples should be < 200ms
		if duration > 200*time.Millisecond {
			t.Errorf("Large scale list performance: took %v, expected < 200ms", duration)
		} else {
			t.Logf("✓ Large scale list completed in %v (< 200ms requirement met)", duration)
		}
	})

	t.Run("Batch operations on large dataset", func(t *testing.T) {
		numExamples := len(setup.exampleIDs)
		const numBatchMoves = 20
		
		start := time.Now()
		
		// Perform batch of moves across the large dataset
		for i := 0; i < numBatchMoves; i++ {
			srcIdx := (i * 7) % numExamples
			targetIdx := ((i * 7) + 13) % numExamples
			
			if srcIdx != targetIdx {
				performMove(t, ctx, setup.rpcExample, setup.endpointID,
					setup.exampleIDs[srcIdx], setup.exampleIDs[targetIdx],
					resourcesv1.MovePosition_MOVE_POSITION_AFTER)
			}
		}
		
		duration := time.Since(start)
		avgBatchTime := duration / numBatchMoves
		
		// Verify final integrity
		counts := countExamplesAllMethods(t, ctx, setup.rpcExample, base.Queries, setup.endpointID)
		if counts.rpcCount != numExamples || counts.isolatedCount != 0 {
			t.Fatalf("Batch operations integrity failure: rpc=%d, isolated=%d", counts.rpcCount, counts.isolatedCount)
		}
		
		t.Logf("✓ Large scale batch operations: %d moves in %v (avg %v per move)", 
			numBatchMoves, duration, avgBatchTime)
		
		// Performance requirement: average batch move time should scale reasonably
		if avgBatchTime > 30*time.Millisecond {
			t.Errorf("Large scale batch performance concern: avg %v per move > 30ms", avgBatchTime)
		}
	})

	t.Log("=== Large Endpoint Moving Test: PASSED ===")
}

// Helper functions for stress tests

type stressTestSetup struct {
	rpcExample   ritemapiexample.ItemAPIExampleRPC
	authedCtx    context.Context
	endpointID   idwrap.IDWrap
	exampleIDs   []idwrap.IDWrap
	exampleNames []string
}

func createStressTestSetup(t *testing.T, base *testutil.BaseDBQueries, numExamples int) *stressTestSetup {
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
		Name:         "stress_test_endpoint",
		Url:          "https://api.stress-test.com/endpoint",
		Method:       "DELETE",
		CollectionID: collectionID,
		FolderID:     nil,
	}

	err := ias.CreateItemApi(ctx, endpoint)
	if err != nil {
		t.Fatalf("Failed to create endpoint: %v", err)
	}

	t.Logf("Creating %d examples for stress test...", numExamples)

	// Create examples with proper linked-list structure
	exampleIDs := make([]idwrap.IDWrap, numExamples)
	exampleNames := make([]string, numExamples)
	
	for i := 0; i < numExamples; i++ {
		exampleIDs[i] = idwrap.NewNow()
		exampleNames[i] = fmt.Sprintf("stress_example_%d", i+1)
		
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
		
		// Progress indicator for large datasets
		if numExamples >= 100 && i > 0 && i%50 == 0 {
			t.Logf("Created %d/%d examples...", i, numExamples)
		}
	}

	// Create RPC handler
	logChanMap := logconsole.NewLogChanMapWith(10000)
	rpcExample := ritemapiexample.New(base.DB, iaes, ias, ifs, ws, cs, us, hs, qs, bfs, bues, brs, erhs, ers, es, vs, as, ars, logChanMap)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	t.Logf("Stress test setup complete with %d examples", numExamples)

	return &stressTestSetup{
		rpcExample:   rpcExample,
		authedCtx:    authedCtx,
		endpointID:   endpointID,
		exampleIDs:   exampleIDs,
		exampleNames: exampleNames,
	}
}