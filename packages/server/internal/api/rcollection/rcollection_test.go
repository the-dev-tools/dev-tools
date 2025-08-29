package rcollection

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
	
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/db/pkg/sqlitemem"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcollection"
	"the-dev-tools/server/pkg/model/muser"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	collectionv1 "the-dev-tools/spec/dist/buf/go/collection/v1"
	resourcesv1 "the-dev-tools/spec/dist/buf/go/resources/v1"
	
	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRPC(t *testing.T) (*CollectionServiceRPC, context.Context, idwrap.IDWrap, idwrap.IDWrap) {
	t.Helper()
	
	// Create in-memory database
	db, cleanup, err := sqlitemem.NewSQLiteMem(context.Background())
	require.NoError(t, err)
	t.Cleanup(cleanup)
	
	// Initialize database schema
	queries, err := gen.Prepare(context.Background(), db)
	require.NoError(t, err)
	
	// Create test services
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	cs := scollection.New(queries, logger)
	ws := sworkspace.New(queries)
	us := suser.New(queries)
	
	// Create RPC service
	rpc := New(db, cs, ws, us)
	
	// Create test user and workspace
	userID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()
	
	// Create authenticated context with user ID for permission checking
	ctx := mwauth.CreateAuthedContext(context.Background(), userID)
	
	// Create actual user record in database
	testUser := &muser.User{
		ID:           userID,
		Email:        "test@example.com",
		Password:     []byte("test"),
		ProviderType: muser.Local,
		Status:       muser.Active,
	}
	err = us.CreateUser(ctx, testUser)
	require.NoError(t, err)
	
	// Create actual workspace record in database
	testWorkspace := &mworkspace.Workspace{
		ID:              workspaceID,
		Name:            "Test Workspace",
		FlowCount:       0,
		CollectionCount: 0,
		Updated:         time.Now(),
		ActiveEnv:       idwrap.NewNow(), // Create dummy env IDs
		GlobalEnv:       idwrap.NewNow(),
	}
	err = ws.Create(ctx, testWorkspace)
	require.NoError(t, err)
	
	// Create workspace-user relationship
	err = queries.CreateWorkspaceUser(ctx, gen.CreateWorkspaceUserParams{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        1, // Owner role (valid values are 1, 2, 3)
	})
	require.NoError(t, err)
	
	return &rpc, ctx, userID, workspaceID
}

func TestCollectionMove_Success(t *testing.T) {
	t.Parallel()
	
	rpc, ctx, _, workspaceID := setupTestRPC(t)
	
	// Create test collections
	collection1 := &mcollection.Collection{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		Name:        "Collection 1",
	}
	collection2 := &mcollection.Collection{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		Name:        "Collection 2",
	}
	collection3 := &mcollection.Collection{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		Name:        "Collection 3",
	}
	
	err := rpc.cs.CreateCollection(ctx, collection1)
	require.NoError(t, err)
	err = rpc.cs.CreateCollection(ctx, collection2)
	require.NoError(t, err)
	err = rpc.cs.CreateCollection(ctx, collection3)
	require.NoError(t, err)
	
	tests := []struct {
		name           string
		collectionID   idwrap.IDWrap
		targetID       idwrap.IDWrap
		position       resourcesv1.MovePosition
		expectedError  bool
	}{
		{
			name:         "Move collection1 after collection2",
			collectionID: collection1.ID,
			targetID:     collection2.ID,
			position:     resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			expectedError: false,
		},
		{
			name:         "Move collection3 before collection1",
			collectionID: collection3.ID,
			targetID:     collection1.ID,
			position:     resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
			expectedError: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &connect.Request[collectionv1.CollectionMoveRequest]{
				Msg: &collectionv1.CollectionMoveRequest{
					CollectionId:       tt.collectionID.Bytes(),
					TargetCollectionId: tt.targetID.Bytes(),
					Position:          tt.position,
					WorkspaceId:       workspaceID.Bytes(),
				},
			}
			
			resp, err := rpc.CollectionMove(ctx, req)
			
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
			}
		})
	}
}

func TestCollectionMove_ErrorCases(t *testing.T) {
	t.Parallel()
	
	rpc, ctx, _, workspaceID := setupTestRPC(t)
	
	// Create test collection
	collection1 := &mcollection.Collection{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		Name:        "Collection 1",
	}
	
	err := rpc.cs.CreateCollection(ctx, collection1)
	require.NoError(t, err)
	
	tests := []struct {
		name           string
		collectionID   []byte
		targetID       []byte
		position       resourcesv1.MovePosition
		workspaceID    []byte
		expectedCode   connect.Code
	}{
		{
			name:         "Invalid collection ID",
			collectionID: []byte("invalid"),
			targetID:     collection1.ID.Bytes(),
			position:     resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			workspaceID:  workspaceID.Bytes(),
			expectedCode: connect.CodeInvalidArgument,
		},
		{
			name:         "Invalid target ID",
			collectionID: collection1.ID.Bytes(),
			targetID:     []byte("invalid"),
			position:     resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			workspaceID:  workspaceID.Bytes(),
			expectedCode: connect.CodeInvalidArgument,
		},
		{
			name:         "Move to itself",
			collectionID: collection1.ID.Bytes(),
			targetID:     collection1.ID.Bytes(),
			position:     resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			workspaceID:  workspaceID.Bytes(),
			expectedCode: connect.CodeInvalidArgument,
		},
		{
			name:         "Unspecified position",
			collectionID: collection1.ID.Bytes(),
			targetID:     idwrap.NewNow().Bytes(),
			position:     resourcesv1.MovePosition_MOVE_POSITION_UNSPECIFIED,
			workspaceID:  workspaceID.Bytes(),
			expectedCode: connect.CodeInvalidArgument,
		},
		{
			name:         "Non-existent target collection",
			collectionID: collection1.ID.Bytes(),
			targetID:     idwrap.NewNow().Bytes(),
			position:     resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			workspaceID:  workspaceID.Bytes(),
			expectedCode: connect.CodeNotFound,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &connect.Request[collectionv1.CollectionMoveRequest]{
				Msg: &collectionv1.CollectionMoveRequest{
					CollectionId:       tt.collectionID,
					TargetCollectionId: tt.targetID,
					Position:          tt.position,
					WorkspaceId:       tt.workspaceID,
				},
			}
			
			_, err := rpc.CollectionMove(ctx, req)
			assert.Error(t, err)
			
			if connectErr := new(connect.Error); assert.ErrorAs(t, err, &connectErr) {
				assert.Equal(t, tt.expectedCode, connectErr.Code())
			}
		})
	}
}

func TestCollectionList_OrderedAfterMove(t *testing.T) {
	t.Parallel()
	
	rpc, ctx, _, workspaceID := setupTestRPC(t)
	
	// Create test collections
	collection1 := &mcollection.Collection{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		Name:        "Collection 1",
	}
	collection2 := &mcollection.Collection{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		Name:        "Collection 2",
	}
	collection3 := &mcollection.Collection{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		Name:        "Collection 3",
	}
	
	err := rpc.cs.CreateCollection(ctx, collection1)
	require.NoError(t, err)
	err = rpc.cs.CreateCollection(ctx, collection2)
	require.NoError(t, err)
	err = rpc.cs.CreateCollection(ctx, collection3)
	require.NoError(t, err)
	
	// Get initial order
	listReq := &connect.Request[collectionv1.CollectionListRequest]{
		Msg: &collectionv1.CollectionListRequest{
			WorkspaceId: workspaceID.Bytes(),
		},
	}
	
	listResp, err := rpc.CollectionList(ctx, listReq)
	require.NoError(t, err)
	require.Len(t, listResp.Msg.Items, 3)
	
	// Move collection1 after collection2
	moveReq := &connect.Request[collectionv1.CollectionMoveRequest]{
		Msg: &collectionv1.CollectionMoveRequest{
			CollectionId:       collection1.ID.Bytes(),
			TargetCollectionId: collection2.ID.Bytes(),
			Position:          resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			WorkspaceId:       workspaceID.Bytes(),
		},
	}
	
	_, err = rpc.CollectionMove(ctx, moveReq)
	require.NoError(t, err)
	
	// Verify new order
	listResp, err = rpc.CollectionList(ctx, listReq)
	require.NoError(t, err)
	require.Len(t, listResp.Msg.Items, 3)
	
	// The order should now reflect the move operation
	// This test will help us verify if the ordering is working correctly
	t.Logf("Collection order after move:")
	for i, item := range listResp.Msg.Items {
		collectionID := idwrap.NewFromBytesMust(item.CollectionId)
		t.Logf("  %d: %s (%s)", i, item.Name, collectionID.String())
	}
}

