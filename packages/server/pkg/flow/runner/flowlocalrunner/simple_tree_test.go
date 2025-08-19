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

func TestSimpleTreeStructure(t *testing.T) {
	nodeID := idwrap.NewNow()

	// Simple test node that just writes some data
	testNode := NewTestNode(nodeID, "simpleNode", []idwrap.IDWrap{}, func(req *node.FlowNodeRequest) error {
		if req.VariableTracker != nil {
			// Write some simple data using tracking
			err := node.WriteNodeVarWithTracking(req, "simpleNode", "result", "success", req.VariableTracker)
			if err != nil {
				return err
			}

			err = node.WriteNodeVarWithTracking(req, "simpleNode", "status", 200, req.VariableTracker)
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
		t.Logf("Status: %s, State: %v, InputData: %+v, OutputData: %+v",
			status.Name, status.State, status.InputData, status.OutputData)
	}

	req := &node.FlowNodeRequest{
		VarMap:           make(map[string]any),
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

	t.Logf("Final captured status:")
	t.Logf("  InputData: %+v", capturedStatus.InputData)
	t.Logf("  OutputData: %+v", capturedStatus.OutputData)
}

func TestReadAndWriteTreeStructure(t *testing.T) {
	nodeID := idwrap.NewNow()

	// Node that reads from initial data and writes output
	testNode := NewTestNode(nodeID, "readWriteNode", []idwrap.IDWrap{}, func(req *node.FlowNodeRequest) error {
		if req.VariableTracker != nil {
			// Read some initial data
			value, err := node.ReadNodeVarWithTracking(req, "input", "value", req.VariableTracker)
			if err != nil && err != node.ErrVarKeyNotFound && err != node.ErrVarNodeNotFound {
				return err
			}

			// Write some output based on the read value
			if value != nil {
				err = node.WriteNodeVarWithTracking(req, "readWriteNode", "processed", "processed_"+value.(string), req.VariableTracker)
				if err != nil {
					return err
				}
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
		t.Logf("Status: %s, State: %v, InputData: %+v, OutputData: %+v",
			status.Name, status.State, status.InputData, status.OutputData)
	}

	// Set up initial data
	initialData := map[string]any{
		"input": map[string]interface{}{
			"value": "test_data",
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

	t.Logf("Final captured status:")
	t.Logf("  InputData: %+v", capturedStatus.InputData)
	t.Logf("  OutputData: %+v", capturedStatus.OutputData)
}
