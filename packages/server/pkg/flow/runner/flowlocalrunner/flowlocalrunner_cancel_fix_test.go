package flowlocalrunner_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/flow/runner/flowlocalrunner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
)

// SlowTestNode simulates a node that takes time to execute
type SlowTestNode struct {
	ID       idwrap.IDWrap
	Name     string
	Duration time.Duration
	NextIDs  []idwrap.IDWrap
	OnRun    func() // Callback when node runs
}

func (n *SlowTestNode) GetID() idwrap.IDWrap {
	return n.ID
}

func (n *SlowTestNode) GetName() string {
	return n.Name
}

func (n *SlowTestNode) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	if n.OnRun != nil {
		n.OnRun()
	}

	// Simulate work with cancellation checks
	select {
	case <-time.After(n.Duration):
		return node.FlowNodeResult{
			NextNodeID: n.NextIDs,
			Err:        nil,
		}
	case <-ctx.Done():
		return node.FlowNodeResult{
			NextNodeID: nil,
			Err:        ctx.Err(),
		}
	}
}

func (n *SlowTestNode) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	result := n.RunSync(ctx, req)
	resultChan <- result
}

// TestFlowCancellation_AllNodesGetCanceledStatus tests that all running and queued nodes
// receive CANCELED status when a flow is cancelled
func TestFlowCancellation_AllNodesGetCanceledStatus(t *testing.T) {
	// Create a chain of nodes: node1 -> node2 -> node3
	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()
	node3ID := idwrap.NewNow()

	var node1Started, node2Started atomic.Bool

	node1 := &SlowTestNode{
		ID:       node1ID,
		Name:     "node1",
		Duration: 50 * time.Millisecond,
		NextIDs:  []idwrap.IDWrap{node2ID},
		OnRun: func() {
			node1Started.Store(true)
		},
	}

	node2 := &SlowTestNode{
		ID:       node2ID,
		Name:     "node2",
		Duration: 100 * time.Millisecond, // This will be cancelled
		NextIDs:  []idwrap.IDWrap{node3ID},
		OnRun: func() {
			node2Started.Store(true)
		},
	}

	node3 := &SlowTestNode{
		ID:       node3ID,
		Name:     "node3",
		Duration: 50 * time.Millisecond,
		NextIDs:  nil,
	}

	flowNodeMap := map[idwrap.IDWrap]node.FlowNode{
		node1ID: node1,
		node2ID: node2,
		node3ID: node3,
	}

	edge1 := edge.NewEdge(idwrap.NewNow(), node1ID, node2ID, edge.HandleUnspecified, edge.EdgeKindUnspecified)
	edge2 := edge.NewEdge(idwrap.NewNow(), node2ID, node3ID, edge.HandleUnspecified, edge.EdgeKindUnspecified)
	edges := []edge.Edge{edge1, edge2}
	edgesMap := edge.NewEdgesMap(edges)

	// Track all status updates
	var statuses []runner.FlowNodeStatus
	var statusMutex sync.Mutex
	statusChan := make(chan runner.FlowNodeStatus, 100)
	flowStatusChan := make(chan runner.FlowStatus, 10)

	// Create context that will be cancelled after node1 completes and node2 starts
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after a short delay
	go func() {
		time.Sleep(75 * time.Millisecond) // Cancel while node2 is running
		cancel()
	}()

	// Run the flow
	flowRunner := flowlocalrunner.CreateFlowRunner(
		idwrap.NewNow(),
		idwrap.NewNow(),
		node1ID,
		flowNodeMap,
		edgesMap,
		0, // Sync mode
	)

	// Collect statuses
	done := make(chan struct{})
	go func() {
		defer close(done)
		for status := range statusChan {
			statusMutex.Lock()
			statuses = append(statuses, status)
			statusMutex.Unlock()
		}
	}()

	err := flowRunner.Run(ctx, statusChan, flowStatusChan, nil)

	// Should get a cancellation error
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}

	// Wait for status collection to complete
	// Note: The runner closes the statusChan, so we just wait for collection to finish
	<-done

	// Analyze collected statuses
	statusMutex.Lock()
	defer statusMutex.Unlock()

	// Count status types per node
	nodeStatuses := make(map[idwrap.IDWrap]map[mnnode.NodeState]int)
	for _, status := range statuses {
		if nodeStatuses[status.NodeID] == nil {
			nodeStatuses[status.NodeID] = make(map[mnnode.NodeState]int)
		}
		nodeStatuses[status.NodeID][status.State]++
	}

	// Node1 should have completed (RUNNING -> SUCCESS)
	if !node1Started.Load() {
		t.Error("Node1 should have started")
	}
	if nodeStatuses[node1ID][mnnode.NODE_STATE_SUCCESS] != 1 {
		t.Errorf("Node1 should have SUCCESS status, got %v", nodeStatuses[node1ID])
	}

	// Node2 should have been cancelled (RUNNING -> CANCELED)
	if !node2Started.Load() {
		t.Error("Node2 should have started before cancellation")
	}
	if nodeStatuses[node2ID][mnnode.NODE_STATE_RUNNING] != 1 {
		t.Errorf("Node2 should have RUNNING status, got %v", nodeStatuses[node2ID])
	}
	// Allow for multiple CANCELED statuses due to cleanup handling
	if nodeStatuses[node2ID][mnnode.NODE_STATE_CANCELED] < 1 {
		t.Errorf("Node2 should have at least one CANCELED status, got %v", nodeStatuses[node2ID])
	}

	// Node3 might not get any status at all if node2 was canceled before completing
	// This is correct behavior - downstream nodes of canceled nodes don't get queued
	if nodeStatuses[node3ID][mnnode.NODE_STATE_RUNNING] > 0 {
		t.Errorf("Node3 should not have RUNNING status if it got any status, got %v", nodeStatuses[node3ID])
	}
	// Node3 may or may not have a CANCELED status depending on timing
	// If node2 completes its next node list before cancellation, node3 gets queued and canceled
	// If node2 is canceled before that, node3 never gets queued
	t.Logf("Node3 status (may be empty if not queued): %v", nodeStatuses[node3ID])

	// Debug: Log status details for troubleshooting
	t.Logf("Node1 statuses: %v", nodeStatuses[node1ID])
	t.Logf("Node2 statuses: %v", nodeStatuses[node2ID])
	t.Logf("Node3 statuses: %v", nodeStatuses[node3ID])

	// Verify all CANCELED statuses have proper fields
	for _, status := range statuses {
		if status.State == mnnode.NODE_STATE_CANCELED {
			if status.ExecutionID == (idwrap.IDWrap{}) {
				t.Errorf("CANCELED status for node %s missing ExecutionID", status.Name)
			}
			if status.Error == nil {
				t.Errorf("CANCELED status for node %s missing Error", status.Name)
			}
			if status.Name == "" {
				t.Errorf("CANCELED status for node %s missing Name", status.Name)
			}
		}
	}
}

// TestFlowCancellation_ConcurrentNodes tests cancellation with multiple nodes running concurrently
func TestFlowCancellation_ConcurrentNodes(t *testing.T) {
	// Create a diamond pattern: start -> [parallel1, parallel2] -> end
	startID := idwrap.NewNow()
	parallel1ID := idwrap.NewNow()
	parallel2ID := idwrap.NewNow()
	endID := idwrap.NewNow()

	var parallel1Started, parallel2Started atomic.Bool

	startNode := &SlowTestNode{
		ID:       startID,
		Name:     "start",
		Duration: 10 * time.Millisecond,
		NextIDs:  []idwrap.IDWrap{parallel1ID, parallel2ID},
	}

	parallel1Node := &SlowTestNode{
		ID:       parallel1ID,
		Name:     "parallel1",
		Duration: 100 * time.Millisecond,
		NextIDs:  []idwrap.IDWrap{endID},
		OnRun: func() {
			parallel1Started.Store(true)
		},
	}

	parallel2Node := &SlowTestNode{
		ID:       parallel2ID,
		Name:     "parallel2",
		Duration: 100 * time.Millisecond,
		NextIDs:  []idwrap.IDWrap{endID},
		OnRun: func() {
			parallel2Started.Store(true)
		},
	}

	endNode := &SlowTestNode{
		ID:       endID,
		Name:     "end",
		Duration: 50 * time.Millisecond,
		NextIDs:  nil,
	}

	flowNodeMap := map[idwrap.IDWrap]node.FlowNode{
		startID:     startNode,
		parallel1ID: parallel1Node,
		parallel2ID: parallel2Node,
		endID:       endNode,
	}

	// Create edges for diamond pattern
	edges := []edge.Edge{
		edge.NewEdge(idwrap.NewNow(), startID, parallel1ID, edge.HandleUnspecified, edge.EdgeKindUnspecified),
		edge.NewEdge(idwrap.NewNow(), startID, parallel2ID, edge.HandleUnspecified, edge.EdgeKindUnspecified),
		edge.NewEdge(idwrap.NewNow(), parallel1ID, endID, edge.HandleUnspecified, edge.EdgeKindUnspecified),
		edge.NewEdge(idwrap.NewNow(), parallel2ID, endID, edge.HandleUnspecified, edge.EdgeKindUnspecified),
	}
	edgesMap := edge.NewEdgesMap(edges)

	// Track all status updates
	var statuses []runner.FlowNodeStatus
	var statusMutex sync.Mutex
	statusChan := make(chan runner.FlowNodeStatus, 100)
	flowStatusChan := make(chan runner.FlowStatus, 10)

	// Create context that will be cancelled while parallel nodes are running
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after parallel nodes start
	go func() {
		time.Sleep(50 * time.Millisecond) // Cancel while parallel nodes are running
		cancel()
	}()

	// Run the flow
	flowRunner := flowlocalrunner.CreateFlowRunner(
		idwrap.NewNow(),
		idwrap.NewNow(),
		startID,
		flowNodeMap,
		edgesMap,
		0, // Sync mode
	)

	// Collect statuses
	done := make(chan struct{})
	go func() {
		defer close(done)
		for status := range statusChan {
			statusMutex.Lock()
			statuses = append(statuses, status)
			statusMutex.Unlock()
		}
	}()

	err := flowRunner.Run(ctx, statusChan, flowStatusChan, nil)

	// Should get a cancellation error
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}

	// Wait for status collection to complete
	// Note: The runner closes the statusChan, so we just wait for collection to finish
	<-done

	// Analyze collected statuses
	statusMutex.Lock()
	defer statusMutex.Unlock()

	// Count status types per node
	nodeStatuses := make(map[idwrap.IDWrap]map[mnnode.NodeState]int)
	canceledNodes := 0
	for _, status := range statuses {
		if nodeStatuses[status.NodeID] == nil {
			nodeStatuses[status.NodeID] = make(map[mnnode.NodeState]int)
		}
		nodeStatuses[status.NodeID][status.State]++
		if status.State == mnnode.NODE_STATE_CANCELED {
			canceledNodes++
		}
	}

	// Start node should have completed
	if nodeStatuses[startID][mnnode.NODE_STATE_SUCCESS] != 1 {
		t.Errorf("Start node should have SUCCESS status, got %v", nodeStatuses[startID])
	}

	// Both parallel nodes should have been started
	if !parallel1Started.Load() || !parallel2Started.Load() {
		t.Error("Both parallel nodes should have started")
	}

	// Both parallel nodes should have been cancelled
	if nodeStatuses[parallel1ID][mnnode.NODE_STATE_CANCELED] < 1 {
		t.Errorf("Parallel1 should have at least one CANCELED status, got %v", nodeStatuses[parallel1ID])
	}
	if nodeStatuses[parallel2ID][mnnode.NODE_STATE_CANCELED] < 1 {
		t.Errorf("Parallel2 should have at least one CANCELED status, got %v", nodeStatuses[parallel2ID])
	}

	// End node should have been cancelled without running
	// Note: Due to the diamond pattern, end node requires both parallel nodes to complete
	// So it should only have CANCELED status
	if nodeStatuses[endID][mnnode.NODE_STATE_RUNNING] > 0 {
		t.Errorf("End node should not have RUNNING status, got %v", nodeStatuses[endID])
	}

	// We should have at least 3 canceled nodes (parallel1, parallel2, and possibly end)
	if canceledNodes < 2 {
		t.Errorf("Expected at least 2 canceled nodes, got %d", canceledNodes)
	}
}

// TestFlowCancellation_AsyncMode tests cancellation in async mode with timeout
func TestFlowCancellation_AsyncMode(t *testing.T) {
	// Create a simple chain of nodes
	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()

	var node1Started atomic.Bool

	node1 := &SlowTestNode{
		ID:       node1ID,
		Name:     "node1",
		Duration: 200 * time.Millisecond, // Longer than timeout
		NextIDs:  []idwrap.IDWrap{node2ID},
		OnRun: func() {
			node1Started.Store(true)
		},
	}

	node2 := &SlowTestNode{
		ID:       node2ID,
		Name:     "node2",
		Duration: 50 * time.Millisecond,
		NextIDs:  nil,
	}

	flowNodeMap := map[idwrap.IDWrap]node.FlowNode{
		node1ID: node1,
		node2ID: node2,
	}

	edge1 := edge.NewEdge(idwrap.NewNow(), node1ID, node2ID, edge.HandleUnspecified, edge.EdgeKindUnspecified)
	edges := []edge.Edge{edge1}
	edgesMap := edge.NewEdgesMap(edges)

	// Track all status updates
	var statuses []runner.FlowNodeStatus
	var statusMutex sync.Mutex
	statusChan := make(chan runner.FlowNodeStatus, 100)
	flowStatusChan := make(chan runner.FlowStatus, 10)

	// Run the flow with timeout (async mode)
	flowRunner := flowlocalrunner.CreateFlowRunner(
		idwrap.NewNow(),
		idwrap.NewNow(),
		node1ID,
		flowNodeMap,
		edgesMap,
		100*time.Millisecond, // Timeout shorter than node1 duration
	)

	// Collect statuses
	done := make(chan struct{})
	go func() {
		defer close(done)
		for status := range statusChan {
			statusMutex.Lock()
			statuses = append(statuses, status)
			statusMutex.Unlock()
		}
	}()

	ctx := context.Background()
	err := flowRunner.Run(ctx, statusChan, flowStatusChan, nil)

	// Should get a timeout error
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected context.DeadlineExceeded error, got %v", err)
	}

	// Wait for status collection to complete
	// Note: The runner closes the statusChan, so we just wait for collection to finish
	<-done

	// Analyze collected statuses
	statusMutex.Lock()
	defer statusMutex.Unlock()

	// Count status types per node
	nodeStatuses := make(map[idwrap.IDWrap]map[mnnode.NodeState]int)
	for _, status := range statuses {
		if nodeStatuses[status.NodeID] == nil {
			nodeStatuses[status.NodeID] = make(map[mnnode.NodeState]int)
		}
		nodeStatuses[status.NodeID][status.State]++
	}

	// Node1 should have started and then been cancelled due to timeout
	if !node1Started.Load() {
		t.Error("Node1 should have started")
	}
	if nodeStatuses[node1ID][mnnode.NODE_STATE_RUNNING] != 1 {
		t.Errorf("Node1 should have RUNNING status, got %v", nodeStatuses[node1ID])
	}
	if nodeStatuses[node1ID][mnnode.NODE_STATE_CANCELED] < 1 {
		t.Errorf("Node1 should have at least one CANCELED status due to timeout, got %v", nodeStatuses[node1ID])
	}

	// Node2 might not get any status if node1 was canceled before completing
	// This is correct behavior - downstream nodes of canceled nodes don't get queued
	if nodeStatuses[node2ID][mnnode.NODE_STATE_RUNNING] > 0 {
		t.Errorf("Node2 should not have RUNNING status if it got any status, got %v", nodeStatuses[node2ID])
	}
	// Node2 may or may not have a CANCELED status depending on timing
	t.Logf("Node2 status (may be empty if not queued): %v", nodeStatuses[node2ID])

	// Verify CANCELED statuses have proper fields
	for _, status := range statuses {
		if status.State == mnnode.NODE_STATE_CANCELED {
			if status.ExecutionID == (idwrap.IDWrap{}) {
				t.Errorf("CANCELED status for node %s missing ExecutionID", status.Name)
			}
			if status.Error == nil {
				t.Errorf("CANCELED status for node %s missing Error", status.Name)
			}
			// In async mode with timeout, RunDuration should be set for nodes that were running
			if status.NodeID == node1ID && status.RunDuration == 0 {
				t.Errorf("CANCELED status for running node %s should have RunDuration", status.Name)
			}
		}
	}
}

// TestFlowCancellation_NoOrphanedRunningNodes ensures no nodes are left in RUNNING state after cancellation
func TestFlowCancellation_NoOrphanedRunningNodes(t *testing.T) {
	// Create multiple nodes
	numNodes := 5
	nodeIDs := make([]idwrap.IDWrap, numNodes)
	flowNodeMap := make(map[idwrap.IDWrap]node.FlowNode)
	edges := []edge.Edge{}

	for i := 0; i < numNodes; i++ {
		nodeIDs[i] = idwrap.NewNow()
		var nextIDs []idwrap.IDWrap
		if i < numNodes-1 {
			nextIDs = []idwrap.IDWrap{nodeIDs[i+1]}
		}

		node := &SlowTestNode{
			ID:       nodeIDs[i],
			Name:     string(rune('A' + i)),
			Duration: 30 * time.Millisecond,
			NextIDs:  nextIDs,
		}
		flowNodeMap[nodeIDs[i]] = node

		if i < numNodes-1 {
			edges = append(edges, edge.NewEdge(idwrap.NewNow(), nodeIDs[i], nodeIDs[i+1], edge.HandleUnspecified, edge.EdgeKindUnspecified))
		}
	}

	edgesMap := edge.NewEdgesMap(edges)

	// Track all status updates
	var statuses []runner.FlowNodeStatus
	var statusMutex sync.Mutex
	statusChan := make(chan runner.FlowNodeStatus, 100)
	flowStatusChan := make(chan runner.FlowStatus, 10)

	// Cancel quickly
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(45 * time.Millisecond) // Cancel while some nodes are running
		cancel()
	}()

	// Run the flow
	flowRunner := flowlocalrunner.CreateFlowRunner(
		idwrap.NewNow(),
		idwrap.NewNow(),
		nodeIDs[0],
		flowNodeMap,
		edgesMap,
		0,
	)

	// Collect statuses
	done := make(chan struct{})
	go func() {
		defer close(done)
		for status := range statusChan {
			statusMutex.Lock()
			statuses = append(statuses, status)
			statusMutex.Unlock()
		}
	}()

	_ = flowRunner.Run(ctx, statusChan, flowStatusChan, nil)

	// Wait for status collection
	// Note: The runner closes the statusChan, so we just wait for collection to finish
	<-done

	// Check for orphaned RUNNING nodes
	statusMutex.Lock()
	defer statusMutex.Unlock()

	// Track final state of each execution
	executionFinalStates := make(map[idwrap.IDWrap]mnnode.NodeState)
	for _, status := range statuses {
		// Keep track of the last state for each execution ID
		executionFinalStates[status.ExecutionID] = status.State
	}

	// No execution should end in RUNNING state
	for execID, finalState := range executionFinalStates {
		if finalState == mnnode.NODE_STATE_RUNNING {
			t.Errorf("Execution %s left in RUNNING state after cancellation", execID)
		}
	}

	// Every RUNNING status should have a corresponding terminal status (SUCCESS/FAILURE/CANCELED)
	runningExecutions := make(map[idwrap.IDWrap]bool)
	terminalExecutions := make(map[idwrap.IDWrap]bool)

	for _, status := range statuses {
		switch status.State {
		case mnnode.NODE_STATE_RUNNING:
			runningExecutions[status.ExecutionID] = true
		case mnnode.NODE_STATE_SUCCESS, mnnode.NODE_STATE_FAILURE, mnnode.NODE_STATE_CANCELED:
			terminalExecutions[status.ExecutionID] = true
		}
	}

	for execID := range runningExecutions {
		if !terminalExecutions[execID] {
			t.Errorf("Execution %s had RUNNING status but no terminal status", execID)
		}
	}
}
