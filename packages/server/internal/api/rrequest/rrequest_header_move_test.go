package rrequest_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rrequest"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/service/sassert"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/sexampleheader"
	"the-dev-tools/server/pkg/service/sexamplequery"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/testutil"
	requestv1 "the-dev-tools/spec/dist/buf/go/collection/item/request/v1"
	resourcesv1 "the-dev-tools/spec/dist/buf/go/resources/v1"
)

// originHeaderMoveTestData holds common test setup data for origin header move functionality testing
type originHeaderMoveTestData struct {
	ctx       context.Context
	rpc       rrequest.RequestRPC
	exampleID idwrap.IDWrap
	userID    idwrap.IDWrap
	ehs       sexampleheader.HeaderService
}

// setupOriginHeaderMoveTestData creates test data for origin header move functionality testing
func setupOriginHeaderMoveTestData(t *testing.T) *originHeaderMoveTestData {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	// Initialize services
	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	mockLogger := mocklogger.NewMockLogger()
	cs := scollection.New(queries, mockLogger)
	us := suser.New(queries)
	ehs := sexampleheader.New(queries)
	eqs := sexamplequery.New(queries)
	as := sassert.New(queries)

	// Create workspace and collection using base services pattern
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	userID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)

	// Create API item (endpoint)
	item := &mitemapi.ItemApi{
		ID:           idwrap.NewNow(),
		CollectionID: collectionID,
		Name:         "test-endpoint",
		Method:       "GET",
		Url:          "https://api.test.com/endpoint",
	}
	err := ias.CreateItemApi(ctx, item)
	if err != nil {
		t.Fatal(err)
	}

	// Create example
	exampleID := idwrap.NewNow()
	example := &mitemapiexample.ItemApiExample{
		ID:           exampleID,
		ItemApiID:    item.ID,
		CollectionID: collectionID,
		Name:         "test-example",
	}
	err = iaes.CreateApiExample(ctx, example)
	if err != nil {
		t.Fatal(err)
	}

	rpc := rrequest.New(db, cs, us, ias, iaes, ehs, eqs, as)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	return &originHeaderMoveTestData{
		ctx:       authedCtx,
		rpc:       rpc,
		exampleID: exampleID,
		userID:    userID,
		ehs:       ehs,
	}
}

// createOriginTestHeader creates an origin header (non-delta) using the RPC and returns its ID
func createOriginTestHeader(t *testing.T, data *originHeaderMoveTestData, key, value string) idwrap.IDWrap {
	resp, err := data.rpc.HeaderCreate(data.ctx, connect.NewRequest(&requestv1.HeaderCreateRequest{
		ExampleId:   data.exampleID.Bytes(),
		Key:         key,
		Value:       value,
		Enabled:     true,
		Description: "Test origin header",
	}))
	if err != nil {
		t.Fatalf("Failed to create origin header %s: %v", key, err)
	}
	headerID, err := idwrap.NewFromBytes(resp.Msg.HeaderId)
	if err != nil {
		t.Fatalf("Failed to parse header ID: %v", err)
	}
	return headerID
}

// TestHeaderMoveIntegration tests the complete header ordering flow for origin headers
func TestHeaderMoveIntegration(t *testing.T) {
	data := setupOriginHeaderMoveTestData(t)

	// Create 4 origin headers using the RPC
	header1ID := createOriginTestHeader(t, data, "Header-1", "Value1")
	header2ID := createOriginTestHeader(t, data, "Header-2", "Value2")
	header3ID := createOriginTestHeader(t, data, "Header-3", "Value3")
	header4ID := createOriginTestHeader(t, data, "Header-4", "Value4")

	// Test Case 1: Move header 4 after header 1 (expected order: 1, 4, 2, 3)
	t.Run("MoveHeader4After1", func(t *testing.T) {
		req := &requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       header4ID.Bytes(),
			TargetHeaderId: header1ID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}

		resp, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
		if err != nil {
			t.Fatalf("HeaderMove failed: %v", err)
		}
		if resp == nil {
			t.Fatal("Expected response, got nil")
		}

		// Verify the new order: 1, 4, 2, 3
		expectedOrder := []idwrap.IDWrap{header1ID, header4ID, header2ID, header3ID}
		contextStr := "MoveHeader4After1"
		verifyHeaderOrder(t, data.ctx, data.ehs, data.exampleID, expectedOrder, &contextStr)
	})

	// Test Case 2: Move header 2 before header 4 (expected order: 1, 2, 4, 3)
	t.Run("MoveHeader2Before4", func(t *testing.T) {
		req := &requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       header2ID.Bytes(),
			TargetHeaderId: header4ID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
		}

		resp, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
		if err != nil {
			t.Fatalf("HeaderMove failed: %v", err)
		}
		if resp == nil {
			t.Fatal("Expected response, got nil")
		}

		// Verify the new order: 1, 2, 4, 3
		expectedOrder := []idwrap.IDWrap{header1ID, header2ID, header4ID, header3ID}
		contextStr := "MoveHeader2Before4"
		verifyHeaderOrder(t, data.ctx, data.ehs, data.exampleID, expectedOrder, &contextStr)
	})
}

// TestHeaderMoveSpecificScenario tests the exact scenario: header 4 after header 1 results in [1,4,2,3]
func TestHeaderMoveSpecificScenario(t *testing.T) {
	data := setupOriginHeaderMoveTestData(t)

	// Create exactly 4 headers in order: 1, 2, 3, 4
	header1ID := createOriginTestHeader(t, data, "Header-1", "Value1")
	header2ID := createOriginTestHeader(t, data, "Header-2", "Value2") 
	header3ID := createOriginTestHeader(t, data, "Header-3", "Value3")
	header4ID := createOriginTestHeader(t, data, "Header-4", "Value4")

	// Verify initial order is 1, 2, 3, 4 (order by creation time, positions may be 0)
	t.Run("InitialOrder", func(t *testing.T) {
		expectedOrder := []idwrap.IDWrap{header1ID, header2ID, header3ID, header4ID}
		contextStr := "InitialOrder"
		verifyHeaderOrder(t, data.ctx, data.ehs, data.exampleID, expectedOrder, &contextStr)
		t.Log("✓ Initial order confirmed: 1, 2, 3, 4 (by creation order)")
	})

	// Move header 4 after header 1 (should result in 1, 4, 2, 3)
	t.Run("MoveHeader4After1_ResultsIn_1_4_2_3", func(t *testing.T) {
		req := &requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       header4ID.Bytes(),
			TargetHeaderId: header1ID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}

		resp, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
		if err != nil {
			t.Fatalf("HeaderMove failed: %v", err)
		}
		if resp == nil {
			t.Fatal("Expected response, got nil")
		}

		// Verify the exact expected order: 1, 4, 2, 3
		expectedOrder := []idwrap.IDWrap{header1ID, header4ID, header2ID, header3ID}
		contextStr := "MoveHeader4After1_ResultsIn_1_4_2_3"
		verifyHeaderOrderWithPositions(t, data.ctx, data.ehs, data.exampleID, expectedOrder, &contextStr)
		
		t.Log("✓ Successfully moved header 4 after header 1")
		t.Log("✓ Result order confirmed: 1, 4, 2, 3")
		t.Log("✓ Linked list integrity maintained")
	})
}

// TestHeaderMoveBasicOperations tests all basic move operations
func TestHeaderMoveBasicOperations(t *testing.T) {
	data := setupOriginHeaderMoveTestData(t)

	// Create 5 headers for comprehensive testing: 1, 2, 3, 4, 5
	header1ID := createOriginTestHeader(t, data, "Header-1", "Value1")
	header2ID := createOriginTestHeader(t, data, "Header-2", "Value2")
	header3ID := createOriginTestHeader(t, data, "Header-3", "Value3")
	header4ID := createOriginTestHeader(t, data, "Header-4", "Value4")
	header5ID := createOriginTestHeader(t, data, "Header-5", "Value5")

	// Verify initial order: 1, 2, 3, 4, 5
	t.Run("InitialState", func(t *testing.T) {
		expectedOrder := []idwrap.IDWrap{header1ID, header2ID, header3ID, header4ID, header5ID}
		assertOrder(t, data.ctx, data.ehs, data.exampleID, expectedOrder, "InitialState")
		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 5, "InitialState")
	})

	// Test 1: Move middle item (3) to different position (after 1)
	// Expected: 1, 3, 2, 4, 5
	t.Run("MoveMiddleToNewPosition", func(t *testing.T) {
		req := &requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       header3ID.Bytes(),
			TargetHeaderId: header1ID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}

		_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
		if err != nil {
			t.Fatalf("HeaderMove failed: %v", err)
		}

		expectedOrder := []idwrap.IDWrap{header1ID, header3ID, header2ID, header4ID, header5ID}
		assertOrder(t, data.ctx, data.ehs, data.exampleID, expectedOrder, "MoveMiddleToNewPosition")
		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 5, "MoveMiddleToNewPosition")
	})

	// Test 2: Move first item (head) to middle
	// Current: 1, 3, 2, 4, 5 -> Move 1 after 4 -> 3, 2, 4, 1, 5
	t.Run("MoveHeadToMiddle", func(t *testing.T) {
		req := &requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       header1ID.Bytes(),
			TargetHeaderId: header4ID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}

		_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
		if err != nil {
			t.Fatalf("HeaderMove failed: %v", err)
		}

		expectedOrder := []idwrap.IDWrap{header3ID, header2ID, header4ID, header1ID, header5ID}
		assertOrder(t, data.ctx, data.ehs, data.exampleID, expectedOrder, "MoveHeadToMiddle")
		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 5, "MoveHeadToMiddle")
	})

	// Test 3: Move last item (tail) to beginning
	// Current: 3, 2, 4, 1, 5 -> Move 5 before 3 -> 5, 3, 2, 4, 1
	t.Run("MoveTailToBeginning", func(t *testing.T) {
		req := &requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       header5ID.Bytes(),
			TargetHeaderId: header3ID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
		}

		_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
		if err != nil {
			t.Fatalf("HeaderMove failed: %v", err)
		}

		expectedOrder := []idwrap.IDWrap{header5ID, header3ID, header2ID, header4ID, header1ID}
		assertOrder(t, data.ctx, data.ehs, data.exampleID, expectedOrder, "MoveTailToBeginning")
		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 5, "MoveTailToBeginning")
	})

	// Test 4: Move item to become new head (before first)
	// Current: 5, 3, 2, 4, 1 -> Move 2 before 5 -> 2, 5, 3, 4, 1
	t.Run("MoveToNewHead", func(t *testing.T) {
		req := &requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       header2ID.Bytes(),
			TargetHeaderId: header5ID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
		}

		_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
		if err != nil {
			t.Fatalf("HeaderMove failed: %v", err)
		}

		expectedOrder := []idwrap.IDWrap{header2ID, header5ID, header3ID, header4ID, header1ID}
		assertOrder(t, data.ctx, data.ehs, data.exampleID, expectedOrder, "MoveToNewHead")
		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 5, "MoveToNewHead")
	})

	// Test 5: Move item to become new tail (after last)
	// Current: 2, 5, 3, 4, 1 -> Move 3 after 1 -> 2, 5, 4, 1, 3
	t.Run("MoveToNewTail", func(t *testing.T) {
		req := &requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       header3ID.Bytes(),
			TargetHeaderId: header1ID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}

		_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
		if err != nil {
			t.Fatalf("HeaderMove failed: %v", err)
		}

		expectedOrder := []idwrap.IDWrap{header2ID, header5ID, header4ID, header1ID, header3ID}
		assertOrder(t, data.ctx, data.ehs, data.exampleID, expectedOrder, "MoveToNewTail")
		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 5, "MoveToNewTail")
	})
}

// TestHeaderMoveEdgeCases tests edge cases and error conditions
func TestHeaderMoveEdgeCases(t *testing.T) {
	data := setupOriginHeaderMoveTestData(t)

	// Create 4 headers for edge case testing: 1, 2, 3, 4
	header1ID := createOriginTestHeader(t, data, "Header-1", "Value1")
	header2ID := createOriginTestHeader(t, data, "Header-2", "Value2")
	header3ID := createOriginTestHeader(t, data, "Header-3", "Value3")
	header4ID := createOriginTestHeader(t, data, "Header-4", "Value4")

	// Test 1: Move item to same position (no-op) - should be allowed but have no effect
	t.Run("MoveToSamePosition", func(t *testing.T) {
		// Get initial order
		initialOrder := getOrderFromLinkedList(t, data.ctx, data.ehs, data.exampleID)
		
		// Move header 2 to its current position (after header 1)
		req := &requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       header2ID.Bytes(),
			TargetHeaderId: header1ID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}

		_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
		if err != nil {
			t.Fatalf("HeaderMove failed on same position move: %v", err)
		}

		// Order should remain unchanged
		assertOrder(t, data.ctx, data.ehs, data.exampleID, initialOrder, "MoveToSamePosition")
		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 4, "MoveToSamePosition")
	})

	// Test 2: Move item right before itself (adjacent move)
	// Current: 1, 2, 3, 4 -> Move 3 before 2 -> 1, 3, 2, 4
	t.Run("MoveBeforeAdjacent", func(t *testing.T) {
		req := &requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       header3ID.Bytes(),
			TargetHeaderId: header2ID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
		}

		_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
		if err != nil {
			t.Fatalf("HeaderMove failed on adjacent move: %v", err)
		}

		expectedOrder := []idwrap.IDWrap{header1ID, header3ID, header2ID, header4ID}
		assertOrder(t, data.ctx, data.ehs, data.exampleID, expectedOrder, "MoveBeforeAdjacent")
		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 4, "MoveBeforeAdjacent")
	})

	// Test 3: Move item right after itself (adjacent move)
	// Current: 1, 3, 2, 4 -> Move 2 after 3 -> 1, 3, 2, 4 (no change, already in position)
	t.Run("MoveAfterAdjacent", func(t *testing.T) {
		// Get initial order
		initialOrder := getOrderFromLinkedList(t, data.ctx, data.ehs, data.exampleID)
		
		req := &requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       header2ID.Bytes(),
			TargetHeaderId: header3ID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}

		_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
		if err != nil {
			t.Fatalf("HeaderMove failed on adjacent move: %v", err)
		}

		// Order should remain unchanged since 2 is already after 3
		assertOrder(t, data.ctx, data.ehs, data.exampleID, initialOrder, "MoveAfterAdjacent")
		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 4, "MoveAfterAdjacent")
	})
}

// TestHeaderMoveErrorCases tests all error conditions
func TestHeaderMoveErrorCases(t *testing.T) {
	data := setupOriginHeaderMoveTestData(t)

	// Create 2 headers for error testing
	header1ID := createOriginTestHeader(t, data, "Header-1", "Value1")
	header2ID := createOriginTestHeader(t, data, "Header-2", "Value2")

	// Test 1: Move header relative to itself (should fail)
	t.Run("MoveHeaderToItself", func(t *testing.T) {
		req := &requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       header1ID.Bytes(),
			TargetHeaderId: header1ID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}

		_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
		if err == nil {
			t.Fatal("Expected error when moving header relative to itself")
		}

		// Should contain the error message
		if !strings.Contains(err.Error(), "cannot move header relative to itself") {
			t.Errorf("Expected error about moving header relative to itself, got: %v", err)
		}

		// Ensure list integrity is maintained after failed operation
		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 2, "MoveHeaderToItselfFailed")
	})

	// Test 2: Move non-existent header
	t.Run("MoveNonExistentHeader", func(t *testing.T) {
		invalidHeaderID := idwrap.NewNow()
		req := &requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       invalidHeaderID.Bytes(),
			TargetHeaderId: header1ID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}

		_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
		if err == nil {
			t.Fatal("Expected error when moving non-existent header")
		}

		// Ensure list integrity is maintained after failed operation
		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 2, "MoveNonExistentHeaderFailed")
	})

	// Test 3: Move to non-existent target
	t.Run("MoveToNonExistentTarget", func(t *testing.T) {
		invalidTargetID := idwrap.NewNow()
		req := &requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       header1ID.Bytes(),
			TargetHeaderId: invalidTargetID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}

		_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
		if err == nil {
			t.Fatal("Expected error when moving to non-existent target")
		}

		// Ensure list integrity is maintained after failed operation
		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 2, "MoveToNonExistentTargetFailed")
	})

	// Test 4: Invalid example ID
	t.Run("InvalidExampleID", func(t *testing.T) {
		invalidExampleID := idwrap.NewNow()
		req := &requestv1.HeaderMoveRequest{
			ExampleId:      invalidExampleID.Bytes(),
			HeaderId:       header1ID.Bytes(),
			TargetHeaderId: header2ID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}

		_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
		if err == nil {
			t.Fatal("Expected error when using invalid example ID")
		}

		// Ensure original list integrity is maintained
		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 2, "InvalidExampleIDFailed")
	})

	// Test 5: Invalid move position (unspecified)
	t.Run("InvalidMovePosition", func(t *testing.T) {
		req := &requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       header1ID.Bytes(),
			TargetHeaderId: header2ID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_UNSPECIFIED,
		}

		_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
		if err == nil {
			t.Fatal("Expected error when using unspecified position")
		}

		// Should contain the error message
		if !strings.Contains(err.Error(), "position must be specified") {
			t.Errorf("Expected error about position being specified, got: %v", err)
		}

		// Ensure list integrity is maintained after failed operation
		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 2, "InvalidMovePositionFailed")
	})

	// Test 6: Invalid header ID bytes
	t.Run("InvalidHeaderIDBytes", func(t *testing.T) {
		req := &requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       []byte("invalid-id"),
			TargetHeaderId: header2ID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}

		_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
		if err == nil {
			t.Fatal("Expected error when using invalid header ID bytes")
		}

		// Should contain the error message
		if !strings.Contains(err.Error(), "invalid header ID") {
			t.Errorf("Expected error about invalid header ID, got: %v", err)
		}

		// Ensure list integrity is maintained after failed operation
		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 2, "InvalidHeaderIDBytesFailec")
	})

	// Test 7: Invalid target header ID bytes
	t.Run("InvalidTargetHeaderIDBytes", func(t *testing.T) {
		req := &requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       header1ID.Bytes(),
			TargetHeaderId: []byte("invalid-target-id"),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}

		_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
		if err == nil {
			t.Fatal("Expected error when using invalid target header ID bytes")
		}

		// Should contain the error message
		if !strings.Contains(err.Error(), "invalid target header ID") {
			t.Errorf("Expected error about invalid target header ID, got: %v", err)
		}

		// Ensure list integrity is maintained after failed operation
		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 2, "InvalidTargetHeaderIDBytesFailec")
	})

	// Test 8: Invalid example ID bytes
	t.Run("InvalidExampleIDBytes", func(t *testing.T) {
		req := &requestv1.HeaderMoveRequest{
			ExampleId:      []byte("invalid-example-id"),
			HeaderId:       header1ID.Bytes(),
			TargetHeaderId: header2ID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}

		_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
		if err == nil {
			t.Fatal("Expected error when using invalid example ID bytes")
		}

		// Should contain the error message
		if !strings.Contains(err.Error(), "invalid example ID") {
			t.Errorf("Expected error about invalid example ID, got: %v", err)
		}

		// Ensure list integrity is maintained after failed operation
		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 2, "InvalidExampleIDBytesFailec")
	})
}

// TestHeaderMoveSingleItemList tests single item list operations
func TestHeaderMoveSingleItemList(t *testing.T) {
	data := setupOriginHeaderMoveTestData(t)

	// Create only one header
	header1ID := createOriginTestHeader(t, data, "Header-1", "Value1")

	t.Run("SingleItemIntegrity", func(t *testing.T) {
		expectedOrder := []idwrap.IDWrap{header1ID}
		assertOrder(t, data.ctx, data.ehs, data.exampleID, expectedOrder, "SingleItemIntegrity")
		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 1, "SingleItemIntegrity")
	})

	// Test moving single item to itself (should fail)
	t.Run("SingleItemMoveToSelf", func(t *testing.T) {
		req := &requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       header1ID.Bytes(),
			TargetHeaderId: header1ID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}

		_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
		if err == nil {
			t.Fatal("Expected error when moving single header to itself")
		}

		if !strings.Contains(err.Error(), "cannot move header relative to itself") {
			t.Errorf("Expected specific error message, got: %v", err)
		}

		// Ensure list integrity is maintained after failed operation
		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 1, "SingleItemMoveToSelfFailed")
	})
}

// TestHeaderMoveTwoItemList tests two item list swaps
func TestHeaderMoveTwoItemList(t *testing.T) {
	data := setupOriginHeaderMoveTestData(t)

	// Create two headers: 1, 2
	header1ID := createOriginTestHeader(t, data, "Header-1", "Value1")
	header2ID := createOriginTestHeader(t, data, "Header-2", "Value2")

	t.Run("InitialTwoItemOrder", func(t *testing.T) {
		expectedOrder := []idwrap.IDWrap{header1ID, header2ID}
		assertOrder(t, data.ctx, data.ehs, data.exampleID, expectedOrder, "InitialTwoItemOrder")
		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 2, "InitialTwoItemOrder")
	})

	// Test swap: Move 2 before 1 -> 2, 1
	t.Run("SwapTwoItems", func(t *testing.T) {
		req := &requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       header2ID.Bytes(),
			TargetHeaderId: header1ID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
		}

		_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
		if err != nil {
			t.Fatalf("HeaderMove failed on two-item swap: %v", err)
		}

		expectedOrder := []idwrap.IDWrap{header2ID, header1ID}
		assertOrder(t, data.ctx, data.ehs, data.exampleID, expectedOrder, "SwapTwoItems")
		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 2, "SwapTwoItems")
	})

	// Test swap back: Move 1 before 2 -> 1, 2
	t.Run("SwapBackTwoItems", func(t *testing.T) {
		req := &requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       header1ID.Bytes(),
			TargetHeaderId: header2ID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
		}

		_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
		if err != nil {
			t.Fatalf("HeaderMove failed on two-item swap back: %v", err)
		}

		expectedOrder := []idwrap.IDWrap{header1ID, header2ID}
		assertOrder(t, data.ctx, data.ehs, data.exampleID, expectedOrder, "SwapBackTwoItems")
		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 2, "SwapBackTwoItems")
	})
}

// TestHeaderMoveEmptyList tests empty list operations
func TestHeaderMoveEmptyList(t *testing.T) {
	data := setupOriginHeaderMoveTestData(t)

	// Test empty list integrity
	t.Run("EmptyListIntegrity", func(t *testing.T) {
		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 0, "EmptyListIntegrity")
	})

	// Test moving non-existent header in empty list
	t.Run("MoveInEmptyList", func(t *testing.T) {
		nonExistentID := idwrap.NewNow()
		anotherNonExistentID := idwrap.NewNow()

		req := &requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       nonExistentID.Bytes(),
			TargetHeaderId: anotherNonExistentID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}

		_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
		if err == nil {
			t.Fatal("Expected error when moving non-existent headers in empty list")
		}

		// Ensure list remains empty and valid
		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 0, "MoveInEmptyListFailed")
	})
}

// TestHeaderMoveIntegrityValidation tests comprehensive integrity validation
func TestHeaderMoveIntegrityValidation(t *testing.T) {
	data := setupOriginHeaderMoveTestData(t)

	// Create 6 headers for comprehensive integrity testing
	headers := make([]idwrap.IDWrap, 6)
	for i := 0; i < 6; i++ {
		headers[i] = createOriginTestHeader(t, data, fmt.Sprintf("Header-%d", i+1), fmt.Sprintf("Value%d", i+1))
	}

	t.Run("ComplexMoveSequence", func(t *testing.T) {
		// Initial: 1, 2, 3, 4, 5, 6
		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 6, "InitialComplexState")

		// Move 6 to second position (after 1): 1, 6, 2, 3, 4, 5
		req1 := &requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       headers[5].Bytes(),
			TargetHeaderId: headers[0].Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}
		_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req1))
		if err != nil {
			t.Fatalf("Move 1 failed: %v", err)
		}
		expectedOrder1 := []idwrap.IDWrap{headers[0], headers[5], headers[1], headers[2], headers[3], headers[4]}
		assertOrder(t, data.ctx, data.ehs, data.exampleID, expectedOrder1, "ComplexMove1")
		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 6, "ComplexMove1")

		// Move 3 to beginning (before 1): 3, 1, 6, 2, 4, 5
		req2 := &requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       headers[2].Bytes(),
			TargetHeaderId: headers[0].Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
		}
		_, err = data.rpc.HeaderMove(data.ctx, connect.NewRequest(req2))
		if err != nil {
			t.Fatalf("Move 2 failed: %v", err)
		}
		expectedOrder2 := []idwrap.IDWrap{headers[2], headers[0], headers[5], headers[1], headers[3], headers[4]}
		assertOrder(t, data.ctx, data.ehs, data.exampleID, expectedOrder2, "ComplexMove2")
		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 6, "ComplexMove2")

		// Move 5 to end (after 4): 3, 1, 6, 2, 4, 5 (already at end, no change expected)
		req3 := &requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       headers[4].Bytes(),
			TargetHeaderId: headers[3].Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}
		_, err = data.rpc.HeaderMove(data.ctx, connect.NewRequest(req3))
		if err != nil {
			t.Fatalf("Move 3 failed: %v", err)
		}
		// Order should remain the same since 5 is already after 4
		assertOrder(t, data.ctx, data.ehs, data.exampleID, expectedOrder2, "ComplexMove3")
		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 6, "ComplexMove3")
	})

	t.Run("NoCircularReferences", func(t *testing.T) {
		// Ensure no circular references exist after complex operations
		orderedHeaders, err := data.ehs.GetHeadersOrdered(data.ctx, data.exampleID)
		if err != nil {
			t.Fatalf("Failed to get ordered headers: %v", err)
		}

		seenIDs := make(map[string]bool)
		for _, header := range orderedHeaders {
			idStr := header.ID.String()
			if seenIDs[idStr] {
				t.Errorf("Circular reference detected: duplicate ID %s", idStr)
			}
			seenIDs[idStr] = true
		}

		t.Logf("✓ No circular references detected in %d headers", len(orderedHeaders))
	})

	t.Run("NoOrphanedHeaders", func(t *testing.T) {
		// All headers should be reachable by walking the list
		orderedHeaders, err := data.ehs.GetHeadersOrdered(data.ctx, data.exampleID)
		if err != nil {
			t.Fatalf("Failed to get ordered headers: %v", err)
		}

		if len(orderedHeaders) != 6 {
			t.Errorf("Expected 6 headers, got %d - some may be orphaned", len(orderedHeaders))
		}

		t.Logf("✓ All %d headers are reachable", len(orderedHeaders))
	})

	t.Run("HeadAndTailIntegrity", func(t *testing.T) {
		orderedHeaders, err := data.ehs.GetHeadersOrdered(data.ctx, data.exampleID)
		if err != nil {
			t.Fatalf("Failed to get ordered headers: %v", err)
		}

		if len(orderedHeaders) > 0 {
			// Head checks
			if orderedHeaders[0].Prev != nil {
				t.Errorf("Head header has non-nil prev pointer: %v", orderedHeaders[0].Prev)
			}

			// Tail checks
			lastIdx := len(orderedHeaders) - 1
			if orderedHeaders[lastIdx].Next != nil {
				t.Errorf("Tail header has non-nil next pointer: %v", orderedHeaders[lastIdx].Next)
			}

			t.Logf("✓ Head and tail integrity confirmed")
		}
	})
}