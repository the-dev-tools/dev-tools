package rflow

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"sync"
	"testing"
	"time"

	"the-dev-tools/server/pkg/flow/node/nrequest"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/http/response"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mexampleresp"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnodeexecution"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// NodeServiceInterface defines the interface for getting node information
type NodeServiceInterface interface {
	GetNode(ctx context.Context, id idwrap.IDWrap) (*mnnode.MNode, error)
}

// MockNodeService implements NodeServiceInterface for testing
type MockNodeService struct {
	nodes map[idwrap.IDWrap]*mnnode.MNode
}

func (m *MockNodeService) GetNode(ctx context.Context, id idwrap.IDWrap) (*mnnode.MNode, error) {
	node, exists := m.nodes[id]
	if !exists {
		return nil, errors.New("node not found")
	}
	return node, nil
}

// Helper function to process node completion (extracted from main logic)
func processNodeCompletion(
	ctx context.Context,
	flowNodeStatus runner.FlowNodeStatus,
	pendingNodeExecutions map[idwrap.IDWrap]*mnodeexecution.NodeExecution,
	pendingMutex *sync.Mutex,
	nodeExecutionChan chan mnodeexecution.NodeExecution,
	nodeService NodeServiceInterface,
) error {
	executionID := flowNodeStatus.ExecutionID

	// Get node type for REQUEST node detection
	node, err := nodeService.GetNode(ctx, flowNodeStatus.NodeID)
	if err != nil {
		// Log error but continue - we'll treat as non-REQUEST node
		log.Printf("Could not get node type for %s: %v", flowNodeStatus.NodeID.String(), err)
	}

	// Handle completion based on state
	switch flowNodeStatus.State {
	case mnnode.NODE_STATE_SUCCESS, mnnode.NODE_STATE_FAILURE, mnnode.NODE_STATE_CANCELED:
		// Update existing NodeExecution with final state
		pendingMutex.Lock()
		if nodeExec, exists := pendingNodeExecutions[executionID]; exists {
			// Update final state
			nodeExec.State = flowNodeStatus.State
			completedAt := time.Now().UnixMilli()
			nodeExec.CompletedAt = &completedAt

			// Set error if present
			if flowNodeStatus.Error != nil {
				errorStr := flowNodeStatus.Error.Error()
				nodeExec.Error = &errorStr
			}

			// Compress and store input data
			if flowNodeStatus.InputData != nil {
				if inputJSON, err := json.Marshal(flowNodeStatus.InputData); err == nil {
					if err := nodeExec.SetInputJSON(inputJSON); err != nil {
						nodeExec.InputData = inputJSON
						nodeExec.InputDataCompressType = 0
					}
				}
			}

			// Compress and store output data
			if flowNodeStatus.OutputData != nil {
				if outputJSON, err := json.Marshal(flowNodeStatus.OutputData); err == nil {
					if err := nodeExec.SetOutputJSON(outputJSON); err != nil {
						nodeExec.OutputData = outputJSON
						nodeExec.OutputDataCompressType = 0
					}
				}
			}

			// For REQUEST nodes, wait for response before sending to channel
			if node != nil && node.NodeKind == mnnode.NODE_KIND_REQUEST {
				// Mark as completed but keep in pending map for response handling
				// Don't send to channel yet - wait for response
			} else {
				// For non-REQUEST nodes, send immediately
				nodeExecutionChan <- *nodeExec
				delete(pendingNodeExecutions, executionID)
			}
		}
		pendingMutex.Unlock()
	}

	return nil
}

// Helper function to process request node responses (extracted from main logic)
func processRequestResponse(
	response nrequest.NodeRequestSideResp,
	pendingNodeExecutions map[idwrap.IDWrap]*mnodeexecution.NodeExecution,
	nodeToExampleMap map[idwrap.IDWrap]idwrap.IDWrap, // maps nodeID to exampleID  
	pendingMutex *sync.Mutex,
	nodeExecutionChan chan mnodeexecution.NodeExecution,
) {
	pendingMutex.Lock()
	defer pendingMutex.Unlock()

	var targetExecutionID idwrap.IDWrap
	var found bool

	// Find the node that corresponds to this example
	var targetNodeID idwrap.IDWrap
	for nodeID, exampleID := range nodeToExampleMap {
		if exampleID == response.Example.ID {
			targetNodeID = nodeID
			break
		}
	}

	if targetNodeID == (idwrap.IDWrap{}) {
		// No matching node found for this example
		return
	}

	// Look for completed REQUEST node execution waiting for response
	for execID, nodeExec := range pendingNodeExecutions {
		if nodeExec.NodeID == targetNodeID &&
			(nodeExec.State == mnnode.NODE_STATE_SUCCESS ||
				nodeExec.State == mnnode.NODE_STATE_FAILURE) {
			targetExecutionID = execID
			found = true
			break
		}
	}

	if found && response.Resp.ExampleResp.ID != (idwrap.IDWrap{}) {
		if nodeExec, exists := pendingNodeExecutions[targetExecutionID]; exists {
			respID := response.Resp.ExampleResp.ID
			nodeExec.ResponseID = &respID

			// Now send the completed execution with ResponseID to channel
			nodeExecutionChan <- *nodeExec
			delete(pendingNodeExecutions, targetExecutionID)
		}
	}
}

func TestRequestNodeResponseID_Success(t *testing.T) {
	// Setup
	nodeID := idwrap.NewNow()
	executionID := idwrap.NewNow()
	responseID := idwrap.NewNow()

	// Create channels
	nodeExecutionChan := make(chan mnodeexecution.NodeExecution, 10)

	// Mock node service to return REQUEST node
	mockNodeService := &MockNodeService{
		nodes: map[idwrap.IDWrap]*mnnode.MNode{
			nodeID: {
				ID:       nodeID,
				NodeKind: mnnode.NODE_KIND_REQUEST,
				Name:     "Test Request Node",
			},
		},
	}

	// Create pending execution map
	pendingNodeExecutions := make(map[idwrap.IDWrap]*mnodeexecution.NodeExecution)
	pendingMutex := sync.Mutex{}

	// Initialize with RUNNING execution
	pendingNodeExecutions[executionID] = &mnodeexecution.NodeExecution{
		ID:     executionID,
		NodeID: nodeID,
		Name:   "Execution 1",
		State:  mnnode.NODE_STATE_RUNNING,
	}

	// Simulate node completion
	flowNodeStatus := runner.FlowNodeStatus{
		ExecutionID: executionID,
		NodeID:      nodeID,
		Name:        "Test Request Node",
		State:       mnnode.NODE_STATE_SUCCESS,
		InputData:   map[string]any{"test": "input"},
		OutputData:  map[string]any{"test": "output"},
	}

	// Process node completion (should NOT send to channel yet)
	err := processNodeCompletion(context.Background(), flowNodeStatus, pendingNodeExecutions, &pendingMutex,
		nodeExecutionChan, mockNodeService)
	require.NoError(t, err)

	// Verify execution is marked complete but still in pending map
	pendingMutex.Lock()
	nodeExec := pendingNodeExecutions[executionID]
	require.NotNil(t, nodeExec)
	assert.Equal(t, mnnode.NODE_STATE_SUCCESS, nodeExec.State)
	assert.Nil(t, nodeExec.ResponseID) // No response yet
	pendingMutex.Unlock()

	// Verify nothing sent to channel yet
	select {
	case <-nodeExecutionChan:
		t.Fatal("Execution should not be sent to channel before response")
	case <-time.After(100 * time.Millisecond):
		// Expected - no execution sent yet
	}

	// Create nodeToExampleMap for mapping
	exampleID := idwrap.NewNow()
	nodeToExampleMap := map[idwrap.IDWrap]idwrap.IDWrap{
		nodeID: exampleID,
	}

	// Simulate response arrival
	nodeResponse := nrequest.NodeRequestSideResp{
		Example: mitemapiexample.ItemApiExample{
			ID: exampleID,
		},
		Resp: response.ResponseCreateOutput{
			ExampleResp: mexampleresp.ExampleResp{
				ID: responseID,
			},
		},
	}

	// Process response (should send execution to channel with ResponseID)
	processRequestResponse(nodeResponse, pendingNodeExecutions, nodeToExampleMap, &pendingMutex, nodeExecutionChan)

	// Verify execution sent to channel with ResponseID
	select {
	case execution := <-nodeExecutionChan:
		assert.Equal(t, executionID, execution.ID)
		assert.Equal(t, mnnode.NODE_STATE_SUCCESS, execution.State)
		require.NotNil(t, execution.ResponseID)
		assert.Equal(t, responseID, *execution.ResponseID)
	case <-time.After(1 * time.Second):
		t.Fatal("Execution should be sent to channel after response")
	}

	// Verify execution removed from pending map
	pendingMutex.Lock()
	_, exists := pendingNodeExecutions[executionID]
	assert.False(t, exists)
	pendingMutex.Unlock()
}

func TestNonRequestNode_ImmediateCompletion(t *testing.T) {
	// Setup
	nodeID := idwrap.NewNow()
	executionID := idwrap.NewNow()

	// Create channels
	nodeExecutionChan := make(chan mnodeexecution.NodeExecution, 10)

	// Mock node service to return FOR node (non-REQUEST)
	mockNodeService := &MockNodeService{
		nodes: map[idwrap.IDWrap]*mnnode.MNode{
			nodeID: {
				ID:       nodeID,
				NodeKind: mnnode.NODE_KIND_FOR,
				Name:     "Test For Node",
			},
		},
	}

	// Create pending execution map
	pendingNodeExecutions := make(map[idwrap.IDWrap]*mnodeexecution.NodeExecution)
	pendingMutex := sync.Mutex{}

	// Initialize with RUNNING execution
	pendingNodeExecutions[executionID] = &mnodeexecution.NodeExecution{
		ID:     executionID,
		NodeID: nodeID,
		Name:   "Execution 1",
		State:  mnnode.NODE_STATE_RUNNING,
	}

	// Simulate node completion
	flowNodeStatus := runner.FlowNodeStatus{
		ExecutionID: executionID,
		NodeID:      nodeID,
		Name:        "Test For Node",
		State:       mnnode.NODE_STATE_SUCCESS,
		InputData:   map[string]any{"test": "input"},
		OutputData:  map[string]any{"test": "output"},
	}

	// Process node completion (should send to channel immediately)
	err := processNodeCompletion(context.Background(), flowNodeStatus, pendingNodeExecutions, &pendingMutex,
		nodeExecutionChan, mockNodeService)
	require.NoError(t, err)

	// Verify execution sent to channel immediately
	select {
	case execution := <-nodeExecutionChan:
		assert.Equal(t, executionID, execution.ID)
		assert.Equal(t, mnnode.NODE_STATE_SUCCESS, execution.State)
		assert.Nil(t, execution.ResponseID) // Non-REQUEST nodes don't have ResponseID
	case <-time.After(1 * time.Second):
		t.Fatal("Non-REQUEST execution should be sent immediately")
	}

	// Verify execution removed from pending map
	pendingMutex.Lock()
	_, exists := pendingNodeExecutions[executionID]
	assert.False(t, exists)
	pendingMutex.Unlock()
}

// Test for REQUEST node timeout when no response arrives
func TestRequestNode_TimeoutWithoutResponse(t *testing.T) {
	// Setup
	nodeID := idwrap.NewNow()
	executionID := idwrap.NewNow()

	// Create channels
	nodeExecutionChan := make(chan mnodeexecution.NodeExecution, 10)

	// Mock node service to return REQUEST node
	mockNodeService := &MockNodeService{
		nodes: map[idwrap.IDWrap]*mnnode.MNode{
			nodeID: {
				ID:       nodeID,
				NodeKind: mnnode.NODE_KIND_REQUEST,
				Name:     "Test Request Node",
			},
		},
	}

	// Create pending execution map
	pendingNodeExecutions := make(map[idwrap.IDWrap]*mnodeexecution.NodeExecution)
	pendingMutex := sync.Mutex{}

	// Initialize with RUNNING execution
	pendingNodeExecutions[executionID] = &mnodeexecution.NodeExecution{
		ID:     executionID,
		NodeID: nodeID,
		Name:   "Execution 1",
		State:  mnnode.NODE_STATE_RUNNING,
	}

	// Simulate node completion
	flowNodeStatus := runner.FlowNodeStatus{
		ExecutionID: executionID,
		NodeID:      nodeID,
		Name:        "Test Request Node",
		State:       mnnode.NODE_STATE_SUCCESS,
		InputData:   map[string]any{"test": "input"},
		OutputData:  map[string]any{"test": "output"},
	}

	// Process node completion (should NOT send to channel yet)
	err := processNodeCompletion(context.Background(), flowNodeStatus, pendingNodeExecutions, &pendingMutex,
		nodeExecutionChan, mockNodeService)
	require.NoError(t, err)

	// Verify execution is marked complete but still in pending map
	pendingMutex.Lock()
	nodeExec := pendingNodeExecutions[executionID]
	require.NotNil(t, nodeExec)
	assert.Equal(t, mnnode.NODE_STATE_SUCCESS, nodeExec.State)
	assert.Nil(t, nodeExec.ResponseID) // No response yet
	pendingMutex.Unlock()

	// Verify nothing sent to channel for a reasonable time (simulating timeout)
	select {
	case <-nodeExecutionChan:
		t.Fatal("Execution should not be sent to channel without response")
	case <-time.After(500 * time.Millisecond):
		// Expected - no execution sent without response
	}

	// Simulate timeout cleanup - send execution without ResponseID
	pendingMutex.Lock()
	if nodeExec, exists := pendingNodeExecutions[executionID]; exists {
		nodeExecutionChan <- *nodeExec
		delete(pendingNodeExecutions, executionID)
	}
	pendingMutex.Unlock()

	// Verify execution is eventually sent without ResponseID
	select {
	case execution := <-nodeExecutionChan:
		assert.Equal(t, executionID, execution.ID)
		assert.Equal(t, mnnode.NODE_STATE_SUCCESS, execution.State)
		assert.Nil(t, execution.ResponseID) // No ResponseID due to timeout
	case <-time.After(1 * time.Second):
		t.Fatal("Execution should be sent to channel after timeout cleanup")
	}

	// Verify execution removed from pending map
	pendingMutex.Lock()
	_, exists := pendingNodeExecutions[executionID]
	assert.False(t, exists)
	pendingMutex.Unlock()
}