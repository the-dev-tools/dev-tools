package rflowv2

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/dbtest"
	gen "github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/dbtime"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream/memory"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/http/resolver"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/senv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/testutil"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
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
			nodeExecService := sflow.NewNodeExecutionService(queries)
			edgeService := sflow.NewEdgeService(queries)
			reqService := sflow.NewNodeRequestService(queries)
			forService := sflow.NewNodeForService(queries)
			forEachService := sflow.NewNodeForEachService(queries)
			ifService := sflow.NewNodeIfService(queries)
			jsService := sflow.NewNodeJsService(queries)
			flowVarService := sflow.NewFlowVariableService(queries)
			logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
			envService := senv.NewEnvironmentService(queries, logger)
			varService := senv.NewVariableService(queries, logger)
			httpService := shttp.New(queries, logger)

			flowStream = memory.NewInMemorySyncStreamer[FlowTopic, FlowEvent]()
			nodeStream := memory.NewInMemorySyncStreamer[NodeTopic, NodeEvent]()
			res := resolver.NewStandardResolver(nil, nil, nil, nil, nil, nil, nil)

			svc = New(FlowServiceV2Deps{
				DB: db,
				Readers: FlowServiceV2Readers{
					Workspace: sworkspace.NewWorkspaceReaderFromQueries(queries),
					Flow:      sflow.NewFlowReaderFromQueries(queries),
					Node:      sflow.NewNodeReaderFromQueries(queries),
					Env:       senv.NewEnvReaderFromQueries(queries, logger),
					Http:      shttp.NewReaderFromQueries(queries, logger, nil),
					Edge:      edgeService.Reader(),
				},
				Services: FlowServiceV2Services{
					Workspace:     &wsService,
					Flow:          &flowService,
					Edge:          &edgeService,
					Node:          &nodeService,
					NodeRequest:   &reqService,
					NodeFor:       &forService,
					NodeForEach:   &forEachService,
					NodeIf:        ifService,
					NodeJs:        &jsService,
					NodeExecution: &nodeExecService,
					FlowVariable:  &flowVarService,
					Env:           &envService,
					Var:           &varService,
					Http:          &httpService,
					HttpBodyRaw:   shttp.NewHttpBodyRawService(queries),
				},
				Streamers: FlowServiceV2Streamers{
					Flow: flowStream,
					Node: nodeStream,
				},
				Resolver: res,
				Logger:   logger,
			})

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
			edgeService := sflow.NewEdgeService(queries)
			reqService := sflow.NewNodeRequestService(queries)
			forService := sflow.NewNodeForService(queries)
			forEachService := sflow.NewNodeForEachService(queries)
			ifService := sflow.NewNodeIfService(queries)
			jsService := sflow.NewNodeJsService(queries)
			flowVarService := sflow.NewFlowVariableService(queries)
			logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
			envService := senv.NewEnvironmentService(queries, logger)
			varService := senv.NewVariableService(queries, logger)
			httpService := shttp.New(queries, logger)

			nodeStream = memory.NewInMemorySyncStreamer[NodeTopic, NodeEvent]()
			flowStream := memory.NewInMemorySyncStreamer[FlowTopic, FlowEvent]()
			res := resolver.NewStandardResolver(nil, nil, nil, nil, nil, nil, nil)

			svc = New(FlowServiceV2Deps{
				DB: db,
				Readers: FlowServiceV2Readers{
					Workspace: sworkspace.NewWorkspaceReaderFromQueries(queries),
					Flow:      sflow.NewFlowReaderFromQueries(queries),
					Node:      sflow.NewNodeReaderFromQueries(queries),
					Env:       senv.NewEnvReaderFromQueries(queries, logger),
					Http:      shttp.NewReaderFromQueries(queries, logger, nil),
					Edge:      edgeService.Reader(),
				},
				Services: FlowServiceV2Services{
					Workspace:     &wsService,
					Flow:          &flowService,
					Edge:          &edgeService,
					Node:          &nodeService,
					NodeRequest:   &reqService,
					NodeFor:       &forService,
					NodeForEach:   &forEachService,
					NodeIf:        ifService,
					NodeJs:        &jsService,
					NodeExecution: &nodeExecService,
					FlowVariable:  &flowVarService,
					Env:           &envService,
					Var:           &varService,
					Http:          &httpService,
					HttpBodyRaw:   shttp.NewHttpBodyRawService(queries),
				},
				Streamers: FlowServiceV2Streamers{
					Flow: flowStream,
					Node: nodeStream,
				},
				Resolver: res,
				Logger:   logger,
			})

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
