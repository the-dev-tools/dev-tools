package flowlocalrunner

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
	"time"
)

func TestTreeStructuredInputOutput(t *testing.T) {
	nodeID := idwrap.NewNow()
	
	// Create a test node that reads from one source and writes to another
	testNode := NewTestNode(nodeID, "treeTestNode", []idwrap.IDWrap{}, func(req *node.FlowNodeRequest) error {
		if req.VariableTracker != nil {
			// Read the entire response data from request_0
			responseData, err := node.ReadNodeVarWithTracking(req, "request_0", "response", req.VariableTracker)
			if err != nil && err != node.ErrVarKeyNotFound && err != node.ErrVarNodeNotFound {
				return err
			}
			
			// If we got the response data, try to access the nested fields
			if responseData != nil {
				if responseMap, ok := responseData.(map[string]interface{}); ok {
					if body, exists := responseMap["body"]; exists {
						if bodyMap, ok := body.(map[string]interface{}); ok {
							if token, exists := bodyMap["token"]; exists {
								// Track reading the token specifically
								_ = node.WriteNodeVarWithTracking(req, "_temp", "read_token", token, req.VariableTracker)
							}
						}
					}
				}
			}
			
			// Write some response data
			err = node.WriteNodeVarWithTracking(req, "treeTestNode", "request.method", "POST", req.VariableTracker)
			if err != nil {
				return err
			}
			
			err = node.WriteNodeVarWithTracking(req, "treeTestNode", "request.body", `{"test": "data"}`, req.VariableTracker)
			if err != nil {
				return err
			}
			
			err = node.WriteNodeVarWithTracking(req, "treeTestNode", "response.status", 201, req.VariableTracker)
			if err != nil {
				return err
			}
		}
		return nil
	})

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		nodeID: testNode,
	}

	var capturedStatus *runner.FlowNodeStatus
	statusFunc := func(status runner.FlowNodeStatus) {
		if status.State == mnnode.NODE_STATE_SUCCESS {
			capturedStatus = &status
		}
	}

	// Set up initial data that the node can read from
	initialData := map[string]any{
		"request_0": map[string]interface{}{
			"response": map[string]interface{}{
				"body": map[string]interface{}{
					"token": "abc123",
				},
				"status": 200,
			},
		},
	}

	req := &node.FlowNodeRequest{
		VarMap:           initialData,
		ReadWriteLock:    &sync.RWMutex{},
		NodeMap:          nodeMap,
		EdgeSourceMap:    make(edge.EdgesMap),
		Timeout:          5 * time.Second,
		LogPushFunc:      statusFunc,
		PendingAtmoicMap: make(map[idwrap.IDWrap]uint32),
	}

	err := RunNodeSync(context.Background(), nodeID, req, statusFunc)
	if err != nil {
		t.Fatalf("RunNodeSync failed: %v", err)
	}

	if capturedStatus == nil {
		t.Fatal("Expected to capture a success status")
	}

	// Verify Input Data has proper tree structure (what was read)
	expectedInputData := map[string]any{
		"request_0": map[string]any{
			"response": map[string]any{
				"body": map[string]any{
					"token": "abc123",
				},
				"status": 200,
			},
		},
	}

	if !reflect.DeepEqual(capturedStatus.InputData, expectedInputData) {
		t.Errorf("Input data tree structure incorrect.\nExpected: %+v\nGot: %+v", 
			expectedInputData, capturedStatus.InputData)
	}

	// Verify Output Data has proper tree structure (what was written)
	expectedOutputData := map[string]any{
		"_temp": map[string]any{
			"read_token": "abc123",
		},
		"treeTestNode": map[string]any{
			"request": map[string]any{
				"method": "POST",
				"body":   `{"test": "data"}`,
			},
			"response": map[string]any{
				"status": 201,
			},
		},
	}

	if !reflect.DeepEqual(capturedStatus.OutputData, expectedOutputData) {
		t.Errorf("Output data tree structure incorrect.\nExpected: %+v\nGot: %+v", 
			expectedOutputData, capturedStatus.OutputData)
	}

	t.Logf("✅ Tree structure verified correctly:")
	t.Logf("Input: %+v", capturedStatus.InputData)
	t.Logf("Output: %+v", capturedStatus.OutputData)
}

func TestTreeStructureWithoutVariablesWrapper(t *testing.T) {
	// This test ensures we no longer have the confusing "variables" wrapper
	nodeID := idwrap.NewNow()
	
	testNode := NewTestNode(nodeID, "noWrapperNode", []idwrap.IDWrap{}, func(req *node.FlowNodeRequest) error {
		if req.VariableTracker != nil {
			// Read a simple value
			_, err := node.ReadNodeVarWithTracking(req, "input", "data", req.VariableTracker)
			if err != nil && err != node.ErrVarKeyNotFound && err != node.ErrVarNodeNotFound {
				return err
			}
		}
		return nil
	})

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		nodeID: testNode,
	}

	var capturedStatus *runner.FlowNodeStatus
	statusFunc := func(status runner.FlowNodeStatus) {
		if status.State == mnnode.NODE_STATE_SUCCESS {
			capturedStatus = &status
		}
	}

	initialData := map[string]any{
		"input": map[string]interface{}{
			"data": "test_value",
		},
	}

	req := &node.FlowNodeRequest{
		VarMap:           initialData,
		ReadWriteLock:    &sync.RWMutex{},
		NodeMap:          nodeMap,
		EdgeSourceMap:    make(edge.EdgesMap),
		Timeout:          5 * time.Second,
		LogPushFunc:      statusFunc,
		PendingAtmoicMap: make(map[idwrap.IDWrap]uint32),
	}

	err := RunNodeSync(context.Background(), nodeID, req, statusFunc)
	if err != nil {
		t.Fatalf("RunNodeSync failed: %v", err)
	}

	if capturedStatus == nil {
		t.Fatal("Expected to capture a success status")
	}

	// Verify no "variables" wrapper exists
	if inputData, ok := capturedStatus.InputData.(map[string]any); ok {
		if _, hasVariables := inputData["variables"]; hasVariables {
			t.Error("Found unwanted 'variables' wrapper in input data")
		}
		
		// Should have direct access to the data structure
		if input, hasInput := inputData["input"]; hasInput {
			if inputMap, ok := input.(map[string]any); ok {
				if data, hasData := inputMap["data"]; hasData {
					if data != "test_value" {
						t.Errorf("Expected data 'test_value', got %v", data)
					}
				} else {
					t.Error("Expected 'data' field in input structure")
				}
			} else {
				t.Error("Expected input to be a map")
			}
		} else {
			t.Error("Expected 'input' field in input data")
		}
	} else {
		t.Error("Expected InputData to be a map")
	}

	t.Logf("✅ No variables wrapper found, clean tree structure: %+v", capturedStatus.InputData)
}