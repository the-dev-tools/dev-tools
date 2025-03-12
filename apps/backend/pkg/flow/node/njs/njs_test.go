package njs_test

import (
	"context"
	"fmt"
	"sync"
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
	key := "result"
	expectedValue := "hello world"
	jsCode := fmt.Sprintf(`%s("%s", "%s");`, njs.SetValFuncName, key, expectedValue)
	nodeJS := njs.New(id, "test-node", jsCode)
	ctx := context.Background()

	edge1 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleUnspecified)
	edges := []edge.Edge{edge1}
	edgesMap := edge.NewEdgesMap(edges)

	req := &node.FlowNodeRequest{
		ReadWriteLock: &sync.RWMutex{},
		VarMap:        map[string]any{},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
	}

	result := nodeJS.RunSync(ctx, req)
	if result.Err != nil {
		t.Errorf("Expected err to be nil, but got %v", result.Err)
	}
	testutil.Assert(t, mockNode1ID, result.NextNodeID[0])

	fmt.Println("req.VarMap", req.VarMap)

	testVal, err := node.ReadNodeVar(req, id, key)
	if err != nil {
		t.Errorf("Expected err to be nil, but got %v", err)
	}
	if testVal != expectedValue {
		t.Errorf("Expected %v, but got %v", expectedValue, testVal)
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
	inputKey := "input"
	outputKey := "output"
	inputVal := "test data"
	outputAdd := "_processed"
	expectedValue := inputVal + outputAdd
	jsCode := fmt.Sprintf(`
		const value = %s("%s");
		%s("%s", value + "%s");
	`, njs.GetValFuncName, inputKey,
		njs.SetValFuncName, outputKey, outputAdd)
	nodeJS := njs.New(id, "test-node", jsCode)
	ctx := context.Background()

	edge1 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleUnspecified)
	edges := []edge.Edge{edge1}
	edgesMap := edge.NewEdgesMap(edges)

	req := &node.FlowNodeRequest{
		ReadWriteLock: &sync.RWMutex{},
		VarMap:        map[string]any{},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
	}

	// Set the input variable
	key := "input"
	err := node.WriteNodeVar(req, id, key, "test data")
	if err != nil {
		t.Errorf("Failed to set input variable: %v", err)
	}

	result := nodeJS.RunSync(ctx, req)
	if result.Err != nil {
		t.Errorf("Expected err to be nil, but got %v", result.Err)
	}
	testutil.Assert(t, mockNode1ID, result.NextNodeID[0])

	fmt.Println("req.VarMap", req.VarMap)
	outputVal, err := node.ReadNodeVar(req, id, outputKey)
	if err != nil {
		t.Errorf("Expected err to be nil, but got %v", err)
	}
	if outputVal != expectedValue {
		t.Errorf("Expected output to be %v, but got %v", expectedValue, outputVal)
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
	jsCode := fmt.Sprintf(`
		%s("stringVal", "string");
		%s("intVal", 42);
		%s("floatVal", 3.14);
		%s("boolVal", true);
	`, njs.SetValFuncName, njs.SetValFuncName, njs.SetValFuncName, njs.SetValFuncName)
	nodeJS := njs.New(id, "test-node", jsCode)
	ctx := context.Background()

	edge1 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleUnspecified)
	edges := []edge.Edge{edge1}
	edgesMap := edge.NewEdgesMap(edges)

	req := &node.FlowNodeRequest{
		ReadWriteLock: &sync.RWMutex{},
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
	stringVal, err := node.ReadNodeVar(req, id, "stringVal")
	if err != nil || stringVal != "string" {
		t.Errorf("Expected stringVal to be 'string', but got %v (err: %v)", stringVal, err)
	}

	intVal, err := node.ReadNodeVar(req, id, "intVal")
	if err != nil || intVal != int32(42) {
		t.Errorf("Expected intVal to be 42, but got %v (err: %v)", intVal, err)
	}

	floatVal, err := node.ReadNodeVar(req, id, "floatVal")
	if err != nil || floatVal != 3.14 {
		t.Errorf("Expected floatVal to be 3.14, but got %v (err: %v)", floatVal, err)
	}

	boolVal, err := node.ReadNodeVar(req, id, "boolVal")
	if err != nil || boolVal != true {
		t.Errorf("Expected boolVal to be true, but got %v (err: %v)", boolVal, err)
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
	jsCode := fmt.Sprintf(`
		const a = 10;
		const b = 5;
		%s("sum", a + b);
		%s("difference", a - b);
		%s("product", a * b);
		%s("quotient", a / b);
	`, njs.SetValFuncName, njs.SetValFuncName, njs.SetValFuncName, njs.SetValFuncName)
	nodeJS := njs.New(id, "test-node", jsCode)
	ctx := context.Background()

	edge1 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleUnspecified)
	edges := []edge.Edge{edge1}
	edgesMap := edge.NewEdgesMap(edges)

	req := &node.FlowNodeRequest{
		ReadWriteLock: &sync.RWMutex{},
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
	sumVal, err := node.ReadNodeVar(req, id, "sum")
	if err != nil || sumVal != int32(15) {
		t.Errorf("Expected sum to be 15, but got %v (err: %v)", sumVal, err)
	}

	diffVal, err := node.ReadNodeVar(req, id, "difference")
	if err != nil || diffVal != int32(5) {
		t.Errorf("Expected difference to be 5, but got %v (err: %v)", diffVal, err)
	}

	prodVal, err := node.ReadNodeVar(req, id, "product")
	if err != nil || prodVal != int32(50) {
		t.Errorf("Expected product to be 50, but got %v (err: %v)", prodVal, err)
	}

	quotVal, err := node.ReadNodeVar(req, id, "quotient")
	if err != nil || quotVal != int32(2) {
		t.Errorf("Expected quotient to be 2, but got %v (err: %v)", quotVal, err)
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
	jsCode := fmt.Sprintf(`%s("async", true);`, njs.SetValFuncName)
	nodeJS := njs.New(id, "test-node", jsCode)
	ctx := context.Background()

	edge1 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleUnspecified)
	edges := []edge.Edge{edge1}
	edgesMap := edge.NewEdgesMap(edges)

	req := &node.FlowNodeRequest{
		ReadWriteLock: &sync.RWMutex{},
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

	asyncVal, err := node.ReadNodeVar(req, id, "async")
	if err != nil || asyncVal != true {
		t.Errorf("Expected async to be true, but got %v (err: %v)", asyncVal, err)
	}
}

// TODO: fetch should be more restricted in future
func TestNodeJS_RunSync_Fetch(t *testing.T) {
	mockNode1ID := idwrap.NewNow()
	mockNode1 := mocknode.NewMockNode(mockNode1ID, nil, func() {})

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		mockNode1ID: mockNode1,
	}

	id := idwrap.NewNow()
	// Try to use restricted operations like file system and network access
	jsCode := fmt.Sprintf(`
		let fetchError = "";
		try {
			// Attempt network access
			fetch('https://example.com');
		} catch (err) {
			fetchError = err.toString();
		}

		%s("fetchError", fetchError);
	`, njs.SetValFuncName)

	nodeJS := njs.New(id, "test-node", jsCode)
	ctx := context.Background()

	edge1 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleUnspecified)
	edges := []edge.Edge{edge1}
	edgesMap := edge.NewEdgesMap(edges)

	req := &node.FlowNodeRequest{
		ReadWriteLock: &sync.RWMutex{},
		VarMap:        map[string]any{},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
	}

	result := nodeJS.RunSync(ctx, req)
	if result.Err != nil {
		t.Errorf("Expected err to be nil, but got %v", result.Err)
	}

	fetchError, err := node.ReadNodeVar(req, id, "fetchError")
	if err != nil {
		t.Errorf("Expected network access, got: %v (err: %v)", fetchError, err)
	}
}

func TestNodeJS_RunSync_RestrictedOperations(t *testing.T) {
	mockNode1ID := idwrap.NewNow()
	mockNode1 := mocknode.NewMockNode(mockNode1ID, nil, func() {})

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		mockNode1ID: mockNode1,
	}

	id := idwrap.NewNow()
	// Try to use restricted operations like file system and network access
	jsCode := fmt.Sprintf(`
		let fileAccessError = "";

		try {
			// Attempt file system access
			const fs = require('fs');
			fs.readFileSync('/etc/passwd');
		} catch (err) {
			fileAccessError = err.toString();
		}


		%s("fileAccessError", fileAccessError);
	`, njs.SetValFuncName)

	nodeJS := njs.New(id, "test-node", jsCode)
	ctx := context.Background()

	edge1 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleUnspecified)
	edges := []edge.Edge{edge1}
	edgesMap := edge.NewEdgesMap(edges)

	req := &node.FlowNodeRequest{
		ReadWriteLock: &sync.RWMutex{},
		VarMap:        map[string]any{},
		NodeMap:       nodeMap,
		EdgeSourceMap: edgesMap,
	}

	result := nodeJS.RunSync(ctx, req)
	if result.Err != nil {
		t.Errorf("Expected err to be nil, but got %v", result.Err)
	}

	// Verify that both operations were blocked
	fileError, err := node.ReadNodeVar(req, id, "fileAccessError")
	if err != nil || fileError == "" {
		t.Errorf("Expected file system access to be blocked with an error, got: %v (err: %v)", fileError, err)
	}
}
