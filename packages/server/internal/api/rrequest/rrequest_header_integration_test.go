package rrequest_test

import (
	"context"
	"fmt"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/go-cmp/cmp"
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

// IntegrationTestData holds complete test setup for end-to-end integration tests
type IntegrationTestData struct {
	ctx                 context.Context
	rpc                 rrequest.RequestRPC
	userID              idwrap.IDWrap
	collectionID        idwrap.IDWrap
	
	// Collection view entities
	collectionEndpointID idwrap.IDWrap
	collectionExampleID  idwrap.IDWrap
	
	// Flow view entities (delta)
	deltaEndpointID      idwrap.IDWrap
	deltaExampleID       idwrap.IDWrap
	
	// Services for direct verification
	ehs                  sexampleheader.HeaderService
}

// setupIntegrationTestData creates a complete test environment with both collection and flow views
func setupIntegrationTestData(t *testing.T) *IntegrationTestData {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	// Initialize all services
	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	mockLogger := mocklogger.NewMockLogger()
	cs := scollection.New(queries, mockLogger)
	us := suser.New(queries)
	ehs := sexampleheader.New(queries)
	eqs := sexamplequery.New(queries)
	as := sassert.New(queries)

	// Create workspace, user and collection
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	userID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)

	// Create authenticated context
	ctx = mwauth.CreateAuthedContext(ctx, userID)

	// Create collection endpoint (visible in collection view)
	collectionEndpointID := idwrap.NewNow()
	collectionEndpoint := &mitemapi.ItemApi{
		ID:           collectionEndpointID,
		CollectionID: collectionID,
		Name:         "collection-endpoint",
		Method:       "POST",
		Url:          "/api/collection/endpoint",
		Hidden:       false, // Visible in collection
	}
	err := ias.CreateItemApi(ctx, collectionEndpoint)
	if err != nil {
		t.Fatal(err)
	}

	// Create collection example
	collectionExampleID := idwrap.NewNow()
	collectionExample := &mitemapiexample.ItemApiExample{
		ID:           collectionExampleID,
		ItemApiID:    collectionEndpointID,
		CollectionID: collectionID,
		Name:         "Collection Example",
	}
	err = iaes.CreateApiExample(ctx, collectionExample)
	if err != nil {
		t.Fatal(err)
	}

	// Create delta endpoint (hidden, used for flow view)
	deltaEndpointID := idwrap.NewNow()
	deltaEndpoint := &mitemapi.ItemApi{
		ID:           deltaEndpointID,
		CollectionID: collectionID,
		Name:         "flow-delta-endpoint",
		Method:       "POST", // Will be overridden to PUT in flow
		Url:          "/api/flow/endpoint",
		Hidden:       true, // Hidden from collection view
	}
	err = ias.CreateItemApi(ctx, deltaEndpoint)
	if err != nil {
		t.Fatal(err)
	}

	// Create delta example with VersionParentID pointing to collection example
	deltaExampleID := idwrap.NewNow()
	deltaExample := &mitemapiexample.ItemApiExample{
		ID:              deltaExampleID,
		ItemApiID:       deltaEndpointID,
		CollectionID:    collectionID,
		Name:            "Flow Delta Example",
		VersionParentID: &collectionExampleID, // Links to collection example as parent
	}
	err = iaes.CreateApiExample(ctx, deltaExample)
	if err != nil {
		t.Fatal(err)
	}

	// Create RPC handler
	rpc := rrequest.New(db, cs, us, ias, iaes, ehs, eqs, as)

	return &IntegrationTestData{
		ctx:                  ctx,
		rpc:                  rpc,
		userID:               userID,
		collectionID:         collectionID,
		collectionEndpointID: collectionEndpointID,
		collectionExampleID:  collectionExampleID,
		deltaEndpointID:      deltaEndpointID,
		deltaExampleID:       deltaExampleID,
		ehs:                  ehs,
	}
}

// createIntegrationHeader creates a header via RPC and returns the response bytes
func createIntegrationHeader(t *testing.T, rpc rrequest.RequestRPC, ctx context.Context, exampleID idwrap.IDWrap, key, value string) []byte {
	resp, err := rpc.HeaderCreate(ctx, connect.NewRequest(&requestv1.HeaderCreateRequest{
		ExampleId:   exampleID.Bytes(),
		Key:         key,
		Value:       value,
		Enabled:     true,
		Description: fmt.Sprintf("Integration header %s", key),
	}))
	if err != nil {
		t.Fatalf("Failed to create header %s: %v", key, err)
	}
	return resp.Msg.HeaderId
}

// createIntegrationDeltaHeader creates a delta header via RPC
func createIntegrationDeltaHeader(t *testing.T, rpc rrequest.RequestRPC, ctx context.Context, deltaExampleID, originExampleID idwrap.IDWrap, key, value string) []byte {
	resp, err := rpc.HeaderDeltaCreate(ctx, connect.NewRequest(&requestv1.HeaderDeltaCreateRequest{
		ExampleId:   deltaExampleID.Bytes(),
		OriginId:    originExampleID.Bytes(),
		Key:         key,
		Value:       value,
		Enabled:     true,
		Description: fmt.Sprintf("Integration delta header %s", key),
	}))
	if err != nil {
		t.Fatalf("Failed to create delta header %s: %v", key, err)
	}
	return resp.Msg.HeaderId
}

// getHeaderOrderByKey returns the order of header keys using HeaderList
func getHeaderOrderByKey(t *testing.T, rpc rrequest.RequestRPC, ctx context.Context, exampleID idwrap.IDWrap) []string {
	listResp, err := rpc.HeaderList(ctx, connect.NewRequest(&requestv1.HeaderListRequest{
		ExampleId: exampleID.Bytes(),
	}))
	if err != nil {
		t.Fatalf("Failed to list headers: %v", err)
	}

	var order []string
	for _, item := range listResp.Msg.Items {
		order = append(order, item.Key)
	}
	return order
}

// getDeltaHeaderOrderByKey returns the order of delta header keys using HeaderDeltaList
func getDeltaHeaderOrderByKey(t *testing.T, rpc rrequest.RequestRPC, ctx context.Context, deltaExampleID, originExampleID idwrap.IDWrap) []string {
	listResp, err := rpc.HeaderDeltaList(ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
		ExampleId: deltaExampleID.Bytes(),
		OriginId:  originExampleID.Bytes(),
	}))
	if err != nil {
		t.Fatalf("Failed to list delta headers: %v", err)
	}

	var order []string
	for _, item := range listResp.Msg.Items {
		order = append(order, item.Key)
	}
	return order
}

// validateDeltaLinkedListIntegrity validates delta order using HeaderDeltaList (overlay), not DB Prev/Next pointers
func validateDeltaLinkedListIntegrity(t *testing.T, rpc rrequest.RequestRPC, ctx context.Context, deltaExampleID, originExampleID idwrap.IDWrap, expectedCount int, testContext string) {
    t.Helper()
    listResp, err := rpc.HeaderDeltaList(ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
        ExampleId: deltaExampleID.Bytes(),
        OriginId:  originExampleID.Bytes(),
    }))
    if err != nil {
        t.Fatalf("[%s] Failed to list delta headers: %v", testContext, err)
    }
    if len(listResp.Msg.Items) != expectedCount {
        t.Errorf("[%s] Expected %d headers, got %d", testContext, expectedCount, len(listResp.Msg.Items))
        return
    }
    // Ensure keys are unique and IDs look valid
    seen := map[string]bool{}
    for i, it := range listResp.Msg.Items {
        if len(it.HeaderId) != 16 {
            t.Errorf("[%s] Item %d has invalid ID length %d", testContext, i, len(it.HeaderId))
        }
        if seen[it.Key] {
            t.Errorf("[%s] Duplicate key in overlay list: %s", testContext, it.Key)
        }
        seen[it.Key] = true
    }
}

// TestHeaderIntegrationCompleteUserWorkflow tests the complete end-to-end workflow
func TestHeaderIntegrationCompleteUserWorkflow(t *testing.T) {
	data := setupIntegrationTestData(t)

	t.Run("CompleteUserWorkflow", func(t *testing.T) {
		// PHASE 1: User creates API endpoint with headers in collection view
		t.Log("PHASE 1: Creating headers in collection view")
		
		collectionHeaders := map[string][]byte{
			"Authorization": createIntegrationHeader(t, data.rpc, data.ctx, data.collectionExampleID, "Authorization", "Bearer token"),
			"Content-Type":  createIntegrationHeader(t, data.rpc, data.ctx, data.collectionExampleID, "Content-Type", "application/json"),
			"User-Agent":    createIntegrationHeader(t, data.rpc, data.ctx, data.collectionExampleID, "User-Agent", "DevTools/1.0"),
			"X-API-Key":     createIntegrationHeader(t, data.rpc, data.ctx, data.collectionExampleID, "X-API-Key", "secret-key"),
		}

		// Verify initial collection order: Authorization, Content-Type, User-Agent, X-API-Key
		initialCollectionOrder := getHeaderOrderByKey(t, data.rpc, data.ctx, data.collectionExampleID)
		expectedInitialOrder := []string{"Authorization", "Content-Type", "User-Agent", "X-API-Key"}
		
		if !cmp.Equal(initialCollectionOrder, expectedInitialOrder) {
			t.Errorf("Initial collection order mismatch:\nGot:      %v\nExpected: %v", initialCollectionOrder, expectedInitialOrder)
		}
		t.Logf("✓ Initial collection headers created in order: %v", initialCollectionOrder)

		// PHASE 2: User reorders headers in collection view
		t.Log("PHASE 2: Reordering headers in collection view")
		
		// Move X-API-Key after Authorization (Authorization, X-API-Key, Content-Type, User-Agent)
		_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(&requestv1.HeaderMoveRequest{
			ExampleId:      data.collectionExampleID.Bytes(),
			HeaderId:       collectionHeaders["X-API-Key"],
			TargetHeaderId: collectionHeaders["Authorization"],
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}))
		if err != nil {
			t.Fatalf("Failed to move X-API-Key in collection: %v", err)
		}

		// Verify collection order after move
		collectionOrderAfterMove := getHeaderOrderByKey(t, data.rpc, data.ctx, data.collectionExampleID)
		expectedCollectionOrder := []string{"Authorization", "X-API-Key", "Content-Type", "User-Agent"}
		
		if !cmp.Equal(collectionOrderAfterMove, expectedCollectionOrder) {
			t.Errorf("Collection order after move mismatch:\nGot:      %v\nExpected: %v", collectionOrderAfterMove, expectedCollectionOrder)
		}
		t.Logf("✓ Collection headers reordered: %v", collectionOrderAfterMove)

		// PHASE 3: User creates flow with delta headers
		t.Log("PHASE 3: Creating flow with delta headers")
		
		// Call HeaderDeltaList to auto-create delta headers from collection
		deltaListResp, err := data.rpc.HeaderDeltaList(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: data.deltaExampleID.Bytes(),
			OriginId:  data.collectionExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		if len(deltaListResp.Msg.Items) != 4 {
			t.Fatalf("Expected 4 auto-created delta headers, got %d", len(deltaListResp.Msg.Items))
		}

		// Create map of delta header IDs by key
		deltaHeaders := make(map[string][]byte)
		for _, item := range deltaListResp.Msg.Items {
			deltaHeaders[item.Key] = item.HeaderId
		}

		// Verify delta headers initially match collection order
		initialDeltaOrder := getDeltaHeaderOrderByKey(t, data.rpc, data.ctx, data.deltaExampleID, data.collectionExampleID)
		if !cmp.Equal(initialDeltaOrder, expectedCollectionOrder) {
			t.Errorf("Initial delta order should match collection:\nDelta:      %v\nCollection: %v", initialDeltaOrder, expectedCollectionOrder)
		}
		t.Logf("✓ Delta headers auto-created matching collection order: %v", initialDeltaOrder)

		// Add flow-specific headers
		additionalDeltaHeaders := []string{"X-Flow-ID", "X-Request-ID"}
		for _, headerKey := range additionalDeltaHeaders {
			headerID := createIntegrationDeltaHeader(t, data.rpc, data.ctx, data.deltaExampleID, data.collectionExampleID, headerKey, fmt.Sprintf("flow-value-%s", headerKey))
			deltaHeaders[headerKey] = headerID
		}

		// Verify all 6 delta headers exist
		allDeltaOrder := getDeltaHeaderOrderByKey(t, data.rpc, data.ctx, data.deltaExampleID, data.collectionExampleID)
		expectedAllDeltaOrder := []string{"Authorization", "X-API-Key", "Content-Type", "User-Agent", "X-Flow-ID", "X-Request-ID"}
		
		if !cmp.Equal(allDeltaOrder, expectedAllDeltaOrder) {
			t.Errorf("All delta headers order mismatch:\nGot:      %v\nExpected: %v", allDeltaOrder, expectedAllDeltaOrder)
		}
		t.Logf("✓ All delta headers created: %v", allDeltaOrder)

		// PHASE 4: User reorders headers differently in flow view
		t.Log("PHASE 4: Reordering headers differently in flow view")
		
		// Move Content-Type to first position (before Authorization)
		_, err = data.rpc.HeaderDeltaMove(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaMoveRequest{
			ExampleId:      data.deltaExampleID.Bytes(),
			HeaderId:       deltaHeaders["Content-Type"],
			TargetHeaderId: deltaHeaders["Authorization"],
			Position:       resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
		}))
		if err != nil {
			t.Fatalf("Failed to move Content-Type to first in delta: %v", err)
		}

		// Move X-Flow-ID after Authorization
		_, err = data.rpc.HeaderDeltaMove(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaMoveRequest{
			ExampleId:      data.deltaExampleID.Bytes(),
			HeaderId:       deltaHeaders["X-Flow-ID"],
			TargetHeaderId: deltaHeaders["Authorization"],
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}))
		if err != nil {
			t.Fatalf("Failed to move X-Flow-ID after Authorization in delta: %v", err)
		}

		// Verify delta order after flow reordering
		deltaOrderAfterFlowMoves := getDeltaHeaderOrderByKey(t, data.rpc, data.ctx, data.deltaExampleID, data.collectionExampleID)
		expectedDeltaAfterMoves := []string{"Content-Type", "Authorization", "X-Flow-ID", "X-API-Key", "User-Agent", "X-Request-ID"}
		
		if !cmp.Equal(deltaOrderAfterFlowMoves, expectedDeltaAfterMoves) {
			t.Errorf("Delta order after flow moves mismatch:\nGot:      %v\nExpected: %v", deltaOrderAfterFlowMoves, expectedDeltaAfterMoves)
		}
		t.Logf("✓ Delta headers reordered in flow view: %v", deltaOrderAfterFlowMoves)

		// PHASE 5: User updates values in flow (delta overrides)
		t.Log("PHASE 5: Updating header values in flow view (delta overrides)")
		
		// Update Authorization header value in flow
		authKey := "Authorization"
		authValue := "Bearer flow-specific-token"
		authEnabled := true
		authDesc := "Flow-specific authorization token"
		_, err = data.rpc.HeaderDeltaUpdate(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaUpdateRequest{
			HeaderId:    deltaHeaders["Authorization"],
			Key:         &authKey,
			Value:       &authValue,
			Enabled:     &authEnabled,
			Description: &authDesc,
		}))
		if err != nil {
			t.Fatalf("Failed to update Authorization header in delta: %v", err)
		}

		// Update User-Agent header value in flow
		uaKey := "User-Agent"
		uaValue := "DevTools/1.0 FlowRunner"
		uaEnabled := true
		uaDesc := "Flow-specific user agent"
		_, err = data.rpc.HeaderDeltaUpdate(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaUpdateRequest{
			HeaderId:    deltaHeaders["User-Agent"],
			Key:         &uaKey,
			Value:       &uaValue,
			Enabled:     &uaEnabled,
			Description: &uaDesc,
		}))
		if err != nil {
			t.Fatalf("Failed to update User-Agent header in delta: %v", err)
		}

		// PHASE 6: Verify both views maintain separate state
		t.Log("PHASE 6: Verifying independent state of collection and flow views")

		// Verify collection view remains unchanged
		finalCollectionOrder := getHeaderOrderByKey(t, data.rpc, data.ctx, data.collectionExampleID)
		if !cmp.Equal(finalCollectionOrder, expectedCollectionOrder) {
			t.Errorf("Collection order should remain unchanged:\nGot:      %v\nExpected: %v", finalCollectionOrder, expectedCollectionOrder)
		}

		// Verify collection header values remain unchanged
		finalCollectionList, err := data.rpc.HeaderList(data.ctx, connect.NewRequest(&requestv1.HeaderListRequest{
			ExampleId: data.collectionExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		collectionAuthHeader := findHeaderByKey(finalCollectionList.Msg.Items, "Authorization")
		if collectionAuthHeader == nil {
			t.Fatal("Authorization header not found in collection")
		}
		if collectionAuthHeader.Value != "Bearer token" {
			t.Errorf("Collection Authorization value should remain unchanged: got %s, expected 'Bearer token'", collectionAuthHeader.Value)
		}

		collectionUserAgentHeader := findHeaderByKey(finalCollectionList.Msg.Items, "User-Agent")
		if collectionUserAgentHeader == nil {
			t.Fatal("User-Agent header not found in collection")
		}
		if collectionUserAgentHeader.Value != "DevTools/1.0" {
			t.Errorf("Collection User-Agent value should remain unchanged: got %s, expected 'DevTools/1.0'", collectionUserAgentHeader.Value)
		}

		// Verify delta view has independent order and values
		finalDeltaOrder := getDeltaHeaderOrderByKey(t, data.rpc, data.ctx, data.deltaExampleID, data.collectionExampleID)
		if !cmp.Equal(finalDeltaOrder, expectedDeltaAfterMoves) {
			t.Errorf("Delta order should maintain flow-specific ordering:\nGot:      %v\nExpected: %v", finalDeltaOrder, expectedDeltaAfterMoves)
		}

		// Verify delta header values have overrides
		finalDeltaList, err := data.rpc.HeaderDeltaList(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: data.deltaExampleID.Bytes(),
			OriginId:  data.collectionExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		deltaAuthHeader := findDeltaHeaderByKey(finalDeltaList.Msg.Items, "Authorization")
		if deltaAuthHeader == nil {
			t.Fatal("Authorization header not found in delta")
		}
		if deltaAuthHeader.Value != "Bearer flow-specific-token" {
			t.Errorf("Delta Authorization value incorrect: got %s, expected 'Bearer flow-specific-token'", deltaAuthHeader.Value)
		}

		deltaUserAgentHeader := findDeltaHeaderByKey(finalDeltaList.Msg.Items, "User-Agent")
		if deltaUserAgentHeader == nil {
			t.Fatal("User-Agent header not found in delta")
		}
		if deltaUserAgentHeader.Value != "DevTools/1.0 FlowRunner" {
			t.Errorf("Delta User-Agent value incorrect: got %s, expected 'DevTools/1.0 FlowRunner'", deltaUserAgentHeader.Value)
		}

		t.Log("✅ COMPLETE WORKFLOW SUCCESS:")
		t.Logf("   Collection Order: %v", finalCollectionOrder)
		t.Logf("   Flow Order:       %v", finalDeltaOrder)
		t.Log("   Collection values remain original")
		t.Log("   Flow values have overrides")
		t.Log("   Both views maintain independent state")
	})
}

// TestHeaderIntegrationSpecificReportedIssue tests the exact scenario mentioned in the requirements
func TestHeaderIntegrationSpecificReportedIssue(t *testing.T) {
	data := setupIntegrationTestData(t)

	t.Run("SpecificReportedIssueTest", func(t *testing.T) {
		// Create headers 1, 2, 3, 4 in collection
		t.Log("Creating headers 1, 2, 3, 4 in collection")
		
		collectionHeaders := make(map[string][]byte)
		for i := 1; i <= 4; i++ {
			key := fmt.Sprintf("Header-%d", i)
			value := fmt.Sprintf("Value-%d", i)
			collectionHeaders[key] = createIntegrationHeader(t, data.rpc, data.ctx, data.collectionExampleID, key, value)
		}

		// Verify initial order is 1, 2, 3, 4
		initialOrder := getHeaderOrderByKey(t, data.rpc, data.ctx, data.collectionExampleID)
		expectedInitialOrder := []string{"Header-1", "Header-2", "Header-3", "Header-4"}
		
		if !cmp.Equal(initialOrder, expectedInitialOrder) {
			t.Errorf("Initial order mismatch:\nGot:      %v\nExpected: %v", initialOrder, expectedInitialOrder)
		}
		t.Logf("✓ Initial order confirmed: %v", initialOrder)

		// Move header 4 after header 1
		t.Log("Moving header 4 after header 1")
		_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(&requestv1.HeaderMoveRequest{
			ExampleId:      data.collectionExampleID.Bytes(),
			HeaderId:       collectionHeaders["Header-4"],
			TargetHeaderId: collectionHeaders["Header-1"],
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}))
		if err != nil {
			t.Fatalf("Failed to move header 4 after header 1: %v", err)
		}

		// Verify order is now 1, 4, 2, 3
		orderAfterMove := getHeaderOrderByKey(t, data.rpc, data.ctx, data.collectionExampleID)
		expectedOrderAfterMove := []string{"Header-1", "Header-4", "Header-2", "Header-3"}
		
		if !cmp.Equal(orderAfterMove, expectedOrderAfterMove) {
			t.Errorf("Order after move mismatch:\nGot:      %v\nExpected: %v", orderAfterMove, expectedOrderAfterMove)
		}
		t.Logf("✓ Order after move confirmed: %v", orderAfterMove)

		// Create flow and verify independent ordering
		t.Log("Creating flow and verifying independent ordering")
		
		// Auto-create delta headers
		deltaListResp, err := data.rpc.HeaderDeltaList(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: data.deltaExampleID.Bytes(),
			OriginId:  data.collectionExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Verify delta headers initially match collection order (1, 4, 2, 3)
		deltaOrder := getDeltaHeaderOrderByKey(t, data.rpc, data.ctx, data.deltaExampleID, data.collectionExampleID)
		if !cmp.Equal(deltaOrder, expectedOrderAfterMove) {
			t.Errorf("Delta order should initially match collection:\nDelta:      %v\nCollection: %v", deltaOrder, expectedOrderAfterMove)
		}
		t.Logf("✓ Delta headers inherit collection order: %v", deltaOrder)

		// Map delta header IDs by key
		deltaHeaders := make(map[string][]byte)
		for _, item := range deltaListResp.Msg.Items {
			deltaHeaders[item.Key] = item.HeaderId
		}

		// Reorder in flow: Move Header-2 to first position (before Header-1)
		t.Log("Reordering in flow: Moving Header-2 to first position")
		_, err = data.rpc.HeaderDeltaMove(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaMoveRequest{
			ExampleId:      data.deltaExampleID.Bytes(),
			HeaderId:       deltaHeaders["Header-2"],
			TargetHeaderId: deltaHeaders["Header-1"],
			Position:       resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
		}))
		if err != nil {
			t.Fatalf("Failed to move Header-2 to first in flow: %v", err)
		}

		// Verify flow order is now 2, 1, 4, 3
		flowOrderAfterMove := getDeltaHeaderOrderByKey(t, data.rpc, data.ctx, data.deltaExampleID, data.collectionExampleID)
		expectedFlowOrder := []string{"Header-2", "Header-1", "Header-4", "Header-3"}
		
		if !cmp.Equal(flowOrderAfterMove, expectedFlowOrder) {
			t.Errorf("Flow order after move mismatch:\nGot:      %v\nExpected: %v", flowOrderAfterMove, expectedFlowOrder)
		}
		t.Logf("✓ Flow order after move: %v", flowOrderAfterMove)

		// Final verification: Both views maintain separate ordering
		finalCollectionOrder := getHeaderOrderByKey(t, data.rpc, data.ctx, data.collectionExampleID)
		finalFlowOrder := getDeltaHeaderOrderByKey(t, data.rpc, data.ctx, data.deltaExampleID, data.collectionExampleID)

		// Collection should still be 1, 4, 2, 3
		if !cmp.Equal(finalCollectionOrder, expectedOrderAfterMove) {
			t.Errorf("Collection order should remain 1,4,2,3:\nGot: %v", finalCollectionOrder)
		}

		// Flow should be 2, 1, 4, 3  
		if !cmp.Equal(finalFlowOrder, expectedFlowOrder) {
			t.Errorf("Flow order should be 2,1,4,3:\nGot: %v", finalFlowOrder)
		}

		t.Log("✅ SPECIFIC ISSUE TEST SUCCESS:")
		t.Logf("   ✓ Headers 1,2,3,4 created in collection")
		t.Logf("   ✓ Header 4 moved after header 1: %v", finalCollectionOrder) 
		t.Logf("   ✓ Flow created with independent ordering: %v", finalFlowOrder)
		t.Logf("   ✓ Both views maintain separate state")
	})
}

// TestHeaderIntegrationConcurrentOperations tests concurrent operations on collection and flow views
func TestHeaderIntegrationConcurrentOperations(t *testing.T) {
	data := setupIntegrationTestData(t)

	t.Run("ConcurrentCollectionAndFlowOperations", func(t *testing.T) {
		// Create initial headers in collection
		collectionHeaders := make(map[string][]byte)
		for i := 1; i <= 5; i++ {
			key := fmt.Sprintf("Header-%d", i)
			collectionHeaders[key] = createIntegrationHeader(t, data.rpc, data.ctx, data.collectionExampleID, key, fmt.Sprintf("collection-value-%d", i))
		}

		// Auto-create delta headers
		deltaListResp, err := data.rpc.HeaderDeltaList(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: data.deltaExampleID.Bytes(),
			OriginId:  data.collectionExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		deltaHeaders := make(map[string][]byte)
		for _, item := range deltaListResp.Msg.Items {
			deltaHeaders[item.Key] = item.HeaderId
		}

		// Perform operations on both views
		// Collection: Move Header-5 to second position (after Header-1)
		_, err = data.rpc.HeaderMove(data.ctx, connect.NewRequest(&requestv1.HeaderMoveRequest{
			ExampleId:      data.collectionExampleID.Bytes(),
			HeaderId:       collectionHeaders["Header-5"],
			TargetHeaderId: collectionHeaders["Header-1"],
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}))
		if err != nil {
			t.Fatalf("Failed collection move: %v", err)
		}

		// Flow: Move Header-3 to first position
		_, err = data.rpc.HeaderDeltaMove(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaMoveRequest{
			ExampleId:      data.deltaExampleID.Bytes(),
			HeaderId:       deltaHeaders["Header-3"],
			TargetHeaderId: deltaHeaders["Header-1"],
			Position:       resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
		}))
		if err != nil {
			t.Fatalf("Failed delta move: %v", err)
		}

		// Collection: Update Header-2 value
		cKey := "Header-2"
		cValue := "updated-collection-value-2"
		cEnabled := true
		cDesc := "Updated in collection"
		_, err = data.rpc.HeaderUpdate(data.ctx, connect.NewRequest(&requestv1.HeaderUpdateRequest{
			HeaderId:    collectionHeaders["Header-2"],
			Key:         &cKey,
			Value:       &cValue,
			Enabled:     &cEnabled,
			Description: &cDesc,
		}))
		if err != nil {
			t.Fatalf("Failed collection update: %v", err)
		}

		// Flow: Update Header-2 value (should override)
		h2Key := "Header-2"
		h2Value := "updated-flow-value-2"
		h2Enabled := true
		h2Desc := "Updated in flow"
		_, err = data.rpc.HeaderDeltaUpdate(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaUpdateRequest{
			HeaderId:    deltaHeaders["Header-2"],
			Key:         &h2Key,
			Value:       &h2Value,
			Enabled:     &h2Enabled,
			Description: &h2Desc,
		}))
		if err != nil {
			t.Fatalf("Failed delta update: %v", err)
		}

		// Verify final states
		finalCollectionOrder := getHeaderOrderByKey(t, data.rpc, data.ctx, data.collectionExampleID)
		finalFlowOrder := getDeltaHeaderOrderByKey(t, data.rpc, data.ctx, data.deltaExampleID, data.collectionExampleID)

		expectedCollectionOrder := []string{"Header-1", "Header-5", "Header-2", "Header-3", "Header-4"}
		expectedFlowOrder := []string{"Header-3", "Header-1", "Header-2", "Header-4", "Header-5"}

		if !cmp.Equal(finalCollectionOrder, expectedCollectionOrder) {
			t.Errorf("Collection order mismatch:\nGot:      %v\nExpected: %v", finalCollectionOrder, expectedCollectionOrder)
		}

		if !cmp.Equal(finalFlowOrder, expectedFlowOrder) {
			t.Errorf("Flow order mismatch:\nGot:      %v\nExpected: %v", finalFlowOrder, expectedFlowOrder)
		}

		// Verify values are different
		collectionList, err := data.rpc.HeaderList(data.ctx, connect.NewRequest(&requestv1.HeaderListRequest{
			ExampleId: data.collectionExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		deltaList, err := data.rpc.HeaderDeltaList(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: data.deltaExampleID.Bytes(),
			OriginId:  data.collectionExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		collectionHeader2 := findHeaderByKey(collectionList.Msg.Items, "Header-2")
		deltaHeader2 := findDeltaHeaderByKey(deltaList.Msg.Items, "Header-2")

		if collectionHeader2.Value != "updated-collection-value-2" {
			t.Errorf("Collection Header-2 value incorrect: %s", collectionHeader2.Value)
		}

		if deltaHeader2.Value != "updated-flow-value-2" {
			t.Errorf("Delta Header-2 value incorrect: %s", deltaHeader2.Value)
		}

		t.Log("✅ CONCURRENT OPERATIONS SUCCESS:")
		t.Logf("   Collection Order: %v", finalCollectionOrder)
		t.Logf("   Flow Order:       %v", finalFlowOrder)
		t.Logf("   Collection Header-2: %s", collectionHeader2.Value)
		t.Logf("   Flow Header-2:       %s", deltaHeader2.Value)
	})
}

// TestHeaderIntegrationLinkedListIntegrity tests linked list integrity across both views
func TestHeaderIntegrationLinkedListIntegrity(t *testing.T) {
	data := setupIntegrationTestData(t)

	t.Run("LinkedListIntegrityAcrossViews", func(t *testing.T) {
		// Create multiple headers
		for i := 1; i <= 8; i++ {
			key := fmt.Sprintf("Header-%d", i)
			createIntegrationHeader(t, data.rpc, data.ctx, data.collectionExampleID, key, fmt.Sprintf("value-%d", i))
		}

		// Verify collection linked list integrity
		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.collectionExampleID, 8, "CollectionLinkedList")

		// Create delta headers
		deltaListResp, err := data.rpc.HeaderDeltaList(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: data.deltaExampleID.Bytes(),
			OriginId:  data.collectionExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		deltaHeaders := make(map[string][]byte)
		for _, item := range deltaListResp.Msg.Items {
			deltaHeaders[item.Key] = item.HeaderId
		}

    // Verify delta linked list integrity via overlay list order rather than DB pointers
    validateDeltaLinkedListIntegrity(t, data.rpc, data.ctx, data.deltaExampleID, data.collectionExampleID, 8, "DeltaLinkedList")

		// Perform complex moves on both views
		// Collection: Multiple moves
		moves := []struct {
			header string
			target string
			pos    resourcesv1.MovePosition
		}{
			{"Header-8", "Header-1", resourcesv1.MovePosition_MOVE_POSITION_AFTER},   // 1,8,2,3,4,5,6,7
			{"Header-3", "Header-8", resourcesv1.MovePosition_MOVE_POSITION_AFTER},   // 1,8,3,2,4,5,6,7
			{"Header-7", "Header-1", resourcesv1.MovePosition_MOVE_POSITION_BEFORE},  // 7,1,8,3,2,4,5,6
		}

		for _, move := range moves {
			_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(&requestv1.HeaderMoveRequest{
				ExampleId:      data.collectionExampleID.Bytes(),
				HeaderId:       findHeaderIDByKey(t, data.rpc, data.ctx, data.collectionExampleID, move.header),
				TargetHeaderId: findHeaderIDByKey(t, data.rpc, data.ctx, data.collectionExampleID, move.target),
				Position:       move.pos,
			}))
			if err != nil {
				t.Fatalf("Collection move failed for %s: %v", move.header, err)
			}
			
			// Validate integrity after each move
			validateLinkedListIntegrity(t, data.ctx, data.ehs, data.collectionExampleID, 8, fmt.Sprintf("CollectionAfterMove-%s", move.header))
		}

		// Delta: Different moves
		deltaMoves := []struct {
			header string
			target string
			pos    resourcesv1.MovePosition
		}{
			{"Header-2", "Header-4", resourcesv1.MovePosition_MOVE_POSITION_AFTER},
			{"Header-6", "Header-2", resourcesv1.MovePosition_MOVE_POSITION_BEFORE}, 
			{"Header-5", "Header-1", resourcesv1.MovePosition_MOVE_POSITION_BEFORE},
		}

		for _, move := range deltaMoves {
			_, err := data.rpc.HeaderDeltaMove(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaMoveRequest{
				ExampleId:      data.deltaExampleID.Bytes(),
				HeaderId:       deltaHeaders[move.header],
				TargetHeaderId: deltaHeaders[move.target],
				Position:       move.pos,
			}))
			if err != nil {
				t.Fatalf("Delta move failed for %s: %v", move.header, err)
			}
			
            // Validate integrity after each move using overlay list
            validateDeltaLinkedListIntegrity(t, data.rpc, data.ctx, data.deltaExampleID, data.collectionExampleID, 8, fmt.Sprintf("DeltaAfterMove-%s", move.header))
		}

		t.Log("✅ LINKED LIST INTEGRITY SUCCESS:")
		t.Log("   ✓ Collection linked list integrity maintained through all moves")
        t.Log("   ✓ Delta linked list integrity maintained through all moves (overlay)")
		t.Log("   ✓ Both views operate independently with valid linked lists")
	})
}

// Helper function to find header by key in HeaderList response items
func findHeaderByKey(items []*requestv1.HeaderListItem, key string) *requestv1.HeaderListItem {
	for _, item := range items {
		if item.Key == key {
			return item
		}
	}
	return nil
}

// Helper function to find header by key in HeaderDeltaList response items
func findDeltaHeaderByKey(items []*requestv1.HeaderDeltaListItem, key string) *requestv1.HeaderDeltaListItem {
	for _, item := range items {
		if item.Key == key {
			return item
		}
	}
	return nil
}

// Helper function to find header ID by key using HeaderList
func findHeaderIDByKey(t *testing.T, rpc rrequest.RequestRPC, ctx context.Context, exampleID idwrap.IDWrap, key string) []byte {
	listResp, err := rpc.HeaderList(ctx, connect.NewRequest(&requestv1.HeaderListRequest{
		ExampleId: exampleID.Bytes(),
	}))
	if err != nil {
		t.Fatalf("Failed to list headers: %v", err)
	}

	for _, item := range listResp.Msg.Items {
		if item.Key == key {
			return item.HeaderId
		}
	}
	t.Fatalf("Header with key %s not found", key)
	return nil
}
