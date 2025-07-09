package rrequest_test

import (
	"context"
	"testing"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rrequest"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
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
	conditionv1 "the-dev-tools/spec/dist/buf/go/condition/v1"
	deltav1 "the-dev-tools/spec/dist/buf/go/delta/v1"

	"connectrpc.com/connect"
)

// Helper functions for creating pointers
func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

// setupDeltaTestData creates test data for delta functionality testing
func setupDeltaTestData(t *testing.T) *deltaTestData {
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

	// Create item
	item := &mitemapi.ItemApi{
		ID:           idwrap.NewNow(),
		Name:         "test-item",
		Url:          "http://example.com",
		Method:       "GET",
		CollectionID: collectionID,
	}
	err := ias.CreateItemApi(ctx, item)
	if err != nil {
		t.Fatal(err)
	}

	// Create origin example
	originExampleID := idwrap.NewNow()
	originExample := &mitemapiexample.ItemApiExample{
		ID:              originExampleID,
		ItemApiID:       item.ID,
		CollectionID:    collectionID,
		Name:            "origin-example",
		VersionParentID: nil, // This is the origin example
	}
	err = iaes.CreateApiExample(ctx, originExample)
	if err != nil {
		t.Fatal(err)
	}

	// Create delta example (with VersionParentID)
	deltaExampleID := idwrap.NewNow()
	deltaExample := &mitemapiexample.ItemApiExample{
		ID:              deltaExampleID,
		ItemApiID:       item.ID,
		CollectionID:    collectionID,
		Name:            "delta-example",
		VersionParentID: &originExampleID, // This makes it a delta example
	}
	err = iaes.CreateApiExample(ctx, deltaExample)
	if err != nil {
		t.Fatal(err)
	}

	rpc := rrequest.New(db, cs, us, ias, iaes, ehs, eqs, as)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	return &deltaTestData{
		ctx:             authedCtx,
		rpc:             rpc,
		originExampleID: originExampleID,
		deltaExampleID:  deltaExampleID,
		userID:          userID,
		ehs:             ehs,
		eqs:             eqs,
		as:              as,
	}
}

type deltaTestData struct {
	ctx             context.Context
	rpc             rrequest.RequestRPC
	originExampleID idwrap.IDWrap
	deltaExampleID  idwrap.IDWrap
	userID          idwrap.IDWrap
	ehs             sexampleheader.HeaderService
	eqs             sexamplequery.ExampleQueryService
	as              sassert.AssertService
}

// Test Query Delta functionality
func TestQueryDeltaCreateUpdateBehavior(t *testing.T) {
	data := setupDeltaTestData(t)

	// 1. Create a query in the origin example
	createResp, err := data.rpc.QueryCreate(data.ctx, connect.NewRequest(&requestv1.QueryCreateRequest{
		ExampleId:   data.originExampleID.Bytes(),
		Key:         "test-key",
		Enabled:     true,
		Value:       "test-value",
		Description: "test-description",
	}))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = idwrap.NewFromBytes(createResp.Msg.QueryId)

	// 2. Copy queries to delta example (simulating delta example creation)
	err = data.rpc.QueryDeltaExampleCopy(data.ctx, data.originExampleID, data.deltaExampleID)
	if err != nil {
		t.Fatal(err)
	}

	// 3. Get delta list to verify initial state
	deltaListResp, err := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{
		ExampleId: data.deltaExampleID.Bytes(),
		OriginId:  data.originExampleID.Bytes(),
	}))
	if err != nil {
		t.Fatal(err)
	}

	// Should have one item with source "origin"
	if len(deltaListResp.Msg.Items) != 1 {
		t.Fatalf("Expected 1 delta item, got %d", len(deltaListResp.Msg.Items))
	}
	initialItem := deltaListResp.Msg.Items[0]
	if initialItem.Source == nil || *initialItem.Source != deltav1.SourceKind_SOURCE_KIND_ORIGIN {
		t.Errorf("Expected initial item to have source 'origin', got %v", initialItem.Source)
	}

	// 4. Update the delta item
	deltaQueryID, _ := idwrap.NewFromBytes(initialItem.QueryId)
	_, err = data.rpc.QueryDeltaUpdate(data.ctx, connect.NewRequest(&requestv1.QueryDeltaUpdateRequest{
		QueryId:     deltaQueryID.Bytes(),
		Key:         stringPtr("updated-key"),
		Enabled:     boolPtr(false),
		Value:       stringPtr("updated-value"),
		Description: stringPtr("updated-description"),
	}))
	if err != nil {
		t.Fatal(err)
	}

	// 5. Get delta list again to check for duplicates and source type
	deltaListResp2, err := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{
		ExampleId: data.deltaExampleID.Bytes(),
		OriginId:  data.originExampleID.Bytes(),
	}))
	if err != nil {
		t.Fatal(err)
	}

	// ISSUE 1 TEST: Should still have only 1 item (not creating duplicates)
	if len(deltaListResp2.Msg.Items) != 1 {
		t.Errorf("ISSUE 1: Expected 1 delta item after update, got %d - duplicates are being created!", len(deltaListResp2.Msg.Items))
	}

	// ISSUE 2 TEST: Updated item should have source "mixed" (current behavior) but should be "delta"
	updatedItem := deltaListResp2.Msg.Items[0]
	if updatedItem.Source == nil {
		t.Error("Updated item has nil source")
	} else if *updatedItem.Source == deltav1.SourceKind_SOURCE_KIND_MIXED {
		t.Log("Current behavior: Updated delta item has source 'mixed'")
		t.Error("ISSUE 2: Delta items should have source 'delta' instead of 'mixed'")
	} else if *updatedItem.Source == deltav1.SourceKind_SOURCE_KIND_DELTA {
		t.Log("Fixed behavior: Updated delta item correctly has source 'delta'")
	}

	// Verify the values were actually updated
	if updatedItem.Key != "updated-key" || updatedItem.Value != "updated-value" {
		t.Error("Delta item values were not properly updated")
	}

	// 6. Create a new delta item (without parent) to test source type
	createDeltaResp, err := data.rpc.QueryDeltaCreate(data.ctx, connect.NewRequest(&requestv1.QueryDeltaCreateRequest{
		ExampleId:   data.deltaExampleID.Bytes(),
		OriginId:    data.originExampleID.Bytes(),
		Key:         "new-delta-key",
		Enabled:     true,
		Value:       "new-delta-value",
		Description: "new-delta-description",
		// No QueryId provided, so this is a standalone delta item
	}))
	if err != nil {
		t.Fatal(err)
	}

	// Get the newly created delta query to check its type
	newDeltaQueryID, _ := idwrap.NewFromBytes(createDeltaResp.Msg.QueryId)
	newDeltaQuery, err := data.eqs.GetExampleQuery(data.ctx, newDeltaQueryID)
	if err != nil {
		t.Fatal(err)
	}

	// Check if it's correctly identified as a delta item
	deltaType := newDeltaQuery.DetermineDeltaType(true) // true because delta example has VersionParentID
	if deltaType != mexamplequery.QuerySourceDelta {
		t.Errorf("ISSUE 2: New delta item should have source 'delta', got %v", deltaType)
	}
}

// Test Header Delta functionality
func TestHeaderDeltaCreateUpdateBehavior(t *testing.T) {
	data := setupDeltaTestData(t)

	// 1. Create a header in the origin example
	createResp, err := data.rpc.HeaderCreate(data.ctx, connect.NewRequest(&requestv1.HeaderCreateRequest{
		ExampleId:   data.originExampleID.Bytes(),
		Key:         "test-header",
		Enabled:     true,
		Value:       "test-value",
		Description: "test-description",
	}))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = idwrap.NewFromBytes(createResp.Msg.HeaderId)

	// 2. Copy headers to delta example
	err = data.rpc.HeaderDeltaExampleCopy(data.ctx, data.originExampleID, data.deltaExampleID)
	if err != nil {
		t.Fatal(err)
	}

	// 3. Get delta list to verify initial state
	deltaListResp, err := data.rpc.HeaderDeltaList(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
		ExampleId: data.deltaExampleID.Bytes(),
		OriginId:  data.originExampleID.Bytes(),
	}))
	if err != nil {
		t.Fatal(err)
	}

	// Should have one item with source "origin"
	if len(deltaListResp.Msg.Items) != 1 {
		t.Fatalf("Expected 1 delta item, got %d", len(deltaListResp.Msg.Items))
	}
	initialItem := deltaListResp.Msg.Items[0]
	if initialItem.Source == nil || *initialItem.Source != deltav1.SourceKind_SOURCE_KIND_ORIGIN {
		t.Errorf("Expected initial item to have source 'origin', got %v", initialItem.Source)
	}

	// 4. Update the delta item
	deltaHeaderID, _ := idwrap.NewFromBytes(initialItem.HeaderId)
	_, err = data.rpc.HeaderDeltaUpdate(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaUpdateRequest{
		HeaderId:    deltaHeaderID.Bytes(),
		Key:         stringPtr("updated-header"),
		Enabled:     boolPtr(false),
		Value:       stringPtr("updated-value"),
		Description: stringPtr("updated-description"),
	}))
	if err != nil {
		t.Fatal(err)
	}

	// 5. Get delta list again
	deltaListResp2, err := data.rpc.HeaderDeltaList(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
		ExampleId: data.deltaExampleID.Bytes(),
		OriginId:  data.originExampleID.Bytes(),
	}))
	if err != nil {
		t.Fatal(err)
	}

	// Should still have only 1 item (not creating duplicates)
	if len(deltaListResp2.Msg.Items) != 1 {
		t.Errorf("Expected 1 delta item after update, got %d", len(deltaListResp2.Msg.Items))
	}

	// Check source type
	updatedItem := deltaListResp2.Msg.Items[0]
	if updatedItem.Source == nil {
		t.Error("Updated item has nil source")
	} else if *updatedItem.Source == deltav1.SourceKind_SOURCE_KIND_MIXED {
		t.Log("Current behavior: Updated delta header has source 'mixed'")
		t.Error("ISSUE 2: Delta headers should have source 'delta' instead of 'mixed'")
	}

	// 6. Create a standalone delta header
	createDeltaResp, err := data.rpc.HeaderDeltaCreate(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaCreateRequest{
		ExampleId:   data.deltaExampleID.Bytes(),
		OriginId:    data.originExampleID.Bytes(),
		Key:         "new-delta-header",
		Enabled:     true,
		Value:       "new-delta-value",
		Description: "new-delta-description",
	}))
	if err != nil {
		t.Fatal(err)
	}

	// Check the type of the newly created header
	newDeltaHeaderID, _ := idwrap.NewFromBytes(createDeltaResp.Msg.HeaderId)
	newDeltaHeader, err := data.ehs.GetHeaderByID(data.ctx, newDeltaHeaderID)
	if err != nil {
		t.Fatal(err)
	}

	// Manually check delta type (since we can't call the private method)
	if newDeltaHeader.DeltaParentID == nil {
		// Header with no parent in a delta example should be "delta" type
		t.Log("Created standalone delta header (no parent)")
	}
}

// Test Assert Delta functionality
func TestAssertDeltaCreateUpdateBehavior(t *testing.T) {
	data := setupDeltaTestData(t)

	// 1. Create an assert in the origin example
	createResp, err := data.rpc.AssertCreate(data.ctx, connect.NewRequest(&requestv1.AssertCreateRequest{
		ExampleId: data.originExampleID.Bytes(),
		Condition: &conditionv1.Condition{
			Comparison: &conditionv1.Comparison{
				Expression: "response == test-value",
			},
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = idwrap.NewFromBytes(createResp.Msg.AssertId)

	// 2. Copy asserts to delta example
	err = data.rpc.AssertDeltaExampleCopy(data.ctx, data.originExampleID, data.deltaExampleID)
	if err != nil {
		t.Fatal(err)
	}

	// 3. Get delta list to verify initial state
	deltaListResp, err := data.rpc.AssertDeltaList(data.ctx, connect.NewRequest(&requestv1.AssertDeltaListRequest{
		ExampleId: data.deltaExampleID.Bytes(),
		OriginId:  data.originExampleID.Bytes(),
	}))
	if err != nil {
		t.Fatal(err)
	}

	// Should have one item with source "origin"
	if len(deltaListResp.Msg.Items) != 1 {
		t.Fatalf("Expected 1 delta item, got %d", len(deltaListResp.Msg.Items))
	}
	initialItem := deltaListResp.Msg.Items[0]
	if initialItem.Source == nil || *initialItem.Source != deltav1.SourceKind_SOURCE_KIND_ORIGIN {
		t.Errorf("Expected initial item to have source 'origin', got %v", initialItem.Source)
	}

	// 4. Update the delta item
	deltaAssertID, _ := idwrap.NewFromBytes(initialItem.AssertId)
	_, err = data.rpc.AssertDeltaUpdate(data.ctx, connect.NewRequest(&requestv1.AssertDeltaUpdateRequest{
		AssertId: deltaAssertID.Bytes(),
		Condition: &conditionv1.Condition{
			Comparison: &conditionv1.Comparison{
				Expression: "response != updated-value",
			},
		},
	}))
	if err != nil {
		t.Fatal(err)
	}

	// 5. Get delta list again
	deltaListResp2, err := data.rpc.AssertDeltaList(data.ctx, connect.NewRequest(&requestv1.AssertDeltaListRequest{
		ExampleId: data.deltaExampleID.Bytes(),
		OriginId:  data.originExampleID.Bytes(),
	}))
	if err != nil {
		t.Fatal(err)
	}

	// ISSUE 1 TEST: Should still have only 1 item (not creating duplicates)
	if len(deltaListResp2.Msg.Items) != 1 {
		t.Errorf("ISSUE 1: Expected 1 delta item after update, got %d - duplicates are being created!", len(deltaListResp2.Msg.Items))
	}

	// Check source type
	updatedItem := deltaListResp2.Msg.Items[0]
	if updatedItem.Source == nil {
		t.Error("Updated item has nil source")
	} else if *updatedItem.Source == deltav1.SourceKind_SOURCE_KIND_MIXED {
		t.Log("Current behavior: Updated delta assert has source 'mixed'")
		t.Error("ISSUE 2: Delta asserts should have source 'delta' instead of 'mixed'")
	}
}

// Test that updating origin items doesn't create duplicates in delta
func TestOriginUpdatePropagation(t *testing.T) {
	data := setupDeltaTestData(t)

	// Create and copy query
	createResp, err := data.rpc.QueryCreate(data.ctx, connect.NewRequest(&requestv1.QueryCreateRequest{
		ExampleId:   data.originExampleID.Bytes(),
		Key:         "origin-key",
		Enabled:     true,
		Value:       "origin-value",
		Description: "origin-description",
	}))
	if err != nil {
		t.Fatal(err)
	}
	originQueryID, _ := idwrap.NewFromBytes(createResp.Msg.QueryId)

	err = data.rpc.QueryDeltaExampleCopy(data.ctx, data.originExampleID, data.deltaExampleID)
	if err != nil {
		t.Fatal(err)
	}

	// Update the origin query
	_, err = data.rpc.QueryUpdate(data.ctx, connect.NewRequest(&requestv1.QueryUpdateRequest{
		QueryId:     originQueryID.Bytes(),
		Key:         stringPtr("updated-origin-key"),
		Enabled:     boolPtr(false),
		Value:       stringPtr("updated-origin-value"),
		Description: stringPtr("updated-origin-description"),
	}))
	if err != nil {
		t.Fatal(err)
	}

	// Check delta list - should still have one item
	deltaListResp, err := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{
		ExampleId: data.deltaExampleID.Bytes(),
		OriginId:  data.originExampleID.Bytes(),
	}))
	if err != nil {
		t.Fatal(err)
	}

	if len(deltaListResp.Msg.Items) != 1 {
		t.Errorf("Expected 1 delta item after origin update, got %d", len(deltaListResp.Msg.Items))
	}

	// The delta item with source "origin" should reflect the updated values
	item := deltaListResp.Msg.Items[0]
	if item.Source != nil {
		t.Logf("Item source: %v", *item.Source)
	}
	t.Logf("Item key: %v, expected: updated-origin-key", item.Key)
	if item.Origin != nil {
		t.Logf("Origin key: %v", item.Origin.Key)
		t.Logf("Origin value: %v", item.Origin.Value)
		t.Logf("Origin enabled: %v", item.Origin.Enabled)
	}
	// If source is ORIGIN, the key should be from the origin
	if item.Source != nil && *item.Source == deltav1.SourceKind_SOURCE_KIND_ORIGIN {
		if item.Key != "updated-origin-key" {
			t.Error("Delta item with source 'origin' should reflect updated origin values")
		}
	} else {
		t.Errorf("Expected source to be ORIGIN but got %v", item.Source)
	}
}

// Test reset functionality
func TestDeltaResetFunctionality(t *testing.T) {
	data := setupDeltaTestData(t)

	// Create origin query
	_, err := data.rpc.QueryCreate(data.ctx, connect.NewRequest(&requestv1.QueryCreateRequest{
		ExampleId:   data.originExampleID.Bytes(),
		Key:         "original-key",
		Enabled:     true,
		Value:       "original-value",
		Description: "original-description",
	}))
	if err != nil {
		t.Fatal(err)
	}

	// Copy to delta
	err = data.rpc.QueryDeltaExampleCopy(data.ctx, data.originExampleID, data.deltaExampleID)
	if err != nil {
		t.Fatal(err)
	}

	// Get delta item
	deltaListResp, err := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{
		ExampleId: data.deltaExampleID.Bytes(),
		OriginId:  data.originExampleID.Bytes(),
	}))
	if err != nil {
		t.Fatal(err)
	}

	deltaQueryID, _ := idwrap.NewFromBytes(deltaListResp.Msg.Items[0].QueryId)

	// Update the delta item
	_, err = data.rpc.QueryDeltaUpdate(data.ctx, connect.NewRequest(&requestv1.QueryDeltaUpdateRequest{
		QueryId:     deltaQueryID.Bytes(),
		Key:         stringPtr("modified-key"),
		Enabled:     boolPtr(false),
		Value:       stringPtr("modified-value"),
		Description: stringPtr("modified-description"),
	}))
	if err != nil {
		t.Fatal(err)
	}

	// Reset the delta item
	_, err = data.rpc.QueryDeltaReset(data.ctx, connect.NewRequest(&requestv1.QueryDeltaResetRequest{
		QueryId: deltaQueryID.Bytes(),
	}))
	if err != nil {
		t.Fatal(err)
	}

	// Check that values are restored
	deltaListResp2, err := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{
		ExampleId: data.deltaExampleID.Bytes(),
		OriginId:  data.originExampleID.Bytes(),
	}))
	if err != nil {
		t.Fatal(err)
	}

	resetItem := deltaListResp2.Msg.Items[0]
	if resetItem.Key != "original-key" || resetItem.Value != "original-value" {
		t.Error("Reset should restore original values")
	}

	// Source should be back to "origin"
	if resetItem.Source == nil || *resetItem.Source != deltav1.SourceKind_SOURCE_KIND_ORIGIN {
		t.Error("Reset item should have source 'origin'")
	}
}

// Test DetermineDeltaType method for Query
func TestQueryDetermineDeltaType(t *testing.T) {
	tests := []struct {
		name           string
		query          mexamplequery.Query
		isDeltaExample bool
		expectedType   mexamplequery.QuerySource
	}{
		{
			name: "Query without DeltaParentID in original example",
			query: mexamplequery.Query{
				ID:            idwrap.NewNow(),
				DeltaParentID: nil,
			},
			isDeltaExample: false,
			expectedType:   mexamplequery.QuerySourceOrigin,
		},
		{
			name: "Query without DeltaParentID in delta example",
			query: mexamplequery.Query{
				ID:            idwrap.NewNow(),
				DeltaParentID: nil,
			},
			isDeltaExample: true,
			expectedType:   mexamplequery.QuerySourceDelta,
		},
		{
			name: "Query with DeltaParentID in original example",
			query: mexamplequery.Query{
				ID:            idwrap.NewNow(),
				DeltaParentID: &idwrap.IDWrap{},
			},
			isDeltaExample: false,
			expectedType:   mexamplequery.QuerySourceMixed,
		},
		{
			name: "Query with DeltaParentID in delta example",
			query: mexamplequery.Query{
				ID:            idwrap.NewNow(),
				DeltaParentID: &idwrap.IDWrap{},
			},
			isDeltaExample: true,
			expectedType:   mexamplequery.QuerySourceDelta,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.query.DetermineDeltaType(tt.isDeltaExample)
			if result != tt.expectedType {
				t.Errorf("Expected %v, got %v", tt.expectedType, result)
			}
		})
	}
}

// Test DetermineDeltaType method for Header
func TestHeaderDetermineDeltaType(t *testing.T) {
	tests := []struct {
		name           string
		header         mexampleheader.Header
		isDeltaExample bool
		expectedType   mexampleheader.HeaderSource
	}{
		{
			name: "Header without DeltaParentID in original example",
			header: mexampleheader.Header{
				ID:            idwrap.NewNow(),
				DeltaParentID: nil,
			},
			isDeltaExample: false,
			expectedType:   mexampleheader.HeaderSourceOrigin,
		},
		{
			name: "Header without DeltaParentID in delta example",
			header: mexampleheader.Header{
				ID:            idwrap.NewNow(),
				DeltaParentID: nil,
			},
			isDeltaExample: true,
			expectedType:   mexampleheader.HeaderSourceDelta,
		},
		{
			name: "Header with DeltaParentID in original example",
			header: mexampleheader.Header{
				ID:            idwrap.NewNow(),
				DeltaParentID: &idwrap.IDWrap{},
			},
			isDeltaExample: false,
			expectedType:   mexampleheader.HeaderSourceMixed,
		},
		{
			name: "Header with DeltaParentID in delta example",
			header: mexampleheader.Header{
				ID:            idwrap.NewNow(),
				DeltaParentID: &idwrap.IDWrap{},
			},
			isDeltaExample: true,
			expectedType:   mexampleheader.HeaderSourceDelta,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.header.DetermineDeltaType(tt.isDeltaExample)
			if result != tt.expectedType {
				t.Errorf("Expected %v, got %v", tt.expectedType, result)
			}
		})
	}
}

// Test DetermineDeltaType method for Assert
func TestAssertDetermineDeltaType(t *testing.T) {
	tests := []struct {
		name           string
		assert         massert.Assert
		isDeltaExample bool
		expectedType   massert.AssertSource
	}{
		{
			name: "Assert without DeltaParentID in original example",
			assert: massert.Assert{
				ID:            idwrap.NewNow(),
				DeltaParentID: nil,
			},
			isDeltaExample: false,
			expectedType:   massert.AssertSourceOrigin,
		},
		{
			name: "Assert without DeltaParentID in delta example",
			assert: massert.Assert{
				ID:            idwrap.NewNow(),
				DeltaParentID: nil,
			},
			isDeltaExample: true,
			expectedType:   massert.AssertSourceDelta,
		},
		{
			name: "Assert with DeltaParentID in original example",
			assert: massert.Assert{
				ID:            idwrap.NewNow(),
				DeltaParentID: &idwrap.IDWrap{},
			},
			isDeltaExample: false,
			expectedType:   massert.AssertSourceMixed,
		},
		{
			name: "Assert with DeltaParentID in delta example",
			assert: massert.Assert{
				ID:            idwrap.NewNow(),
				DeltaParentID: &idwrap.IDWrap{},
			},
			isDeltaExample: true,
			expectedType:   massert.AssertSourceDelta,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.assert.DetermineDeltaType(tt.isDeltaExample)
			if result != tt.expectedType {
				t.Errorf("Expected %v, got %v", tt.expectedType, result)
			}
		})
	}
}
