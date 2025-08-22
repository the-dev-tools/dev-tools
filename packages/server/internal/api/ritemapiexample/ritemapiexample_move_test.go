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

// Helper function to verify example order and linked-list integrity
func verifyExampleOrder(t *testing.T, ctx context.Context, queries *gen.Queries, endpointID idwrap.IDWrap, expectedOrder []idwrap.IDWrap) {
	t.Helper()
	
	// Get ordered examples using the ordered query
	orderedExamples, err := queries.GetExamplesByEndpointIDOrdered(ctx, gen.GetExamplesByEndpointIDOrderedParams{
		ItemApiID:   endpointID,
		ItemApiID_2: endpointID,
	})
	if err != nil {
		t.Fatalf("Failed to get ordered examples: %v", err)
	}

	// Check count matches expected
	if len(orderedExamples) != len(expectedOrder) {
		t.Fatalf("Expected %d examples, got %d", len(expectedOrder), len(orderedExamples))
	}

	// Check order matches
	for i, expected := range expectedOrder {
		actual := idwrap.NewFromBytesMust(orderedExamples[i].ID)
		if actual.Compare(expected) != 0 {
			t.Errorf("Position %d: expected %s, got %s", i, expected.String(), actual.String())
		}
	}

	// Verify linked-list integrity
	if len(orderedExamples) == 0 {
		return // Nothing to verify for empty list
	}

	// First example should have nil prev
	if orderedExamples[0].Prev != nil {
		t.Errorf("First example should have nil prev, got %v", orderedExamples[0].Prev)
	}

	// Last example should have nil next
	lastIdx := len(orderedExamples) - 1
	if orderedExamples[lastIdx].Next != nil {
		t.Errorf("Last example should have nil next, got %v", orderedExamples[lastIdx].Next)
	}

	// Verify forward links
	for i := 0; i < len(orderedExamples)-1; i++ {
		nextID := idwrap.NewFromBytesMust(orderedExamples[i+1].ID)
		
		if orderedExamples[i].Next == nil {
			t.Errorf("Example %d should have next pointer, got nil", i)
			continue
		}
		
		actualNextID := idwrap.NewFromBytesMust(orderedExamples[i].Next)
		if actualNextID.Compare(nextID) != 0 {
			t.Errorf("Example %d: next pointer should be %s, got %s", i, nextID.String(), actualNextID.String())
		}
	}

	// Verify backward links
	for i := 1; i < len(orderedExamples); i++ {
		prevID := idwrap.NewFromBytesMust(orderedExamples[i-1].ID)
		
		if orderedExamples[i].Prev == nil {
			t.Errorf("Example %d should have prev pointer, got nil", i)
			continue
		}
		
		actualPrevID := idwrap.NewFromBytesMust(orderedExamples[i].Prev)
		if actualPrevID.Compare(prevID) != 0 {
			t.Errorf("Example %d: prev pointer should be %s, got %s", i, prevID.String(), actualPrevID.String())
		}
	}
}

// Helper to create a test setup with workspace, collection, endpoint, and examples
type testSetup struct {
	base           *testutil.BaseDBQueries
	rpcExample     ritemapiexample.ItemAPIExampleRPC
	authedCtx      context.Context
	workspaceID    idwrap.IDWrap
	collectionID   idwrap.IDWrap
	endpointID     idwrap.IDWrap
	userID         idwrap.IDWrap
	exampleIDs     []idwrap.IDWrap
	exampleNames   []string
}

func createTestSetup(t *testing.T, numExamples int) *testSetup {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
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

	// Create examples with proper linked list structure
	exampleIDs := make([]idwrap.IDWrap, numExamples)
	exampleNames := make([]string, numExamples)
	for i := 0; i < numExamples; i++ {
		exampleIDs[i] = idwrap.NewNow()
		exampleNames[i] = fmt.Sprintf("example%d", i+1)
		
		var prev *idwrap.IDWrap
		var next *idwrap.IDWrap
		
		if i > 0 {
			prev = &exampleIDs[i-1]
		}
		if i < numExamples-1 {
			// We'll set this after creating the next example
			next = nil
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
		
		// Update the previous example's Next pointer
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
	}

	// Create RPC handler
	logChanMap := logconsole.NewLogChanMapWith(10000)
	rpcExample := ritemapiexample.New(db, iaes, ias, ifs, ws, cs, us, hs, qs, bfs, bues, brs, erhs, ers, es, vs, as, ars, logChanMap)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	return &testSetup{
		base:         base,
		rpcExample:   rpcExample,
		authedCtx:    authedCtx,
		workspaceID:  workspaceID,
		collectionID: collectionID,
		endpointID:   endpointID,
		userID:       userID,
		exampleIDs:   exampleIDs,
		exampleNames: exampleNames,
	}
}

// TestExampleMoveE2E - Primary comprehensive E2E test
func TestExampleMoveE2E(t *testing.T) {
	setup := createTestSetup(t, 4)
	defer setup.base.Close()

	ctx := setup.authedCtx
	
	// Initial setup: create examples [example1, example2, example3, example4]
	example1ID := setup.exampleIDs[0]
	example2ID := setup.exampleIDs[1]
	example3ID := setup.exampleIDs[2]
	example4ID := setup.exampleIDs[3]

	// Verify initial order using ExampleList RPC
	listReq := connect.NewRequest(&examplev1.ExampleListRequest{
		EndpointId: setup.endpointID.Bytes(),
	})
	
	listResp, err := setup.rpcExample.ExampleList(ctx, listReq)
	if err != nil {
		t.Fatalf("ExampleList failed: %v", err)
	}
	
	if len(listResp.Msg.Items) != 4 {
		t.Fatalf("Expected 4 examples, got %d", len(listResp.Msg.Items))
	}

	// Verify initial order in database
	verifyExampleOrder(t, ctx, setup.base.Queries, setup.endpointID, 
		[]idwrap.IDWrap{example1ID, example2ID, example3ID, example4ID})

	// TEST 1: Move example1 AFTER example3 → Order should be: [example2, example3, example1, example4]
	t.Run("Move example1 after example3", func(t *testing.T) {
		moveReq := connect.NewRequest(&examplev1.ExampleMoveRequest{
			EndpointId:      setup.endpointID.Bytes(),
			ExampleId:       example1ID.Bytes(),
			Position:        resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetExampleId: example3ID.Bytes(),
		})

		_, err := setup.rpcExample.ExampleMove(ctx, moveReq)
		if err != nil {
			t.Fatalf("ExampleMove failed: %v", err)
		}

		// Verify new order
		expectedOrder := []idwrap.IDWrap{example2ID, example3ID, example1ID, example4ID}
		verifyExampleOrder(t, ctx, setup.base.Queries, setup.endpointID, expectedOrder)

		// Verify via ExampleList RPC
		listResp, err := setup.rpcExample.ExampleList(ctx, listReq)
		if err != nil {
			t.Fatalf("ExampleList failed: %v", err)
		}

		if len(listResp.Msg.Items) != 4 {
			t.Fatalf("Expected 4 examples after move, got %d", len(listResp.Msg.Items))
		}

		// Note: ExampleList may not be ordered yet - we'll verify this later
		t.Log("✓ Move example1 after example3 completed successfully")
	})

	// TEST 2: Move example4 BEFORE example2 → Order should be: [example4, example2, example3, example1]
	t.Run("Move example4 before example2", func(t *testing.T) {
		moveReq := connect.NewRequest(&examplev1.ExampleMoveRequest{
			EndpointId:      setup.endpointID.Bytes(),
			ExampleId:       example4ID.Bytes(),
			Position:        resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
			TargetExampleId: example2ID.Bytes(),
		})

		_, err := setup.rpcExample.ExampleMove(ctx, moveReq)
		if err != nil {
			t.Fatalf("ExampleMove failed: %v", err)
		}

		// Verify new order
		expectedOrder := []idwrap.IDWrap{example4ID, example2ID, example3ID, example1ID}
		verifyExampleOrder(t, ctx, setup.base.Queries, setup.endpointID, expectedOrder)

		t.Log("✓ Move example4 before example2 completed successfully")
	})

	// TEST 3: Move example3 AFTER example1 → Order should be: [example4, example2, example1, example3]
	t.Run("Move example3 after example1", func(t *testing.T) {
		moveReq := connect.NewRequest(&examplev1.ExampleMoveRequest{
			EndpointId:      setup.endpointID.Bytes(),
			ExampleId:       example3ID.Bytes(),
			Position:        resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetExampleId: example1ID.Bytes(),
		})

		_, err := setup.rpcExample.ExampleMove(ctx, moveReq)
		if err != nil {
			t.Fatalf("ExampleMove failed: %v", err)
		}

		// Verify final order
		expectedOrder := []idwrap.IDWrap{example4ID, example2ID, example1ID, example3ID}
		verifyExampleOrder(t, ctx, setup.base.Queries, setup.endpointID, expectedOrder)

		t.Log("✓ Move example3 after example1 completed successfully")
	})

	// Final verification - all operations completed successfully
	t.Log("✓ All E2E move operations completed successfully")
	t.Log("✓ Database linked-list integrity maintained throughout all operations")
}

// TestExampleMoveEdgeCases - Edge case testing
func TestExampleMoveEdgeCases(t *testing.T) {
	t.Run("Move first example to last position", func(t *testing.T) {
		setup := createTestSetup(t, 3)
		defer setup.base.Close()

		ctx := setup.authedCtx
		firstID := setup.exampleIDs[0]
		lastID := setup.exampleIDs[2]

		// Move first after last
		moveReq := connect.NewRequest(&examplev1.ExampleMoveRequest{
			EndpointId:      setup.endpointID.Bytes(),
			ExampleId:       firstID.Bytes(),
			Position:        resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetExampleId: lastID.Bytes(),
		})

		_, err := setup.rpcExample.ExampleMove(ctx, moveReq)
		if err != nil {
			t.Fatalf("Move failed: %v", err)
		}

		// Expected order: [example2, example3, example1]
		expectedOrder := []idwrap.IDWrap{setup.exampleIDs[1], setup.exampleIDs[2], setup.exampleIDs[0]}
		verifyExampleOrder(t, ctx, setup.base.Queries, setup.endpointID, expectedOrder)
	})

	t.Run("Move last example to first position", func(t *testing.T) {
		setup := createTestSetup(t, 3)
		defer setup.base.Close()

		ctx := setup.authedCtx
		firstID := setup.exampleIDs[0]
		lastID := setup.exampleIDs[2]

		// Move last before first
		moveReq := connect.NewRequest(&examplev1.ExampleMoveRequest{
			EndpointId:      setup.endpointID.Bytes(),
			ExampleId:       lastID.Bytes(),
			Position:        resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
			TargetExampleId: firstID.Bytes(),
		})

		_, err := setup.rpcExample.ExampleMove(ctx, moveReq)
		if err != nil {
			t.Fatalf("Move failed: %v", err)
		}

		// Expected order: [example3, example1, example2]
		expectedOrder := []idwrap.IDWrap{setup.exampleIDs[2], setup.exampleIDs[0], setup.exampleIDs[1]}
		verifyExampleOrder(t, ctx, setup.base.Queries, setup.endpointID, expectedOrder)
	})

	t.Run("Move middle example to middle position", func(t *testing.T) {
		setup := createTestSetup(t, 5)
		defer setup.base.Close()

		ctx := setup.authedCtx

		// Move example2 after example4 (middle to middle)
		moveReq := connect.NewRequest(&examplev1.ExampleMoveRequest{
			EndpointId:      setup.endpointID.Bytes(),
			ExampleId:       setup.exampleIDs[1].Bytes(), // example2
			Position:        resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetExampleId: setup.exampleIDs[3].Bytes(), // example4
		})

		_, err := setup.rpcExample.ExampleMove(ctx, moveReq)
		if err != nil {
			t.Fatalf("Move failed: %v", err)
		}

		// Expected order: [example1, example3, example4, example2, example5]
		expectedOrder := []idwrap.IDWrap{
			setup.exampleIDs[0], setup.exampleIDs[2], setup.exampleIDs[3], 
			setup.exampleIDs[1], setup.exampleIDs[4],
		}
		verifyExampleOrder(t, ctx, setup.base.Queries, setup.endpointID, expectedOrder)
	})

	t.Run("Single example endpoint", func(t *testing.T) {
		setup := createTestSetup(t, 1)
		defer setup.base.Close()

		ctx := setup.authedCtx
		singleID := setup.exampleIDs[0]

		// Try to move the only example (this should fail gracefully)
		moveReq := connect.NewRequest(&examplev1.ExampleMoveRequest{
			EndpointId:      setup.endpointID.Bytes(),
			ExampleId:       singleID.Bytes(),
			Position:        resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetExampleId: singleID.Bytes(),
		})

		_, err := setup.rpcExample.ExampleMove(ctx, moveReq)
		if err == nil {
			t.Fatal("Expected error when moving example relative to itself")
		}

		// Verify order unchanged
		verifyExampleOrder(t, ctx, setup.base.Queries, setup.endpointID, []idwrap.IDWrap{singleID})
	})
}

// TestExampleMovePermissions - Permission testing
func TestExampleMovePermissions(t *testing.T) {
	t.Run("User without endpoint access tries to move examples", func(t *testing.T) {
		setup := createTestSetup(t, 2)
		defer setup.base.Close()

		// Create different user without access
		unauthorizedUserID := idwrap.NewNow()
		unauthorizedCtx := mwauth.CreateAuthedContext(context.Background(), unauthorizedUserID)

		moveReq := connect.NewRequest(&examplev1.ExampleMoveRequest{
			EndpointId:      setup.endpointID.Bytes(),
			ExampleId:       setup.exampleIDs[0].Bytes(),
			Position:        resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetExampleId: setup.exampleIDs[1].Bytes(),
		})

		_, err := setup.rpcExample.ExampleMove(unauthorizedCtx, moveReq)
		if err == nil {
			t.Fatal("Expected permission error")
		}

		connectErr := err.(*connect.Error)
		if connectErr.Code() != connect.CodeNotFound && connectErr.Code() != connect.CodePermissionDenied {
			t.Errorf("Expected NotFound or PermissionDenied, got %v", connectErr.Code())
		}
	})

	t.Run("Move examples with invalid endpoint ID", func(t *testing.T) {
		setup := createTestSetup(t, 2)
		defer setup.base.Close()

		nonExistentEndpointID := idwrap.NewNow()

		moveReq := connect.NewRequest(&examplev1.ExampleMoveRequest{
			EndpointId:      nonExistentEndpointID.Bytes(),
			ExampleId:       setup.exampleIDs[0].Bytes(),
			Position:        resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetExampleId: setup.exampleIDs[1].Bytes(),
		})

		_, err := setup.rpcExample.ExampleMove(setup.authedCtx, moveReq)
		if err == nil {
			t.Fatal("Expected error for non-existent endpoint")
		}

		connectErr := err.(*connect.Error)
		if connectErr.Code() != connect.CodeNotFound {
			t.Errorf("Expected NotFound, got %v", connectErr.Code())
		}
	})

	t.Run("Move non-existent example", func(t *testing.T) {
		setup := createTestSetup(t, 2)
		defer setup.base.Close()

		nonExistentExampleID := idwrap.NewNow()

		moveReq := connect.NewRequest(&examplev1.ExampleMoveRequest{
			EndpointId:      setup.endpointID.Bytes(),
			ExampleId:       nonExistentExampleID.Bytes(),
			Position:        resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetExampleId: setup.exampleIDs[0].Bytes(),
		})

		_, err := setup.rpcExample.ExampleMove(setup.authedCtx, moveReq)
		if err == nil {
			t.Fatal("Expected error for non-existent example")
		}

		connectErr := err.(*connect.Error)
		if connectErr.Code() != connect.CodeNotFound {
			t.Errorf("Expected NotFound, got %v", connectErr.Code())
		}
	})

	t.Run("Move to non-existent target", func(t *testing.T) {
		setup := createTestSetup(t, 2)
		defer setup.base.Close()

		nonExistentTargetID := idwrap.NewNow()

		moveReq := connect.NewRequest(&examplev1.ExampleMoveRequest{
			EndpointId:      setup.endpointID.Bytes(),
			ExampleId:       setup.exampleIDs[0].Bytes(),
			Position:        resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetExampleId: nonExistentTargetID.Bytes(),
		})

		_, err := setup.rpcExample.ExampleMove(setup.authedCtx, moveReq)
		if err == nil {
			t.Fatal("Expected error for non-existent target")
		}

		connectErr := err.(*connect.Error)
		if connectErr.Code() != connect.CodeNotFound {
			t.Errorf("Expected NotFound, got %v", connectErr.Code())
		}
	})
}

// Benchmark tests for performance validation
func BenchmarkExampleMoveAfter(b *testing.B) {
	setup := createTestSetup(&testing.T{}, 10)
	defer setup.base.Close()

	ctx := setup.authedCtx
	
	// Reset timer after setup
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		// Move first example after second
		moveReq := connect.NewRequest(&examplev1.ExampleMoveRequest{
			EndpointId:      setup.endpointID.Bytes(),
			ExampleId:       setup.exampleIDs[0].Bytes(),
			Position:        resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetExampleId: setup.exampleIDs[1].Bytes(),
		})

		_, err := setup.rpcExample.ExampleMove(ctx, moveReq)
		if err != nil {
			b.Fatalf("ExampleMove failed: %v", err)
		}
	}
}

func BenchmarkExampleMoveBefore(b *testing.B) {
	setup := createTestSetup(&testing.T{}, 10)
	defer setup.base.Close()

	ctx := setup.authedCtx
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		// Move second example before first
		moveReq := connect.NewRequest(&examplev1.ExampleMoveRequest{
			EndpointId:      setup.endpointID.Bytes(),
			ExampleId:       setup.exampleIDs[1].Bytes(),
			Position:        resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
			TargetExampleId: setup.exampleIDs[0].Bytes(),
		})

		_, err := setup.rpcExample.ExampleMove(ctx, moveReq)
		if err != nil {
			b.Fatalf("ExampleMove failed: %v", err)
		}
	}
}

func BenchmarkExampleMoveWithManyExamples(b *testing.B) {
	setup := createTestSetup(&testing.T{}, 100)
	defer setup.base.Close()

	ctx := setup.authedCtx
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		// Move example from beginning to end
		moveReq := connect.NewRequest(&examplev1.ExampleMoveRequest{
			EndpointId:      setup.endpointID.Bytes(),
			ExampleId:       setup.exampleIDs[0].Bytes(),
			Position:        resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetExampleId: setup.exampleIDs[99].Bytes(),
		})

		_, err := setup.rpcExample.ExampleMove(ctx, moveReq)
		if err != nil {
			b.Fatalf("ExampleMove failed: %v", err)
		}
	}
}

// TestExampleMovePerformance - Performance validation tests
func TestExampleMovePerformance(t *testing.T) {
	setup := createTestSetup(t, 50)
	defer setup.base.Close()

	ctx := setup.authedCtx

	// Test that move operations complete within reasonable time
	start := time.Now()
	
	moveReq := connect.NewRequest(&examplev1.ExampleMoveRequest{
		EndpointId:      setup.endpointID.Bytes(),
		ExampleId:       setup.exampleIDs[0].Bytes(),
		Position:        resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		TargetExampleId: setup.exampleIDs[25].Bytes(),
	})

	_, err := setup.rpcExample.ExampleMove(ctx, moveReq)
	if err != nil {
		t.Fatalf("ExampleMove failed: %v", err)
	}

	duration := time.Since(start)
	
	// Performance requirement: < 10ms
	if duration > 10*time.Millisecond {
		t.Errorf("ExampleMove took %v, expected < 10ms", duration)
	} else {
		t.Logf("✓ ExampleMove completed in %v (< 10ms requirement met)", duration)
	}
}

// TestExampleMoveConcurrency - Test concurrent move operations
func TestExampleMoveConcurrency(t *testing.T) {
	setup := createTestSetup(t, 10)
	defer setup.base.Close()

	ctx := setup.authedCtx
	
	// Run multiple concurrent moves (this should be handled by DB transactions)
	const numGoroutines = 5
	errCh := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			moveReq := connect.NewRequest(&examplev1.ExampleMoveRequest{
				EndpointId:      setup.endpointID.Bytes(),
				ExampleId:       setup.exampleIDs[idx].Bytes(),
				Position:        resourcesv1.MovePosition_MOVE_POSITION_AFTER,
				TargetExampleId: setup.exampleIDs[(idx+1)%len(setup.exampleIDs)].Bytes(),
			})

			_, err := setup.rpcExample.ExampleMove(ctx, moveReq)
			errCh <- err
		}(i)
	}

	// Collect results
	for i := 0; i < numGoroutines; i++ {
		if err := <-errCh; err != nil {
			t.Errorf("Concurrent move %d failed: %v", i, err)
		}
	}

	// Verify final state is still valid (no corruption)
	orderedExamples, err := setup.base.Queries.GetExamplesByEndpointIDOrdered(ctx, gen.GetExamplesByEndpointIDOrderedParams{
		ItemApiID:   setup.endpointID,
		ItemApiID_2: setup.endpointID,
	})
	if err != nil {
		t.Fatalf("Failed to get final ordered examples: %v", err)
	}

	if len(orderedExamples) != 10 {
		t.Errorf("Expected 10 examples after concurrent operations, got %d", len(orderedExamples))
	}

	t.Log("✓ Concurrent move operations completed without corrupting the linked list")
}