package nforeach

import (
	"context"
	"sync"
	"testing"
	"time"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForEachNode_ArrayIteration_OutputFormat(t *testing.T) {
	// Test array iteration outputs index and value
	forEachNode := New(
		idwrap.NewNow(), 
		"testForEach", 
		"testArray", 
		time.Second*2,
		mcondition.Condition{},
		mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED,
	)
	
	var loggedStatuses []runner.FlowNodeStatus
	var mu sync.Mutex
	logFunc := func(status runner.FlowNodeStatus) {
		mu.Lock()
		loggedStatuses = append(loggedStatuses, status)
		mu.Unlock()
	}
	
	// Set up test array
	testArray := []any{"apple", "banana", "cherry"}
	req := &node.FlowNodeRequest{
		VarMap: map[string]any{
			"testArray": testArray,
		},
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       make(map[idwrap.IDWrap]node.FlowNode),
		EdgeSourceMap: make(edge.EdgesMap),
		LogPushFunc:   logFunc,
	}
	
	result := forEachNode.RunSync(context.Background(), req)
	require.NoError(t, result.Err)
	
	// Filter iteration records
	var iterationRecords []runner.FlowNodeStatus
	for _, status := range loggedStatuses {
		if status.State == mnnode.NODE_STATE_RUNNING && status.OutputData != nil {
			outputMap, ok := status.OutputData.(map[string]any)
			if ok && (outputMap["index"] != nil || outputMap["key"] != nil) {
				iterationRecords = append(iterationRecords, status)
			}
		}
	}
	
	// Should have 3 iteration records
	assert.Len(t, iterationRecords, 3)
	
	// Check each iteration
	for i, record := range iterationRecords {
		outputMap, ok := record.OutputData.(map[string]any)
		require.True(t, ok)
		
		// Should have exactly "index" and "value"
		assert.Len(t, outputMap, 2, "Should have exactly 2 fields")
		
		index, hasIndex := outputMap["index"]
		assert.True(t, hasIndex, "Should have 'index' field")
		assert.Equal(t, i, index, "Index should match")
		
		value, hasValue := outputMap["value"]
		assert.True(t, hasValue, "Should have 'value' field")
		assert.Equal(t, testArray[i], value, "Value should match array item")
		
		// Should NOT have "key" for array iteration
		_, hasKey := outputMap["key"]
		assert.False(t, hasKey, "Should NOT have 'key' for array iteration")
	}
}

func TestForEachNode_MapIteration_OutputFormat(t *testing.T) {
	// Test map iteration outputs key and value
	forEachNode := New(
		idwrap.NewNow(), 
		"testForEach", 
		"testMap", 
		time.Second*2,
		mcondition.Condition{},
		mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED,
	)
	
	var loggedStatuses []runner.FlowNodeStatus
	var mu sync.Mutex
	logFunc := func(status runner.FlowNodeStatus) {
		mu.Lock()
		loggedStatuses = append(loggedStatuses, status)
		mu.Unlock()
	}
	
	// Set up test map
	testMap := map[string]any{
		"first": "apple",
		"second": "banana",
		"third": "cherry",
	}
	req := &node.FlowNodeRequest{
		VarMap: map[string]any{
			"testMap": testMap,
		},
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       make(map[idwrap.IDWrap]node.FlowNode),
		EdgeSourceMap: make(edge.EdgesMap),
		LogPushFunc:   logFunc,
	}
	
	result := forEachNode.RunSync(context.Background(), req)
	require.NoError(t, result.Err)
	
	// Filter iteration records
	var iterationRecords []runner.FlowNodeStatus
	for _, status := range loggedStatuses {
		if status.State == mnnode.NODE_STATE_RUNNING && status.OutputData != nil {
			outputMap, ok := status.OutputData.(map[string]any)
			if ok && outputMap["key"] != nil {
				iterationRecords = append(iterationRecords, status)
			}
		}
	}
	
	// Should have 3 iteration records
	assert.Len(t, iterationRecords, 3)
	
	// Collect all keys to verify
	seenKeys := make(map[string]bool)
	
	for _, record := range iterationRecords {
		outputMap, ok := record.OutputData.(map[string]any)
		require.True(t, ok)
		
		// Should have exactly "key" and "value"
		assert.Len(t, outputMap, 2, "Should have exactly 2 fields")
		
		key, hasKey := outputMap["key"]
		assert.True(t, hasKey, "Should have 'key' field")
		keyStr, ok := key.(string)
		require.True(t, ok, "Key should be string")
		
		value, hasValue := outputMap["value"]
		assert.True(t, hasValue, "Should have 'value' field")
		
		// Verify key-value pair matches original map
		expectedValue, exists := testMap[keyStr]
		assert.True(t, exists, "Key should exist in original map")
		assert.Equal(t, expectedValue, value, "Value should match")
		
		// Should NOT have "index" for map iteration
		_, hasIndex := outputMap["index"]
		assert.False(t, hasIndex, "Should NOT have 'index' for map iteration")
		
		seenKeys[keyStr] = true
	}
	
	// Should have seen all keys
	assert.Len(t, seenKeys, len(testMap))
}

func TestForEachNode_EmptyCollection_OutputFormat(t *testing.T) {
	testCases := []struct {
		name  string
		value any
	}{
		{"EmptyArray", []any{}},
		{"EmptyMap", map[string]any{}},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			forEachNode := New(
				idwrap.NewNow(), 
				"testForEach", 
				"testValue", 
				time.Second*2,
				mcondition.Condition{},
				mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED,
			)
			
			var loggedStatuses []runner.FlowNodeStatus
			logFunc := func(status runner.FlowNodeStatus) {
				loggedStatuses = append(loggedStatuses, status)
			}
			
			req := &node.FlowNodeRequest{
				VarMap: map[string]any{
					"testValue": tc.value,
				},
				ReadWriteLock: &sync.RWMutex{},
				NodeMap:       make(map[idwrap.IDWrap]node.FlowNode),
				EdgeSourceMap: make(edge.EdgesMap),
				LogPushFunc:   logFunc,
			}
			
			result := forEachNode.RunSync(context.Background(), req)
			assert.NoError(t, result.Err)
			
			// Should have no iteration records for empty collections
			iterationCount := 0
			for _, status := range loggedStatuses {
				if status.OutputData != nil {
					outputMap, ok := status.OutputData.(map[string]any)
					if ok && (outputMap["index"] != nil || outputMap["key"] != nil) {
						iterationCount++
					}
				}
			}
			assert.Equal(t, 0, iterationCount)
		})
	}
}

func TestForEachNode_MixedTypeArray_OutputFormat(t *testing.T) {
	// Test array with mixed types
	forEachNode := New(
		idwrap.NewNow(), 
		"testForEach", 
		"mixedArray", 
		time.Second*2,
		mcondition.Condition{},
		mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED,
	)
	
	var loggedStatuses []runner.FlowNodeStatus
	logFunc := func(status runner.FlowNodeStatus) {
		loggedStatuses = append(loggedStatuses, status)
	}
	
	// Mixed type array
	mixedArray := []any{
		"string",
		123,
		45.67,
		true,
		map[string]any{"nested": "object"},
		[]any{1, 2, 3},
	}
	
	req := &node.FlowNodeRequest{
		VarMap: map[string]any{
			"mixedArray": mixedArray,
		},
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       make(map[idwrap.IDWrap]node.FlowNode),
		EdgeSourceMap: make(edge.EdgesMap),
		LogPushFunc:   logFunc,
	}
	
	result := forEachNode.RunSync(context.Background(), req)
	require.NoError(t, result.Err)
	
	// Count and verify iteration records
	iterationCount := 0
	for _, status := range loggedStatuses {
		if status.OutputData != nil {
			outputMap, ok := status.OutputData.(map[string]any)
			if ok && outputMap["index"] != nil {
				iterationCount++
				
				// Verify value is preserved correctly
				index := outputMap["index"].(int)
				value := outputMap["value"]
				assert.Equal(t, mixedArray[index], value, "Value type should be preserved")
			}
		}
	}
	
	assert.Equal(t, len(mixedArray), iterationCount)
}

func TestForEachNode_AsyncConsistency(t *testing.T) {
	// Test that async produces same output format as sync
	forEachNode := New(
		idwrap.NewNow(), 
		"testForEach", 
		"testData", 
		time.Second*2,
		mcondition.Condition{},
		mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED,
	)
	
	testCases := []struct {
		name     string
		data     any
		checkKey bool
	}{
		{"Array", []any{"a", "b", "c"}, false},
		{"Map", map[string]any{"x": 1, "y": 2}, true},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var loggedStatuses []runner.FlowNodeStatus
			var mu sync.Mutex
			logFunc := func(status runner.FlowNodeStatus) {
				mu.Lock()
				loggedStatuses = append(loggedStatuses, status)
				mu.Unlock()
			}
			
			req := &node.FlowNodeRequest{
				VarMap: map[string]any{
					"testData": tc.data,
				},
				ReadWriteLock: &sync.RWMutex{},
				NodeMap:       make(map[idwrap.IDWrap]node.FlowNode),
				EdgeSourceMap: make(edge.EdgesMap),
				LogPushFunc:   logFunc,
			}
			
			resultChan := make(chan node.FlowNodeResult)
			forEachNode.RunAsync(context.Background(), req, resultChan)
			
			result := <-resultChan
			require.NoError(t, result.Err)
			
			// Verify output format consistency
			for _, status := range loggedStatuses {
				if status.OutputData != nil {
					outputMap, ok := status.OutputData.(map[string]any)
					if ok {
						if tc.checkKey {
							// Map iteration
							if outputMap["key"] != nil {
								assert.NotNil(t, outputMap["value"], "Map iteration should have value")
								assert.Nil(t, outputMap["index"], "Map iteration should not have index")
							}
						} else {
							// Array iteration
							if outputMap["index"] != nil {
								assert.NotNil(t, outputMap["value"], "Array iteration should have value")
								assert.Nil(t, outputMap["key"], "Array iteration should not have key")
							}
						}
					}
				}
			}
		})
	}
}