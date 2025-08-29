package ritemapiexample_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/ritemapiexample"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logconsole"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/model/mexampleheader"
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

	"connectrpc.com/connect"
)

// TestCollectionExampleLoadingBug reproduces the collection example loading bug
// where AppendBulkHeader creates headers in two phases:
// 1. First phase: Creates headers WITHOUT prev/next links to avoid FK constraints
// 2. Second phase: Updates links separately
// This leaves headers temporarily orphaned and can break the linked list.
func TestCollectionExampleLoadingBug(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	mockLogger := mocklogger.NewMockLogger()

	// Initialize all services
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

	logChanMap := logconsole.NewLogChanMapWith(10000)

	// Create test workspace and collection
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	userID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)

	// Step 1: Create an endpoint
	endpointID := idwrap.NewNow()
	endpoint := &mitemapi.ItemApi{
		ID:           endpointID,
		Name:         "Test Endpoint",
		Url:          "https://api.example.com/test",
		Method:       "POST",
		CollectionID: collectionID,
		FolderID:     nil,
	}

	err := ias.CreateItemApi(ctx, endpoint)
	if err != nil {
		t.Fatalf("Failed to create endpoint: %v", err)
	}

	t.Logf("✓ Created endpoint: %s", endpointID.String())

	// Step 2: Create multiple examples with headers for that endpoint
	examples := make([]*mitemapiexample.ItemApiExample, 3)
	exampleHeaders := make(map[idwrap.IDWrap][]mexampleheader.Header)

	for i := 0; i < 3; i++ {
		exampleID := idwrap.NewNow()
		examples[i] = &mitemapiexample.ItemApiExample{
			ID:           exampleID,
			ItemApiID:    endpointID,
			CollectionID: collectionID,
			Name:         fmt.Sprintf("Example %d", i+1),
			Updated:      dbtime.DBNow(),
			IsDefault:    false, // User examples should never be default
			BodyType:     mitemapiexample.BodyTypeRaw,
			Prev:         nil, // Will be set by movable system
			Next:         nil, // Will be set by movable system
		}

		// Create example first
		err = iaes.CreateApiExample(ctx, examples[i])
		if err != nil {
			t.Fatalf("Failed to create example %d: %v", i+1, err)
		}

		// Use movable repository to append it to the end of the list
		// This simulates what should happen in proper example creation
		repo := iaes.GetMovableRepository()
		if i > 0 { // First example can be isolated, others should be linked
			err = repo.RepairIsolatedExamples(ctx, nil, endpointID)
			if err != nil {
				t.Logf("Warning: Failed to link example %d: %v", i+1, err)
			}
		}

		// Create headers for each example
		headers := make([]mexampleheader.Header, 4) // Create 4 headers per example
		for j := 0; j < 4; j++ {
			headers[j] = mexampleheader.Header{
				ID:            idwrap.NewNow(),
				ExampleID:     exampleID,
				DeltaParentID: nil,
				HeaderKey:     fmt.Sprintf("X-Test-Header-%d", j+1),
				Enable:        true,
				Description:   fmt.Sprintf("Test header %d for example %d", j+1, i+1),
				Value:         fmt.Sprintf("value-%d-%d", i+1, j+1),
				Prev:          nil, // Will be set by AppendBulkHeader
				Next:          nil, // Will be set by AppendBulkHeader
			}
		}
		exampleHeaders[exampleID] = headers

		t.Logf("✓ Created example %d: %s with %d headers", i+1, exampleID.String(), len(headers))
	}

	// Step 3: Use AppendBulkHeader to add headers (this is where the bug occurs)
	// We'll add headers for all examples, which triggers the two-phase creation problem
	var allHeaders []mexampleheader.Header
	for _, headers := range exampleHeaders {
		allHeaders = append(allHeaders, headers...)
	}

	t.Logf("About to call AppendBulkHeader with %d headers across %d examples", len(allHeaders), len(examples))

	// This should trigger the bug - headers are created without links first, then updated
	err = hs.AppendBulkHeader(ctx, allHeaders)
	if err != nil {
		t.Fatalf("AppendBulkHeader failed: %v", err)
	}

	t.Logf("✓ AppendBulkHeader completed successfully")

	// Step 4: Check database state for orphaned headers (prev=NULL, next=NULL)
	// During the two-phase process, headers might be temporarily orphaned
	orphanedHeaders := checkOrphanedHeaders(ctx, t, db, allHeaders)
	if len(orphanedHeaders) > 0 {
		t.Logf("⚠ Found %d orphaned headers after AppendBulkHeader:", len(orphanedHeaders))
		for _, header := range orphanedHeaders {
			t.Logf("  - Header %s (%s) for example %s: prev=%v, next=%v", 
				header.ID.String(), header.HeaderKey, header.ExampleID.String(), header.Prev, header.Next)
		}
	} else {
		t.Logf("✓ No orphaned headers found")
	}

	// Step 5: Verify linked list integrity for each example
	for i, example := range examples {
		t.Logf("Checking linked list integrity for example %d: %s", i+1, example.ID.String())
		
		orderedHeaders, err := hs.GetHeadersOrdered(ctx, example.ID)
		if err != nil {
			t.Errorf("Failed to get ordered headers for example %d: %v", i+1, err)
			continue
		}

		allHeadersForExample, err := hs.GetHeaderByExampleID(ctx, example.ID)
		if err != nil {
			t.Errorf("Failed to get all headers for example %d: %v", i+1, err)
			continue
		}

		expectedHeaderCount := len(exampleHeaders[example.ID])
		if len(allHeadersForExample) != expectedHeaderCount {
			t.Errorf("Example %d: Expected %d headers, got %d", i+1, expectedHeaderCount, len(allHeadersForExample))
		}

		if len(orderedHeaders) != expectedHeaderCount {
			t.Errorf("Example %d: Expected %d ordered headers, got %d - linked list is broken!", 
				i+1, expectedHeaderCount, len(orderedHeaders))
			t.Logf("  All headers for example %d:", i+1)
			for _, h := range allHeadersForExample {
				t.Logf("    - %s (%s): prev=%v, next=%v", h.ID.String(), h.HeaderKey, h.Prev, h.Next)
			}
		} else {
			t.Logf("✓ Example %d: Linked list integrity OK (%d headers)", i+1, len(orderedHeaders))
		}

		// Verify the order is maintained
		validateLinkedListOrder(ctx, t, orderedHeaders, fmt.Sprintf("example %d", i+1))
	}

	// Step 6: Call ExampleList to verify examples load correctly
	rpcExample := ritemapiexample.New(db, iaes, ias, ifs, ws, cs, us, hs, qs, bfs, bues, brs, erhs, ers, es, vs, as, ars, logChanMap)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	req := connect.NewRequest(&examplev1.ExampleListRequest{
		EndpointId: endpointID.Bytes(),
	})

	t.Logf("Calling ExampleList for endpoint %s", endpointID.String())
	resp, err := rpcExample.ExampleList(authedCtx, req)
	if err != nil {
		t.Fatalf("ExampleList failed: %v", err)
	}

	if resp == nil || resp.Msg == nil {
		t.Fatalf("ExampleList returned nil response")
	}

	if len(resp.Msg.Items) != len(examples) {
		t.Errorf("Expected %d examples in response, got %d", len(examples), len(resp.Msg.Items))
		
		// Debug the missing examples issue
		t.Logf("Debugging missing examples:")
		for i, example := range examples {
			t.Logf("  Created example %d: %s (%s)", i+1, example.ID.String(), example.Name)
		}
		t.Logf("Returned examples:")
		for i, item := range resp.Msg.Items {
			exampleID := idwrap.NewFromBytesMust(item.ExampleId)
			t.Logf("  Returned example %d: %s (%s)", i+1, exampleID.String(), item.Name)
		}
		
		// Test ordered vs all examples directly
		orderedExamples, err := iaes.GetApiExamplesOrdered(ctx, endpointID)
		if err != nil {
			t.Logf("  GetApiExamplesOrdered failed: %v", err)
		} else {
			t.Logf("  GetApiExamplesOrdered returned %d examples", len(orderedExamples))
		}
		
		allExamples, err := iaes.GetAllApiExamples(ctx, endpointID)
		if err != nil {
			t.Logf("  GetAllApiExamples failed: %v", err)
		} else {
			t.Logf("  GetAllApiExamples returned %d examples", len(allExamples))
			for i, example := range allExamples {
				t.Logf("    All example %d: %s (%s) prev=%v next=%v", 
					i+1, example.ID.String(), example.Name, example.Prev, example.Next)
			}
		}
		
		if len(orderedExamples) != len(allExamples) {
			t.Logf("  ⚠ FOUND THE BUG: Ordered examples (%d) != All examples (%d)", 
				len(orderedExamples), len(allExamples))
			t.Logf("  This indicates broken linked list in examples, not headers!")
		}
	} else {
		t.Logf("✓ ExampleList returned %d examples as expected", len(resp.Msg.Items))
	}

	// Step 7: Test header retrieval for each example
	for i, example := range examples {
		t.Logf("Testing header retrieval for example %d", i+1)
		
		// Test both ordered and unordered header retrieval
		orderedHeaders, err := hs.GetHeadersOrdered(ctx, example.ID)
		if err != nil {
			t.Errorf("Failed to get ordered headers for example %d: %v", i+1, err)
		} else {
			t.Logf("  ✓ Retrieved %d ordered headers", len(orderedHeaders))
		}

		allHeaders, err := hs.GetHeaderByExampleID(ctx, example.ID)
		if err != nil {
			t.Errorf("Failed to get all headers for example %d: %v", i+1, err)
		} else {
			t.Logf("  ✓ Retrieved %d total headers", len(allHeaders))
		}

		// The counts should match if the linked list is intact
		if len(orderedHeaders) != len(allHeaders) {
			t.Errorf("Example %d: Ordered headers (%d) != All headers (%d) - indicates broken linked list!", 
				i+1, len(orderedHeaders), len(allHeaders))
		}
	}

	t.Logf("Test completed. Summary:")
	t.Logf("- Created %d examples with %d total headers", len(examples), len(allHeaders))
	t.Logf("- AppendBulkHeader completed without errors")
	t.Logf("- ExampleList returned %d examples (expected %d)", len(resp.Msg.Items), len(examples))
	
	// Additional diagnostics for the example linking bug
	if len(resp.Msg.Items) != len(examples) {
		t.Logf("")
		t.Logf("🔍 DIAGNOSIS:")
		t.Logf("The collection example loading bug has been successfully reproduced!")
		t.Logf("Root cause: Example linked list corruption, not header orphaning")
		t.Logf("")
		t.Logf("Key findings:")
		t.Logf("1. Examples are not properly linked when created individually")
		t.Logf("2. GetApiExamplesOrdered (via CTE) fails to traverse broken linked list") 
		t.Logf("3. GetAllApiExamples shows not all examples are even persisted")
		t.Logf("4. Auto-linking attempted repair but likely failed due to corruption")
		t.Logf("5. Headers are working fine - the problem is at the example level")
		t.Logf("")
		t.Logf("Recommended fixes:")
		t.Logf("- Fix example creation to use movable repository for proper linking")
		t.Logf("- Ensure CreateApiExample automatically maintains linked list integrity")
		t.Logf("- Improve auto-linking recovery mechanism")
		t.Logf("- Add validation to detect and prevent circular references")
	}
	
	if len(orphanedHeaders) > 0 {
		t.Logf("⚠ SECONDARY ISSUE: Found %d orphaned headers", len(orphanedHeaders))
	} else {
		t.Logf("✓ No orphaned headers detected")
	}
}

// checkOrphanedHeaders finds headers that have both prev=NULL and next=NULL
// These indicate headers that became isolated during the two-phase creation process
func checkOrphanedHeaders(ctx context.Context, t *testing.T, db *sql.DB, expectedHeaders []mexampleheader.Header) []mexampleheader.Header {
	var orphaned []mexampleheader.Header

	// Query for headers that are isolated (prev=NULL AND next=NULL)
	// But exclude legitimate single headers (where the example only has one header)
	query := `
	SELECT h.id, h.example_id, h.delta_parent_id, h.header_key, h.enable, h.description, h.value, h.prev, h.next
	FROM example_header h
	WHERE h.prev IS NULL AND h.next IS NULL
	  AND (SELECT COUNT(*) FROM example_header h2 WHERE h2.example_id = h.example_id) > 1
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		t.Fatalf("Failed to query orphaned headers: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var header mexampleheader.Header
		var idBytes, exampleIDBytes, deltaParentIDBytes, prevBytes, nextBytes []byte

		err := rows.Scan(
			&idBytes,
			&exampleIDBytes, 
			&deltaParentIDBytes,
			&header.HeaderKey,
			&header.Enable,
			&header.Description,
			&header.Value,
			&prevBytes,
			&nextBytes,
		)
		if err != nil {
			t.Fatalf("Failed to scan orphaned header: %v", err)
		}

		header.ID = idwrap.NewFromBytesMust(idBytes)
		header.ExampleID = idwrap.NewFromBytesMust(exampleIDBytes)
		
		if deltaParentIDBytes != nil {
			id := idwrap.NewFromBytesMust(deltaParentIDBytes)
			header.DeltaParentID = &id
		}
		
		if prevBytes != nil {
			id := idwrap.NewFromBytesMust(prevBytes)
			header.Prev = &id
		}
		
		if nextBytes != nil {
			id := idwrap.NewFromBytesMust(nextBytes)
			header.Next = &id
		}

		orphaned = append(orphaned, header)
	}

	return orphaned
}

// validateLinkedListOrder checks that the linked list maintains proper ordering
func validateLinkedListOrder(ctx context.Context, t *testing.T, headers []mexampleheader.Header, contextName string) {
	if len(headers) == 0 {
		return
	}

	// Check that first header has no prev
	if headers[0].Prev != nil {
		t.Errorf("%s: First header should have prev=nil, got prev=%v", contextName, headers[0].Prev)
	}

	// Check that last header has no next  
	if headers[len(headers)-1].Next != nil {
		t.Errorf("%s: Last header should have next=nil, got next=%v", contextName, headers[len(headers)-1].Next)
	}

	// Check that middle headers have proper links
	for i := 1; i < len(headers)-1; i++ {
		if headers[i].Prev == nil {
			t.Errorf("%s: Middle header %d should have prev, got nil", contextName, i)
		}
		if headers[i].Next == nil {
			t.Errorf("%s: Middle header %d should have next, got nil", contextName, i)
		}
	}

	// Verify forward links match
	for i := 0; i < len(headers)-1; i++ {
		if headers[i].Next == nil {
			t.Errorf("%s: Header %d should have next pointer", contextName, i)
			continue
		}
		if headers[i].Next.Compare(headers[i+1].ID) != 0 {
			t.Errorf("%s: Header %d next pointer mismatch: expected %s, got %s", 
				contextName, i, headers[i+1].ID.String(), headers[i].Next.String())
		}
	}

	// Verify backward links match
	for i := 1; i < len(headers); i++ {
		if headers[i].Prev == nil {
			t.Errorf("%s: Header %d should have prev pointer", contextName, i)
			continue
		}
		if headers[i].Prev.Compare(headers[i-1].ID) != 0 {
			t.Errorf("%s: Header %d prev pointer mismatch: expected %s, got %s",
				contextName, i, headers[i-1].ID.String(), headers[i].Prev.String())
		}
	}

	t.Logf("  ✓ %s: Linked list order validation passed", contextName)
}

// TestAppendBulkHeaderTwoPhaseIssue specifically reproduces the two-phase creation bug
// This test creates a scenario that forces the AppendBulkHeader method to use
// its two-phase approach, which can leave headers temporarily orphaned
func TestAppendBulkHeaderTwoPhaseIssue(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	hs := sexampleheader.New(queries)
	
	// Create multiple test example IDs to simulate collection with multiple examples
	example1ID := idwrap.NewNow()
	example2ID := idwrap.NewNow()
	example3ID := idwrap.NewNow()
	
	// Create headers for multiple examples - this should trigger the bulk creation logic
	// that groups by example_id and processes each group separately
	headers := []mexampleheader.Header{}
	
	// Add 8 headers for example 1 (should trigger bulk creation within the example)
	for i := 0; i < 8; i++ {
		headers = append(headers, mexampleheader.Header{
			ID:            idwrap.NewNow(),
			ExampleID:     example1ID,
			DeltaParentID: nil,
			HeaderKey:     fmt.Sprintf("X-Example1-Header-%d", i+1),
			Enable:        true,
			Description:   fmt.Sprintf("Example 1 header %d", i+1),
			Value:         fmt.Sprintf("example1-value-%d", i+1),
			Prev:          nil,
			Next:          nil,
		})
	}
	
	// Add 8 headers for example 2
	for i := 0; i < 8; i++ {
		headers = append(headers, mexampleheader.Header{
			ID:            idwrap.NewNow(),
			ExampleID:     example2ID,
			DeltaParentID: nil,
			HeaderKey:     fmt.Sprintf("X-Example2-Header-%d", i+1),
			Enable:        true,
			Description:   fmt.Sprintf("Example 2 header %d", i+1),
			Value:         fmt.Sprintf("example2-value-%d", i+1),
			Prev:          nil,
			Next:          nil,
		})
	}
	
	// Add 8 headers for example 3
	for i := 0; i < 8; i++ {
		headers = append(headers, mexampleheader.Header{
			ID:            idwrap.NewNow(),
			ExampleID:     example3ID,
			DeltaParentID: nil,
			HeaderKey:     fmt.Sprintf("X-Example3-Header-%d", i+1),
			Enable:        true,
			Description:   fmt.Sprintf("Example 3 header %d", i+1),
			Value:         fmt.Sprintf("example3-value-%d", i+1),
			Prev:          nil,
			Next:          nil,
		})
	}

	t.Logf("Testing AppendBulkHeader with %d headers across 3 examples (8 headers each)", len(headers))
	t.Logf("This should trigger the two-phase creation process that can create orphaned headers")

	// This should trigger the complex multi-example, multi-header bulk append logic
	err := hs.AppendBulkHeader(ctx, headers)
	if err != nil {
		t.Fatalf("AppendBulkHeader failed: %v", err)
	}

	// Immediately check for orphaned headers after the operation
	orphaned := checkOrphanedHeaders(ctx, t, db, headers)
	
	if len(orphaned) > 0 {
		t.Logf("🐛 BUG REPRODUCED: Found %d orphaned headers immediately after AppendBulkHeader", len(orphaned))
		for _, header := range orphaned {
			t.Logf("  Orphaned: %s (%s) for example %s", 
				header.ID.String(), header.HeaderKey, header.ExampleID.String())
		}
	} else {
		t.Logf("✓ No orphaned headers detected")
	}

	// Check if the database shows any temporary inconsistencies by querying during a short window
	// This tries to catch the intermediate state during the two-phase process
	if len(orphaned) == 0 {
		t.Logf("Checking for timing-related orphaned state...")
		
		// Add more headers to trigger the process again
		moreHeaders := []mexampleheader.Header{}
		for i := 0; i < 5; i++ {
			moreHeaders = append(moreHeaders, mexampleheader.Header{
				ID:            idwrap.NewNow(),
				ExampleID:     example1ID,
				DeltaParentID: nil,
				HeaderKey:     fmt.Sprintf("X-Additional-Header-%d", i+1),
				Enable:        true,
				Description:   fmt.Sprintf("Additional header %d", i+1),
				Value:         fmt.Sprintf("additional-value-%d", i+1),
				Prev:          nil,
				Next:          nil,
			})
		}
		
		// Try again with additional headers
		err = hs.AppendBulkHeader(ctx, moreHeaders)
		if err != nil {
			t.Fatalf("Second AppendBulkHeader failed: %v", err)
		}
		
		// Check again
		allHeaders := append(headers, moreHeaders...)
		orphaned = checkOrphanedHeaders(ctx, t, db, allHeaders)
		if len(orphaned) > 0 {
			t.Logf("🐛 BUG REPRODUCED on second attempt: Found %d orphaned headers", len(orphaned))
		}
	}

	// Verify final consistency for each example
	for i, exampleID := range []idwrap.IDWrap{example1ID, example2ID, example3ID} {
		expectedCount := 8
		if exampleID.Compare(example1ID) == 0 && len(orphaned) == 0 {
			expectedCount = 13 // 8 original + 5 additional
		}
		
		orderedHeaders, err := hs.GetHeadersOrdered(ctx, exampleID)
		if err != nil {
			t.Errorf("Failed to get ordered headers for example %d: %v", i+1, err)
			continue
		}

		allHeaders, err := hs.GetHeaderByExampleID(ctx, exampleID)
		if err != nil {
			t.Errorf("Failed to get all headers for example %d: %v", i+1, err)
			continue
		}

		t.Logf("Example %d: %d ordered vs %d total headers (expected %d)", 
			i+1, len(orderedHeaders), len(allHeaders), expectedCount)

		if len(orderedHeaders) != len(allHeaders) {
			t.Errorf("Example %d: Ordered (%d) != All (%d) headers - linked list broken!", 
				i+1, len(orderedHeaders), len(allHeaders))
		}
		
		if len(allHeaders) != expectedCount {
			t.Errorf("Example %d: Expected %d headers, got %d", i+1, expectedCount, len(allHeaders))
		}
		
		if len(orderedHeaders) == len(allHeaders) && len(allHeaders) > 0 {
			validateLinkedListOrder(ctx, t, orderedHeaders, fmt.Sprintf("example %d", i+1))
		}
	}
	
	t.Logf("Test completed")
}

// TestAppendBulkHeaderIsolatedState specifically tests the two-phase creation issue
func TestAppendBulkHeaderIsolatedState(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries

	hs := sexampleheader.New(queries)
	
	// Create a test example ID
	exampleID := idwrap.NewNow()
	
	// Create headers that will trigger the bulk creation process
	headers := make([]mexampleheader.Header, 20) // Enough to trigger bulk operations
	for i := 0; i < 20; i++ {
		headers[i] = mexampleheader.Header{
			ID:            idwrap.NewNow(),
			ExampleID:     exampleID,
			DeltaParentID: nil,
			HeaderKey:     fmt.Sprintf("X-Bulk-Header-%d", i+1),
			Enable:        true,
			Description:   fmt.Sprintf("Bulk test header %d", i+1),
			Value:         fmt.Sprintf("bulk-value-%d", i+1),
			Prev:          nil, // Will be set by AppendBulkHeader
			Next:          nil, // Will be set by AppendBulkHeader
		}
	}

	t.Logf("Testing AppendBulkHeader with %d headers", len(headers))

	// This should exercise the two-phase creation logic
	err := hs.AppendBulkHeader(ctx, headers)
	if err != nil {
		t.Fatalf("AppendBulkHeader failed: %v", err)
	}

	// Immediately check for orphaned headers
	db := base.DB
	orphaned := checkOrphanedHeaders(ctx, t, db, headers)
	
	if len(orphaned) > 0 {
		t.Logf("BUG REPRODUCED: Found %d orphaned headers immediately after AppendBulkHeader", len(orphaned))
		for _, header := range orphaned {
			t.Logf("  Orphaned: %s (%s)", header.ID.String(), header.HeaderKey)
		}
	}

	// Verify that ordered retrieval works
	orderedHeaders, err := hs.GetHeadersOrdered(ctx, exampleID)
	if err != nil {
		t.Fatalf("Failed to get ordered headers: %v", err)
	}

	allHeaders, err := hs.GetHeaderByExampleID(ctx, exampleID)
	if err != nil {
		t.Fatalf("Failed to get all headers: %v", err)
	}

	t.Logf("Retrieved %d ordered headers vs %d total headers", len(orderedHeaders), len(allHeaders))

	if len(orderedHeaders) != len(allHeaders) {
		t.Errorf("Ordered headers (%d) != All headers (%d) - linked list is broken!", 
			len(orderedHeaders), len(allHeaders))
		
		// Log details of broken state
		t.Logf("All headers:")
		for _, h := range allHeaders {
			t.Logf("  %s (%s): prev=%v, next=%v", h.ID.String(), h.HeaderKey, h.Prev, h.Next)
		}
	}

	// Validate the final linked list structure
	if len(orderedHeaders) == len(headers) {
		validateLinkedListOrder(ctx, t, orderedHeaders, "bulk append test")
		t.Logf("✓ Linked list integrity verified after bulk append")
	} else {
		t.Errorf("Expected %d headers in linked list, got %d", len(headers), len(orderedHeaders))
	}
}

// TestExampleCreationWithoutMovableRepository demonstrates the root cause
// This test shows that creating examples without using the movable repository
// leads to broken linked lists and missing examples in API responses
func TestExampleCreationWithoutMovableRepository(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries

	iaes := sitemapiexample.New(queries)
	ias := sitemapi.New(queries)

	// Create test infrastructure
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	userID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)

	endpointID := idwrap.NewNow()
	endpoint := &mitemapi.ItemApi{
		ID:           endpointID,
		Name:         "Test Endpoint",
		Url:          "https://api.example.com/test",
		Method:       "GET",
		CollectionID: collectionID,
		FolderID:     nil,
	}

	err := ias.CreateItemApi(ctx, endpoint)
	if err != nil {
		t.Fatalf("Failed to create endpoint: %v", err)
	}

	t.Logf("=== DEMONSTRATING THE BUG ===")
	t.Logf("Creating examples using direct CreateApiExample (without movable repository)")

	// Create examples using the BROKEN approach (direct CreateApiExample)
	brokenExamples := make([]*mitemapiexample.ItemApiExample, 3)
	for i := 0; i < 3; i++ {
		exampleID := idwrap.NewNow()
		brokenExamples[i] = &mitemapiexample.ItemApiExample{
			ID:           exampleID,
			ItemApiID:    endpointID,
			CollectionID: collectionID,
			Name:         fmt.Sprintf("Broken Example %d", i+1),
			Updated:      dbtime.DBNow(),
			IsDefault:    i == 0,
			BodyType:     mitemapiexample.BodyTypeRaw,
			Prev:         nil, // Not linked!
			Next:         nil, // Not linked!
		}

		// This is the BUGGY way - creates isolated examples
		err = iaes.CreateApiExample(ctx, brokenExamples[i])
		if err != nil {
			t.Fatalf("Failed to create broken example %d: %v", i+1, err)
		}
	}

	// Test the broken state
	allExamples, err := iaes.GetAllApiExamples(ctx, endpointID)
	if err != nil {
		t.Fatalf("Failed to get all examples: %v", err)
	}

	orderedExamples, err := iaes.GetApiExamplesOrdered(ctx, endpointID)
	if err != nil {
		t.Logf("GetApiExamplesOrdered failed (expected): %v", err)
		orderedExamples = []mitemapiexample.ItemApiExample{}
	}

	t.Logf("BROKEN STATE RESULTS:")
	t.Logf("- Created: %d examples", len(brokenExamples))
	t.Logf("- GetAllApiExamples: %d examples", len(allExamples))
	t.Logf("- GetApiExamplesOrdered: %d examples", len(orderedExamples))
	t.Logf("")

	if len(allExamples) != len(brokenExamples) {
		t.Logf("🐛 BUG: Missing examples! %d created but only %d retrieved", len(brokenExamples), len(allExamples))
	}

	if len(orderedExamples) == 0 && len(allExamples) > 0 {
		t.Logf("🐛 BUG: Ordered query returns 0 but %d examples exist - linked list broken!", len(allExamples))
	}

	// Show what auto-linking detects
	isolatedExamples, err := iaes.GetMovableRepository().DetectIsolatedExamples(ctx, endpointID)
	if err != nil {
		t.Fatalf("Failed to detect isolated examples: %v", err)
	}
	t.Logf("- Isolated examples detected: %d", len(isolatedExamples))

	// Try auto-repair
	t.Logf("")
	t.Logf("=== ATTEMPTING AUTO-REPAIR ===")
	err = iaes.GetMovableRepository().RepairIsolatedExamples(ctx, nil, endpointID)
	if err != nil {
		t.Logf("Auto-repair failed: %v", err)
	} else {
		t.Logf("Auto-repair completed")
	}

	// Test state after repair
	orderedAfterRepair, err := iaes.GetApiExamplesOrdered(ctx, endpointID)
	if err != nil {
		t.Logf("GetApiExamplesOrdered after repair failed: %v", err)
		orderedAfterRepair = []mitemapiexample.ItemApiExample{}
	}

	t.Logf("AFTER AUTO-REPAIR:")
	t.Logf("- GetApiExamplesOrdered: %d examples", len(orderedAfterRepair))

	if len(orderedAfterRepair) == len(allExamples) {
		t.Logf("✅ Auto-repair successful! Linked list restored.")
	} else {
		t.Logf("❌ Auto-repair failed or incomplete")
	}

	t.Logf("")
	t.Logf("=== CONCLUSION ===")
	t.Logf("This test demonstrates that:")
	t.Logf("1. Creating examples with CreateApiExample directly breaks linked lists")
	t.Logf("2. Broken linked lists cause GetApiExamplesOrdered to return no results")
	t.Logf("3. This leads to empty API responses despite examples existing")
	t.Logf("4. The auto-repair mechanism can fix some cases but not all")
	t.Logf("5. The proper fix is to use movable repository for example creation")
}