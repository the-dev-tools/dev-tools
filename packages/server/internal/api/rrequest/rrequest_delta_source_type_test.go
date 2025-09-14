package rrequest_test

import (
    "testing"

    "the-dev-tools/server/pkg/idwrap"
    requestv1 "the-dev-tools/spec/dist/buf/go/collection/item/request/v1"
    conditionv1 "the-dev-tools/spec/dist/buf/go/condition/v1"
    deltav1 "the-dev-tools/spec/dist/buf/go/delta/v1"

    "connectrpc.com/connect"
)

// TestDeltaSourceTypes - Comprehensive test suite for delta source type functionality
func TestDeltaSourceTypes(t *testing.T) {
	t.Run("BasicSourceTypeDetermination", func(t *testing.T) {
		t.Run("ORIGIN_Detection", testOriginDetection)
		t.Run("MIXED_Detection", testMixedDetection)
		t.Run("DELTA_Detection", testDeltaDetection)
	})

	t.Run("StateTransitions", func(t *testing.T) {
		t.Run("ORIGIN_to_MIXED", testOriginToMixed)
		t.Run("MIXED_to_ORIGIN_Reset", testMixedToOriginReset)
		t.Run("DELTA_remains_DELTA", testDeltaRemainsDelta)
	})

	t.Run("OriginUpdatePropagation", func(t *testing.T) {
		t.Run("Origin_Update_Creates_Delta_Override", testOriginUpdateCreatesOverride)
		t.Run("Origin_Update_Updates_ORIGIN_Delta", testOriginUpdateUpdatesOriginDelta)
		t.Run("Origin_Update_Doesnt_Affect_MIXED_Delta", testOriginUpdateDoesntAffectMixed)
	})

	t.Run("EdgeCases", func(t *testing.T) {
		t.Run("Null_vs_Empty_String", testNullVsEmptyString)
		t.Run("Complex_Field_Comparison", testComplexFieldComparison)
		t.Run("Multiple_Delta_Examples", testMultipleDeltaExamples)
	})

	t.Run("ResetFunctions", func(t *testing.T) {
		t.Run("Reset_Query_with_Parent", testResetQueryWithParent)
		t.Run("Reset_Header_with_Parent", testResetHeaderWithParent)
		t.Run("Reset_Assert_with_Parent", testResetAssertWithParent)
		t.Run("Reset_Item_without_Parent", testResetItemWithoutParent)
	})

	t.Run("ListOperations", func(t *testing.T) {
		t.Run("QueryDeltaList_Mixed_Results", testQueryDeltaListMixedResults)
		t.Run("Empty_Delta_List", testEmptyDeltaList)
	})
}

// Test 1.1: ORIGIN Detection
func testOriginDetection(t *testing.T) {
	data := setupDeltaTestData(t)

    // Create origin query
    _, err := data.rpc.QueryCreate(data.ctx, connect.NewRequest(&requestv1.QueryCreateRequest{
        ExampleId:   data.originExampleID.Bytes(),
        Key:         "api-key",
        Enabled:     true,
        Value:       "12345",
        Description: "test description",
    }))
	if err != nil {
		t.Fatal(err)
	}

	// Copy to delta example
	err = data.rpc.QueryDeltaExampleCopy(data.ctx, data.originExampleID, data.deltaExampleID)
	if err != nil {
		t.Fatal(err)
	}

	// Get delta list
	deltaListResp, err := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{
		ExampleId: data.deltaExampleID.Bytes(),
		OriginId:  data.originExampleID.Bytes(),
	}))
	if err != nil {
		t.Fatal(err)
	}

	if len(deltaListResp.Msg.Items) != 1 {
		t.Fatalf("Expected 1 delta item, got %d", len(deltaListResp.Msg.Items))
	}

	item := deltaListResp.Msg.Items[0]
	if item.Source == nil || *item.Source != deltav1.SourceKind_SOURCE_KIND_ORIGIN {
		t.Errorf("Expected ORIGIN source type, got %v", item.Source)
	}

	// Verify values match
	if item.Key != "api-key" || item.Value != "12345" || !item.Enabled {
		t.Error("ORIGIN item values should match parent")
	}

    // no-op
}

// Test 1.2: MIXED Detection
func testMixedDetection(t *testing.T) {
    data := setupDeltaTestData(t)

    // Create origin query
    _, err := data.rpc.QueryCreate(data.ctx, connect.NewRequest(&requestv1.QueryCreateRequest{
        ExampleId:   data.originExampleID.Bytes(),
        Key:         "api-key",
        Enabled:     true,
        Value:       "12345",
        Description: "original description",
    }))
	if err != nil {
		t.Fatal(err)
	}

    // Copy to delta and update via overlay to create MIXED
    _ = data.rpc.QueryDeltaExampleCopy(data.ctx, data.originExampleID, data.deltaExampleID)
    list, err := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{ ExampleId: data.deltaExampleID.Bytes(), OriginId: data.originExampleID.Bytes() }))
    if err != nil { t.Fatal(err) }
    if len(list.Msg.Items) == 0 { t.Fatal("expected at least one item in overlay list") }
    overlayID := list.Msg.Items[0].QueryId
    _, err = data.rpc.QueryDeltaUpdate(data.ctx, connect.NewRequest(&requestv1.QueryDeltaUpdateRequest{ QueryId: overlayID, Value: stringPtr("67890") }))
    if err != nil { t.Fatal(err) }

	// Get delta list
	deltaListResp, err := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{
		ExampleId: data.deltaExampleID.Bytes(),
		OriginId:  data.originExampleID.Bytes(),
	}))
	if err != nil {
		t.Fatal(err)
	}

    if len(deltaListResp.Msg.Items) != 1 { t.Fatalf("Expected 1 delta item, got %d", len(deltaListResp.Msg.Items)) }

	item := deltaListResp.Msg.Items[0]
    if item.Source == nil || *item.Source != deltav1.SourceKind_SOURCE_KIND_MIXED {
        t.Errorf("Expected MIXED source type, got %v", item.Source)
    }

	// Verify values are from delta (modified)
    if item.Value != "67890" {
        t.Error("MIXED item should show delta values")
    }
}

// Test 1.3: DELTA Detection
func testDeltaDetection(t *testing.T) {
	data := setupDeltaTestData(t)

    // Create standalone delta query (no parent)
    _, err := data.rpc.QueryDeltaCreate(data.ctx, connect.NewRequest(&requestv1.QueryDeltaCreateRequest{
        ExampleId:   data.deltaExampleID.Bytes(),
        OriginId:    data.originExampleID.Bytes(),
        Key:         "new-key",
        Enabled:     true,
        Value:       "new-value",
        Description: "new description",
    }))
	if err != nil {
		t.Fatal(err)
	}

	// Get delta list
	deltaListResp, err := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{
		ExampleId: data.deltaExampleID.Bytes(),
		OriginId:  data.originExampleID.Bytes(),
	}))
	if err != nil {
		t.Fatal(err)
	}

	if len(deltaListResp.Msg.Items) != 1 {
		t.Fatalf("Expected 1 delta item, got %d", len(deltaListResp.Msg.Items))
	}

	item := deltaListResp.Msg.Items[0]
	if item.Source == nil || *item.Source != deltav1.SourceKind_SOURCE_KIND_DELTA {
		t.Errorf("Expected DELTA source type, got %v", item.Source)
	}

	// Verify values are standalone
	if item.Key != "new-key" || item.Value != "new-value" {
		t.Error("DELTA item should show its own values")
	}

    // no-op
}

// Test 2.1: ORIGIN → MIXED Transition
func testOriginToMixed(t *testing.T) {
	data := setupDeltaTestData(t)

    // Create origin query
    _, err := data.rpc.QueryCreate(data.ctx, connect.NewRequest(&requestv1.QueryCreateRequest{
        ExampleId:   data.originExampleID.Bytes(),
        Key:         "api-key",
        Enabled:     true,
        Value:       "12345",
        Description: "original",
    }))
	if err != nil {
		t.Fatal(err)
	}

	// Copy to delta (creates ORIGIN state)
	err = data.rpc.QueryDeltaExampleCopy(data.ctx, data.originExampleID, data.deltaExampleID)
	if err != nil {
		t.Fatal(err)
	}

	// Verify initial ORIGIN state
	deltaListResp, err := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{
		ExampleId: data.deltaExampleID.Bytes(),
		OriginId:  data.originExampleID.Bytes(),
	}))
	if err != nil {
		t.Fatal(err)
	}

	initialItem := deltaListResp.Msg.Items[0]
	if *initialItem.Source != deltav1.SourceKind_SOURCE_KIND_ORIGIN {
		t.Error("Expected initial state to be ORIGIN")
	}

	// Update delta query
	deltaQueryID, _ := idwrap.NewFromBytes(initialItem.QueryId)
	_, err = data.rpc.QueryDeltaUpdate(data.ctx, connect.NewRequest(&requestv1.QueryDeltaUpdateRequest{
		QueryId: deltaQueryID.Bytes(),
		Value:   stringPtr("67890"), // Change value
	}))
	if err != nil {
		t.Fatal(err)
	}

	// Verify transition to MIXED
	deltaListResp2, err := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{
		ExampleId: data.deltaExampleID.Bytes(),
		OriginId:  data.originExampleID.Bytes(),
	}))
	if err != nil {
		t.Fatal(err)
	}

	updatedItem := deltaListResp2.Msg.Items[0]
	if *updatedItem.Source != deltav1.SourceKind_SOURCE_KIND_MIXED {
		t.Errorf("Expected transition to MIXED, got %v", *updatedItem.Source)
	}

	if updatedItem.Value != "67890" {
		t.Error("Expected updated value to be shown")
	}
}

// Test 2.2: MIXED → ORIGIN Transition (Reset)
func testMixedToOriginReset(t *testing.T) {
	data := setupDeltaTestData(t)

	// Create origin query
	_, err := data.rpc.QueryCreate(data.ctx, connect.NewRequest(&requestv1.QueryCreateRequest{
		ExampleId:   data.originExampleID.Bytes(),
		Key:         "api-key",
		Enabled:     true,
		Value:       "12345",
		Description: "original",
	}))
	if err != nil {
		t.Fatal(err)
	}

	// Copy to delta and modify (creates MIXED state)
	err = data.rpc.QueryDeltaExampleCopy(data.ctx, data.originExampleID, data.deltaExampleID)
	if err != nil {
		t.Fatal(err)
	}

	// Get delta item and modify it
	deltaListResp, err := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{
		ExampleId: data.deltaExampleID.Bytes(),
		OriginId:  data.originExampleID.Bytes(),
	}))
	if err != nil {
		t.Fatal(err)
	}

	deltaQueryID, _ := idwrap.NewFromBytes(deltaListResp.Msg.Items[0].QueryId)

	// Modify to create MIXED state
	_, err = data.rpc.QueryDeltaUpdate(data.ctx, connect.NewRequest(&requestv1.QueryDeltaUpdateRequest{
		QueryId: deltaQueryID.Bytes(),
		Key:     stringPtr("custom"),
		Value:   stringPtr("custom"),
	}))
	if err != nil {
		t.Fatal(err)
	}

	// Reset delta query
	_, err = data.rpc.QueryDeltaReset(data.ctx, connect.NewRequest(&requestv1.QueryDeltaResetRequest{
		QueryId: deltaQueryID.Bytes(),
	}))
	if err != nil {
		t.Fatal(err)
	}

	// Verify transition back to ORIGIN
	deltaListResp2, err := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{
		ExampleId: data.deltaExampleID.Bytes(),
		OriginId:  data.originExampleID.Bytes(),
	}))
	if err != nil {
		t.Fatal(err)
	}

	resetItem := deltaListResp2.Msg.Items[0]
	if *resetItem.Source != deltav1.SourceKind_SOURCE_KIND_ORIGIN {
		t.Errorf("Expected transition to ORIGIN after reset, got %v", *resetItem.Source)
	}

	// Verify values are restored
	if resetItem.Key != "api-key" || resetItem.Value != "12345" {
		t.Error("Reset should restore original values")
	}
}

// Test 2.3: DELTA Remains DELTA
func testDeltaRemainsDelta(t *testing.T) {
	data := setupDeltaTestData(t)

	// Create standalone delta query
	createResp, err := data.rpc.QueryDeltaCreate(data.ctx, connect.NewRequest(&requestv1.QueryDeltaCreateRequest{
		ExampleId:   data.deltaExampleID.Bytes(),
		OriginId:    data.originExampleID.Bytes(),
		Key:         "test",
		Enabled:     true,
		Value:       "test",
		Description: "test",
	}))
	if err != nil {
		t.Fatal(err)
	}

	deltaQueryID, _ := idwrap.NewFromBytes(createResp.Msg.QueryId)

	// Update the standalone delta
	_, err = data.rpc.QueryDeltaUpdate(data.ctx, connect.NewRequest(&requestv1.QueryDeltaUpdateRequest{
		QueryId: deltaQueryID.Bytes(),
		Key:     stringPtr("updated"),
		Value:   stringPtr("updated"),
	}))
	if err != nil {
		t.Fatal(err)
	}

	// Verify still DELTA
	deltaListResp, err := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{
		ExampleId: data.deltaExampleID.Bytes(),
		OriginId:  data.originExampleID.Bytes(),
	}))
	if err != nil {
		t.Fatal(err)
	}

	item := deltaListResp.Msg.Items[0]
	if *item.Source != deltav1.SourceKind_SOURCE_KIND_DELTA {
		t.Errorf("Expected DELTA to remain DELTA, got %v", *item.Source)
	}
}

// Test 3.1: Origin Update Creates Delta Override
func testOriginUpdateCreatesOverride(t *testing.T) {
    data := setupDeltaTestData(t)

    // Create origin query
    createResp, err := data.rpc.QueryCreate(data.ctx, connect.NewRequest(&requestv1.QueryCreateRequest{
        ExampleId:   data.originExampleID.Bytes(),
        Key:         "api-key",
        Enabled:     true,
        Value:       "12345",
        Description: "original",
    }))
    if err != nil {
        t.Fatal(err)
    }

    originQueryID, _ := idwrap.NewFromBytes(createResp.Msg.QueryId)

    // Update origin query
    _, err = data.rpc.QueryUpdate(data.ctx, connect.NewRequest(&requestv1.QueryUpdateRequest{
        QueryId: originQueryID.Bytes(),
        Value:   stringPtr("67890"),
    }))
    if err != nil {
        t.Fatal(err)
    }

    // Check if delta was created/updated in delta example
    deltaListResp, err := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{
        ExampleId: data.deltaExampleID.Bytes(),
        OriginId:  data.originExampleID.Bytes(),
    }))
    if err != nil {
        t.Fatal(err)
    }

    // Should auto-create delta items
    if len(deltaListResp.Msg.Items) != 1 {
        t.Fatalf("Expected 1 delta item to be auto-created, got %d", len(deltaListResp.Msg.Items))
    }

    item := deltaListResp.Msg.Items[0]
    if item.Value != "67890" {
        t.Error("Delta should have updated origin values")
    }

    // Since this is auto-created and matches origin, it should be ORIGIN
    if *item.Source != deltav1.SourceKind_SOURCE_KIND_ORIGIN {
        t.Errorf("Auto-created delta should be ORIGIN, got %v", *item.Source)
    }
}

// Test 3.2: Origin Update Updates Existing ORIGIN Delta
func testOriginUpdateUpdatesOriginDelta(t *testing.T) {
	data := setupDeltaTestData(t)

	// Create origin query and copy to delta
	createResp, err := data.rpc.QueryCreate(data.ctx, connect.NewRequest(&requestv1.QueryCreateRequest{
		ExampleId:   data.originExampleID.Bytes(),
		Key:         "api-key",
		Enabled:     true,
		Value:       "12345",
		Description: "original",
	}))
	if err != nil {
		t.Fatal(err)
	}

    // origin created
    originQueryID, _ := idwrap.NewFromBytes(createResp.Msg.QueryId)

	// Copy to delta (creates ORIGIN state)
	err = data.rpc.QueryDeltaExampleCopy(data.ctx, data.originExampleID, data.deltaExampleID)
	if err != nil {
		t.Fatal(err)
	}

    // Update origin
    _, err = data.rpc.QueryUpdate(data.ctx, connect.NewRequest(&requestv1.QueryUpdateRequest{
        QueryId: originQueryID.Bytes(),
        Value:   stringPtr("67890"),
    }))
	if err != nil {
		t.Fatal(err)
	}

	// Check delta list
	deltaListResp, err := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{
		ExampleId: data.deltaExampleID.Bytes(),
		OriginId:  data.originExampleID.Bytes(),
	}))
	if err != nil {
		t.Fatal(err)
	}

	item := deltaListResp.Msg.Items[0]

	// Should still be ORIGIN since it was automatically updated to match the new origin
	if *item.Source != deltav1.SourceKind_SOURCE_KIND_ORIGIN {
		t.Errorf("Updated delta should remain ORIGIN, got %v", *item.Source)
	}

	if item.Value != "67890" {
		t.Error("Delta should have new origin values")
	}
}

// Test 3.3: Origin Update Doesn't Affect MIXED Delta
func testOriginUpdateDoesntAffectMixed(t *testing.T) {
    data := setupDeltaTestData(t)

	// Create origin query
	createResp, err := data.rpc.QueryCreate(data.ctx, connect.NewRequest(&requestv1.QueryCreateRequest{
		ExampleId:   data.originExampleID.Bytes(),
		Key:         "api-key",
		Enabled:     true,
		Value:       "12345",
		Description: "original",
	}))
	if err != nil {
		t.Fatal(err)
	}

    originQueryID, _ := idwrap.NewFromBytes(createResp.Msg.QueryId)

    // Copy to delta and create MIXED by overriding via overlay
    _ = data.rpc.QueryDeltaExampleCopy(data.ctx, data.originExampleID, data.deltaExampleID)
    list, err := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{ ExampleId: data.deltaExampleID.Bytes(), OriginId: data.originExampleID.Bytes() }))
    if err != nil { t.Fatal(err) }
    if len(list.Msg.Items) == 0 { t.Fatal("expected at least one item in overlay list") }
    overlayID := list.Msg.Items[0].QueryId
    _, err = data.rpc.QueryDeltaUpdate(data.ctx, connect.NewRequest(&requestv1.QueryDeltaUpdateRequest{ QueryId: overlayID, Key: stringPtr("api-key"), Value: stringPtr("custom"), Description: stringPtr("original"), Enabled: boolPtr(true) }))
    if err != nil { t.Fatal(err) }

	// Update origin
	_, err = data.rpc.QueryUpdate(data.ctx, connect.NewRequest(&requestv1.QueryUpdateRequest{
		QueryId: originQueryID.Bytes(),
		Value:   stringPtr("67890"),
	}))
	if err != nil {
		t.Fatal(err)
	}

	// Check delta list
	deltaListResp, err := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{
		ExampleId: data.deltaExampleID.Bytes(),
		OriginId:  data.originExampleID.Bytes(),
	}))
	if err != nil {
		t.Fatal(err)
	}

    item := deltaListResp.Msg.Items[0]

	// Should remain MIXED and keep custom values
	if *item.Source != deltav1.SourceKind_SOURCE_KIND_MIXED {
		t.Errorf("Customized delta should remain MIXED, got %v", *item.Source)
	}

    if item.Value != "custom" {
        t.Error("MIXED delta should keep customization")
    }
}

// Test 4.1: Null vs Empty String Handling
func testNullVsEmptyString(t *testing.T) {
	data := setupDeltaTestData(t)

    // Create origin with empty description
    _, err := data.rpc.QueryCreate(data.ctx, connect.NewRequest(&requestv1.QueryCreateRequest{
        ExampleId:   data.originExampleID.Bytes(),
        Key:         "test",
        Enabled:     true,
        Value:       "value",
        Description: "", // Empty string
    }))
	if err != nil {
		t.Fatal(err)
	}

    // Copy to delta and set non-empty description via overlay update
    _ = data.rpc.QueryDeltaExampleCopy(data.ctx, data.originExampleID, data.deltaExampleID)
    list, err := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{ ExampleId: data.deltaExampleID.Bytes(), OriginId: data.originExampleID.Bytes() }))
    if err != nil { t.Fatal(err) }
    if len(list.Msg.Items) == 0 { t.Fatal("expected one item in overlay list") }
    id := list.Msg.Items[0].QueryId
    _, err = data.rpc.QueryDeltaUpdate(data.ctx, connect.NewRequest(&requestv1.QueryDeltaUpdateRequest{ QueryId: id, Description: stringPtr("not empty") }))
    if err != nil { t.Fatal(err) }

	// Check delta list
	deltaListResp, err := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{
		ExampleId: data.deltaExampleID.Bytes(),
		OriginId:  data.originExampleID.Bytes(),
	}))
	if err != nil {
		t.Fatal(err)
	}

    item := deltaListResp.Msg.Items[0]
	if *item.Source != deltav1.SourceKind_SOURCE_KIND_MIXED {
		t.Errorf("Different descriptions should create MIXED, got %v", *item.Source)
	}
}

// Test 4.2: Complex Field Comparison
func testComplexFieldComparison(t *testing.T) {
	data := setupDeltaTestData(t)

    // Create origin query
    _, err := data.rpc.QueryCreate(data.ctx, connect.NewRequest(&requestv1.QueryCreateRequest{
        ExampleId:   data.originExampleID.Bytes(),
        Key:         "key",
        Enabled:     true,
        Value:       "value",
        Description: "desc",
    }))
	if err != nil {
		t.Fatal(err)
	}

    // Copy to delta and flip enabled via overlay update
    _ = data.rpc.QueryDeltaExampleCopy(data.ctx, data.originExampleID, data.deltaExampleID)
    list, err := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{ ExampleId: data.deltaExampleID.Bytes(), OriginId: data.originExampleID.Bytes() }))
    if err != nil { t.Fatal(err) }
    if len(list.Msg.Items) == 0 { t.Fatal("expected one item in overlay list") }
    id := list.Msg.Items[0].QueryId
    _, err = data.rpc.QueryDeltaUpdate(data.ctx, connect.NewRequest(&requestv1.QueryDeltaUpdateRequest{ QueryId: id, Enabled: boolPtr(false) }))
    if err != nil { t.Fatal(err) }

	// Check delta list
	deltaListResp, err := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{
		ExampleId: data.deltaExampleID.Bytes(),
		OriginId:  data.originExampleID.Bytes(),
	}))
	if err != nil {
		t.Fatal(err)
	}

	item := deltaListResp.Msg.Items[0]
	if *item.Source != deltav1.SourceKind_SOURCE_KIND_MIXED {
		t.Errorf("Single field difference should create MIXED, got %v", *item.Source)
	}
}

// Test 4.3: Multiple Delta Examples
func testMultipleDeltaExamples(t *testing.T) {
	// This test would need a more complex setup with multiple delta examples
	// For now, we'll create a placeholder
	t.Skip("Multiple delta examples test requires extended setup - placeholder")
}

// Test 5.1: Reset Query with Parent
func testResetQueryWithParent(t *testing.T) {
	data := setupDeltaTestData(t)

    // Create origin with specific values
    _, err := data.rpc.QueryCreate(data.ctx, connect.NewRequest(&requestv1.QueryCreateRequest{
        ExampleId:   data.originExampleID.Bytes(),
        Key:         "orig",
        Enabled:     true,
        Value:       "orig",
        Description: "orig",
    }))
	if err != nil {
		t.Fatal(err)
	}

    // Copy then modify via overlay and reset
    _ = data.rpc.QueryDeltaExampleCopy(data.ctx, data.originExampleID, data.deltaExampleID)
    list, err := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{ ExampleId: data.deltaExampleID.Bytes(), OriginId: data.originExampleID.Bytes() }))
    if err != nil { t.Fatal(err) }
    if len(list.Msg.Items) == 0 { t.Fatal("expected one item in overlay list") }
    qid := list.Msg.Items[0].QueryId
    _, err = data.rpc.QueryDeltaUpdate(data.ctx, connect.NewRequest(&requestv1.QueryDeltaUpdateRequest{ QueryId: qid, Key: stringPtr("mod"), Value: stringPtr("mod"), Description: stringPtr("mod"), Enabled: boolPtr(false) }))
    if err != nil { t.Fatal(err) }
    // Reset delta via overlay id
    _, err = data.rpc.QueryDeltaReset(data.ctx, connect.NewRequest(&requestv1.QueryDeltaResetRequest{ QueryId: qid }))
	if err != nil {
		t.Fatal(err)
	}

	// Check result
	deltaListResp, err := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{
		ExampleId: data.deltaExampleID.Bytes(),
		OriginId:  data.originExampleID.Bytes(),
	}))
	if err != nil {
		t.Fatal(err)
	}

	item := deltaListResp.Msg.Items[0]

	// Should be ORIGIN after reset
	if *item.Source != deltav1.SourceKind_SOURCE_KIND_ORIGIN {
		t.Errorf("Reset should create ORIGIN, got %v", *item.Source)
	}

	// Values should match origin
	if item.Key != "orig" || item.Value != "orig" || !item.Enabled {
		t.Error("Reset should restore all original values")
	}
}

// Test 5.2: Reset Header with Parent
func testResetHeaderWithParent(t *testing.T) {
    data := setupDeltaTestData(t)

    // Create origin header
    _, err := data.rpc.HeaderCreate(data.ctx, connect.NewRequest(&requestv1.HeaderCreateRequest{
        ExampleId:   data.originExampleID.Bytes(),
        Key:         "X-API-Key",
        Enabled:     true,
        Value:       "12345",
        Description: "API Key",
    }))
    if err != nil {
        t.Fatal(err)
    }

    // Seed overlay and fetch overlay ID
    _ = data.rpc.HeaderDeltaExampleCopy(data.ctx, data.originExampleID, data.deltaExampleID)
    listResp, err := data.rpc.HeaderDeltaList(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
        ExampleId: data.deltaExampleID.Bytes(),
        OriginId:  data.originExampleID.Bytes(),
    }))
    if err != nil {
        t.Fatal(err)
    }
    if len(listResp.Msg.Items) == 0 {
        t.Fatal("expected at least one header in overlay list")
    }
    overlayID := listResp.Msg.Items[0].HeaderId

    // Modify overlay to create MIXED
    _, err = data.rpc.HeaderDeltaUpdate(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaUpdateRequest{
        HeaderId:    overlayID,
        Key:         stringPtr("X-Custom"),
        Enabled:     boolPtr(false),
        Value:       stringPtr("custom"),
        Description: stringPtr("Custom"),
    }))
    if err != nil {
        t.Fatal(err)
    }

    // Reset via overlay ID
    _, err = data.rpc.HeaderDeltaReset(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaResetRequest{
        HeaderId: overlayID,
    }))
    if err != nil {
        t.Fatal(err)
    }

    // Verify result
    deltaListResp, err := data.rpc.HeaderDeltaList(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
        ExampleId: data.deltaExampleID.Bytes(),
        OriginId:  data.originExampleID.Bytes(),
    }))
    if err != nil {
        t.Fatal(err)
    }
    if len(deltaListResp.Msg.Items) != 1 {
        t.Fatalf("Expected 1 header item, got %d", len(deltaListResp.Msg.Items))
    }
    item := deltaListResp.Msg.Items[0]
    if *item.Source != deltav1.SourceKind_SOURCE_KIND_ORIGIN {
        t.Errorf("Reset header should be ORIGIN, got %v", *item.Source)
    }
    if item.Key != "X-API-Key" || item.Value != "12345" || !item.Enabled {
        t.Error("Reset should restore all original header values")
    }
}

// Test 5.3: Reset Assert with Parent
func testResetAssertWithParent(t *testing.T) {
    data := setupDeltaTestData(t)

    // Create origin assert
    _, err := data.rpc.AssertCreate(data.ctx, connect.NewRequest(&requestv1.AssertCreateRequest{
        ExampleId: data.originExampleID.Bytes(),
        Condition: &conditionv1.Condition{Comparison: &conditionv1.Comparison{Expression: "status == 200"}},
    }))
    if err != nil {
        t.Fatal(err)
    }

    // Seed overlay and fetch overlay ID
    _ = data.rpc.AssertDeltaExampleCopy(data.ctx, data.originExampleID, data.deltaExampleID)
    listResp, err := data.rpc.AssertDeltaList(data.ctx, connect.NewRequest(&requestv1.AssertDeltaListRequest{
        ExampleId: data.deltaExampleID.Bytes(),
        OriginId:  data.originExampleID.Bytes(),
    }))
    if err != nil {
        t.Fatal(err)
    }
    if len(listResp.Msg.Items) == 0 {
        t.Fatal("expected at least one assert in overlay list")
    }
    overlayID := listResp.Msg.Items[0].AssertId

    // Modify overlay to MIXED
    _, err = data.rpc.AssertDeltaUpdate(data.ctx, connect.NewRequest(&requestv1.AssertDeltaUpdateRequest{
        AssertId: overlayID,
        Condition: &conditionv1.Condition{Comparison: &conditionv1.Comparison{Expression: "status != 200"}},
    }))
    if err != nil {
        t.Fatal(err)
    }

    // Reset via overlay ID
    _, err = data.rpc.AssertDeltaReset(data.ctx, connect.NewRequest(&requestv1.AssertDeltaResetRequest{
        AssertId: overlayID,
    }))
    if err != nil {
        t.Fatal(err)
    }

    // Verify result
    deltaListResp, err := data.rpc.AssertDeltaList(data.ctx, connect.NewRequest(&requestv1.AssertDeltaListRequest{
        ExampleId: data.deltaExampleID.Bytes(),
        OriginId:  data.originExampleID.Bytes(),
    }))
    if err != nil {
        t.Fatal(err)
    }
    if len(deltaListResp.Msg.Items) != 1 {
        t.Fatalf("Expected 1 assert item, got %d", len(deltaListResp.Msg.Items))
    }
    item := deltaListResp.Msg.Items[0]
    if *item.Source != deltav1.SourceKind_SOURCE_KIND_ORIGIN {
        t.Errorf("Reset assert should be ORIGIN, got %v", *item.Source)
    }
    if item.Condition == nil {
        t.Error("Reset should restore original condition")
    }
}

// Test 5.4: Reset Item without Parent
func testResetItemWithoutParent(t *testing.T) {
	data := setupDeltaTestData(t)

	// Create standalone delta query
	createResp, err := data.rpc.QueryDeltaCreate(data.ctx, connect.NewRequest(&requestv1.QueryDeltaCreateRequest{
		ExampleId:   data.deltaExampleID.Bytes(),
		OriginId:    data.originExampleID.Bytes(),
		Key:         "test",
		Enabled:     true,
		Value:       "test",
		Description: "test",
	}))
	if err != nil {
		t.Fatal(err)
	}

	deltaQueryID, _ := idwrap.NewFromBytes(createResp.Msg.QueryId)

	// Reset standalone delta
	_, err = data.rpc.QueryDeltaReset(data.ctx, connect.NewRequest(&requestv1.QueryDeltaResetRequest{
		QueryId: deltaQueryID.Bytes(),
	}))
	if err != nil {
		t.Fatal(err)
	}

	// Check result
	deltaListResp, err := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{
		ExampleId: data.deltaExampleID.Bytes(),
		OriginId:  data.originExampleID.Bytes(),
	}))
	if err != nil {
		t.Fatal(err)
	}

	// The item might be deleted or cleared by reset - check implementation
	// For this test, we expect it to remain DELTA
	if len(deltaListResp.Msg.Items) > 0 {
		item := deltaListResp.Msg.Items[0]
		if *item.Source != deltav1.SourceKind_SOURCE_KIND_DELTA {
			t.Errorf("Reset DELTA should remain DELTA, got %v", *item.Source)
		}
	}
}

// Test 6.1: QueryDeltaList Mixed Results
func testQueryDeltaListMixedResults(t *testing.T) {
	data := setupDeltaTestData(t)

	// Create multiple origin queries
    _, err := data.rpc.QueryCreate(data.ctx, connect.NewRequest(&requestv1.QueryCreateRequest{
        ExampleId: data.originExampleID.Bytes(),
        Key:       "k1",
        Enabled:   true,
        Value:     "v1",
    }))
	if err != nil {
		t.Fatal(err)
	}

    _, err = data.rpc.QueryCreate(data.ctx, connect.NewRequest(&requestv1.QueryCreateRequest{
        ExampleId: data.originExampleID.Bytes(),
        Key:       "k2",
        Enabled:   true,
        Value:     "v2",
    }))
	if err != nil {
		t.Fatal(err)
	}

    // no-op: overlay IDs resolved via QueryDeltaList

	// Create delta queries with different source types
    // Create overlay view: copy to delta, then modify one and create a delta-only
    _ = data.rpc.QueryDeltaExampleCopy(data.ctx, data.originExampleID, data.deltaExampleID)
    // Modify k2 via overlay to MIXED
    list, _ := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{ ExampleId: data.deltaExampleID.Bytes(), OriginId: data.originExampleID.Bytes() }))
    // locate k2 overlay id
    var k2id []byte
    for _, it := range list.Msg.Items { if it.Key == "k2" { k2id = it.QueryId; break } }
    if len(k2id) == 0 { t.Fatal("expected k2 in overlay list") }
    _, _ = data.rpc.QueryDeltaUpdate(data.ctx, connect.NewRequest(&requestv1.QueryDeltaUpdateRequest{ QueryId: k2id, Value: stringPtr("modified") }))
    // Create delta-only k3
    _, _ = data.rpc.QueryDeltaCreate(data.ctx, connect.NewRequest(&requestv1.QueryDeltaCreateRequest{ ExampleId: data.deltaExampleID.Bytes(), Key: "k3", Enabled: true, Value: "v3" }))

	// Get delta list
	deltaListResp, err := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{
		ExampleId: data.deltaExampleID.Bytes(),
		OriginId:  data.originExampleID.Bytes(),
	}))
	if err != nil {
		t.Fatal(err)
	}

    if len(deltaListResp.Msg.Items) != 3 {
        t.Fatalf("Expected 3 items, got %d", len(deltaListResp.Msg.Items))
    }

	// Check source types (order might vary)
	sourceTypes := make(map[string]deltav1.SourceKind)
	for _, item := range deltaListResp.Msg.Items {
		sourceTypes[item.Key] = *item.Source
	}

	if sourceTypes["k1"] != deltav1.SourceKind_SOURCE_KIND_ORIGIN {
		t.Errorf("k1 should be ORIGIN, got %v", sourceTypes["k1"])
	}
	if sourceTypes["k2"] != deltav1.SourceKind_SOURCE_KIND_MIXED {
		t.Errorf("k2 should be MIXED, got %v", sourceTypes["k2"])
	}
	if sourceTypes["k3"] != deltav1.SourceKind_SOURCE_KIND_DELTA {
		t.Errorf("k3 should be DELTA, got %v", sourceTypes["k3"])
	}
}

// Test 6.2: Empty Delta List
func testEmptyDeltaList(t *testing.T) {
	data := setupDeltaTestData(t)

	// Create multiple origin queries
	for i := 0; i < 5; i++ {
		_, err := data.rpc.QueryCreate(data.ctx, connect.NewRequest(&requestv1.QueryCreateRequest{
			ExampleId: data.originExampleID.Bytes(),
			Key:       "key" + string(rune('1'+i)),
			Enabled:   true,
			Value:     "value" + string(rune('1'+i)),
		}))
		if err != nil {
			t.Fatal(err)
		}
	}

	// Call QueryDeltaList on empty delta example
	deltaListResp, err := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{
		ExampleId: data.deltaExampleID.Bytes(),
		OriginId:  data.originExampleID.Bytes(),
	}))
	if err != nil {
		t.Fatal(err)
	}

	// Should auto-create 5 items
	if len(deltaListResp.Msg.Items) != 5 {
		t.Fatalf("Expected 5 auto-created items, got %d", len(deltaListResp.Msg.Items))
	}

	// All should be ORIGIN
	for _, item := range deltaListResp.Msg.Items {
		if *item.Source != deltav1.SourceKind_SOURCE_KIND_ORIGIN {
			t.Errorf("Auto-created item should be ORIGIN, got %v", *item.Source)
		}
	}
}
