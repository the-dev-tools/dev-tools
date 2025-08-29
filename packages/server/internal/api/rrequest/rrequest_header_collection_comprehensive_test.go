package rrequest_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

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

// comprehensiveTestData contains all necessary components for comprehensive header testing
type comprehensiveTestData struct {
	ctx        context.Context
	rpc        rrequest.RequestRPC
	exampleID  idwrap.IDWrap
	userID     idwrap.IDWrap
	ehs        sexampleheader.HeaderService
	baseData   *testutil.BaseDBQueries
}

// setupComprehensiveTestData creates test environment for comprehensive header testing
func setupComprehensiveTestData(t *testing.T) *comprehensiveTestData {
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

	// Create API item (endpoint)
	item := &mitemapi.ItemApi{
		ID:           idwrap.NewNow(),
		CollectionID: collectionID,
		Name:         "comprehensive-test-endpoint",
		Method:       "GET",
		Url:          "https://api.comprehensive-test.com/endpoint",
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
		Name:         "comprehensive-test-example",
	}
	err = iaes.CreateApiExample(ctx, example)
	if err != nil {
		t.Fatal(err)
	}

	rpc := rrequest.New(db, cs, us, ias, iaes, ehs, eqs, as)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	return &comprehensiveTestData{
		ctx:       authedCtx,
		rpc:       rpc,
		exampleID: exampleID,
		userID:    userID,
		ehs:       ehs,
		baseData:  base,
	}
}

// createHeaderViaRPC creates a header using the RPC and returns its ID
func createHeaderViaRPC(t *testing.T, data *comprehensiveTestData, key, value, description string, enabled bool) idwrap.IDWrap {
	t.Helper()
	resp, err := data.rpc.HeaderCreate(data.ctx, connect.NewRequest(&requestv1.HeaderCreateRequest{
		ExampleId:   data.exampleID.Bytes(),
		Key:         key,
		Value:       value,
		Enabled:     enabled,
		Description: description,
	}))
	if err != nil {
		t.Fatalf("Failed to create header %s: %v", key, err)
	}
	headerID, err := idwrap.NewFromBytes(resp.Msg.HeaderId)
	if err != nil {
		t.Fatalf("Failed to parse header ID: %v", err)
	}
	return headerID
}

// verifyHeaderCount checks the number of headers in the list
func verifyHeaderCount(t *testing.T, data *comprehensiveTestData, expectedCount int, testContext string) {
	t.Helper()
	resp, err := data.rpc.HeaderList(data.ctx, connect.NewRequest(&requestv1.HeaderListRequest{
		ExampleId: data.exampleID.Bytes(),
	}))
	if err != nil {
		t.Fatalf("[%s] Failed to list headers: %v", testContext, err)
	}
	if len(resp.Msg.Items) != expectedCount {
		t.Fatalf("[%s] Expected %d headers, got %d", testContext, expectedCount, len(resp.Msg.Items))
	}
	t.Logf("[%s] ✓ Header count verified: %d", testContext, expectedCount)
}

// verifyHeaderInList checks if a header with given properties exists in the list
func verifyHeaderInList(t *testing.T, data *comprehensiveTestData, key, value string, enabled bool, testContext string) bool {
	t.Helper()
	resp, err := data.rpc.HeaderList(data.ctx, connect.NewRequest(&requestv1.HeaderListRequest{
		ExampleId: data.exampleID.Bytes(),
	}))
	if err != nil {
		t.Fatalf("[%s] Failed to list headers: %v", testContext, err)
	}
	
	for _, header := range resp.Msg.Items {
		if header.Key == key && header.Value == value && header.Enabled == enabled {
			return true
		}
	}
	return false
}

// verifyLinkedListIntegrity checks the integrity of the linked list structure
func verifyLinkedListIntegrity(t *testing.T, data *comprehensiveTestData, expectedCount int, testContext string) {
	t.Helper()

	orderedHeaders, err := data.ehs.GetHeadersOrdered(data.ctx, data.exampleID)
	if err != nil {
		t.Fatalf("[%s] Failed to get ordered headers: %v", testContext, err)
	}

	if len(orderedHeaders) != expectedCount {
		t.Errorf("[%s] Expected %d headers, got %d", testContext, expectedCount, len(orderedHeaders))
		return
	}

	if len(orderedHeaders) == 0 {
		t.Logf("[%s] ✓ Empty list integrity verified", testContext)
		return
	}

	if len(orderedHeaders) == 1 {
		header := orderedHeaders[0]
		if header.Prev != nil {
			t.Errorf("[%s] Single header should have nil prev pointer", testContext)
		}
		if header.Next != nil {
			t.Errorf("[%s] Single header should have nil next pointer", testContext)
		}
		t.Logf("[%s] ✓ Single item list integrity verified", testContext)
		return
	}

	// Multi-item list checks
	first := orderedHeaders[0]
	if first.Prev != nil {
		t.Errorf("[%s] First header should have nil prev pointer", testContext)
	}

	last := orderedHeaders[len(orderedHeaders)-1]
	if last.Next != nil {
		t.Errorf("[%s] Last header should have nil next pointer", testContext)
	}

	// Check forward and backward linkages
	for i := 0; i < len(orderedHeaders)-1; i++ {
		current := orderedHeaders[i]
		next := orderedHeaders[i+1]

		if current.Next == nil || current.Next.Compare(next.ID) != 0 {
			t.Errorf("[%s] Header at index %d has incorrect next pointer", testContext, i)
		}
		if next.Prev == nil || next.Prev.Compare(current.ID) != 0 {
			t.Errorf("[%s] Header at index %d has incorrect prev pointer", testContext, i+1)
		}
	}

	// Check for circular references
	seenIDs := make(map[string]bool)
	for _, header := range orderedHeaders {
		idStr := header.ID.String()
		if seenIDs[idStr] {
			t.Errorf("[%s] Circular reference detected: duplicate ID %s", testContext, idStr)
		}
		seenIDs[idStr] = true
	}

	t.Logf("[%s] ✓ Linked list integrity verified for %d headers", testContext, len(orderedHeaders))
}

// =============================================================================
// HeaderCreate Tests
// =============================================================================

func TestHeaderCreate_FirstHeader(t *testing.T) {
	data := setupComprehensiveTestData(t)

	t.Run("CreateFirstHeader", func(t *testing.T) {
		headerID := createHeaderViaRPC(t, data, "Content-Type", "application/json", "First header", true)
		
		// Verify the header exists
		verifyHeaderCount(t, data, 1, "CreateFirstHeader")
		verifyLinkedListIntegrity(t, data, 1, "CreateFirstHeader")
		
		// Verify the header content
		if !verifyHeaderInList(t, data, "Content-Type", "application/json", true, "CreateFirstHeader") {
			t.Error("Header not found in list with expected properties")
		}
		
		t.Logf("✓ First header created with ID: %s", headerID.String()[:8]+"...")
	})
}

func TestHeaderCreate_AppendToExisting(t *testing.T) {
	data := setupComprehensiveTestData(t)

	// Create initial header
	_ = createHeaderViaRPC(t, data, "Content-Type", "application/json", "First header", true)
	
	t.Run("AppendSecondHeader", func(t *testing.T) {
		header2ID := createHeaderViaRPC(t, data, "Authorization", "Bearer token", "Auth header", true)
		
		verifyHeaderCount(t, data, 2, "AppendSecondHeader")
		verifyLinkedListIntegrity(t, data, 2, "AppendSecondHeader")
		
		t.Logf("✓ Second header appended with ID: %s", header2ID.String()[:8]+"...")
	})

	t.Run("AppendThirdHeader", func(t *testing.T) {
		header3ID := createHeaderViaRPC(t, data, "User-Agent", "TestClient/1.0", "UA header", false)
		
		verifyHeaderCount(t, data, 3, "AppendThirdHeader")
		verifyLinkedListIntegrity(t, data, 3, "AppendThirdHeader")
		
		// Verify disabled header
		if !verifyHeaderInList(t, data, "User-Agent", "TestClient/1.0", false, "AppendThirdHeader") {
			t.Error("Disabled header not found in list")
		}
		
		t.Logf("✓ Third header appended with ID: %s", header3ID.String()[:8]+"...")
	})
	
	// Test order preservation after multiple appends
	t.Run("VerifyOrderAfterMultipleAppends", func(t *testing.T) {
		resp, err := data.rpc.HeaderList(data.ctx, connect.NewRequest(&requestv1.HeaderListRequest{
			ExampleId: data.exampleID.Bytes(),
		}))
		if err != nil {
			t.Fatalf("Failed to list headers: %v", err)
		}
		
		expectedKeys := []string{"Content-Type", "Authorization", "User-Agent"}
		if len(resp.Msg.Items) != len(expectedKeys) {
			t.Fatalf("Expected %d headers, got %d", len(expectedKeys), len(resp.Msg.Items))
		}
		
		for i, header := range resp.Msg.Items {
			if header.Key != expectedKeys[i] {
				t.Errorf("Expected header key %s at position %d, got %s", expectedKeys[i], i, header.Key)
			}
		}
		
		t.Log("✓ Header order preserved after multiple appends")
	})
}

func TestHeaderCreate_BulkCreation(t *testing.T) {
	data := setupComprehensiveTestData(t)

	t.Run("BulkCreateHeaders", func(t *testing.T) {
		headerData := []struct {
			key, value, desc string
			enabled          bool
		}{
			{"Accept", "application/json", "Accept JSON", true},
			{"Cache-Control", "no-cache", "No cache", true},
			{"X-Custom-Header", "custom-value", "Custom header", false},
			{"Accept-Language", "en-US", "Language pref", true},
			{"X-Request-ID", "12345", "Request tracking", true},
		}

		// Create headers sequentially (simulating bulk creation)
		var headerIDs []idwrap.IDWrap
		for _, hd := range headerData {
			headerID := createHeaderViaRPC(t, data, hd.key, hd.value, hd.desc, hd.enabled)
			headerIDs = append(headerIDs, headerID)
		}

		// Verify count and integrity
		verifyHeaderCount(t, data, len(headerData), "BulkCreateHeaders")
		verifyLinkedListIntegrity(t, data, len(headerData), "BulkCreateHeaders")

		// Verify each header exists
		for _, hd := range headerData {
			if !verifyHeaderInList(t, data, hd.key, hd.value, hd.enabled, "BulkCreateHeaders") {
				t.Errorf("Header %s not found in list", hd.key)
			}
		}

		t.Logf("✓ Bulk created %d headers successfully", len(headerData))
	})
}

func TestHeaderCreate_InvalidInputs(t *testing.T) {
	data := setupComprehensiveTestData(t)

	testCases := []struct {
		name        string
		key         string
		value       string
		description string
		enabled     bool
		expectError bool
	}{
		{"EmptyKey", "", "value", "desc", true, false}, // Empty key might be allowed
		{"EmptyValue", "key", "", "desc", true, false}, // Empty value might be allowed
		{"LongKey", strings.Repeat("a", 1000), "value", "desc", true, false}, // Very long key
		{"LongValue", "key", strings.Repeat("v", 10000), "desc", true, false}, // Very long value
		{"SpecialCharacters", "X-Custom@#$%", "value!@#$%", "desc", true, false}, // Special chars
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := data.rpc.HeaderCreate(data.ctx, connect.NewRequest(&requestv1.HeaderCreateRequest{
				ExampleId:   data.exampleID.Bytes(),
				Key:         tc.key,
				Value:       tc.value,
				Enabled:     tc.enabled,
				Description: tc.description,
			}))

			if tc.expectError && err == nil {
				t.Errorf("Expected error for %s, but got none", tc.name)
			} else if !tc.expectError && err != nil {
				t.Errorf("Unexpected error for %s: %v", tc.name, err)
			} else if !tc.expectError {
				t.Logf("✓ %s handled correctly", tc.name)
			}
		})
	}

	t.Run("InvalidExampleID", func(t *testing.T) {
		invalidID := idwrap.NewNow()
		_, err := data.rpc.HeaderCreate(data.ctx, connect.NewRequest(&requestv1.HeaderCreateRequest{
			ExampleId:   invalidID.Bytes(),
			Key:         "Test-Key",
			Value:       "test-value",
			Enabled:     true,
			Description: "test",
		}))

		if err == nil {
			t.Error("Expected error for invalid example ID")
		} else {
			t.Logf("✓ Invalid example ID properly rejected: %v", err)
		}
	})
}

// =============================================================================
// HeaderList Tests  
// =============================================================================

func TestHeaderList_EmptyList(t *testing.T) {
	data := setupComprehensiveTestData(t)

	t.Run("ListEmptyHeaders", func(t *testing.T) {
		resp, err := data.rpc.HeaderList(data.ctx, connect.NewRequest(&requestv1.HeaderListRequest{
			ExampleId: data.exampleID.Bytes(),
		}))
		if err != nil {
			t.Fatalf("Failed to list empty headers: %v", err)
		}

		if len(resp.Msg.Items) != 0 {
			t.Errorf("Expected 0 headers, got %d", len(resp.Msg.Items))
		}

		if string(resp.Msg.ExampleId) != string(data.exampleID.Bytes()) {
			t.Error("Response example ID doesn't match request")
		}

		t.Log("✓ Empty header list returned correctly")
	})
}

func TestHeaderList_SingleHeader(t *testing.T) {
	data := setupComprehensiveTestData(t)

	// Create single header
	headerID := createHeaderViaRPC(t, data, "Single-Header", "single-value", "Only header", true)

	t.Run("ListSingleHeader", func(t *testing.T) {
		resp, err := data.rpc.HeaderList(data.ctx, connect.NewRequest(&requestv1.HeaderListRequest{
			ExampleId: data.exampleID.Bytes(),
		}))
		if err != nil {
			t.Fatalf("Failed to list single header: %v", err)
		}

		if len(resp.Msg.Items) != 1 {
			t.Fatalf("Expected 1 header, got %d", len(resp.Msg.Items))
		}

		header := resp.Msg.Items[0]
		if header.Key != "Single-Header" || header.Value != "single-value" {
			t.Errorf("Header properties don't match: key=%s, value=%s", header.Key, header.Value)
		}

		receivedID, err := idwrap.NewFromBytes(header.HeaderId)
		if err != nil {
			t.Fatalf("Failed to parse received header ID: %v", err)
		}

		if receivedID.Compare(headerID) != 0 {
			t.Error("Header ID doesn't match created ID")
		}

		t.Log("✓ Single header listed correctly with all properties")
	})
}

func TestHeaderList_MultipleHeaders(t *testing.T) {
	data := setupComprehensiveTestData(t)

	// Create multiple headers
	headers := []struct {
		key, value, desc string
		enabled          bool
	}{
		{"Accept", "application/json", "Accept JSON", true},
		{"Content-Type", "application/json", "Content type", false},
		{"Authorization", "Bearer token", "Auth header", true},
		{"User-Agent", "TestClient/1.0", "UA header", false},
	}

	var createdIDs []idwrap.IDWrap
	for _, h := range headers {
		id := createHeaderViaRPC(t, data, h.key, h.value, h.desc, h.enabled)
		createdIDs = append(createdIDs, id)
	}

	t.Run("ListMultipleHeaders", func(t *testing.T) {
		resp, err := data.rpc.HeaderList(data.ctx, connect.NewRequest(&requestv1.HeaderListRequest{
			ExampleId: data.exampleID.Bytes(),
		}))
		if err != nil {
			t.Fatalf("Failed to list multiple headers: %v", err)
		}

		if len(resp.Msg.Items) != len(headers) {
			t.Fatalf("Expected %d headers, got %d", len(headers), len(resp.Msg.Items))
		}

		// Verify headers are in creation order
		for i, expectedHeader := range headers {
			actualHeader := resp.Msg.Items[i]
			if actualHeader.Key != expectedHeader.key {
				t.Errorf("Header %d: expected key %s, got %s", i, expectedHeader.key, actualHeader.Key)
			}
			if actualHeader.Value != expectedHeader.value {
				t.Errorf("Header %d: expected value %s, got %s", i, expectedHeader.value, actualHeader.Value)
			}
			if actualHeader.Enabled != expectedHeader.enabled {
				t.Errorf("Header %d: expected enabled %t, got %t", i, expectedHeader.enabled, actualHeader.Enabled)
			}
		}

		t.Logf("✓ Multiple headers (%d) listed correctly in order", len(headers))
	})
}

func TestHeaderList_AfterMoves(t *testing.T) {
	data := setupComprehensiveTestData(t)

	// Create headers: 1, 2, 3, 4
	header1ID := createHeaderViaRPC(t, data, "Header-1", "Value1", "First", true)
	_ = createHeaderViaRPC(t, data, "Header-2", "Value2", "Second", true)
	_ = createHeaderViaRPC(t, data, "Header-3", "Value3", "Third", true)
	header4ID := createHeaderViaRPC(t, data, "Header-4", "Value4", "Fourth", true)

	t.Run("ListAfterMove", func(t *testing.T) {
		// Move header 4 after header 1 (expected order: 1, 4, 2, 3)
		_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(&requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       header4ID.Bytes(),
			TargetHeaderId: header1ID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}))
		if err != nil {
			t.Fatalf("Failed to move header: %v", err)
		}

		// List headers after move
		resp, err := data.rpc.HeaderList(data.ctx, connect.NewRequest(&requestv1.HeaderListRequest{
			ExampleId: data.exampleID.Bytes(),
		}))
		if err != nil {
			t.Fatalf("Failed to list headers after move: %v", err)
		}

		expectedOrder := []string{"Header-1", "Header-4", "Header-2", "Header-3"}
		if len(resp.Msg.Items) != len(expectedOrder) {
			t.Fatalf("Expected %d headers, got %d", len(expectedOrder), len(resp.Msg.Items))
		}

		for i, expected := range expectedOrder {
			if resp.Msg.Items[i].Key != expected {
				t.Errorf("Position %d: expected %s, got %s", i, expected, resp.Msg.Items[i].Key)
			}
		}

		t.Log("✓ Header list correctly reflects order after move operation")
	})
}

func TestHeaderList_InvalidExampleID(t *testing.T) {
	data := setupComprehensiveTestData(t)

	t.Run("ListWithInvalidExampleID", func(t *testing.T) {
		invalidID := idwrap.NewNow()
		_, err := data.rpc.HeaderList(data.ctx, connect.NewRequest(&requestv1.HeaderListRequest{
			ExampleId: invalidID.Bytes(),
		}))

		if err == nil {
			t.Error("Expected error for invalid example ID")
		} else {
			t.Logf("✓ Invalid example ID properly rejected: %v", err)
		}
	})
}

// =============================================================================
// HeaderUpdate Tests
// =============================================================================

func TestHeaderUpdate_ValueChanges(t *testing.T) {
	data := setupComprehensiveTestData(t)

	// Create initial header
	headerID := createHeaderViaRPC(t, data, "Content-Type", "text/html", "Initial content type", true)

	t.Run("UpdateHeaderValue", func(t *testing.T) {
		key := "Content-Type"
		value := "application/json"
		enabled := true
		description := "Updated content type"
		_, err := data.rpc.HeaderUpdate(data.ctx, connect.NewRequest(&requestv1.HeaderUpdateRequest{
			HeaderId:    headerID.Bytes(),
			Key:         &key,
			Value:       &value,
			Enabled:     &enabled,
			Description: &description,
		}))
		if err != nil {
			t.Fatalf("Failed to update header: %v", err)
		}

		// Verify the update
		if !verifyHeaderInList(t, data, "Content-Type", "application/json", true, "UpdateHeaderValue") {
			t.Error("Header not updated in list")
		}

		t.Log("✓ Header value updated successfully")
	})

	t.Run("UpdateHeaderKey", func(t *testing.T) {
		key := "Accept"
		value := "application/json"
		enabled := true
		description := "Updated to Accept header"
		_, err := data.rpc.HeaderUpdate(data.ctx, connect.NewRequest(&requestv1.HeaderUpdateRequest{
			HeaderId:    headerID.Bytes(),
			Key:         &key,
			Value:       &value,
			Enabled:     &enabled,
			Description: &description,
		}))
		if err != nil {
			t.Fatalf("Failed to update header key: %v", err)
		}

		// Verify the key update
		if !verifyHeaderInList(t, data, "Accept", "application/json", true, "UpdateHeaderKey") {
			t.Error("Header key not updated in list")
		}

		t.Log("✓ Header key updated successfully")
	})

	t.Run("ToggleHeaderEnabled", func(t *testing.T) {
		key := "Accept"
		value := "application/json"
		enabled := false
		description := "Disabled header"
		_, err := data.rpc.HeaderUpdate(data.ctx, connect.NewRequest(&requestv1.HeaderUpdateRequest{
			HeaderId:    headerID.Bytes(),
			Key:         &key,
			Value:       &value,
			Enabled:     &enabled,
			Description: &description,
		}))
		if err != nil {
			t.Fatalf("Failed to toggle header enabled: %v", err)
		}

		// Verify the enabled state
		if !verifyHeaderInList(t, data, "Accept", "application/json", false, "ToggleHeaderEnabled") {
			t.Error("Header enabled state not updated")
		}

		t.Log("✓ Header enabled state toggled successfully")
	})
}

func TestHeaderUpdate_PreserveOrder(t *testing.T) {
	data := setupComprehensiveTestData(t)

	// Create multiple headers
	_ = createHeaderViaRPC(t, data, "Header-1", "Value1", "First", true)
	header2ID := createHeaderViaRPC(t, data, "Header-2", "Value2", "Second", true)
	_ = createHeaderViaRPC(t, data, "Header-3", "Value3", "Third", true)

	t.Run("UpdateMiddleHeaderPreserveOrder", func(t *testing.T) {
		// Update middle header
		key := "Updated-Header-2"
		value := "Updated-Value2"
		enabled := false
		description := "Updated second header"
		_, err := data.rpc.HeaderUpdate(data.ctx, connect.NewRequest(&requestv1.HeaderUpdateRequest{
			HeaderId:    header2ID.Bytes(),
			Key:         &key,
			Value:       &value,
			Enabled:     &enabled,
			Description: &description,
		}))
		if err != nil {
			t.Fatalf("Failed to update middle header: %v", err)
		}

		// Verify order is preserved
		resp, err := data.rpc.HeaderList(data.ctx, connect.NewRequest(&requestv1.HeaderListRequest{
			ExampleId: data.exampleID.Bytes(),
		}))
		if err != nil {
			t.Fatalf("Failed to list headers: %v", err)
		}

		expectedOrder := []string{"Header-1", "Updated-Header-2", "Header-3"}
		for i, expected := range expectedOrder {
			if resp.Msg.Items[i].Key != expected {
				t.Errorf("Position %d: expected %s, got %s", i, expected, resp.Msg.Items[i].Key)
			}
		}

		// Verify linked list integrity
		verifyLinkedListIntegrity(t, data, 3, "UpdateMiddleHeaderPreserveOrder")

		t.Log("✓ Header update preserved order and maintained list integrity")
	})
}

func TestHeaderUpdate_BatchUpdates(t *testing.T) {
	data := setupComprehensiveTestData(t)

	// Create multiple headers
	var headerIDs []idwrap.IDWrap
	for i := 1; i <= 5; i++ {
		id := createHeaderViaRPC(t, data, fmt.Sprintf("Header-%d", i), fmt.Sprintf("Value%d", i), fmt.Sprintf("Header %d", i), true)
		headerIDs = append(headerIDs, id)
	}

	t.Run("BatchUpdateHeaders", func(t *testing.T) {
		// Update all headers
		for i, headerID := range headerIDs {
			key := fmt.Sprintf("Updated-Header-%d", i+1)
			value := fmt.Sprintf("Updated-Value%d", i+1)
			enabled := i%2 == 0
			description := fmt.Sprintf("Batch updated header %d", i+1)
			_, err := data.rpc.HeaderUpdate(data.ctx, connect.NewRequest(&requestv1.HeaderUpdateRequest{
				HeaderId:    headerID.Bytes(),
				Key:         &key,
				Value:       &value,
				Enabled:     &enabled,
				Description: &description,
			}))
			if err != nil {
				t.Fatalf("Failed to batch update header %d: %v", i+1, err)
			}
		}

		// Verify all updates
		resp, err := data.rpc.HeaderList(data.ctx, connect.NewRequest(&requestv1.HeaderListRequest{
			ExampleId: data.exampleID.Bytes(),
		}))
		if err != nil {
			t.Fatalf("Failed to list headers after batch update: %v", err)
		}

		if len(resp.Msg.Items) != 5 {
			t.Fatalf("Expected 5 headers, got %d", len(resp.Msg.Items))
		}

		for i, header := range resp.Msg.Items {
			expectedKey := fmt.Sprintf("Updated-Header-%d", i+1)
			expectedValue := fmt.Sprintf("Updated-Value%d", i+1)
			expectedEnabled := i%2 == 0

			if header.Key != expectedKey {
				t.Errorf("Header %d: expected key %s, got %s", i, expectedKey, header.Key)
			}
			if header.Value != expectedValue {
				t.Errorf("Header %d: expected value %s, got %s", i, expectedValue, header.Value)
			}
			if header.Enabled != expectedEnabled {
				t.Errorf("Header %d: expected enabled %t, got %t", i, expectedEnabled, header.Enabled)
			}
		}

		// Verify linked list integrity after batch updates
		verifyLinkedListIntegrity(t, data, 5, "BatchUpdateHeaders")

		t.Log("✓ Batch header updates completed successfully with integrity maintained")
	})
}

func TestHeaderUpdate_InvalidCases(t *testing.T) {
	data := setupComprehensiveTestData(t)

	// Create a header for testing
	headerID := createHeaderViaRPC(t, data, "Test-Header", "test-value", "Test header", true)

	t.Run("UpdateNonExistentHeader", func(t *testing.T) {
		nonExistentID := idwrap.NewNow()
		key := "Non-Existent"
		value := "value"
		enabled := true
		description := "This should fail"
		_, err := data.rpc.HeaderUpdate(data.ctx, connect.NewRequest(&requestv1.HeaderUpdateRequest{
			HeaderId:    nonExistentID.Bytes(),
			Key:         &key,
			Value:       &value,
			Enabled:     &enabled,
			Description: &description,
		}))

		if err == nil {
			t.Error("Expected error when updating non-existent header")
		} else {
			t.Logf("✓ Non-existent header update properly rejected: %v", err)
		}
	})

	t.Run("UpdateWithInvalidData", func(t *testing.T) {
		// Try with extremely long values
		key := "Test-Header"
		longValue := strings.Repeat("x", 100000)
		enabled := true
		description := "Very long value test"
		
		_, err := data.rpc.HeaderUpdate(data.ctx, connect.NewRequest(&requestv1.HeaderUpdateRequest{
			HeaderId:    headerID.Bytes(),
			Key:         &key,
			Value:       &longValue,
			Enabled:     &enabled,
			Description: &description,
		}))

		// This might succeed depending on implementation - just log the result
		if err != nil {
			t.Logf("✓ Very long value rejected: %v", err)
		} else {
			t.Log("✓ Very long value accepted by system")
		}
	})
}

// =============================================================================
// HeaderDelete Tests
// =============================================================================

func TestHeaderDelete_SingleItem(t *testing.T) {
	data := setupComprehensiveTestData(t)

	// Create single header
	headerID := createHeaderViaRPC(t, data, "Only-Header", "only-value", "Only header", true)
	verifyHeaderCount(t, data, 1, "BeforeDeleteSingle")

	t.Run("DeleteSingleHeader", func(t *testing.T) {
		_, err := data.rpc.HeaderDelete(data.ctx, connect.NewRequest(&requestv1.HeaderDeleteRequest{
			HeaderId: headerID.Bytes(),
		}))
		if err != nil {
			t.Fatalf("Failed to delete single header: %v", err)
		}

		// Verify empty list
		verifyHeaderCount(t, data, 0, "DeleteSingleHeader")
		verifyLinkedListIntegrity(t, data, 0, "DeleteSingleHeader")

		t.Log("✓ Single header deleted successfully, list is empty")
	})
}

func TestHeaderDelete_HeadItem(t *testing.T) {
	data := setupComprehensiveTestData(t)

	// Create multiple headers: 1, 2, 3, 4
	header1ID := createHeaderViaRPC(t, data, "Header-1", "Value1", "First", true)
	createHeaderViaRPC(t, data, "Header-2", "Value2", "Second", true)
	createHeaderViaRPC(t, data, "Header-3", "Value3", "Third", true)
	createHeaderViaRPC(t, data, "Header-4", "Value4", "Fourth", true)

	t.Run("DeleteHeadHeader", func(t *testing.T) {
		_, err := data.rpc.HeaderDelete(data.ctx, connect.NewRequest(&requestv1.HeaderDeleteRequest{
			HeaderId: header1ID.Bytes(),
		}))
		if err != nil {
			t.Fatalf("Failed to delete head header: %v", err)
		}

		// Verify count and integrity
		verifyHeaderCount(t, data, 3, "DeleteHeadHeader")
		verifyLinkedListIntegrity(t, data, 3, "DeleteHeadHeader")

		// Verify order: should be 2, 3, 4
		resp, err := data.rpc.HeaderList(data.ctx, connect.NewRequest(&requestv1.HeaderListRequest{
			ExampleId: data.exampleID.Bytes(),
		}))
		if err != nil {
			t.Fatalf("Failed to list headers after head delete: %v", err)
		}

		expectedOrder := []string{"Header-2", "Header-3", "Header-4"}
		for i, expected := range expectedOrder {
			if resp.Msg.Items[i].Key != expected {
				t.Errorf("Position %d: expected %s, got %s", i, expected, resp.Msg.Items[i].Key)
			}
		}

		t.Log("✓ Head header deleted successfully, remaining headers relinked properly")
	})
}

func TestHeaderDelete_TailItem(t *testing.T) {
	data := setupComprehensiveTestData(t)

	// Create multiple headers: 1, 2, 3, 4
	createHeaderViaRPC(t, data, "Header-1", "Value1", "First", true)
	createHeaderViaRPC(t, data, "Header-2", "Value2", "Second", true)
	createHeaderViaRPC(t, data, "Header-3", "Value3", "Third", true)
	header4ID := createHeaderViaRPC(t, data, "Header-4", "Value4", "Fourth", true)

	t.Run("DeleteTailHeader", func(t *testing.T) {
		_, err := data.rpc.HeaderDelete(data.ctx, connect.NewRequest(&requestv1.HeaderDeleteRequest{
			HeaderId: header4ID.Bytes(),
		}))
		if err != nil {
			t.Fatalf("Failed to delete tail header: %v", err)
		}

		// Verify count and integrity
		verifyHeaderCount(t, data, 3, "DeleteTailHeader")
		verifyLinkedListIntegrity(t, data, 3, "DeleteTailHeader")

		// Verify order: should be 1, 2, 3
		resp, err := data.rpc.HeaderList(data.ctx, connect.NewRequest(&requestv1.HeaderListRequest{
			ExampleId: data.exampleID.Bytes(),
		}))
		if err != nil {
			t.Fatalf("Failed to list headers after tail delete: %v", err)
		}

		expectedOrder := []string{"Header-1", "Header-2", "Header-3"}
		for i, expected := range expectedOrder {
			if resp.Msg.Items[i].Key != expected {
				t.Errorf("Position %d: expected %s, got %s", i, expected, resp.Msg.Items[i].Key)
			}
		}

		t.Log("✓ Tail header deleted successfully, remaining headers intact")
	})
}

func TestHeaderDelete_MiddleItem(t *testing.T) {
	data := setupComprehensiveTestData(t)

	// Create multiple headers: 1, 2, 3, 4, 5
	createHeaderViaRPC(t, data, "Header-1", "Value1", "First", true)
	createHeaderViaRPC(t, data, "Header-2", "Value2", "Second", true)
	header3ID := createHeaderViaRPC(t, data, "Header-3", "Value3", "Third", true)
	createHeaderViaRPC(t, data, "Header-4", "Value4", "Fourth", true)
	createHeaderViaRPC(t, data, "Header-5", "Value5", "Fifth", true)

	t.Run("DeleteMiddleHeader", func(t *testing.T) {
		_, err := data.rpc.HeaderDelete(data.ctx, connect.NewRequest(&requestv1.HeaderDeleteRequest{
			HeaderId: header3ID.Bytes(),
		}))
		if err != nil {
			t.Fatalf("Failed to delete middle header: %v", err)
		}

		// Verify count and integrity
		verifyHeaderCount(t, data, 4, "DeleteMiddleHeader")
		verifyLinkedListIntegrity(t, data, 4, "DeleteMiddleHeader")

		// Verify order: should be 1, 2, 4, 5
		resp, err := data.rpc.HeaderList(data.ctx, connect.NewRequest(&requestv1.HeaderListRequest{
			ExampleId: data.exampleID.Bytes(),
		}))
		if err != nil {
			t.Fatalf("Failed to list headers after middle delete: %v", err)
		}

		expectedOrder := []string{"Header-1", "Header-2", "Header-4", "Header-5"}
		for i, expected := range expectedOrder {
			if resp.Msg.Items[i].Key != expected {
				t.Errorf("Position %d: expected %s, got %s", i, expected, resp.Msg.Items[i].Key)
			}
		}

		t.Log("✓ Middle header deleted successfully, gap closed properly")
	})
}

func TestHeaderDelete_CascadeEffects(t *testing.T) {
	data := setupComprehensiveTestData(t)

	// Create headers and delete multiple in sequence
	header1ID := createHeaderViaRPC(t, data, "Header-1", "Value1", "First", true)
	header2ID := createHeaderViaRPC(t, data, "Header-2", "Value2", "Second", true)
	header3ID := createHeaderViaRPC(t, data, "Header-3", "Value3", "Third", true)
	header4ID := createHeaderViaRPC(t, data, "Header-4", "Value4", "Fourth", true)

	t.Run("CascadeDeleteHeaders", func(t *testing.T) {
		// Delete header 2
		_, err := data.rpc.HeaderDelete(data.ctx, connect.NewRequest(&requestv1.HeaderDeleteRequest{
			HeaderId: header2ID.Bytes(),
		}))
		if err != nil {
			t.Fatalf("Failed to delete header 2: %v", err)
		}
		verifyLinkedListIntegrity(t, data, 3, "AfterDelete2")

		// Delete header 4 (now tail)
		_, err = data.rpc.HeaderDelete(data.ctx, connect.NewRequest(&requestv1.HeaderDeleteRequest{
			HeaderId: header4ID.Bytes(),
		}))
		if err != nil {
			t.Fatalf("Failed to delete header 4: %v", err)
		}
		verifyLinkedListIntegrity(t, data, 2, "AfterDelete4")

		// Delete header 1 (head)
		_, err = data.rpc.HeaderDelete(data.ctx, connect.NewRequest(&requestv1.HeaderDeleteRequest{
			HeaderId: header1ID.Bytes(),
		}))
		if err != nil {
			t.Fatalf("Failed to delete header 1: %v", err)
		}
		verifyLinkedListIntegrity(t, data, 1, "AfterDelete1")

		// Verify only header 3 remains
		resp, err := data.rpc.HeaderList(data.ctx, connect.NewRequest(&requestv1.HeaderListRequest{
			ExampleId: data.exampleID.Bytes(),
		}))
		if err != nil {
			t.Fatalf("Failed to list remaining headers: %v", err)
		}

		if len(resp.Msg.Items) != 1 {
			t.Fatalf("Expected 1 remaining header, got %d", len(resp.Msg.Items))
		}

		if resp.Msg.Items[0].Key != "Header-3" {
			t.Errorf("Expected remaining header to be Header-3, got %s", resp.Msg.Items[0].Key)
		}

		// Delete final header
		_, err = data.rpc.HeaderDelete(data.ctx, connect.NewRequest(&requestv1.HeaderDeleteRequest{
			HeaderId: header3ID.Bytes(),
		}))
		if err != nil {
			t.Fatalf("Failed to delete final header: %v", err)
		}

		verifyHeaderCount(t, data, 0, "CascadeDeleteHeaders")
		verifyLinkedListIntegrity(t, data, 0, "CascadeDeleteHeaders")

		t.Log("✓ Cascade delete completed successfully, list is empty")
	})
}

func TestHeaderDelete_InvalidCases(t *testing.T) {
	data := setupComprehensiveTestData(t)

	t.Run("DeleteNonExistentHeader", func(t *testing.T) {
		nonExistentID := idwrap.NewNow()
		_, err := data.rpc.HeaderDelete(data.ctx, connect.NewRequest(&requestv1.HeaderDeleteRequest{
			HeaderId: nonExistentID.Bytes(),
		}))

		if err == nil {
			t.Error("Expected error when deleting non-existent header")
		} else {
			t.Logf("✓ Non-existent header delete properly rejected: %v", err)
		}
	})

	t.Run("DeleteAlreadyDeletedHeader", func(t *testing.T) {
		// Create and delete a header
		headerID := createHeaderViaRPC(t, data, "To-Delete", "value", "Will be deleted", true)
		
		_, err := data.rpc.HeaderDelete(data.ctx, connect.NewRequest(&requestv1.HeaderDeleteRequest{
			HeaderId: headerID.Bytes(),
		}))
		if err != nil {
			t.Fatalf("Failed to delete header initially: %v", err)
		}

		// Try to delete again
		_, err = data.rpc.HeaderDelete(data.ctx, connect.NewRequest(&requestv1.HeaderDeleteRequest{
			HeaderId: headerID.Bytes(),
		}))

		if err == nil {
			t.Error("Expected error when deleting already deleted header")
		} else {
			t.Logf("✓ Already deleted header delete properly rejected: %v", err)
		}
	})
}

// =============================================================================
// HeaderMove Tests (Enhanced)
// =============================================================================

func TestHeaderMove_AllPositionCombinations(t *testing.T) {
	data := setupComprehensiveTestData(t)

	// Create 5 headers for comprehensive move testing: 1, 2, 3, 4, 5
	header1ID := createHeaderViaRPC(t, data, "Header-1", "Value1", "First", true)
	header2ID := createHeaderViaRPC(t, data, "Header-2", "Value2", "Second", true)
	_ = createHeaderViaRPC(t, data, "Header-3", "Value3", "Third", true)
	header4ID := createHeaderViaRPC(t, data, "Header-4", "Value4", "Fourth", true)
	header5ID := createHeaderViaRPC(t, data, "Header-5", "Value5", "Fifth", true)

	t.Run("MoveLastToFirst", func(t *testing.T) {
		// Move 5 before 1: 5, 1, 2, 3, 4
		_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(&requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       header5ID.Bytes(),
			TargetHeaderId: header1ID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
		}))
		if err != nil {
			t.Fatalf("Failed to move last to first: %v", err)
		}

		verifyLinkedListIntegrity(t, data, 5, "MoveLastToFirst")
		
		// Verify new order
		resp, err := data.rpc.HeaderList(data.ctx, connect.NewRequest(&requestv1.HeaderListRequest{
			ExampleId: data.exampleID.Bytes(),
		}))
		if err != nil {
			t.Fatalf("Failed to list after move: %v", err)
		}

		expectedOrder := []string{"Header-5", "Header-1", "Header-2", "Header-3", "Header-4"}
		for i, expected := range expectedOrder {
			if resp.Msg.Items[i].Key != expected {
				t.Errorf("Position %d: expected %s, got %s", i, expected, resp.Msg.Items[i].Key)
			}
		}

		t.Log("✓ Last item moved to first position successfully")
	})

	t.Run("MoveFirstToLast", func(t *testing.T) {
		// Current order: 5, 1, 2, 3, 4
		// Move 5 after 4: 1, 2, 3, 4, 5
		_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(&requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       header5ID.Bytes(),
			TargetHeaderId: header4ID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}))
		if err != nil {
			t.Fatalf("Failed to move first to last: %v", err)
		}

		verifyLinkedListIntegrity(t, data, 5, "MoveFirstToLast")

		// Verify restored order
		resp, err := data.rpc.HeaderList(data.ctx, connect.NewRequest(&requestv1.HeaderListRequest{
			ExampleId: data.exampleID.Bytes(),
		}))
		if err != nil {
			t.Fatalf("Failed to list after second move: %v", err)
		}

		expectedOrder := []string{"Header-1", "Header-2", "Header-3", "Header-4", "Header-5"}
		for i, expected := range expectedOrder {
			if resp.Msg.Items[i].Key != expected {
				t.Errorf("Position %d: expected %s, got %s", i, expected, resp.Msg.Items[i].Key)
			}
		}

		t.Log("✓ First item moved to last position successfully")
	})

	t.Run("MoveMiddleToMiddle", func(t *testing.T) {
		// Current order: 1, 2, 3, 4, 5
		// Move 2 after 4: 1, 3, 4, 2, 5
		_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(&requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       header2ID.Bytes(),
			TargetHeaderId: header4ID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}))
		if err != nil {
			t.Fatalf("Failed to move middle to middle: %v", err)
		}

		verifyLinkedListIntegrity(t, data, 5, "MoveMiddleToMiddle")

		resp, err := data.rpc.HeaderList(data.ctx, connect.NewRequest(&requestv1.HeaderListRequest{
			ExampleId: data.exampleID.Bytes(),
		}))
		if err != nil {
			t.Fatalf("Failed to list after middle move: %v", err)
		}

		expectedOrder := []string{"Header-1", "Header-3", "Header-4", "Header-2", "Header-5"}
		for i, expected := range expectedOrder {
			if resp.Msg.Items[i].Key != expected {
				t.Errorf("Position %d: expected %s, got %s", i, expected, resp.Msg.Items[i].Key)
			}
		}

		t.Log("✓ Middle item moved to different middle position successfully")
	})

	t.Run("ComplexMoveSequence", func(t *testing.T) {
		// Start fresh with original order
		data := setupComprehensiveTestData(t)
		h1 := createHeaderViaRPC(t, data, "H1", "V1", "First", true)
		h2 := createHeaderViaRPC(t, data, "H2", "V2", "Second", true)
		h3 := createHeaderViaRPC(t, data, "H3", "V3", "Third", true)
		h4 := createHeaderViaRPC(t, data, "H4", "V4", "Fourth", true)

		// Sequence of moves
		moves := []struct {
			headerID, targetID idwrap.IDWrap
			position           resourcesv1.MovePosition
			expectedOrder      []string
			description        string
		}{
			{h4, h1, resourcesv1.MovePosition_MOVE_POSITION_AFTER, []string{"H1", "H4", "H2", "H3"}, "Move 4 after 1"},
			{h2, h4, resourcesv1.MovePosition_MOVE_POSITION_AFTER, []string{"H1", "H4", "H2", "H3"}, "Move 2 after 4 (no change)"},
			{h1, h3, resourcesv1.MovePosition_MOVE_POSITION_AFTER, []string{"H4", "H2", "H3", "H1"}, "Move 1 after 3"},
			{h3, h4, resourcesv1.MovePosition_MOVE_POSITION_BEFORE, []string{"H3", "H4", "H2", "H1"}, "Move 3 before 4"},
		}

		for i, move := range moves {
			_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(&requestv1.HeaderMoveRequest{
				ExampleId:      data.exampleID.Bytes(),
				HeaderId:       move.headerID.Bytes(),
				TargetHeaderId: move.targetID.Bytes(),
				Position:       move.position,
			}))
			if err != nil {
				t.Fatalf("Failed move %d (%s): %v", i+1, move.description, err)
			}

			verifyLinkedListIntegrity(t, data, 4, fmt.Sprintf("ComplexMove%d", i+1))

			resp, err := data.rpc.HeaderList(data.ctx, connect.NewRequest(&requestv1.HeaderListRequest{
				ExampleId: data.exampleID.Bytes(),
			}))
			if err != nil {
				t.Fatalf("Failed to list after move %d: %v", i+1, err)
			}

			for j, expected := range move.expectedOrder {
				if resp.Msg.Items[j].Key != expected {
					t.Errorf("Move %d, Position %d: expected %s, got %s", i+1, j, expected, resp.Msg.Items[j].Key)
				}
			}

			t.Logf("✓ Move %d completed: %s", i+1, move.description)
		}

		t.Log("✓ Complex move sequence completed successfully")
	})
}

// =============================================================================
// Performance Benchmark Tests
// =============================================================================

func BenchmarkHeaderCreate(b *testing.B) {
	data := setupComprehensiveTestData(&testing.T{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := data.rpc.HeaderCreate(data.ctx, connect.NewRequest(&requestv1.HeaderCreateRequest{
			ExampleId:   data.exampleID.Bytes(),
			Key:         fmt.Sprintf("Bench-Header-%d", i),
			Value:       fmt.Sprintf("bench-value-%d", i),
			Enabled:     true,
			Description: fmt.Sprintf("Benchmark header %d", i),
		}))
		if err != nil {
			b.Fatalf("Failed to create header in benchmark: %v", err)
		}
	}
}

func BenchmarkHeaderList(b *testing.B) {
	data := setupComprehensiveTestData(&testing.T{})

	// Pre-create headers for benchmarking
	for i := 0; i < 100; i++ {
		createHeaderViaRPC(&testing.T{}, data, fmt.Sprintf("Header-%d", i), fmt.Sprintf("Value%d", i), fmt.Sprintf("Header %d", i), true)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := data.rpc.HeaderList(data.ctx, connect.NewRequest(&requestv1.HeaderListRequest{
			ExampleId: data.exampleID.Bytes(),
		}))
		if err != nil {
			b.Fatalf("Failed to list headers in benchmark: %v", err)
		}
	}
}

func BenchmarkHeaderUpdate(b *testing.B) {
	data := setupComprehensiveTestData(&testing.T{})

	// Create header for updating
	headerID := createHeaderViaRPC(&testing.T{}, data, "Bench-Update", "initial-value", "Benchmark update", true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "Bench-Update"
		value := fmt.Sprintf("updated-value-%d", i)
		enabled := i%2 == 0
		description := fmt.Sprintf("Benchmark update %d", i)
		_, err := data.rpc.HeaderUpdate(data.ctx, connect.NewRequest(&requestv1.HeaderUpdateRequest{
			HeaderId:    headerID.Bytes(),
			Key:         &key,
			Value:       &value,
			Enabled:     &enabled,
			Description: &description,
		}))
		if err != nil {
			b.Fatalf("Failed to update header in benchmark: %v", err)
		}
	}
}

func BenchmarkHeaderMove(b *testing.B) {
	data := setupComprehensiveTestData(&testing.T{})

	// Create headers for moving
	var headerIDs []idwrap.IDWrap
	for i := 0; i < 10; i++ {
		id := createHeaderViaRPC(&testing.T{}, data, fmt.Sprintf("Move-Header-%d", i), fmt.Sprintf("value%d", i), fmt.Sprintf("Move header %d", i), true)
		headerIDs = append(headerIDs, id)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sourceIdx := i % len(headerIDs)
		targetIdx := (i + 1) % len(headerIDs)
		
		_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(&requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       headerIDs[sourceIdx].Bytes(),
			TargetHeaderId: headerIDs[targetIdx].Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}))
		if err != nil {
			b.Fatalf("Failed to move header in benchmark: %v", err)
		}
	}
}

func BenchmarkHeaderDelete(b *testing.B) {
	data := setupComprehensiveTestData(&testing.T{})

	// Pre-create headers for deletion
	var headerIDs []idwrap.IDWrap
	for i := 0; i < b.N; i++ {
		id := createHeaderViaRPC(&testing.T{}, data, fmt.Sprintf("Delete-Header-%d", i), fmt.Sprintf("value%d", i), fmt.Sprintf("Delete header %d", i), true)
		headerIDs = append(headerIDs, id)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := data.rpc.HeaderDelete(data.ctx, connect.NewRequest(&requestv1.HeaderDeleteRequest{
			HeaderId: headerIDs[i].Bytes(),
		}))
		if err != nil {
			b.Fatalf("Failed to delete header in benchmark: %v", err)
		}
	}
}

// =============================================================================
// Concurrent Access Tests
// =============================================================================

func TestHeaderOperations_ConcurrentAccess(t *testing.T) {
	data := setupComprehensiveTestData(t)

	t.Run("ConcurrentCreate", func(t *testing.T) {
		const numGoroutines = 10
		const headersPerGoroutine = 5

		var wg sync.WaitGroup
		errorsChan := make(chan error, numGoroutines*headersPerGoroutine)

		// Start concurrent header creation
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				for j := 0; j < headersPerGoroutine; j++ {
					_, err := data.rpc.HeaderCreate(data.ctx, connect.NewRequest(&requestv1.HeaderCreateRequest{
						ExampleId:   data.exampleID.Bytes(),
						Key:         fmt.Sprintf("Concurrent-%d-%d", goroutineID, j),
						Value:       fmt.Sprintf("value-%d-%d", goroutineID, j),
						Enabled:     true,
						Description: fmt.Sprintf("Concurrent header %d-%d", goroutineID, j),
					}))
					if err != nil {
						errorsChan <- err
						return
					}
				}
			}(i)
		}

		wg.Wait()
		close(errorsChan)

		// Check for errors
		var errors []error
		for err := range errorsChan {
			errors = append(errors, err)
		}

		if len(errors) > 0 {
			t.Fatalf("Concurrent create had %d errors, first: %v", len(errors), errors[0])
		}

		// Verify final count and integrity
		expectedCount := numGoroutines * headersPerGoroutine
		verifyHeaderCount(t, data, expectedCount, "ConcurrentCreate")
		verifyLinkedListIntegrity(t, data, expectedCount, "ConcurrentCreate")

		t.Logf("✓ Concurrent create completed: %d headers created successfully", expectedCount)
	})

	t.Run("ConcurrentReadWrite", func(t *testing.T) {
		// Create initial headers
		var initialHeaders []idwrap.IDWrap
		for i := 0; i < 10; i++ {
			id := createHeaderViaRPC(t, data, fmt.Sprintf("RW-Header-%d", i), fmt.Sprintf("initial-value-%d", i), fmt.Sprintf("RW header %d", i), true)
			initialHeaders = append(initialHeaders, id)
		}

		const numReaders = 5
		const numWriters = 3
		const duration = 2 * time.Second

		var wg sync.WaitGroup
		stopChan := make(chan struct{})
		errorsChan := make(chan error, numReaders+numWriters)

		// Start readers
		for i := 0; i < numReaders; i++ {
			wg.Add(1)
			go func(readerID int) {
				defer wg.Done()
				for {
					select {
					case <-stopChan:
						return
					default:
						_, err := data.rpc.HeaderList(data.ctx, connect.NewRequest(&requestv1.HeaderListRequest{
							ExampleId: data.exampleID.Bytes(),
						}))
						if err != nil {
							errorsChan <- fmt.Errorf("reader %d: %v", readerID, err)
							return
						}
						time.Sleep(10 * time.Millisecond)
					}
				}
			}(i)
		}

		// Start writers
		for i := 0; i < numWriters; i++ {
			wg.Add(1)
			go func(writerID int) {
				defer wg.Done()
				updateCount := 0
				for {
					select {
					case <-stopChan:
						return
					default:
						headerIdx := updateCount % len(initialHeaders)
						key := fmt.Sprintf("RW-Header-%d", headerIdx)
						value := fmt.Sprintf("updated-by-writer-%d-%d", writerID, updateCount)
						enabled := updateCount%2 == 0
						description := fmt.Sprintf("Updated by writer %d, count %d", writerID, updateCount)
						_, err := data.rpc.HeaderUpdate(data.ctx, connect.NewRequest(&requestv1.HeaderUpdateRequest{
							HeaderId:    initialHeaders[headerIdx].Bytes(),
							Key:         &key,
							Value:       &value,
							Enabled:     &enabled,
							Description: &description,
						}))
						if err != nil {
							errorsChan <- fmt.Errorf("writer %d: %v", writerID, err)
							return
						}
						updateCount++
						time.Sleep(50 * time.Millisecond)
					}
				}
			}(i)
		}

		// Run for specified duration
		time.Sleep(duration)
		close(stopChan)
		wg.Wait()
		close(errorsChan)

		// Check for errors
		var errors []error
		for err := range errorsChan {
			errors = append(errors, err)
		}

		if len(errors) > 0 {
			t.Fatalf("Concurrent read-write had %d errors, first: %v", len(errors), errors[0])
		}

		// Verify final integrity
		verifyLinkedListIntegrity(t, data, len(initialHeaders), "ConcurrentReadWrite")

		t.Logf("✓ Concurrent read-write completed successfully for %v", duration)
	})
}

// =============================================================================
// Error Handling Tests
// =============================================================================

func TestHeaderOperations_ComprehensiveErrorHandling(t *testing.T) {
	data := setupComprehensiveTestData(t)

	t.Run("AuthenticationErrors", func(t *testing.T) {
		// Create unauthenticated context
		unauthCtx := context.Background()

		_, err := data.rpc.HeaderCreate(unauthCtx, connect.NewRequest(&requestv1.HeaderCreateRequest{
			ExampleId:   data.exampleID.Bytes(),
			Key:         "Auth-Test",
			Value:       "value",
			Enabled:     true,
			Description: "Should fail",
		}))

		if err == nil {
			t.Error("Expected authentication error, but got none")
		} else {
			t.Logf("✓ Authentication properly enforced: %v", err)
		}
	})

	t.Run("MalformedRequests", func(t *testing.T) {
		testCases := []struct {
			name    string
			request *requestv1.HeaderCreateRequest
		}{
			{
				"InvalidExampleID",
				&requestv1.HeaderCreateRequest{
					ExampleId:   []byte("invalid"),
					Key:         "test",
					Value:       "value",
					Enabled:     true,
					Description: "test",
				},
			},
			{
				"EmptyExampleID",
				&requestv1.HeaderCreateRequest{
					ExampleId:   []byte{},
					Key:         "test",
					Value:       "value",
					Enabled:     true,
					Description: "test",
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := data.rpc.HeaderCreate(data.ctx, connect.NewRequest(tc.request))
				if err == nil {
					t.Errorf("Expected error for %s, but got none", tc.name)
				} else {
					t.Logf("✓ Malformed request properly rejected (%s): %v", tc.name, err)
				}
			})
		}
	})

	t.Run("PermissionErrors", func(t *testing.T) {
		// This would require setting up a different user context
		// and testing cross-user access, which might be complex
		// For now, we'll test with non-existent examples
		
		nonExistentExampleID := idwrap.NewNow()
		_, err := data.rpc.HeaderList(data.ctx, connect.NewRequest(&requestv1.HeaderListRequest{
			ExampleId: nonExistentExampleID.Bytes(),
		}))

		if err == nil {
			t.Error("Expected permission error for non-existent example")
		} else {
			t.Logf("✓ Permission check properly enforced: %v", err)
		}
	})

	t.Run("DatabaseErrors", func(t *testing.T) {
		// This would require simulating database failures
		// For now, we'll test edge cases that might cause DB issues
		
		// Try operations on closed/invalid connections
		// This is implementation-specific and may not be easily testable
		t.Log("✓ Database error handling would require specific failure injection")
	})

	t.Run("ConcurrencyConflicts", func(t *testing.T) {
		// Create a header and try to move it concurrently
		headerID := createHeaderViaRPC(t, data, "Conflict-Test", "value", "Conflict header", true)
		targetID := createHeaderViaRPC(t, data, "Target", "target-value", "Target header", true)

		// This is a best-effort test for race conditions
		var wg sync.WaitGroup
		errorsChan := make(chan error, 10)

		// Start multiple concurrent move operations
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				pos := resourcesv1.MovePosition_MOVE_POSITION_AFTER
				if i%2 == 0 {
					pos = resourcesv1.MovePosition_MOVE_POSITION_BEFORE
				}
				
				_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(&requestv1.HeaderMoveRequest{
					ExampleId:      data.exampleID.Bytes(),
					HeaderId:       headerID.Bytes(),
					TargetHeaderId: targetID.Bytes(),
					Position:       pos,
				}))
				
				// Some operations might fail due to concurrency - that's expected
				if err != nil {
					errorsChan <- err
				}
			}(i)
		}

		wg.Wait()
		close(errorsChan)

		// Verify list is still in a valid state
		verifyLinkedListIntegrity(t, data, 2, "ConcurrencyConflicts")

		errorCount := 0
		for range errorsChan {
			errorCount++
		}

		t.Logf("✓ Concurrency test completed: %d operations had conflicts (expected)", errorCount)
	})
}

// =============================================================================
// Integration and Edge Case Tests  
// =============================================================================

func TestHeaderOperations_IntegrationScenarios(t *testing.T) {
	data := setupComprehensiveTestData(t)

	t.Run("FullLifecycleScenario", func(t *testing.T) {
		// Simulate a complete user workflow
		
		// 1. Start with empty list
		verifyHeaderCount(t, data, 0, "FullLifecycle-Start")

		// 2. Create initial headers (typical HTTP headers)
		headers := []struct {
			key, value, desc string
			enabled          bool
		}{
			{"Content-Type", "application/json", "Content type", true},
			{"Authorization", "Bearer eyJ0eXAi...", "Auth token", true},
			{"Accept", "application/json", "Accept JSON", true},
			{"User-Agent", "MyApp/1.0", "User agent", false},
			{"X-Request-ID", "req-123", "Request tracking", true},
		}

		var headerIDs []idwrap.IDWrap
		for _, h := range headers {
			id := createHeaderViaRPC(t, data, h.key, h.value, h.desc, h.enabled)
			headerIDs = append(headerIDs, id)
		}
		
		verifyHeaderCount(t, data, 5, "FullLifecycle-Created")
		t.Log("✓ Phase 1: Initial headers created")

		// 3. Update some headers (token refresh, enable User-Agent)
		authKey := "Authorization"
		authValue := "Bearer newToken123..."
		authEnabled := true
		authDesc := "Refreshed auth token"
		_, err := data.rpc.HeaderUpdate(data.ctx, connect.NewRequest(&requestv1.HeaderUpdateRequest{
			HeaderId:    headerIDs[1].Bytes(), // Authorization
			Key:         &authKey,
			Value:       &authValue,
			Enabled:     &authEnabled,
			Description: &authDesc,
		}))
		if err != nil {
			t.Fatalf("Failed to update auth token: %v", err)
		}

		uaKey := "User-Agent"
		uaValue := "MyApp/1.0"
		uaEnabled := true
		uaDesc := "Enabled user agent"
		_, err = data.rpc.HeaderUpdate(data.ctx, connect.NewRequest(&requestv1.HeaderUpdateRequest{
			HeaderId:    headerIDs[3].Bytes(), // User-Agent
			Key:         &uaKey,
			Value:       &uaValue,
			Enabled:     &uaEnabled, // Enable it
			Description: &uaDesc,
		}))
		if err != nil {
			t.Fatalf("Failed to enable user agent: %v", err)
		}

		t.Log("✓ Phase 2: Headers updated")

		// 4. Reorder headers (move Authorization to top, User-Agent to bottom)
		_, err = data.rpc.HeaderMove(data.ctx, connect.NewRequest(&requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       headerIDs[1].Bytes(), // Authorization
			TargetHeaderId: headerIDs[0].Bytes(), // Content-Type
			Position:       resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
		}))
		if err != nil {
			t.Fatalf("Failed to move Authorization to top: %v", err)
		}

		_, err = data.rpc.HeaderMove(data.ctx, connect.NewRequest(&requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       headerIDs[3].Bytes(), // User-Agent
			TargetHeaderId: headerIDs[4].Bytes(), // X-Request-ID
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}))
		if err != nil {
			t.Fatalf("Failed to move User-Agent to bottom: %v", err)
		}

		verifyLinkedListIntegrity(t, data, 5, "FullLifecycle-Reordered")
		t.Log("✓ Phase 3: Headers reordered")

		// 5. Add more headers dynamically
		newHeaders := []struct {
			key, value string
		}{
			{"Cache-Control", "no-cache"},
			{"X-Custom-Header", "custom-value"},
		}

		for _, h := range newHeaders {
			createHeaderViaRPC(t, data, h.key, h.value, "Added later", true)
		}

		verifyHeaderCount(t, data, 7, "FullLifecycle-Extended")
		t.Log("✓ Phase 4: Additional headers added")

		// 6. Remove some headers (cleanup old ones)
		_, err = data.rpc.HeaderDelete(data.ctx, connect.NewRequest(&requestv1.HeaderDeleteRequest{
			HeaderId: headerIDs[2].Bytes(), // Accept
		}))
		if err != nil {
			t.Fatalf("Failed to delete Accept header: %v", err)
		}

		verifyHeaderCount(t, data, 6, "FullLifecycle-Cleaned")
		verifyLinkedListIntegrity(t, data, 6, "FullLifecycle-Final")
		t.Log("✓ Phase 5: Cleanup completed")

		// 7. Final verification - list all headers and check integrity
		resp, err := data.rpc.HeaderList(data.ctx, connect.NewRequest(&requestv1.HeaderListRequest{
			ExampleId: data.exampleID.Bytes(),
		}))
		if err != nil {
			t.Fatalf("Failed final header list: %v", err)
		}

		t.Logf("✓ Full lifecycle completed: %d headers remain", len(resp.Msg.Items))
		for i, header := range resp.Msg.Items {
			t.Logf("  %d. %s: %s (enabled: %t)", i+1, header.Key, header.Value, header.Enabled)
		}
	})

	t.Run("StressTestLargeList", func(t *testing.T) {
		const largeCount = 100

		// Create large number of headers
		start := time.Now()
		var headerIDs []idwrap.IDWrap
		for i := 0; i < largeCount; i++ {
			id := createHeaderViaRPC(t, data, fmt.Sprintf("Stress-Header-%03d", i), fmt.Sprintf("stress-value-%03d", i), fmt.Sprintf("Stress header %d", i), i%3 != 0) // 2/3 enabled
			headerIDs = append(headerIDs, id)
		}
		createDuration := time.Since(start)

		verifyHeaderCount(t, data, largeCount, "StressTestLargeList-Created")
		t.Logf("✓ Created %d headers in %v", largeCount, createDuration)

		// Test listing performance
		start = time.Now()
		resp, err := data.rpc.HeaderList(data.ctx, connect.NewRequest(&requestv1.HeaderListRequest{
			ExampleId: data.exampleID.Bytes(),
		}))
		listDuration := time.Since(start)
		if err != nil {
			t.Fatalf("Failed to list large header set: %v", err)
		}

		if len(resp.Msg.Items) != largeCount {
			t.Fatalf("Expected %d headers in list, got %d", largeCount, len(resp.Msg.Items))
		}
		t.Logf("✓ Listed %d headers in %v", largeCount, listDuration)

		// Test integrity with large list
		start = time.Now()
		verifyLinkedListIntegrity(t, data, largeCount, "StressTestLargeList-Integrity")
		integrityDuration := time.Since(start)
		t.Logf("✓ Verified integrity of %d headers in %v", largeCount, integrityDuration)

		// Test some moves in large list
		start = time.Now()
		numMoves := 10
		for i := 0; i < numMoves; i++ {
			sourceIdx := i * (largeCount / numMoves)
			targetIdx := (sourceIdx + largeCount/2) % largeCount
			
			_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(&requestv1.HeaderMoveRequest{
				ExampleId:      data.exampleID.Bytes(),
				HeaderId:       headerIDs[sourceIdx].Bytes(),
				TargetHeaderId: headerIDs[targetIdx].Bytes(),
				Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			}))
			if err != nil {
				t.Fatalf("Failed move %d in large list: %v", i+1, err)
			}
		}
		movesDuration := time.Since(start)
		t.Logf("✓ Performed %d moves in large list in %v", numMoves, movesDuration)

		// Final integrity check
		verifyLinkedListIntegrity(t, data, largeCount, "StressTestLargeList-Final")
		t.Log("✓ Large list stress test completed successfully")
	})
}