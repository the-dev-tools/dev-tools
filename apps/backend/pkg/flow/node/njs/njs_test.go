package njs_test

import (
	"context"
	"testing"
	"the-dev-tools/backend/pkg/flow/edge"
	"the-dev-tools/backend/pkg/flow/node"
	"the-dev-tools/backend/pkg/flow/node/mocknode"
	"the-dev-tools/backend/pkg/flow/node/njs"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/testutil"
)

func TestNodeJS_RunSync_SetVariable(t *testing.T) {
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
	jsCode := `setVal("result", "hello world");`
	nodeJS := njs.New(id, "test-node", jsCode)
	ctx := context.Background()

	edge1 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleUnspecified)
	edges := []edge.Edge{edge1}
	edgesMap := edge.NewEdgesMap(edges)

	req := &node.FlowNodeRequest{
		VarMap:        map[string]any{},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
	}

	result := nodeJS.RunSync(ctx, req)
	if result.Err != nil {
		t.Errorf("Expected err to be nil, but got %v", result.Err)
	}
	testutil.Assert(t, mockNode1ID, result.NextNodeID[0])

	// Check that the variable was set correctly
	expectedValue := "hello world"
	if val, ok := req.VarMap["result"]; !ok || val != expectedValue {
		t.Errorf("Expected VarMap['result'] to be %v, but got %v", expectedValue, val)
	}
}

func TestNodeJS_RunSync_GetVariable(t *testing.T) {
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
	jsCode := `
		const value = getVal("input");
		setVal("output", value + " processed");
	`
	nodeJS := njs.New(id, "test-node", jsCode)
	ctx := context.Background()

	edge1 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleUnspecified)
	edges := []edge.Edge{edge1}
	edgesMap := edge.NewEdgesMap(edges)

	req := &node.FlowNodeRequest{
		VarMap: map[string]any{
			"input": "test data",
		},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
	}

	result := nodeJS.RunSync(ctx, req)
	if result.Err != nil {
		t.Errorf("Expected err to be nil, but got %v", result.Err)
	}
	testutil.Assert(t, mockNode1ID, result.NextNodeID[0])

	// Check that the variable was processed and set correctly
	expectedValue := "test data processed"
	if val, ok := req.VarMap["output"]; !ok || val != expectedValue {
		t.Errorf("Expected VarMap['output'] to be %v, but got %v", expectedValue, val)
	}
}

func TestNodeJS_RunSync_DifferentTypes(t *testing.T) {
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
	jsCode := `
		setVal("stringVal", "string");
		setVal("intVal", 42);
		setVal("floatVal", 3.14);
		setVal("boolVal", true);
	`
	nodeJS := njs.New(id, "test-node", jsCode)
	ctx := context.Background()

	edge1 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleUnspecified)
	edges := []edge.Edge{edge1}
	edgesMap := edge.NewEdgesMap(edges)

	req := &node.FlowNodeRequest{
		VarMap:        map[string]any{},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
	}

	result := nodeJS.RunSync(ctx, req)
	if result.Err != nil {
		t.Errorf("Expected err to be nil, but got %v", result.Err)
	}
	testutil.Assert(t, mockNode1ID, result.NextNodeID[0])

	// Check that variables of different types were set correctly
	if val, ok := req.VarMap["stringVal"]; !ok || val != "string" {
		t.Errorf("Expected VarMap['stringVal'] to be 'string', but got %v", val)
	}

	if val, ok := req.VarMap["intVal"]; !ok || val != int32(42) {
		t.Errorf("Expected VarMap['intVal'] to be 42, but got %v", val)
	}

	if val, ok := req.VarMap["floatVal"]; !ok || val != 3.14 {
		t.Errorf("Expected VarMap['floatVal'] to be 3.14, but got %v", val)
	}

	if val, ok := req.VarMap["boolVal"]; !ok || val != true {
		t.Errorf("Expected VarMap['boolVal'] to be true, but got %v", val)
	}
}

func TestNodeJS_RunSync_Computation(t *testing.T) {
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
	jsCode := `
		const a = 10;
		const b = 5;
		setVal("sum", a + b);
		setVal("difference", a - b);
		setVal("product", a * b);
		setVal("quotient", a / b);
	`
	nodeJS := njs.New(id, "test-node", jsCode)
	ctx := context.Background()

	edge1 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleUnspecified)
	edges := []edge.Edge{edge1}
	edgesMap := edge.NewEdgesMap(edges)

	req := &node.FlowNodeRequest{
		VarMap:        map[string]any{},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
	}

	result := nodeJS.RunSync(ctx, req)
	if result.Err != nil {
		t.Errorf("Expected err to be nil, but got %v", result.Err)
	}
	testutil.Assert(t, mockNode1ID, result.NextNodeID[0])

	// Check computation results
	if val, ok := req.VarMap["sum"]; !ok || val != int32(15) {
		t.Errorf("Expected VarMap['sum'] to be 15, but got %v", val)
	}

	if val, ok := req.VarMap["difference"]; !ok || val != int32(5) {
		t.Errorf("Expected VarMap['difference'] to be 5, but got %v", val)
	}

	if val, ok := req.VarMap["product"]; !ok || val != int32(50) {
		t.Errorf("Expected VarMap['product'] to be 50, but got %v", val)
	}

	if val, ok := req.VarMap["quotient"]; !ok || val != int32(2) {
		t.Errorf("Expected VarMap['quotient'] to be 2, but got %v", val)
	}
}

func TestNodeJS_RunAsync(t *testing.T) {
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
	jsCode := `setVal("async", true);`
	nodeJS := njs.New(id, "test-node", jsCode)
	ctx := context.Background()

	edge1 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleUnspecified)
	edges := []edge.Edge{edge1}
	edgesMap := edge.NewEdgesMap(edges)

	req := &node.FlowNodeRequest{
		VarMap:        map[string]any{},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
	}

	resultChan := make(chan node.FlowNodeResult, 1)
	nodeJS.RunAsync(ctx, req, resultChan)

	result := <-resultChan
	if result.Err != nil {
		t.Errorf("Expected err to be nil, but got %v", result.Err)
	}
	testutil.Assert(t, mockNode1ID, result.NextNodeID[0])
}

func TestNodeJS_RunSync_RestrictedOperations(t *testing.T) {
	mockNode1ID := idwrap.NewNow()
	mockNode1 := mocknode.NewMockNode(mockNode1ID, nil, func() {})

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		mockNode1ID: mockNode1,
	}

	id := idwrap.NewNow()
	// Try to use restricted operations like file system and network access
	jsCode := `
		let fileAccessError = "";
		let fetchError = "";

		try {
			// Attempt file system access
			const fs = require('fs');
			fs.readFileSync('/etc/passwd');
		} catch (err) {
			fileAccessError = err.toString();
		}

		try {
			// Attempt network access
			fetch('https://example.com');
		} catch (err) {
			fetchError = err.toString();
		}

		setVal("fileAccessError", fileAccessError);
		setVal("fetchError", fetchError);
	`

	nodeJS := njs.New(id, "test-node", jsCode)
	ctx := context.Background()

	edge1 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleUnspecified)
	edges := []edge.Edge{edge1}
	edgesMap := edge.NewEdgesMap(edges)

	req := &node.FlowNodeRequest{
		VarMap:        map[string]any{},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
	}

	result := nodeJS.RunSync(ctx, req)
	if result.Err != nil {
		t.Errorf("Expected err to be nil, but got %v", result.Err)
	}

	// Verify that both operations were blocked
	if val, ok := req.VarMap["fileAccessError"]; !ok || val == "" {
		t.Error("Expected file system access to be blocked with an error")
	}

	if val, ok := req.VarMap["fetchError"]; !ok || val == "" {
		t.Error("Expected network access to be blocked with an error")
	}
}
