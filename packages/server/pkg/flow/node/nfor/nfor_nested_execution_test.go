package nfor_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/node/nfor"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/flow/runner/flowlocalrunner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"time"
)

// CountingNode is a test node that counts executions
type CountingNode struct {
	ID             idwrap.IDWrap
	Name           string
	ExecutionCount atomic.Int32
	mu             sync.Mutex
	executions     []string // Track execution details
}

func NewCountingNode(id idwrap.IDWrap, name string) *CountingNode {
	return &CountingNode{
		ID:         id,
		Name:       name,
		executions: make([]string, 0),
	}
}

func (n *CountingNode) GetID() idwrap.IDWrap {
	return n.ID
}

func (n *CountingNode) GetName() string {
	return n.Name
}

func (n *CountingNode) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	count := n.ExecutionCount.Add(1)
	
	// Build execution detail string
	detail := fmt.Sprintf("Execution %d", count)
	if req.IterationContext != nil && len(req.IterationContext.IterationPath) > 0 {
		detail += fmt.Sprintf(" (path: %v)", req.IterationContext.IterationPath)
	}
	
	n.mu.Lock()
	n.executions = append(n.executions, detail)
	n.mu.Unlock()
	
	// Simulate some work
	time.Sleep(1 * time.Millisecond)
	
	// Get next nodes from edge map
	nextNodes := edge.GetNextNodeID(req.EdgeSourceMap, n.ID, edge.HandleUnspecified)
	
	return node.FlowNodeResult{
		NextNodeID: nextNodes,
	}
}

func (n *CountingNode) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	result := n.RunSync(ctx, req)
	resultChan <- result
}

func (n *CountingNode) GetExecutions() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]string{}, n.executions...)
}

// TestNestedForLoopMissingExecution reproduces the specific issue of missing 10th execution
func TestNestedForLoopMissingExecution(t *testing.T) {
	testCases := []struct {
		name              string
		outerIterations   int64
		innerIterations   int64
		useTimeout        bool // true = async, false = sync
		expectedExecCount int32
	}{
		{
			name:              "Nested FOR 5x2 - Sync",
			outerIterations:   5,
			innerIterations:   2,
			useTimeout:        false,
			expectedExecCount: 10,
		},
		{
			name:              "Nested FOR 5x2 - Async (REPRODUCES BUG)",
			outerIterations:   5,
			innerIterations:   2,
			useTimeout:        true,
			expectedExecCount: 10,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup nodes
			outerForID := idwrap.NewNow()
			innerForID := idwrap.NewNow()
			requestID := idwrap.NewNow()
			
			// Create outer FOR node (5 iterations)
			outerFor := nfor.New(outerForID, "for_18", tc.outerIterations, time.Second*5, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)
			
			// Create inner FOR node (2 iterations)  
			innerFor := nfor.New(innerForID, "for_21", tc.innerIterations, time.Second*5, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)
			
			// Create counting node (simulates REQUEST node)
			requestNode := NewCountingNode(requestID, "request_0")
			
			// Setup edges: outerFor -> innerFor -> requestNode
			edges := []edge.Edge{
				edge.NewEdge(idwrap.NewNow(), outerForID, innerForID, edge.HandleLoop, edge.EdgeKindUnspecified),
				edge.NewEdge(idwrap.NewNow(), innerForID, requestID, edge.HandleLoop, edge.EdgeKindUnspecified),
			}
			edgeMap := edge.NewEdgesMap(edges)
			
			// Setup node map
			nodeMap := map[idwrap.IDWrap]node.FlowNode{
				outerForID: outerFor,
				innerForID: innerFor,
				requestID:  requestNode,
			}
			
			// Determine timeout for sync vs async
			var timeout time.Duration
			if tc.useTimeout {
				timeout = time.Second * 10
			}
			
			// Create flow runner
			flowRunner := flowlocalrunner.CreateFlowRunner(
				idwrap.NewNow(),
				idwrap.NewNow(),
				outerForID,
				nodeMap,
				edgeMap,
				timeout,
			)
			
			// Setup channels
			flowNodeStatusChan := make(chan runner.FlowNodeStatus, 1000)
			flowStatusChan := make(chan runner.FlowStatus, 10)
			
			// Track request executions
			var requestExecutions []string
			var mu sync.Mutex
			statusDone := make(chan struct{})
			
			go func() {
				defer close(statusDone)
				for status := range flowNodeStatusChan {
					// Track request node executions
					if status.NodeID == requestID && strings.Contains(status.Name, "request_0") {
						mu.Lock()
						requestExecutions = append(requestExecutions, status.Name)
						mu.Unlock()
					}
				}
			}()
			
			// Run the flow
			ctx := context.Background()
			err := flowRunner.Run(ctx, flowNodeStatusChan, flowStatusChan, nil)
			
			// Wait for completion
			<-statusDone
			
			if err != nil {
				t.Fatalf("Flow execution failed: %v", err)
			}
			
			// Check execution count
			actualCount := requestNode.ExecutionCount.Load()
			if actualCount != tc.expectedExecCount {
				t.Errorf("Expected %d executions, got %d", tc.expectedExecCount, actualCount)
				
				// Log execution details
				t.Logf("Execution mode: %s", tc.name)
				t.Logf("CountingNode internal count: %d", actualCount)
				t.Logf("Status updates count: %d", len(requestExecutions))
				
				// Show all executions
				executions := requestNode.GetExecutions()
				t.Logf("\nInternal executions (%d):", len(executions))
				for i, exec := range executions {
					t.Logf("  %d: %s", i+1, exec)
				}
				
				t.Logf("\nStatus executions (%d):", len(requestExecutions))
				for i, exec := range requestExecutions {
					t.Logf("  %d: %s", i+1, exec)
				}
				
				// Check for the specific missing execution pattern
				if tc.useTimeout && actualCount == 9 {
					t.Logf("\nThis matches the reported bug: missing 10th execution in async mode!")
					
					// Analyze which iteration is missing
					foundPaths := make(map[string]bool)
					for _, exec := range executions {
						foundPaths[exec] = true
					}
					
					// Check all expected paths
					t.Logf("\nMissing iterations:")
					for i := 0; i < int(tc.outerIterations); i++ {
						for j := 0; j < int(tc.innerIterations); j++ {
							expected := fmt.Sprintf("Execution %d (path: [%d %d])", i*int(tc.innerIterations)+j+1, i, j)
							if !foundPaths[expected] {
								t.Logf("  - Outer iteration %d, Inner iteration %d", i+1, j+1)
							}
						}
					}
				}
			}
		})
	}
}

// TestForNodeAsyncChildExecution tests that FOR nodes properly wait for async child executions
func TestForNodeAsyncChildExecution(t *testing.T) {
	// This test verifies that a FOR node waits for its child nodes to complete
	// even when using async execution
	
	forNodeID := idwrap.NewNow()
	childNodeID := idwrap.NewNow()
	
	// Create FOR node with 3 iterations
	forNode := nfor.New(forNodeID, "test_for", 3, time.Second*5, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)
	
	// Create a child node that tracks executions
	childNode := NewCountingNode(childNodeID, "child_node")
	
	// Setup edges
	edges := []edge.Edge{
		edge.NewEdge(idwrap.NewNow(), forNodeID, childNodeID, edge.HandleLoop, edge.EdgeKindUnspecified),
	}
	edgeMap := edge.NewEdgesMap(edges)
	
	// Setup node map
	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		forNodeID:   forNode,
		childNodeID: childNode,
	}
	
	// Test both sync and async
	testModes := []struct {
		name    string
		timeout time.Duration
	}{
		{"Sync", 0},
		{"Async", time.Second * 10},
	}
	
	for _, mode := range testModes {
		t.Run(mode.name, func(t *testing.T) {
			// Reset counter
			childNode.ExecutionCount.Store(0)
			childNode.mu.Lock()
			childNode.executions = nil
			childNode.mu.Unlock()
			
			// Create flow runner
			flowRunner := flowlocalrunner.CreateFlowRunner(
				idwrap.NewNow(),
				idwrap.NewNow(),
				forNodeID,
				nodeMap,
				edgeMap,
				mode.timeout,
			)
			
			// Setup channels
			flowNodeStatusChan := make(chan runner.FlowNodeStatus, 100)
			flowStatusChan := make(chan runner.FlowStatus, 10)
			
			// Consume status updates
			done := make(chan struct{})
			go func() {
				defer close(done)
				for range flowNodeStatusChan {
					// Just consume
				}
			}()
			
			// Run the flow
			ctx := context.Background()
			err := flowRunner.Run(ctx, flowNodeStatusChan, flowStatusChan, nil)
			<-done
			
			if err != nil {
				t.Fatalf("Flow execution failed: %v", err)
			}
			
			// Verify all iterations completed
			expectedCount := int32(3)
			actualCount := childNode.ExecutionCount.Load()
			if actualCount != expectedCount {
				t.Errorf("%s mode: Expected %d executions, got %d", mode.name, expectedCount, actualCount)
			}
		})
	}
}