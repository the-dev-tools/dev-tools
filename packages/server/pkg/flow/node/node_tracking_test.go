package node_test

import (
	"fmt"
	"sync"
	"testing"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/tracking"
)

func TestWriteNodeVar_WithTracking(t *testing.T) {
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		ReadWriteLock: &sync.RWMutex{},
	}

	tracker := tracking.NewVariableTracker()

	// Test writing a node variable with tracking
	err := node.WriteNodeVarWithTracking(req, "testNode", "key1", "value1", tracker)
	if err != nil {
		t.Fatalf("WriteNodeVarWithTracking failed: %v", err)
	}

	// Verify the value was written correctly
	value, err := node.ReadNodeVar(req, "testNode", "key1")
	if err != nil {
		t.Fatalf("ReadNodeVar failed: %v", err)
	}
	if value != "value1" {
		t.Errorf("Expected 'value1', got %v", value)
	}

	// Verify the write was tracked
	writtenVars := tracker.GetWrittenVars()
	if len(writtenVars) != 1 {
		t.Errorf("Expected 1 tracked write, got %d", len(writtenVars))
	}
	if writtenVars["testNode.key1"] != "value1" {
		t.Errorf("Expected tracked write 'testNode.key1'='value1', got %v", writtenVars["testNode.key1"])
	}
}

func TestWriteNodeVar_WithoutTracking(t *testing.T) {
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		ReadWriteLock: &sync.RWMutex{},
	}

	// Test writing without tracker (should work normally)
	err := node.WriteNodeVarWithTracking(req, "testNode", "key1", "value1", nil)
	if err != nil {
		t.Fatalf("WriteNodeVarWithTracking with nil tracker failed: %v", err)
	}

	// Verify the value was written correctly
	value, err := node.ReadNodeVar(req, "testNode", "key1")
	if err != nil {
		t.Fatalf("ReadNodeVar failed: %v", err)
	}
	if value != "value1" {
		t.Errorf("Expected 'value1', got %v", value)
	}
}

func TestWriteNodeVarRaw_WithTracking(t *testing.T) {
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		ReadWriteLock: &sync.RWMutex{},
	}

	tracker := tracking.NewVariableTracker()

	testData := map[string]interface{}{
		"result": "success",
		"code":   200,
	}

	// Test writing raw node variable with tracking
	err := node.WriteNodeVarRawWithTracking(req, "testNode", testData, tracker)
	if err != nil {
		t.Fatalf("WriteNodeVarRawWithTracking failed: %v", err)
	}

	// Verify the value was written correctly
	value, err := node.ReadVarRaw(req, "testNode")
	if err != nil {
		t.Fatalf("ReadVarRaw failed: %v", err)
	}
	dataMap, ok := value.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map[string]interface{}, got %T", value)
	}
	if dataMap["result"] != "success" {
		t.Errorf("Expected result='success', got %v", dataMap["result"])
	}

	// Verify the write was tracked
	writtenVars := tracker.GetWrittenVars()
	if len(writtenVars) != 1 {
		t.Errorf("Expected 1 tracked write, got %d", len(writtenVars))
	}
	trackedData, ok := writtenVars["testNode"].(map[string]interface{})
	if !ok {
		t.Errorf("Expected tracked data to be map[string]interface{}, got %T", writtenVars["testNode"])
	}
	if trackedData["result"] != "success" {
		t.Errorf("Expected tracked result='success', got %v", trackedData["result"])
	}
}

func TestWriteNodeVarBulk_WithTracking(t *testing.T) {
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		ReadWriteLock: &sync.RWMutex{},
	}

	tracker := tracking.NewVariableTracker()

	bulkData := map[string]interface{}{
		"field1": "value1",
		"field2": 42,
		"field3": true,
	}

	// Test bulk writing with tracking
	err := node.WriteNodeVarBulkWithTracking(req, "testNode", bulkData, tracker)
	if err != nil {
		t.Fatalf("WriteNodeVarBulkWithTracking failed: %v", err)
	}

	// Verify all values were written correctly
	for key, expectedValue := range bulkData {
		value, err := node.ReadNodeVar(req, "testNode", key)
		if err != nil {
			t.Fatalf("ReadNodeVar failed for %s: %v", key, err)
		}
		if value != expectedValue {
			t.Errorf("Expected %s=%v, got %v", key, expectedValue, value)
		}
	}

	// Verify all writes were tracked
	writtenVars := tracker.GetWrittenVars()
	if len(writtenVars) != 3 {
		t.Errorf("Expected 3 tracked writes, got %d", len(writtenVars))
	}

	expectedKeys := []string{"testNode.field1", "testNode.field2", "testNode.field3"}
	for _, expectedKey := range expectedKeys {
		if _, exists := writtenVars[expectedKey]; !exists {
			t.Errorf("Expected tracked write for key %s", expectedKey)
		}
	}

	if writtenVars["testNode.field1"] != "value1" {
		t.Errorf("Expected field1='value1', got %v", writtenVars["testNode.field1"])
	}
	if writtenVars["testNode.field2"] != 42 {
		t.Errorf("Expected field2=42, got %v", writtenVars["testNode.field2"])
	}
	if writtenVars["testNode.field3"] != true {
		t.Errorf("Expected field3=true, got %v", writtenVars["testNode.field3"])
	}
}

func TestReadVarRaw_WithTracking(t *testing.T) {
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		ReadWriteLock: &sync.RWMutex{},
	}

	// Set up test data
	req.VarMap["testKey"] = "testValue"
	req.VarMap["complexData"] = map[string]interface{}{
		"nested": "value",
	}

	tracker := tracking.NewVariableTracker()

	// Test reading raw variable with tracking
	value, err := node.ReadVarRawWithTracking(req, "testKey", tracker)
	if err != nil {
		t.Fatalf("ReadVarRawWithTracking failed: %v", err)
	}
	if value != "testValue" {
		t.Errorf("Expected 'testValue', got %v", value)
	}

	// Test reading complex data
	complexValue, err := node.ReadVarRawWithTracking(req, "complexData", tracker)
	if err != nil {
		t.Fatalf("ReadVarRawWithTracking failed for complex data: %v", err)
	}
	complexMap, ok := complexValue.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map[string]interface{}, got %T", complexValue)
	}
	if complexMap["nested"] != "value" {
		t.Errorf("Expected nested='value', got %v", complexMap["nested"])
	}

	// Verify reads were tracked
	readVars := tracker.GetReadVars()
	if len(readVars) != 2 {
		t.Errorf("Expected 2 tracked reads, got %d", len(readVars))
	}
	if readVars["testKey"] != "testValue" {
		t.Errorf("Expected tracked read testKey='testValue', got %v", readVars["testKey"])
	}
}

func TestReadVarRaw_WithoutTracking(t *testing.T) {
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		ReadWriteLock: &sync.RWMutex{},
	}

	req.VarMap["testKey"] = "testValue"

	// Test reading without tracker (should work normally)
	value, err := node.ReadVarRawWithTracking(req, "testKey", nil)
	if err != nil {
		t.Fatalf("ReadVarRawWithTracking with nil tracker failed: %v", err)
	}
	if value != "testValue" {
		t.Errorf("Expected 'testValue', got %v", value)
	}
}

func TestReadNodeVar_WithTracking(t *testing.T) {
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		ReadWriteLock: &sync.RWMutex{},
	}

	// Set up test data
	nodeData := map[string]interface{}{
		"result": "success",
		"code":   200,
		"data":   []int{1, 2, 3},
	}
	req.VarMap["testNode"] = nodeData

	tracker := tracking.NewVariableTracker()

	// Test reading node variables with tracking
	result, err := node.ReadNodeVarWithTracking(req, "testNode", "result", tracker)
	if err != nil {
		t.Fatalf("ReadNodeVarWithTracking failed: %v", err)
	}
	if result != "success" {
		t.Errorf("Expected 'success', got %v", result)
	}

	code, err := node.ReadNodeVarWithTracking(req, "testNode", "code", tracker)
	if err != nil {
		t.Fatalf("ReadNodeVarWithTracking failed: %v", err)
	}
	if code != 200 {
		t.Errorf("Expected 200, got %v", code)
	}

	// Verify reads were tracked
	readVars := tracker.GetReadVars()
	if len(readVars) != 2 {
		t.Errorf("Expected 2 tracked reads, got %d", len(readVars))
	}
	if readVars["testNode.result"] != "success" {
		t.Errorf("Expected tracked read testNode.result='success', got %v", readVars["testNode.result"])
	}
	if readVars["testNode.code"] != 200 {
		t.Errorf("Expected tracked read testNode.code=200, got %v", readVars["testNode.code"])
	}
}

func TestNode_ReadWrite_Integration(t *testing.T) {
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		ReadWriteLock: &sync.RWMutex{},
	}

	tracker := tracking.NewVariableTracker()

	// Simulate a complete node execution cycle
	// 1. Node reads input data
	req.VarMap["inputData"] = map[string]interface{}{
		"value": 10,
		"multiplier": 3,
	}

	inputValue, err := node.ReadNodeVarWithTracking(req, "inputData", "value", tracker)
	if err != nil {
		t.Fatalf("Failed to read input value: %v", err)
	}

	multiplier, err := node.ReadNodeVarWithTracking(req, "inputData", "multiplier", tracker)
	if err != nil {
		t.Fatalf("Failed to read multiplier: %v", err)
	}

	// 2. Node processes data and writes output
	result := inputValue.(int) * multiplier.(int)
	
	err = node.WriteNodeVarWithTracking(req, "outputData", "result", result, tracker)
	if err != nil {
		t.Fatalf("Failed to write result: %v", err)
	}

	err = node.WriteNodeVarWithTracking(req, "outputData", "status", "completed", tracker)
	if err != nil {
		t.Fatalf("Failed to write status: %v", err)
	}

	// 3. Verify complete tracking
	readVars := tracker.GetReadVars()
	writtenVars := tracker.GetWrittenVars()

	// Should have 2 reads and 2 writes
	if len(readVars) != 2 {
		t.Errorf("Expected 2 tracked reads, got %d", len(readVars))
	}
	if len(writtenVars) != 2 {
		t.Errorf("Expected 2 tracked writes, got %d", len(writtenVars))
	}

	// Verify specific tracking
	if readVars["inputData.value"] != 10 {
		t.Errorf("Expected read inputData.value=10, got %v", readVars["inputData.value"])
	}
	if readVars["inputData.multiplier"] != 3 {
		t.Errorf("Expected read inputData.multiplier=3, got %v", readVars["inputData.multiplier"])
	}
	if writtenVars["outputData.result"] != 30 {
		t.Errorf("Expected write outputData.result=30, got %v", writtenVars["outputData.result"])
	}
	if writtenVars["outputData.status"] != "completed" {
		t.Errorf("Expected write outputData.status='completed', got %v", writtenVars["outputData.status"])
	}
}

func TestNode_ErrorHandling_WithTracking(t *testing.T) {
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		ReadWriteLock: &sync.RWMutex{},
	}

	tracker := tracking.NewVariableTracker()

	// Test reading non-existent key
	_, err := node.ReadVarRawWithTracking(req, "nonexistent", tracker)
	if err != node.ErrVarKeyNotFound {
		t.Errorf("Expected ErrVarKeyNotFound, got %v", err)
	}

	// Test reading non-existent node
	_, err = node.ReadNodeVarWithTracking(req, "nonexistent", "key", tracker)
	if err != node.ErrVarNodeNotFound {
		t.Errorf("Expected ErrVarNodeNotFound, got %v", err)
	}

	// No reads should be tracked for failed operations
	readVars := tracker.GetReadVars()
	if len(readVars) != 0 {
		t.Errorf("Expected no tracked reads for failed operations, got %d", len(readVars))
	}
}

func TestNode_ConcurrentTracking(t *testing.T) {
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		ReadWriteLock: &sync.RWMutex{},
	}

	tracker := tracking.NewVariableTracker()

	// Set up initial data
	for i := 0; i < 50; i++ {
		req.VarMap[fmt.Sprintf("key%d", i)] = fmt.Sprintf("value%d", i)
	}

	var wg sync.WaitGroup
	numGoroutines := 10

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				key := fmt.Sprintf("key%d", j)
				
				// Read with tracking
				_, err := node.ReadVarRawWithTracking(req, key, tracker)
				if err != nil {
					t.Errorf("Goroutine %d: ReadVarRawWithTracking failed: %v", goroutineID, err)
				}

				// Write with tracking
				writeKey := fmt.Sprintf("output_%d_%d", goroutineID, j)
				writeValue := fmt.Sprintf("result_%d_%d", goroutineID, j)
				err = node.WriteNodeVarWithTracking(req, "outputs", writeKey, writeValue, tracker)
				if err != nil {
					t.Errorf("Goroutine %d: WriteNodeVarWithTracking failed: %v", goroutineID, err)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify tracking worked correctly under concurrency
	readVars := tracker.GetReadVars()
	writtenVars := tracker.GetWrittenVars()

	// Should have tracked some reads and writes
	if len(readVars) == 0 {
		t.Error("Expected some tracked reads from concurrent operations")
	}
	if len(writtenVars) == 0 {
		t.Error("Expected some tracked writes from concurrent operations")
	}

	t.Logf("Concurrent test completed with %d reads and %d writes tracked", len(readVars), len(writtenVars))
}