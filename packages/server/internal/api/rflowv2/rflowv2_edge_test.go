package rflowv2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
)

func TestEdgeEventToSyncResponse_StateField(t *testing.T) {
	edgeID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	sourceID := idwrap.NewNow()
	targetID := idwrap.NewNow()

	t.Run("Edge event UPDATE includes State field when not UNSPECIFIED", func(t *testing.T) {
		edgeEvent := EdgeEvent{
			Type:   edgeEventUpdate,
			FlowID: flowID,
			Edge: &flowv1.Edge{
				EdgeId:       edgeID.Bytes(),
				FlowId:       flowID.Bytes(),
				SourceId:     sourceID.Bytes(),
				TargetId:     targetID.Bytes(),
				SourceHandle: flowv1.HandleKind_HANDLE_KIND_THEN,
				State:        flowv1.FlowItemState_FLOW_ITEM_STATE_SUCCESS,
			},
		}

		resp := edgeEventToSyncResponse(edgeEvent)
		require.NotNil(t, resp)
		require.Len(t, resp.Items, 1)

		update := resp.Items[0].Value.GetUpdate()
		require.NotNil(t, update)

		assert.NotNil(t, update.State, "State field should be included when not UNSPECIFIED")
		assert.Equal(t, flowv1.FlowItemState_FLOW_ITEM_STATE_SUCCESS, *update.State)
	})

	t.Run("Edge event UPDATE includes State field even when UNSPECIFIED", func(t *testing.T) {
		edgeEvent := EdgeEvent{
			Type:   edgeEventUpdate,
			FlowID: flowID,
			Edge: &flowv1.Edge{
				EdgeId:       edgeID.Bytes(),
				FlowId:       flowID.Bytes(),
				SourceId:     sourceID.Bytes(),
				TargetId:     targetID.Bytes(),
				SourceHandle: flowv1.HandleKind_HANDLE_KIND_THEN,
				State:        flowv1.FlowItemState_FLOW_ITEM_STATE_UNSPECIFIED,
			},
		}

		resp := edgeEventToSyncResponse(edgeEvent)
		require.NotNil(t, resp)
		require.Len(t, resp.Items, 1)

		update := resp.Items[0].Value.GetUpdate()
		require.NotNil(t, update)

		// State should always be included to support resetting to UNSPECIFIED
		assert.NotNil(t, update.State, "State field should always be included")
		assert.Equal(t, flowv1.FlowItemState_FLOW_ITEM_STATE_UNSPECIFIED, *update.State)
	})

	t.Run("Edge event UPDATE includes State for FAILURE", func(t *testing.T) {
		edgeEvent := EdgeEvent{
			Type:   edgeEventUpdate,
			FlowID: flowID,
			Edge: &flowv1.Edge{
				EdgeId:       edgeID.Bytes(),
				FlowId:       flowID.Bytes(),
				SourceId:     sourceID.Bytes(),
				TargetId:     targetID.Bytes(),
				SourceHandle: flowv1.HandleKind_HANDLE_KIND_THEN,
				State:        flowv1.FlowItemState_FLOW_ITEM_STATE_FAILURE,
			},
		}

		resp := edgeEventToSyncResponse(edgeEvent)
		require.NotNil(t, resp)
		require.Len(t, resp.Items, 1)

		update := resp.Items[0].Value.GetUpdate()
		require.NotNil(t, update)

		assert.NotNil(t, update.State, "State field should be included for FAILURE state")
		assert.Equal(t, flowv1.FlowItemState_FLOW_ITEM_STATE_FAILURE, *update.State)
	})

	t.Run("Edge event UPDATE includes State for RUNNING", func(t *testing.T) {
		edgeEvent := EdgeEvent{
			Type:   edgeEventUpdate,
			FlowID: flowID,
			Edge: &flowv1.Edge{
				EdgeId:       edgeID.Bytes(),
				FlowId:       flowID.Bytes(),
				SourceId:     sourceID.Bytes(),
				TargetId:     targetID.Bytes(),
				SourceHandle: flowv1.HandleKind_HANDLE_KIND_THEN,
				State:        flowv1.FlowItemState_FLOW_ITEM_STATE_RUNNING,
			},
		}

		resp := edgeEventToSyncResponse(edgeEvent)
		require.NotNil(t, resp)
		require.Len(t, resp.Items, 1)

		update := resp.Items[0].Value.GetUpdate()
		require.NotNil(t, update)

		assert.NotNil(t, update.State, "State field should be included for RUNNING state")
		assert.Equal(t, flowv1.FlowItemState_FLOW_ITEM_STATE_RUNNING, *update.State)
	})

	t.Run("Edge event UPDATE includes State for CANCELED", func(t *testing.T) {
		edgeEvent := EdgeEvent{
			Type:   edgeEventUpdate,
			FlowID: flowID,
			Edge: &flowv1.Edge{
				EdgeId:       edgeID.Bytes(),
				FlowId:       flowID.Bytes(),
				SourceId:     sourceID.Bytes(),
				TargetId:     targetID.Bytes(),
				SourceHandle: flowv1.HandleKind_HANDLE_KIND_THEN,
				State:        flowv1.FlowItemState_FLOW_ITEM_STATE_CANCELED,
			},
		}

		resp := edgeEventToSyncResponse(edgeEvent)
		require.NotNil(t, resp)
		require.Len(t, resp.Items, 1)

		update := resp.Items[0].Value.GetUpdate()
		require.NotNil(t, update)

		assert.NotNil(t, update.State, "State field should be included for CANCELED state")
		assert.Equal(t, flowv1.FlowItemState_FLOW_ITEM_STATE_CANCELED, *update.State)
	})

	t.Run("Edge event INSERT includes all fields", func(t *testing.T) {
		edgeEvent := EdgeEvent{
			Type:   edgeEventInsert,
			FlowID: flowID,
			Edge: &flowv1.Edge{
				EdgeId:       edgeID.Bytes(),
				FlowId:       flowID.Bytes(),
				SourceId:     sourceID.Bytes(),
				TargetId:     targetID.Bytes(),
				SourceHandle: flowv1.HandleKind_HANDLE_KIND_THEN,
				State:        flowv1.FlowItemState_FLOW_ITEM_STATE_UNSPECIFIED,
			},
		}

		resp := edgeEventToSyncResponse(edgeEvent)
		require.NotNil(t, resp)
		require.Len(t, resp.Items, 1)

		insert := resp.Items[0].Value.GetInsert()
		require.NotNil(t, insert)

		assert.Equal(t, edgeID.Bytes(), insert.EdgeId)
		assert.Equal(t, flowID.Bytes(), insert.FlowId)
		assert.Equal(t, sourceID.Bytes(), insert.SourceId)
		assert.Equal(t, targetID.Bytes(), insert.TargetId)
		assert.Equal(t, flowv1.HandleKind_HANDLE_KIND_THEN, insert.SourceHandle)
		// INSERT events don't have state field in the sync response (state is in the Edge model)
	})

	t.Run("Edge event DELETE", func(t *testing.T) {
		edgeEvent := EdgeEvent{
			Type:   edgeEventDelete,
			FlowID: flowID,
			Edge: &flowv1.Edge{
				EdgeId: edgeID.Bytes(),
			},
		}

		resp := edgeEventToSyncResponse(edgeEvent)
		require.NotNil(t, resp)
		require.Len(t, resp.Items, 1)

		del := resp.Items[0].Value.GetDelete()
		require.NotNil(t, del)

		assert.Equal(t, edgeID.Bytes(), del.EdgeId)
	})

	t.Run("Edge event unknown type returns nil", func(t *testing.T) {
		edgeEvent := EdgeEvent{
			Type:   "unknown",
			FlowID: flowID,
			Edge: &flowv1.Edge{
				EdgeId: edgeID.Bytes(),
			},
		}

		resp := edgeEventToSyncResponse(edgeEvent)
		assert.Nil(t, resp)
	})

	t.Run("Edge event with nil Edge returns nil", func(t *testing.T) {
		edgeEvent := EdgeEvent{
			Type:   edgeEventUpdate,
			FlowID: flowID,
			Edge:   nil,
		}

		resp := edgeEventToSyncResponse(edgeEvent)
		assert.Nil(t, resp)
	})
}

func TestSerializeEdge_WithState(t *testing.T) {
	edgeID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	sourceID := idwrap.NewNow()
	targetID := idwrap.NewNow()

	tests := []struct {
		name     string
		edge     mflow.Edge
		expected flowv1.FlowItemState
	}{
		{
			name: "Edge with UNSPECIFIED state",
			edge: mflow.Edge{
				ID:            edgeID,
				FlowID:        flowID,
				SourceID:      sourceID,
				TargetID:      targetID,
				SourceHandler: mflow.HandleThen,
				State:         mflow.NODE_STATE_UNSPECIFIED,
			},
			expected: flowv1.FlowItemState_FLOW_ITEM_STATE_UNSPECIFIED,
		},
		{
			name: "Edge with SUCCESS state",
			edge: mflow.Edge{
				ID:            edgeID,
				FlowID:        flowID,
				SourceID:      sourceID,
				TargetID:      targetID,
				SourceHandler: mflow.HandleThen,
				State:         mflow.NODE_STATE_SUCCESS,
			},
			expected: flowv1.FlowItemState_FLOW_ITEM_STATE_SUCCESS,
		},
		{
			name: "Edge with FAILURE state",
			edge: mflow.Edge{
				ID:            edgeID,
				FlowID:        flowID,
				SourceID:      sourceID,
				TargetID:      targetID,
				SourceHandler: mflow.HandleThen,
				State:         mflow.NODE_STATE_FAILURE,
			},
			expected: flowv1.FlowItemState_FLOW_ITEM_STATE_FAILURE,
		},
		{
			name: "Edge with RUNNING state",
			edge: mflow.Edge{
				ID:            edgeID,
				FlowID:        flowID,
				SourceID:      sourceID,
				TargetID:      targetID,
				SourceHandler: mflow.HandleThen,
				State:         mflow.NODE_STATE_RUNNING,
			},
			expected: flowv1.FlowItemState_FLOW_ITEM_STATE_RUNNING,
		},
		{
			name: "Edge with CANCELED state",
			edge: mflow.Edge{
				ID:            edgeID,
				FlowID:        flowID,
				SourceID:      sourceID,
				TargetID:      targetID,
				SourceHandler: mflow.HandleThen,
				State:         mflow.NODE_STATE_CANCELED,
			},
			expected: flowv1.FlowItemState_FLOW_ITEM_STATE_CANCELED,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := serializeEdge(tt.edge)
			require.NotNil(t, result)

			assert.Equal(t, edgeID.Bytes(), result.EdgeId)
			assert.Equal(t, flowID.Bytes(), result.FlowId)
			assert.Equal(t, sourceID.Bytes(), result.SourceId)
			assert.Equal(t, targetID.Bytes(), result.TargetId)
			assert.Equal(t, tt.expected, result.State)
		})
	}
}
