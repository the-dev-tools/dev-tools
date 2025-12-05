package nif_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/node/mocknode"
	"the-dev-tools/server/pkg/flow/node/nif"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcondition"
)

func TestForNode_RunSync_true(t *testing.T) {
	mockNode1ID := idwrap.NewNow()
	mockNode2ID := idwrap.NewNow()

	var runCounter int
	testFuncInc := func() {
		runCounter++
	}

	mockNode1 := mocknode.NewMockNode(mockNode1ID, nil, testFuncInc)
	mockNode2 := mocknode.NewMockNode(mockNode2ID, nil, testFuncInc)

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		mockNode1ID: mockNode1,
		mockNode2ID: mockNode2,
	}

	id := idwrap.NewNow()
	nodeName := "test-node"

	nodeFor := nif.New(id, nodeName, mcondition.Condition{
		Comparisons: mcondition.Comparison{
			Expression: "1 == 1",
		},
	})
	ctx := context.Background()

	edge1 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleThen, edge.EdgeKindUnspecified)
	edge2 := edge.NewEdge(idwrap.NewNow(), id, mockNode2ID, edge.HandleElse, edge.EdgeKindUnspecified)
	edges := []edge.Edge{edge1, edge2}
	edgesMap := edge.NewEdgesMap(edges)

	req := &node.FlowNodeRequest{
		VarMap:        map[string]interface{}{},
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
	}

	resault := nodeFor.RunSync(ctx, req)
	require.NoError(t, resault.Err)
	require.Equal(t, mockNode1ID, resault.NextNodeID[0])
}

func TestForNode_RunSync_false(t *testing.T) {
	mockNode1ID := idwrap.NewNow()
	mockNode2ID := idwrap.NewNow()

	var runCounter int
	testFuncInc := func() {
		runCounter++
	}

	mockNode1 := mocknode.NewMockNode(mockNode1ID, nil, testFuncInc)
	mockNode2 := mocknode.NewMockNode(mockNode2ID, nil, testFuncInc)

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		mockNode1ID: mockNode1,
		mockNode2ID: mockNode2,
	}

	id := idwrap.NewNow()
	nodeName := "test-node"

	nodeFor := nif.New(id, nodeName, mcondition.Condition{
		Comparisons: mcondition.Comparison{
			Expression: "2 == 1",
		},
	})
	ctx := context.Background()

	edge1 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleThen, edge.EdgeKindUnspecified)
	edge2 := edge.NewEdge(idwrap.NewNow(), id, mockNode2ID, edge.HandleElse, edge.EdgeKindUnspecified)
	edges := []edge.Edge{edge1, edge2}
	edgesMap := edge.NewEdgesMap(edges)

	req := &node.FlowNodeRequest{
		VarMap:        map[string]interface{}{},
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
	}

	resault := nodeFor.RunSync(ctx, req)
	require.NoError(t, resault.Err)
	require.Equal(t, mockNode2ID, resault.NextNodeID[0])
}

func TestForNode_RunSync_ThenOnlyTrue(t *testing.T) {
	mockNode1ID := idwrap.NewNow()

	var runCounter int
	testFuncInc := func() {
		runCounter++
	}

	mockNode1 := mocknode.NewMockNode(mockNode1ID, nil, testFuncInc)

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		mockNode1ID: mockNode1,
	}

	id := idwrap.NewNow()
	nodeName := "test-node"

	nodeFor := nif.New(id, nodeName, mcondition.Condition{
		Comparisons: mcondition.Comparison{
			Expression: "1 == 1",
		},
	})
	ctx := context.Background()

	edge1 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleThen, edge.EdgeKindUnspecified)
	edgesMap := edge.NewEdgesMap([]edge.Edge{edge1})

	req := &node.FlowNodeRequest{
		VarMap:        map[string]interface{}{},
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
	}

	resault := nodeFor.RunSync(ctx, req)
	require.NoError(t, resault.Err)
	require.Equal(t, mockNode1ID, resault.NextNodeID[0])
}

func TestForNode_RunSync_ThenOnlyFalse(t *testing.T) {
	mockNode1ID := idwrap.NewNow()

	var runCounter int
	testFuncInc := func() {
		runCounter++
	}

	mockNode1 := mocknode.NewMockNode(mockNode1ID, nil, testFuncInc)

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		mockNode1ID: mockNode1,
	}

	id := idwrap.NewNow()
	nodeName := "test-node"

	nodeFor := nif.New(id, nodeName, mcondition.Condition{
		Comparisons: mcondition.Comparison{
			Expression: "1 == 2",
		},
	})
	ctx := context.Background()

	edge1 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleThen, edge.EdgeKindUnspecified)
	edgesMap := edge.NewEdgesMap([]edge.Edge{edge1})

	req := &node.FlowNodeRequest{
		VarMap:        map[string]interface{}{},
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
	}

	resault := nodeFor.RunSync(ctx, req)
	require.NoError(t, resault.Err)
	require.Len(t, resault.NextNodeID, 0)
}

func TestForNode_RunSync_ElseOnlyTrue(t *testing.T) {
	mockElseID := idwrap.NewNow()

	var runCounter int
	testFuncInc := func() {
		runCounter++
	}

	mockElse := mocknode.NewMockNode(mockElseID, nil, testFuncInc)

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		mockElseID: mockElse,
	}

	id := idwrap.NewNow()
	nodeName := "test-node"

	nodeFor := nif.New(id, nodeName, mcondition.Condition{
		Comparisons: mcondition.Comparison{
			Expression: "1 == 1",
		},
	})
	ctx := context.Background()

	edgeElse := edge.NewEdge(idwrap.NewNow(), id, mockElseID, edge.HandleElse, edge.EdgeKindUnspecified)
	edgesMap := edge.NewEdgesMap([]edge.Edge{edgeElse})

	req := &node.FlowNodeRequest{
		VarMap:        map[string]interface{}{},
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
	}

	resault := nodeFor.RunSync(ctx, req)
	require.NoError(t, resault.Err)
	require.Len(t, resault.NextNodeID, 0)
}

func TestForNode_RunSync_ElseOnlyFalse(t *testing.T) {
	mockElseID := idwrap.NewNow()

	var runCounter int
	testFuncInc := func() {
		runCounter++
	}

	mockElse := mocknode.NewMockNode(mockElseID, nil, testFuncInc)

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		mockElseID: mockElse,
	}

	id := idwrap.NewNow()
	nodeName := "test-node"

	nodeFor := nif.New(id, nodeName, mcondition.Condition{
		Comparisons: mcondition.Comparison{
			Expression: "1 == 2",
		},
	})
	ctx := context.Background()

	edgeElse := edge.NewEdge(idwrap.NewNow(), id, mockElseID, edge.HandleElse, edge.EdgeKindUnspecified)
	edgesMap := edge.NewEdgesMap([]edge.Edge{edgeElse})

	req := &node.FlowNodeRequest{
		VarMap:        map[string]interface{}{},
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
	}

	resault := nodeFor.RunSync(ctx, req)
	require.NoError(t, resault.Err)
	require.Equal(t, mockElseID, resault.NextNodeID[0])
}

func TestForNode_RunSync_VarTrue(t *testing.T) {
	mockNode1ID := idwrap.NewNow()
	mockNode2ID := idwrap.NewNow()

	var runCounter int
	testFuncInc := func() {
		runCounter++
	}

	mockNode1 := mocknode.NewMockNode(mockNode1ID, nil, testFuncInc)
	mockNode2 := mocknode.NewMockNode(mockNode2ID, nil, testFuncInc)

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		mockNode1ID: mockNode1,
		mockNode2ID: mockNode2,
	}

	id := idwrap.NewNow()
	nodeName := "test-node"

	nodeFor := nif.New(id, nodeName, mcondition.Condition{
		Comparisons: mcondition.Comparison{
			Expression: "a == 1",
		},
	})
	ctx := context.Background()

	edge1 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleThen, edge.EdgeKindUnspecified)
	edge2 := edge.NewEdge(idwrap.NewNow(), id, mockNode2ID, edge.HandleElse, edge.EdgeKindUnspecified)
	edges := []edge.Edge{edge1, edge2}
	edgesMap := edge.NewEdgesMap(edges)

	req := &node.FlowNodeRequest{
		VarMap: map[string]interface{}{
			"a": 1,
		},
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
	}

	resault := nodeFor.RunSync(ctx, req)
	require.NoError(t, resault.Err)
	require.Equal(t, mockNode1ID, resault.NextNodeID[0])
}

func TestForNode_RunSync_VarFalse(t *testing.T) {
	mockNode1ID := idwrap.NewNow()
	mockNode2ID := idwrap.NewNow()

	var runCounter int
	testFuncInc := func() {
		runCounter++
	}

	mockNode1 := mocknode.NewMockNode(mockNode1ID, nil, testFuncInc)
	mockNode2 := mocknode.NewMockNode(mockNode2ID, nil, testFuncInc)

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		mockNode1ID: mockNode1,
		mockNode2ID: mockNode2,
	}

	id := idwrap.NewNow()
	nodeName := "test-node"

	nodeFor := nif.New(id, nodeName, mcondition.Condition{
		Comparisons: mcondition.Comparison{
			Expression: "a == 1",
		},
	})
	ctx := context.Background()

	edge1 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleThen, edge.EdgeKindUnspecified)
	edge2 := edge.NewEdge(idwrap.NewNow(), id, mockNode2ID, edge.HandleElse, edge.EdgeKindUnspecified)
	edges := []edge.Edge{edge1, edge2}
	edgesMap := edge.NewEdgesMap(edges)

	req := &node.FlowNodeRequest{
		VarMap: map[string]interface{}{
			"a": 2,
		},
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
	}

	resault := nodeFor.RunSync(ctx, req)
	require.NoError(t, resault.Err)
	require.Equal(t, mockNode2ID, resault.NextNodeID[0])
}
