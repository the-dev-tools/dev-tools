package node_test

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/idwrap"
	"time"
)

// TestMultipleSourceReads tests a node reading from multiple sources
func TestMultipleSourceReads(t *testing.T) {
	// Setup
	nodeID := idwrap.NewNow()
	req := &node.FlowNodeRequest{
		VarMap:           make(map[string]interface{}),
		ReadWriteLock:    &sync.RWMutex{},
		ReadTracker:      make(map[string]interface{}),
		ReadTrackerMutex: &sync.Mutex{},
		CurrentNodeID:    nodeID,
	}

	// Set up data from multiple sources
	// Flow variables
	req.VarMap["apiKey"] = "secret-key-123"
	req.VarMap["baseUrl"] = "https://api.example.com"
	req.VarMap["timeout"] = 30

	// Node outputs
	req.VarMap["authNode"] = map[string]interface{}{
		"token":     "bearer-token-xyz",
		"expiresAt": "2024-12-31T23:59:59Z",
		"userId":    "user-123",
	}
	req.VarMap["configNode"] = map[string]interface{}{
		"headers": map[string]string{
			"Content-Type": "application/json",
			"Accept":       "application/json",
		},
		"retryCount": 3,
	}
	req.VarMap["dataNode"] = map[string]interface{}{
		"items": []interface{}{
			map[string]interface{}{"id": 1, "name": "Item 1"},
			map[string]interface{}{"id": 2, "name": "Item 2"},
		},
		"totalCount": 2,
	}

	// Simulate a node reading from all these sources
	// Read flow variables
	apiKey, _ := node.ReadVarRaw(req, "apiKey")
	baseUrl, _ := node.ReadVarRaw(req, "baseUrl")
	timeout, _ := node.ReadVarRaw(req, "timeout")

	// Read from node outputs
	token, _ := node.ReadNodeVar(req, "authNode", "token")
	_, _ = node.ReadNodeVar(req, "configNode", "headers")
	_, _ = node.ReadNodeVar(req, "dataNode", "items")

	// Verify all sources were tracked
	req.ReadTrackerMutex.Lock()
	defer req.ReadTrackerMutex.Unlock()

	// Check flow variables are tracked
	if tracked, exists := req.ReadTracker["apiKey"]; !exists || tracked != apiKey {
		t.Errorf("apiKey not tracked correctly: %v", tracked)
	}
	if tracked, exists := req.ReadTracker["baseUrl"]; !exists || tracked != baseUrl {
		t.Errorf("baseUrl not tracked correctly: %v", tracked)
	}
	if tracked, exists := req.ReadTracker["timeout"]; !exists || tracked != timeout {
		t.Errorf("timeout not tracked correctly: %v", tracked)
	}

	// Check node outputs are tracked (entire node data should be tracked)
	if authData, exists := req.ReadTracker["authNode"]; exists {
		if authMap, ok := authData.(map[string]interface{}); ok {
			if authMap["token"] != token {
				t.Error("authNode token not tracked correctly")
			}
		} else {
			t.Error("authNode data not tracked as map")
		}
	} else {
		t.Error("authNode not tracked")
	}

	// Verify we have exactly 6 tracked items (3 flow vars + 3 nodes)
	if len(req.ReadTracker) != 6 {
		t.Errorf("Expected 6 tracked items, got %d", len(req.ReadTracker))
	}

	// Verify the tracked data structure
	trackedJSON, err := json.MarshalIndent(req.ReadTracker, "", "  ")
	if err != nil {
		t.Errorf("Failed to marshal tracked data: %v", err)
	}
	t.Logf("Tracked data:\n%s", trackedJSON)
}

// TestConditionalReads tests tracking when reads are conditional
func TestConditionalReads(t *testing.T) {
	// Setup
	nodeID := idwrap.NewNow()
	req := &node.FlowNodeRequest{
		VarMap:           make(map[string]interface{}),
		ReadWriteLock:    &sync.RWMutex{},
		ReadTracker:      make(map[string]interface{}),
		ReadTrackerMutex: &sync.Mutex{},
		CurrentNodeID:    nodeID,
	}

	// Set up test data
	req.VarMap["condition"] = true
	req.VarMap["dataIfTrue"] = "true-branch-data"
	req.VarMap["dataIfFalse"] = "false-branch-data"
	req.VarMap["alwaysRead"] = "always-accessed"

	// Simulate conditional logic
	condition, _ := node.ReadVarRaw(req, "condition")
	alwaysData, _ := node.ReadVarRaw(req, "alwaysRead")

	if condition.(bool) {
		// This branch is taken
		trueData, _ := node.ReadVarRaw(req, "dataIfTrue")
		t.Logf("Read true branch data: %v", trueData)
	} else {
		// This branch is NOT taken
		falseData, _ := node.ReadVarRaw(req, "dataIfFalse")
		t.Logf("Read false branch data: %v", falseData)
	}

	// Verify only the accessed data was tracked
	req.ReadTrackerMutex.Lock()
	defer req.ReadTrackerMutex.Unlock()

	// Should have tracked: condition, alwaysRead, dataIfTrue
	// Should NOT have tracked: dataIfFalse
	if len(req.ReadTracker) != 3 {
		t.Errorf("Expected 3 tracked reads, got %d", len(req.ReadTracker))
	}

	if _, exists := req.ReadTracker["condition"]; !exists {
		t.Error("condition should be tracked")
	}
	if _, exists := req.ReadTracker["alwaysRead"]; !exists {
		t.Error("alwaysRead should be tracked")
	}
	if _, exists := req.ReadTracker["dataIfTrue"]; !exists {
		t.Error("dataIfTrue should be tracked")
	}
	if _, exists := req.ReadTracker["dataIfFalse"]; exists {
		t.Error("dataIfFalse should NOT be tracked")
	}

	t.Logf("Conditional reads tracked correctly: %v", alwaysData)
}

// TestNestedDataStructureTracking tests deep nested structures
func TestNestedDataStructureTracking(t *testing.T) {
	// Setup
	nodeID := idwrap.NewNow()
	req := &node.FlowNodeRequest{
		VarMap:           make(map[string]interface{}),
		ReadWriteLock:    &sync.RWMutex{},
		ReadTracker:      make(map[string]interface{}),
		ReadTrackerMutex: &sync.Mutex{},
		CurrentNodeID:    nodeID,
	}

	// Create a deeply nested structure
	deeplyNested := map[string]interface{}{
		"level1": map[string]interface{}{
			"level2": map[string]interface{}{
				"level3": map[string]interface{}{
					"level4": map[string]interface{}{
						"level5": map[string]interface{}{
							"data": "deeply nested value",
							"array": []interface{}{
								map[string]interface{}{
									"item": "array item 1",
								},
								map[string]interface{}{
									"item": "array item 2",
									"nested": map[string]interface{}{
										"value": 42,
									},
								},
							},
						},
					},
				},
			},
			"sibling": "sibling value",
		},
	}

	req.VarMap["deepData"] = deeplyNested

	// Read the nested data
	data, err := node.ReadVarRaw(req, "deepData")
	if err != nil {
		t.Fatalf("Failed to read nested data: %v", err)
	}

	// Verify the structure was preserved
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		t.Fatal("Expected map type")
	}

	// Navigate to the deeply nested value
	level1 := dataMap["level1"].(map[string]interface{})
	level2 := level1["level2"].(map[string]interface{})
	level3 := level2["level3"].(map[string]interface{})
	level4 := level3["level4"].(map[string]interface{})
	level5 := level4["level5"].(map[string]interface{})

	if level5["data"] != "deeply nested value" {
		t.Error("Deep nesting not preserved")
	}

	// Verify the tracked data is a proper deep copy
	req.ReadTrackerMutex.Lock()
	trackedData := req.ReadTracker["deepData"]
	req.ReadTrackerMutex.Unlock()

	// Modify the original to ensure deep copy
	level5["data"] = "modified value"

	// Check tracked data wasn't affected
	trackedMap := trackedData.(map[string]interface{})
	trackedLevel1 := trackedMap["level1"].(map[string]interface{})
	trackedLevel2 := trackedLevel1["level2"].(map[string]interface{})
	trackedLevel3 := trackedLevel2["level3"].(map[string]interface{})
	trackedLevel4 := trackedLevel3["level4"].(map[string]interface{})
	trackedLevel5 := trackedLevel4["level5"].(map[string]interface{})

	if trackedLevel5["data"] != "deeply nested value" {
		t.Error("Tracked data was modified - deep copy failed")
	}
}

// TestLargeDataTracking tests tracking performance with large data
func TestLargeDataTracking(t *testing.T) {
	// Setup
	nodeID := idwrap.NewNow()
	req := &node.FlowNodeRequest{
		VarMap:           make(map[string]interface{}),
		ReadWriteLock:    &sync.RWMutex{},
		ReadTracker:      make(map[string]interface{}),
		ReadTrackerMutex: &sync.Mutex{},
		CurrentNodeID:    nodeID,
	}

	// Create large data structures
	largeArray := make([]interface{}, 10000)
	for i := 0; i < 10000; i++ {
		largeArray[i] = map[string]interface{}{
			"id":          i,
			"name":        fmt.Sprintf("Item %d", i),
			"description": fmt.Sprintf("This is a longer description for item %d to increase data size", i),
			"metadata": map[string]interface{}{
				"created": time.Now().Format(time.RFC3339),
				"updated": time.Now().Format(time.RFC3339),
				"version": 1,
				"tags":    []string{"tag1", "tag2", "tag3"},
			},
		}
	}

	largeMap := make(map[string]interface{})
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key_%d", i)
		largeMap[key] = map[string]interface{}{
			"value": fmt.Sprintf("Value for key %d", i),
			"nested": map[string]interface{}{
				"data": largeArray[i : i+10], // Reference portion of array
			},
		}
	}

	req.VarMap["largeArray"] = largeArray
	req.VarMap["largeMap"] = largeMap

	// Measure time for reading and tracking large data
	start := time.Now()

	arrayData, err := node.ReadVarRaw(req, "largeArray")
	if err != nil {
		t.Fatalf("Failed to read large array: %v", err)
	}

	arrayReadTime := time.Since(start)
	t.Logf("Time to read and track large array (10k items): %v", arrayReadTime)

	start = time.Now()

	mapData, err := node.ReadVarRaw(req, "largeMap")
	if err != nil {
		t.Fatalf("Failed to read large map: %v", err)
	}

	mapReadTime := time.Since(start)
	t.Logf("Time to read and track large map (1k items): %v", mapReadTime)

	// Verify data integrity
	if arr, ok := arrayData.([]interface{}); ok {
		if len(arr) != 10000 {
			t.Errorf("Expected 10000 array items, got %d", len(arr))
		}
	} else {
		t.Error("Array data type mismatch")
	}

	if m, ok := mapData.(map[string]interface{}); ok {
		if len(m) != 1000 {
			t.Errorf("Expected 1000 map entries, got %d", len(m))
		}
	} else {
		t.Error("Map data type mismatch")
	}

	// Performance thresholds (adjust based on your requirements)
	if arrayReadTime > 100*time.Millisecond {
		t.Logf("Warning: Large array read took longer than expected: %v", arrayReadTime)
	}
	if mapReadTime > 100*time.Millisecond {
		t.Logf("Warning: Large map read took longer than expected: %v", mapReadTime)
	}

	// Verify tracking
	req.ReadTrackerMutex.Lock()
	numTracked := len(req.ReadTracker)
	req.ReadTrackerMutex.Unlock()

	if numTracked != 2 {
		t.Errorf("Expected 2 tracked items, got %d", numTracked)
	}
}

// TestNonExistentVariableRead tests behavior when reading non-existent variables
func TestNonExistentVariableRead(t *testing.T) {
	// Setup
	nodeID := idwrap.NewNow()
	req := &node.FlowNodeRequest{
		VarMap:           make(map[string]interface{}),
		ReadWriteLock:    &sync.RWMutex{},
		ReadTracker:      make(map[string]interface{}),
		ReadTrackerMutex: &sync.Mutex{},
		CurrentNodeID:    nodeID,
	}

	// Add some data
	req.VarMap["existingVar"] = "exists"
	req.VarMap["existingNode"] = map[string]interface{}{"field": "value"}

	// Test 1: Read non-existent flow variable
	_, err := node.ReadVarRaw(req, "nonExistentVar")
	if err != node.ErrVarKeyNotFound {
		t.Errorf("Expected ErrVarKeyNotFound, got %v", err)
	}

	// Test 2: Read from non-existent node
	_, err = node.ReadNodeVar(req, "nonExistentNode", "field")
	if err != node.ErrVarNodeNotFound {
		t.Errorf("Expected ErrVarNodeNotFound, got %v", err)
	}

	// Test 3: Read non-existent field from existing node
	_, err = node.ReadNodeVar(req, "existingNode", "nonExistentField")
	if err != node.ErrVarKeyNotFound {
		t.Errorf("Expected ErrVarKeyNotFound, got %v", err)
	}

	// Test 4: Read existing data to ensure tracking still works
	val, err := node.ReadVarRaw(req, "existingVar")
	if err != nil {
		t.Fatalf("Failed to read existing var: %v", err)
	}
	if val != "exists" {
		t.Errorf("Expected 'exists', got %v", val)
	}

	// Verify only successful reads were tracked
	req.ReadTrackerMutex.Lock()
	defer req.ReadTrackerMutex.Unlock()

	if len(req.ReadTracker) != 1 {
		t.Errorf("Expected 1 tracked read, got %d", len(req.ReadTracker))
	}

	if tracked, exists := req.ReadTracker["existingVar"]; !exists || tracked != "exists" {
		t.Error("Existing var not tracked correctly")
	}

	// Non-existent reads should not be in tracker
	if _, exists := req.ReadTracker["nonExistentVar"]; exists {
		t.Error("Non-existent var should not be tracked")
	}
	if _, exists := req.ReadTracker["nonExistentNode"]; exists {
		t.Error("Non-existent node should not be tracked")
	}
}

// TestNilReadTracker tests behavior when ReadTracker is nil
func TestNilReadTracker(t *testing.T) {
	testCases := []struct {
		name         string
		setupTracker func() (*node.FlowNodeRequest, bool)
		expectPanic  bool
	}{
		{
			name: "nil ReadTracker",
			setupTracker: func() (*node.FlowNodeRequest, bool) {
				req := &node.FlowNodeRequest{
					VarMap:        make(map[string]interface{}),
					ReadWriteLock: &sync.RWMutex{},
					ReadTracker:   nil,
				}
				req.VarMap["test"] = "value"
				return req, false
			},
		},
		{
			name: "nil ReadTrackerMutex",
			setupTracker: func() (*node.FlowNodeRequest, bool) {
				req := &node.FlowNodeRequest{
					VarMap:           make(map[string]interface{}),
					ReadWriteLock:    &sync.RWMutex{},
					ReadTracker:      make(map[string]interface{}),
					ReadTrackerMutex: nil,
				}
				req.VarMap["test"] = "value"
				return req, false
			},
		},
		{
			name: "both nil",
			setupTracker: func() (*node.FlowNodeRequest, bool) {
				req := &node.FlowNodeRequest{
					VarMap:        make(map[string]interface{}),
					ReadWriteLock: &sync.RWMutex{},
				}
				req.VarMap["test"] = "value"
				return req, false
			},
		},
		{
			name: "empty CurrentNodeID",
			setupTracker: func() (*node.FlowNodeRequest, bool) {
				req := &node.FlowNodeRequest{
					VarMap:           make(map[string]interface{}),
					ReadWriteLock:    &sync.RWMutex{},
					ReadTracker:      make(map[string]interface{}),
					ReadTrackerMutex: &sync.Mutex{},
					CurrentNodeID:    idwrap.IDWrap{}, // Empty ID
				}
				req.VarMap["test"] = "value"
				return req, false
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, expectPanic := tc.setupTracker()

			// Function to capture panic
			didPanic := false
			func() {
				defer func() {
					if r := recover(); r != nil {
						didPanic = true
						t.Logf("Recovered from panic: %v", r)
					}
				}()

				// Try to read
				val, err := node.ReadVarRaw(req, "test")
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
					return
				}
				if val != "value" {
					t.Errorf("Expected 'value', got %v", val)
				}
			}()

			if expectPanic && !didPanic {
				t.Error("Expected panic but didn't get one")
			} else if !expectPanic && didPanic {
				t.Error("Unexpected panic")
			}

			// Verify no tracking occurred
			if len(req.ReadTracker) > 0 {
				// For empty CurrentNodeID case, tracking still happens
				if tc.name != "empty CurrentNodeID" {
					t.Error("No tracking should occur when tracker is not properly initialized")
				}
			}
		})
	}
}

// TestConcurrentReadsFromMultipleGoroutines tests thread safety with high concurrency
func TestConcurrentReadsFromMultipleGoroutines(t *testing.T) {
	// Setup
	nodeID := idwrap.NewNow()
	req := &node.FlowNodeRequest{
		VarMap:           make(map[string]interface{}),
		ReadWriteLock:    &sync.RWMutex{},
		ReadTracker:      make(map[string]interface{}),
		ReadTrackerMutex: &sync.Mutex{},
		CurrentNodeID:    nodeID,
	}

	// Create shared data that will be read concurrently
	sharedData := map[string]interface{}{
		"counter": 0,
		"config": map[string]interface{}{
			"setting1": "value1",
			"setting2": "value2",
		},
		"array": []int{1, 2, 3, 4, 5},
	}
	req.VarMap["shared"] = sharedData

	// Create node-specific data
	for i := 0; i < 10; i++ {
		nodeName := fmt.Sprintf("node%d", i)
		req.VarMap[nodeName] = map[string]interface{}{
			"output": fmt.Sprintf("result from node %d", i),
			"status": "success",
		}
	}

	// Track read counts for verification
	readCounts := &sync.Map{}

	// Run concurrent reads
	var wg sync.WaitGroup
	numGoroutines := 100
	readsPerGoroutine := 50

	start := time.Now()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()

			for j := 0; j < readsPerGoroutine; j++ {
				// Mix of different read patterns
				switch j % 4 {
				case 0:
					// Read shared data
					val, err := node.ReadVarRaw(req, "shared")
					if err != nil {
						t.Errorf("Routine %d: Failed to read shared: %v", routineID, err)
						continue
					}
					readCounts.Store("shared", val)

				case 1:
					// Read from a specific node
					nodeID := j % 10
					nodeName := fmt.Sprintf("node%d", nodeID)
					val, err := node.ReadNodeVar(req, nodeName, "output")
					if err != nil {
						t.Errorf("Routine %d: Failed to read %s: %v", routineID, nodeName, err)
						continue
					}
					readCounts.Store(nodeName, val)

				case 2:
					// Try to read non-existent data (should fail gracefully)
					_, err := node.ReadVarRaw(req, fmt.Sprintf("nonexistent%d", j))
					if err == nil {
						t.Errorf("Routine %d: Expected error for non-existent key", routineID)
					}

				case 3:
					// Read nested data from shared
					val, err := node.ReadVarRaw(req, "shared")
					if err != nil {
						continue
					}
					// Access nested fields to ensure deep copy works
					if sharedMap, ok := val.(map[string]interface{}); ok {
						_ = sharedMap["config"]
						_ = sharedMap["array"]
					}
				}
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	t.Logf("Concurrent test completed in %v", elapsed)
	t.Logf("Total operations: %d", numGoroutines*readsPerGoroutine)

	// Verify tracking consistency
	req.ReadTrackerMutex.Lock()
	numTracked := len(req.ReadTracker)
	req.ReadTrackerMutex.Unlock()

	// Should have tracked "shared" + up to 10 nodes
	if numTracked == 0 || numTracked > 11 {
		t.Errorf("Unexpected number of tracked items: %d", numTracked)
	}

	// Verify data integrity
	readCounts.Range(func(key, value interface{}) bool {
		req.ReadTrackerMutex.Lock()
		trackedVal, exists := req.ReadTracker[key.(string)]
		req.ReadTrackerMutex.Unlock()

		if !exists {
			t.Errorf("Key %s was read but not tracked", key)
		}

		// For shared data, verify it matches
		if key == "shared" {
			if _, ok := trackedVal.(map[string]interface{}); !ok {
				t.Error("Shared data should be a map")
			}
		}

		return true
	})
}
