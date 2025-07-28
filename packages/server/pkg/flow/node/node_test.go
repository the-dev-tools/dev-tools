package node_test

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/idwrap"
)

func TestAddNodeVar(t *testing.T) {
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		ReadWriteLock: &sync.RWMutex{},
	}

	key := "testKey"
	value := "testValue"
	nodeName := "test-node"

	err := node.WriteNodeVar(req, nodeName, key, value)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	storedValue, err := node.ReadNodeVar(req, nodeName, key)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if storedValue != value {
		t.Fatalf("expected %v, got %v", value, storedValue)
	}
}

func TestReadVarRaw(t *testing.T) {
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		ReadWriteLock: &sync.RWMutex{},
	}

	key := "testKey"
	value := "testValue"
	req.VarMap[key] = value

	storedValue, err := node.ReadVarRaw(req, key)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if storedValue != value {
		t.Fatalf("expected %v, got %v", value, storedValue)
	}
}

func TestReadNodeVar(t *testing.T) {
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		ReadWriteLock: &sync.RWMutex{},
	}

	key := "testKey"
	value := "testValue"
	nodeName := "test-node"
	req.VarMap[nodeName] = map[string]interface{}{key: value}

	storedValue, err := node.ReadNodeVar(req, nodeName, key)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if storedValue != value {
		t.Fatalf("expected %v, got %v", value, storedValue)
	}
}

func TestReadNodeVar_NodeNotFound(t *testing.T) {
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		ReadWriteLock: &sync.RWMutex{},
	}

	key := "testKey"
	nodeName := "test-node"

	_, err := node.ReadNodeVar(req, nodeName, key)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if err != node.ErrVarNodeNotFound {
		t.Fatalf("expected %v, got %v", node.ErrNodeNotFound, err)
	}
}

func TestReadNodeVar_KeyNotFound(t *testing.T) {
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		ReadWriteLock: &sync.RWMutex{},
	}

	nodeName := "test-node"
	req.VarMap[nodeName] = map[string]interface{}{}

	key := "testKey"

	_, err := node.ReadNodeVar(req, nodeName, key)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	expectedErr := errors.New("key not found")
	if err.Error() != expectedErr.Error() {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

// New tests for read tracking functionality

func TestReadVarRawTracking(t *testing.T) {
	// Setup
	nodeID := idwrap.NewNow()
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		ReadWriteLock: &sync.RWMutex{},
		// Initialize tracking fields
		ReadTracker:      make(map[string]interface{}),
		ReadTrackerMutex: &sync.Mutex{},
		CurrentNodeID:    nodeID,
	}

	// Set some test data
	req.VarMap["testKey1"] = "testValue1"
	req.VarMap["testKey2"] = map[string]interface{}{"nested": "value"}
	req.VarMap["testKey3"] = 123

	// Test 1: Read a simple value
	val1, err := node.ReadVarRaw(req, "testKey1")
	if err != nil {
		t.Fatalf("ReadVarRaw failed: %v", err)
	}
	if val1 != "testValue1" {
		t.Errorf("Expected testValue1, got %v", val1)
	}

	// Verify it was tracked
	req.ReadTrackerMutex.Lock()
	tracked1, exists := req.ReadTracker["testKey1"]
	req.ReadTrackerMutex.Unlock()
	if !exists {
		t.Error("testKey1 was not tracked")
	}
	if tracked1 != "testValue1" {
		t.Errorf("Tracked value mismatch: expected testValue1, got %v", tracked1)
	}

	// Test 2: Read a complex value
	val2, err := node.ReadVarRaw(req, "testKey2")
	if err != nil {
		t.Fatalf("ReadVarRaw failed: %v", err)
	}
	mapVal, ok := val2.(map[string]interface{})
	if !ok || mapVal["nested"] != "value" {
		t.Errorf("Expected nested map, got %v", val2)
	}

	// Verify complex value was tracked
	req.ReadTrackerMutex.Lock()
	tracked2, exists := req.ReadTracker["testKey2"]
	req.ReadTrackerMutex.Unlock()
	if !exists {
		t.Error("testKey2 was not tracked")
	}
	// Verify the tracked value is correct
	if trackedMap, ok := tracked2.(map[string]interface{}); ok {
		if trackedMap["nested"] != "value" {
			t.Errorf("Tracked nested value mismatch: expected 'value', got %v", trackedMap["nested"])
		}
	} else {
		t.Errorf("Tracked value is not a map: %T", tracked2)
	}

	// Test 3: Read non-existent key
	_, err = node.ReadVarRaw(req, "nonExistent")
	if err != node.ErrVarKeyNotFound {
		t.Errorf("Expected ErrVarKeyNotFound, got %v", err)
	}

	// Verify non-existent key was NOT tracked
	req.ReadTrackerMutex.Lock()
	_, exists = req.ReadTracker["nonExistent"]
	req.ReadTrackerMutex.Unlock()
	if exists {
		t.Error("Non-existent key should not be tracked")
	}

	// Test 4: Verify all tracked reads
	req.ReadTrackerMutex.Lock()
	numTracked := len(req.ReadTracker)
	req.ReadTrackerMutex.Unlock()
	if numTracked != 2 {
		t.Errorf("Expected 2 tracked reads, got %d", numTracked)
	}
}

func TestReadNodeVarTracking(t *testing.T) {
	// Setup
	nodeID := idwrap.NewNow()
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		ReadWriteLock: &sync.RWMutex{},
		// Initialize tracking fields
		ReadTracker:      make(map[string]interface{}),
		ReadTrackerMutex: &sync.Mutex{},
		CurrentNodeID:    nodeID,
	}

	// Set some test node data
	req.VarMap["node1"] = map[string]interface{}{
		"output": "result1",
		"status": "success",
	}
	req.VarMap["node2"] = map[string]interface{}{
		"data": []int{1, 2, 3},
	}

	// Test 1: Read node variable
	val1, err := node.ReadNodeVar(req, "node1", "output")
	if err != nil {
		t.Fatalf("ReadNodeVar failed: %v", err)
	}
	if val1 != "result1" {
		t.Errorf("Expected result1, got %v", val1)
	}

	// Verify it was tracked with node prefix
	req.ReadTrackerMutex.Lock()
	trackedNode, exists := req.ReadTracker["node1"]
	req.ReadTrackerMutex.Unlock()
	if !exists {
		t.Error("node1 was not tracked")
	}

	// Check the tracked value is the entire node data
	nodeData, ok := trackedNode.(map[string]interface{})
	if !ok {
		t.Errorf("Expected map for tracked node data, got %T", trackedNode)
	}
	if nodeData["output"] != "result1" || nodeData["status"] != "success" {
		t.Error("Tracked node data doesn't match expected values")
	}

	// Test 2: Read from non-existent node
	_, err = node.ReadNodeVar(req, "nodeX", "field")
	if err != node.ErrVarNodeNotFound {
		t.Errorf("Expected ErrVarNodeNotFound, got %v", err)
	}

	// Test 3: Read non-existent key from existing node
	_, err = node.ReadNodeVar(req, "node1", "nonExistent")
	if err != node.ErrVarKeyNotFound {
		t.Errorf("Expected ErrVarKeyNotFound, got %v", err)
	}

	// Test 4: Multiple reads from same node should not duplicate tracking
	val2, err := node.ReadNodeVar(req, "node1", "status")
	if err != nil {
		t.Fatalf("ReadNodeVar failed: %v", err)
	}
	if val2 != "success" {
		t.Errorf("Expected success, got %v", val2)
	}

	req.ReadTrackerMutex.Lock()
	numTracked := len(req.ReadTracker)
	req.ReadTrackerMutex.Unlock()
	if numTracked != 1 { // Should still be 1 since we only read from node1
		t.Errorf("Expected 1 tracked node, got %d", numTracked)
	}
}

func TestNoTrackingWhenNil(t *testing.T) {
	// Setup without tracking
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		ReadWriteLock: &sync.RWMutex{},
		// ReadTracker is nil - no tracking should occur
		ReadTracker:      nil,
		ReadTrackerMutex: nil,
		CurrentNodeID:    idwrap.IDWrap{},
	}

	req.VarMap["testKey"] = "testValue"
	req.VarMap["node1"] = map[string]interface{}{"field": "value"}

	// Test ReadVarRaw without tracking
	val1, err := node.ReadVarRaw(req, "testKey")
	if err != nil {
		t.Fatalf("ReadVarRaw failed: %v", err)
	}
	if val1 != "testValue" {
		t.Errorf("Expected testValue, got %v", val1)
	}

	// Test ReadNodeVar without tracking
	val2, err := node.ReadNodeVar(req, "node1", "field")
	if err != nil {
		t.Fatalf("ReadNodeVar failed: %v", err)
	}
	if val2 != "value" {
		t.Errorf("Expected value, got %v", val2)
	}

	// Verify no tracking occurred (ReadTracker is still nil)
	if req.ReadTracker != nil {
		t.Error("ReadTracker should remain nil when tracking is disabled")
	}
}

func TestThreadSafety(t *testing.T) {
	// Setup
	nodeID := idwrap.NewNow()
	req := &node.FlowNodeRequest{
		VarMap:           make(map[string]interface{}),
		ReadWriteLock:    &sync.RWMutex{},
		ReadTracker:      make(map[string]interface{}),
		ReadTrackerMutex: &sync.Mutex{},
		CurrentNodeID:    nodeID,
	}

	// Populate with test data
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key%d", i)
		req.VarMap[key] = fmt.Sprintf("value%d", i)
	}

	// Run concurrent reads
	var wg sync.WaitGroup
	numGoroutines := 10
	readsPerGoroutine := 50

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < readsPerGoroutine; j++ {
				keyNum := (goroutineID*readsPerGoroutine + j) % 100
				key := fmt.Sprintf("key%d", keyNum)

				_, err := node.ReadVarRaw(req, key)
				if err != nil {
					t.Errorf("Goroutine %d: ReadVarRaw failed: %v", goroutineID, err)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify tracking worked correctly
	req.ReadTrackerMutex.Lock()
	numTracked := len(req.ReadTracker)
	req.ReadTrackerMutex.Unlock()

	// We should have tracked up to 100 unique keys (depending on access pattern)
	if numTracked == 0 || numTracked > 100 {
		t.Errorf("Unexpected number of tracked reads: %d", numTracked)
	}

	// Verify tracked values are correct
	req.ReadTrackerMutex.Lock()
	for key, trackedVal := range req.ReadTracker {
		expectedVal := req.VarMap[key]
		if trackedVal != expectedVal {
			t.Errorf("Tracked value mismatch for %s: expected %v, got %v", key, expectedVal, trackedVal)
		}
	}
	req.ReadTrackerMutex.Unlock()
}

func TestDeepCopyTracking(t *testing.T) {
	// Setup
	nodeID := idwrap.NewNow()
	req := &node.FlowNodeRequest{
		VarMap:           make(map[string]interface{}),
		ReadWriteLock:    &sync.RWMutex{},
		ReadTracker:      make(map[string]interface{}),
		ReadTrackerMutex: &sync.Mutex{},
		CurrentNodeID:    nodeID,
	}

	// Create a complex nested structure
	complexData := map[string]interface{}{
		"nested": map[string]interface{}{
			"deep": []interface{}{1, 2, 3},
		},
	}
	req.VarMap["complex"] = complexData

	// Read the complex data
	val, err := node.ReadVarRaw(req, "complex")
	if err != nil {
		t.Fatalf("ReadVarRaw failed: %v", err)
	}

	// Modify the returned value
	if mapVal, ok := val.(map[string]interface{}); ok {
		if nestedMap, ok := mapVal["nested"].(map[string]interface{}); ok {
			nestedMap["modified"] = true
		}
	}

	// Verify the tracked value wasn't modified
	req.ReadTrackerMutex.Lock()
	trackedVal := req.ReadTracker["complex"]
	req.ReadTrackerMutex.Unlock()

	trackedMap, ok := trackedVal.(map[string]interface{})
	if !ok {
		t.Fatal("Tracked value should be a map")
	}

	if nestedMap, ok := trackedMap["nested"].(map[string]interface{}); ok {
		if _, exists := nestedMap["modified"]; exists {
			t.Error("Tracked value was modified - deep copy failed")
		}
	}
}
