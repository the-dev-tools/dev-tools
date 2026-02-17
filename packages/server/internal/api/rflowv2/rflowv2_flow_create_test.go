package rflowv2

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/dbtest"
	gen "github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/dbtime"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream/memory"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
)

func TestFlowInsert_PublishesStartNodeEvent(t *testing.T) {
	// Setup DB
	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer db.Close()

	queries := gen.New(db)

	// Setup Services
	wsService := sworkspace.NewWorkspaceService(queries)
	flowService := sflow.NewFlowService(queries)
	nodeService := sflow.NewNodeService(queries)

	// Mock streams
	flowStream := memory.NewInMemorySyncStreamer[FlowTopic, FlowEvent]()
	nodeStream := memory.NewInMemorySyncStreamer[NodeTopic, NodeEvent]()

	// Minimal svc setup for FlowInsert
	svc := &FlowServiceV2RPC{
		DB:         db,
		wsReader:     sworkspace.NewWorkspaceReaderFromQueries(queries),
		wsUserReader: sworkspace.NewUserReaderFromQueries(queries),
		fsReader:   sflow.NewFlowReaderFromQueries(queries),
		ws:         &wsService,
		fs:         &flowService,
		ns:         &nodeService,
		flowStream: flowStream,
		nodeStream: nodeStream,
	}

	// Setup Data
	userID := idwrap.NewNow()
	ctx = mwauth.CreateAuthedContext(ctx, userID)

	err = queries.CreateUser(ctx, gen.CreateUserParams{
		ID:    userID,
		Email: "test@example.com",
	})
	require.NoError(t, err)

	workspaceID := idwrap.NewNow()
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
		Role:        3,
	})
	require.NoError(t, err)

	// Subscribe to node events
	nodeEvents, err := nodeStream.Subscribe(ctx, func(topic NodeTopic) bool { return true })
	require.NoError(t, err)

	// Create Flow
	flowID := idwrap.NewNow()
	req := connect.NewRequest(&flowv1.FlowInsertRequest{
		Items: []*flowv1.FlowInsert{{
			FlowId:      flowID.Bytes(),
			WorkspaceId: workspaceID.Bytes(),
			Name:        "Test Flow",
		}},
	})

	_, err = svc.FlowInsert(ctx, req)
	require.NoError(t, err)

	// Verify Start Node Event
	select {
	case evt := <-nodeEvents:
		assert.Equal(t, nodeEventInsert, evt.Payload.Type)
		assert.Equal(t, flowID, evt.Payload.FlowID)
		assert.Equal(t, "Start", evt.Payload.Node.Name)
		assert.Equal(t, flowv1.NodeKind_NODE_KIND_MANUAL_START, evt.Payload.Node.Kind)
	case <-time.After(1 * time.Second):
		assert.Fail(t, "Timeout waiting for start node event")
	}
}
