package nif_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/mocknode"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/nif"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcondition"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
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

	edge1 := mflow.NewEdge(idwrap.NewNow(), id, mockNode1ID, mflow.HandleThen)
	edge2 := mflow.NewEdge(idwrap.NewNow(), id, mockNode2ID, mflow.HandleElse)
	edges := []mflow.Edge{edge1, edge2}
	edgesMap := mflow.NewEdgesMap(edges)

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

	edge1 := mflow.NewEdge(idwrap.NewNow(), id, mockNode1ID, mflow.HandleThen)
	edge2 := mflow.NewEdge(idwrap.NewNow(), id, mockNode2ID, mflow.HandleElse)
	edges := []mflow.Edge{edge1, edge2}
	edgesMap := mflow.NewEdgesMap(edges)

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

	edge1 := mflow.NewEdge(idwrap.NewNow(), id, mockNode1ID, mflow.HandleThen)
	edgesMap := mflow.NewEdgesMap([]mflow.Edge{edge1})

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

	edge1 := mflow.NewEdge(idwrap.NewNow(), id, mockNode1ID, mflow.HandleThen)
	edgesMap := mflow.NewEdgesMap([]mflow.Edge{edge1})

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

	edgeElse := mflow.NewEdge(idwrap.NewNow(), id, mockElseID, mflow.HandleElse)
	edgesMap := mflow.NewEdgesMap([]mflow.Edge{edgeElse})

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

	edgeElse := mflow.NewEdge(idwrap.NewNow(), id, mockElseID, mflow.HandleElse)
	edgesMap := mflow.NewEdgesMap([]mflow.Edge{edgeElse})

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

	edge1 := mflow.NewEdge(idwrap.NewNow(), id, mockNode1ID, mflow.HandleThen)
	edge2 := mflow.NewEdge(idwrap.NewNow(), id, mockNode2ID, mflow.HandleElse)
	edges := []mflow.Edge{edge1, edge2}
	edgesMap := mflow.NewEdgesMap(edges)

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

	edge1 := mflow.NewEdge(idwrap.NewNow(), id, mockNode1ID, mflow.HandleThen)
	edge2 := mflow.NewEdge(idwrap.NewNow(), id, mockNode2ID, mflow.HandleElse)
	edges := []mflow.Edge{edge1, edge2}
	edgesMap := mflow.NewEdgesMap(edges)

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
