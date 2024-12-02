package flowlocalrunner_test

import (
	"testing"
	"the-dev-tools/backend/pkg/flow/node"
	"the-dev-tools/backend/pkg/flow/node/mocknode"
	"the-dev-tools/backend/pkg/flow/runner/flowlocalrunner"
	"the-dev-tools/backend/pkg/idwrap"
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
	mockNode1 := mocknode.NewMockNode(node1ID, &node2ID, onRun)
	mockNode2 := mocknode.NewMockNode(node2ID, &node3ID, onRun)
	mockNode3 := mocknode.NewMockNode(node3ID, nil, onRun)

	flowNodeMap := map[idwrap.IDWrap]node.FlowNode{
		node1ID: mockNode1,
		node2ID: mockNode2,
		node3ID: mockNode3,
	}
	runner := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), node1ID, flowNodeMap)
	err := runner.Run(nil)
	if err != nil {
		t.Errorf("Expected err to be nil, but got %v", err)
	}
	if runCounter != CountShouldRun {
		t.Errorf("Expected runCounter to be %d, but got %d", CountShouldRun, runCounter)
	}
}

func TestLocalFlowRunner_Run_NonFull(t *testing.T) {
	var runCounter int
	onRun := func() {
		runCounter++
	}

	const CountShouldRun = 3

	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()
	node3ID := idwrap.NewNow()
	node4ID := idwrap.NewNow()
	mockNode1 := mocknode.NewMockNode(node1ID, &node2ID, onRun)
	mockNode2 := mocknode.NewMockNode(node2ID, &node3ID, onRun)
	mockNode3 := mocknode.NewMockNode(node3ID, nil, onRun)
	mockNode4 := mocknode.NewMockNode(node4ID, &node1ID, onRun)

	flowNodeMap := map[idwrap.IDWrap]node.FlowNode{
		node1ID: mockNode1,
		node2ID: mockNode2,
		node3ID: mockNode3,
		node4ID: mockNode4,
	}
	runner := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), node1ID, flowNodeMap)
	err := runner.Run(nil)
	if err != nil {
		t.Errorf("Expected err to be nil, but got %v", err)
	}
	if runCounter != CountShouldRun {
		t.Errorf("Expected runCounter to be %d, but got %d", CountShouldRun, runCounter)
	}
}
