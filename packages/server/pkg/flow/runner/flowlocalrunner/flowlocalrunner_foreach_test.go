package flowlocalrunner_test

import (
	"context"
	"errors"
	"testing"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/node/nforeach"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/flow/runner/flowlocalrunner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"time"
)

// TestForEachNode_ErrorHandling_SubNodeError tests that errors are shown on sub-nodes, not the foreach node
func TestForEachNode_ErrorHandling_SubNodeError(t *testing.T) {
	tests := []struct {
		name               string
		errorHandling      mnfor.ErrorHandling
		expectForEachError bool
	}{
		{
			name:               "IGNORE - foreach node succeeds, sub-node shows error",
			errorHandling:      mnfor.ErrorHandling_ERROR_HANDLING_IGNORE,
			expectForEachError: false,
		},
		{
			name:               "BREAK - foreach node succeeds, sub-node shows error",
			errorHandling:      mnfor.ErrorHandling_ERROR_HANDLING_BREAK,
			expectForEachError: false,
		},
		{
			name:               "UNSPECIFIED - both nodes show error",
			errorHandling:      mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED,
			expectForEachError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup nodes
			forEachNodeID := idwrap.NewNow()
			subNodeID := idwrap.NewNow()
			nextNodeID := idwrap.NewNow()
			
			// Create a foreach node that iterates over a literal array
			forEachNode := nforeach.New(
				forEachNodeID,
				"TestForEachLoop",
				"[1, 2, 3]", // Literal array
				time.Second*5,
				mcondition.Condition{},
				tt.errorHandling,
			)
			
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
				edge.NewEdge(idwrap.NewNow(), forEachNodeID, subNodeID, edge.HandleLoop),
				edge.NewEdge(idwrap.NewNow(), forEachNodeID, nextNodeID, edge.HandleThen),
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
				forEachNodeID: forEachNode,
				subNodeID:     subNode,
				nextNodeID:    nextNode,
			}
			
			// Create flow runner
			flowRunner := flowlocalrunner.CreateFlowRunner(
				idwrap.NewNow(),
				idwrap.NewNow(),
				forEachNodeID,
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
			forEachNodeFinalStatus := statusTracker.GetFinalStatus(forEachNodeID)
			subNodeStatuses := statusTracker.GetStatuses(subNodeID)
			
			if forEachNodeFinalStatus == nil {
				t.Fatal("Expected foreach node to have status updates")
			}
			
			// Check foreach node status
			if tt.expectForEachError {
				// UNSPECIFIED mode - foreach node should fail
				if err == nil {
					t.Error("Expected flow to fail with UNSPECIFIED error handling")
				}
				if forEachNodeFinalStatus.State != mnnode.NODE_STATE_FAILURE {
					t.Errorf("Expected foreach node to have FAILURE state, got %v", forEachNodeFinalStatus.State)
				}
			} else {
				// IGNORE/BREAK modes - foreach node should succeed
				if err != nil {
					t.Errorf("Expected flow to succeed with %v error handling, got error: %v", tt.errorHandling, err)
				}
				if forEachNodeFinalStatus.State != mnnode.NODE_STATE_SUCCESS {
					t.Errorf("Expected foreach node to have SUCCESS state, got %v", forEachNodeFinalStatus.State)
				}
				if forEachNodeFinalStatus.Error != nil {
					t.Errorf("Expected foreach node to have no error, got: %v", forEachNodeFinalStatus.Error)
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

// TestForEachNode_ErrorHandling_MultipleSubNodes tests error handling with multiple sub-nodes in the loop
func TestForEachNode_ErrorHandling_MultipleSubNodes(t *testing.T) {
	// Setup nodes
	forEachNodeID := idwrap.NewNow()
	subNode1ID := idwrap.NewNow()
	subNode2ID := idwrap.NewNow()
	subNode3ID := idwrap.NewNow()
	nextNodeID := idwrap.NewNow()
	
	// Create a foreach node with IGNORE error handling
	forEachNode := nforeach.New(
		forEachNodeID,
		"TestForEachLoop",
		"[1, 2, 3]",
		time.Second*5,
		mcondition.Condition{},
		mnfor.ErrorHandling_ERROR_HANDLING_IGNORE,
	)
	
	// Create sub-nodes - second one fails
	subNode1 := &ErrorNode{
		ID:         subNode1ID,
		Name:       "SubNode1",
		ShouldFail: func(int) bool { return false },
		FailError:  errors.New("test error from SubNode1"),
	}
	subNode2 := &ErrorNode{
		ID:         subNode2ID,
		Name:       "SubNode2",
		ShouldFail: func(int) bool { return true }, // Always fails
		FailError:  errors.New("test error from SubNode2"),
	}
	subNode3 := &ErrorNode{
		ID:         subNode3ID,
		Name:       "SubNode3",
		ShouldFail: func(int) bool { return false },
		FailError:  errors.New("test error from SubNode3"),
	}
	
	// Setup edges - chain of sub-nodes
	edges := []edge.Edge{
		edge.NewEdge(idwrap.NewNow(), forEachNodeID, subNode1ID, edge.HandleLoop),
		edge.NewEdge(idwrap.NewNow(), subNode1ID, subNode2ID, edge.HandleUnspecified),
		edge.NewEdge(idwrap.NewNow(), subNode2ID, subNode3ID, edge.HandleUnspecified),
		edge.NewEdge(idwrap.NewNow(), forEachNodeID, nextNodeID, edge.HandleThen),
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
		forEachNodeID: forEachNode,
		subNode1ID:    subNode1,
		subNode2ID:    subNode2,
		subNode3ID:    subNode3,
		nextNodeID:    nextNode,
	}
	
	// Create flow runner
	flowRunner := flowlocalrunner.CreateFlowRunner(
		idwrap.NewNow(),
		idwrap.NewNow(),
		forEachNodeID,
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
	
	// Verify results
	if err != nil {
		t.Errorf("Expected flow to succeed with IGNORE error handling, got: %v", err)
	}
	
	// Check foreach node succeeded
	forEachStatus := statusTracker.GetFinalStatus(forEachNodeID)
	if forEachStatus == nil || forEachStatus.State != mnnode.NODE_STATE_SUCCESS {
		t.Error("Expected foreach node to succeed with IGNORE error handling")
	}
	
	// Check sub-nodes
	subNode1Status := statusTracker.GetFinalStatus(subNode1ID)
	subNode2Statuses := statusTracker.GetStatuses(subNode2ID)
	subNode3Status := statusTracker.GetFinalStatus(subNode3ID)
	
	// SubNode1 should succeed
	if subNode1Status == nil || subNode1Status.State != mnnode.NODE_STATE_SUCCESS {
		t.Error("Expected SubNode1 to succeed")
	}
	
	// SubNode2 should have failure statuses
	failureCount := 0
	for _, status := range subNode2Statuses {
		if status.State == mnnode.NODE_STATE_FAILURE {
			failureCount++
		}
	}
	
	// Debug: Print actual executions
	t.Logf("SubNode1 executions: %d", subNode1.Executions)
	t.Logf("SubNode2 executions: %d", subNode2.Executions)
	t.Logf("SubNode3 executions: %d", subNode3.Executions)
	t.Logf("SubNode2 failure count: %d", failureCount)
	
	// With IGNORE at the foreach level, the foreach continues but each iteration's chain fails at SubNode2
	if subNode1.Executions != 3 {
		t.Errorf("Expected SubNode1 to execute 3 times, got %d", subNode1.Executions)
	}
	if subNode2.Executions != 3 {
		t.Errorf("Expected SubNode2 to execute 3 times, got %d", subNode2.Executions)
	}
	if failureCount == 0 {
		t.Error("Expected SubNode2 to have failure statuses")
	}
	
	// SubNode3 should not run because SubNode2's failure stops the chain
	// (IGNORE only applies at the foreach level, not within the sub-node chain)
	if subNode3Status != nil && subNode3Status.State == mnnode.NODE_STATE_SUCCESS {
		t.Error("Expected SubNode3 to not succeed after SubNode2 failed")
	}
}