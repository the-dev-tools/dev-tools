package nfor_test

/*
func TestForNode_RunSync(t *testing.T) {
	mockNode1ID := idwrap.NewNow()
	mockNode2ID := idwrap.NewNow()
	mockNode3ID := idwrap.NewNow()

	var runCounter int
	testFuncInc := func() {
		runCounter++
	}

	mockNode1 := mocknode.NewMockNode(mockNode1ID, &mockNode2ID, testFuncInc)
	mockNode2 := mocknode.NewMockNode(mockNode2ID, &mockNode3ID, testFuncInc)
	mockNode3 := mocknode.NewMockNode(mockNode3ID, nil, testFuncInc)

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		mockNode1ID: mockNode1,
		mockNode2ID: mockNode2,
		mockNode3ID: mockNode3,
	}

	id := idwrap.NewNow()
	name := "test"
	iterCount := int64(3)

	timeOut := time.Duration(0)

	nodeFor := nfor.New(id, name, iterCount, timeOut)
	ctx := context.Background()

	edge1 := edge.NewEdge(idwrap.NewNow(), mockNode1ID, mockNode2ID, edge.HandleUnspecified)
	edge2 := edge.NewEdge(idwrap.NewNow(), mockNode2ID, mockNode3ID, edge.HandleUnspecified)
	edge3 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleLoop)
	edges := []edge.Edge{edge1, edge2, edge3}
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
	if runCounter != 9 {
		t.Errorf("Expected runCounter to be 9, but got %d", runCounter)
	}
}

func TestForNode_RunAsync(t *testing.T) {
	mockNode1ID := idwrap.NewNow()
	mockNode2ID := idwrap.NewNow()
	mockNode3ID := idwrap.NewNow()

	var runCounter int
	testFuncInc := func() {
		runCounter++
	}

	mockNode1 := mocknode.NewMockNode(mockNode1ID, &mockNode2ID, testFuncInc)
	mockNode2 := mocknode.NewMockNode(mockNode2ID, &mockNode3ID, testFuncInc)
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

	name := "test"
	iterCount := int64(3)
	nodeFor := nfor.New(id, name, iterCount, time.Minute)

	ctx := context.Background()

	req := &node.FlowNodeRequest{
		VarMap:        map[string]interface{}{},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
	}

	resultChan := make(chan node.FlowNodeResult, 1)
	go nodeFor.RunAsync(ctx, req, resultChan)
	result := <-resultChan
	if result.Err != nil {
		t.Errorf("Expected err to be nil, but got %v", result.Err)
	}
	if runCounter != 9 {
		t.Errorf("Expected runCounter to be 9, but got %d", runCounter)
	}
}

func TestForNode_SetID(t *testing.T) {
	id := idwrap.NewNow()
	nodeFor := nfor.New(id, "test", 1, time.Minute)
	nodeFor.SetID(id)
	if nodeFor.GetID() != id {
		t.Errorf("Expected nodeFor.GetID() to be %v, but got %v", id, nodeFor.GetID())
	}
}
*/
