package flowlocalrunner_test

import (
	"context"
	"sync/atomic"
	"testing"
	"the-dev-tools/backend/pkg/flow/edge"
	"the-dev-tools/backend/pkg/flow/node"
	"the-dev-tools/backend/pkg/flow/node/mocknode"
	"the-dev-tools/backend/pkg/flow/runner"
	"the-dev-tools/backend/pkg/flow/runner/flowlocalrunner"
	"the-dev-tools/backend/pkg/idwrap"
	"time"
)

func TestLocalFlowRunner_Run_Full(t *testing.T) {
	var runCounter int
	onRun := func() {
		runCounter++
	}
	const CountShouldRun = 3

	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()
	node3ID := idwrap.NewNow()
	mockNode1 := mocknode.NewMockNode(node1ID, []idwrap.IDWrap{node2ID}, onRun)
	mockNode2 := mocknode.NewMockNode(node2ID, []idwrap.IDWrap{node3ID}, onRun)
	mockNode3 := mocknode.NewMockNode(node3ID, nil, onRun)

	flowNodeMap := map[idwrap.IDWrap]node.FlowNode{
		node1ID: mockNode1,
		node2ID: mockNode2,
		node3ID: mockNode3,
	}

	edge1 := edge.NewEdge(idwrap.NewNow(), node1ID, node2ID, edge.HandleUnspecified)
	edge2 := edge.NewEdge(idwrap.NewNow(), node2ID, node3ID, edge.HandleUnspecified)
	edges := []edge.Edge{edge1, edge2}
	edgesMap := edge.NewEdgesMap(edges)

	t.Run("Sync", func(t *testing.T) {
		runnerLocal := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), node1ID, flowNodeMap, edgesMap, 0)
		statusChan := make(chan runner.FlowStatusResp, 10)
		err := runnerLocal.Run(context.Background(), statusChan)
		if err != nil {
			t.Errorf("Expected err to be nil, but got %v", err)
		}
		if runCounter != CountShouldRun {
			t.Errorf("Expected runCounter to be %d, but got %d", CountShouldRun, runCounter)
		}
	})
	runCounter = 0
	t.Run("Async", func(t *testing.T) {
		statusChan := make(chan runner.FlowStatusResp, 10)
		runnerLocal := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), node1ID, flowNodeMap, edgesMap, time.Minute)
		err := runnerLocal.Run(context.Background(), statusChan)
		if err != nil {
			t.Errorf("Expected err to be nil, but got %v", err)
		}
	})
}

func TestLocalFlowRunner_Run_NonFull(t *testing.T) {
	var runCounter atomic.Int32
	onRun := func() {
		runCounter.Add(1)
	}

	const CountShouldRun = 3

	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()
	node3ID := idwrap.NewNow()
	node4ID := idwrap.NewNow()
	mockNode1 := mocknode.NewMockNode(node1ID, []idwrap.IDWrap{node2ID}, onRun)
	mockNode2 := mocknode.NewMockNode(node2ID, []idwrap.IDWrap{node3ID}, onRun)
	mockNode3 := mocknode.NewMockNode(node3ID, nil, onRun)
	mockNode4 := mocknode.NewMockNode(node4ID, []idwrap.IDWrap{node1ID}, onRun)

	flowNodeMap := map[idwrap.IDWrap]node.FlowNode{
		node1ID: mockNode1,
		node2ID: mockNode2,
		node3ID: mockNode3,
		node4ID: mockNode4,
	}

	edge1 := edge.NewEdge(idwrap.NewNow(), node1ID, node2ID, edge.HandleUnspecified)
	edge2 := edge.NewEdge(idwrap.NewNow(), node2ID, node3ID, edge.HandleUnspecified)
	edges := []edge.Edge{edge1, edge2}
	edgesMap := edge.NewEdgesMap(edges)

	runnerLocal := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), node1ID, flowNodeMap, edgesMap, time.Minute)
	statusChan := make(chan runner.FlowStatusResp, 10)
	err := runnerLocal.Run(context.Background(), statusChan)
	if err != nil {
		t.Errorf("Expected err to be nil, but got %v", err)
	}
	a := runCounter.Load()
	if a != CountShouldRun {
		t.Errorf("Expected runCounter to be %d, but got %d", CountShouldRun, a)
	}
}

func TestLocalFlowRunner_Run_Timeout(t *testing.T) {
	sleepTime := time.Second
	timeout := time.Millisecond * 300

	onRun := func() {
		time.Sleep(sleepTime)
	}

	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()
	node3ID := idwrap.NewNow()
	mockNode1 := mocknode.NewMockNode(node1ID, []idwrap.IDWrap{node2ID}, onRun)
	mockNode2 := mocknode.NewMockNode(node2ID, []idwrap.IDWrap{node3ID}, onRun)
	mockNode3 := mocknode.NewMockNode(node3ID, nil, onRun)

	flowNodeMap := map[idwrap.IDWrap]node.FlowNode{
		node1ID: mockNode1,
		node2ID: mockNode2,
		node3ID: mockNode3,
	}

	edge1 := edge.NewEdge(idwrap.NewNow(), node1ID, node2ID, edge.HandleUnspecified)
	edge2 := edge.NewEdge(idwrap.NewNow(), node2ID, node3ID, edge.HandleUnspecified)
	edges := []edge.Edge{edge1, edge2}
	edgesMap := edge.NewEdgesMap(edges)

	runnerLocal := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), node1ID, flowNodeMap, edgesMap, timeout)
	statusChan := make(chan runner.FlowStatusResp, 10)
	err := runnerLocal.Run(context.Background(), statusChan)
	if err == nil {
		t.Errorf("Expected err to be not nil, but got %v", err)
	}
}

func TestLocalFlowRunner_Run_ParallelExecution(t *testing.T) {
	sleepTime := time.Millisecond * 500
	timeout := time.Minute * 5

	onRun := func() {
		time.Sleep(sleepTime)
	}

	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()
	node3ID := idwrap.NewNow()
	node4ID := idwrap.NewNow()
	node5ID := idwrap.NewNow()

	mockNode1 := mocknode.NewMockNode(node1ID, []idwrap.IDWrap{node2ID, node3ID}, onRun)
	mockNode2 := mocknode.NewMockNode(node2ID, []idwrap.IDWrap{node4ID}, onRun)
	mockNode3 := mocknode.NewMockNode(node3ID, []idwrap.IDWrap{node4ID}, onRun)
	mockNode4 := mocknode.NewMockNode(node4ID, []idwrap.IDWrap{node5ID}, onRun)
	mockNode5 := mocknode.NewMockNode(node5ID, nil, onRun)

	flowNodeMap := map[idwrap.IDWrap]node.FlowNode{
		node1ID: mockNode1,
		node2ID: mockNode2,
		node3ID: mockNode3,
		node4ID: mockNode4,
		node5ID: mockNode5,
	}

	edge1 := edge.NewEdge(idwrap.NewNow(), node1ID, node2ID, edge.HandleUnspecified)
	edge2 := edge.NewEdge(idwrap.NewNow(), node1ID, node3ID, edge.HandleUnspecified)
	edge3 := edge.NewEdge(idwrap.NewNow(), node2ID, node4ID, edge.HandleUnspecified)
	edge4 := edge.NewEdge(idwrap.NewNow(), node3ID, node4ID, edge.HandleUnspecified)
	edge5 := edge.NewEdge(idwrap.NewNow(), node4ID, node5ID, edge.HandleUnspecified)
	edges := []edge.Edge{edge1, edge2, edge3, edge4, edge5}
	edgesMap := edge.NewEdgesMap(edges)

	runnerLocal := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), node1ID, flowNodeMap, edgesMap, timeout)
	statusChan := make(chan runner.FlowStatusResp, 10)
	err := runnerLocal.Run(context.Background(), statusChan)
	if err != nil {
		t.Errorf("Expected err to be nil, but got %v", err)
	}

	// Check if nodes are running in parallel
	startTimes := make(map[idwrap.IDWrap]time.Time)
	endTimes := make(map[idwrap.IDWrap]time.Time)
	for status := range statusChan {
		if status.CurrentNodeID == nil {
			return
		}
		switch status.NodeStatus {
		case node.NodeStatusRunning:
			startTimes[*status.CurrentNodeID] = time.Now()
		case node.NodeStatusSuccess:
			endTimes[*status.CurrentNodeID] = time.Now()
		}
	}

	if startTimes[node2ID].After(startTimes[node3ID]) || startTimes[node3ID].After(startTimes[node2ID]) {
		t.Errorf("Expected node2 and node3 to start at the same time, but got %v and %v", startTimes[node2ID], startTimes[node3ID])
	}

	if endTimes[node4ID].Before(endTimes[node2ID]) || endTimes[node4ID].Before(endTimes[node3ID]) {
		t.Errorf("Expected node4 to start after node2 and node3 ended, but got %v, %v and %v", endTimes[node4ID], endTimes[node2ID], endTimes[node3ID])
	}
}

func TestLocalFlowRunner_Run_SplitAndMerge(t *testing.T) {
	var runCounter atomic.Int32
	onRun := func() {
		runCounter.Add(1)
	}

	const CountShouldRun = 4

	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()
	node3ID := idwrap.NewNow()
	node4ID := idwrap.NewNow()

	mockNode1 := mocknode.NewMockNode(node1ID, []idwrap.IDWrap{node2ID, node3ID}, onRun)
	mockNode2 := mocknode.NewMockNode(node2ID, []idwrap.IDWrap{node4ID}, onRun)
	mockNode3 := mocknode.NewMockNode(node3ID, []idwrap.IDWrap{node4ID}, onRun)
	mockNode4 := mocknode.NewMockNode(node4ID, nil, onRun)

	flowNodeMap := map[idwrap.IDWrap]node.FlowNode{
		node1ID: mockNode1,
		node2ID: mockNode2,
		node3ID: mockNode3,
		node4ID: mockNode4,
	}

	edge1 := edge.NewEdge(idwrap.NewNow(), node1ID, node2ID, edge.HandleUnspecified)
	edge2 := edge.NewEdge(idwrap.NewNow(), node1ID, node3ID, edge.HandleUnspecified)
	edge3 := edge.NewEdge(idwrap.NewNow(), node2ID, node4ID, edge.HandleUnspecified)
	edge4 := edge.NewEdge(idwrap.NewNow(), node3ID, node4ID, edge.HandleUnspecified)
	edges := []edge.Edge{edge1, edge2, edge3, edge4}
	edgesMap := edge.NewEdgesMap(edges)

	runnerLocal := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), node1ID, flowNodeMap, edgesMap, time.Minute)
	statusChan := make(chan runner.FlowStatusResp, 10)
	err := runnerLocal.Run(context.Background(), statusChan)
	if err != nil {
		t.Errorf("Expected err to be nil, but got %v", err)
	}
	a := runCounter.Load()
	if a != CountShouldRun {
		t.Errorf("Expected runCounter to be %d, but got %d", CountShouldRun, a)
	}
}

func TestLocalFlowRunner_Run_SplitAndMergeWithSubNodes(t *testing.T) {
	var runCounter atomic.Int32
	onRun := func() {
		runCounter.Add(1)
	}

	const CountShouldRun = 9

	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()
	node3ID := idwrap.NewNow()
	node4ID := idwrap.NewNow()
	node5ID := idwrap.NewNow()
	node6ID := idwrap.NewNow()
	node7ID := idwrap.NewNow()
	node8ID := idwrap.NewNow()
	node9ID := idwrap.NewNow()

	mockNode1 := mocknode.NewMockNode(node1ID, []idwrap.IDWrap{node2ID, node3ID}, onRun)
	mockNode2 := mocknode.NewMockNode(node2ID, []idwrap.IDWrap{node4ID, node5ID}, onRun)
	mockNode3 := mocknode.NewMockNode(node3ID, []idwrap.IDWrap{node6ID, node7ID}, onRun)
	mockNode4 := mocknode.NewMockNode(node4ID, []idwrap.IDWrap{node8ID}, onRun)
	mockNode5 := mocknode.NewMockNode(node5ID, []idwrap.IDWrap{node8ID}, onRun)
	mockNode6 := mocknode.NewMockNode(node6ID, []idwrap.IDWrap{node9ID}, onRun)
	mockNode7 := mocknode.NewMockNode(node7ID, []idwrap.IDWrap{node9ID}, onRun)
	mockNode8 := mocknode.NewMockNode(node8ID, []idwrap.IDWrap{node9ID}, onRun)
	mockNode9 := mocknode.NewMockNode(node9ID, nil, onRun)

	flowNodeMap := map[idwrap.IDWrap]node.FlowNode{
		node1ID: mockNode1,
		node2ID: mockNode2,
		node3ID: mockNode3,
		node4ID: mockNode4,
		node5ID: mockNode5,
		node6ID: mockNode6,
		node7ID: mockNode7,
		node8ID: mockNode8,
		node9ID: mockNode9,
	}

	edge1 := edge.NewEdge(idwrap.NewNow(), node1ID, node2ID, edge.HandleUnspecified)
	edge2 := edge.NewEdge(idwrap.NewNow(), node1ID, node3ID, edge.HandleUnspecified)
	edge3 := edge.NewEdge(idwrap.NewNow(), node2ID, node4ID, edge.HandleUnspecified)
	edge4 := edge.NewEdge(idwrap.NewNow(), node2ID, node5ID, edge.HandleUnspecified)
	edge5 := edge.NewEdge(idwrap.NewNow(), node3ID, node6ID, edge.HandleUnspecified)
	edge6 := edge.NewEdge(idwrap.NewNow(), node3ID, node7ID, edge.HandleUnspecified)
	edge7 := edge.NewEdge(idwrap.NewNow(), node4ID, node8ID, edge.HandleUnspecified)
	edge8 := edge.NewEdge(idwrap.NewNow(), node5ID, node8ID, edge.HandleUnspecified)
	edge9 := edge.NewEdge(idwrap.NewNow(), node6ID, node9ID, edge.HandleUnspecified)
	edge10 := edge.NewEdge(idwrap.NewNow(), node7ID, node9ID, edge.HandleUnspecified)
	edge11 := edge.NewEdge(idwrap.NewNow(), node8ID, node9ID, edge.HandleUnspecified)
	edges := []edge.Edge{edge1, edge2, edge3, edge4, edge5, edge6, edge7, edge8, edge9, edge10, edge11}
	edgesMap := edge.NewEdgesMap(edges)

	runnerLocal := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), node1ID, flowNodeMap, edgesMap, time.Minute)
	statusChan := make(chan runner.FlowStatusResp, 10)
	err := runnerLocal.Run(context.Background(), statusChan)
	if err != nil {
		t.Errorf("Expected err to be nil, but got %v", err)
	}
	a := runCounter.Load()
	if a != CountShouldRun {
		t.Errorf("Expected runCounter to be %d, but got %d", CountShouldRun, a)
	}
}
