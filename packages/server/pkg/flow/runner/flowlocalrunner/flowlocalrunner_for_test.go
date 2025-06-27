package flowlocalrunner_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/node/nfor"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/flow/runner/flowlocalrunner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"time"
)

// ErrorNode is a test node that can be configured to fail
type ErrorNode struct {
	ID         idwrap.IDWrap
	Name       string
	ShouldFail func(iteration int) bool
	FailError  error
	mu         sync.Mutex
	Executions int
}

func NewErrorNode(id idwrap.IDWrap, name string, shouldFail bool) *ErrorNode {
	return &ErrorNode{
		ID:         id,
		Name:       name,
		ShouldFail: func(int) bool { return shouldFail },
		FailError:  errors.New("test error from " + name),
	}
}

func (n *ErrorNode) GetID() idwrap.IDWrap {
	return n.ID
}

func (n *ErrorNode) GetName() string {
	return n.Name
}

func (n *ErrorNode) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	n.mu.Lock()
	n.Executions++
	currentExecution := n.Executions
	n.mu.Unlock()
	
	if n.ShouldFail != nil && n.ShouldFail(currentExecution) {
		return node.FlowNodeResult{
			Err: n.FailError,
		}
	}
	
	// Get next nodes from edge map
	nextNodes := edge.GetNextNodeID(req.EdgeSourceMap, n.ID, edge.HandleUnspecified)
	
	return node.FlowNodeResult{
		NextNodeID: nextNodes,
	}
}

func (n *ErrorNode) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	result := n.RunSync(ctx, req)
	resultChan <- result
}

// NodeStatusTracker tracks all node status updates
type NodeStatusTracker struct {
	mu       sync.Mutex
	statuses map[idwrap.IDWrap][]runner.FlowNodeStatus
}

func NewNodeStatusTracker() *NodeStatusTracker {
	return &NodeStatusTracker{
		statuses: make(map[idwrap.IDWrap][]runner.FlowNodeStatus),
	}
}

func (t *NodeStatusTracker) Track(status runner.FlowNodeStatus) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.statuses[status.NodeID] = append(t.statuses[status.NodeID], status)
}

func (t *NodeStatusTracker) GetStatuses(nodeID idwrap.IDWrap) []runner.FlowNodeStatus {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]runner.FlowNodeStatus{}, t.statuses[nodeID]...)
}

func (t *NodeStatusTracker) GetFinalStatus(nodeID idwrap.IDWrap) *runner.FlowNodeStatus {
	statuses := t.GetStatuses(nodeID)
	if len(statuses) == 0 {
		return nil
	}
	return &statuses[len(statuses)-1]
}

// TestForNode_ErrorHandling_SubNodeError tests that errors are shown on sub-nodes, not the for node
func TestForNode_ErrorHandling_SubNodeError(t *testing.T) {
	tests := []struct {
		name          string
		errorHandling mnfor.ErrorHandling
		expectForNodeError bool
	}{
		{
			name:          "IGNORE - for node succeeds, sub-node shows error",
			errorHandling: mnfor.ErrorHandling_ERROR_HANDLING_IGNORE,
			expectForNodeError: false,
		},
		{
			name:          "BREAK - for node succeeds, sub-node shows error",
			errorHandling: mnfor.ErrorHandling_ERROR_HANDLING_BREAK,
			expectForNodeError: false,
		},
		{
			name:          "UNSPECIFIED - both nodes show error",
			errorHandling: mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED,
			expectForNodeError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup nodes
			forNodeID := idwrap.NewNow()
			subNodeID := idwrap.NewNow()
			nextNodeID := idwrap.NewNow()
			
			// Create a for node with 3 iterations
			forNode := nfor.New(forNodeID, "TestForLoop", 3, time.Second*5, tt.errorHandling)
			
			// Create a sub-node that fails on second iteration
			subNode := &ErrorNode{
				ID:   subNodeID,
				Name: "SubNode",
				ShouldFail: func(iteration int) bool {
					return iteration == 2
				},
				FailError: errors.New("test error from SubNode"),
			}
			
			// Setup edges
			edges := []edge.Edge{
				edge.NewEdge(idwrap.NewNow(), forNodeID, subNodeID, edge.HandleLoop, edge.EdgeKindUnspecified),
				edge.NewEdge(idwrap.NewNow(), forNodeID, nextNodeID, edge.HandleThen, edge.EdgeKindUnspecified),
			}
			edgeMap := edge.NewEdgesMap(edges)
			
			// Create a simple next node
			nextNode := &ErrorNode{
				ID:         nextNodeID,
				Name:       "NextNode",
				ShouldFail: func(int) bool { return false },
			}
			
			// Setup node map
			nodeMap := map[idwrap.IDWrap]node.FlowNode{
				forNodeID:  forNode,
				subNodeID:  subNode,
				nextNodeID: nextNode,
			}
			
			// Create flow runner
			flowRunner := flowlocalrunner.CreateFlowRunner(
				idwrap.NewNow(),
				idwrap.NewNow(),
				forNodeID,
				nodeMap,
				edgeMap,
				time.Second*10,
			)
			
			// Setup status tracking
			statusTracker := NewNodeStatusTracker()
			flowNodeStatusChan := make(chan runner.FlowNodeStatus, 100)
			flowStatusChan := make(chan runner.FlowStatus, 10)
			
			// Track statuses in background
			statusDone := make(chan struct{})
			go func() {
				defer close(statusDone)
				for status := range flowNodeStatusChan {
					statusTracker.Track(status)
				}
			}()
			
			// Run the flow
			ctx := context.Background()
			err := flowRunner.Run(ctx, flowNodeStatusChan, flowStatusChan, nil)
			
			// Wait for status tracking to complete
			<-statusDone
			
			// Verify results based on error handling mode
			forNodeFinalStatus := statusTracker.GetFinalStatus(forNodeID)
			subNodeStatuses := statusTracker.GetStatuses(subNodeID)
			
			if forNodeFinalStatus == nil {
				t.Fatal("Expected for node to have status updates")
			}
			
			// Check for node status
			if tt.expectForNodeError {
				// UNSPECIFIED mode - for node should fail
				if err == nil {
					t.Error("Expected flow to fail with UNSPECIFIED error handling")
				}
				if forNodeFinalStatus.State != mnnode.NODE_STATE_FAILURE {
					t.Errorf("Expected for node to have FAILURE state, got %v", forNodeFinalStatus.State)
				}
			} else {
				// IGNORE/BREAK modes - for node should succeed
				if err != nil {
					t.Errorf("Expected flow to succeed with %v error handling, got error: %v", tt.errorHandling, err)
				}
				if forNodeFinalStatus.State != mnnode.NODE_STATE_SUCCESS {
					t.Errorf("Expected for node to have SUCCESS state, got %v", forNodeFinalStatus.State)
				}
				if forNodeFinalStatus.Error != nil {
					t.Errorf("Expected for node to have no error, got: %v", forNodeFinalStatus.Error)
				}
			}
			
			// Check sub-node statuses
			// Find the status update where the sub-node failed
			foundFailure := false
			for _, status := range subNodeStatuses {
				if status.State == mnnode.NODE_STATE_FAILURE && status.Error != nil {
					foundFailure = true
					t.Logf("Sub-node failed with error: %v", status.Error)
					break
				}
			}
			
			if !foundFailure {
				t.Error("Expected to find a failure status for the sub-node")
			}
			
			// Verify iteration count based on error handling
			expectedIterations := 0
			switch tt.errorHandling {
			case mnfor.ErrorHandling_ERROR_HANDLING_IGNORE:
				expectedIterations = 3 // All iterations run
			case mnfor.ErrorHandling_ERROR_HANDLING_BREAK:
				expectedIterations = 2 // Stops after error
			case mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED:
				expectedIterations = 2 // Stops after error
			}
			
			if subNode.Executions != expectedIterations {
				t.Errorf("Expected %d iterations, got %d", expectedIterations, subNode.Executions)
			}
		})
	}
}