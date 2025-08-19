package nfor_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/node/nfor"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
)

// SlowNode simulates a node that takes time to execute
type SlowNode struct {
	ID             idwrap.IDWrap
	Name           string
	ExecutionCount atomic.Int32
	ExecutionDelay time.Duration
}

func (n *SlowNode) GetID() idwrap.IDWrap {
	return n.ID
}

func (n *SlowNode) GetName() string {
	return n.Name
}

func (n *SlowNode) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	n.ExecutionCount.Add(1)

	// Simulate slow execution
	select {
	case <-time.After(n.ExecutionDelay):
		// Normal completion
	case <-ctx.Done():
		// Context cancelled
		return node.FlowNodeResult{
			Err: ctx.Err(),
		}
	}

	// Get next nodes
	nextNodes := edge.GetNextNodeID(req.EdgeSourceMap, n.ID, edge.HandleUnspecified)

	return node.FlowNodeResult{
		NextNodeID: nextNodes,
	}
}

func (n *SlowNode) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	go func() {
		result := n.RunSync(ctx, req)
		resultChan <- result
	}()
}

// TestForNodeAsyncWaitsForChildCompletion verifies that FOR node waits for async child executions
func TestForNodeAsyncWaitsForChildCompletion(t *testing.T) {
	// Setup nodes
	forNodeID := idwrap.NewNow()
	slowNodeID := idwrap.NewNow()

	// Create FOR node with 3 iterations
	forNode := nfor.New(forNodeID, "test_for", 3, 10*time.Second, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)

	// Create slow child node (50ms per execution)
	slowNode := &SlowNode{
		ID:             slowNodeID,
		Name:           "slow_node",
		ExecutionDelay: 50 * time.Millisecond,
	}

	// Setup edges
	edges := []edge.Edge{
		edge.NewEdge(idwrap.NewNow(), forNodeID, slowNodeID, edge.HandleLoop, edge.EdgeKindUnspecified),
	}
	edgeMap := edge.NewEdgesMap(edges)

	// Setup node map
	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		forNodeID:  forNode,
		slowNodeID: slowNode,
	}

	// Setup request
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		NodeMap:       nodeMap,
		EdgeSourceMap: edgeMap,
		Timeout:       5 * time.Second, // This makes FOR node use async
		ReadWriteLock: &sync.RWMutex{},
		LogPushFunc: func(status runner.FlowNodeStatus) {
			// Just consume status updates
		},
	}

	// Create result channel for async execution
	resultChan := make(chan node.FlowNodeResult, 1)

	// Run FOR node asynchronously
	ctx := context.Background()
	forNode.RunAsync(ctx, req, resultChan)

	// Wait for FOR node to complete
	select {
	case result := <-resultChan:
		if result.Err != nil {
			t.Fatalf("FOR node failed: %v", result.Err)
		}

		// Verify all child executions completed
		expectedCount := int32(3)
		actualCount := slowNode.ExecutionCount.Load()
		if actualCount != expectedCount {
			t.Errorf("Expected %d executions, got %d", expectedCount, actualCount)
			t.Logf("This indicates FOR node didn't wait for all async child executions to complete")
		}

	case <-time.After(2 * time.Second):
		t.Fatal("FOR node execution timed out")
	}
}

// TestNestedForAsyncExecution tests nested FOR loops with async execution
func TestNestedForAsyncExecution(t *testing.T) {
	// Setup nodes
	outerForID := idwrap.NewNow()
	innerForID := idwrap.NewNow()
	slowNodeID := idwrap.NewNow()

	// Create outer FOR node (2 iterations)
	outerFor := nfor.New(outerForID, "outer_for", 2, 10*time.Second, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)

	// Create inner FOR node (3 iterations)
	innerFor := nfor.New(innerForID, "inner_for", 3, 10*time.Second, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)

	// Create slow child node (10ms per execution)
	slowNode := &SlowNode{
		ID:             slowNodeID,
		Name:           "slow_node",
		ExecutionDelay: 10 * time.Millisecond,
	}

	// Setup edges: outerFor -> innerFor -> slowNode
	edges := []edge.Edge{
		edge.NewEdge(idwrap.NewNow(), outerForID, innerForID, edge.HandleLoop, edge.EdgeKindUnspecified),
		edge.NewEdge(idwrap.NewNow(), innerForID, slowNodeID, edge.HandleLoop, edge.EdgeKindUnspecified),
	}
	edgeMap := edge.NewEdgesMap(edges)

	// Setup node map
	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		outerForID: outerFor,
		innerForID: innerFor,
		slowNodeID: slowNode,
	}

	// Setup request
	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]interface{}),
		NodeMap:       nodeMap,
		EdgeSourceMap: edgeMap,
		Timeout:       5 * time.Second, // This makes FOR nodes use async
		ReadWriteLock: &sync.RWMutex{},
		LogPushFunc: func(status runner.FlowNodeStatus) {
			// Just consume status updates
		},
	}

	// Create result channel for async execution
	resultChan := make(chan node.FlowNodeResult, 1)

	// Run outer FOR node asynchronously
	ctx := context.Background()
	outerFor.RunAsync(ctx, req, resultChan)

	// Wait for outer FOR node to complete
	select {
	case result := <-resultChan:
		if result.Err != nil {
			t.Fatalf("Outer FOR node failed: %v", result.Err)
		}

		// Verify all nested executions completed (2 * 3 = 6)
		expectedCount := int32(6)
		actualCount := slowNode.ExecutionCount.Load()
		if actualCount != expectedCount {
			t.Errorf("Expected %d executions, got %d", expectedCount, actualCount)
			t.Logf("Missing executions indicate async synchronization issue in nested FOR loops")
		}

	case <-time.After(2 * time.Second):
		t.Fatal("Nested FOR execution timed out")
	}
}
