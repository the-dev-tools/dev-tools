package rflowv2

import (
	"testing"

	"the-dev-tools/server/pkg/idwrap"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"

	"github.com/stretchr/testify/assert"
)

func TestNodeEventToSyncResponse_StartNode(t *testing.T) {
	// Create a "Start" node event
	nodeID := idwrap.NewNow()
	flowID := idwrap.NewNow()

	// Construct a Node protobuf that mimics a StartNode
	// StartNode is typically a NO_OP node with name "Start"
	startNodePB := &flowv1.Node{
		NodeId: nodeID.Bytes(),
		FlowId: flowID.Bytes(),
		Kind:   flowv1.NodeKind_NODE_KIND_NO_OP,
		Name:   "Start",
		Position: &flowv1.Position{
			X: 0,
			Y: 0,
		},
	}

	evt := NodeEvent{
		Type:   nodeEventInsert,
		FlowID: flowID,
		Node:   startNodePB,
	}

	// Test that it currently returns nil (filtered out)
	// OR if we fixed it, it should return a response
	resp := nodeEventToSyncResponse(evt)

	// We removed the filtering, so now it should return a response
	if resp == nil {
		t.Error("StartNode is still filtered out!")
	} else {
		t.Log("StartNode is correctly synced")
		assert.Equal(t, flowv1.NodeSync_ValueUnion_KIND_INSERT, resp.Items[0].Value.Kind)
	}
}

func TestNodeEventToSyncResponse_OtherNode(t *testing.T) {
	nodeID := idwrap.NewNow()
	flowID := idwrap.NewNow()

	otherNodePB := &flowv1.Node{
		NodeId: nodeID.Bytes(),
		FlowId: flowID.Bytes(),
		Kind:   flowv1.NodeKind_NODE_KIND_HTTP,
		Name:   "Request",
	}

	evt := NodeEvent{
		Type:   nodeEventInsert,
		FlowID: flowID,
		Node:   otherNodePB,
	}

	resp := nodeEventToSyncResponse(evt)
	assert.NotNil(t, resp)
	assert.Equal(t, flowv1.NodeSync_ValueUnion_KIND_INSERT, resp.Items[0].Value.Kind)
}
