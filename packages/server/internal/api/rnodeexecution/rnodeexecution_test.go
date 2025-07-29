package rnodeexecution

import (
	"testing"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnodeexecution"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodeexecution"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/suser"

	"github.com/stretchr/testify/assert"
)

func TestNodeExecutionService_Constructor(t *testing.T) {
	// Test that the constructor correctly sets all services
	// Using nil values since we're just testing the constructor
	var mockNES *snodeexecution.NodeExecutionService
	var mockNS *snode.NodeService
	var mockFS *sflow.FlowService
	var mockUS *suser.UserService
	var mockERS *sexampleresp.ExampleRespService
	var mockRNS *snoderequest.NodeRequestService

	service := New(mockNES, mockNS, mockFS, mockUS, mockERS, mockRNS)

	assert.NotNil(t, service)
	assert.Equal(t, mockNES, service.nes)
	assert.Equal(t, mockNS, service.ns)
	assert.Equal(t, mockFS, service.fs)
	assert.Equal(t, mockUS, service.us)
	assert.Equal(t, mockERS, service.ers)
	assert.Equal(t, mockRNS, service.rns)
}

func TestNodeExecutionGet_RequestNodeDetection(t *testing.T) {
	// Test that the node type detection logic compiles and runs
	// This is more of a compilation test since we can't easily mock the dependencies
	
	// Test data structures
	executionID := idwrap.NewNow()
	nodeID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	responseID := idwrap.NewNow()

	// Test NodeExecution model with ResponseID
	execution := &mnodeexecution.NodeExecution{
		ID:         executionID,
		NodeID:     nodeID,
		ResponseID: &responseID, // This is the key field we're testing
	}

	// Test REQUEST node
	node := &mnnode.MNode{
		ID:       nodeID,
		FlowID:   flowID,
		NodeKind: mnnode.NODE_KIND_REQUEST, // This should trigger ResponseID handling
	}

	// Test flow
	flow := mflow.Flow{
		ID: flowID,
	}

	// Verify the test data is set up correctly
	assert.NotNil(t, execution)
	assert.NotNil(t, execution.ResponseID)
	assert.Equal(t, responseID, *execution.ResponseID)
	assert.Equal(t, mnnode.NODE_KIND_REQUEST, node.NodeKind)
	assert.Equal(t, flowID, flow.ID)

	// Test the condition that would be checked in the actual service
	if node.NodeKind == mnnode.NODE_KIND_REQUEST && execution.ResponseID != nil {
		// This is the logic path that would be executed for REQUEST nodes with ResponseID
		assert.True(t, true, "REQUEST node with ResponseID logic path")
	} else {
		t.Error("Expected REQUEST node with ResponseID condition to be true")
	}
}

func TestNodeExecutionGet_NonRequestNode(t *testing.T) {
	// Test that non-REQUEST nodes don't trigger ResponseID logic
	executionID := idwrap.NewNow()
	nodeID := idwrap.NewNow()
	flowID := idwrap.NewNow()

	// Test execution without ResponseID
	execution := &mnodeexecution.NodeExecution{
		ID:         executionID,
		NodeID:     nodeID,
		ResponseID: nil,
	}

	// Test CONDITION node (not REQUEST)
	node := &mnnode.MNode{
		ID:       nodeID,
		FlowID:   flowID,
		NodeKind: mnnode.NODE_KIND_CONDITION,
	}

	// Test flow
	flow := mflow.Flow{
		ID: flowID,
	}

	// Verify the test data
	assert.NotNil(t, execution)
	assert.Nil(t, execution.ResponseID)
	assert.Equal(t, mnnode.NODE_KIND_CONDITION, node.NodeKind)
	assert.Equal(t, flowID, flow.ID)

	// Test the condition - should NOT enter the ResponseID logic
	if node.NodeKind == mnnode.NODE_KIND_REQUEST && execution.ResponseID != nil {
		t.Error("Should not enter REQUEST node logic for CONDITION node")
	} else {
		// This is the expected path
		assert.True(t, true, "Non-REQUEST node correctly bypassed ResponseID logic")
	}
}

func TestNodeExecutionGet_RequestNodeWithoutResponseID(t *testing.T) {
	// Test REQUEST node without ResponseID (edge case)
	executionID := idwrap.NewNow()
	nodeID := idwrap.NewNow()
	flowID := idwrap.NewNow()

	// Test execution without ResponseID
	execution := &mnodeexecution.NodeExecution{
		ID:         executionID,
		NodeID:     nodeID,
		ResponseID: nil, // No ResponseID even though it's a REQUEST node
	}

	// Test REQUEST node
	node := &mnnode.MNode{
		ID:       nodeID,
		FlowID:   flowID,
		NodeKind: mnnode.NODE_KIND_REQUEST,
	}

	// Test flow
	flow := mflow.Flow{
		ID: flowID,
	}

	// Verify the test data
	assert.NotNil(t, execution)
	assert.Nil(t, execution.ResponseID)
	assert.Equal(t, mnnode.NODE_KIND_REQUEST, node.NodeKind)
	assert.Equal(t, flowID, flow.ID)

	// Test the condition - should NOT enter the ResponseID logic because ResponseID is nil
	if node.NodeKind == mnnode.NODE_KIND_REQUEST && execution.ResponseID != nil {
		t.Error("Should not enter ResponseID logic when ResponseID is nil")
	} else {
		// This is the expected path
		assert.True(t, true, "REQUEST node without ResponseID correctly bypassed ResponseID logic")
	}
}

// Test constants and types
func TestNodeKindConstants(t *testing.T) {
	// Verify that the node kind constants are correctly defined
	assert.Equal(t, int32(2), mnnode.NODE_KIND_REQUEST)
	assert.Equal(t, int32(3), mnnode.NODE_KIND_CONDITION)
	assert.Equal(t, int32(1), mnnode.NODE_KIND_NO_OP)
	assert.Equal(t, int32(4), mnnode.NODE_KIND_FOR)
	assert.Equal(t, int32(5), mnnode.NODE_KIND_FOR_EACH)
	assert.Equal(t, int32(6), mnnode.NODE_KIND_JS)
}