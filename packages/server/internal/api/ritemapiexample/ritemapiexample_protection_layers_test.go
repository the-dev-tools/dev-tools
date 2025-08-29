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

// TestAtomicMovesPrevention - Verify atomic moves prevent isolation (Layer 1: Prevention)
func TestAtomicMovesPrevention(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()
	
	setup := createProtectionTestSetup(t, base, 3)

	t.Log("=== Layer 1: Atomic Moves Prevention Test ===")

	t.Run("Atomic transaction prevents partial moves", func(t *testing.T) {
		initialCounts := countExamplesAllMethods(t, setup.authedCtx, setup.rpcExample, base.Queries, setup.endpointID)
		
		// Perform move operation
		moveReq := connect.NewRequest(&examplev1.ExampleMoveRequest{
			EndpointId:      setup.endpointID.Bytes(),
			ExampleId:       setup.exampleIDs[0].Bytes(),
			Position:        resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetExampleId: setup.exampleIDs[2].Bytes(),
		})

		_, err := setup.rpcExample.ExampleMove(setup.authedCtx, moveReq)
		if err != nil {
			t.Fatalf("Move failed: %v", err)
		}

		// Verify atomic operation completed successfully
		finalCounts := countExamplesAllMethods(t, setup.authedCtx, setup.rpcExample, base.Queries, setup.endpointID)
		
		// ALL counts should remain identical (no examples lost or isolated)
		if finalCounts.rpcCount != initialCounts.rpcCount {
			t.Fatalf("Atomic prevention failed: RPC count changed from %d to %d", initialCounts.rpcCount, finalCounts.rpcCount)
		}
		if finalCounts.dbAllCount != initialCounts.dbAllCount {
			t.Fatalf("Atomic prevention failed: DB count changed from %d to %d", initialCounts.dbAllCount, finalCounts.dbAllCount)
		}
		if finalCounts.isolatedCount != 0 {
			t.Fatalf("Atomic prevention failed: Created %d isolated examples", finalCounts.isolatedCount)
		}

		t.Log("✓ Layer 1 Prevention: Atomic moves successfully prevented isolation")
	})

	t.Run("Transaction rollback on failure maintains integrity", func(t *testing.T) {
		// Try to perform an invalid move (move to non-existent target)
		invalidTargetID := idwrap.NewNow() // Non-existent ID
		
		moveReq := connect.NewRequest(&examplev1.ExampleMoveRequest{
			EndpointId:      setup.endpointID.Bytes(),
			ExampleId:       setup.exampleIDs[0].Bytes(),
			Position:        resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetExampleId: invalidTargetID.Bytes(),
		})

		_, err := setup.rpcExample.ExampleMove(setup.authedCtx, moveReq)
		if err == nil {
			t.Fatal("Expected error for invalid target, but got none")
		}

		// Verify that failure didn't corrupt anything
		counts := countExamplesAllMethods(t, setup.authedCtx, setup.rpcExample, base.Queries, setup.endpointID)
		
		if counts.rpcCount != 3 || counts.dbAllCount != 3 || counts.isolatedCount != 0 {
			t.Fatalf("Transaction rollback failed: rpc=%d, db=%d, isolated=%d", counts.rpcCount, counts.dbAllCount, counts.isolatedCount)
		}

		t.Log("✓ Layer 1 Prevention: Transaction rollback maintained integrity on failure")
	})

	t.Log("=== Layer 1 Prevention Test: PASSED ===")
}

// TestAutoLinkingDetection - Verify auto-linking detects isolated examples (Layer 2: Detection) 
func TestAutoLinkingDetection(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()
	
	setup := createProtectionTestSetup(t, base, 4)

	t.Log("=== Layer 2: Auto-Linking Detection Test ===")

	t.Run("Auto-linking detects and fixes isolated examples", func(t *testing.T) {
		// Artificially create an isolated example by breaking linked-list pointers
		brokenExampleID := setup.exampleIDs[1]
		
		// Set this example's prev and next to null, making it isolated
		err := base.Queries.UpdateItemApiExampleOrder(ctx, gen.UpdateItemApiExampleOrderParams{
			ID:   brokenExampleID,
			Prev: nil,
			Next: nil,
		})
		if err != nil {
			t.Fatalf("Failed to create isolated example: %v", err)
		}

		// Also break the chain by updating the example before it
		err = base.Queries.UpdateItemApiExampleOrder(ctx, gen.UpdateItemApiExampleOrderParams{
			ID:   setup.exampleIDs[0],
			Prev: nil,
			Next: &setup.exampleIDs[2], // Skip the isolated example
		})
		if err != nil {
			t.Fatalf("Failed to break chain: %v", err)
		}

		// Update the example after the isolated one
		err = base.Queries.UpdateItemApiExampleOrder(ctx, gen.UpdateItemApiExampleOrderParams{
			ID:   setup.exampleIDs[2],
			Prev: &setup.exampleIDs[0], // Skip the isolated example
			Next: &setup.exampleIDs[3],
		})
		if err != nil {
			t.Fatalf("Failed to complete chain break: %v", err)
		}

		// Verify we created isolation (should have 1 isolated example)
		repo := sitemapiexample.NewExampleMovableRepository(base.Queries)
		isolatedExamples, err := repo.DetectIsolatedExamples(ctx, setup.endpointID)
		if err != nil {
			t.Fatalf("Failed to get isolated examples: %v", err)
		}
		if len(isolatedExamples) != 1 {
			t.Fatalf("Expected 1 isolated example, got %d", len(isolatedExamples))
		}

		t.Log("✓ Successfully created isolated example for detection test")

		// Now call ExampleList - auto-linking should detect and fix the isolation
		listReq := connect.NewRequest(&examplev1.ExampleListRequest{
			EndpointId: setup.endpointID.Bytes(),
		})

		listResp, err := setup.rpcExample.ExampleList(ctx, listReq)
		if err != nil {
			t.Fatalf("ExampleList failed: %v", err)
		}

		// Auto-linking should have fixed the issue - all 4 examples should be returned
		if len(listResp.Msg.Items) != 4 {
			t.Fatalf("Auto-linking detection failed: expected 4 examples, got %d", len(listResp.Msg.Items))
		}

		// Give auto-linking a moment to complete
		time.Sleep(10 * time.Millisecond)

		// Verify isolation was fixed
		isolatedExamplesAfter, err := repo.DetectIsolatedExamples(ctx, setup.endpointID)
		if err != nil {
			t.Fatalf("Failed to get isolated examples after fix: %v", err)
		}
		if len(isolatedExamplesAfter) != 0 {
			t.Fatalf("Auto-linking detection failed: still have %d isolated examples", len(isolatedExamplesAfter))
		}

		t.Log("✓ Layer 2 Detection: Auto-linking successfully detected and fixed isolated example")
	})

	t.Run("Auto-linking handles multiple isolated examples", func(t *testing.T) {
		// Create multiple isolated examples
		for i := 1; i <= 2; i++ {
			err := base.Queries.UpdateItemApiExampleOrder(ctx, gen.UpdateItemApiExampleOrderParams{
				ID:   setup.exampleIDs[i],
				Prev: nil,
				Next: nil,
			})
			if err != nil {
				t.Fatalf("Failed to create isolated example %d: %v", i, err)
			}
		}

		// Verify multiple isolated examples exist
		repo := sitemapiexample.NewExampleMovableRepository(base.Queries)
		isolatedExamples, err := repo.DetectIsolatedExamples(ctx, setup.endpointID)
		if err != nil {
			t.Fatalf("Failed to get isolated examples: %v", err)
		}
		if len(isolatedExamples) < 2 {
			t.Fatalf("Expected at least 2 isolated examples, got %d", len(isolatedExamples))
		}

		// Auto-linking should handle multiple isolated examples
		listResp := callExampleList(t, ctx, setup.rpcExample, setup.endpointID)
		
		if len(listResp.Items) != 4 {
			t.Fatalf("Auto-linking with multiple isolated examples failed: expected 4, got %d", len(listResp.Items))
		}

		t.Log("✓ Layer 2 Detection: Auto-linking handled multiple isolated examples")
	})

	t.Log("=== Layer 2 Detection Test: PASSED ===")
}

// TestRepositoryRecovery - Verify repository repairs work (Layer 3: Recovery)
func TestRepositoryRecovery(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()
	
	setup := createProtectionTestSetup(t, base, 5)

	t.Log("=== Layer 3: Repository Recovery Test ===")

	t.Run("Repository auto-linking repairs broken chains", func(t *testing.T) {
		// Create a complex broken chain scenario
		
		// Break multiple links in the chain
		// Original: [0] -> [1] -> [2] -> [3] -> [4]
		// Broken:   [0]    [1]    [2] -> [4]    [3] (isolated)
		
		// Isolate example 3
		err := base.Queries.UpdateItemApiExampleOrder(ctx, gen.UpdateItemApiExampleOrderParams{
			ID:   setup.exampleIDs[3],
			Prev: nil,
			Next: nil,
		})
		if err != nil {
			t.Fatalf("Failed to isolate example 3: %v", err)
		}

		// Break link from example 2 to 4 (skipping isolated 3)
		err = base.Queries.UpdateItemApiExampleOrder(ctx, gen.UpdateItemApiExampleOrderParams{
			ID:   setup.exampleIDs[2],
			Prev: &setup.exampleIDs[1],
			Next: &setup.exampleIDs[4], // Skip isolated example 3
		})
		if err != nil {
			t.Fatalf("Failed to break chain: %v", err)
		}

		// Update example 4's prev to skip isolated 3
		err = base.Queries.UpdateItemApiExampleOrder(ctx, gen.UpdateItemApiExampleOrderParams{
			ID:   setup.exampleIDs[4],
			Prev: &setup.exampleIDs[2], // Skip isolated example 3
			Next: nil,
		})
		if err != nil {
			t.Fatalf("Failed to update chain: %v", err)
		}

		t.Log("✓ Created complex broken chain for recovery test")

		// Call the repository auto-linking directly
		iaes := sitemapiexample.New(base.Queries)
		err = iaes.AutoLinkIsolatedExamples(ctx, setup.endpointID)
		if err != nil {
			t.Fatalf("Repository auto-linking failed: %v", err)
		}

		// Verify recovery worked - all examples should now be accessible
		counts := countExamplesAllMethods(t, ctx, setup.rpcExample, base.Queries, setup.endpointID)
		
		if counts.dbAllCount != 5 {
			t.Fatalf("Repository recovery failed: expected 5 examples, got %d", counts.dbAllCount)
		}
		if counts.isolatedCount != 0 {
			t.Fatalf("Repository recovery failed: still have %d isolated examples", counts.isolatedCount)
		}

		t.Log("✓ Layer 3 Recovery: Repository auto-linking successfully repaired broken chain")
	})

	t.Run("Repository recovery handles edge cases", func(t *testing.T) {
		// Test recovery with first and last examples isolated
		
		// Isolate first example
		err := base.Queries.UpdateItemApiExampleOrder(ctx, gen.UpdateItemApiExampleOrderParams{
			ID:   setup.exampleIDs[0],
			Prev: nil,
			Next: nil,
		})
		if err != nil {
			t.Fatalf("Failed to isolate first example: %v", err)
		}

		// Isolate last example
		err = base.Queries.UpdateItemApiExampleOrder(ctx, gen.UpdateItemApiExampleOrderParams{
			ID:   setup.exampleIDs[4],
			Prev: nil,
			Next: nil,
		})
		if err != nil {
			t.Fatalf("Failed to isolate last example: %v", err)
		}

		// Update middle chain
		err = base.Queries.UpdateItemApiExampleOrder(ctx, gen.UpdateItemApiExampleOrderParams{
			ID:   setup.exampleIDs[1],
			Prev: nil, // Now first in chain
			Next: &setup.exampleIDs[2],
		})
		if err != nil {
			t.Fatalf("Failed to update middle chain: %v", err)
		}

		err = base.Queries.UpdateItemApiExampleOrder(ctx, gen.UpdateItemApiExampleOrderParams{
			ID:   setup.exampleIDs[3],
			Prev: &setup.exampleIDs[2],
			Next: nil, // Now last in chain
		})
		if err != nil {
			t.Fatalf("Failed to update middle chain end: %v", err)
		}

		// Repository recovery should handle this edge case
		iaes := sitemapiexample.New(base.Queries)
		err = iaes.AutoLinkIsolatedExamples(ctx, setup.endpointID)
		if err != nil {
			t.Fatalf("Repository recovery failed on edge case: %v", err)
		}

		// All examples should still be recoverable
		listResp := callExampleList(t, ctx, setup.rpcExample, setup.endpointID)
		if len(listResp.Items) != 5 {
			t.Fatalf("Edge case recovery failed: expected 5 examples, got %d", len(listResp.Items))
		}

		t.Log("✓ Layer 3 Recovery: Repository handled edge case recovery")
	})

	t.Log("=== Layer 3 Recovery Test: PASSED ===")
}

// TestRPCFallback - Verify RPC fallback works (Layer 4: Fallback)
func TestRPCFallback(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()
	
	setup := createProtectionTestSetup(t, base, 6)

	t.Log("=== Layer 4: RPC Fallback Test ===")

	t.Run("RPC fallback when ordered query fails", func(t *testing.T) {
		// Completely destroy the linked-list structure to force ordered query failure
		for i := 0; i < len(setup.exampleIDs); i++ {
			err := base.Queries.UpdateItemApiExampleOrder(ctx, gen.UpdateItemApiExampleOrderParams{
				ID:   setup.exampleIDs[i],
				Prev: nil,
				Next: nil,
			})
			if err != nil {
				t.Fatalf("Failed to destroy linked-list: %v", err)
			}
		}

		t.Log("✓ Completely destroyed linked-list to test fallback")

		// ExampleList should still return all examples via fallback query
		listResp := callExampleList(t, ctx, setup.rpcExample, setup.endpointID)
		
		if len(listResp.Items) != 6 {
			t.Fatalf("RPC fallback failed: expected 6 examples, got %d", len(listResp.Items))
		}

		// Verify the fallback was actually used by checking that ordered query fails
		orderedExamples, err := base.Queries.GetExamplesByEndpointIDOrdered(ctx, gen.GetExamplesByEndpointIDOrderedParams{
			ItemApiID:   setup.endpointID,
			ItemApiID_2: setup.endpointID,
		})
		
		// Ordered query should return 0 or fail because the chain is broken
		if err == nil && len(orderedExamples) == 6 {
			t.Fatal("Expected ordered query to fail or return fewer results with broken chain")
		}

		t.Log("✓ Layer 4 Fallback: RPC successfully used fallback when ordered query failed")
	})

	t.Run("Fallback performance meets requirements", func(t *testing.T) {
		// Test that fallback doesn't significantly impact performance
		start := time.Now()
		
		listResp := callExampleList(t, ctx, setup.rpcExample, setup.endpointID)
		
		duration := time.Since(start)
		
		if len(listResp.Items) != 6 {
			t.Fatalf("Fallback performance test: expected 6 examples, got %d", len(listResp.Items))
		}
		
		// Even with fallback, should complete within 100ms
		if duration > 100*time.Millisecond {
			t.Errorf("Fallback performance: took %v, expected < 100ms", duration)
		} else {
			t.Logf("✓ Layer 4 Fallback: Performance acceptable at %v (< 100ms)", duration)
		}
	})

	t.Run("Fallback with concurrent operations", func(t *testing.T) {
		// Test fallback works correctly under concurrent load
		const numGoroutines = 5
		resultsCh := make(chan int, numGoroutines)
		errorsCh := make(chan error, numGoroutines)

		// Run multiple concurrent ExampleList calls
		for i := 0; i < numGoroutines; i++ {
			go func(idx int) {
				listResp, err := setup.rpcExample.ExampleList(ctx, connect.NewRequest(&examplev1.ExampleListRequest{
					EndpointId: setup.endpointID.Bytes(),
				}))
				
				if err != nil {
					errorsCh <- err
					return
				}
				
				resultsCh <- len(listResp.Msg.Items)
			}(i)
		}

		// Collect results
		for i := 0; i < numGoroutines; i++ {
			select {
			case count := <-resultsCh:
				if count != 6 {
					t.Errorf("Concurrent fallback test %d: expected 6 examples, got %d", i, count)
				}
			case err := <-errorsCh:
				t.Errorf("Concurrent fallback test %d failed: %v", i, err)
			case <-time.After(1 * time.Second):
				t.Errorf("Concurrent fallback test %d timed out", i)
			}
		}

		t.Log("✓ Layer 4 Fallback: Worked correctly under concurrent load")
	})

	t.Log("=== Layer 4 Fallback Test: PASSED ===")
}

// TestProtectionLayersIntegration - Test all layers working together
func TestProtectionLayersIntegration(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()
	
	setup := createProtectionTestSetup(t, base, 7)

	t.Log("=== Multi-Layer Protection Integration Test ===")

	t.Run("All layers working together under stress", func(t *testing.T) {
		// Create a scenario that tests all protection layers simultaneously
		
		// 1. Perform rapid moves (tests Layer 1: Atomic Prevention)
		for i := 0; i < 10; i++ {
			srcIdx := i % 7
			targetIdx := (i + 2) % 7
			if srcIdx != targetIdx {
				performMove(t, ctx, setup.rpcExample, setup.endpointID, setup.exampleIDs[srcIdx], setup.exampleIDs[targetIdx], resourcesv1.MovePosition_MOVE_POSITION_AFTER)
			}
		}

		// 2. Artificially introduce corruption (tests Layer 2: Detection & Layer 3: Recovery)
		err := base.Queries.UpdateItemApiExampleOrder(ctx, gen.UpdateItemApiExampleOrderParams{
			ID:   setup.exampleIDs[3],
			Prev: nil,
			Next: nil, // Isolate this example
		})
		if err != nil {
			t.Fatalf("Failed to introduce corruption: %v", err)
		}

		// 3. Break additional links to force fallback (tests Layer 4: Fallback)
		for i := 4; i < 6; i++ {
			err := base.Queries.UpdateItemApiExampleOrder(ctx, gen.UpdateItemApiExampleOrderParams{
				ID:   setup.exampleIDs[i],
				Prev: nil,
				Next: nil,
			})
			if err != nil {
				t.Fatalf("Failed to break additional links: %v", err)
			}
		}

		// 4. Despite all this chaos, ExampleList should STILL return all 7 examples
		listResp := callExampleList(t, ctx, setup.rpcExample, setup.endpointID)
		
		if len(listResp.Items) != 7 {
			t.Fatalf("Multi-layer protection failed: expected 7 examples, got %d", len(listResp.Items))
		}

		t.Log("✓ Multi-Layer Protection: All layers working together successfully")
	})

	t.Run("Performance under multi-layer protection", func(t *testing.T) {
		// Test that multiple protection layers don't significantly impact performance
		start := time.Now()
		
		// Perform operations that trigger all layers
		for i := 0; i < 3; i++ {
			listResp := callExampleList(t, ctx, setup.rpcExample, setup.endpointID)
			if len(listResp.Items) != 7 {
				t.Fatalf("Performance test iteration %d: expected 7 examples, got %d", i, len(listResp.Items))
			}
		}
		
		duration := time.Since(start)
		
		// Multiple operations with all protection layers should complete within 500ms
		if duration > 500*time.Millisecond {
			t.Errorf("Multi-layer performance: took %v, expected < 500ms", duration)
		} else {
			t.Logf("✓ Multi-Layer Protection: Performance acceptable at %v (< 500ms)", duration)
		}
	})

	t.Log("=== Multi-Layer Protection Integration Test: PASSED ===")
}

// Helper functions for protection layer tests

type protectionTestSetup struct {
	rpcExample   ritemapiexample.ItemAPIExampleRPC
	authedCtx    context.Context
	endpointID   idwrap.IDWrap
	exampleIDs   []idwrap.IDWrap
	exampleNames []string
}

func createProtectionTestSetup(t *testing.T, base *testutil.BaseDBQueries, numExamples int) *protectionTestSetup {
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
		Name:         "protection_test_endpoint",
		Url:          "https://api.protection-test.com/endpoint",
		Method:       "PUT",
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
		exampleNames[i] = fmt.Sprintf("protection_example_%d", i+1)
		
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

	return &protectionTestSetup{
		rpcExample:   rpcExample,
		authedCtx:    authedCtx,
		endpointID:   endpointID,
		exampleIDs:   exampleIDs,
		exampleNames: exampleNames,
	}
}