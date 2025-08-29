package rrequest_test

import (
	"context"
	"fmt"
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
	deltav1 "the-dev-tools/spec/dist/buf/go/delta/v1"
	resourcesv1 "the-dev-tools/spec/dist/buf/go/resources/v1"
)

// Helper functions for pointer values in proto requests
var (
	stringPtrFlow = func(s string) *string { return &s }
	boolPtrFlow   = func(b bool) *bool { return &b }
)

// flowTestData contains all the setup data for comprehensive flow header testing
type flowTestData struct {
	ctx              context.Context
	rpc              rrequest.RequestRPC
	originExampleID  idwrap.IDWrap
	deltaExampleID   idwrap.IDWrap
	userID           idwrap.IDWrap
	ehs              sexampleheader.HeaderService
	iaes             sitemapiexample.ItemApiExampleService
	ias              sitemapi.ItemApiService
	
	// For testing cascade behavior
	secondDeltaExampleID idwrap.IDWrap
}

// setupFlowTestData creates test data for comprehensive flow header testing
func setupFlowTestData(t *testing.T) *flowTestData {
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

	// Create workspace and collection
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	userID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)

	// Create origin API endpoint
	originItem := &mitemapi.ItemApi{
		ID:           idwrap.NewNow(),
		CollectionID: collectionID,
		Name:         "origin-endpoint",
		Method:       "GET",
		Url:          "https://api.test.com/origin",
	}
	err := ias.CreateItemApi(ctx, originItem)
	if err != nil {
		t.Fatal(err)
	}

	// Create delta API endpoint (hidden)
	deltaItem := &mitemapi.ItemApi{
		ID:           idwrap.NewNow(),
		CollectionID: collectionID,
		Name:         "delta-endpoint",
		Method:       "GET",
		Url:          "https://api.test.com/delta",
		Hidden:       true,
	}
	err = ias.CreateItemApi(ctx, deltaItem)
	if err != nil {
		t.Fatal(err)
	}

	// Create second delta API endpoint for cascade testing
	secondDeltaItem := &mitemapi.ItemApi{
		ID:           idwrap.NewNow(),
		CollectionID: collectionID,
		Name:         "second-delta-endpoint",
		Method:       "GET",
		Url:          "https://api.test.com/second-delta",
		Hidden:       true,
	}
	err = ias.CreateItemApi(ctx, secondDeltaItem)
	if err != nil {
		t.Fatal(err)
	}

	// Create origin example
	originExampleID := idwrap.NewNow()
	originExample := &mitemapiexample.ItemApiExample{
		ID:           originExampleID,
		ItemApiID:    originItem.ID,
		CollectionID: collectionID,
		Name:         "origin-example",
	}
	err = iaes.CreateApiExample(ctx, originExample)
	if err != nil {
		t.Fatal(err)
	}

	// Create delta example (with VersionParentID pointing to origin)
	deltaExampleID := idwrap.NewNow()
	deltaExample := &mitemapiexample.ItemApiExample{
		ID:              deltaExampleID,
		ItemApiID:       deltaItem.ID,
		CollectionID:    collectionID,
		Name:            "delta-example",
		VersionParentID: &originExampleID,
	}
	err = iaes.CreateApiExample(ctx, deltaExample)
	if err != nil {
		t.Fatal(err)
	}

	// Create second delta example for cascade testing
	secondDeltaExampleID := idwrap.NewNow()
	secondDeltaExample := &mitemapiexample.ItemApiExample{
		ID:              secondDeltaExampleID,
		ItemApiID:       secondDeltaItem.ID,
		CollectionID:    collectionID,
		Name:            "second-delta-example",
		VersionParentID: &originExampleID,
	}
	err = iaes.CreateApiExample(ctx, secondDeltaExample)
	if err != nil {
		t.Fatal(err)
	}

	rpc := rrequest.New(db, cs, us, ias, iaes, ehs, eqs, as)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	return &flowTestData{
		ctx:                  authedCtx,
		rpc:                  rpc,
		originExampleID:      originExampleID,
		deltaExampleID:       deltaExampleID,
		secondDeltaExampleID: secondDeltaExampleID,
		userID:               userID,
		ehs:                  ehs,
		iaes:                 iaes,
		ias:                  ias,
	}
}

// createOriginHeader creates a header in the origin example using regular HeaderCreate
func createOriginHeader(t *testing.T, data *flowTestData, key, value string) idwrap.IDWrap {
	t.Helper()
	resp, err := data.rpc.HeaderCreate(data.ctx, connect.NewRequest(&requestv1.HeaderCreateRequest{
		ExampleId:   data.originExampleID.Bytes(),
		Key:         key,
		Value:       value,
		Enabled:     true,
		Description: fmt.Sprintf("Origin header: %s", key),
	}))
	if err != nil {
		t.Fatalf("Failed to create origin header %s: %v", key, err)
	}
	headerID, err := idwrap.NewFromBytes(resp.Msg.HeaderId)
	if err != nil {
		t.Fatalf("Failed to parse origin header ID: %v", err)
	}
	return headerID
}

// createDeltaHeader creates a header in the delta example using HeaderDeltaCreate
func createDeltaHeader(t *testing.T, data *flowTestData, key, value string) idwrap.IDWrap {
	t.Helper()
	resp, err := data.rpc.HeaderDeltaCreate(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaCreateRequest{
		ExampleId:   data.deltaExampleID.Bytes(),
		OriginId:    data.originExampleID.Bytes(),
		Key:         key,
		Value:       value,
		Enabled:     true,
		Description: fmt.Sprintf("Delta header: %s", key),
	}))
	if err != nil {
		t.Fatalf("Failed to create delta header %s: %v", key, err)
	}
	headerID, err := idwrap.NewFromBytes(resp.Msg.HeaderId)
	if err != nil {
		t.Fatalf("Failed to parse delta header ID: %v", err)
	}
	return headerID
}

// verifyFlowHeaderOrder verifies that headers are in the expected order (renamed to avoid conflict)
func verifyFlowHeaderOrder(t *testing.T, ctx context.Context, headerService sexampleheader.HeaderService, exampleID idwrap.IDWrap, expectedOrder []idwrap.IDWrap, testContext string) {
	t.Helper()
	
	orderedHeaders, err := headerService.GetHeadersOrdered(ctx, exampleID)
	if err != nil {
		t.Fatalf("[%s] Failed to get ordered headers: %v", testContext, err)
	}

	if len(orderedHeaders) != len(expectedOrder) {
		t.Fatalf("[%s] Expected %d headers, got %d", testContext, len(expectedOrder), len(orderedHeaders))
	}

	for i, expected := range expectedOrder {
		if orderedHeaders[i].ID.Compare(expected) != 0 {
			t.Errorf("[%s] Position %d: expected %s, got %s", testContext, i, expected.String(), orderedHeaders[i].ID.String())
		}
	}
}

// verifySourceType verifies that a header has the expected source type
func verifySourceType(t *testing.T, items []*requestv1.HeaderDeltaListItem, headerID idwrap.IDWrap, expectedSource deltav1.SourceKind, testContext string) {
	t.Helper()
	
	for _, item := range items {
		itemID, err := idwrap.NewFromBytes(item.HeaderId)
		if err != nil {
			continue
		}
		if itemID.Compare(headerID) == 0 {
			if item.Source == nil || *item.Source != expectedSource {
				actualSource := deltav1.SourceKind_SOURCE_KIND_UNSPECIFIED
				if item.Source != nil {
					actualSource = *item.Source
				}
				t.Errorf("[%s] Header %s expected source %v, got %v", testContext, headerID.String()[:8], expectedSource, actualSource)
			}
			return
		}
	}
	t.Errorf("[%s] Header %s not found in list", testContext, headerID.String()[:8])
}

// TestHeaderDeltaCreate tests delta header creation functionality
func TestHeaderDeltaCreate(t *testing.T) {
	data := setupFlowTestData(t)

	t.Run("AutoCreationFromOrigin", func(t *testing.T) {
		// Create headers in origin example
		originH1ID := createOriginHeader(t, data, "Authorization", "Bearer origin-token")
		originH2ID := createOriginHeader(t, data, "Content-Type", "application/json")
		originH3ID := createOriginHeader(t, data, "X-Custom", "origin-value")

		// Call HeaderDeltaList which should auto-create delta headers from origin
		listResp, err := data.rpc.HeaderDeltaList(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: data.deltaExampleID.Bytes(),
			OriginId:  data.originExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatalf("HeaderDeltaList failed: %v", err)
		}

		// Verify 3 headers were auto-created
		if len(listResp.Msg.Items) != 3 {
			t.Fatalf("Expected 3 auto-created headers, got %d", len(listResp.Msg.Items))
		}

		// Verify headers exist in delta example and have correct parent references
		deltaHeaders, err := data.ehs.GetHeaderByExampleID(data.ctx, data.deltaExampleID)
		if err != nil {
			t.Fatal(err)
		}

		if len(deltaHeaders) != 3 {
			t.Fatalf("Expected 3 delta headers in database, got %d", len(deltaHeaders))
		}

		// Verify each delta header has correct DeltaParentID
		originIDs := map[idwrap.IDWrap]bool{
			originH1ID: true,
			originH2ID: true,
			originH3ID: true,
		}
		
		for _, deltaHeader := range deltaHeaders {
			if deltaHeader.DeltaParentID == nil {
				t.Error("Auto-created delta header missing DeltaParentID")
				continue
			}
			
			if !originIDs[*deltaHeader.DeltaParentID] {
				t.Errorf("Delta header has unexpected DeltaParentID: %s", deltaHeader.DeltaParentID.String())
			}
		}
	})

	t.Run("ManualCreation", func(t *testing.T) {
		// Create a completely new header in delta example (no origin counterpart)
		newHeaderID := createDeltaHeader(t, data, "X-New-Header", "new-value")

		// Verify it was created
		newHeader, err := data.ehs.GetHeaderByID(data.ctx, newHeaderID)
		if err != nil {
			t.Fatal(err)
		}

		// Verify it has no DeltaParentID (it's a standalone delta header)
		if newHeader.DeltaParentID != nil {
			t.Error("Manually created delta header should not have DeltaParentID")
		}

		// Verify it appears in delta list
		listResp, err := data.rpc.HeaderDeltaList(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: data.deltaExampleID.Bytes(),
			OriginId:  data.originExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Find our new header in the list
		found := false
		for _, item := range listResp.Msg.Items {
			itemID, err := idwrap.NewFromBytes(item.HeaderId)
			if err != nil {
				continue
			}
			if itemID.Compare(newHeaderID) == 0 {
				found = true
				// Verify it's marked as DELTA source
				if item.Source == nil || *item.Source != deltav1.SourceKind_SOURCE_KIND_DELTA {
					actualSource := deltav1.SourceKind_SOURCE_KIND_UNSPECIFIED
					if item.Source != nil {
						actualSource = *item.Source
					}
					t.Errorf("Expected DELTA source, got %v", actualSource)
				}
				break
			}
		}
		if !found {
			t.Error("Manually created header not found in delta list")
		}
	})

	t.Run("PreserveOrderingOnAutoCreation", func(t *testing.T) {
		// Clear existing data for this test
		data2 := setupFlowTestData(t)
		
		// Create headers in specific order in origin
		h1ID := createOriginHeader(t, data2, "Header-1", "Value1")
		h2ID := createOriginHeader(t, data2, "Header-2", "Value2")
		h3ID := createOriginHeader(t, data2, "Header-3", "Value3")
		h4ID := createOriginHeader(t, data2, "Header-4", "Value4")

		// Verify origin order
		verifyFlowHeaderOrder(t, data2.ctx, data2.ehs, data2.originExampleID, []idwrap.IDWrap{h1ID, h2ID, h3ID, h4ID}, "OriginBeforeAutoCreate")

		// Auto-create by calling HeaderDeltaList
		_, err := data2.rpc.HeaderDeltaList(data2.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: data2.deltaExampleID.Bytes(),
			OriginId:  data2.originExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Get delta headers and verify they preserve origin order
		deltaHeaders, err := data2.ehs.GetHeadersOrdered(data2.ctx, data2.deltaExampleID)
		if err != nil {
			t.Fatal(err)
		}

		if len(deltaHeaders) != 4 {
			t.Fatalf("Expected 4 delta headers, got %d", len(deltaHeaders))
		}

		// Verify delta headers preserve the order by checking their parent IDs in sequence
		expectedParentOrder := []idwrap.IDWrap{h1ID, h2ID, h3ID, h4ID}
		for i, deltaHeader := range deltaHeaders {
			if deltaHeader.DeltaParentID == nil {
				t.Errorf("Delta header at position %d missing DeltaParentID", i)
				continue
			}
			if deltaHeader.DeltaParentID.Compare(expectedParentOrder[i]) != 0 {
				t.Errorf("Delta header at position %d has wrong parent: expected %s, got %s", 
					i, expectedParentOrder[i].String(), deltaHeader.DeltaParentID.String())
			}
		}
	})
}

// TestHeaderDeltaList tests delta header listing functionality
func TestHeaderDeltaList(t *testing.T) {
	data := setupFlowTestData(t)

	t.Run("IndependentOrderingFromOrigin", func(t *testing.T) {
		// Create headers in origin
		originH1ID := createOriginHeader(t, data, "H1", "V1")
		originH2ID := createOriginHeader(t, data, "H2", "V2")
		originH3ID := createOriginHeader(t, data, "H3", "V3")
		originH4ID := createOriginHeader(t, data, "H4", "V4")

		// Auto-create delta headers
		_, err := data.rpc.HeaderDeltaList(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: data.deltaExampleID.Bytes(),
			OriginId:  data.originExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Get delta headers to find their IDs
		deltaHeaders, err := data.ehs.GetHeaderByExampleID(data.ctx, data.deltaExampleID)
		if err != nil {
			t.Fatal(err)
		}

		// Create a map from parent ID to delta ID
		parentToDelta := make(map[idwrap.IDWrap]idwrap.IDWrap)
		for _, dh := range deltaHeaders {
			if dh.DeltaParentID != nil {
				parentToDelta[*dh.DeltaParentID] = dh.ID
			}
		}

		deltaH1ID := parentToDelta[originH1ID]
		deltaH2ID := parentToDelta[originH2ID]
		deltaH3ID := parentToDelta[originH3ID]
		deltaH4ID := parentToDelta[originH4ID]

		// Move delta headers to different order: H1, H4, H2, H3
		_, err = data.rpc.HeaderDeltaMove(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaMoveRequest{
			ExampleId:      data.deltaExampleID.Bytes(),
			HeaderId:       deltaH4ID.Bytes(),
			TargetHeaderId: deltaH1ID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Verify origin order unchanged
		verifyFlowHeaderOrder(t, data.ctx, data.ehs, data.originExampleID, 
			[]idwrap.IDWrap{originH1ID, originH2ID, originH3ID, originH4ID}, "OriginOrderUnchanged")

		// Verify delta order changed
		verifyFlowHeaderOrder(t, data.ctx, data.ehs, data.deltaExampleID, 
			[]idwrap.IDWrap{deltaH1ID, deltaH4ID, deltaH2ID, deltaH3ID}, "DeltaOrderChanged")

		// Verify HeaderDeltaList returns correct order for delta
		listResp, err := data.rpc.HeaderDeltaList(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: data.deltaExampleID.Bytes(),
			OriginId:  data.originExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Check that list reflects delta order, not origin order
		expectedDeltaOrder := []idwrap.IDWrap{deltaH1ID, deltaH4ID, deltaH2ID, deltaH3ID}
		if len(listResp.Msg.Items) != len(expectedDeltaOrder) {
			t.Fatalf("Expected %d items, got %d", len(expectedDeltaOrder), len(listResp.Msg.Items))
		}

		for i, expectedID := range expectedDeltaOrder {
			actualID, err := idwrap.NewFromBytes(listResp.Msg.Items[i].HeaderId)
			if err != nil {
				t.Fatal(err)
			}
			if actualID.Compare(expectedID) != 0 {
				t.Errorf("Position %d: expected %s, got %s", i, expectedID.String()[:8], actualID.String()[:8])
			}
		}
	})

	t.Run("MixedOriginAndDeltaHeaders", func(t *testing.T) {
		data2 := setupFlowTestData(t)

		// Create origin headers
		_ = createOriginHeader(t, data2, "Origin-1", "OV1")
		_ = createOriginHeader(t, data2, "Origin-2", "OV2")

		// Auto-create delta copies
		_, err := data2.rpc.HeaderDeltaList(data2.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: data2.deltaExampleID.Bytes(),
			OriginId:  data2.originExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Create new delta-only headers (no origin counterpart)
		newDeltaH1ID := createDeltaHeader(t, data2, "Delta-Only-1", "DV1")
		newDeltaH2ID := createDeltaHeader(t, data2, "Delta-Only-2", "DV2")

		// Get updated list
		listResp, err := data2.rpc.HeaderDeltaList(data2.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: data2.deltaExampleID.Bytes(),
			OriginId:  data2.originExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Should have 4 items total: 2 auto-created + 2 new delta-only
		if len(listResp.Msg.Items) != 4 {
			t.Fatalf("Expected 4 headers, got %d", len(listResp.Msg.Items))
		}

		// Find and verify source types
		foundAutoCreated := 0
		foundDeltaOnly := 0

		for _, item := range listResp.Msg.Items {
			itemID, err := idwrap.NewFromBytes(item.HeaderId)
			if err != nil {
				continue
			}

			if itemID.Compare(newDeltaH1ID) == 0 || itemID.Compare(newDeltaH2ID) == 0 {
				// New delta-only headers should have DELTA source
				if item.Source == nil || *item.Source != deltav1.SourceKind_SOURCE_KIND_DELTA {
					actualSource := deltav1.SourceKind_SOURCE_KIND_UNSPECIFIED
					if item.Source != nil {
						actualSource = *item.Source
					}
					t.Errorf("Delta-only header should have DELTA source, got %v", actualSource)
				}
				foundDeltaOnly++
			} else {
				// Auto-created headers should have ORIGIN source (until modified)
				if item.Source == nil || *item.Source != deltav1.SourceKind_SOURCE_KIND_ORIGIN {
					actualSource := deltav1.SourceKind_SOURCE_KIND_UNSPECIFIED
					if item.Source != nil {
						actualSource = *item.Source
					}
					t.Errorf("Auto-created header should have ORIGIN source, got %v", actualSource)
				}
				foundAutoCreated++
			}
		}

		if foundAutoCreated != 2 {
			t.Errorf("Expected 2 auto-created headers, found %d", foundAutoCreated)
		}
		if foundDeltaOnly != 2 {
			t.Errorf("Expected 2 delta-only headers, found %d", foundDeltaOnly)
		}
	})
}

// TestHeaderDeltaMove tests delta header movement functionality
func TestHeaderDeltaMove(t *testing.T) {
	data := setupFlowTestData(t)

	t.Run("IndependentFromOrigin", func(t *testing.T) {
		// Create headers in both origin and delta
		originH1ID := createOriginHeader(t, data, "H1", "V1")
		originH2ID := createOriginHeader(t, data, "H2", "V2")
		originH3ID := createOriginHeader(t, data, "H3", "V3")
		originH4ID := createOriginHeader(t, data, "H4", "V4")

		// Auto-create delta headers
		_, err := data.rpc.HeaderDeltaList(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: data.deltaExampleID.Bytes(),
			OriginId:  data.originExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Get delta header IDs
		deltaHeaders, err := data.ehs.GetHeaderByExampleID(data.ctx, data.deltaExampleID)
		if err != nil {
			t.Fatal(err)
		}

		parentToDelta := make(map[idwrap.IDWrap]idwrap.IDWrap)
		for _, dh := range deltaHeaders {
			if dh.DeltaParentID != nil {
				parentToDelta[*dh.DeltaParentID] = dh.ID
			}
		}

		deltaH1ID := parentToDelta[originH1ID]
		deltaH2ID := parentToDelta[originH2ID]
		deltaH3ID := parentToDelta[originH3ID]
		deltaH4ID := parentToDelta[originH4ID]

		// Verify initial state: both origin and delta have same order
		originInitialOrder := []idwrap.IDWrap{originH1ID, originH2ID, originH3ID, originH4ID}
		deltaInitialOrder := []idwrap.IDWrap{deltaH1ID, deltaH2ID, deltaH3ID, deltaH4ID}
		
		verifyFlowHeaderOrder(t, data.ctx, data.ehs, data.originExampleID, originInitialOrder, "OriginInitialOrder")
		verifyFlowHeaderOrder(t, data.ctx, data.ehs, data.deltaExampleID, deltaInitialOrder, "DeltaInitialOrder")

		// Move delta H4 after H1: should result in H1, H4, H2, H3 in delta only
		_, err = data.rpc.HeaderDeltaMove(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaMoveRequest{
			ExampleId:      data.deltaExampleID.Bytes(),
			HeaderId:       deltaH4ID.Bytes(),
			TargetHeaderId: deltaH1ID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Verify origin order remains unchanged
		verifyFlowHeaderOrder(t, data.ctx, data.ehs, data.originExampleID, originInitialOrder, "OriginAfterDeltaMove")

		// Verify delta order changed
		deltaNewOrder := []idwrap.IDWrap{deltaH1ID, deltaH4ID, deltaH2ID, deltaH3ID}
		verifyFlowHeaderOrder(t, data.ctx, data.ehs, data.deltaExampleID, deltaNewOrder, "DeltaAfterMove")

		// Move origin header and verify it doesn't affect delta
		_, err = data.rpc.HeaderMove(data.ctx, connect.NewRequest(&requestv1.HeaderMoveRequest{
			ExampleId:      data.originExampleID.Bytes(),
			HeaderId:       originH2ID.Bytes(),
			TargetHeaderId: originH3ID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Verify origin order changed
		originAfterMove := []idwrap.IDWrap{originH1ID, originH3ID, originH2ID, originH4ID}
		verifyFlowHeaderOrder(t, data.ctx, data.ehs, data.originExampleID, originAfterMove, "OriginAfterOriginMove")

		// Verify delta order remained unchanged
		verifyFlowHeaderOrder(t, data.ctx, data.ehs, data.deltaExampleID, deltaNewOrder, "DeltaUnchangedAfterOriginMove")
	})
}

// TestHeaderDeltaUpdate tests delta header update functionality
func TestHeaderDeltaUpdate(t *testing.T) {
	data := setupFlowTestData(t)

	t.Run("OverrideValuesPreserveOrder", func(t *testing.T) {
		// Create origin headers
		_ = createOriginHeader(t, data, "Authorization", "Bearer origin-token")
		_ = createOriginHeader(t, data, "Content-Type", "application/json")
		_ = createOriginHeader(t, data, "X-Custom", "origin-value")

		// Auto-create delta headers
		listResp, err := data.rpc.HeaderDeltaList(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: data.deltaExampleID.Bytes(),
			OriginId:  data.originExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Find delta header ID for Authorization header
		var authDeltaID idwrap.IDWrap
		var ctDeltaID idwrap.IDWrap
		
		for _, item := range listResp.Msg.Items {
			if item.Key == "Authorization" {
				authDeltaID, _ = idwrap.NewFromBytes(item.HeaderId)
			} else if item.Key == "Content-Type" {
				ctDeltaID, _ = idwrap.NewFromBytes(item.HeaderId)
			}
		}

		// Update Authorization delta header value
		_, err = data.rpc.HeaderDeltaUpdate(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaUpdateRequest{
			HeaderId:    authDeltaID.Bytes(),
			Key:         stringPtrFlow("Authorization"),
			Value:       stringPtrFlow("Bearer delta-token-updated"),
			Enabled:     boolPtrFlow(true),
			Description: stringPtrFlow("Updated authorization header"),
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Update Content-Type delta header
		_, err = data.rpc.HeaderDeltaUpdate(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaUpdateRequest{
			HeaderId:    ctDeltaID.Bytes(),
			Key:         stringPtrFlow("Content-Type"),
			Value:       stringPtrFlow("application/xml"),
			Enabled:     boolPtrFlow(false), // Also disable it
			Description: stringPtrFlow("Updated content type"),
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Verify origin headers unchanged
		originHeaders, err := data.ehs.GetHeaderByExampleID(data.ctx, data.originExampleID)
		if err != nil {
			t.Fatal(err)
		}

		for _, oh := range originHeaders {
			switch oh.HeaderKey {
			case "Authorization":
				if oh.Value != "Bearer origin-token" {
					t.Error("Origin Authorization header value was modified")
				}
				if !oh.Enable {
					t.Error("Origin Authorization header was disabled")
				}
			case "Content-Type":
				if oh.Value != "application/json" {
					t.Error("Origin Content-Type header value was modified")
				}
				if !oh.Enable {
					t.Error("Origin Content-Type header was disabled")
				}
			}
		}

		// Verify delta headers have new values
		updatedListResp, err := data.rpc.HeaderDeltaList(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: data.deltaExampleID.Bytes(),
			OriginId:  data.originExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		for _, item := range updatedListResp.Msg.Items {
			switch item.Key {
			case "Authorization":
				if item.Value != "Bearer delta-token-updated" {
					t.Error("Delta Authorization header not updated")
				}
				// Should now be marked as MIXED since it's been modified from parent
				if item.Source == nil || *item.Source != deltav1.SourceKind_SOURCE_KIND_MIXED {
					actualSource := deltav1.SourceKind_SOURCE_KIND_UNSPECIFIED
					if item.Source != nil {
						actualSource = *item.Source
					}
					t.Errorf("Updated delta header should have MIXED source, got %v", actualSource)
				}
			case "Content-Type":
				if item.Value != "application/xml" {
					t.Error("Delta Content-Type header not updated")
				}
				if item.Enabled {
					t.Error("Delta Content-Type header should be disabled")
				}
			case "X-Custom":
				// This one wasn't updated, should still have ORIGIN source
				if item.Source == nil || *item.Source != deltav1.SourceKind_SOURCE_KIND_ORIGIN {
					actualSource := deltav1.SourceKind_SOURCE_KIND_UNSPECIFIED
					if item.Source != nil {
						actualSource = *item.Source
					}
					t.Errorf("Unmodified delta header should still have ORIGIN source, got %v", actualSource)
				}
			}
		}

		// Verify order is preserved despite updates
		expectedOrder := []idwrap.IDWrap{authDeltaID, ctDeltaID} // First two items
		actualOrder := make([]idwrap.IDWrap, 2)
		for i := 0; i < 2; i++ {
			actualOrder[i], _ = idwrap.NewFromBytes(updatedListResp.Msg.Items[i].HeaderId)
		}

		for i, expected := range expectedOrder {
			if actualOrder[i].Compare(expected) != 0 {
				t.Errorf("Order changed after update: position %d expected %s, got %s", 
					i, expected.String()[:8], actualOrder[i].String()[:8])
			}
		}
	})
}

// TestHeaderDeltaDelete tests delta header deletion functionality
func TestHeaderDeltaDelete(t *testing.T) {
	data := setupFlowTestData(t)

	t.Run("CleanupWithoutAffectingOrigin", func(t *testing.T) {
		// Create origin headers
		_ = createOriginHeader(t, data, "Auth", "Bearer token")
		_ = createOriginHeader(t, data, "Type", "application/json")
		_ = createOriginHeader(t, data, "Custom", "custom-value")

		// Auto-create delta headers
		listResp, err := data.rpc.HeaderDeltaList(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: data.deltaExampleID.Bytes(),
			OriginId:  data.originExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Find Auth delta header ID
		var authDeltaID idwrap.IDWrap
		for _, item := range listResp.Msg.Items {
			if item.Key == "Auth" {
				authDeltaID, _ = idwrap.NewFromBytes(item.HeaderId)
				break
			}
		}

		// Create a delta-only header
		deltaOnlyID := createDeltaHeader(t, data, "Delta-Only", "delta-value")

		// Delete the auto-created delta header
		_, err = data.rpc.HeaderDeltaDelete(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaDeleteRequest{
			HeaderId: authDeltaID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Delete the delta-only header
		_, err = data.rpc.HeaderDeltaDelete(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaDeleteRequest{
			HeaderId: deltaOnlyID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Verify origin headers are unchanged
		originHeaders, err := data.ehs.GetHeaderByExampleID(data.ctx, data.originExampleID)
		if err != nil {
			t.Fatal(err)
		}

		if len(originHeaders) != 3 {
			t.Fatalf("Origin should still have 3 headers, got %d", len(originHeaders))
		}

		// Verify all origin headers still exist
		originKeys := make(map[string]bool)
		for _, oh := range originHeaders {
			originKeys[oh.HeaderKey] = true
		}

		expectedKeys := []string{"Auth", "Type", "Custom"}
		for _, key := range expectedKeys {
			if !originKeys[key] {
				t.Errorf("Origin header %s was deleted", key)
			}
		}

		// Verify delta headers were deleted
		deltaHeaders, err := data.ehs.GetHeaderByExampleID(data.ctx, data.deltaExampleID)
		if err != nil {
			t.Fatal(err)
		}

		// Should only have 2 delta headers now (Type and Custom)
		if len(deltaHeaders) != 2 {
			t.Fatalf("Expected 2 remaining delta headers, got %d", len(deltaHeaders))
		}

		deltaKeys := make(map[string]bool)
		for _, dh := range deltaHeaders {
			deltaKeys[dh.HeaderKey] = true
		}

		if deltaKeys["Auth"] {
			t.Error("Auth delta header was not deleted")
		}
		if deltaKeys["Delta-Only"] {
			t.Error("Delta-Only header was not deleted")
		}
		if !deltaKeys["Type"] || !deltaKeys["Custom"] {
			t.Error("Wrong delta headers were deleted")
		}
	})

	t.Run("OrphanedDeltaHandling", func(t *testing.T) {
		data2 := setupFlowTestData(t)

		// Create origin header
		originHID := createOriginHeader(t, data2, "Test-Header", "test-value")

		// Auto-create delta header
		_, err := data2.rpc.HeaderDeltaList(data2.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: data2.deltaExampleID.Bytes(),
			OriginId:  data2.originExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Delete the origin header (this should cascade to delta)
		_, err = data2.rpc.HeaderDelete(data2.ctx, connect.NewRequest(&requestv1.HeaderDeleteRequest{
			HeaderId: originHID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Verify delta header was also deleted (cascade)
		deltaHeaders, err := data2.ehs.GetHeaderByExampleID(data2.ctx, data2.deltaExampleID)
		if err != nil {
			t.Fatal(err)
		}

		if len(deltaHeaders) != 0 {
			t.Error("Delta header should have been cascade deleted when origin was deleted")
		}
	})
}

// TestHeaderDeltaReset tests delta header reset functionality
func TestHeaderDeltaReset(t *testing.T) {
	data := setupFlowTestData(t)

	t.Run("RevertToOriginValues", func(t *testing.T) {
		// Create origin header
		_ = createOriginHeader(t, data, "Reset-Test", "original-value")

		// Auto-create delta header
		listResp, err := data.rpc.HeaderDeltaList(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: data.deltaExampleID.Bytes(),
			OriginId:  data.originExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Find delta header
		var deltaHID idwrap.IDWrap
		for _, item := range listResp.Msg.Items {
			if item.Key == "Reset-Test" {
				deltaHID, _ = idwrap.NewFromBytes(item.HeaderId)
				break
			}
		}

		// Update delta header to different values
		_, err = data.rpc.HeaderDeltaUpdate(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaUpdateRequest{
			HeaderId:    deltaHID.Bytes(),
			Key:         stringPtrFlow("Reset-Test"),
			Value:       stringPtrFlow("modified-value"),
			Enabled:     boolPtrFlow(false),
			Description: stringPtrFlow("modified description"),
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Verify delta header was modified
		modifiedListResp, err := data.rpc.HeaderDeltaList(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: data.deltaExampleID.Bytes(),
			OriginId:  data.originExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		var modifiedItem *requestv1.HeaderDeltaListItem
		for _, item := range modifiedListResp.Msg.Items {
			if item.Key == "Reset-Test" {
				modifiedItem = item
				break
			}
		}

		if modifiedItem.Value != "modified-value" {
			t.Error("Delta header was not updated")
		}
		if modifiedItem.Source == nil || *modifiedItem.Source != deltav1.SourceKind_SOURCE_KIND_MIXED {
			actualSource := deltav1.SourceKind_SOURCE_KIND_UNSPECIFIED
			if modifiedItem.Source != nil {
				actualSource = *modifiedItem.Source
			}
			t.Errorf("Updated delta header should have MIXED source, got %v", actualSource)
		}

		// Reset delta header
		_, err = data.rpc.HeaderDeltaReset(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaResetRequest{
			HeaderId: deltaHID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Verify delta header values reverted to origin
		resetListResp, err := data.rpc.HeaderDeltaList(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: data.deltaExampleID.Bytes(),
			OriginId:  data.originExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		var resetItem *requestv1.HeaderDeltaListItem
		for _, item := range resetListResp.Msg.Items {
			if item.Key == "Reset-Test" {
				resetItem = item
				break
			}
		}

		if resetItem.Value != "original-value" {
			t.Errorf("Reset failed: expected 'original-value', got '%s'", resetItem.Value)
		}
		if !resetItem.Enabled {
			t.Error("Reset failed: header should be enabled like origin")
		}
		if resetItem.Source == nil || *resetItem.Source != deltav1.SourceKind_SOURCE_KIND_ORIGIN {
			actualSource := deltav1.SourceKind_SOURCE_KIND_UNSPECIFIED
			if resetItem.Source != nil {
				actualSource = *resetItem.Source
			}
			t.Errorf("Reset header should have ORIGIN source, got %v", actualSource)
		}
	})

	t.Run("ResetDeltaOnlyHeader", func(t *testing.T) {
		data2 := setupFlowTestData(t)

		// Create delta-only header (no origin counterpart)
		deltaOnlyID := createDeltaHeader(t, data2, "Delta-Only", "delta-value")

		// Try to reset it (should fail or be no-op since no origin exists)
		_, err := data2.rpc.HeaderDeltaReset(data2.ctx, connect.NewRequest(&requestv1.HeaderDeltaResetRequest{
			HeaderId: deltaOnlyID.Bytes(),
		}))
		
		// This might fail or succeed depending on implementation
		// If it succeeds, verify header still exists (value may or may not change)
		if err == nil {
			// Verify header still exists
			header, err := data2.ehs.GetHeaderByID(data2.ctx, deltaOnlyID)
			if err != nil {
				t.Error("Delta-only header was deleted during reset")
			} else {
				// The value might change if the system resets it to match some default,
				// or it might stay the same. Either behavior is acceptable for delta-only headers.
				t.Logf("Delta-only header after reset: value='%s' (original was 'delta-value')", header.Value)
			}
		} else {
			// If it fails, that's also acceptable behavior for delta-only headers
			t.Logf("Reset failed for delta-only header (acceptable): %v", err)
		}
	})
}

// TestHeaderDeltaEdgeCases tests edge cases and complex scenarios
func TestHeaderDeltaEdgeCases(t *testing.T) {
	data := setupFlowTestData(t)

	t.Run("ConcurrentModifications", func(t *testing.T) {
		// Create origin header
		originHID := createOriginHeader(t, data, "Concurrent-Test", "original")

		// Auto-create delta header
		listResp, err := data.rpc.HeaderDeltaList(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: data.deltaExampleID.Bytes(),
			OriginId:  data.originExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		var deltaHID idwrap.IDWrap
		for _, item := range listResp.Msg.Items {
			if item.Key == "Concurrent-Test" {
				deltaHID, _ = idwrap.NewFromBytes(item.HeaderId)
				break
			}
		}

		// Modify origin header
		_, err = data.rpc.HeaderUpdate(data.ctx, connect.NewRequest(&requestv1.HeaderUpdateRequest{
			HeaderId:    originHID.Bytes(),
			Key:         stringPtrFlow("Concurrent-Test"),
			Value:       stringPtrFlow("origin-updated"),
			Enabled:     boolPtrFlow(true),
			Description: stringPtrFlow("origin updated"),
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Modify delta header
		_, err = data.rpc.HeaderDeltaUpdate(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaUpdateRequest{
			HeaderId:    deltaHID.Bytes(),
			Key:         stringPtrFlow("Concurrent-Test"),
			Value:       stringPtrFlow("delta-updated"),
			Enabled:     boolPtrFlow(true),
			Description: stringPtrFlow("delta updated"),
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Verify both have their respective values
		originHeader, err := data.ehs.GetHeaderByID(data.ctx, originHID)
		if err != nil {
			t.Fatal(err)
		}
		if originHeader.Value != "origin-updated" {
			t.Error("Origin header value not preserved during concurrent modification")
		}

		deltaHeader, err := data.ehs.GetHeaderByID(data.ctx, deltaHID)
		if err != nil {
			t.Fatal(err)
		}
		if deltaHeader.Value != "delta-updated" {
			t.Error("Delta header value not preserved during concurrent modification")
		}
	})

	t.Run("CascadeDeleteBehavior", func(t *testing.T) {
		data2 := setupFlowTestData(t)

		// Create origin header
		originHID := createOriginHeader(t, data2, "Cascade-Test", "cascade-value")

		// Create delta headers in multiple delta examples
		_, err := data2.rpc.HeaderDeltaList(data2.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: data2.deltaExampleID.Bytes(),
			OriginId:  data2.originExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		_, err = data2.rpc.HeaderDeltaList(data2.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: data2.secondDeltaExampleID.Bytes(),
			OriginId:  data2.originExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Verify both delta examples have the header
		deltaHeaders1, err := data2.ehs.GetHeaderByExampleID(data2.ctx, data2.deltaExampleID)
		if err != nil {
			t.Fatal(err)
		}
		deltaHeaders2, err := data2.ehs.GetHeaderByExampleID(data2.ctx, data2.secondDeltaExampleID)
		if err != nil {
			t.Fatal(err)
		}

		if len(deltaHeaders1) != 1 || len(deltaHeaders2) != 1 {
			t.Fatal("Expected 1 header in each delta example")
		}

		// Delete origin header
		_, err = data2.rpc.HeaderDelete(data2.ctx, connect.NewRequest(&requestv1.HeaderDeleteRequest{
			HeaderId: originHID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Verify cascade delete to both delta examples
		deltaHeaders1After, err := data2.ehs.GetHeaderByExampleID(data2.ctx, data2.deltaExampleID)
		if err != nil {
			t.Fatal(err)
		}
		deltaHeaders2After, err := data2.ehs.GetHeaderByExampleID(data2.ctx, data2.secondDeltaExampleID)
		if err != nil {
			t.Fatal(err)
		}

		if len(deltaHeaders1After) != 0 {
			t.Error("Delta header in first example was not cascade deleted")
		}
		if len(deltaHeaders2After) != 0 {
			t.Error("Delta header in second example was not cascade deleted")
		}
	})

	t.Run("ComplexOrderingScenario", func(t *testing.T) {
		data3 := setupFlowTestData(t)

		// Test the exact scenario from requirements:
		// Origin: [H1, H2, H3, H4] with values [V1, V2, V3, V4]
		// Delta after moves: [H1, H4, H2, H3]
		// Delta after updates: [H1:V1', H4:V4, H2:V2', H3:V3]
		// Verify: Origin unchanged, Delta has new order and values

		// Create origin headers
		h1ID := createOriginHeader(t, data3, "H1", "V1")
		h2ID := createOriginHeader(t, data3, "H2", "V2")
		h3ID := createOriginHeader(t, data3, "H3", "V3")
		h4ID := createOriginHeader(t, data3, "H4", "V4")

		// Auto-create delta headers
		_, err := data3.rpc.HeaderDeltaList(data3.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: data3.deltaExampleID.Bytes(),
			OriginId:  data3.originExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Get delta header IDs
		deltaHeaders, err := data3.ehs.GetHeaderByExampleID(data3.ctx, data3.deltaExampleID)
		if err != nil {
			t.Fatal(err)
		}

		parentToDelta := make(map[idwrap.IDWrap]idwrap.IDWrap)
		for _, dh := range deltaHeaders {
			if dh.DeltaParentID != nil {
				parentToDelta[*dh.DeltaParentID] = dh.ID
			}
		}

		dH1ID := parentToDelta[h1ID]
		dH2ID := parentToDelta[h2ID]
		dH3ID := parentToDelta[h3ID]
		dH4ID := parentToDelta[h4ID]

		// Move delta H4 after H1: [H1, H4, H2, H3]
		_, err = data3.rpc.HeaderDeltaMove(data3.ctx, connect.NewRequest(&requestv1.HeaderDeltaMoveRequest{
			ExampleId:      data3.deltaExampleID.Bytes(),
			HeaderId:       dH4ID.Bytes(),
			TargetHeaderId: dH1ID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Update delta H1 and H2
		_, err = data3.rpc.HeaderDeltaUpdate(data3.ctx, connect.NewRequest(&requestv1.HeaderDeltaUpdateRequest{
			HeaderId:    dH1ID.Bytes(),
			Key:         stringPtrFlow("H1"),
			Value:       stringPtrFlow("V1'"), // Changed value
			Enabled:     boolPtrFlow(true),
			Description: stringPtrFlow("Updated H1"),
		}))
		if err != nil {
			t.Fatal(err)
		}

		_, err = data3.rpc.HeaderDeltaUpdate(data3.ctx, connect.NewRequest(&requestv1.HeaderDeltaUpdateRequest{
			HeaderId:    dH2ID.Bytes(),
			Key:         stringPtrFlow("H2"),
			Value:       stringPtrFlow("V2'"), // Changed value
			Enabled:     boolPtrFlow(true),
			Description: stringPtrFlow("Updated H2"),
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Verify origin unchanged
		verifyFlowHeaderOrder(t, data3.ctx, data3.ehs, data3.originExampleID, 
			[]idwrap.IDWrap{h1ID, h2ID, h3ID, h4ID}, "OriginUnchangedAfterComplexScenario")

		originHeaders, err := data3.ehs.GetHeaderByExampleID(data3.ctx, data3.originExampleID)
		if err != nil {
			t.Fatal(err)
		}

		for _, oh := range originHeaders {
			switch oh.HeaderKey {
			case "H1":
				if oh.Value != "V1" {
					t.Error("Origin H1 value was changed")
				}
			case "H2":
				if oh.Value != "V2" {
					t.Error("Origin H2 value was changed")
				}
			case "H3":
				if oh.Value != "V3" {
					t.Error("Origin H3 value was changed")
				}
			case "H4":
				if oh.Value != "V4" {
					t.Error("Origin H4 value was changed")
				}
			}
		}

		// Verify delta has new order [H1, H4, H2, H3]
		verifyFlowHeaderOrder(t, data3.ctx, data3.ehs, data3.deltaExampleID, 
			[]idwrap.IDWrap{dH1ID, dH4ID, dH2ID, dH3ID}, "DeltaNewOrderAfterComplexScenario")

		// Verify delta has updated values [H1:V1', H4:V4, H2:V2', H3:V3]
		finalListResp, err := data3.rpc.HeaderDeltaList(data3.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: data3.deltaExampleID.Bytes(),
			OriginId:  data3.originExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		expectedValues := map[string]string{
			"H1": "V1'", // Updated
			"H4": "V4",  // Original
			"H2": "V2'", // Updated
			"H3": "V3",  // Original
		}

		expectedSources := map[string]deltav1.SourceKind{
			"H1": deltav1.SourceKind_SOURCE_KIND_MIXED, // Modified
			"H4": deltav1.SourceKind_SOURCE_KIND_ORIGIN, // Unmodified
			"H2": deltav1.SourceKind_SOURCE_KIND_MIXED,  // Modified
			"H3": deltav1.SourceKind_SOURCE_KIND_ORIGIN, // Unmodified
		}

		for i, item := range finalListResp.Msg.Items {
			expectedValue := expectedValues[item.Key]
			expectedSource := expectedSources[item.Key]

			if item.Value != expectedValue {
				t.Errorf("Position %d (%s): expected value '%s', got '%s'", i, item.Key, expectedValue, item.Value)
			}

			if item.Source == nil || *item.Source != expectedSource {
				actualSource := deltav1.SourceKind_SOURCE_KIND_UNSPECIFIED
				if item.Source != nil {
					actualSource = *item.Source
				}
				t.Errorf("Position %d (%s): expected source %v, got %v", i, item.Key, expectedSource, actualSource)
			}
		}
	})
}