package node_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/tracking"

	"github.com/stretchr/testify/require"
)

func TestWriteNodeVar_WithTracking(t *testing.T) {
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		ReadWriteLock: &sync.RWMutex{},
	}

	tracker := tracking.NewVariableTracker()

	// Test writing a node variable with tracking
	err := node.WriteNodeVarWithTracking(req, "testNode", "key1", "value1", tracker)
	require.NoError(t, err, "WriteNodeVarWithTracking failed")

	// Verify the value was written correctly
	value, err := node.ReadNodeVar(req, "testNode", "key1")
	require.NoError(t, err, "ReadNodeVar failed")
	require.Equal(t, "value1", value)

	// Verify the write was tracked
	writtenVars := tracker.GetWrittenVars()
	require.Len(t, writtenVars, 1, "Expected 1 tracked write")
	require.Equal(t, "value1", writtenVars["testNode.key1"])
}

func TestWriteNodeVar_WithoutTracking(t *testing.T) {
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		ReadWriteLock: &sync.RWMutex{},
	}

	// Test writing without tracker (should work normally)
	err := node.WriteNodeVarWithTracking(req, "testNode", "key1", "value1", nil)
	require.NoError(t, err, "WriteNodeVarWithTracking with nil tracker failed")

	// Verify the value was written correctly
	value, err := node.ReadNodeVar(req, "testNode", "key1")
	require.NoError(t, err, "ReadNodeVar failed")
	require.Equal(t, "value1", value)
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
	require.NoError(t, err, "WriteNodeVarRawWithTracking failed")

	// Verify the value was written correctly
	value, err := node.ReadVarRaw(req, "testNode")
	require.NoError(t, err, "ReadVarRaw failed")
	dataMap, ok := value.(map[string]interface{})
	require.True(t, ok, "Expected map[string]interface{}, got %T", value)
	require.Equal(t, "success", dataMap["result"])

	// Verify the write was tracked
	writtenVars := tracker.GetWrittenVars()
	require.Len(t, writtenVars, 1, "Expected 1 tracked write")
	trackedData, ok := writtenVars["testNode"].(map[string]interface{})
	if !ok {
		t.Errorf("Expected tracked data to be map[string]interface{}, got %T", writtenVars["testNode"])
	}
	require.Equal(t, "success", trackedData["result"])
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
	require.NoError(t, err, "WriteNodeVarBulkWithTracking failed")

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
	require.Len(t, writtenVars, 3, "Expected 3 tracked writes")

	expectedKeys := []string{"testNode.field1", "testNode.field2", "testNode.field3"}
	for _, expectedKey := range expectedKeys {
		if _, exists := writtenVars[expectedKey]; !exists {
			t.Errorf("Expected tracked write for key %s", expectedKey)
		}
	}

	require.Equal(t, "value1", writtenVars["testNode.field1"])
	require.Equal(t, 42, writtenVars["testNode.field2"])
	require.Equal(t, true, writtenVars["testNode.field3"])
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
	require.NoError(t, err, "ReadVarRawWithTracking failed")
	require.Equal(t, "testValue", value)

	// Test reading complex data
	complexValue, err := node.ReadVarRawWithTracking(req, "complexData", tracker)
	require.NoError(t, err, "ReadVarRawWithTracking failed for complex data")
	complexMap, ok := complexValue.(map[string]interface{})
	require.True(t, ok, "Expected map[string]interface{}, got %T", complexValue)
	require.Equal(t, "value", complexMap["nested"])

	// Verify reads were tracked
	readVars := tracker.GetReadVars()
	require.Len(t, readVars, 2, "Expected 2 tracked reads")
	require.Equal(t, "testValue", readVars["testKey"])
}

func TestReadVarRaw_WithoutTracking(t *testing.T) {
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		ReadWriteLock: &sync.RWMutex{},
	}

	req.VarMap["testKey"] = "testValue"

	// Test reading without tracker (should work normally)
	value, err := node.ReadVarRawWithTracking(req, "testKey", nil)
	require.NoError(t, err, "ReadVarRawWithTracking with nil tracker failed")
	require.Equal(t, "testValue", value)
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
	require.NoError(t, err, "ReadNodeVarWithTracking failed")
	require.Equal(t, "success", result)

	code, err := node.ReadNodeVarWithTracking(req, "testNode", "code", tracker)
	require.NoError(t, err, "ReadNodeVarWithTracking failed")
	if code != 200 {
		t.Errorf("Expected 200, got %v", code)
	}

	// Verify reads were tracked
	readVars := tracker.GetReadVars()
	require.Len(t, readVars, 2, "Expected 2 tracked reads")
	require.Equal(t, "success", readVars["testNode.result"])
	require.Equal(t, 200, readVars["testNode.code"])
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
		"value":      10,
		"multiplier": 3,
	}

	inputValue, err := node.ReadNodeVarWithTracking(req, "inputData", "value", tracker)
	require.NoError(t, err, "Failed to read input value")

	multiplier, err := node.ReadNodeVarWithTracking(req, "inputData", "multiplier", tracker)
	require.NoError(t, err, "Failed to read multiplier")

	// 2. Node processes data and writes output
	result := inputValue.(int) * multiplier.(int)

	err = node.WriteNodeVarWithTracking(req, "outputData", "result", result, tracker)
	require.NoError(t, err, "Failed to write result")

	err = node.WriteNodeVarWithTracking(req, "outputData", "status", "completed", tracker)
	require.NoError(t, err, "Failed to write status")

	// 3. Verify complete tracking
	readVars := tracker.GetReadVars()
	writtenVars := tracker.GetWrittenVars()

	// Should have 2 reads and 2 writes
	require.Len(t, readVars, 2, "Expected 2 tracked reads")
	require.Len(t, writtenVars, 2, "Expected 2 tracked writes")

	// Verify specific tracking
	require.Equal(t, 10, readVars["inputData.value"])
	require.Equal(t, 3, readVars["inputData.multiplier"])
	require.Equal(t, 30, writtenVars["outputData.result"])
	require.Equal(t, "completed", writtenVars["outputData.status"])
}

func TestNode_ErrorHandling_WithTracking(t *testing.T) {
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		ReadWriteLock: &sync.RWMutex{},
	}

	tracker := tracking.NewVariableTracker()

	// Test reading non-existent key
	_, err := node.ReadVarRawWithTracking(req, "nonexistent", tracker)
	require.Equal(t, node.ErrVarKeyNotFound, err)

	// Test reading non-existent node
	_, err = node.ReadNodeVarWithTracking(req, "nonexistent", "key", tracker)
	require.Equal(t, node.ErrVarNodeNotFound, err)

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
					require.NoError(t, err, "Goroutine %d: ReadVarRawWithTracking failed", goroutineID)
				}

				// Write with tracking
				writeKey := fmt.Sprintf("output_%d_%d", goroutineID, j)
				writeValue := fmt.Sprintf("result_%d_%d", goroutineID, j)
				err = node.WriteNodeVarWithTracking(req, "outputs", writeKey, writeValue, tracker)
				if err != nil {
					require.NoError(t, err, "Goroutine %d: WriteNodeVarWithTracking failed", goroutineID)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify tracking worked correctly under concurrency
	readVars := tracker.GetReadVars()
	writtenVars := tracker.GetWrittenVars()

	// Should have tracked some reads and writes
	require.NotEmpty(t, readVars, "Expected some tracked reads from concurrent operations")
	require.NotEmpty(t, writtenVars, "Expected some tracked writes from concurrent operations")

	t.Logf("Concurrent test completed with %d reads and %d writes tracked", len(readVars), len(writtenVars))
}
