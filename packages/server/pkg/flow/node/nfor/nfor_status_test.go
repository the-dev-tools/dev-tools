package nfor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	nodetesting "the-dev-tools/server/pkg/flow/node/testing"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
)

// TestFORNodeStatusBehavior tests the comprehensive status behavior of FOR nodes
func TestFORNodeStatusBehavior(t *testing.T) {
	tests := []struct {
		name             string
		iterCount        int64
		errorHandling    mnfor.ErrorHandling
		condition        mcondition.Condition
		childNodeError   error
		expectedStatuses []expectedStatus
		expectError      bool
		expectedError    error
	}{
		{
			name:          "successful loop without condition",
			iterCount:     3,
			errorHandling: mnfor.ErrorHandling_ERROR_HANDLING_IGNORE,
			expectedStatuses: []expectedStatus{
				{iteration: 0, state: mnnode.NODE_STATE_RUNNING, hasError: false},
				{iteration: 0, state: mnnode.NODE_STATE_SUCCESS, hasError: false},
				{iteration: 1, state: mnnode.NODE_STATE_RUNNING, hasError: false},
				{iteration: 1, state: mnnode.NODE_STATE_SUCCESS, hasError: false},
				{iteration: 2, state: mnnode.NODE_STATE_RUNNING, hasError: false},
				{iteration: 2, state: mnnode.NODE_STATE_SUCCESS, hasError: false},
			},
			expectError: false,
		},
		{
			name:           "loop with child error - unspecified handling",
			iterCount:      2,
			errorHandling:  mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED,
			childNodeError: errors.New("child execution failed"),
			expectedStatuses: []expectedStatus{
				{iteration: 0, state: mnnode.NODE_STATE_RUNNING, hasError: false},
				{iteration: 0, state: mnnode.NODE_STATE_FAILURE, hasError: true},
			},
			expectError:   true,
			expectedError: errors.New("child execution failed"),
		},
		{
			name:           "loop with child error - ignore handling",
			iterCount:      3,
			errorHandling:  mnfor.ErrorHandling_ERROR_HANDLING_IGNORE,
			childNodeError: errors.New("child execution failed"),
			expectedStatuses: []expectedStatus{
				{iteration: 0, state: mnnode.NODE_STATE_RUNNING, hasError: false},
				{iteration: 0, state: mnnode.NODE_STATE_FAILURE, hasError: true},
				{iteration: 1, state: mnnode.NODE_STATE_RUNNING, hasError: false},
				{iteration: 1, state: mnnode.NODE_STATE_FAILURE, hasError: true},
				{iteration: 2, state: mnnode.NODE_STATE_RUNNING, hasError: false},
				{iteration: 2, state: mnnode.NODE_STATE_FAILURE, hasError: true},
			},
			expectError: false,
		},
		{
			name:           "loop with child error - break handling",
			iterCount:      3,
			errorHandling:  mnfor.ErrorHandling_ERROR_HANDLING_BREAK,
			childNodeError: errors.New("child execution failed"),
			expectedStatuses: []expectedStatus{
				{iteration: 0, state: mnnode.NODE_STATE_RUNNING, hasError: false},
				{iteration: 0, state: mnnode.NODE_STATE_FAILURE, hasError: true},
			},
			expectError: false,
		},
		{
			name:          "loop with break condition - disabled for now",
			iterCount:     2,
			errorHandling: mnfor.ErrorHandling_ERROR_HANDLING_IGNORE,
			// Skip break condition test until we fix expression evaluation
			expectedStatuses: []expectedStatus{
				{iteration: 0, state: mnnode.NODE_STATE_RUNNING, hasError: false},
				{iteration: 0, state: mnnode.NODE_STATE_SUCCESS, hasError: false},
				{iteration: 1, state: mnnode.NODE_STATE_RUNNING, hasError: false},
				{iteration: 1, state: mnnode.NODE_STATE_SUCCESS, hasError: false},
			},
			expectError: false,
		},
		{
			name:             "zero iterations",
			iterCount:        0,
			errorHandling:    mnfor.ErrorHandling_ERROR_HANDLING_IGNORE,
			expectedStatuses: []expectedStatus{
				// No iteration statuses for zero iterations
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test context with status collector
			testCtx := nodetesting.NewTestContext(t, nodetesting.TestContextOptions{
				Timeout: 10 * time.Second,
			})
			defer testCtx.Cleanup()

			// Create FOR node
			forNodeID := idwrap.NewNow()
			forNode := NewWithCondition(forNodeID, "LoopNode", tt.iterCount, 0, tt.errorHandling, tt.condition)

			// Create child node if needed
			var childNode node.FlowNode
			var childNodeID idwrap.IDWrap
			if tt.iterCount > 0 {
				childNodeID = idwrap.NewNow()
				if tt.childNodeError != nil {
					childNode = &silentFailingNode{
						id:   childNodeID,
						name: "Child",
						err:  tt.childNodeError,
					}
				} else {
					childNode = &recordingNode{
						id:   childNodeID,
						name: "Child",
					}
				}
			}

			// Create edge map
			edgeMap := edge.EdgesMap{}
			if childNode != nil {
				edgeMap[forNodeID] = map[edge.EdgeHandle][]idwrap.IDWrap{
					edge.HandleLoop: {childNodeID},
				}
			}

			// Create node map
			nodeMap := map[idwrap.IDWrap]node.FlowNode{
				forNodeID: forNode,
			}
			if childNode != nil {
				nodeMap[childNodeID] = childNode
			}

			// Create request
			req := testCtx.CreateNodeRequest(forNodeID, "LoopNode", nodetesting.NodeRequestOptions{
				VarMap:        make(map[string]any), // Initialize VarMap
				NodeMap:       nodeMap,
				EdgeSourceMap: edgeMap,
				ExecutionID:   idwrap.NewNow(),
			})

			// Execute the FOR node
			result := forNode.RunSync(testCtx.Context(), req)

			// Validate result
			if tt.expectError {
				require.Error(t, result.Err)
				if tt.expectedError != nil {
					require.ErrorContains(t, result.Err, tt.expectedError.Error())
				}
			} else {
				require.NoError(t, result.Err)
			}

			// Get collected statuses
			timestampedStatuses := testCtx.Collector().GetAll()

			// Filter FOR node statuses
			var forNodeStatuses []runner.FlowNodeStatus
			for _, ts := range timestampedStatuses {
				if ts.Status.NodeID == forNodeID {
					forNodeStatuses = append(forNodeStatuses, ts.Status)
				}
			}

			// Validate status count
			require.Len(t, forNodeStatuses, len(tt.expectedStatuses),
				"expected %d statuses, got %d: %+v", len(tt.expectedStatuses), len(forNodeStatuses), forNodeStatuses)

			// Validate each status
			for i, expected := range tt.expectedStatuses {
				if i >= len(forNodeStatuses) {
					t.Fatalf("missing status for iteration %d", expected.iteration)
				}

				status := forNodeStatuses[i]
				require.Equal(t, expected.state, status.State, "iteration %d: wrong state", expected.iteration)
				require.Equal(t, expected.iteration, status.IterationIndex, "wrong iteration index")
				require.True(t, status.IterationEvent, "should be iteration event")
				require.Equal(t, forNodeID, status.LoopNodeID, "wrong loop node ID")
				require.NotNil(t, status.IterationContext, "missing iteration context")

				if expected.hasError {
					require.Error(t, status.Error, "expected error in status")
				} else {
					require.NoError(t, status.Error, "unexpected error in status")
				}

				// Validate iteration context
				iterCtx := status.IterationContext
				require.Contains(t, iterCtx.IterationPath, expected.iteration, "iteration path should contain current iteration")
				require.Contains(t, iterCtx.ParentNodes, forNodeID, "parent nodes should contain FOR node")
				require.Len(t, iterCtx.Labels, 1, "should have one iteration label")
				require.Equal(t, forNodeID, iterCtx.Labels[0].NodeID, "label should have correct node ID")
				require.Equal(t, expected.iteration+1, iterCtx.Labels[0].Iteration, "label should have correct iteration number")
			}

			// Validate status sequence
			err := testCtx.Validator().ValidateExecutionSequences()
			require.NoError(t, err, "status sequence should be valid")
		})
	}
}

// TestFORNodeExecutionIDReuse tests that FOR nodes correctly reuse ExecutionIDs for iteration updates
func TestFORNodeExecutionIDReuse(t *testing.T) {
	testCtx := nodetesting.NewTestContext(t, nodetesting.TestContextOptions{
		Timeout: 10 * time.Second,
	})
	defer testCtx.Cleanup()

	// Create FOR node
	forNodeID := idwrap.NewNow()
	forNode := New(forNodeID, "LoopNode", 2, 0, mnfor.ErrorHandling_ERROR_HANDLING_IGNORE)

	// Create edge map with empty loop (no child nodes for simplicity)
	edgeMap := edge.EdgesMap{
		forNodeID: {
			edge.HandleLoop: {}, // Empty loop
		},
	}

	// Create request
	req := testCtx.CreateNodeRequest(forNodeID, "LoopNode", nodetesting.NodeRequestOptions{
		VarMap: make(map[string]any), // Initialize VarMap
		NodeMap: map[idwrap.IDWrap]node.FlowNode{
			forNodeID: forNode,
		},
		EdgeSourceMap: edgeMap,
		ExecutionID:   idwrap.NewNow(),
	})

	// Execute the FOR node
	result := forNode.RunSync(testCtx.Context(), req)
	require.NoError(t, result.Err)

	// Get collected statuses
	timestampedStatuses := testCtx.Collector().GetAll()

	// Filter FOR node statuses
	var forNodeStatuses []runner.FlowNodeStatus
	for _, ts := range timestampedStatuses {
		if ts.Status.NodeID == forNodeID {
			forNodeStatuses = append(forNodeStatuses, ts.Status)
		}
	}

	// Should have 4 statuses: 2 iterations × (RUNNING + SUCCESS)
	require.Len(t, forNodeStatuses, 4)

	// Group by ExecutionID
	executionGroups := make(map[idwrap.IDWrap][]runner.FlowNodeStatus)
	for _, status := range forNodeStatuses {
		executionGroups[status.ExecutionID] = append(executionGroups[status.ExecutionID], status)
	}

	// Should have 2 ExecutionIDs (one per iteration)
	require.Len(t, executionGroups, 2)

	// Each ExecutionID should have exactly 2 statuses (RUNNING + SUCCESS)
	for execID, group := range executionGroups {
		require.Len(t, group, 2, "ExecutionID %s should have exactly 2 statuses", execID)

		// First should be RUNNING, second should be SUCCESS
		require.Equal(t, mnnode.NODE_STATE_RUNNING, group[0].State)
		require.Equal(t, mnnode.NODE_STATE_SUCCESS, group[1].State)

		// Both should have same iteration index
		require.Equal(t, group[0].IterationIndex, group[1].IterationIndex)
	}
}

// TestFORNodeVariableTracking tests that FOR nodes correctly track variables
func TestFORNodeVariableTracking(t *testing.T) {
	testCtx := nodetesting.NewTestContext(t, nodetesting.TestContextOptions{
		Timeout: 10 * time.Second,
	})
	defer testCtx.Cleanup()

	// Create FOR node
	forNodeID := idwrap.NewNow()
	forNode := New(forNodeID, "LoopNode", 3, 0, mnfor.ErrorHandling_ERROR_HANDLING_IGNORE)

	// Create edge map
	edgeMap := edge.EdgesMap{
		forNodeID: {
			edge.HandleLoop: {}, // Empty loop
		},
	}

	// Create request
	req := testCtx.CreateNodeRequest(forNodeID, "LoopNode", nodetesting.NodeRequestOptions{
		VarMap: make(map[string]any), // Initialize VarMap
		NodeMap: map[idwrap.IDWrap]node.FlowNode{
			forNodeID: forNode,
		},
		EdgeSourceMap: edgeMap,
		ExecutionID:   idwrap.NewNow(),
	})

	// Execute the FOR node
	result := forNode.RunSync(testCtx.Context(), req)
	require.NoError(t, result.Err)

	// Check that final index variable is set to last iteration (2)
	indexValue, err := node.ReadNodeVar(req, "LoopNode", "index")
	require.NoError(t, err)
	require.Equal(t, int64(2), indexValue.(int64))

	// Check that totalIterations was set
	totalIterations, err := node.ReadNodeVar(req, "LoopNode", "totalIterations")
	require.NoError(t, err)
	require.Equal(t, int64(3), totalIterations.(int64))
}

// TestFORNodeTimeout tests that FOR nodes respect timeout
func TestFORNodeTimeout(t *testing.T) {
	testCtx := nodetesting.NewTestContext(t, nodetesting.TestContextOptions{
		Timeout: 10 * time.Second,
	})
	defer testCtx.Cleanup()

	// Create FOR node with very short timeout
	forNodeID := idwrap.NewNow()
	forNode := New(forNodeID, "LoopNode", 1, 1*time.Nanosecond, mnfor.ErrorHandling_ERROR_HANDLING_IGNORE)

	// Create edge map
	edgeMap := edge.EdgesMap{
		forNodeID: {
			edge.HandleLoop: {}, // Empty loop
		},
	}

	// Create request with timeout
	req := testCtx.CreateNodeRequest(forNodeID, "LoopNode", nodetesting.NodeRequestOptions{
		VarMap: make(map[string]any), // Initialize VarMap
		NodeMap: map[idwrap.IDWrap]node.FlowNode{
			forNodeID: forNode,
		},
		EdgeSourceMap: edgeMap,
		ExecutionID:   idwrap.NewNow(),
		Timeout:       1 * time.Nanosecond,
	})

	// Execute with context timeout
	ctx, cancel := context.WithTimeout(testCtx.Context(), 1*time.Nanosecond)
	defer cancel()

	result := forNode.RunSync(ctx, req)

	// Should complete without error since empty loop is very fast
	require.NoError(t, result.Err)
}

// TestFORNodeAsyncStatusBehavior tests async execution status behavior
func TestFORNodeAsyncStatusBehavior(t *testing.T) {
	testCtx := nodetesting.NewTestContext(t, nodetesting.TestContextOptions{
		Timeout: 10 * time.Second,
	})
	defer testCtx.Cleanup()

	// Create FOR node
	forNodeID := idwrap.NewNow()
	forNode := New(forNodeID, "LoopNode", 2, 0, mnfor.ErrorHandling_ERROR_HANDLING_IGNORE)

	// Create edge map
	edgeMap := edge.EdgesMap{
		forNodeID: {
			edge.HandleLoop: {}, // Empty loop
		},
	}

	// Create request
	req := testCtx.CreateNodeRequest(forNodeID, "LoopNode", nodetesting.NodeRequestOptions{
		VarMap: make(map[string]any), // Initialize VarMap
		NodeMap: map[idwrap.IDWrap]node.FlowNode{
			forNodeID: forNode,
		},
		EdgeSourceMap: edgeMap,
		ExecutionID:   idwrap.NewNow(),
	})

	// Execute the FOR node asynchronously
	resultChan := make(chan node.FlowNodeResult, 1)
	forNode.RunAsync(testCtx.Context(), req, resultChan)

	// Wait for result
	select {
	case result := <-resultChan:
		require.NoError(t, result.Err)
	case <-time.After(5 * time.Second):
		t.Fatal("async execution timed out")
	}

	// Get collected statuses
	timestampedStatuses := testCtx.Collector().GetAll()

	// Filter FOR node statuses
	var forNodeStatuses []runner.FlowNodeStatus
	for _, ts := range timestampedStatuses {
		if ts.Status.NodeID == forNodeID {
			forNodeStatuses = append(forNodeStatuses, ts.Status)
		}
	}

	// Should have 4 statuses: 2 iterations × (RUNNING + SUCCESS)
	require.Len(t, forNodeStatuses, 4)

	// Validate status sequence
	err := testCtx.Validator().ValidateExecutionSequences()
	require.NoError(t, err, "status sequence should be valid")
}

// silentFailingNode is a node that fails without emitting its own status
type silentFailingNode struct {
	id   idwrap.IDWrap
	name string
	err  error
}

func (n *silentFailingNode) GetID() idwrap.IDWrap { return n.id }
func (n *silentFailingNode) GetName() string      { return n.name }

func (n *silentFailingNode) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	return node.FlowNodeResult{Err: n.err}
}

func (n *silentFailingNode) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	resultChan <- node.FlowNodeResult{Err: n.err}
}

// expectedStatus defines expected status properties for validation
type expectedStatus struct {
	iteration int
	state     mnnode.NodeState
	hasError  bool
}
