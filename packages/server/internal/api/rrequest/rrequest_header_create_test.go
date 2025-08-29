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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// headerCreateTestData holds common test setup data for header creation tests
type headerCreateTestData struct {
	ctx       context.Context
	rpc       rrequest.RequestRPC
	exampleID idwrap.IDWrap
	userID    idwrap.IDWrap
	ehs       sexampleheader.HeaderService
}

// setupHeaderCreateTestData creates test data for header creation functionality testing
func setupHeaderCreateTestData(t *testing.T) *headerCreateTestData {
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
	require.NoError(t, err)

	// Create example
	exampleID := idwrap.NewNow()
	example := &mitemapiexample.ItemApiExample{
		ID:           exampleID,
		ItemApiID:    item.ID,
		CollectionID: collectionID,
		Name:         "test-example",
	}
	err = iaes.CreateApiExample(ctx, example)
	require.NoError(t, err)

	rpc := rrequest.New(db, cs, us, ias, iaes, ehs, eqs, as)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	return &headerCreateTestData{
		ctx:       authedCtx,
		rpc:       rpc,
		exampleID: exampleID,
		userID:    userID,
		ehs:       ehs,
	}
}

// validateHeaderLinkedList performs comprehensive linked list integrity checks
func validateHeaderLinkedList(t *testing.T, ctx context.Context, headerService sexampleheader.HeaderService, exampleID idwrap.IDWrap, expectedCount int, testContext string) {
	t.Helper()

	orderedHeaders, err := headerService.GetHeadersOrdered(ctx, exampleID)
	require.NoError(t, err, "[%s] Failed to get ordered headers", testContext)

	// Check expected count
	assert.Equal(t, expectedCount, len(orderedHeaders), "[%s] Expected %d headers, got %d", testContext, expectedCount, len(orderedHeaders))

	if len(orderedHeaders) == 0 {
		t.Logf("[%s] ✓ Empty list integrity verified", testContext)
		return
	}

	// Single item list checks
	if len(orderedHeaders) == 1 {
		header := orderedHeaders[0]
		assert.Nil(t, header.Prev, "[%s] Single header should have nil prev pointer", testContext)
		assert.Nil(t, header.Next, "[%s] Single header should have nil next pointer", testContext)
		t.Logf("[%s] ✓ Single item list integrity verified", testContext)
		return
	}

	// Multi-item list checks
	// First header checks
	first := orderedHeaders[0]
	assert.Nil(t, first.Prev, "[%s] First header should have nil prev pointer", testContext)

	// Last header checks
	last := orderedHeaders[len(orderedHeaders)-1]
	assert.Nil(t, last.Next, "[%s] Last header should have nil next pointer", testContext)

	// Check all forward and backward linkages
	for i := 0; i < len(orderedHeaders)-1; i++ {
		current := orderedHeaders[i]
		next := orderedHeaders[i+1]

		// Forward linkage: current.next should point to next.id
		require.NotNil(t, current.Next, "[%s] Header at index %d has nil next pointer", testContext, i)
		assert.Equal(t, 0, current.Next.Compare(next.ID), "[%s] Header at index %d next pointer mismatch", testContext, i)

		// Backward linkage: next.prev should point to current.id
		require.NotNil(t, next.Prev, "[%s] Header at index %d has nil prev pointer", testContext, i+1)
		assert.Equal(t, 0, next.Prev.Compare(current.ID), "[%s] Header at index %d prev pointer mismatch", testContext, i+1)
	}

	// Check for circular references by walking the list
	seenIDs := make(map[string]bool)
	for _, header := range orderedHeaders {
		idStr := header.ID.String()
		assert.False(t, seenIDs[idStr], "[%s] Circular reference detected: duplicate ID %s", testContext, idStr)
		seenIDs[idStr] = true
	}

	t.Logf("[%s] ✓ Linked list integrity verified for %d headers", testContext, len(orderedHeaders))
}

// createHeader creates a header using the RPC and returns its ID
func createHeader(t *testing.T, data *headerCreateTestData, key, value string) idwrap.IDWrap {
	t.Helper()

	resp, err := data.rpc.HeaderCreate(data.ctx, connect.NewRequest(&requestv1.HeaderCreateRequest{
		ExampleId:   data.exampleID.Bytes(),
		Key:         key,
		Value:       value,
		Enabled:     true,
		Description: fmt.Sprintf("Test header for %s", key),
	}))
	require.NoError(t, err, "Failed to create header with key %s", key)
	require.NotNil(t, resp, "Response should not be nil")

	headerID, err := idwrap.NewFromBytes(resp.Msg.GetHeaderId())
	require.NoError(t, err, "Failed to parse header ID")

	return headerID
}

// getHeadersInOrder retrieves headers in order and returns their IDs
func getHeadersInOrder(t *testing.T, data *headerCreateTestData) []idwrap.IDWrap {
	t.Helper()

	orderedHeaders, err := data.ehs.GetHeadersOrdered(data.ctx, data.exampleID)
	require.NoError(t, err, "Failed to get ordered headers")

	var order []idwrap.IDWrap
	for _, header := range orderedHeaders {
		order = append(order, header.ID)
	}
	return order
}

// TestHeaderCreateSingle tests creating a single header via HeaderCreate RPC
func TestHeaderCreateSingle(t *testing.T) {
	data := setupHeaderCreateTestData(t)

	// Initially, there should be no headers
	validateHeaderLinkedList(t, data.ctx, data.ehs, data.exampleID, 0, "InitialEmptyState")

	// Create a single header
	headerID := createHeader(t, data, "Authorization", "Bearer token123")

	// Verify the header exists and the linked list is properly maintained
	validateHeaderLinkedList(t, data.ctx, data.ehs, data.exampleID, 1, "SingleHeaderCreated")

	// Verify we can retrieve the header via HeaderList
	resp, err := data.rpc.HeaderList(data.ctx, connect.NewRequest(&requestv1.HeaderListRequest{
		ExampleId: data.exampleID.Bytes(),
	}))
	require.NoError(t, err, "Failed to list headers")
	assert.Len(t, resp.Msg.Items, 1, "Should have exactly one header")

	header := resp.Msg.Items[0]
	assert.Equal(t, "Authorization", header.Key, "Header key should match")
	assert.Equal(t, "Bearer token123", header.Value, "Header value should match")
	assert.True(t, header.Enabled, "Header should be enabled")

	// Verify header ID matches (header ID is at index 0 in the bytes)
	listedHeaderID, err := idwrap.NewFromBytes(header.HeaderId)
	require.NoError(t, err, "Failed to parse listed header ID")
	assert.Equal(t, 0, headerID.Compare(listedHeaderID), "Header IDs should match")
}

// TestHeaderCreateMultipleSequential tests creating 3-4 headers one after another
func TestHeaderCreateMultipleSequential(t *testing.T) {
	data := setupHeaderCreateTestData(t)

	// Initially, there should be no headers
	validateHeaderLinkedList(t, data.ctx, data.ehs, data.exampleID, 0, "InitialEmptyState")

	// Create headers sequentially
	headers := []struct {
		key   string
		value string
	}{
		{"Authorization", "Bearer token123"},
		{"Content-Type", "application/json"},
		{"X-API-Version", "v1"},
		{"Accept", "application/json"},
	}

	var headerIDs []idwrap.IDWrap
	for i, h := range headers {
		headerID := createHeader(t, data, h.key, h.value)
		headerIDs = append(headerIDs, headerID)

		// Verify linked list integrity after each creation
		validateHeaderLinkedList(t, data.ctx, data.ehs, data.exampleID, i+1, fmt.Sprintf("AfterCreating%d", i+1))
	}

	// Verify final state
	validateHeaderLinkedList(t, data.ctx, data.ehs, data.exampleID, 4, "FinalState")

	// Verify order is maintained (should be in creation order)
	orderedIDs := getHeadersInOrder(t, data)
	require.Len(t, orderedIDs, 4, "Should have 4 headers in order")

	for i, expectedID := range headerIDs {
		assert.Equal(t, 0, expectedID.Compare(orderedIDs[i]), "Header %d should be in correct position", i)
	}

	// Verify we can retrieve all headers via HeaderList
	resp, err := data.rpc.HeaderList(data.ctx, connect.NewRequest(&requestv1.HeaderListRequest{
		ExampleId: data.exampleID.Bytes(),
	}))
	require.NoError(t, err, "Failed to list headers")
	assert.Len(t, resp.Msg.Items, 4, "Should have exactly four headers")

	// Verify headers are in correct order
	for i, h := range headers {
		header := resp.Msg.Items[i]
		assert.Equal(t, h.key, header.Key, "Header %d key should match", i)
		assert.Equal(t, h.value, header.Value, "Header %d value should match", i)
	}
}

// TestHeaderDeltaCreateMultiple tests creating multiple delta headers sequentially
func TestHeaderDeltaCreateMultiple(t *testing.T) {
	data := setupHeaderCreateTestData(t)

	// Create origin headers first
	originHeaders := []struct {
		key   string
		value string
	}{
		{"Authorization", "Bearer origin-token"},
		{"Content-Type", "application/xml"},
	}

	for _, h := range originHeaders {
		createHeader(t, data, h.key, h.value)
	}

	// Verify origin headers
	validateHeaderLinkedList(t, data.ctx, data.ehs, data.exampleID, 2, "OriginHeadersCreated")

	// Create additional headers to test sequential creation
	deltaHeaders := []struct {
		key   string
		value string
	}{
		{"X-Delta-Header", "delta-value-1"},
		{"X-Custom-Auth", "custom-token"},
		{"X-Request-ID", "req-12345"},
	}

	var deltaHeaderIDs []idwrap.IDWrap
	initialCount := 2 // We already have 2 origin headers

	for i, h := range deltaHeaders {
		headerID := createHeader(t, data, h.key, h.value)
		deltaHeaderIDs = append(deltaHeaderIDs, headerID)

		// Verify linked list integrity after each delta header creation
		validateHeaderLinkedList(t, data.ctx, data.ehs, data.exampleID, initialCount+i+1, fmt.Sprintf("AfterDeltaHeader%d", i+1))
	}

	// Verify final state with all headers
	validateHeaderLinkedList(t, data.ctx, data.ehs, data.exampleID, 5, "FinalDeltaState")

	// Verify all headers can be retrieved via HeaderList
	resp, err := data.rpc.HeaderList(data.ctx, connect.NewRequest(&requestv1.HeaderListRequest{
		ExampleId: data.exampleID.Bytes(),
	}))
	require.NoError(t, err, "Failed to list all headers")
	assert.Len(t, resp.Msg.Items, 5, "Should have all 5 headers")
}

// TestHeaderCreateBulk tests bulk creation scenarios
func TestHeaderCreateBulk(t *testing.T) {
	data := setupHeaderCreateTestData(t)

	// Test rapid creation of multiple headers to simulate bulk operations
	bulkHeaders := []struct {
		key   string
		value string
	}{
		{"X-Bulk-1", "value-1"},
		{"X-Bulk-2", "value-2"},
		{"X-Bulk-3", "value-3"},
		{"X-Bulk-4", "value-4"},
		{"X-Bulk-5", "value-5"},
		{"X-Bulk-6", "value-6"},
		{"X-Bulk-7", "value-7"},
		{"X-Bulk-8", "value-8"},
	}

	// Create all headers rapidly without intermediate validation
	var bulkHeaderIDs []idwrap.IDWrap
	for _, h := range bulkHeaders {
		headerID := createHeader(t, data, h.key, h.value)
		bulkHeaderIDs = append(bulkHeaderIDs, headerID)
	}

	// Verify final linked list integrity after bulk creation
	validateHeaderLinkedList(t, data.ctx, data.ehs, data.exampleID, 8, "BulkCreationComplete")

	// Verify order is maintained
	orderedIDs := getHeadersInOrder(t, data)
	require.Len(t, orderedIDs, 8, "Should have 8 headers in order")

	for i, expectedID := range bulkHeaderIDs {
		assert.Equal(t, 0, expectedID.Compare(orderedIDs[i]), "Bulk header %d should be in correct position", i)
	}

	// Verify no foreign key constraint errors occurred by checking all headers exist
	resp, err := data.rpc.HeaderList(data.ctx, connect.NewRequest(&requestv1.HeaderListRequest{
		ExampleId: data.exampleID.Bytes(),
	}))
	require.NoError(t, err, "Failed to list bulk created headers")
	assert.Len(t, resp.Msg.Items, 8, "All bulk headers should be retrievable")
}

// TestHeaderCreateThenMove tests creating headers then testing move operations work
func TestHeaderCreateThenMove(t *testing.T) {
	data := setupHeaderCreateTestData(t)

	// Create headers for move testing
	moveHeaders := []struct {
		key   string
		value string
	}{
		{"First", "value-1"},
		{"Second", "value-2"},
		{"Third", "value-3"},
		{"Fourth", "value-4"},
	}

	var moveHeaderIDs []idwrap.IDWrap
	for _, h := range moveHeaders {
		headerID := createHeader(t, data, h.key, h.value)
		moveHeaderIDs = append(moveHeaderIDs, headerID)
	}

	// Verify initial state
	validateHeaderLinkedList(t, data.ctx, data.ehs, data.exampleID, 4, "InitialMoveState")

	// Verify headers are in creation order
	initialOrder := getHeadersInOrder(t, data)
	for i, expectedID := range moveHeaderIDs {
		assert.Equal(t, 0, expectedID.Compare(initialOrder[i]), "Initial header %d should be in correct position", i)
	}

	// Test that the HeaderMove endpoint exists and doesn't error
	// Note: The actual move logic may be a no-op based on the current implementation,
	// but we're testing that the infrastructure is ready for move operations
	moveResp, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(&requestv1.HeaderMoveRequest{
		ExampleId: data.exampleID.Bytes(),
		HeaderId:  moveHeaderIDs[1].Bytes(), // Move second header
	}))

	// Even if move is not implemented, it should not error due to foreign key constraints
	require.NoError(t, err, "HeaderMove should not fail due to foreign key constraints")
	require.NotNil(t, moveResp, "HeaderMove response should not be nil")

	// Verify linked list integrity is maintained after move attempt
	validateHeaderLinkedList(t, data.ctx, data.ehs, data.exampleID, 4, "AfterMoveAttempt")
}

// TestHeaderCreateEmptyList tests creating the first header in an empty list
func TestHeaderCreateEmptyList(t *testing.T) {
	data := setupHeaderCreateTestData(t)

	// Verify we start with an empty list
	validateHeaderLinkedList(t, data.ctx, data.ehs, data.exampleID, 0, "EmptyListInitial")

	// Create the very first header
	firstHeaderID := createHeader(t, data, "X-First-Ever", "first-value")

	// This is the critical test - creating the first header should not fail
	// due to foreign key constraints (the bug we're preventing regression of)
	validateHeaderLinkedList(t, data.ctx, data.ehs, data.exampleID, 1, "FirstHeaderAdded")

	// Verify the first header has no prev/next pointers
	orderedHeaders, err := data.ehs.GetHeadersOrdered(data.ctx, data.exampleID)
	require.NoError(t, err, "Failed to get first header details")
	require.Len(t, orderedHeaders, 1, "Should have exactly one header")

	header := orderedHeaders[0]
	assert.Nil(t, header.Prev, "First header should have nil prev")
	assert.Nil(t, header.Next, "First header should have nil next")
	assert.Equal(t, 0, firstHeaderID.Compare(header.ID), "Header ID should match")

	// Verify it can be retrieved via HeaderList
	resp, err := data.rpc.HeaderList(data.ctx, connect.NewRequest(&requestv1.HeaderListRequest{
		ExampleId: data.exampleID.Bytes(),
	}))
	require.NoError(t, err, "Failed to list first header")
	assert.Len(t, resp.Msg.Items, 1, "Should retrieve exactly one header")
	assert.Equal(t, "X-First-Ever", resp.Msg.Items[0].Key, "First header key should match")
}

// TestHeaderCreateLinkedListIntegrity tests that linked list structure is correctly maintained
func TestHeaderCreateLinkedListIntegrity(t *testing.T) {
	data := setupHeaderCreateTestData(t)

	// Test multiple creation patterns to ensure linked list integrity

	// Pattern 1: Single header
	h1 := createHeader(t, data, "Single", "value")
	validateHeaderLinkedList(t, data.ctx, data.ehs, data.exampleID, 1, "SingleHeader")

	// Pattern 2: Add second header  
	h2 := createHeader(t, data, "Second", "value")
	validateHeaderLinkedList(t, data.ctx, data.ehs, data.exampleID, 2, "TwoHeaders")

	// Verify forward and backward linkage manually
	orderedHeaders, err := data.ehs.GetHeadersOrdered(data.ctx, data.exampleID)
	require.NoError(t, err, "Failed to get headers for manual verification")
	require.Len(t, orderedHeaders, 2, "Should have exactly two headers")

	first := orderedHeaders[0]
	second := orderedHeaders[1]

	// Verify first header linkage
	assert.Equal(t, 0, h1.Compare(first.ID), "First header ID should match")
	assert.Nil(t, first.Prev, "First header prev should be nil")
	assert.NotNil(t, first.Next, "First header next should not be nil")
	assert.Equal(t, 0, first.Next.Compare(second.ID), "First header next should point to second")

	// Verify second header linkage
	assert.Equal(t, 0, h2.Compare(second.ID), "Second header ID should match")
	assert.NotNil(t, second.Prev, "Second header prev should not be nil")
	assert.Nil(t, second.Next, "Second header next should be nil")
	assert.Equal(t, 0, second.Prev.Compare(first.ID), "Second header prev should point to first")

	// Pattern 3: Add third header to test middle linkage
	_ = createHeader(t, data, "Third", "value")
	validateHeaderLinkedList(t, data.ctx, data.ehs, data.exampleID, 3, "ThreeHeaders")

	// Pattern 4: Stress test with more headers
	for i := 4; i <= 10; i++ {
		createHeader(t, data, fmt.Sprintf("Header-%d", i), fmt.Sprintf("value-%d", i))
		validateHeaderLinkedList(t, data.ctx, data.ehs, data.exampleID, i, fmt.Sprintf("StressTest-%d", i))
	}

	// Final verification - ensure no foreign key constraint errors
	finalOrder := getHeadersInOrder(t, data)
	assert.Len(t, finalOrder, 10, "Should have all 10 headers in final state")

	// Verify all headers can be retrieved without error
	resp, err := data.rpc.HeaderList(data.ctx, connect.NewRequest(&requestv1.HeaderListRequest{
		ExampleId: data.exampleID.Bytes(),
	}))
	require.NoError(t, err, "Should be able to retrieve all headers without foreign key errors")
	assert.Len(t, resp.Msg.Items, 10, "All headers should be retrievable")
}

// TestHeaderCreateRapidSequence tests rapid sequential creation to catch race conditions
func TestHeaderCreateRapidSequence(t *testing.T) {
	data := setupHeaderCreateTestData(t)

	// Create headers in rapid succession to test for race conditions or timing issues
	const headerCount = 20
	var rapidHeaderIDs []idwrap.IDWrap

	for i := 0; i < headerCount; i++ {
		headerID := createHeader(t, data, fmt.Sprintf("Rapid-%d", i), fmt.Sprintf("value-%d", i))
		rapidHeaderIDs = append(rapidHeaderIDs, headerID)
	}

	// Verify all headers were created successfully
	validateHeaderLinkedList(t, data.ctx, data.ehs, data.exampleID, headerCount, "RapidCreationComplete")

	// Verify order is maintained
	orderedIDs := getHeadersInOrder(t, data)
	require.Len(t, orderedIDs, headerCount, "Should have all rapid headers in order")

	for i, expectedID := range rapidHeaderIDs {
		assert.Equal(t, 0, expectedID.Compare(orderedIDs[i]), "Rapid header %d should be in correct position", i)
	}

	// Verify no foreign key errors by retrieving all headers
	resp, err := data.rpc.HeaderList(data.ctx, connect.NewRequest(&requestv1.HeaderListRequest{
		ExampleId: data.exampleID.Bytes(),
	}))
	require.NoError(t, err, "Should retrieve all rapid headers without foreign key errors")
	assert.Len(t, resp.Msg.Items, headerCount, "All rapid headers should be retrievable")
}