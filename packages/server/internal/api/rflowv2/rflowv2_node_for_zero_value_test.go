package rflowv2

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/dbtest"
	gen "github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/dbtime"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream/memory"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/testutil"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
)

// TestNodeFor_ZeroIterationsSync uses the zero-value sync test framework to verify
// that setting iterations to 0 is correctly synced. This is a regression test for
// the bug where `if iterations != 0` incorrectly excluded zero values from sync updates.
func TestNodeFor_ZeroIterationsSync(t *testing.T) {
	testutil.VerifyZeroValueSync(t,
		testutil.ZeroValueSyncTestConfig[int32, ForEvent]{
			Setup: func(t *testing.T) (context.Context, func()) {
				return setupForZeroValueTest(t)
			},
			StartSync: func(ctx context.Context, t *testing.T) (<-chan ForEvent, func()) {
				// Get the for stream from context (stored during setup)
				forStream := ctx.Value(forStreamKey).(eventstream.SyncStreamer[ForTopic, ForEvent])

				eventChan, err := forStream.Subscribe(ctx, func(topic ForTopic) bool {
					return true
				})
				require.NoError(t, err)

				forEventChan := make(chan ForEvent, 100)
				go func() {
					for evt := range eventChan {
						forEventChan <- evt.Payload
					}
					close(forEventChan)
				}()

				return forEventChan, func() {}
			},
			TriggerUpdate: func(ctx context.Context, t *testing.T, value int32) {
				svc := ctx.Value(forServiceKey).(*FlowServiceV2RPC)
				nodeID := ctx.Value(nodeIDKey).(idwrap.IDWrap)

				updateReq := connect.NewRequest(&flowv1.NodeForUpdateRequest{
					Items: []*flowv1.NodeForUpdate{{
						NodeId:     nodeID.Bytes(),
						Iterations: &value,
					}},
				})

				_, err := svc.NodeForUpdate(ctx, updateReq)
				require.NoError(t, err)
			},
			GetActualValue: func(ctx context.Context, t *testing.T) int32 {
				svc := ctx.Value(forServiceKey).(*FlowServiceV2RPC)
				nodeID := ctx.Value(nodeIDKey).(idwrap.IDWrap)

				nodeFor, err := svc.nfs.GetNodeFor(ctx, nodeID)
				require.NoError(t, err)
				require.NotNil(t, nodeFor)

				return int32(nodeFor.IterCount)
			},
			ExtractSyncedValue: func(t *testing.T, syncItem ForEvent) (value int32, present bool) {
				if syncItem.Node == nil {
					return 0, false
				}

				// Convert to sync response to check what the client would receive
				resp := forEventToSyncResponse(syncItem)
				if resp == nil || len(resp.Items) == 0 {
					return 0, false
				}

				item := resp.Items[0]
				if item.Value == nil {
					return 0, false
				}

				switch item.Value.Kind {
				case flowv1.NodeForSync_ValueUnion_KIND_UPDATE:
					update := item.Value.Update
					if update == nil || update.Iterations == nil {
						return 0, false
					}
					return *update.Iterations, true
				case flowv1.NodeForSync_ValueUnion_KIND_INSERT:
					insert := item.Value.Insert
					if insert == nil {
						return 0, false
					}
					return insert.Iterations, true
				default:
					return 0, false
				}
			},
		},
		testutil.ZeroValueTestCase[int32]{
			Name:         "iterations",
			InitialValue: 5,
			ZeroValue:    0,
		},
	)
}

// Context keys for passing test fixtures
type contextKey string

const (
	forServiceKey contextKey = "forService"
	forStreamKey  contextKey = "forStream"
	nodeIDKey     contextKey = "nodeID"
)

// setupForZeroValueTest creates the test infrastructure for zero-value testing.
func setupForZeroValueTest(t *testing.T) (context.Context, func()) {
	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)

	queries := gen.New(db)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Setup Services
	wsService := sworkspace.NewWorkspaceService(queries)
	flowService := sflow.NewFlowService(queries)
	nodeService := sflow.NewNodeService(queries)
	nodeExecService := sflow.NewNodeExecutionService(queries)
	edgeService := sflow.NewEdgeService(queries)
	flowVarService := sflow.NewFlowVariableService(queries)
	nodeRequestService := sflow.NewNodeRequestService(queries)
	nodeForService := sflow.NewNodeForService(queries)
	nodeForEachService := sflow.NewNodeForEachService(queries)
	nodeIfService := sflow.NewNodeIfService(queries)
	nodeNodeJsService := sflow.NewNodeJsService(queries)

	// Readers
	wsReader := sworkspace.NewWorkspaceReaderFromQueries(queries)
	fsReader := sflow.NewFlowReaderFromQueries(queries)
	nsReader := sflow.NewNodeReaderFromQueries(queries)

	// Create in-memory for stream
	forStream := memory.NewInMemorySyncStreamer[ForTopic, ForEvent]()

	svc := &FlowServiceV2RPC{
		DB:        db,
		wsReader:  wsReader,
		fsReader:  fsReader,
		nsReader:  nsReader,
		ws:        &wsService,
		fs:        &flowService,
		ns:        &nodeService,
		nes:       &nodeExecService,
		es:        &edgeService,
		fvs:       &flowVarService,
		nrs:       &nodeRequestService,
		nfs:       &nodeForService,
		nfes:      &nodeForEachService,
		nifs:      nodeIfService,
		njss:      &nodeNodeJsService,
		logger:    logger,
		forStream: forStream,
	}

	// Setup user and workspace
	userID := idwrap.NewNow()
	ctx = mwauth.CreateAuthedContext(ctx, userID)

	err = queries.CreateUser(ctx, gen.CreateUserParams{
		ID:    userID,
		Email: "test@example.com",
	})
	require.NoError(t, err)

	workspaceID := idwrap.NewNow()
	err = wsService.Create(ctx, &mworkspace.Workspace{
		ID:      workspaceID,
		Name:    "Test Workspace",
		Updated: dbtime.DBNow(),
	})
	require.NoError(t, err)

	err = queries.CreateWorkspaceUser(ctx, gen.CreateWorkspaceUserParams{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        1,
	})
	require.NoError(t, err)

	// Create flow
	flowID := idwrap.NewNow()
	err = svc.fs.CreateFlow(ctx, mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	})
	require.NoError(t, err)

	// Create base node (FOR kind)
	nodeID := idwrap.NewNow()
	err = svc.ns.CreateNode(ctx, mflow.Node{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     "For Node",
		NodeKind: mflow.NODE_KIND_FOR,
	})
	require.NoError(t, err)

	// Insert For node config with initial iterations
	insertReq := connect.NewRequest(&flowv1.NodeForInsertRequest{
		Items: []*flowv1.NodeForInsert{{
			NodeId:     nodeID.Bytes(),
			Iterations: 1, // Start with non-zero
			Condition:  "true",
		}},
	})
	_, err = svc.NodeForInsert(ctx, insertReq)
	require.NoError(t, err)

	// Wait a bit for the insert event to be processed
	time.Sleep(50 * time.Millisecond)

	// Store services in context for later use
	ctx = context.WithValue(ctx, forServiceKey, svc)
	ctx = context.WithValue(ctx, forStreamKey, forStream)
	ctx = context.WithValue(ctx, nodeIDKey, nodeID)

	cleanup := func() {
		db.Close()
	}

	return ctx, cleanup
}
