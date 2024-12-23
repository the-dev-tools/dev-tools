package nif_test

import (
	"context"
	"fmt"
	"testing"
	"the-dev-tools/backend/pkg/flow/edge"
	"the-dev-tools/backend/pkg/flow/node"
	"the-dev-tools/backend/pkg/flow/node/mocknode"
	"the-dev-tools/backend/pkg/flow/node/nif"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mnode/mnif"
	"the-dev-tools/backend/pkg/testutil"
)

func TestForNode_RunSync_true(t *testing.T) {
	mockNode1ID := idwrap.NewNow()
	mockNode2ID := idwrap.NewNow()

	var runCounter int
	testFuncInc := func() {
		fmt.Println("testFuncInc")
		runCounter++
	}

	mockNode1 := mocknode.NewMockNode(mockNode1ID, nil, testFuncInc)
	mockNode2 := mocknode.NewMockNode(mockNode2ID, nil, testFuncInc)

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		mockNode1ID: mockNode1,
		mockNode2ID: mockNode2,
	}

	id := idwrap.NewNow()
	name := "test"

	nodeFor := nif.New(id, name, mnif.ConditionTypeEqual, "1", "1")
	ctx := context.Background()

	edge1 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleTrue)
	edge2 := edge.NewEdge(idwrap.NewNow(), id, mockNode2ID, edge.HandleFalse)
	edges := []edge.Edge{edge1, edge2}
	edgesMap := edge.NewEdgesMap(edges)

	req := &node.FlowNodeRequest{
		VarMap:        map[string]interface{}{},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
	}

	resault := nodeFor.RunSync(ctx, req)
	if resault.Err != nil {
		t.Errorf("Expected err to be nil, but got %v", resault.Err)
	}
	testutil.Assert(t, mockNode1ID, *resault.NextNodeID)
}

func TestForNode_RunSync_false(t *testing.T) {
	mockNode1ID := idwrap.NewNow()
	mockNode2ID := idwrap.NewNow()

	var runCounter int
	testFuncInc := func() {
		fmt.Println("testFuncInc")
		runCounter++
	}

	mockNode1 := mocknode.NewMockNode(mockNode1ID, nil, testFuncInc)
	mockNode2 := mocknode.NewMockNode(mockNode2ID, nil, testFuncInc)

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		mockNode1ID: mockNode1,
		mockNode2ID: mockNode2,
	}

	id := idwrap.NewNow()
	name := "test"

	nodeFor := nif.New(id, name, mnif.ConditionTypeEqual, "2", "1")
	ctx := context.Background()

	edge1 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleTrue)
	edge2 := edge.NewEdge(idwrap.NewNow(), id, mockNode2ID, edge.HandleFalse)
	edges := []edge.Edge{edge1, edge2}
	edgesMap := edge.NewEdgesMap(edges)

	req := &node.FlowNodeRequest{
		VarMap:        map[string]interface{}{},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
	}

	resault := nodeFor.RunSync(ctx, req)
	if resault.Err != nil {
		t.Errorf("Expected err to be nil, but got %v", resault.Err)
	}
	testutil.Assert(t, mockNode2ID, *resault.NextNodeID)
}

func TestForNode_RunSync_VarTrue(t *testing.T) {
	mockNode1ID := idwrap.NewNow()
	mockNode2ID := idwrap.NewNow()

	var runCounter int
	testFuncInc := func() {
		fmt.Println("testFuncInc")
		runCounter++
	}

	mockNode1 := mocknode.NewMockNode(mockNode1ID, nil, testFuncInc)
	mockNode2 := mocknode.NewMockNode(mockNode2ID, nil, testFuncInc)

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		mockNode1ID: mockNode1,
		mockNode2ID: mockNode2,
	}

	id := idwrap.NewNow()
	name := "test"

	nodeFor := nif.New(id, name, mnif.ConditionTypeEqual, "var.a", "1")
	ctx := context.Background()

	edge1 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleTrue)
	edge2 := edge.NewEdge(idwrap.NewNow(), id, mockNode2ID, edge.HandleFalse)
	edges := []edge.Edge{edge1, edge2}
	edgesMap := edge.NewEdgesMap(edges)

	req := &node.FlowNodeRequest{
		VarMap: map[string]interface{}{
			"a": 1,
		},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
	}

	resault := nodeFor.RunSync(ctx, req)
	if resault.Err != nil {
		t.Errorf("Expected err to be nil, but got %v", resault.Err)
	}
	testutil.Assert(t, mockNode1ID, *resault.NextNodeID)
}

func TestForNode_RunSync_VarFalse(t *testing.T) {
	mockNode1ID := idwrap.NewNow()
	mockNode2ID := idwrap.NewNow()

	var runCounter int
	testFuncInc := func() {
		fmt.Println("testFuncInc")
		runCounter++
	}

	mockNode1 := mocknode.NewMockNode(mockNode1ID, nil, testFuncInc)
	mockNode2 := mocknode.NewMockNode(mockNode2ID, nil, testFuncInc)

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		mockNode1ID: mockNode1,
		mockNode2ID: mockNode2,
	}

	id := idwrap.NewNow()
	name := "test"

	nodeFor := nif.New(id, name, mnif.ConditionTypeEqual, "var.a", "1")
	ctx := context.Background()

	edge1 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleTrue)
	edge2 := edge.NewEdge(idwrap.NewNow(), id, mockNode2ID, edge.HandleFalse)
	edges := []edge.Edge{edge1, edge2}
	edgesMap := edge.NewEdgesMap(edges)

	req := &node.FlowNodeRequest{
		VarMap: map[string]interface{}{
			"a": 2,
		},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
	}

	resault := nodeFor.RunSync(ctx, req)
	if resault.Err != nil {
		t.Errorf("Expected err to be nil, but got %v", resault.Err)
	}
	testutil.Assert(t, mockNode2ID, *resault.NextNodeID)
}
