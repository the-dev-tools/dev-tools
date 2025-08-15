package flowlocalrunner_test

import (
	"context"
	"sync"
	"testing"

	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/flow/runner/flowlocalrunner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
)

// TestNode is a simple test implementation of FlowNode that can write/read variables
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

// TestComprehensiveInputDataTracking tests various input data scenarios
func TestComprehensiveInputDataTracking(t *testing.T) {
	var mu sync.Mutex
	nodeStatuses := make(map[idwrap.IDWrap][]runner.FlowNodeStatus)
	
	// Create flow: firstNode -> middleNode -> lastNode
	firstNodeID := idwrap.NewNow()
	middleNodeID := idwrap.NewNow()
	lastNodeID := idwrap.NewNow()
	
	// First node - has NO predecessors, reads NO variables
	// Should have EMPTY input data (this is correct behavior)
	firstNode := NewTestNode(firstNodeID, "first_node", []idwrap.IDWrap{middleNodeID}, func(req *node.FlowNodeRequest) error {
		// Write data for middle node to consume
		if err := node.WriteNodeVarRaw(req, "first_node", "first_output"); err != nil {
			return err
		}
		// Don't read any variables - should result in empty input
		return nil
	})
	
	// Middle node - HAS predecessors, reads variables from first node
	// Should have NON-EMPTY input data with predecessor data
	middleNode := NewTestNode(middleNodeID, "middle_node", []idwrap.IDWrap{lastNodeID}, func(req *node.FlowNodeRequest) error {
		// Read data from first node WITH TRACKING
		var data interface{}
		var err error
		if req.VariableTracker != nil {
			data, err = node.ReadVarRawWithTracking(req, "first_node", req.VariableTracker)
		} else {
			data, err = node.ReadVarRaw(req, "first_node")
		}
		
		if err != nil {
			t.Logf("Middle node failed to read from first_node: %v", err)
		} else {
			t.Logf("Middle node read from first_node: %v", data)
		}
		
		// Write data for last node
		if err := node.WriteNodeVarRaw(req, "middle_node", "middle_output"); err != nil {
			return err
		}
		return nil
	})
	
	// Last node - HAS predecessors, reads variables from both previous nodes
	// Should have NON-EMPTY input data with all predecessor data
	lastNode := NewTestNode(lastNodeID, "last_node", nil, func(req *node.FlowNodeRequest) error {
		// Read from both previous nodes WITH TRACKING
		var firstData, middleData interface{}
		var err1, err2 error
		
		if req.VariableTracker != nil {
			firstData, err1 = node.ReadVarRawWithTracking(req, "first_node", req.VariableTracker)
			middleData, err2 = node.ReadVarRawWithTracking(req, "middle_node", req.VariableTracker)
		} else {
			firstData, err1 = node.ReadVarRaw(req, "first_node")
			middleData, err2 = node.ReadVarRaw(req, "middle_node")
		}
		
		if err1 != nil {
			t.Logf("Last node failed to read from first_node: %v", err1)
		} else {
			t.Logf("Last node read from first_node: %v", firstData)
		}
		
		if err2 != nil {
			t.Logf("Last node failed to read from middle_node: %v", err2)
		} else {
			t.Logf("Last node read from middle_node: %v", middleData)
		}
		
		return nil
	})
	
	flowNodeMap := map[idwrap.IDWrap]node.FlowNode{
		firstNodeID:  firstNode,
		middleNodeID: middleNode,
		lastNodeID:   lastNode,
	}
	
	// Create edges: first -> middle -> last
	edge1 := edge.NewEdge(idwrap.NewNow(), firstNodeID, middleNodeID, edge.HandleUnspecified, edge.EdgeKindUnspecified)
	edge2 := edge.NewEdge(idwrap.NewNow(), middleNodeID, lastNodeID, edge.HandleUnspecified, edge.EdgeKindUnspecified)
	edges := []edge.Edge{edge1, edge2}
	edgesMap := edge.NewEdgesMap(edges)
	
	// Create flow runner
	runnerLocal := flowlocalrunner.CreateFlowRunner(
		idwrap.NewNow(), 
		idwrap.NewNow(), 
		firstNodeID, // Start with first node
		flowNodeMap, 
		edgesMap, 
		0, // No timeout for sync execution
	)
	
	// Channels for status tracking
	flowNodeStatusChan := make(chan runner.FlowNodeStatus, 100)
	flowStatusChan := make(chan runner.FlowStatus, 10)
	
	// Start flow execution
	ctx := context.Background()
	err := runnerLocal.Run(ctx, flowNodeStatusChan, flowStatusChan, nil)
	
	if err != nil {
		t.Fatalf("Flow execution failed: %v", err)
	}
	
	// Collect all node status updates (channel is already closed by runner)
	for status := range flowNodeStatusChan {
		mu.Lock()
		nodeStatuses[status.NodeID] = append(nodeStatuses[status.NodeID], status)
		mu.Unlock()
	}
	
	// Verify first node (should have empty input - this is CORRECT)
	t.Run("FirstNodeEmptyInput", func(t *testing.T) {
		verifyNodeInputData(t, nodeStatuses, firstNodeID, "first_node", true, "First node should have empty input (no predecessors, no variable reads)")
	})
	
	// Verify middle node (should have non-empty input from first node)
	t.Run("MiddleNodeHasInput", func(t *testing.T) {
		verifyNodeInputData(t, nodeStatuses, middleNodeID, "middle_node", false, "Middle node should have input from first_node")
	})
	
	// Verify last node (should have input from both previous nodes)
	t.Run("LastNodeHasInput", func(t *testing.T) {
		verifyNodeInputDataMultiple(t, nodeStatuses, lastNodeID, "last_node", []string{"first_node", "middle_node"}, "Last node should have input from both predecessors")
	})
}

func verifyNodeInputData(t *testing.T, nodeStatuses map[idwrap.IDWrap][]runner.FlowNodeStatus, nodeID idwrap.IDWrap, nodeName string, expectEmpty bool, description string) {
	statuses := nodeStatuses[nodeID]
	if len(statuses) == 0 {
		t.Fatalf("No status updates received for %s", nodeName)
	}
	
	// Find the final SUCCESS status
	var finalStatus *runner.FlowNodeStatus
	for i := range statuses {
		status := &statuses[i]
		if status.State == mnnode.NODE_STATE_SUCCESS {
			finalStatus = status
			break
		}
	}
	
	if finalStatus == nil {
		t.Fatalf("No SUCCESS status found for %s", nodeName)
	}
	
	// Check input data
	if finalStatus.InputData == nil {
		if !expectEmpty {
			t.Errorf("%s: InputData is nil but expected non-empty - %s", nodeName, description)
		} else {
			t.Logf("%s: InputData is nil as expected", nodeName)
		}
		return
	}
	
	inputMap, ok := finalStatus.InputData.(map[string]interface{})
	if !ok {
		t.Errorf("%s: InputData is not a map: %T", nodeName, finalStatus.InputData)
		return
	}
	
	isEmpty := len(inputMap) == 0
	
	if expectEmpty && !isEmpty {
		t.Errorf("%s: Expected empty InputData but got: %+v - %s", nodeName, inputMap, description)
	} else if !expectEmpty && isEmpty {
		t.Errorf("%s: Expected non-empty InputData but got empty map - %s", nodeName, description)
	} else {
		t.Logf("%s: InputData correct (%d items): %+v", nodeName, len(inputMap), inputMap)
	}
}

func verifyNodeInputDataMultiple(t *testing.T, nodeStatuses map[idwrap.IDWrap][]runner.FlowNodeStatus, nodeID idwrap.IDWrap, nodeName string, expectedKeys []string, description string) {
	statuses := nodeStatuses[nodeID]
	if len(statuses) == 0 {
		t.Fatalf("No status updates received for %s", nodeName)
	}
	
	// Find the final SUCCESS status
	var finalStatus *runner.FlowNodeStatus
	for i := range statuses {
		status := &statuses[i]
		if status.State == mnnode.NODE_STATE_SUCCESS {
			finalStatus = status
			break
		}
	}
	
	if finalStatus == nil {
		t.Fatalf("No SUCCESS status found for %s", nodeName)
	}
	
	// Check input data
	if finalStatus.InputData == nil {
		t.Errorf("%s: InputData is nil but expected non-empty - %s", nodeName, description)
		return
	}
	
	inputMap, ok := finalStatus.InputData.(map[string]interface{})
	if !ok {
		t.Errorf("%s: InputData is not a map: %T", nodeName, finalStatus.InputData)
		return
	}
	
	// Check for expected keys
	for _, expectedKey := range expectedKeys {
		if _, exists := inputMap[expectedKey]; !exists {
			t.Errorf("%s: Expected key '%s' not found in InputData - %s", nodeName, expectedKey, description)
		}
	}
	
	t.Logf("%s: InputData correct (%d items): %+v", nodeName, len(inputMap), inputMap)
}

