package flowlocalrunner_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/flow/runner/flowlocalrunner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
	"time"
)

// DataTrackingNode is a node that writes output data and can verify input data
type DataTrackingNode struct {
	ID         idwrap.IDWrap
	Name       string
	Next       []idwrap.IDWrap
	OutputData map[string]interface{}

	// Verification function called during Run
	VerifyInput func(inputData map[string]interface{}) error

	// Transform function called to generate output based on input
	TransformData func(inputData map[string]interface{}) map[string]interface{}

	// Track what was actually received
	ReceivedInput map[string]interface{}
	mu            sync.Mutex
}

func NewDataTrackingNode(id idwrap.IDWrap, name string, next []idwrap.IDWrap, outputData map[string]interface{}) *DataTrackingNode {
	return &DataTrackingNode{
		ID:         id,
		Name:       name,
		Next:       next,
		OutputData: outputData,
	}
}

func (n *DataTrackingNode) GetID() idwrap.IDWrap {
	return n.ID
}

func (n *DataTrackingNode) GetName() string {
	return n.Name
}

func (n *DataTrackingNode) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	// Capture what input this node sees
	n.mu.Lock()
	n.ReceivedInput = make(map[string]interface{})

	// Check all predecessor nodes
	if req.EdgeSourceMap != nil {
		predecessors := getPredecessorNodes(n.ID, req.EdgeSourceMap)
		for _, predID := range predecessors {
			if predNode, ok := req.NodeMap[predID]; ok {
				predName := predNode.GetName()
				if predData, err := node.ReadVarRaw(req, predName); err == nil {
					n.ReceivedInput[predName] = predData
				}
			}
		}
	}
	n.mu.Unlock()

	// Run verification if provided
	if n.VerifyInput != nil {
		if err := n.VerifyInput(n.ReceivedInput); err != nil {
			return node.FlowNodeResult{
				NextNodeID: nil,
				Err:        err,
			}
		}
	}

	// Transform data if transform function is provided
	outputToWrite := n.OutputData
	if n.TransformData != nil {
		outputToWrite = n.TransformData(n.ReceivedInput)
	}

	// Write output data
	if outputToWrite != nil {
		if err := node.WriteNodeVarBulk(req, n.Name, outputToWrite); err != nil {
			return node.FlowNodeResult{
				NextNodeID: nil,
				Err:        err,
			}
		}
	}

	return node.FlowNodeResult{
		NextNodeID: n.Next,
		Err:        nil,
	}
}

func (n *DataTrackingNode) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	result := n.RunSync(ctx, req)
	resultChan <- result
}

// Helper function (copy from main code since it's not exported)
func getPredecessorNodes(nodeID idwrap.IDWrap, edgesMap edge.EdgesMap) []idwrap.IDWrap {
	var predecessors []idwrap.IDWrap
	seen := make(map[idwrap.IDWrap]bool)

	for sourceID, edges := range edgesMap {
		for _, targetNodes := range edges {
			for _, targetID := range targetNodes {
				if targetID == nodeID && !seen[sourceID] {
					predecessors = append(predecessors, sourceID)
					seen[sourceID] = true
				}
			}
		}
	}

	return predecessors
}

func TestInputOutputTracking_SimpleLinearFlow(t *testing.T) {
	// Create a simple flow: Node A -> Node B
	// Node A writes output data, Node B should see it as input

	nodeAID := idwrap.NewNow()
	nodeBID := idwrap.NewNow()

	nodeAData := map[string]interface{}{
		"message": "Hello from Node A",
		"value":   42,
		"array":   []interface{}{"a", "b", "c"},
	}

	nodeA := NewDataTrackingNode(nodeAID, "NodeA", []idwrap.IDWrap{nodeBID}, nodeAData)
	nodeB := NewDataTrackingNode(nodeBID, "NodeB", nil, nil)

	// Set up verification for Node B
	nodeB.VerifyInput = func(inputData map[string]interface{}) error {
		// Verify we received data from Node A
		nodeAInput, ok := inputData["NodeA"]
		if !ok {
			return fmt.Errorf("NodeB did not receive input from NodeA")
		}

		// Verify the data structure
		nodeAMap, ok := nodeAInput.(map[string]interface{})
		if !ok {
			return fmt.Errorf("NodeA input is not a map, got %T", nodeAInput)
		}

		// Verify specific values
		if msg, ok := nodeAMap["message"]; !ok || msg != "Hello from Node A" {
			return fmt.Errorf("Expected message 'Hello from Node A', got %v", msg)
		}

		if val, ok := nodeAMap["value"]; !ok || val != 42 {
			return fmt.Errorf("Expected value 42, got %v", val)
		}

		return nil
	}

	flowNodeMap := map[idwrap.IDWrap]node.FlowNode{
		nodeAID: nodeA,
		nodeBID: nodeB,
	}

	edge1 := edge.NewEdge(idwrap.NewNow(), nodeAID, nodeBID, edge.HandleUnspecified, edge.EdgeKindUnspecified)
	edges := []edge.Edge{edge1}
	edgesMap := edge.NewEdgesMap(edges)

	// Test both sync and async
	testCases := []struct {
		name    string
		timeout time.Duration
	}{
		{"Sync", 0},
		{"Async", time.Second},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Track status updates
			var statuses []runner.FlowNodeStatus
			var statusMu sync.Mutex

			runnerLocal := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), nodeAID, flowNodeMap, edgesMap, tc.timeout)
			flowNodeStatusChan := make(chan runner.FlowNodeStatus, 100)
			flowStatusChan := make(chan runner.FlowStatus, 10)

			// Collect statuses in background
			go func() {
				for status := range flowNodeStatusChan {
					statusMu.Lock()
					statuses = append(statuses, status)
					statusMu.Unlock()
				}
			}()

			err := runnerLocal.Run(context.Background(), flowNodeStatusChan, flowStatusChan, nil)
			if err != nil {
				t.Fatalf("Flow execution failed: %v", err)
			}

			// Verify Node B received correct input
			if nodeB.ReceivedInput == nil {
				t.Errorf("NodeB ReceivedInput is nil")
			} else if _, ok := nodeB.ReceivedInput["NodeA"]; !ok {
				t.Errorf("NodeB did not receive input from NodeA. ReceivedInput: %+v", nodeB.ReceivedInput)
			}

			// Verify status updates show correct input/output
			statusMu.Lock()
			defer statusMu.Unlock()

			for _, status := range statuses {
				if status.State == mnnode.NODE_STATE_SUCCESS {
					t.Logf("%s status - Input: %v, Output: %v", status.Name, status.InputData, status.OutputData)

					if status.Name == "NodeA" {
						// NodeA should have no input (it's the first node)
						if status.InputData != nil && len(status.InputData.(map[string]interface{})) > 0 {
							t.Errorf("NodeA should have empty input, got: %v", status.InputData)
						}
						// NodeA should have output
						if status.OutputData == nil {
							t.Errorf("NodeA should have output data")
						}
					} else if status.Name == "NodeB" {
						// NodeB should have input from NodeA
						if status.InputData == nil {
							t.Errorf("NodeB should have input data")
						} else {
							inputMap, ok := status.InputData.(map[string]interface{})
							if !ok {
								t.Errorf("NodeB input data is not a map")
							} else if _, hasNodeA := inputMap["NodeA"]; !hasNodeA {
								t.Errorf("NodeB input should contain NodeA data")
							}
						}
					}
				}
			}
		})
	}
}

func TestInputOutputTracking_ComplexFlow(t *testing.T) {
	// Create a more complex flow:
	//    NodeA
	//   /     \
	// NodeB   NodeC
	//   \     /
	//    NodeD

	nodeAID := idwrap.NewNow()
	nodeBID := idwrap.NewNow()
	nodeCID := idwrap.NewNow()
	nodeDID := idwrap.NewNow()

	nodeA := NewDataTrackingNode(nodeAID, "NodeA", []idwrap.IDWrap{nodeBID, nodeCID}, map[string]interface{}{
		"initial": "data from A",
		"number":  100,
	})

	nodeB := NewDataTrackingNode(nodeBID, "NodeB", []idwrap.IDWrap{nodeDID}, map[string]interface{}{
		"processed":  "data from B",
		"multiplied": 200, // 100 * 2
	})

	nodeC := NewDataTrackingNode(nodeCID, "NodeC", []idwrap.IDWrap{nodeDID}, map[string]interface{}{
		"processed": "data from C",
		"divided":   50, // 100 / 2
	})

	nodeD := NewDataTrackingNode(nodeDID, "NodeD", nil, nil)

	// NodeD should receive input from both B and C
	nodeD.VerifyInput = func(inputData map[string]interface{}) error {
		// Should have data from both NodeB and NodeC
		nodeBInput, hasB := inputData["NodeB"]
		nodeCInput, hasC := inputData["NodeC"]

		if !hasB || !hasC {
			return fmt.Errorf("NodeD should have input from both NodeB and NodeC, got: %+v", inputData)
		}

		// Verify NodeB data
		nodeBMap, ok := nodeBInput.(map[string]interface{})
		if !ok {
			return fmt.Errorf("NodeB input is not a map")
		}
		if nodeBMap["multiplied"] != 200 {
			return fmt.Errorf("Expected NodeB multiplied=200, got %v", nodeBMap["multiplied"])
		}

		// Verify NodeC data
		nodeCMap, ok := nodeCInput.(map[string]interface{})
		if !ok {
			return fmt.Errorf("NodeC input is not a map")
		}
		if nodeCMap["divided"] != 50 {
			return fmt.Errorf("Expected NodeC divided=50, got %v", nodeCMap["divided"])
		}

		return nil
	}

	flowNodeMap := map[idwrap.IDWrap]node.FlowNode{
		nodeAID: nodeA,
		nodeBID: nodeB,
		nodeCID: nodeC,
		nodeDID: nodeD,
	}

	edges := []edge.Edge{
		edge.NewEdge(idwrap.NewNow(), nodeAID, nodeBID, edge.HandleUnspecified, edge.EdgeKindUnspecified),
		edge.NewEdge(idwrap.NewNow(), nodeAID, nodeCID, edge.HandleUnspecified, edge.EdgeKindUnspecified),
		edge.NewEdge(idwrap.NewNow(), nodeBID, nodeDID, edge.HandleUnspecified, edge.EdgeKindUnspecified),
		edge.NewEdge(idwrap.NewNow(), nodeCID, nodeDID, edge.HandleUnspecified, edge.EdgeKindUnspecified),
	}
	edgesMap := edge.NewEdgesMap(edges)

	// Run the flow
	runnerLocal := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), nodeAID, flowNodeMap, edgesMap, 0)
	flowNodeStatusChan := make(chan runner.FlowNodeStatus, 100)
	flowStatusChan := make(chan runner.FlowStatus, 10)

	var allStatuses []runner.FlowNodeStatus
	var statusWg sync.WaitGroup
	statusWg.Add(1)
	go func() {
		defer statusWg.Done()
		for status := range flowNodeStatusChan {
			allStatuses = append(allStatuses, status)
			if status.State == mnnode.NODE_STATE_SUCCESS {
				t.Logf("%s completed - Input: %s", status.Name, prettyJSON(status.InputData))
			}
		}
	}()

	err := runnerLocal.Run(context.Background(), flowNodeStatusChan, flowStatusChan, nil)
	if err != nil {
		t.Fatalf("Flow execution failed: %v", err)
	}

	// Wait for status collection to complete
	statusWg.Wait()

	// Additional verification
	if nodeD.ReceivedInput == nil {
		t.Errorf("NodeD did not receive any input")
	} else {
		t.Logf("NodeD received input: %s", prettyJSON(nodeD.ReceivedInput))
	}
}

func TestControlFlowNodes_InputOutputTracking(t *testing.T) {
	// Test various control flow scenarios
	// This simulates IF, FOR, FOR_EACH nodes that write their state

	nodeStartID := idwrap.NewNow()
	nodeIfID := idwrap.NewNow()
	nodeForID := idwrap.NewNow()
	nodeForEachID := idwrap.NewNow()
	nodeEndID := idwrap.NewNow()

	// Start node
	nodeStart := NewDataTrackingNode(nodeStartID, "Start", []idwrap.IDWrap{nodeIfID}, map[string]interface{}{
		"items": []interface{}{"apple", "banana", "cherry"},
		"count": 3,
	})

	// IF node (simulated)
	nodeIf := NewDataTrackingNode(nodeIfID, "IfNode", []idwrap.IDWrap{nodeForID}, map[string]interface{}{
		"condition": true,
		"branch":    "true_branch",
	})

	// FOR node (simulated)
	nodeFor := NewDataTrackingNode(nodeForID, "ForNode", []idwrap.IDWrap{nodeForEachID}, map[string]interface{}{
		"iterations": 5,
		"completed":  true,
	})

	// FOR_EACH node (simulated)
	nodeForEach := NewDataTrackingNode(nodeForEachID, "ForEachNode", []idwrap.IDWrap{nodeEndID}, map[string]interface{}{
		"itemCount":      3,
		"processedItems": []interface{}{"apple", "banana", "cherry"},
	})

	// End node - verify it sees only its direct predecessor (ForEachNode)
	nodeEnd := NewDataTrackingNode(nodeEndID, "End", nil, nil)
	nodeEnd.VerifyInput = func(inputData map[string]interface{}) error {
		// Should have data only from ForEachNode (direct predecessor)
		if _, ok := inputData["ForEachNode"]; !ok {
			return fmt.Errorf("End node missing input from ForEachNode")
		}

		// Should NOT have data from other nodes (they're not direct predecessors)
		unexpectedNodes := []string{"IfNode", "ForNode", "Start"}
		for _, nodeName := range unexpectedNodes {
			if _, ok := inputData[nodeName]; ok {
				return fmt.Errorf("End node should not have input from %s (not a direct predecessor)", nodeName)
			}
		}

		// Verify FOR_EACH node data
		if forEachData, ok := inputData["ForEachNode"].(map[string]interface{}); ok {
			if forEachData["itemCount"] != 3 {
				return fmt.Errorf("Expected ForEachNode itemCount=3")
			}
		}

		return nil
	}

	flowNodeMap := map[idwrap.IDWrap]node.FlowNode{
		nodeStartID:   nodeStart,
		nodeIfID:      nodeIf,
		nodeForID:     nodeFor,
		nodeForEachID: nodeForEach,
		nodeEndID:     nodeEnd,
	}

	edges := []edge.Edge{
		edge.NewEdge(idwrap.NewNow(), nodeStartID, nodeIfID, edge.HandleUnspecified, edge.EdgeKindUnspecified),
		edge.NewEdge(idwrap.NewNow(), nodeIfID, nodeForID, edge.HandleUnspecified, edge.EdgeKindUnspecified),
		edge.NewEdge(idwrap.NewNow(), nodeForID, nodeForEachID, edge.HandleUnspecified, edge.EdgeKindUnspecified),
		edge.NewEdge(idwrap.NewNow(), nodeForEachID, nodeEndID, edge.HandleUnspecified, edge.EdgeKindUnspecified),
	}
	edgesMap := edge.NewEdgesMap(edges)

	// Test both sync and async
	for _, isAsync := range []bool{false, true} {
		testName := "Sync"
		timeout := time.Duration(0)
		if isAsync {
			testName = "Async"
			timeout = time.Second
		}

		t.Run(testName, func(t *testing.T) {
			runnerLocal := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), nodeStartID, flowNodeMap, edgesMap, timeout)
			flowNodeStatusChan := make(chan runner.FlowNodeStatus, 100)
			flowStatusChan := make(chan runner.FlowStatus, 10)

			var statusWg sync.WaitGroup
			statusWg.Add(1)
			go func() {
				defer statusWg.Done()
				for status := range flowNodeStatusChan {
					if status.State == mnnode.NODE_STATE_SUCCESS && status.InputData != nil {
						t.Logf("%s - Input nodes: %v", status.Name, getInputNodeNames(status.InputData))
					}
				}
			}()

			err := runnerLocal.Run(context.Background(), flowNodeStatusChan, flowStatusChan, nil)
			if err != nil {
				t.Fatalf("Flow execution failed: %v", err)
			}

			// Wait for status collection to complete
			statusWg.Wait()

			// Verify the end node received input from its direct predecessor
			if nodeEnd.ReceivedInput == nil || len(nodeEnd.ReceivedInput) != 1 {
				t.Errorf("End node should have received input from exactly 1 node (ForEachNode). Got: %+v", nodeEnd.ReceivedInput)
			}
		})
	}
}

func TestIntegrationScenario_RequestJavaScriptTransform(t *testing.T) {
	// Simulate: REQUEST node -> JavaScript transform -> Another node
	// This tests a realistic scenario

	requestID := idwrap.NewNow()
	jsTransformID := idwrap.NewNow()
	finalNodeID := idwrap.NewNow()

	// REQUEST node output (simulated API response)
	requestNode := NewDataTrackingNode(requestID, "RequestNode", []idwrap.IDWrap{jsTransformID}, map[string]interface{}{
		"status": 200,
		"body": map[string]interface{}{
			"users": []interface{}{
				map[string]interface{}{"id": 1, "name": "Alice", "age": 30},
				map[string]interface{}{"id": 2, "name": "Bob", "age": 25},
				map[string]interface{}{"id": 3, "name": "Charlie", "age": 35},
			},
		},
		"headers": map[string]interface{}{
			"content-type": "application/json",
		},
	})

	// JavaScript transform node - creates transformed output based on input
	jsNode := NewDataTrackingNode(jsTransformID, "JSTransform", []idwrap.IDWrap{finalNodeID}, nil)

	// Transform function that filters users
	jsNode.TransformData = func(inputData map[string]interface{}) map[string]interface{} {
		// Get request data
		reqData, ok := inputData["RequestNode"]
		if !ok {
			return nil
		}

		// Simulate transformation: extract users over 30
		if reqMap, ok := reqData.(map[string]interface{}); ok {
			if body, ok := reqMap["body"].(map[string]interface{}); ok {
				if users, ok := body["users"].([]interface{}); ok {
					var filtered []interface{}
					for _, user := range users {
						if u, ok := user.(map[string]interface{}); ok {
							if age, ok := u["age"].(int); ok && age >= 30 {
								filtered = append(filtered, u)
							}
						}
					}
					// Return transformed data
					return map[string]interface{}{
						"filteredUsers": filtered,
						"count":         len(filtered),
					}
				}
			}
		}
		return nil
	}

	// Final node verifies the transformation
	finalNode := NewDataTrackingNode(finalNodeID, "FinalNode", nil, nil)
	finalNode.VerifyInput = func(inputData map[string]interface{}) error {
		// Should have transformed data from JS node (direct predecessor)
		jsData, ok := inputData["JSTransform"]
		if !ok {
			return fmt.Errorf("FinalNode missing JSTransform input")
		}

		// Should NOT have data from RequestNode (not a direct predecessor)
		if _, ok := inputData["RequestNode"]; ok {
			return fmt.Errorf("FinalNode should not have input from RequestNode (not a direct predecessor)")
		}

		jsMap, ok := jsData.(map[string]interface{})
		if !ok {
			return fmt.Errorf("JSTransform data is not a map")
		}

		// Verify the transformation worked
		if count, ok := jsMap["count"].(int); !ok || count != 2 {
			return fmt.Errorf("Expected 2 filtered users, got %v", count)
		}

		return nil
	}

	flowNodeMap := map[idwrap.IDWrap]node.FlowNode{
		requestID:     requestNode,
		jsTransformID: jsNode,
		finalNodeID:   finalNode,
	}

	edges := []edge.Edge{
		edge.NewEdge(idwrap.NewNow(), requestID, jsTransformID, edge.HandleUnspecified, edge.EdgeKindUnspecified),
		edge.NewEdge(idwrap.NewNow(), jsTransformID, finalNodeID, edge.HandleUnspecified, edge.EdgeKindUnspecified),
	}
	edgesMap := edge.NewEdgesMap(edges)

	// Run the flow
	runnerLocal := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), requestID, flowNodeMap, edgesMap, 0)
	flowNodeStatusChan := make(chan runner.FlowNodeStatus, 100)
	flowStatusChan := make(chan runner.FlowStatus, 10)

	var successCount int
	var successMu sync.Mutex
	var statusWg sync.WaitGroup
	statusWg.Add(1)
	go func() {
		defer statusWg.Done()
		for status := range flowNodeStatusChan {
			if status.State == mnnode.NODE_STATE_SUCCESS {
				successMu.Lock()
				successCount++
				successMu.Unlock()
				t.Logf("%s completed successfully", status.Name)
				if status.InputData != nil {
					t.Logf("  Input: %s", prettyJSON(status.InputData))
				}
				if status.OutputData != nil {
					t.Logf("  Output: %s", prettyJSON(status.OutputData))
				}
			}
		}
	}()

	err := runnerLocal.Run(context.Background(), flowNodeStatusChan, flowStatusChan, nil)
	if err != nil {
		t.Fatalf("Flow execution failed: %v", err)
	}

	// Wait for status collection to complete
	statusWg.Wait()

	successMu.Lock()
	finalSuccessCount := successCount
	successMu.Unlock()

	if finalSuccessCount != 3 {
		t.Errorf("Expected 3 successful nodes, got %d", finalSuccessCount)
	}
}

// Helper functions
func prettyJSON(data interface{}) string {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Sprintf("%+v", data)
	}
	return string(b)
}

func getInputNodeNames(inputData interface{}) []string {
	var names []string
	if m, ok := inputData.(map[string]interface{}); ok {
		for name := range m {
			names = append(names, name)
		}
	}
	return names
}
