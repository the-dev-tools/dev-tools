package flowlocalrunner_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"the-dev-tools/backend/pkg/flow/edge"
	"the-dev-tools/backend/pkg/flow/node"
	"the-dev-tools/backend/pkg/flow/node/mocknode"
	"the-dev-tools/backend/pkg/flow/runner"
	"the-dev-tools/backend/pkg/flow/runner/flowlocalrunner"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mnnode"
	"the-dev-tools/backend/pkg/testutil"
	"time"
)

const (
	FlowNodeStatusChanBufferSize = 100
	FlowStatusChanBufferSize     = 10
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
		flowNodeStatusChan := make(chan runner.FlowNodeStatus, FlowNodeStatusChanBufferSize)
		flowStatusChan := make(chan runner.FlowStatus, FlowStatusChanBufferSize)
		err := runnerLocal.Run(context.Background(), flowNodeStatusChan, flowStatusChan)
		if err != nil {
			t.Errorf("Expected err to be nil, but got %v", err)
		}
		if runCounter != CountShouldRun {
			t.Errorf("Expected runCounter to be %d, but got %d", CountShouldRun, runCounter)
		}
	})
	runCounter = 0
	t.Run("Async", func(t *testing.T) {
		runnerLocal := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), node1ID, flowNodeMap, edgesMap, time.Minute)
		flowNodeStatusChan := make(chan runner.FlowNodeStatus, FlowNodeStatusChanBufferSize)
		flowStatusChan := make(chan runner.FlowStatus, FlowStatusChanBufferSize)
		err := runnerLocal.Run(context.Background(), flowNodeStatusChan, flowStatusChan)
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
	flowNodeStatusChan := make(chan runner.FlowNodeStatus, FlowNodeStatusChanBufferSize)
	flowStatusChan := make(chan runner.FlowStatus, FlowStatusChanBufferSize)
	err := runnerLocal.Run(context.Background(), flowNodeStatusChan, flowStatusChan)
	if err != nil {
		t.Errorf("Expected err to be nil, but got %v", err)
	}
	a := runCounter.Load()
	if a != CountShouldRun {
		t.Errorf("Expected runCounter to be %d, but got %d", CountShouldRun, a)
	}
}

func TestLocalFlowRunner_Run_Timeout(t *testing.T) {
	sleepTime := time.Microsecond
	timeout := time.Microsecond

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
	flowNodeStatusChan := make(chan runner.FlowNodeStatus, FlowNodeStatusChanBufferSize)
	flowStatusChan := make(chan runner.FlowStatus, FlowStatusChanBufferSize)
	err := runnerLocal.Run(context.Background(), flowNodeStatusChan, flowStatusChan)
	if err == nil {
		t.Errorf("Expected err to be not nil, but got %v", err)
	}
}

func TestLocalFlowRunner_Run_ParallelExecution(t *testing.T) {
	sleepTime := time.Microsecond
	timeout := time.Millisecond

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
	flowNodeStatusChan := make(chan runner.FlowNodeStatus, FlowNodeStatusChanBufferSize)
	flowStatusChan := make(chan runner.FlowStatus, FlowStatusChanBufferSize)
	err := runnerLocal.Run(context.Background(), flowNodeStatusChan, flowStatusChan)
	if err != nil {
		t.Errorf("Expected err to be nil, but got %v", err)
	}

	// Check if nodes are running in parallel
	startTimes := make(map[idwrap.IDWrap]time.Time)
	endTimes := make(map[idwrap.IDWrap]time.Time)
	for status := range flowNodeStatusChan {
		switch status.State {
		case mnnode.NODE_STATE_RUNNING:
			startTimes[status.NodeID] = time.Now()
		case mnnode.NODE_STATE_SUCCESS:
			endTimes[status.NodeID] = time.Now()
		}
	}

	timeDifference := startTimes[node2ID].Sub(startTimes[node3ID])
	if timeDifference < -time.Millisecond || timeDifference > time.Millisecond {
		t.Errorf("Expected node2 and node3 to start at approximately the same time, but got %v and %v (diff: %v)",
			startTimes[node2ID], startTimes[node3ID], timeDifference)
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
	flowNodeStatusChan := make(chan runner.FlowNodeStatus, FlowNodeStatusChanBufferSize)
	flowStatusChan := make(chan runner.FlowStatus, FlowStatusChanBufferSize)
	err := runnerLocal.Run(context.Background(), flowNodeStatusChan, flowStatusChan)
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
	flowNodeStatusChan := make(chan runner.FlowNodeStatus, FlowNodeStatusChanBufferSize)
	flowStatusChan := make(chan runner.FlowStatus, FlowStatusChanBufferSize)
	err := runnerLocal.Run(context.Background(), flowNodeStatusChan, flowStatusChan)
	if err != nil {
		t.Errorf("Expected err to be nil, but got %v", err)
	}
	a := runCounter.Load()
	if a != CountShouldRun {
		t.Errorf("Expected runCounter to be %d, but got %d", CountShouldRun, a)
	}
}

func TestRunNodeASync_IncompleteExecution(t *testing.T) {
	nodeRunMapCounter := make(map[idwrap.IDWrap]*atomic.Int32)

	onRun := func(id idwrap.IDWrap) {
		a, ok := nodeRunMapCounter[id]
		if !ok {
			a = &atomic.Int32{}
		}
		a.Add(1)
	}

	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()
	node3ID := idwrap.NewNow()
	node4ID := idwrap.NewNow()

	// mockNode1 starts, sets runningNode, then yields
	mockNode1 := mocknode.NewMockNode(node1ID, []idwrap.IDWrap{node2ID}, func() {
		onRun(node1ID)
	})

	// mockNode2 starts, sets runningNode, then yields
	mockNode2 := mocknode.NewMockNode(node2ID, []idwrap.IDWrap{node3ID}, func() {
		onRun(node2ID)
	})

	// mockNode3 starts, sets runningNode, then yields
	mockNode3 := mocknode.NewMockNode(node3ID, []idwrap.IDWrap{node4ID}, func() {
		onRun(node3ID)
	})

	// mockNode4 starts, sets runningNode, then yields
	mockNode4 := mocknode.NewMockNode(node4ID, nil, func() {
		onRun(node4ID)
	})

	flowNodeMap := map[idwrap.IDWrap]node.FlowNode{
		node1ID: mockNode1,
		node2ID: mockNode2,
		node3ID: mockNode3,
		node4ID: mockNode4,
	}

	edge1 := edge.NewEdge(idwrap.NewNow(), node1ID, node2ID, edge.HandleUnspecified)
	edge2 := edge.NewEdge(idwrap.NewNow(), node2ID, node3ID, edge.HandleUnspecified)
	edge3 := edge.NewEdge(idwrap.NewNow(), node3ID, node4ID, edge.HandleUnspecified)
	edges := []edge.Edge{edge1, edge2, edge3}
	edgesMap := edge.NewEdgesMap(edges)

	flowID := idwrap.NewNow()
	timeout := 50 * time.Millisecond
	runnerLocal := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), flowID, node1ID, flowNodeMap, edgesMap, timeout)
	flowNodeStatusChan := make(chan runner.FlowNodeStatus, FlowNodeStatusChanBufferSize)
	flowStatusChan := make(chan runner.FlowStatus, FlowStatusChanBufferSize)
	err := runnerLocal.Run(context.Background(), flowNodeStatusChan, flowStatusChan)
	testutil.Assert(t, nil, err)

	type nodeRunCounter struct {
		runCounter     int32
		successCounter int32
	}

	testMapCounter := make(map[idwrap.IDWrap]*nodeRunCounter)
	for status := range flowNodeStatusChan {
		s, ok := testMapCounter[status.NodeID]
		if !ok {
			s = &nodeRunCounter{}
		}
		switch status.State {
		case mnnode.NODE_STATE_RUNNING:
			s.runCounter++
		case mnnode.NODE_STATE_SUCCESS:
			s.successCounter++
		default:
			t.Errorf("Expected status to be either NodeStatusRunning or NodeStatusSuccess, but got %v", status.State)
		}
	}

	for k, v := range nodeRunMapCounter {
		a, ok := testMapCounter[k]
		if !ok {
			t.Errorf("Expected key %v to be in testMapCounter", k)
		}

		testutil.Assert(t, v.Load(), a.runCounter)
		testutil.Assert(t, v.Load(), a.successCounter)

	}
}

// BenchmarkLinearFlow benchmarks running a simple linear flow
func BenchmarkLinearFlow(b *testing.B) {
	// Create a simple linear flow: node1 -> node2 -> node3
	onRun := func() {}

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

	// Run benchmark
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runnerLocal := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), node1ID, flowNodeMap, edgesMap, 0)
		flowNodeStatusChan := make(chan runner.FlowNodeStatus, FlowNodeStatusChanBufferSize)
		flowStatusChan := make(chan runner.FlowStatus, FlowStatusChanBufferSize)
		_ = runnerLocal.Run(context.Background(), flowNodeStatusChan, flowStatusChan)
	}
}

// BenchmarkSplitAndMergeFlow benchmarks running a split and merge flow pattern
func BenchmarkSplitAndMergeFlow(b *testing.B) {
	// Create a split and merge flow: node1 -> node2 -> node4
	//                                   \-> node3 -/
	onRun := func() {}

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

	// Run benchmark
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runnerLocal := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), node1ID, flowNodeMap, edgesMap, 0)
		flowNodeStatusChan := make(chan runner.FlowNodeStatus, FlowNodeStatusChanBufferSize)
		flowStatusChan := make(chan runner.FlowStatus, FlowStatusChanBufferSize)
		_ = runnerLocal.Run(context.Background(), flowNodeStatusChan, flowStatusChan)
	}
}

// BenchmarkComplexFlow benchmarks running a complex flow with multiple splits and merges
func BenchmarkComplexFlow(b *testing.B) {
	// Create complex flow based on TestLocalFlowRunner_Run_SplitAndMergeWithSubNodes
	onRun := func() {}

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

	// Run benchmark
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runnerLocal := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), node1ID, flowNodeMap, edgesMap, 0)
		statusChan := make(chan runner.FlowNodeStatus, 100)
		flowStatusChan := make(chan runner.FlowStatus, 10)
		_ = runnerLocal.Run(context.Background(), statusChan, flowStatusChan)
	}
}

// BenchmarkDelayedNodes benchmarks behavior with nodes that take time to process
func BenchmarkDelayedNodes(b *testing.B) {
	// Create mock nodes with delays
	delayedRun := func() {
		time.Sleep(time.Nanosecond)
	}

	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()
	node3ID := idwrap.NewNow()

	mockNode1 := mocknode.NewMockNode(node1ID, []idwrap.IDWrap{node2ID}, delayedRun)
	mockNode2 := mocknode.NewMockNode(node2ID, []idwrap.IDWrap{node3ID}, delayedRun)
	mockNode3 := mocknode.NewMockNode(node3ID, nil, delayedRun)

	flowNodeMap := map[idwrap.IDWrap]node.FlowNode{
		node1ID: mockNode1,
		node2ID: mockNode2,
		node3ID: mockNode3,
	}

	edge1 := edge.NewEdge(idwrap.NewNow(), node1ID, node2ID, edge.HandleUnspecified)
	edge2 := edge.NewEdge(idwrap.NewNow(), node2ID, node3ID, edge.HandleUnspecified)
	edges := []edge.Edge{edge1, edge2}
	edgesMap := edge.NewEdgesMap(edges)

	// Run benchmark with a timeout
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runnerLocal := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), node1ID, flowNodeMap, edgesMap, time.Second)
		flowNodeStatusChan := make(chan runner.FlowNodeStatus, FlowNodeStatusChanBufferSize)
		flowStatusChan := make(chan runner.FlowStatus, FlowStatusChanBufferSize)
		_ = runnerLocal.Run(context.Background(), flowNodeStatusChan, flowStatusChan)
	}
}

// BenchmarkAsyncVsSync compares async and sync modes
func BenchmarkAsyncVsSync(b *testing.B) {
	// Setup common test structure
	onRun := func() {}

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

	// Benchmark sync mode
	b.Run("Sync", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			runnerLocal := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), node1ID, flowNodeMap, edgesMap, 0)
			flowNodeStatusChan := make(chan runner.FlowNodeStatus, FlowNodeStatusChanBufferSize)
			flowStatusChan := make(chan runner.FlowStatus, FlowStatusChanBufferSize)
			_ = runnerLocal.Run(context.Background(), flowNodeStatusChan, flowStatusChan)
		}
	})

	// Benchmark async mode
	b.Run("Async", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			runnerLocal := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), node1ID, flowNodeMap, edgesMap, 0)
			flowNodeStatusChan := make(chan runner.FlowNodeStatus, FlowNodeStatusChanBufferSize)
			flowStatusChan := make(chan runner.FlowStatus, FlowStatusChanBufferSize)
			_ = runnerLocal.Run(context.Background(), flowNodeStatusChan, flowStatusChan)
		}
	})
}

// TestMultipleIncomingEdges verifies that nodes with multiple incoming edges
// are only executed after all their dependencies have completed

func TestMultipleIncomingEdges(t *testing.T) {
	// Create a diamond-shaped flow:
	//    node1
	//   /     \
	// node2   node3
	//   \     /
	//    node4
	// Node4 should only execute after both node2 and node3 have completed

	executionOrder := make(map[idwrap.IDWrap]int)
	executionCounter := 0
	executionMutex := &sync.Mutex{}

	onRun := func(nodeID idwrap.IDWrap) func() {
		return func() {
			executionMutex.Lock()
			executionCounter++
			executionOrder[nodeID] = executionCounter
			executionMutex.Unlock()
			time.Sleep(10 * time.Millisecond) // Small delay to ensure predictable execution order
		}
	}

	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()
	node3ID := idwrap.NewNow()
	node4ID := idwrap.NewNow()

	mockNode1 := mocknode.NewMockNode(node1ID, []idwrap.IDWrap{node2ID, node3ID}, onRun(node1ID))
	mockNode2 := mocknode.NewMockNode(node2ID, []idwrap.IDWrap{node4ID}, onRun(node2ID))
	mockNode3 := mocknode.NewMockNode(node3ID, []idwrap.IDWrap{node4ID}, onRun(node3ID))
	mockNode4 := mocknode.NewMockNode(node4ID, nil, onRun(node4ID))

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

	// Test both sync and async versions
	testCases := []struct {
		name    string
		timeout time.Duration
	}{
		{"Sync", 0},
		{"Async", time.Second},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset counters for each test case
			executionCounter = 0
			executionOrder = make(map[idwrap.IDWrap]int)

			runnerLocal := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), node1ID, flowNodeMap, edgesMap, tc.timeout)
			flowNodeStatusChan := make(chan runner.FlowNodeStatus, FlowNodeStatusChanBufferSize)
			flowStatusChan := make(chan runner.FlowStatus, FlowStatusChanBufferSize)
			err := runnerLocal.Run(context.Background(), flowNodeStatusChan, flowStatusChan)
			if err != nil {
				t.Errorf("Expected err to be nil, but got %v", err)
			}

			// Verify node1 runs first
			if executionOrder[node1ID] != 1 {
				t.Errorf("Expected node1 to be executed first, but it was executed at position %d", executionOrder[node1ID])
			}

			// Verify node2 and node3 run after node1 but before node4
			if executionOrder[node2ID] <= executionOrder[node1ID] || executionOrder[node2ID] >= executionOrder[node4ID] {
				t.Errorf("Expected node2 to be executed after node1 and before node4, but got: node1=%d, node2=%d, node4=%d",
					executionOrder[node1ID], executionOrder[node2ID], executionOrder[node4ID])
			}

			if executionOrder[node3ID] <= executionOrder[node1ID] || executionOrder[node3ID] >= executionOrder[node4ID] {
				t.Errorf("Expected node3 to be executed after node1 and before node4, but got: node1=%d, node3=%d, node4=%d",
					executionOrder[node1ID], executionOrder[node3ID], executionOrder[node4ID])
			}

			// Verify node4 runs last (order must be 4)
			if executionOrder[node4ID] != 4 {
				t.Errorf("Expected node4 to be executed last (position 4), but it was executed at position %d", executionOrder[node4ID])
			}

			// Make sure we get status updates for all nodes
			nodeStatuses := make(map[idwrap.IDWrap]mnnode.NodeState)
			for status := range flowNodeStatusChan {
				if status.State == mnnode.NODE_STATE_SUCCESS {
					nodeStatuses[status.NodeID] = status.State
				}
			}

			if len(nodeStatuses) != 4 {
				t.Errorf("Expected statuses for 4 nodes, but got %d", len(nodeStatuses))
			}
		})
	}
}
