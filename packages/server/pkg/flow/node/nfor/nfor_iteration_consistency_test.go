package nfor

import (
	"context"
	"sync"
	"testing"
	"time"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForNode_IterationOutput_BasicCase(t *testing.T) {
	// Test that FOR node outputs "index" not "iteration"
	forNode := New(idwrap.NewNow(), "testFor", 3, time.Second*60, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)
	
	var loggedStatuses []runner.FlowNodeStatus
	var mu sync.Mutex
	logFunc := func(status runner.FlowNodeStatus) {
		mu.Lock()
		loggedStatuses = append(loggedStatuses, status)
		mu.Unlock()
	}
	
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       make(map[idwrap.IDWrap]node.FlowNode),
		EdgeSourceMap: make(edge.EdgesMap),
		LogPushFunc:   logFunc,
	}
	
	result := forNode.RunSync(context.Background(), req)
	require.NoError(t, result.Err)
	
	// Filter iteration tracking records
	var iterationRecords []runner.FlowNodeStatus
	for _, status := range loggedStatuses {
		if status.State == mnnode.NODE_STATE_RUNNING && status.OutputData != nil {
			iterationRecords = append(iterationRecords, status)
		}
	}
	
	// Should have exactly 3 iteration records
	assert.Len(t, iterationRecords, 3)
	
	// Check each iteration has correct format
	for i, record := range iterationRecords {
		assert.NotNil(t, record.OutputData)
		
		// Should have "index" field, not "iteration"
		outputMap, ok := record.OutputData.(map[string]any)
		require.True(t, ok, "OutputData should be map[string]any")
		
		index, hasIndex := outputMap["index"]
		assert.True(t, hasIndex, "Should have 'index' field")
		assert.Equal(t, int64(i), index, "Index should match iteration number")
		
		_, hasIteration := outputMap["iteration"]
		assert.False(t, hasIteration, "Should NOT have 'iteration' field")
		
		// Should only have "index" field
		assert.Len(t, outputMap, 1, "Should only have one field")
	}
}

func TestForNode_IterationOutput_ZeroIterations(t *testing.T) {
	// Edge case: 0 iterations
	forNode := New(idwrap.NewNow(), "testFor", 0, time.Second*60, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)
	
	var loggedStatuses []runner.FlowNodeStatus
	logFunc := func(status runner.FlowNodeStatus) {
		loggedStatuses = append(loggedStatuses, status)
	}
	
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       make(map[idwrap.IDWrap]node.FlowNode),
		EdgeSourceMap: make(edge.EdgesMap),
		LogPushFunc:   logFunc,
	}
	
	result := forNode.RunSync(context.Background(), req)
	require.NoError(t, result.Err)
	
	// Should have no iteration tracking records
	iterationCount := 0
	for _, status := range loggedStatuses {
		if status.OutputData != nil {
			if outputMap, ok := status.OutputData.(map[string]any); ok {
				if _, hasIndex := outputMap["index"]; hasIndex {
					iterationCount++
				}
			}
		}
	}
	assert.Equal(t, 0, iterationCount)
}

func TestForNode_IterationOutput_WithError(t *testing.T) {
	// Test error handling modes
	testCases := []struct {
		name          string
		errorHandling mnfor.ErrorHandling
		expectRecords int
	}{
		{"Ignore", mnfor.ErrorHandling_ERROR_HANDLING_IGNORE, 3},
		{"Break", mnfor.ErrorHandling_ERROR_HANDLING_BREAK, 3},
		{"Unspecified", mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED, 3},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			forNode := New(idwrap.NewNow(), "testFor", 3, time.Second*60, tc.errorHandling)
			
			var loggedStatuses []runner.FlowNodeStatus
			logFunc := func(status runner.FlowNodeStatus) {
				loggedStatuses = append(loggedStatuses, status)
			}
			
			req := &node.FlowNodeRequest{
				VarMap:        make(map[string]any),
				ReadWriteLock: &sync.RWMutex{},
				NodeMap:       make(map[idwrap.IDWrap]node.FlowNode),
				EdgeSourceMap: make(edge.EdgesMap),
				LogPushFunc:   logFunc,
			}
			
			_ = forNode.RunSync(context.Background(), req)
			
			// Count iteration records
			iterationCount := 0
			for _, status := range loggedStatuses {
				if status.OutputData != nil {
					if outputMap, ok := status.OutputData.(map[string]any); ok {
						if _, hasIndex := outputMap["index"]; hasIndex {
							iterationCount++
						}
					}
				}
			}
			
			// Should log all iterations regardless of error handling
			assert.Equal(t, tc.expectRecords, iterationCount)
		})
	}
}

func TestForNode_IterationOutput_AsyncConsistency(t *testing.T) {
	// Test that async behaves same as sync
	forNode := New(idwrap.NewNow(), "testFor", 3, time.Second*60, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)
	
	var loggedStatuses []runner.FlowNodeStatus
	var mu sync.Mutex
	logFunc := func(status runner.FlowNodeStatus) {
		mu.Lock()
		loggedStatuses = append(loggedStatuses, status)
		mu.Unlock()
	}
	
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       make(map[idwrap.IDWrap]node.FlowNode),
		EdgeSourceMap: make(edge.EdgesMap),
		LogPushFunc:   logFunc,
	}
	
	resultChan := make(chan node.FlowNodeResult)
	forNode.RunAsync(context.Background(), req, resultChan)
	
	result := <-resultChan
	require.NoError(t, result.Err)
	
	// Check all iteration records have "index" not "iteration"
	for _, status := range loggedStatuses {
		if status.OutputData != nil {
			if outputMap, ok := status.OutputData.(map[string]any); ok {
				if _, hasIndex := outputMap["index"]; hasIndex {
					_, hasIteration := outputMap["iteration"]
					assert.False(t, hasIteration, "Should NOT have 'iteration' field in async")
				}
			}
		}
	}
}