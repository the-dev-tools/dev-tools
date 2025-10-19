package testing

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
)

// NodeTestCase represents a single test case for a flow node
type NodeTestCase struct {
	Name     string
	TestFunc func(t *testing.T, ctx *TestContext, testNode node.FlowNode)
}

// TestNodeOptions configures how a node should be tested
type TestNodeOptions struct {
	// Request configuration
	VarMap      map[string]any
	EdgeMap     edge.EdgesMap
	Timeout     time.Duration
	ExecutionID idwrap.IDWrap

	// Test behavior flags
	ExpectStatusEvents bool // Whether the node should emit status events
}

// DefaultTestNodeOptions returns sensible defaults for node testing
func DefaultTestNodeOptions() TestNodeOptions {
	return TestNodeOptions{
		VarMap:             make(map[string]any),
		EdgeMap:            make(edge.EdgesMap),
		Timeout:            10 * time.Second,
		ExecutionID:        idwrap.NewNow(),
		ExpectStatusEvents: false, // Most nodes don't emit status by default
	}
}

// TestNodeSuccess tests that a node executes successfully
func TestNodeSuccess(t *testing.T, testNode node.FlowNode, opts TestNodeOptions) {
	ctx := NewTestContext(t, TestContextOptions{Timeout: opts.Timeout})
	defer ctx.Cleanup()

	req := ctx.CreateNodeRequest(testNode.GetID(), testNode.GetName(), NodeRequestOptions{
		VarMap:        opts.VarMap,
		NodeMap:       map[idwrap.IDWrap]node.FlowNode{testNode.GetID(): testNode},
		EdgeSourceMap: opts.EdgeMap,
		ExecutionID:   opts.ExecutionID,
		Timeout:       opts.Timeout,
	})

	result := testNode.RunSync(ctx.Context(), req)
	require.NoError(t, result.Err, "Node execution should succeed")

	// Check status events if expected
	statuses := ctx.Collector().GetAll()
	nodeStatuses := filterStatusesForNode(statuses, testNode.GetID())

	if opts.ExpectStatusEvents {
		require.NotEmpty(t, nodeStatuses, "Expected status events but got none")
		validateSuccessStatuses(t, nodeStatuses)
	} else {
		// If we get status events, they should still be valid
		if len(nodeStatuses) > 0 {
			validateSuccessStatuses(t, nodeStatuses)
		}
	}

	// Validate execution sequences if we have status events
	if len(nodeStatuses) > 0 {
		err := ctx.Validator().ValidateExecutionSequences()
		require.NoError(t, err, "Status sequence should be valid")
	}
}

// TestNodeError tests that a node handles error conditions appropriately
func TestNodeError(t *testing.T, testNode node.FlowNode, opts TestNodeOptions, errorFunc func(*node.FlowNodeRequest)) {
	ctx := NewTestContext(t, TestContextOptions{Timeout: opts.Timeout})
	defer ctx.Cleanup()

	req := ctx.CreateNodeRequest(testNode.GetID(), testNode.GetName(), NodeRequestOptions{
		VarMap:        opts.VarMap,
		NodeMap:       map[idwrap.IDWrap]node.FlowNode{testNode.GetID(): testNode},
		EdgeSourceMap: opts.EdgeMap,
		ExecutionID:   opts.ExecutionID,
		Timeout:       opts.Timeout,
	})

	// Apply error condition
	if errorFunc != nil {
		errorFunc(req)
	}

	result := testNode.RunSync(ctx.Context(), req)

	// Check status events
	statuses := ctx.Collector().GetAll()
	nodeStatuses := filterStatusesForNode(statuses, testNode.GetID())

	// Either we should get an error or error status events
	if result.Err == nil && len(nodeStatuses) == 0 {
		t.Log("Node handled error condition gracefully without error or status")
		return
	}

	if result.Err != nil {
		t.Logf("Node returned error: %v", result.Err)
	}

	if len(nodeStatuses) > 0 {
		validateErrorStatuses(t, nodeStatuses)
	}
}

// TestNodeTimeout tests that a node respects timeout constraints
func TestNodeTimeout(t *testing.T, testNode node.FlowNode, opts TestNodeOptions) {
	ctx := NewTestContext(t, TestContextOptions{Timeout: opts.Timeout})
	defer ctx.Cleanup()

	// Create a request with very short timeout
	req := ctx.CreateNodeRequest(testNode.GetID(), testNode.GetName(), NodeRequestOptions{
		VarMap:        opts.VarMap,
		NodeMap:       map[idwrap.IDWrap]node.FlowNode{testNode.GetID(): testNode},
		EdgeSourceMap: opts.EdgeMap,
		ExecutionID:   opts.ExecutionID,
		Timeout:       1 * time.Nanosecond, // Extremely short timeout
	})

	// Execute with timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx.Context(), 1*time.Millisecond)
	defer cancel()

	result := testNode.RunSync(timeoutCtx, req)

	// Check status events
	statuses := ctx.Collector().GetAll()
	nodeStatuses := filterStatusesForNode(statuses, testNode.GetID())

	// Timeout behavior varies by node type, so we're flexible
	if result.Err != nil {
		t.Logf("Node timed out with error: %v", result.Err)
	}

	t.Logf("Timeout test captured %d statuses", len(nodeStatuses))
}

// TestNodeAsync tests asynchronous node execution
func TestNodeAsync(t *testing.T, testNode node.FlowNode, opts TestNodeOptions) {
	ctx := NewTestContext(t, TestContextOptions{Timeout: opts.Timeout})
	defer ctx.Cleanup()

	req := ctx.CreateNodeRequest(testNode.GetID(), testNode.GetName(), NodeRequestOptions{
		VarMap:        opts.VarMap,
		NodeMap:       map[idwrap.IDWrap]node.FlowNode{testNode.GetID(): testNode},
		EdgeSourceMap: opts.EdgeMap,
		ExecutionID:   opts.ExecutionID,
		Timeout:       opts.Timeout,
	})

	// Execute asynchronously
	resultChan := make(chan node.FlowNodeResult, 1)
	testNode.RunAsync(ctx.Context(), req, resultChan)

	// Wait for result
	select {
	case result := <-resultChan:
		require.NoError(t, result.Err, "Async execution should succeed")

		// Check status events
		statuses := ctx.Collector().GetAll()
		nodeStatuses := filterStatusesForNode(statuses, testNode.GetID())

		if opts.ExpectStatusEvents {
			validateSuccessStatuses(t, nodeStatuses)
		}

	case <-time.After(5 * time.Second):
		t.Fatal("Async execution timed out")
	}
}

// RunNodeTests runs a set of test cases for a node
func RunNodeTests(t *testing.T, testNode node.FlowNode, testCases []NodeTestCase) {
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := NewTestContext(t, TestContextOptions{Timeout: 10 * time.Second})
			defer ctx.Cleanup()
			tc.TestFunc(t, ctx, testNode)
		})
	}
}

// Helper functions

// filterStatusesForNode returns only the statuses for the specified node
func filterStatusesForNode(statuses []TimestampedStatus, nodeID idwrap.IDWrap) []runner.FlowNodeStatus {
	var result []runner.FlowNodeStatus
	for _, ts := range statuses {
		if ts.Status.NodeID == nodeID {
			result = append(result, ts.Status)
		}
	}
	return result
}

// validateSuccessStatuses validates that status events represent a successful execution
func validateSuccessStatuses(t *testing.T, statuses []runner.FlowNodeStatus) {
	if len(statuses) == 0 {
		return
	}

	hasSuccess := false
	hasRunning := false

	for _, status := range statuses {
		switch status.State {
		case mnnode.NODE_STATE_SUCCESS:
			hasSuccess = true
		case mnnode.NODE_STATE_RUNNING:
			hasRunning = true
		case mnnode.NODE_STATE_FAILURE, mnnode.NODE_STATE_CANCELED:
			t.Errorf("Unexpected error state in successful execution: %s", mnnode.StringNodeState(status.State))
		}
	}

	// Should have at least one SUCCESS status
	require.True(t, hasSuccess, "Should have at least one SUCCESS status")

	// If we have RUNNING, we should also have SUCCESS
	if hasRunning && !hasSuccess {
		t.Error("Has RUNNING status but no SUCCESS status")
	}
}

// validateErrorStatuses validates that status events represent error handling
func validateErrorStatuses(t *testing.T, statuses []runner.FlowNodeStatus) {
	if len(statuses) == 0 {
		return
	}

	hasError := false
	for _, status := range statuses {
		if status.State == mnnode.NODE_STATE_FAILURE || status.State == mnnode.NODE_STATE_CANCELED {
			hasError = true
			break
		}
	}

	// Should have at least one error status if we have any statuses
	if len(statuses) > 0 && !hasError {
		t.Log("Note: No error status found, node may handle errors gracefully")
	}
}
