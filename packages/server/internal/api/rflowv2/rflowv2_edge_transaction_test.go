package rflowv2

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
)

// TestEdgeInsert_TransactionRollback verifies that if inserting multiple edges fails,
// ALL edges are rolled back (not just the ones after the failure).
func TestEdgeInsert_TransactionRollback(t *testing.T) {
	t.Parallel()
	tc := NewRFlowTestContext(t)
	defer tc.Close()

	// Create nodes for the edges
	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()
	node3ID := idwrap.NewNow()

	for i, id := range []idwrap.IDWrap{node1ID, node2ID, node3ID} {
		err := tc.NS.CreateNode(tc.Ctx, mflow.Node{
			ID:        id,
			FlowID:    tc.FlowID,
			Name:      "Node",
			NodeKind:  mflow.NODE_KIND_REQUEST,
			PositionX: float64(i * 100),
			PositionY: 0,
		})
		require.NoError(t, err)
	}

	// Attempt to insert 3 edges, but the 2nd one will fail due to invalid flow access
	invalidFlowID := idwrap.NewNow() // User doesn't have access to this flow

	req := connect.NewRequest(&flowv1.EdgeInsertRequest{
		Items: []*flowv1.EdgeInsert{
			{
				EdgeId:   idwrap.NewNow().Bytes(),
				FlowId:   tc.FlowID.Bytes(),
				SourceId: node1ID.Bytes(),
				TargetId: node2ID.Bytes(),
			},
			{
				EdgeId:   idwrap.NewNow().Bytes(),
				FlowId:   invalidFlowID.Bytes(), // Invalid - user doesn't have access
				SourceId: node2ID.Bytes(),
				TargetId: node3ID.Bytes(),
			},
			{
				EdgeId:   idwrap.NewNow().Bytes(),
				FlowId:   tc.FlowID.Bytes(),
				SourceId: node1ID.Bytes(),
				TargetId: node3ID.Bytes(),
			},
		},
	})

	// Execute the insert - this should fail validation before transaction
	_, err := tc.Svc.EdgeInsert(tc.Ctx, req)
	require.Error(t, err, "Insert should fail due to invalid flow access")

	// Verify NO edges were inserted
	edges, err := tc.ES.GetEdgesByFlowID(tc.Ctx, tc.FlowID)
	require.NoError(t, err)
	require.Empty(t, edges, "No edges should be inserted when validation fails")
}

// TestEdgeInsert_PartialSuccess_ValidatesFirst verifies that all items are validated
// before the transaction begins, so we never get partial inserts.
func TestEdgeInsert_PartialSuccess_ValidatesFirst(t *testing.T) {
	t.Parallel()
	tc := NewRFlowTestContext(t)
	defer tc.Close()

	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()

	for i, id := range []idwrap.IDWrap{node1ID, node2ID} {
		err := tc.NS.CreateNode(tc.Ctx, mflow.Node{
			ID:        id,
			FlowID:    tc.FlowID,
			Name:      "Node",
			NodeKind:  mflow.NODE_KIND_REQUEST,
			PositionX: float64(i * 100),
			PositionY: 0,
		})
		require.NoError(t, err)
	}

	invalidFlowID := idwrap.NewNow()

	req := connect.NewRequest(&flowv1.EdgeInsertRequest{
		Items: []*flowv1.EdgeInsert{
			{
				EdgeId:   idwrap.NewNow().Bytes(),
				FlowId:   tc.FlowID.Bytes(),
				SourceId: node1ID.Bytes(),
				TargetId: node2ID.Bytes(),
			},
			{
				EdgeId:   idwrap.NewNow().Bytes(),
				FlowId:   invalidFlowID.Bytes(), // Invalid flow - user doesn't have access
				SourceId: node1ID.Bytes(),
				TargetId: node2ID.Bytes(),
			},
		},
	})

	_, err := tc.Svc.EdgeInsert(tc.Ctx, req)
	require.Error(t, err, "Insert should fail due to invalid flow access")

	// Verify edge1 was NOT inserted
	edges, err := tc.ES.GetEdgesByFlowID(tc.Ctx, tc.FlowID)
	require.NoError(t, err)
	require.Empty(t, edges, "Edge 1 should NOT be inserted when edge 2 validation fails")
}

// TestEdgeInsert_AllOrNothing verifies successful batch insert
func TestEdgeInsert_AllOrNothing(t *testing.T) {
	t.Parallel()
	tc := NewRFlowTestContext(t)
	defer tc.Close()

	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()
	node3ID := idwrap.NewNow()

	for i, id := range []idwrap.IDWrap{node1ID, node2ID, node3ID} {
		err := tc.NS.CreateNode(tc.Ctx, mflow.Node{
			ID:        id,
			FlowID:    tc.FlowID,
			Name:      "Node",
			NodeKind:  mflow.NODE_KIND_REQUEST,
			PositionX: float64(i * 100),
			PositionY: 0,
		})
		require.NoError(t, err)
	}

	// Insert 3 valid edges
	edge1ID := idwrap.NewNow()
	edge2ID := idwrap.NewNow()
	edge3ID := idwrap.NewNow()

	req := connect.NewRequest(&flowv1.EdgeInsertRequest{
		Items: []*flowv1.EdgeInsert{
			{
				EdgeId:   edge1ID.Bytes(),
				FlowId:   tc.FlowID.Bytes(),
				SourceId: node1ID.Bytes(),
				TargetId: node2ID.Bytes(),
			},
			{
				EdgeId:   edge2ID.Bytes(),
				FlowId:   tc.FlowID.Bytes(),
				SourceId: node2ID.Bytes(),
				TargetId: node3ID.Bytes(),
			},
			{
				EdgeId:   edge3ID.Bytes(),
				FlowId:   tc.FlowID.Bytes(),
				SourceId: node1ID.Bytes(),
				TargetId: node3ID.Bytes(),
			},
		},
	})

	_, err := tc.Svc.EdgeInsert(tc.Ctx, req)
	require.NoError(t, err, "All valid edges should insert successfully")

	// Verify ALL 3 edges were inserted
	edges, err := tc.ES.GetEdgesByFlowID(tc.Ctx, tc.FlowID)
	require.NoError(t, err)
	require.Len(t, edges, 3, "All 3 edges should be inserted")

	// Verify the edge IDs
	edgeIDs := make(map[string]bool)
	for _, edge := range edges {
		edgeIDs[edge.ID.String()] = true
	}

	require.True(t, edgeIDs[edge1ID.String()], "Edge 1 should exist")
	require.True(t, edgeIDs[edge2ID.String()], "Edge 2 should exist")
	require.True(t, edgeIDs[edge3ID.String()], "Edge 3 should exist")
}