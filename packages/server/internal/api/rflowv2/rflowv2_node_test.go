package rflowv2

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
)

func TestNodeInsert(t *testing.T) {
	tc := NewRFlowTestContext(t)
	defer tc.Close()

	nodeID := idwrap.NewNow()

	req := connect.NewRequest(&flowv1.NodeInsertRequest{
		Items: []*flowv1.NodeInsert{{
			NodeId: nodeID.Bytes(),
			FlowId: tc.FlowID.Bytes(),
			Name:   "New Node",
			Kind:   flowv1.NodeKind_NODE_KIND_HTTP,
			Position: &flowv1.Position{
				X: 100,
				Y: 200,
			},
		}},
	})

	_, err := tc.Svc.NodeInsert(tc.Ctx, req)
	require.NoError(t, err)

	// Verify node exists in DB
	node, err := tc.NS.GetNode(tc.Ctx, nodeID)
	require.NoError(t, err)
	assert.Equal(t, "New Node", node.Name)
	assert.Equal(t, mflow.NODE_KIND_REQUEST, node.NodeKind)
	assert.Equal(t, 100.0, node.PositionX)
	assert.Equal(t, 200.0, node.PositionY)
	assert.Equal(t, tc.FlowID, node.FlowID)
}

func TestNodeUpdate(t *testing.T) {
	tc := NewRFlowTestContext(t)
	defer tc.Close()

	// Create initial node
	nodeID := idwrap.NewNow()
	initialNode := mflow.Node{
		ID:        nodeID,
		FlowID:    tc.FlowID,
		Name:      "Initial Node",
		NodeKind:  mflow.NODE_KIND_REQUEST,
		PositionX: 0,
		PositionY: 0,
	}
	err := tc.NS.CreateNode(tc.Ctx, initialNode)
	require.NoError(t, err)

	// 1. Success Update
	newName := "Updated Node"
	req := connect.NewRequest(&flowv1.NodeUpdateRequest{
		Items: []*flowv1.NodeUpdate{{
			NodeId: nodeID.Bytes(),
			Name:   &newName,
			Position: &flowv1.Position{
				X: 50,
				Y: 60,
			},
		}},
	})

	_, err = tc.Svc.NodeUpdate(tc.Ctx, req)
	require.NoError(t, err)

	// Verify update
	node, err := tc.NS.GetNode(tc.Ctx, nodeID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Node", node.Name)
	assert.Equal(t, 50.0, node.PositionX)
	assert.Equal(t, 60.0, node.PositionY)

	// 2. Unsupported Update: Kind
	kind := flowv1.NodeKind_NODE_KIND_HTTP
	reqKind := connect.NewRequest(&flowv1.NodeUpdateRequest{
		Items: []*flowv1.NodeUpdate{{
			NodeId: nodeID.Bytes(),
			Kind:   &kind,
		}},
	})
	_, err = tc.Svc.NodeUpdate(tc.Ctx, reqKind)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "node kind updates are not supported")

	// 3. Unsupported Update: Flow Reassignment
	reqFlow := connect.NewRequest(&flowv1.NodeUpdateRequest{
		Items: []*flowv1.NodeUpdate{{
			NodeId: nodeID.Bytes(),
			FlowId: idwrap.NewNow().Bytes(),
		}},
	})
	_, err = tc.Svc.NodeUpdate(tc.Ctx, reqFlow)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "node flow reassignment is not supported")
}

func TestNodeDelete(t *testing.T) {
	tc := NewRFlowTestContext(t)
	defer tc.Close()

	// Create node to delete
	nodeID := idwrap.NewNow()
	node := mflow.Node{
		ID:        nodeID,
		FlowID:    tc.FlowID,
		Name:      "Node To Delete",
		NodeKind:  mflow.NODE_KIND_REQUEST,
		PositionX: 0,
		PositionY: 0,
	}
	err := tc.NS.CreateNode(tc.Ctx, node)
	require.NoError(t, err)

	// Delete Node
	req := connect.NewRequest(&flowv1.NodeDeleteRequest{
		Items: []*flowv1.NodeDelete{{
			NodeId: nodeID.Bytes(),
		}},
	})

	_, err = tc.Svc.NodeDelete(tc.Ctx, req)
	require.NoError(t, err)

	// Verify node is gone
	_, err = tc.NS.GetNode(tc.Ctx, nodeID)
	require.Error(t, err)
}