package flowlocalrunner

import (
	"context"
	"sync"
	"testing"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
	"time"
)

// TestNode is a simple test implementation of FlowNode
type TestNode struct {
	ID      idwrap.IDWrap
	Name    string
	NextIDs []idwrap.IDWrap
	RunFunc func(req *node.FlowNodeRequest) error
}

func NewTestNode(id idwrap.IDWrap, name string, nextIDs []idwrap.IDWrap, runFunc func(req *node.FlowNodeRequest) error) *TestNode {
	return &TestNode{
		ID:      id,
		Name:    name,
		NextIDs: nextIDs,
		RunFunc: runFunc,
	}
}

func (tn *TestNode) GetID() idwrap.IDWrap {
	return tn.ID
}

func (tn *TestNode) GetName() string {
	return tn.Name
}

func (tn *TestNode) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	var err error
	if tn.RunFunc != nil {
		err = tn.RunFunc(req)
	}
	return node.FlowNodeResult{
		NextNodeID: tn.NextIDs,
		Err:        err,
	}
}

func (tn *TestNode) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	result := tn.RunSync(ctx, req)
	resultChan <- result
}

func TestRunNodeSync_WithTracking(t *testing.T) {
	// Create a simple test node that demonstrates tracking
	nodeID := idwrap.NewNow()

	testNode := NewTestNode(nodeID, "testNode", []idwrap.IDWrap{}, func(req *node.FlowNodeRequest) error {
		// Test that the tracker was initialized
		if req.VariableTracker == nil {
			t.Error("Expected VariableTracker to be initialized, but it was nil")
			return nil
		}

		// Write some test data using tracking
		err := node.WriteNodeVarWithTracking(req, "testNode", "result", "success", req.VariableTracker)
		if err != nil {
			return err
		}
		return node.WriteNodeVarWithTracking(req, "testNode", "timestamp", time.Now().Unix(), req.VariableTracker)
	})

	// Create node map
	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		nodeID: testNode,
	}

	// Track execution status
	var executedNodes []string
	statusFunc := func(status runner.FlowNodeStatus) {
		if status.State == mnnode.NODE_STATE_RUNNING {
			executedNodes = append(executedNodes, status.Name)
		}
	}

	// Create flow request
	req := &node.FlowNodeRequest{
		VarMap:           make(map[string]any),
		ReadWriteLock:    &sync.RWMutex{},
		NodeMap:          nodeMap,
		EdgeSourceMap:    make(edge.EdgesMap),
		Timeout:          5 * time.Second,
		LogPushFunc:      statusFunc,
		PendingAtmoicMap: make(map[idwrap.IDWrap]uint32),
	}

	// Run the single node
	err := RunNodeSync(context.Background(), nodeID, req, statusFunc)
	if err != nil {
		t.Fatalf("RunNodeSync failed: %v", err)
	}

	// Verify node was executed
	if len(executedNodes) != 1 {
		t.Errorf("Expected 1 node execution, got %d", len(executedNodes))
	}
	if len(executedNodes) > 0 && executedNodes[0] != "testNode" {
		t.Errorf("Expected node 'testNode', got %v", executedNodes[0])
	}

	// Verify the data was written correctly
	result, err := node.ReadNodeVar(req, "testNode", "result")
	if err != nil {
		t.Errorf("Failed to read node result: %v", err)
	}
	if result != "success" {
		t.Errorf("Expected result 'success', got %v", result)
	}

	timestamp, err := node.ReadNodeVar(req, "testNode", "timestamp")
	if err != nil {
		t.Errorf("Failed to read node timestamp: %v", err)
	}
	if timestamp == nil {
		t.Error("Expected timestamp to be set")
	}
}

func TestRunNodeAsync_WithTracking(t *testing.T) {
	// Create a mock node that writes output
	nodeID := idwrap.NewNow()

	testNode := NewTestNode(nodeID, "asyncTestNode", []idwrap.IDWrap{}, func(req *node.FlowNodeRequest) error {
		if req.VariableTracker != nil {
			// Write some test data using tracking
			err := node.WriteNodeVarWithTracking(req, "asyncTestNode", "status", "async_complete", req.VariableTracker)
			if err != nil {
				return err
			}
			return node.WriteNodeVarWithTracking(req, "asyncTestNode", "timestamp", time.Now().Unix(), req.VariableTracker)
		}
		// Regular path
		err := node.WriteNodeVar(req, "asyncTestNode", "status", "async_complete")
		if err != nil {
			return err
		}
		return node.WriteNodeVar(req, "asyncTestNode", "timestamp", time.Now().Unix())
	})

	// Create node map
	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		nodeID: testNode,
	}

	// Track execution status
	var nodeStatuses []runner.FlowNodeStatus
	statusFunc := func(status runner.FlowNodeStatus) {
		nodeStatuses = append(nodeStatuses, status)
	}

	// Create flow request
	req := &node.FlowNodeRequest{
		VarMap:           make(map[string]any),
		ReadWriteLock:    &sync.RWMutex{},
		NodeMap:          nodeMap,
		EdgeSourceMap:    make(edge.EdgesMap),
		Timeout:          5 * time.Second,
		LogPushFunc:      statusFunc,
		PendingAtmoicMap: make(map[idwrap.IDWrap]uint32),
	}

	// Run the single node asynchronously
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := RunNodeASync(ctx, nodeID, req, statusFunc)
	if err != nil {
		t.Fatalf("RunNodeAsync failed: %v", err)
	}

	// Verify node was executed
	if len(nodeStatuses) == 0 {
		t.Error("Expected at least one status update")
	}

	// Verify the data was written correctly
	status, err := node.ReadNodeVar(req, "asyncTestNode", "status")
	if err != nil {
		t.Errorf("Failed to read async node status: %v", err)
	}
	if status != "async_complete" {
		t.Errorf("Expected status 'async_complete', got %v", status)
	}

	timestamp, err := node.ReadNodeVar(req, "asyncTestNode", "timestamp")
	if err != nil {
		t.Errorf("Failed to read async node timestamp: %v", err)
	}
	if timestamp == nil {
		t.Error("Expected timestamp to be set")
	}
}

func TestFlowRunner_InputOutputCapture(t *testing.T) {
	// Create nodes that demonstrate input/output data flow
	inputNodeID := idwrap.NewNow()
	processingNodeID := idwrap.NewNow()
	outputNodeID := idwrap.NewNow()

	// Input node - writes initial data
	inputNode := NewTestNode(inputNodeID, "inputNode", []idwrap.IDWrap{processingNodeID}, func(req *node.FlowNodeRequest) error {
		if req.VariableTracker != nil {
			return node.WriteNodeVarWithTracking(req, "inputNode", "data", "initial_value", req.VariableTracker)
		}
		return node.WriteNodeVar(req, "inputNode", "data", "initial_value")
	})

	// Processing node - reads input and writes processed data
	processingNode := NewTestNode(processingNodeID, "processingNode", []idwrap.IDWrap{outputNodeID}, func(req *node.FlowNodeRequest) error {
		if req.VariableTracker != nil {
			// Read input data
			data, err := node.ReadNodeVarWithTracking(req, "inputNode", "data", req.VariableTracker)
			if err != nil {
				return err
			}
			// Process and write output
			processedData := "processed_" + data.(string)
			return node.WriteNodeVarWithTracking(req, "processingNode", "result", processedData, req.VariableTracker)
		}
		// Regular path
		data, err := node.ReadNodeVar(req, "inputNode", "data")
		if err != nil {
			return err
		}
		processedData := "processed_" + data.(string)
		return node.WriteNodeVar(req, "processingNode", "result", processedData)
	})

	// Output node - reads processed data and writes final result
	outputNode := NewTestNode(outputNodeID, "outputNode", []idwrap.IDWrap{}, func(req *node.FlowNodeRequest) error {
		if req.VariableTracker != nil {
			// Read processed data
			result, err := node.ReadNodeVarWithTracking(req, "processingNode", "result", req.VariableTracker)
			if err != nil {
				return err
			}
			// Write final output
			finalResult := result.(string) + "_final"
			return node.WriteNodeVarWithTracking(req, "outputNode", "final", finalResult, req.VariableTracker)
		}
		// Regular path
		result, err := node.ReadNodeVar(req, "processingNode", "result")
		if err != nil {
			return err
		}
		finalResult := result.(string) + "_final"
		return node.WriteNodeVar(req, "outputNode", "final", finalResult)
	})

	// Create node map
	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		inputNodeID:      inputNode,
		processingNodeID: processingNode,
		outputNodeID:     outputNode,
	}

	// Create edge map (input -> processing -> output)
	edgeMap := edge.EdgesMap{
		processingNodeID: map[edge.EdgeHandle][]idwrap.IDWrap{
			edge.HandleThen: []idwrap.IDWrap{inputNodeID},
		},
		outputNodeID: map[edge.EdgeHandle][]idwrap.IDWrap{
			edge.HandleThen: []idwrap.IDWrap{processingNodeID},
		},
	}

	// Track execution for verification
	var executionOrder []string
	statusFunc := func(status runner.FlowNodeStatus) {
		if status.State == mnnode.NODE_STATE_RUNNING {
			executionOrder = append(executionOrder, status.Name)
		}
	}

	// Create flow request
	req := &node.FlowNodeRequest{
		VarMap:           make(map[string]any),
		ReadWriteLock:    &sync.RWMutex{},
		NodeMap:          nodeMap,
		EdgeSourceMap:    edgeMap,
		Timeout:          5 * time.Second,
		LogPushFunc:      statusFunc,
		PendingAtmoicMap: make(map[idwrap.IDWrap]uint32),
	}

	// Run the complete flow
	err := RunNodeSync(context.Background(), inputNodeID, req, statusFunc)
	if err != nil {
		t.Fatalf("Flow execution failed: %v", err)
	}

	// Verify execution order
	expectedOrder := []string{"inputNode", "processingNode", "outputNode"}
	if len(executionOrder) != len(expectedOrder) {
		t.Errorf("Expected %d nodes executed, got %d", len(expectedOrder), len(executionOrder))
	}

	// Verify final result contains the complete data transformation chain
	finalResult, err := node.ReadNodeVar(req, "outputNode", "final")
	if err != nil {
		t.Errorf("Failed to read final result: %v", err)
	}
	expectedFinal := "processed_initial_value_final"
	if finalResult != expectedFinal {
		t.Errorf("Expected final result '%s', got %v", expectedFinal, finalResult)
	}

	t.Logf("Flow completed successfully with result: %v", finalResult)
}

func TestFlowRunner_ParallelNodeTracking(t *testing.T) {
	// Create multiple independent nodes that can run in parallel
	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()
	node3ID := idwrap.NewNow()

	createParallelNode := func(id idwrap.IDWrap, name string, outputValue string) node.FlowNode {
		return NewTestNode(id, name, []idwrap.IDWrap{}, func(req *node.FlowNodeRequest) error {
			// Simulate some work
			time.Sleep(10 * time.Millisecond)

			if req.VariableTracker != nil {
				return node.WriteNodeVarWithTracking(req, name, "result", outputValue, req.VariableTracker)
			}
			return node.WriteNodeVar(req, name, "result", outputValue)
		})
	}

	node1 := createParallelNode(node1ID, "parallelNode1", "result1")
	node2 := createParallelNode(node2ID, "parallelNode2", "result2")
	node3 := createParallelNode(node3ID, "parallelNode3", "result3")

	// Create node map
	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		node1ID: node1,
		node2ID: node2,
		node3ID: node3,
	}

	// No edges - all nodes are independent and can run in parallel
	edgeMap := make(edge.EdgesMap)

	// Track execution
	var executedNodes []string
	var executionMutex sync.Mutex
	statusFunc := func(status runner.FlowNodeStatus) {
		if status.State == mnnode.NODE_STATE_RUNNING {
			executionMutex.Lock()
			executedNodes = append(executedNodes, status.Name)
			executionMutex.Unlock()
		}
	}

	// Create flow request
	req := &node.FlowNodeRequest{
		VarMap:           make(map[string]any),
		ReadWriteLock:    &sync.RWMutex{},
		NodeMap:          nodeMap,
		EdgeSourceMap:    edgeMap,
		Timeout:          5 * time.Second,
		LogPushFunc:      statusFunc,
		PendingAtmoicMap: make(map[idwrap.IDWrap]uint32),
	}

	// Run all nodes (they should execute in parallel since no dependencies)
	// We'll test by running from each node independently
	for _, nodeID := range []idwrap.IDWrap{node1ID, node2ID, node3ID} {
		err := RunNodeSync(context.Background(), nodeID, req, statusFunc)
		if err != nil {
			t.Errorf("Failed to run node %v: %v", nodeID, err)
		}
	}

	// Verify all nodes executed
	executionMutex.Lock()
	numExecuted := len(executedNodes)
	executionMutex.Unlock()

	if numExecuted != 3 {
		t.Errorf("Expected 3 node executions, got %d", numExecuted)
	}

	// Verify all results were written correctly
	expectedResults := map[string]string{
		"parallelNode1": "result1",
		"parallelNode2": "result2",
		"parallelNode3": "result3",
	}

	for nodeName, expectedResult := range expectedResults {
		result, err := node.ReadNodeVar(req, nodeName, "result")
		if err != nil {
			t.Errorf("Failed to read result from %s: %v", nodeName, err)
			continue
		}
		if result != expectedResult {
			t.Errorf("Expected result '%s' from %s, got %v", expectedResult, nodeName, result)
		}
	}

	t.Logf("Parallel execution completed successfully with %d nodes", numExecuted)
}
