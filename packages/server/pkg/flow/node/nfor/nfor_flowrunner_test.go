package nfor_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/node/nfor"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/flow/runner/flowlocalrunner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
)

// ExecutionTrackingNode tracks all executions with detailed context
type ExecutionTrackingNode struct {
	ID             idwrap.IDWrap
	Name           string
	ExecutionCount atomic.Int32
	mu             sync.Mutex
	executions     []ExecutionDetail
}

type ExecutionDetail struct {
	Count         int32
	IterationPath []int
	Timestamp     time.Time
	ExecutionName string
}

func NewExecutionTrackingNode(id idwrap.IDWrap, name string) *ExecutionTrackingNode {
	return &ExecutionTrackingNode{
		ID:         id,
		Name:       name,
		executions: make([]ExecutionDetail, 0),
	}
}

func (n *ExecutionTrackingNode) GetID() idwrap.IDWrap {
	return n.ID
}

func (n *ExecutionTrackingNode) GetName() string {
	return n.Name
}

func (n *ExecutionTrackingNode) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	count := n.ExecutionCount.Add(1)

	// Track execution details
	n.mu.Lock()
	detail := ExecutionDetail{
		Count:     count,
		Timestamp: time.Now(),
	}

	if req.IterationContext != nil {
		detail.IterationPath = append([]int{}, req.IterationContext.IterationPath...)
		// Build execution name like the real system would
		detail.ExecutionName = fmt.Sprintf("%s - Execution %d", n.Name, count)
	}

	n.executions = append(n.executions, detail)
	n.mu.Unlock()

	// Simulate minimal work
	time.Sleep(1 * time.Millisecond)

	// Get next nodes
	nextNodes := edge.GetNextNodeID(req.EdgeSourceMap, n.ID, edge.HandleUnspecified)

	return node.FlowNodeResult{
		NextNodeID: nextNodes,
	}
}

func (n *ExecutionTrackingNode) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	result := n.RunSync(ctx, req)
	resultChan <- result
}

func (n *ExecutionTrackingNode) GetExecutions() []ExecutionDetail {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]ExecutionDetail{}, n.executions...)
}

// TestNestedForWithFlowRunner reproduces the exact scenario from the bug report
func TestNestedForWithFlowRunner(t *testing.T) {
	testCases := []struct {
		name              string
		timeout           time.Duration // 0 = sync, >0 = async
		expectedExecCount int32
	}{
		{
			name:              "Sync execution (timeout=0)",
			timeout:           0,
			expectedExecCount: 10,
		},
		{
			name:              "Async execution (timeout>0) - REPRODUCES BUG",
			timeout:           10 * time.Second,
			expectedExecCount: 10,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup nodes exactly like the bug report
			outerForID := idwrap.NewNow()
			innerForID := idwrap.NewNow()
			requestID := idwrap.NewNow()

			// Create outer FOR node "for_18" with 5 iterations
			outerFor := nfor.New(outerForID, "for_18", 5, 5*time.Second, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)

			// Create inner FOR node "for_21" with 2 iterations
			innerFor := nfor.New(innerForID, "for_21", 2, 5*time.Second, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)

			// Create request node
			requestNode := NewExecutionTrackingNode(requestID, "request_0")

			// Setup edges: Start -> for_18 -> for_21 -> request_0
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

			// Create flow runner with specified timeout
			flowRunner := flowlocalrunner.CreateFlowRunner(
				idwrap.NewNow(),
				idwrap.NewNow(),
				outerForID, // Start with outer FOR
				nodeMap,
				edgeMap,
				tc.timeout,
			)

			// Setup channels
			flowNodeStatusChan := make(chan runner.FlowNodeStatus, 1000)
			flowStatusChan := make(chan runner.FlowStatus, 10)

			// Track all status updates
			var allStatuses []runner.FlowNodeStatus
			var statusMu sync.Mutex
			statusDone := make(chan struct{})

			go func() {
				defer close(statusDone)
				for status := range flowNodeStatusChan {
					statusMu.Lock()
					allStatuses = append(allStatuses, status)
					statusMu.Unlock()
				}
			}()

			// Run the flow
			ctx := context.Background()
			err := flowRunner.Run(ctx, flowNodeStatusChan, flowStatusChan, nil)

			// Wait for status collection to complete
			<-statusDone

			// Check for errors
			if err != nil {
				t.Fatalf("Flow execution failed: %v", err)
			}

			// Verify execution count
			actualCount := requestNode.ExecutionCount.Load()
			if actualCount != tc.expectedExecCount {
				t.Errorf("Expected %d executions, got %d", tc.expectedExecCount, actualCount)

				// Detailed analysis
				executions := requestNode.GetExecutions()
				t.Logf("\nExecution Details (%d total):", len(executions))
				for i, exec := range executions {
					t.Logf("  %d: Count=%d, Path=%v, Time=%v",
						i+1, exec.Count, exec.IterationPath, exec.Timestamp.Format("15:04:05.000"))
				}

				// Check which specific execution is missing
				if actualCount == 9 && tc.expectedExecCount == 10 {
					t.Logf("\nMISSING 10th EXECUTION - This matches the bug report!")

					// Analyze iteration paths to find the missing one
					foundPaths := make(map[string]bool)
					for _, exec := range executions {
						key := fmt.Sprintf("%v", exec.IterationPath)
						foundPaths[key] = true
					}

					// Check all expected paths
					t.Logf("\nExpected iteration paths:")
					for i := 0; i < 5; i++ {
						for j := 0; j < 2; j++ {
							path := fmt.Sprintf("[%d %d]", i, j)
							status := "✓"
							if !foundPaths[path] {
								status = "✗ MISSING"
							}
							t.Logf("  %s %s (outer=%d, inner=%d)", status, path, i+1, j+1)
						}
					}
				}

				// Count request executions from status updates
				requestStatusCount := 0
				for _, status := range allStatuses {
					if status.NodeID == requestID {
						requestStatusCount++
					}
				}
				t.Logf("\nRequest status updates: %d", requestStatusCount)
			}
		})
	}
}

// TestFlowRunnerAsyncTiming tests if timing affects the issue
func TestFlowRunnerAsyncTiming(t *testing.T) {
	// This test adds delays to see if the issue is timing-related

	outerForID := idwrap.NewNow()
	innerForID := idwrap.NewNow()
	requestID := idwrap.NewNow()

	// Create nodes
	outerFor := nfor.New(outerForID, "for_outer", 3, 5*time.Second, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)
	innerFor := nfor.New(innerForID, "for_inner", 2, 5*time.Second, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)

	// Create a request node
	requestNode := NewExecutionTrackingNode(requestID, "request_delayed")

	// Setup edges
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

	// Run with async mode
	flowRunner := flowlocalrunner.CreateFlowRunner(
		idwrap.NewNow(),
		idwrap.NewNow(),
		outerForID,
		nodeMap,
		edgeMap,
		5*time.Second, // Async mode
	)

	// Setup channels
	flowNodeStatusChan := make(chan runner.FlowNodeStatus, 1000)
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

	// Check execution count
	expectedCount := int32(6) // 3*2
	actualCount := requestNode.ExecutionCount.Load()
	if actualCount != expectedCount {
		t.Errorf("Expected %d executions, got %d", expectedCount, actualCount)
		t.Logf("Timing issue detected: delayed last execution may have been skipped")
	}
}
