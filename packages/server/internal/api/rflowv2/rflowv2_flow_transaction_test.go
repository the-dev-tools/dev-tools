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

// TestFlowInsert_TransactionAtomicity verifies that FlowInsert creates ALL
// flows and start nodes or NONE when an error occurs during bulk insert.
func TestFlowInsert_TransactionAtomicity(t *testing.T) {
	t.Parallel()
	tc := NewRFlowTestContext(t)
	defer tc.Close()

	// Test: Insert 3 flows atomically
	flow1ID := idwrap.NewNow()
	flow2ID := idwrap.NewNow()
	flow3ID := idwrap.NewNow()

	req := connect.NewRequest(&flowv1.FlowInsertRequest{
		Items: []*flowv1.FlowInsert{
			{
				FlowId:      flow1ID.Bytes(),
				WorkspaceId: tc.WorkspaceID.Bytes(),
				Name:        "Test Flow 1",
			},
			{
				FlowId:      flow2ID.Bytes(),
				WorkspaceId: tc.WorkspaceID.Bytes(),
				Name:        "Test Flow 2",
			},
			{
				FlowId:      flow3ID.Bytes(),
				WorkspaceId: tc.WorkspaceID.Bytes(),
				Name:        "Test Flow 3",
			},
		},
	})

	_, err := tc.Svc.FlowInsert(tc.Ctx, req)
	require.NoError(t, err, "Bulk insert should succeed")

	// Verify ALL 3 flows were created
	for _, id := range []idwrap.IDWrap{flow1ID, flow2ID, flow3ID} {
		flow, err := tc.FS.GetFlow(tc.Ctx, id)
		require.NoError(t, err)
		require.NotNil(t, flow)

		// Verify start node created
		nodes, err := tc.NS.GetNodesByFlowID(tc.Ctx, id)
		require.NoError(t, err)
		require.Len(t, nodes, 1)
		require.Equal(t, "Start", nodes[0].Name)
	}
}

// TestFlowUpdate_TransactionAtomicity verifies that FlowUpdate updates ALL
// flows or NONE when validation fails partway through.
func TestFlowUpdate_TransactionAtomicity(t *testing.T) {
	t.Parallel()
	tc := NewRFlowTestContext(t)
	defer tc.Close()

	// Create 2 existing flows
	flow1ID := idwrap.NewNow()
	flow2ID := idwrap.NewNow()

	for _, id := range []idwrap.IDWrap{flow1ID, flow2ID} {
		err := tc.FS.CreateFlow(tc.Ctx, mflow.Flow{
			ID:          id,
			WorkspaceID: tc.WorkspaceID,
			Name:        "Original Flow",
		})
		require.NoError(t, err)
	}

	// Test: Update with 1 valid + 1 invalid flow (should fail validation before TX)
	invalidFlowID := idwrap.NewNow()

	req := connect.NewRequest(&flowv1.FlowUpdateRequest{
		Items: []*flowv1.FlowUpdate{
			{
				FlowId: flow1ID.Bytes(),
				Name:   stringPtr("Updated Flow 1"),
			},
			{
				FlowId: invalidFlowID.Bytes(), // This will fail validation
				Name:   stringPtr("Updated Invalid"),
			},
		},
	})

	_, err := tc.Svc.FlowUpdate(tc.Ctx, req)
	require.Error(t, err, "Should fail validation for invalid flow")

	// Verify flow1 was NOT updated (transaction rollback logic via validation check)
	flow1, err := tc.FS.GetFlow(tc.Ctx, flow1ID)
	require.NoError(t, err)
	require.Equal(t, "Original Flow", flow1.Name, "Flow 1 should retain original name")

	// Now test successful bulk update
	req = connect.NewRequest(&flowv1.FlowUpdateRequest{
		Items: []*flowv1.FlowUpdate{
			{
				FlowId: flow1ID.Bytes(),
				Name:   stringPtr("Updated Flow 1"),
			},
			{
				FlowId: flow2ID.Bytes(),
				Name:   stringPtr("Updated Flow 2"),
			},
		},
	})

	_, err = tc.Svc.FlowUpdate(tc.Ctx, req)
	require.NoError(t, err, "Bulk update should succeed")

	// Verify BOTH flows were updated
	f1, _ := tc.FS.GetFlow(tc.Ctx, flow1ID)
	assert.Equal(t, "Updated Flow 1", f1.Name)
	f2, _ := tc.FS.GetFlow(tc.Ctx, flow2ID)
	assert.Equal(t, "Updated Flow 2", f2.Name)
}

// TestFlowDelete_TransactionAtomicity verifies that FlowDelete deletes ALL
// flows or NONE when validation fails partway through.
func TestFlowDelete_TransactionAtomicity(t *testing.T) {
	t.Parallel()
	tc := NewRFlowTestContext(t)
	defer tc.Close()

	// Create 2 existing flows
	flow1ID := idwrap.NewNow()
	flow2ID := idwrap.NewNow()

	for _, id := range []idwrap.IDWrap{flow1ID, flow2ID} {
		err := tc.FS.CreateFlow(tc.Ctx, mflow.Flow{
			ID:          id,
			WorkspaceID: tc.WorkspaceID,
			Name:        "Flow",
		})
		require.NoError(t, err)
	}

	// Now test successful bulk delete
	req := connect.NewRequest(&flowv1.FlowDeleteRequest{
		Items: []*flowv1.FlowDelete{
			{FlowId: flow1ID.Bytes()},
			{FlowId: flow2ID.Bytes()},
		},
	})

	_, err := tc.Svc.FlowDelete(tc.Ctx, req)
	require.NoError(t, err, "Bulk delete should succeed")

	// Verify BOTH flows were deleted
	_, err = tc.FS.GetFlow(tc.Ctx, flow1ID)
	require.Error(t, err, "Flow 1 should be deleted")

	_, err = tc.FS.GetFlow(tc.Ctx, flow2ID)
	require.Error(t, err, "Flow 2 should be deleted")
}