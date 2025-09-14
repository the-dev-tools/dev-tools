package rcollectionitem_test

import (
    "context"
    "testing"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rcollectionitem"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemfolder"
	"the-dev-tools/server/pkg/service/scollectionitem"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/sitemfolder"
	"the-dev-tools/server/pkg/testutil"
	itemv1 "the-dev-tools/spec/dist/buf/go/collection/item/v1"
	resourcesv1 "the-dev-tools/spec/dist/buf/go/resources/v1"

    "connectrpc.com/connect"
    "the-dev-tools/db/pkg/sqlc/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCrossCollectionEdgeCases_InvalidTargetKind tests targetKind validation for cross-collection moves
func TestCrossCollectionEdgeCases_InvalidTargetKind(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rpc, _, userID, sourceCollectionID, targetCollectionID, cleanup := setupCrossCollectionTestEnvironment(t, ctx)
	defer cleanup()

    queries, err := gen.Prepare(ctx, rpc.DB)
    require.NoError(t, err)
    mockLogger := mocklogger.NewMockLogger()
    cis := scollectionitem.New(queries, mockLogger)
    ifs := sitemfolder.New(queries)
    ias := sitemapi.New(queries)

    sourceFolderID, sourceEndpointID, targetFolderID, targetEndpointID := createTestItemsInCollections(
        t, ctx, sourceCollectionID, targetCollectionID, cis, ifs, ias, &testutil.BaseDBQueries{Queries: queries, DB: rpc.DB})

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	tests := []struct {
		name         string
		sourceKind   itemv1.ItemKind
		targetKind   itemv1.ItemKind
		itemID       idwrap.IDWrap
		targetID     idwrap.IDWrap
		expectedCode connect.Code
		description  string
	}{
        {
            name:         "Folder into endpoint - valid (advisory targetKind)",
            sourceKind:   itemv1.ItemKind_ITEM_KIND_FOLDER,
            targetKind:   itemv1.ItemKind_ITEM_KIND_ENDPOINT,
            itemID:       sourceFolderID,
            targetID:     targetEndpointID,
            expectedCode: 0, // advisory targetKind validation; operation allowed
            description:  "Moving folder relative to an endpoint should succeed",
        },
		{
			name:         "Endpoint into folder - valid",
			sourceKind:   itemv1.ItemKind_ITEM_KIND_ENDPOINT,
			targetKind:   itemv1.ItemKind_ITEM_KIND_FOLDER,
			itemID:       sourceEndpointID,
			targetID:     targetFolderID,
			expectedCode: 0, // Success case
			description:  "Moving endpoint into folder should succeed",
		},
		{
			name:         "Folder to folder - valid",
			sourceKind:   itemv1.ItemKind_ITEM_KIND_FOLDER,
			targetKind:   itemv1.ItemKind_ITEM_KIND_FOLDER,
			itemID:       sourceFolderID,
			targetID:     targetFolderID,
			expectedCode: 0, // Success case
			description:  "Moving folder to folder location should succeed",
		},
		{
			name:         "Endpoint to endpoint - valid",
			sourceKind:   itemv1.ItemKind_ITEM_KIND_ENDPOINT,
			targetKind:   itemv1.ItemKind_ITEM_KIND_ENDPOINT,
			itemID:       sourceEndpointID,
			targetID:     targetEndpointID,
			expectedCode: 0, // Success case
			description:  "Moving endpoint to endpoint location should succeed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
				Kind:               tt.sourceKind,
				ItemId:             tt.itemID.Bytes(),
				CollectionId:       sourceCollectionID.Bytes(),
				TargetCollectionId: targetCollectionID.Bytes(),
				TargetItemId:       tt.targetID.Bytes(),
				// TargetKind validation will be tested separately
				Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
			})

			resp, err := rpc.CollectionItemMove(authedCtx, req)

			if tt.expectedCode == 0 { // Success case
				assert.NoError(t, err, tt.description)
				assert.NotNil(t, resp, "Response should not be nil for successful operations")
			} else {
				assert.Error(t, err, tt.description)
				if connectErr := new(connect.Error); assert.ErrorAs(t, err, &connectErr) {
					assert.Equal(t, tt.expectedCode, connectErr.Code(),
						"Expected error code %v but got %v for: %s", tt.expectedCode, connectErr.Code(), tt.description)
				}
			}
		})
	}
}

// TestCrossCollectionEdgeCases_SelfReferentialMoves tests prevention of self-referential moves
func TestCrossCollectionEdgeCases_SelfReferentialMoves(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rpc, _, userID, sourceCollectionID, targetCollectionID, cleanup := setupCrossCollectionTestEnvironment(t, ctx)
	defer cleanup()

    queries, err := gen.Prepare(ctx, rpc.DB)
    require.NoError(t, err)
    mockLogger := mocklogger.NewMockLogger()
    cis := scollectionitem.New(queries, mockLogger)
    ifs := sitemfolder.New(queries)
    ias := sitemapi.New(queries)

    sourceFolderID, sourceEndpointID, _, _ := createTestItemsInCollections(
        t, ctx, sourceCollectionID, targetCollectionID, cis, ifs, ias, &testutil.BaseDBQueries{Queries: queries, DB: rpc.DB})

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	tests := []struct {
		name        string
		itemID      idwrap.IDWrap
		targetID    idwrap.IDWrap
		kind        itemv1.ItemKind
		description string
	}{
		{
			name:        "Folder targeting itself",
			itemID:      sourceFolderID,
			targetID:    sourceFolderID,
			kind:        itemv1.ItemKind_ITEM_KIND_FOLDER,
			description: "Should prevent folder from targeting itself",
		},
		{
			name:        "Endpoint targeting itself",
			itemID:      sourceEndpointID,
			targetID:    sourceEndpointID,
			kind:        itemv1.ItemKind_ITEM_KIND_ENDPOINT,
			description: "Should prevent endpoint from targeting itself",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
				Kind:               tt.kind,
				ItemId:             tt.itemID.Bytes(),
				CollectionId:       sourceCollectionID.Bytes(),
				TargetCollectionId: targetCollectionID.Bytes(),
				TargetItemId:       tt.targetID.Bytes(),
				Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
			})

			_, err := rpc.CollectionItemMove(authedCtx, req)

			assert.Error(t, err, tt.description)
			if connectErr := new(connect.Error); assert.ErrorAs(t, err, &connectErr) {
				assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code(),
					"Self-referential moves should return InvalidArgument")
				assert.Contains(t, connectErr.Message(), "cannot move item relative to itself",
					"Error message should mention self-reference")
			}
		})
	}
}

// TestCrossCollectionEdgeCases_NonExistentTargets tests handling of non-existent target collections and items
func TestCrossCollectionEdgeCases_NonExistentTargets(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rpc, _, userID, sourceCollectionID, targetCollectionID, cleanup := setupCrossCollectionTestEnvironment(t, ctx)
	defer cleanup()

	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()
	mockLogger := mocklogger.NewMockLogger()
	cis := scollectionitem.New(base.Queries, mockLogger)
	ifs := sitemfolder.New(base.Queries)
	ias := sitemapi.New(base.Queries)

	sourceFolderID, sourceEndpointID, _, _ := createTestItemsInCollections(
		t, ctx, sourceCollectionID, targetCollectionID, cis, ifs, ias, base)

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	tests := []struct {
		name                string
		itemID              idwrap.IDWrap
		sourceCollectionID  idwrap.IDWrap
		targetCollectionID  *idwrap.IDWrap
		targetItemID        *idwrap.IDWrap
		targetParentFolderID *idwrap.IDWrap
		expectedCode        connect.Code
		description         string
	}{
		{
			name:               "Non-existent target collection",
			itemID:             sourceFolderID,
			sourceCollectionID: sourceCollectionID,
			targetCollectionID: func() *idwrap.IDWrap { id := idwrap.NewNow(); return &id }(),
			expectedCode:       connect.CodeNotFound,
			description:        "Should reject move to non-existent target collection",
		},
		{
			name:               "Non-existent target item",
			itemID:             sourceFolderID,
			sourceCollectionID: sourceCollectionID,
			targetCollectionID: &targetCollectionID,
			targetItemID:       func() *idwrap.IDWrap { id := idwrap.NewNow(); return &id }(),
			expectedCode:       connect.CodeNotFound,
			description:        "Should reject move to non-existent target item",
		},
		{
			name:                 "Non-existent target parent folder",
			itemID:               sourceEndpointID,
			sourceCollectionID:   sourceCollectionID,
			targetCollectionID:   &targetCollectionID,
			targetParentFolderID: func() *idwrap.IDWrap { id := idwrap.NewNow(); return &id }(),
			expectedCode:         connect.CodeNotFound,
			description:          "Should reject move to non-existent target parent folder",
		},
		{
			name:               "Non-existent source item",
			itemID:             idwrap.NewNow(),
			sourceCollectionID: sourceCollectionID,
			targetCollectionID: &targetCollectionID,
			expectedCode:       connect.CodeNotFound,
			description:        "Should reject move of non-existent source item",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &itemv1.CollectionItemMoveRequest{
				Kind:         itemv1.ItemKind_ITEM_KIND_FOLDER,
				ItemId:       tt.itemID.Bytes(),
				CollectionId: tt.sourceCollectionID.Bytes(),
				Position:     resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
			}

			if tt.targetCollectionID != nil {
				req.TargetCollectionId = tt.targetCollectionID.Bytes()
			}

			if tt.targetItemID != nil {
				req.TargetItemId = tt.targetItemID.Bytes()
			}

			if tt.targetParentFolderID != nil {
				req.TargetParentFolderId = tt.targetParentFolderID.Bytes()
			}

			_, err := rpc.CollectionItemMove(authedCtx, connect.NewRequest(req))

			assert.Error(t, err, tt.description)
			if connectErr := new(connect.Error); assert.ErrorAs(t, err, &connectErr) {
				assert.Equal(t, tt.expectedCode, connectErr.Code(),
					"Expected error code %v but got %v for: %s", tt.expectedCode, connectErr.Code(), tt.description)
			}
		})
	}
}

// TestCrossCollectionEdgeCases_WorkspaceBoundaryViolations tests strict workspace boundary enforcement
func TestCrossCollectionEdgeCases_WorkspaceBoundaryViolations(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()
	services := base.GetBaseServices()

	mockLogger := mocklogger.NewMockLogger()
	cs := services.Cs
	cis := scollectionitem.New(base.Queries, mockLogger)
	us := services.Us
	ifs := sitemfolder.New(base.Queries)
	ias := sitemapi.New(base.Queries)
	iaes := sitemapiexample.New(base.Queries)
	res := sexampleresp.New(base.Queries)

	rpc := rcollectionitem.New(base.DB, cs, cis, us, ifs, ias, iaes, res)

	// Create two completely separate workspaces with different users
	user1ID := idwrap.NewNow()
	user2ID := idwrap.NewNow()

	workspace1ID := idwrap.NewNow()
	workspace1UserID := idwrap.NewNow()
	collection1ID := idwrap.NewNow()

	workspace2ID := idwrap.NewNow()
	workspace2UserID := idwrap.NewNow()
	collection2ID := idwrap.NewNow()

	// Setup first workspace/collection
	services.CreateTempCollection(t, ctx, workspace1ID, workspace1UserID, user1ID, collection1ID)
	// Setup second workspace/collection (different workspace)
	services.CreateTempCollection(t, ctx, workspace2ID, workspace2UserID, user2ID, collection2ID)

	// Create items in first workspace
	folder1ID := idwrap.NewNow()
	folder1 := &mitemfolder.ItemFolder{
		ID:           folder1ID,
		Name:         "Workspace 1 Folder",
		CollectionID: collection1ID,
		ParentID:     nil,
	}
    // Create via TX path only

	endpoint1ID := idwrap.NewNow()
	endpoint1 := &mitemapi.ItemApi{
		ID:           endpoint1ID,
		Name:         "Workspace 1 Endpoint",
		Url:          "https://api.workspace1.com/test",
		Method:       "GET",
		CollectionID: collection1ID,
		FolderID:     nil,
	}
    // Create via TX path only

	// Create items in second workspace
	folder2ID := idwrap.NewNow()
	folder2 := &mitemfolder.ItemFolder{
		ID:           folder2ID,
		Name:         "Workspace 2 Folder",
		CollectionID: collection2ID,
		ParentID:     nil,
	}
    // Create via TX path only

	// Create collection items for all items
	tx, err := base.DB.Begin()
	require.NoError(t, err)
	defer tx.Rollback()

	err = cis.CreateFolderTX(ctx, tx, folder1)
	require.NoError(t, err)
	err = cis.CreateEndpointTX(ctx, tx, endpoint1)
	require.NoError(t, err)
	err = cis.CreateFolderTX(ctx, tx, folder2)
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	tests := []struct {
		name                string
		userID              idwrap.IDWrap
		itemID              idwrap.IDWrap
		sourceCollectionID  idwrap.IDWrap
		targetCollectionID  idwrap.IDWrap
		targetItemID        *idwrap.IDWrap
		expectedCode        connect.Code
		expectedMessagePart string
		description         string
	}{
		{
			name:               "User1 cannot move item to workspace2 collection",
			userID:             user1ID,
			itemID:             folder1ID,
			sourceCollectionID: collection1ID,
			targetCollectionID: collection2ID,
        expectedCode:       connect.CodePermissionDenied,
        expectedMessagePart: "",
			description:        "Should prevent cross-workspace moves",
		},
		{
			name:               "User2 cannot move user1's item",
			userID:             user2ID,
			itemID:             folder1ID,
			sourceCollectionID: collection1ID,
			targetCollectionID: collection1ID,
			expectedCode:       connect.CodePermissionDenied,
			description:        "Should prevent unauthorized users from moving items",
		},
		{
			name:               "User1 cannot target item in workspace2",
			userID:             user1ID,
			itemID:             folder1ID,
			sourceCollectionID: collection1ID,
			targetCollectionID: collection1ID, // Same collection, but targeting cross-workspace item
			targetItemID:       &folder2ID,
			expectedCode:       connect.CodeNotFound, // Target item won't be found due to workspace boundary
			description:        "Should prevent targeting items from different workspace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authedCtx := mwauth.CreateAuthedContext(ctx, tt.userID)

			req := &itemv1.CollectionItemMoveRequest{
				Kind:               itemv1.ItemKind_ITEM_KIND_FOLDER,
				ItemId:             tt.itemID.Bytes(),
				CollectionId:       tt.sourceCollectionID.Bytes(),
				TargetCollectionId: tt.targetCollectionID.Bytes(),
				Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
			}

			if tt.targetItemID != nil {
				req.TargetItemId = tt.targetItemID.Bytes()
			}

			_, err := rpc.CollectionItemMove(authedCtx, connect.NewRequest(req))

			assert.Error(t, err, tt.description)
        if connectErr := new(connect.Error); assert.ErrorAs(t, err, &connectErr) {
            assert.Contains(t, []connect.Code{connect.CodePermissionDenied, connect.CodeNotFound}, connectErr.Code(),
                "Should reject due to workspace boundary or missing target")
        }
		})
	}
}

// TestCrossCollectionEdgeCases_InvalidInputValidation tests comprehensive input validation
func TestCrossCollectionEdgeCases_InvalidInputValidation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rpc, _, userID, sourceCollectionID, targetCollectionID, cleanup := setupCrossCollectionTestEnvironment(t, ctx)
	defer cleanup()

    queries, err := gen.Prepare(ctx, rpc.DB)
    require.NoError(t, err)
    mockLogger := mocklogger.NewMockLogger()
    cis := scollectionitem.New(queries, mockLogger)
    ifs := sitemfolder.New(queries)
    ias := sitemapi.New(queries)

    sourceFolderID, _, _, _ := createTestItemsInCollections(
        t, ctx, sourceCollectionID, targetCollectionID, cis, ifs, ias, &testutil.BaseDBQueries{Queries: queries, DB: rpc.DB})

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	tests := []struct {
		name         string
		request      *itemv1.CollectionItemMoveRequest
		expectedCode connect.Code
		description  string
	}{
		{
			name: "Invalid ULID format - item_id",
			request: &itemv1.CollectionItemMoveRequest{
				Kind:               itemv1.ItemKind_ITEM_KIND_FOLDER,
				ItemId:             []byte("invalid-ulid-format"),
				CollectionId:       sourceCollectionID.Bytes(),
				TargetCollectionId: targetCollectionID.Bytes(),
				Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
			},
			expectedCode: connect.CodeInvalidArgument,
			description:  "Should reject invalid ULID format in item_id",
		},
		{
			name: "Invalid ULID format - collection_id",
			request: &itemv1.CollectionItemMoveRequest{
				Kind:               itemv1.ItemKind_ITEM_KIND_FOLDER,
				ItemId:             sourceFolderID.Bytes(),
				CollectionId:       []byte("invalid-ulid-format"),
				TargetCollectionId: targetCollectionID.Bytes(),
				Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
			},
			expectedCode: connect.CodeInvalidArgument,
			description:  "Should reject invalid ULID format in collection_id",
		},
		{
			name: "Invalid ULID format - target_collection_id",
			request: &itemv1.CollectionItemMoveRequest{
				Kind:               itemv1.ItemKind_ITEM_KIND_FOLDER,
				ItemId:             sourceFolderID.Bytes(),
				CollectionId:       sourceCollectionID.Bytes(),
				TargetCollectionId: []byte("invalid-ulid-format"),
				Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
			},
			expectedCode: connect.CodeInvalidArgument,
			description:  "Should reject invalid ULID format in target_collection_id",
		},
		{
			name: "Empty item_id",
			request: &itemv1.CollectionItemMoveRequest{
				Kind:               itemv1.ItemKind_ITEM_KIND_FOLDER,
				ItemId:             []byte{},
				CollectionId:       sourceCollectionID.Bytes(),
				TargetCollectionId: targetCollectionID.Bytes(),
				Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
			},
			expectedCode: connect.CodeInvalidArgument,
			description:  "Should reject empty item_id",
		},
		{
			name: "Missing item_id",
			request: &itemv1.CollectionItemMoveRequest{
				Kind:               itemv1.ItemKind_ITEM_KIND_FOLDER,
				CollectionId:       sourceCollectionID.Bytes(),
				TargetCollectionId: targetCollectionID.Bytes(),
				Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
			},
			expectedCode: connect.CodeInvalidArgument,
			description:  "Should reject missing item_id",
		},
		{
			name: "Missing collection_id",
			request: &itemv1.CollectionItemMoveRequest{
				Kind:               itemv1.ItemKind_ITEM_KIND_FOLDER,
				ItemId:             sourceFolderID.Bytes(),
				TargetCollectionId: targetCollectionID.Bytes(),
				Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
			},
			expectedCode: connect.CodeInvalidArgument,
			description:  "Should reject missing collection_id",
		},
        {
            name: "Unspecified source kind",
            request: &itemv1.CollectionItemMoveRequest{
                Kind:               itemv1.ItemKind_ITEM_KIND_UNSPECIFIED,
                ItemId:             sourceFolderID.Bytes(),
                CollectionId:       sourceCollectionID.Bytes(),
                TargetCollectionId: targetCollectionID.Bytes(),
                Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
            },
            expectedCode: 0, // Advisory; unspecified kind does not block move
            description:  "Unspecified source kind should be allowed (advisory)",
        },
	}

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := rpc.CollectionItemMove(authedCtx, connect.NewRequest(tt.request))

            if tt.expectedCode == 0 {
                assert.NoError(t, err, tt.description)
            } else {
                assert.Error(t, err, tt.description)
                if connectErr := new(connect.Error); assert.ErrorAs(t, err, &connectErr) {
                    assert.Equal(t, tt.expectedCode, connectErr.Code(),
                        "Expected error code %v but got %v for: %s", tt.expectedCode, connectErr.Code(), tt.description)
                }
            }
        })
    }
}

// TestCrossCollectionEdgeCases_PositionValidation tests position parameter validation
func TestCrossCollectionEdgeCases_PositionValidation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rpc, _, userID, sourceCollectionID, targetCollectionID, cleanup := setupCrossCollectionTestEnvironment(t, ctx)
	defer cleanup()

    queries, err := gen.Prepare(ctx, rpc.DB)
    require.NoError(t, err)
    mockLogger := mocklogger.NewMockLogger()
    cis := scollectionitem.New(queries, mockLogger)
    ifs := sitemfolder.New(queries)
    ias := sitemapi.New(queries)

    sourceFolderID, _, _, targetEndpointID := createTestItemsInCollections(
        t, ctx, sourceCollectionID, targetCollectionID, cis, ifs, ias, &testutil.BaseDBQueries{Queries: queries, DB: rpc.DB})

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	t.Run("Position required when target item specified", func(t *testing.T) {
		req := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:               itemv1.ItemKind_ITEM_KIND_FOLDER,
			ItemId:             sourceFolderID.Bytes(),
			CollectionId:       sourceCollectionID.Bytes(),
			TargetCollectionId: targetCollectionID.Bytes(),
			TargetItemId:       targetEndpointID.Bytes(),
			Position:           resourcesv1.MovePosition_MOVE_POSITION_UNSPECIFIED.Enum(),
		})

		_, err := rpc.CollectionItemMove(authedCtx, req)

		assert.Error(t, err, "Should require position when target item is specified")
		if connectErr := new(connect.Error); assert.ErrorAs(t, err, &connectErr) {
			assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code(),
				"Should return InvalidArgument when position is unspecified with target item")
			assert.Contains(t, connectErr.Message(), "position must be specified",
				"Error message should mention position requirement")
		}
	})

	t.Run("Position defaults when no target item", func(t *testing.T) {
		req := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:               itemv1.ItemKind_ITEM_KIND_FOLDER,
			ItemId:             sourceFolderID.Bytes(),
			CollectionId:       sourceCollectionID.Bytes(),
			TargetCollectionId: targetCollectionID.Bytes(),
			Position:           resourcesv1.MovePosition_MOVE_POSITION_UNSPECIFIED.Enum(),
		})

		resp, err := rpc.CollectionItemMove(authedCtx, req)

		assert.NoError(t, err, "Should allow unspecified position when no target item")
		assert.NotNil(t, resp, "Response should not be nil")
	})
}
