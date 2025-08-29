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

// e2eTestData holds test setup for E2E tests
type e2eTestData struct {
	ctx             context.Context
	rpc             rrequest.RequestRPC
	originExampleID idwrap.IDWrap
	deltaExampleID  idwrap.IDWrap
	userID          idwrap.IDWrap
}

// setupE2ETestData creates the test setup
func setupE2ETestData(t *testing.T) *e2eTestData {
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

	// Create workspace, user and collection using helper
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	userID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)

	// Create authenticated context
	ctx = mwauth.CreateAuthedContext(ctx, userID)

	// Create origin endpoint
	originEndpointID := idwrap.NewNow()
	originEndpoint := &mitemapi.ItemApi{
		ID:           originEndpointID,
		CollectionID: collectionID,
		Name:         "test-origin",
		Method:       "GET",
		Url:          "/test-origin",
		Hidden:       false,
	}
	err := ias.CreateItemApi(ctx, originEndpoint)
	if err != nil {
		t.Fatal(err)
	}

	// Create origin example
	originExampleID := idwrap.NewNow()
	originExample := &mitemapiexample.ItemApiExample{
		ID:           originExampleID,
		ItemApiID:    originEndpointID,
		CollectionID: collectionID,
		Name:         "Origin Example",
	}
	err = iaes.CreateApiExample(ctx, originExample)
	if err != nil {
		t.Fatal(err)
	}

	// Create delta endpoint (hidden)
	deltaEndpointID := idwrap.NewNow()
	deltaEndpoint := &mitemapi.ItemApi{
		ID:           deltaEndpointID,
		CollectionID: collectionID,
		Name:         "test-delta",
		Method:       "GET",
		Url:          "/test-delta",
		Hidden:       true,
	}
	err = ias.CreateItemApi(ctx, deltaEndpoint)
	if err != nil {
		t.Fatal(err)
	}

	// Create delta example with VersionParentID
	deltaExampleID := idwrap.NewNow()
	deltaExample := &mitemapiexample.ItemApiExample{
		ID:              deltaExampleID,
		ItemApiID:       deltaEndpointID,
		CollectionID:    collectionID,
		Name:            "Delta Example",
		VersionParentID: &originExampleID,
	}
	err = iaes.CreateApiExample(ctx, deltaExample)
	if err != nil {
		t.Fatal(err)
	}

	// Create RPC handler
	rpc := rrequest.New(db, cs, us, ias, iaes, ehs, eqs, as)

	return &e2eTestData{
		ctx:             ctx,
		rpc:             rpc,
		originExampleID: originExampleID,
		deltaExampleID:  deltaExampleID,
		userID:          userID,
	}
}

// createE2ETestHeader creates a header via RPC
func createE2ETestHeader(t *testing.T, rpc rrequest.RequestRPC, ctx context.Context, exampleID idwrap.IDWrap, key, value string) []byte {
	resp, err := rpc.HeaderCreate(ctx, connect.NewRequest(&requestv1.HeaderCreateRequest{
		ExampleId:   exampleID.Bytes(),
		Key:         key,
		Value:       value,
		Enabled:     true,
		Description: fmt.Sprintf("Header %s", key),
	}))
	if err != nil {
		t.Fatalf("Failed to create header %s: %v", key, err)
	}
	return resp.Msg.HeaderId
}

// TestHeaderE2ECompleteFlow tests the complete flow: create, move, and list headers
func TestHeaderE2ECompleteFlow(t *testing.T) {
	// Setup
	data := setupE2ETestData(t)

	t.Run("CompleteFlowWithDeltaHeaders", func(t *testing.T) {
		// Step 1: Create origin headers (1, 2, 3)
		t.Log("Step 1: Creating 3 origin headers")
		originHeaders := []string{"Header-1", "Header-2", "Header-3"}
		originHeaderIDs := make([][]byte, 0)

		for i, key := range originHeaders {
			headerID := createE2ETestHeader(t, data.rpc, data.ctx, data.originExampleID, key, fmt.Sprintf("value-%d", i+1))
			originHeaderIDs = append(originHeaderIDs, headerID)
			t.Logf("Created origin header %s with ID: %x", key, headerID)
		}

		// Step 2: Verify origin headers are in correct order via HeaderList
		t.Log("Step 2: Verifying origin headers order via HeaderList")
		listResp, err := data.rpc.HeaderList(data.ctx, connect.NewRequest(&requestv1.HeaderListRequest{
			ExampleId: data.originExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		if len(listResp.Msg.Items) != 3 {
			t.Fatalf("Expected 3 headers, got %d", len(listResp.Msg.Items))
		}

		for i, item := range listResp.Msg.Items {
			expectedKey := fmt.Sprintf("Header-%d", i+1)
			if item.Key != expectedKey {
				t.Errorf("Position %d: expected key %s, got %s", i, expectedKey, item.Key)
			}
			t.Logf("Origin header at position %d: %s", i, item.Key)
		}

		// Step 3: Call HeaderDeltaList to auto-create delta headers
		t.Log("Step 3: Auto-creating delta headers via HeaderDeltaList")
		deltaListResp, err := data.rpc.HeaderDeltaList(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: data.deltaExampleID.Bytes(),
			OriginId:  data.originExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		if len(deltaListResp.Msg.Items) != 3 {
			t.Fatalf("Expected 3 delta headers, got %d", len(deltaListResp.Msg.Items))
		}

		// Map delta headers by key
		deltaHeaderMap := make(map[string][]byte)
		for _, item := range deltaListResp.Msg.Items {
			deltaHeaderMap[item.Key] = item.HeaderId
			t.Logf("Delta header: %s (ID: %x)", item.Key, item.HeaderId)
		}

		// Step 4: Create additional delta headers (4, 5, 6, 7, 8, 9, 10)
		t.Log("Step 4: Creating 7 additional delta headers")
		for i := 4; i <= 10; i++ {
			key := fmt.Sprintf("Header-%d", i)
			_, err := data.rpc.HeaderDeltaCreate(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaCreateRequest{
				ExampleId:   data.deltaExampleID.Bytes(),
				OriginId:    data.originExampleID.Bytes(),
				Key:         key,
				Value:       fmt.Sprintf("value-%d", i),
				Enabled:     true,
				Description: fmt.Sprintf("Delta header %d", i),
			}))
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("Created delta header %s", key)
		}

		// Step 5: Get the updated list to have all header IDs
		t.Log("Step 5: Getting updated delta header list")
		deltaListResp2, err := data.rpc.HeaderDeltaList(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: data.deltaExampleID.Bytes(),
			OriginId:  data.originExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		if len(deltaListResp2.Msg.Items) != 10 {
			t.Fatalf("Expected 10 delta headers, got %d", len(deltaListResp2.Msg.Items))
		}

		// Update map with all headers
		for _, item := range deltaListResp2.Msg.Items {
			deltaHeaderMap[item.Key] = item.HeaderId
		}

		// Verify initial order is 1,2,3,4,5,6,7,8,9,10
		t.Log("Initial order verification:")
		for i, item := range deltaListResp2.Msg.Items {
			expectedKey := fmt.Sprintf("Header-%d", i+1)
			if item.Key != expectedKey {
				t.Errorf("Position %d: expected %s, got %s", i, expectedKey, item.Key)
			}
			t.Logf("Position %d: %s", i, item.Key)
		}

		// Step 6: Perform moves to create complex ordering
		t.Log("Step 6: Performing complex header moves")

		moves := []struct {
			header string
			after  string
			desc   string
		}{
			{"Header-10", "Header-1", "Move 10 after 1"},   // 1,10,2,3,4,5,6,7,8,9
			{"Header-5", "Header-10", "Move 5 after 10"},   // 1,10,5,2,3,4,6,7,8,9
			{"Header-7", "Header-2", "Move 7 after 2"},     // 1,10,5,2,7,3,4,6,8,9
			{"Header-9", "Header-3", "Move 9 after 3"},     // 1,10,5,2,7,3,9,4,6,8
			{"Header-8", "Header-5", "Move 8 after 5"},     // 1,10,5,8,2,7,3,9,4,6
		}

		for _, move := range moves {
			t.Logf("Performing: %s", move.desc)
			_, err := data.rpc.HeaderDeltaMove(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaMoveRequest{
				ExampleId:      data.deltaExampleID.Bytes(),
				HeaderId:       deltaHeaderMap[move.header],
				TargetHeaderId: deltaHeaderMap[move.after],
				Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			}))
			if err != nil {
				t.Fatalf("Failed to %s: %v", move.desc, err)
			}
		}

		// Step 7: Verify final order via HeaderDeltaList
		t.Log("Step 7: Verifying final order after moves")
		finalListResp, err := data.rpc.HeaderDeltaList(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: data.deltaExampleID.Bytes(),
			OriginId:  data.originExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		expectedFinalOrder := []string{
			"Header-1", "Header-10", "Header-5", "Header-8",
			"Header-2", "Header-7", "Header-3", "Header-9",
			"Header-4", "Header-6",
		}

		t.Log("Final order after moves:")
		actualOrder := make([]string, 0)
		for i, item := range finalListResp.Msg.Items {
			actualOrder = append(actualOrder, item.Key)
			t.Logf("Position %d: %s (expected: %s)", i, item.Key, expectedFinalOrder[i])
		}

		if !cmp.Equal(actualOrder, expectedFinalOrder) {
			t.Errorf("Final order mismatch:\nGot:      %v\nExpected: %v", actualOrder, expectedFinalOrder)
		}

		// Step 8: Verify origin headers remain unchanged
		t.Log("Step 8: Verifying origin headers remain unchanged")
		originListResp, err := data.rpc.HeaderList(data.ctx, connect.NewRequest(&requestv1.HeaderListRequest{
			ExampleId: data.originExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		if len(originListResp.Msg.Items) != 3 {
			t.Errorf("Origin should still have 3 headers, got %d", len(originListResp.Msg.Items))
		}

		for i, item := range originListResp.Msg.Items {
			expectedKey := fmt.Sprintf("Header-%d", i+1)
			if item.Key != expectedKey {
				t.Errorf("Origin position %d: expected %s, got %s", i, expectedKey, item.Key)
			}
		}

		t.Log("✅ Complete flow test passed: Headers created, moved, and listed correctly!")
	})
}

// TestHeaderDeltaCreateAndMoveStressTest creates many headers and performs random moves
func TestHeaderDeltaCreateAndMoveStressTest(t *testing.T) {
	data := setupE2ETestData(t)

	t.Run("StressTestWith20Headers", func(t *testing.T) {
		// Create 20 delta headers
		t.Log("Creating 20 delta headers...")
		headerIDs := make(map[int][]byte)

		for i := 1; i <= 20; i++ {
			resp, err := data.rpc.HeaderDeltaCreate(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaCreateRequest{
				ExampleId:   data.deltaExampleID.Bytes(),
				OriginId:    data.originExampleID.Bytes(),
				Key:         fmt.Sprintf("Header-%02d", i),
				Value:       fmt.Sprintf("value-%d", i),
				Enabled:     true,
				Description: fmt.Sprintf("Header number %d", i),
			}))
			if err != nil {
				t.Fatalf("Failed to create header %d: %v", i, err)
			}
			headerIDs[i] = resp.Msg.HeaderId
		}

		// Verify all 20 headers are created
		listResp, err := data.rpc.HeaderDeltaList(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: data.deltaExampleID.Bytes(),
			OriginId:  data.originExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		if len(listResp.Msg.Items) != 20 {
			t.Fatalf("Expected 20 headers, got %d", len(listResp.Msg.Items))
		}

		// Perform 10 random moves
		t.Log("Performing 10 random moves...")
		moves := []struct {
			from int
			to   int
		}{
			{20, 1}, {15, 3}, {10, 5}, {8, 12}, {1, 19},
			{5, 10}, {13, 2}, {18, 7}, {3, 15}, {11, 4},
		}

		for _, move := range moves {
			t.Logf("Moving Header-%02d after Header-%02d", move.from, move.to)
			_, err := data.rpc.HeaderDeltaMove(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaMoveRequest{
				ExampleId:      data.deltaExampleID.Bytes(),
				HeaderId:       headerIDs[move.from],
				TargetHeaderId: headerIDs[move.to],
				Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			}))
			if err != nil {
				t.Fatalf("Failed to move header %d after %d: %v", move.from, move.to, err)
			}
		}

		// Get final order and verify integrity
		finalResp, err := data.rpc.HeaderDeltaList(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: data.deltaExampleID.Bytes(),
			OriginId:  data.originExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Verify we still have 20 headers
		if len(finalResp.Msg.Items) != 20 {
			t.Fatalf("Headers lost during moves! Expected 20, got %d", len(finalResp.Msg.Items))
		}

		// Verify all header keys are unique
		seen := make(map[string]bool)
		for i, item := range finalResp.Msg.Items {
			if seen[item.Key] {
				t.Errorf("Duplicate header key found: %s", item.Key)
			}
			seen[item.Key] = true
			t.Logf("Final position %d: %s", i, item.Key)
		}

		t.Log("✅ Stress test passed: 20 headers created and moved successfully!")
	})
}