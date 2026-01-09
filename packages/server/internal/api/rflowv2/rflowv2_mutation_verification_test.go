package rflowv2

import (
	"context"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream/memory"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/mutation"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
)

// eventRecorder captures events published by the mutation system.
type eventRecorder struct {
	events []mutation.Event
}

func (r *eventRecorder) PublishAll(events []mutation.Event) {
	r.events = append(r.events, events...)
}

// TestMutation_FlowDelete_CascadeVerify verifies that deleting a flow
// correctly tracks delete events for all its children (nodes, edges, variables).
func TestMutation_FlowDelete_CascadeVerify(t *testing.T) {
	t.Parallel()
	tc := NewRFlowTestContext(t)
	defer tc.Close()

	// Create children for the flow
	node1ID := idwrap.NewNow()
	err := tc.NS.CreateNode(tc.Ctx, mflow.Node{
		ID:       node1ID,
		FlowID:   tc.FlowID,
		Name:     "Node 1",
		NodeKind: mflow.NODE_KIND_REQUEST,
	})
	require.NoError(t, err)

	node2ID := idwrap.NewNow()
	err = tc.NS.CreateNode(tc.Ctx, mflow.Node{
		ID:       node2ID,
		FlowID:   tc.FlowID,
		Name:     "Node 2",
		NodeKind: mflow.NODE_KIND_REQUEST,
	})
	require.NoError(t, err)

	edgeID := idwrap.NewNow()
	err = tc.ES.CreateEdge(tc.Ctx, mflow.Edge{
		ID:       edgeID,
		FlowID:   tc.FlowID,
		SourceID: node1ID,
		TargetID: node2ID,
	})
	require.NoError(t, err)

	varID := idwrap.NewNow()
	err = tc.FVS.CreateFlowVariable(tc.Ctx, mflow.FlowVariable{
		ID:      varID,
		FlowID:  tc.FlowID,
		Name:    "test_var",
		Value:   "test_val",
		Enabled: true,
	})
	require.NoError(t, err)

	recorder := &eventRecorder{}

	t.Run("Mutation_DeleteFlowBatch_TracksAllCascades", func(t *testing.T) {
		mut := mutation.New(tc.DB, mutation.WithPublisher(recorder))
		err := mut.Begin(tc.Ctx)
		require.NoError(t, err)

		err = mut.DeleteFlowBatch(tc.Ctx, []mutation.FlowDeleteItem{
			{ID: tc.FlowID, WorkspaceID: tc.WorkspaceID},
		})
		require.NoError(t, err)

		err = mut.Commit(tc.Ctx)
		require.NoError(t, err)

		// Verify events tracked
		// Expected: 1 Flow delete, 2 Node deletes, 1 Edge delete, 1 Variable delete
		assert.Equal(t, 5, len(recorder.events), "Should have tracked 5 delete events")

		eventMap := make(map[string]mutation.Operation)
		for _, e := range recorder.events {
			key := fmt.Sprintf("%d:%s", e.Entity, e.ID.String())
			eventMap[key] = e.Op
		}

		assert.Equal(t, mutation.OpDelete, eventMap[fmt.Sprintf("%d:%s", mutation.EntityFlow, tc.FlowID.String())])
		assert.Equal(t, mutation.OpDelete, eventMap[fmt.Sprintf("%d:%s", mutation.EntityFlowNode, node1ID.String())])
		assert.Equal(t, mutation.OpDelete, eventMap[fmt.Sprintf("%d:%s", mutation.EntityFlowNode, node2ID.String())])
		assert.Equal(t, mutation.OpDelete, eventMap[fmt.Sprintf("%d:%s", mutation.EntityFlowEdge, edgeID.String())])
		assert.Equal(t, mutation.OpDelete, eventMap[fmt.Sprintf("%d:%s", mutation.EntityFlowVariable, varID.String())])
	})
}

// TestMutation_SyncParity_FlowDelete verifies that FlowDelete RPC
// results in a sync stream event that removes the flow.
func TestMutation_SyncParity_FlowDelete(t *testing.T) {
	t.Parallel()
	tc := NewRFlowTestContext(t)
	defer tc.Close()

	// Inject memory stream into RPC service
	tc.Svc.flowStream = memory.NewInMemorySyncStreamer[FlowTopic, FlowEvent]()

	// Subscribe to sync stream
	syncCtx, cancelSync := context.WithCancel(tc.Ctx)
	defer cancelSync()

	syncEvents, err := tc.Svc.flowStream.Subscribe(syncCtx, func(topic FlowTopic) bool {
		return topic.WorkspaceID == tc.WorkspaceID
	})
	require.NoError(t, err)

	// Trigger Delete via RPC
	req := connect.NewRequest(&flowv1.FlowDeleteRequest{
		Items: []*flowv1.FlowDelete{{FlowId: tc.FlowID.Bytes()}},
	})

	_, err = tc.Svc.FlowDelete(tc.Ctx, req)
	require.NoError(t, err)

	// Verify sync event received
	select {
	case evt := <-syncEvents:
		assert.Equal(t, flowEventDelete, evt.Payload.Type)
		assert.Equal(t, tc.FlowID.Bytes(), evt.Payload.Flow.FlowId)
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for sync event")
	}
}
