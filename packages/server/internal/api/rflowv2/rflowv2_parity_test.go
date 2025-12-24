package rflowv2

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/db/pkg/dbtest"
	gen "the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/eventstream"
	"the-dev-tools/server/pkg/eventstream/memory"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/testutil"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
)

func TestFlowInsert_FlowParity(t *testing.T) {
	var svc *FlowServiceV2RPC
	var flowStream eventstream.SyncStreamer[FlowTopic, FlowEvent]
	var workspaceID idwrap.IDWrap
	var flowID idwrap.IDWrap

	testutil.VerifySyncParity(t, testutil.SyncParityTestConfig[*flowv1.Flow, *flowv1.FlowSync]{
		Setup: func(t *testing.T) (context.Context, func()) {
			ctx := context.Background()
			db, err := dbtest.GetTestDB(ctx)
			require.NoError(t, err)

			queries := gen.New(db)
			wsService := sworkspace.NewWorkspaceService(queries)
			flowService := sflow.NewFlowService(queries)
			nodeService := sflow.NewNodeService(queries)

			flowStream = memory.NewInMemorySyncStreamer[FlowTopic, FlowEvent]()
			nodeStream := memory.NewInMemorySyncStreamer[NodeTopic, NodeEvent]()

			svc = &FlowServiceV2RPC{
				DB:         db,
				wsReader:   sworkspace.NewWorkspaceReaderFromQueries(queries),
				fsReader:   sflow.NewFlowReaderFromQueries(queries),
				ws:         &wsService,
				fs:         &flowService,
				ns:         &nodeService,
				flowStream: flowStream,
				nodeStream: nodeStream,
			}

			userID := idwrap.NewNow()
			ctx = mwauth.CreateAuthedContext(ctx, userID)

			err = queries.CreateUser(ctx, gen.CreateUserParams{
				ID:    userID,
				Email: "test@example.com",
			})
			require.NoError(t, err)

			workspaceID = idwrap.NewNow()
			workspace := mworkspace.Workspace{
				ID:              workspaceID,
				Name:            "Test Workspace",
				Updated:         dbtime.DBNow(),
				CollectionCount: 0,
				FlowCount:       0,
			}
			err = wsService.Create(ctx, &workspace)
			require.NoError(t, err)

			err = queries.CreateWorkspaceUser(ctx, gen.CreateWorkspaceUserParams{
				ID:          idwrap.NewNow(),
				WorkspaceID: workspaceID,
				UserID:      userID,
				Role:        1,
			})
			require.NoError(t, err)

			return ctx, func() { db.Close() }
		},
		StartSync: func(ctx context.Context, t *testing.T) (<-chan *flowv1.FlowSync, func()) {
			ch := make(chan *flowv1.FlowSync, 10)
			ctx, cancel := context.WithCancel(ctx)

			// Helper to bridge the stream to the channel
			go func() {
				defer close(ch)
				_ = svc.streamFlowSync(ctx, func(resp *flowv1.FlowSyncResponse) error {
					for _, item := range resp.Items {
						ch <- item
					}
					return nil
				})
			}()

			return ch, cancel
		},
		TriggerUpdate: func(ctx context.Context, t *testing.T) {
			flowID = idwrap.NewNow()
			req := connect.NewRequest(&flowv1.FlowInsertRequest{
				Items: []*flowv1.FlowInsert{{
					FlowId:      flowID.Bytes(),
					WorkspaceId: workspaceID.Bytes(),
					Name:        "Test Flow",
				}},
			})
			_, err := svc.FlowInsert(ctx, req)
			require.NoError(t, err)
		},
		GetCollection: func(ctx context.Context, t *testing.T) []*flowv1.Flow {
			resp, err := svc.FlowCollection(ctx, connect.NewRequest(&emptypb.Empty{}))
			require.NoError(t, err)
			return resp.Msg.Items
		},
		Compare: func(t *testing.T, collItem *flowv1.Flow, syncItem *flowv1.FlowSync) {
			val := syncItem.GetValue()
			require.NotNil(t, val)
			insert := val.GetInsert()
			require.NotNil(t, insert)
			assert.Equal(t, collItem.FlowId, insert.FlowId)
			assert.Equal(t, collItem.Name, insert.Name)
			assert.Equal(t, collItem.WorkspaceId, insert.WorkspaceId)
		},
	})
}

func TestFlowInsert_StartNodeParity(t *testing.T) {
	var svc *FlowServiceV2RPC
	var nodeStream eventstream.SyncStreamer[NodeTopic, NodeEvent]
	var workspaceID idwrap.IDWrap

	testutil.VerifySyncParity(t, testutil.SyncParityTestConfig[*flowv1.Node, *flowv1.NodeSync]{
		Setup: func(t *testing.T) (context.Context, func()) {
			ctx := context.Background()
			db, err := dbtest.GetTestDB(ctx)
			require.NoError(t, err)

			queries := gen.New(db)
			wsService := sworkspace.NewWorkspaceService(queries)
			flowService := sflow.NewFlowService(queries)
			nodeService := sflow.NewNodeService(queries)
			nodeExecService := sflow.NewNodeExecutionService(queries)

			// We need these readers for NodeCollection
			nsReader := sflow.NewNodeReaderFromQueries(queries)

			flowStream := memory.NewInMemorySyncStreamer[FlowTopic, FlowEvent]()
			nodeStream = memory.NewInMemorySyncStreamer[NodeTopic, NodeEvent]()

			svc = &FlowServiceV2RPC{
				DB:         db,
				wsReader:   sworkspace.NewWorkspaceReaderFromQueries(queries),
				fsReader:   sflow.NewFlowReaderFromQueries(queries),
				nsReader:   nsReader,
				ws:         &wsService,
				fs:         &flowService,
				ns:         &nodeService,
				nes:        &nodeExecService,
				flowStream: flowStream,
				nodeStream: nodeStream,
			}

			userID := idwrap.NewNow()
			ctx = mwauth.CreateAuthedContext(ctx, userID)

			err = queries.CreateUser(ctx, gen.CreateUserParams{
				ID:    userID,
				Email: "test@example.com",
			})
			require.NoError(t, err)

			workspaceID = idwrap.NewNow()
			workspace := mworkspace.Workspace{
				ID:              workspaceID,
				Name:            "Test Workspace",
				Updated:         dbtime.DBNow(),
				CollectionCount: 0,
				FlowCount:       0,
			}
			err = wsService.Create(ctx, &workspace)
			require.NoError(t, err)

			err = queries.CreateWorkspaceUser(ctx, gen.CreateWorkspaceUserParams{
				ID:          idwrap.NewNow(),
				WorkspaceID: workspaceID,
				UserID:      userID,
				Role:        1,
			})
			require.NoError(t, err)

			return ctx, func() { db.Close() }
		},
		StartSync: func(ctx context.Context, t *testing.T) (<-chan *flowv1.NodeSync, func()) {
			ch := make(chan *flowv1.NodeSync, 10)
			ctx, cancel := context.WithCancel(ctx)

			go func() {
				defer close(ch)
				_ = svc.streamNodeSync(ctx, func(resp *flowv1.NodeSyncResponse) error {
					for _, item := range resp.Items {
						ch <- item
					}
					return nil
				})
			}()

			return ch, cancel
		},
		TriggerUpdate: func(ctx context.Context, t *testing.T) {
			flowID := idwrap.NewNow()
			req := connect.NewRequest(&flowv1.FlowInsertRequest{
				Items: []*flowv1.FlowInsert{{
					FlowId:      flowID.Bytes(),
					WorkspaceId: workspaceID.Bytes(),
					Name:        "Test Flow",
				}},
			})
			_, err := svc.FlowInsert(ctx, req)
			require.NoError(t, err)
		},
		GetCollection: func(ctx context.Context, t *testing.T) []*flowv1.Node {
			resp, err := svc.NodeCollection(ctx, connect.NewRequest(&emptypb.Empty{}))
			require.NoError(t, err)
			return resp.Msg.Items
		},
		Compare: func(t *testing.T, collItem *flowv1.Node, syncItem *flowv1.NodeSync) {
			val := syncItem.GetValue()
			require.NotNil(t, val)
			insert := val.GetInsert()
			require.NotNil(t, insert)
			assert.Equal(t, collItem.NodeId, insert.NodeId)
			assert.Equal(t, collItem.Name, insert.Name)
			assert.Equal(t, collItem.Kind, insert.Kind)
			// Start node should be MANUAL_START
			assert.Equal(t, flowv1.NodeKind_NODE_KIND_MANUAL_START, collItem.Kind)
		},
	})
}

