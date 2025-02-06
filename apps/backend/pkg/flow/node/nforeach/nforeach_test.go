package nforeach_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"the-dev-tools/backend/pkg/flow/edge"
	"the-dev-tools/backend/pkg/flow/node"
	"the-dev-tools/backend/pkg/flow/node/mocknode"
	"the-dev-tools/backend/pkg/flow/node/nforeach"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mcondition"
	"the-dev-tools/backend/pkg/model/mnnode/mnfor"
	"time"
)

// TODO: refactor this tests

func TestForEachNode_RunSyncArray(t *testing.T) {
	mockNode1ID := idwrap.NewNow()
	mockNode2ID := idwrap.NewNow()
	mockNode3ID := idwrap.NewNow()

	var runCounter atomic.Int32
	testFuncInc := func() {
		runCounter.Add(1)
	}

	mockNode1 := mocknode.NewMockNode(mockNode1ID, []idwrap.IDWrap{mockNode2ID}, testFuncInc)
	mockNode2 := mocknode.NewMockNode(mockNode2ID, []idwrap.IDWrap{mockNode3ID}, testFuncInc)
	mockNode3 := mocknode.NewMockNode(mockNode3ID, nil, testFuncInc)

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		mockNode1ID: mockNode1,
		mockNode2ID: mockNode2,
		mockNode3ID: mockNode3,
	}

	id := idwrap.NewNow()
	// iterCount := int64(3)

	timeOut := time.Duration(0)

	condition := mcondition.Condition{
		Comparisons: mcondition.Comparison{
			Kind:  mcondition.COMPARISON_KIND_EQUAL,
			Path:  "var.test",
			Value: "test",
		},
	}

	arrPath := "array"

	nodeForEach := nforeach.New(id, "test", "var.array", timeOut, condition, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)
	ctx := context.Background()

	edge1 := edge.NewEdge(idwrap.NewNow(), mockNode1ID, mockNode2ID, edge.HandleUnspecified)
	edge2 := edge.NewEdge(idwrap.NewNow(), mockNode2ID, mockNode3ID, edge.HandleUnspecified)
	edge3 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleLoop)
	edges := []edge.Edge{edge1, edge2, edge3}
	edgesMap := edge.NewEdgesMap(edges)

	logMockFunc := func(node.NodeStatus, idwrap.IDWrap) {
	}

	varMap := map[string]interface{}{
		arrPath: []string{"a", "b", "c"},
		"test":  "test",
	}

	req := &node.FlowNodeRequest{
		VarMap:        varMap,
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
		Timeout:       timeOut,
		LogPushFunc:   logMockFunc,
	}

	resault := nodeForEach.RunSync(ctx, req)
	if resault.Err != nil {
		t.Errorf("Expected err to be nil, but got %v", resault.Err)
	}
	// TODO: fix this test
	//if runCounter.Load() != 9 {
	//	t.Errorf("Expected runCounter to be 9, but got %d", runCounter.Load())
	//}
}

func TestForEachNode_RunAsyncArray(t *testing.T) {
	mockNode1ID := idwrap.NewNow()
	mockNode2ID := idwrap.NewNow()
	mockNode3ID := idwrap.NewNow()

	var runCounter atomic.Int32
	var wg sync.WaitGroup
	testFuncInc := func() {
		runCounter.Add(1)
		wg.Done()
	}

	mockNode1 := mocknode.NewMockNode(mockNode1ID, []idwrap.IDWrap{mockNode2ID}, testFuncInc)
	mockNode2 := mocknode.NewMockNode(mockNode2ID, []idwrap.IDWrap{mockNode3ID}, testFuncInc)
	mockNode3 := mocknode.NewMockNode(mockNode3ID, nil, testFuncInc)

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		mockNode1ID: mockNode1,
		mockNode2ID: mockNode2,
		mockNode3ID: mockNode3,
	}
	id := idwrap.NewNow()

	edge1 := edge.NewEdge(idwrap.NewNow(), mockNode1ID, mockNode2ID, edge.HandleUnspecified)
	edge2 := edge.NewEdge(idwrap.NewNow(), mockNode2ID, mockNode3ID, edge.HandleUnspecified)
	edge3 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleLoop)
	edges := []edge.Edge{edge1, edge2, edge3}
	edgesMap := edge.NewEdgesMap(edges)

	// iterCount := int64(3)
	timeOut := time.Duration(0)

	condition := mcondition.Condition{
		Comparisons: mcondition.Comparison{
			Kind:  mcondition.COMPARISON_KIND_EQUAL,
			Path:  "var.test",
			Value: "test",
		},
	}

	arrPath := "array"

	nodeForEach := nforeach.New(id, "test", "var.array", timeOut, condition, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)

	ctx := context.Background()

	logMockFunc := func(node.NodeStatus, idwrap.IDWrap) {
	}

	varMap := map[string]interface{}{
		arrPath: []string{"a", "b", "c"},
		"test":  "test",
	}

	req := &node.FlowNodeRequest{
		VarMap:        varMap,
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
		LogPushFunc:   logMockFunc,
	}

	wg.Add(9) // Expect 9 runs

	resultChan := make(chan node.FlowNodeResult, 1)
	go nodeForEach.RunAsync(ctx, req, resultChan)

	go func() {
		wg.Wait()
		close(resultChan) // Close the channel after all runs are done
	}()

	result := <-resultChan
	if result.Err != nil {
		t.Errorf("Expected err to be nil, but got %v", result.Err)
	}

	// TODO: fix this test
	//if runCounter.Load() != 9 {
	//	t.Errorf("Expected runCounter to be 9, but got %d", runCounter.Load())
	//}
}

func TestForEachNode_RunSync_Map(t *testing.T) {
	mockNode1ID := idwrap.NewNow()
	mockNode2ID := idwrap.NewNow()
	mockNode3ID := idwrap.NewNow()

	var runCounter atomic.Int32
	testFuncInc := func() {
		runCounter.Add(1)
	}

	mockNode1 := mocknode.NewMockNode(mockNode1ID, []idwrap.IDWrap{mockNode2ID}, testFuncInc)
	mockNode2 := mocknode.NewMockNode(mockNode2ID, []idwrap.IDWrap{mockNode3ID}, testFuncInc)
	mockNode3 := mocknode.NewMockNode(mockNode3ID, nil, testFuncInc)

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		mockNode1ID: mockNode1,
		mockNode2ID: mockNode2,
		mockNode3ID: mockNode3,
	}

	id := idwrap.NewNow()
	timeOut := time.Duration(0)

	// Use a map for iteration by setting IterPath to "var.hash"
	nodeForEach := nforeach.New(id, "test", "var.hash", timeOut, mcondition.Condition{}, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)
	ctx := context.Background()

	edge1 := edge.NewEdge(idwrap.NewNow(), mockNode1ID, mockNode2ID, edge.HandleUnspecified)
	edge2 := edge.NewEdge(idwrap.NewNow(), mockNode2ID, mockNode3ID, edge.HandleUnspecified)
	edge3 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleLoop)
	edges := []edge.Edge{edge1, edge2, edge3}
	edgesMap := edge.NewEdgesMap(edges)

	logMockFunc := func(node.NodeStatus, idwrap.IDWrap) {}

	req := &node.FlowNodeRequest{
		VarMap: map[string]interface{}{
			"hash": map[string]string{
				"a": "valueA",
				"b": "valueB",
				"c": "valueC",
			},
		},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
		Timeout:       timeOut,
		LogPushFunc:   logMockFunc,
	}

	result := nodeForEach.RunSync(ctx, req)
	if result.Err != nil {
		t.Errorf("Expected err to be nil, but got %v", result.Err)
	}
	if runCounter.Load() != 9 {
		t.Errorf("Expected runCounter to be 9, but got %d", runCounter.Load())
	}
}

func TestForEachNode_RunAsync_Map(t *testing.T) {
	mockNode1ID := idwrap.NewNow()
	mockNode2ID := idwrap.NewNow()
	mockNode3ID := idwrap.NewNow()

	var runCounter atomic.Int32
	var wg sync.WaitGroup
	testFuncInc := func() {
		runCounter.Add(1)
		wg.Done()
	}

	mockNode1 := mocknode.NewMockNode(mockNode1ID, []idwrap.IDWrap{mockNode2ID}, testFuncInc)
	mockNode2 := mocknode.NewMockNode(mockNode2ID, []idwrap.IDWrap{mockNode3ID}, testFuncInc)
	mockNode3 := mocknode.NewMockNode(mockNode3ID, nil, testFuncInc)

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		mockNode1ID: mockNode1,
		mockNode2ID: mockNode2,
		mockNode3ID: mockNode3,
	}
	id := idwrap.NewNow()

	edge1 := edge.NewEdge(idwrap.NewNow(), mockNode1ID, mockNode2ID, edge.HandleUnspecified)
	edge2 := edge.NewEdge(idwrap.NewNow(), mockNode2ID, mockNode3ID, edge.HandleUnspecified)
	edge3 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleLoop)
	edges := []edge.Edge{edge1, edge2, edge3}
	edgesMap := edge.NewEdgesMap(edges)

	timeOut := time.Duration(0)
	// Use a map for iteration by setting IterPath to "var.hash"
	nodeForEach := nforeach.New(id, "test", "var.hash", timeOut, mcondition.Condition{}, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)

	ctx := context.Background()

	logMockFunc := func(node.NodeStatus, idwrap.IDWrap) {}

	req := &node.FlowNodeRequest{
		VarMap: map[string]interface{}{
			"hash": map[string]string{
				"a": "valueA",
				"b": "valueB",
				"c": "valueC",
			},
		},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
		LogPushFunc:   logMockFunc,
	}

	wg.Add(9) // Expect 3 iterations since the hashmap has three keys

	resultChan := make(chan node.FlowNodeResult, 1)
	go func() {
		nodeForEach.RunAsync(ctx, req, resultChan)
		wg.Wait() // Ensure all iterations complete before closing the channel
		close(resultChan)
	}()

	result := <-resultChan
	if result.Err != nil {
		t.Errorf("Expected err to be nil, but got %v", result.Err)
	}
	if runCounter.Load() != 9 {
		t.Errorf("Expected runCounter to be 3, but got %d", runCounter.Load())
	}
}

func TestForEachNode_SetID(t *testing.T) {
	id := idwrap.NewNow()
	timeOut := time.Duration(0)

	condition := mcondition.Condition{
		Comparisons: mcondition.Comparison{
			Kind:  mcondition.COMPARISON_KIND_EQUAL,
			Path:  "test",
			Value: "test",
		},
	}
	nodeForEach := nforeach.New(id, "test", "test", timeOut, condition, mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED)
	nodeForEach.SetID(id)
	if nodeForEach.GetID() != id {
		t.Errorf("Expected nodeFor.GetID() to be %v, but got %v", id, nodeForEach.GetID())
	}
}
