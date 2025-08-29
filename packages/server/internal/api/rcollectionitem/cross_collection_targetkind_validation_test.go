package rcollectionitem_test

import (
	"context"
	"testing"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemfolder"
	"the-dev-tools/server/pkg/service/scollectionitem"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemfolder"
	"the-dev-tools/server/pkg/testutil"
	itemv1 "the-dev-tools/spec/dist/buf/go/collection/item/v1"
	resourcesv1 "the-dev-tools/spec/dist/buf/go/resources/v1"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TargetKindTestCase represents a targetKind validation test scenario
type TargetKindTestCase struct {
	name                string
	sourceKind          itemv1.ItemKind
	targetKind          itemv1.ItemKind
	expectError         bool
	expectedCode        connect.Code
	expectedMessagePart string
	description         string
	setupFunc           func(t *testing.T, ctx context.Context, sourceCollectionID, targetCollectionID idwrap.IDWrap, cis *scollectionitem.CollectionItemService, ifs sitemfolder.ItemFolderService, ias sitemapi.ItemApiService) (sourceItemID, targetItemID idwrap.IDWrap)
}

// TestCrossCollectionTargetKindValidation tests comprehensive targetKind validation scenarios
func TestCrossCollectionTargetKindValidation(t *testing.T) {
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

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	tests := []TargetKindTestCase{
		{
			name:        "Valid: Folder to Folder",
			sourceKind:  itemv1.ItemKind_ITEM_KIND_FOLDER,
			targetKind:  itemv1.ItemKind_ITEM_KIND_FOLDER,
			expectError: false,
			description: "Moving folder relative to another folder should succeed",
			setupFunc: func(t *testing.T, ctx context.Context, sourceCollectionID, targetCollectionID idwrap.IDWrap, cis *scollectionitem.CollectionItemService, ifs sitemfolder.ItemFolderService, ias sitemapi.ItemApiService) (sourceItemID, targetItemID idwrap.IDWrap) {
				// Create source folder
				sourceFolderID := idwrap.NewNow()
				sourceFolder := &mitemfolder.ItemFolder{
					ID:           sourceFolderID,
					Name:         "Source Documentation",
					CollectionID: sourceCollectionID,
					ParentID:     nil,
				}
				err := ifs.CreateItemFolder(ctx, sourceFolder)
				require.NoError(t, err)

				// Create target folder
				targetFolderID := idwrap.NewNow()
				targetFolder := &mitemfolder.ItemFolder{
					ID:           targetFolderID,
					Name:         "Target APIs",
					CollectionID: targetCollectionID,
					ParentID:     nil,
				}
				err = ifs.CreateItemFolder(ctx, targetFolder)
				require.NoError(t, err)

				// Create collection items
				tx, err := base.DB.Begin()
				require.NoError(t, err)
				defer tx.Rollback()

				err = cis.CreateFolderTX(ctx, tx, sourceFolder)
				require.NoError(t, err)
				err = cis.CreateFolderTX(ctx, tx, targetFolder)
				require.NoError(t, err)
				err = tx.Commit()
				require.NoError(t, err)

				return sourceFolderID, targetFolderID
			},
		},
		{
			name:                "Invalid: Folder to Endpoint",
			sourceKind:          itemv1.ItemKind_ITEM_KIND_FOLDER,
			targetKind:          itemv1.ItemKind_ITEM_KIND_ENDPOINT,
			expectError:         true,
			expectedCode:        connect.CodeInvalidArgument,
			expectedMessagePart: "cannot move folder into an endpoint",
			description:         "Moving folder into an endpoint should fail",
			setupFunc: func(t *testing.T, ctx context.Context, sourceCollectionID, targetCollectionID idwrap.IDWrap, cis *scollectionitem.CollectionItemService, ifs sitemfolder.ItemFolderService, ias sitemapi.ItemApiService) (sourceItemID, targetItemID idwrap.IDWrap) {
				// Create source folder
				sourceFolderID := idwrap.NewNow()
				sourceFolder := &mitemfolder.ItemFolder{
					ID:           sourceFolderID,
					Name:         "Admin Tools",
					CollectionID: sourceCollectionID,
					ParentID:     nil,
				}
				err := ifs.CreateItemFolder(ctx, sourceFolder)
				require.NoError(t, err)

				// Create target endpoint
				targetEndpointID := idwrap.NewNow()
				targetEndpoint := &mitemapi.ItemApi{
					ID:           targetEndpointID,
					Name:         "GET /admin/settings",
					Url:          "https://api.myapp.com/admin/settings",
					Method:       "GET",
					CollectionID: targetCollectionID,
					FolderID:     nil,
				}
				err = ias.CreateItemApi(ctx, targetEndpoint)
				require.NoError(t, err)

				// Create collection items
				tx, err := base.DB.Begin()
				require.NoError(t, err)
				defer tx.Rollback()

				err = cis.CreateFolderTX(ctx, tx, sourceFolder)
				require.NoError(t, err)
				err = cis.CreateEndpointTX(ctx, tx, targetEndpoint)
				require.NoError(t, err)
				err = tx.Commit()
				require.NoError(t, err)

				return sourceFolderID, targetEndpointID
			},
		},
		{
			name:        "Valid: Endpoint to Folder",
			sourceKind:  itemv1.ItemKind_ITEM_KIND_ENDPOINT,
			targetKind:  itemv1.ItemKind_ITEM_KIND_FOLDER,
			expectError: false,
			description: "Moving endpoint into a folder should succeed",
			setupFunc: func(t *testing.T, ctx context.Context, sourceCollectionID, targetCollectionID idwrap.IDWrap, cis *scollectionitem.CollectionItemService, ifs sitemfolder.ItemFolderService, ias sitemapi.ItemApiService) (sourceItemID, targetItemID idwrap.IDWrap) {
				// Create source endpoint
				sourceEndpointID := idwrap.NewNow()
				sourceEndpoint := &mitemapi.ItemApi{
					ID:           sourceEndpointID,
					Name:         "POST /users/create",
					Url:          "https://api.myapp.com/users/create",
					Method:       "POST",
					CollectionID: sourceCollectionID,
					FolderID:     nil,
				}
				err := ias.CreateItemApi(ctx, sourceEndpoint)
				require.NoError(t, err)

				// Create target folder
				targetFolderID := idwrap.NewNow()
				targetFolder := &mitemfolder.ItemFolder{
					ID:           targetFolderID,
					Name:         "User Management",
					CollectionID: targetCollectionID,
					ParentID:     nil,
				}
				err = ifs.CreateItemFolder(ctx, targetFolder)
				require.NoError(t, err)

				// Create collection items
				tx, err := base.DB.Begin()
				require.NoError(t, err)
				defer tx.Rollback()

				err = cis.CreateEndpointTX(ctx, tx, sourceEndpoint)
				require.NoError(t, err)
				err = cis.CreateFolderTX(ctx, tx, targetFolder)
				require.NoError(t, err)
				err = tx.Commit()
				require.NoError(t, err)

				return sourceEndpointID, targetFolderID
			},
		},
		{
			name:        "Valid: Endpoint to Endpoint",
			sourceKind:  itemv1.ItemKind_ITEM_KIND_ENDPOINT,
			targetKind:  itemv1.ItemKind_ITEM_KIND_ENDPOINT,
			expectError: false,
			description: "Moving endpoint relative to another endpoint should succeed",
			setupFunc: func(t *testing.T, ctx context.Context, sourceCollectionID, targetCollectionID idwrap.IDWrap, cis *scollectionitem.CollectionItemService, ifs sitemfolder.ItemFolderService, ias sitemapi.ItemApiService) (sourceItemID, targetItemID idwrap.IDWrap) {
				// Create source endpoint
				sourceEndpointID := idwrap.NewNow()
				sourceEndpoint := &mitemapi.ItemApi{
					ID:           sourceEndpointID,
					Name:         "PUT /users/update",
					Url:          "https://api.myapp.com/users/update",
					Method:       "PUT",
					CollectionID: sourceCollectionID,
					FolderID:     nil,
				}
				err := ias.CreateItemApi(ctx, sourceEndpoint)
				require.NoError(t, err)

				// Create target endpoint
				targetEndpointID := idwrap.NewNow()
				targetEndpoint := &mitemapi.ItemApi{
					ID:           targetEndpointID,
					Name:         "DELETE /users/delete",
					Url:          "https://api.myapp.com/users/delete",
					Method:       "DELETE",
					CollectionID: targetCollectionID,
					FolderID:     nil,
				}
				err = ias.CreateItemApi(ctx, targetEndpoint)
				require.NoError(t, err)

				// Create collection items
				tx, err := base.DB.Begin()
				require.NoError(t, err)
				defer tx.Rollback()

				err = cis.CreateEndpointTX(ctx, tx, sourceEndpoint)
				require.NoError(t, err)
				err = cis.CreateEndpointTX(ctx, tx, targetEndpoint)
				require.NoError(t, err)
				err = tx.Commit()
				require.NoError(t, err)

				return sourceEndpointID, targetEndpointID
			},
		},
		{
			name:                "Invalid: Unspecified Source Kind",
			sourceKind:          itemv1.ItemKind_ITEM_KIND_UNSPECIFIED,
			targetKind:          itemv1.ItemKind_ITEM_KIND_FOLDER,
			expectError:         true,
			expectedCode:        connect.CodeInvalidArgument,
			expectedMessagePart: "source kind must be specified",
			description:         "Unspecified source kind should fail",
			setupFunc: func(t *testing.T, ctx context.Context, sourceCollectionID, targetCollectionID idwrap.IDWrap, cis *scollectionitem.CollectionItemService, ifs sitemfolder.ItemFolderService, ias sitemapi.ItemApiService) (sourceItemID, targetItemID idwrap.IDWrap) {
				// Create any items for the test (won't be used due to validation failure)
				sourceFolderID := idwrap.NewNow()
				targetFolderID := idwrap.NewNow()
				return sourceFolderID, targetFolderID
			},
		},
		{
			name:        "Valid: Unspecified Target Kind (Should Skip Validation)",
			sourceKind:  itemv1.ItemKind_ITEM_KIND_ENDPOINT,
			targetKind:  itemv1.ItemKind_ITEM_KIND_UNSPECIFIED,
			expectError: false,
			description: "Unspecified target kind should skip validation and succeed",
			setupFunc: func(t *testing.T, ctx context.Context, sourceCollectionID, targetCollectionID idwrap.IDWrap, cis *scollectionitem.CollectionItemService, ifs sitemfolder.ItemFolderService, ias sitemapi.ItemApiService) (sourceItemID, targetItemID idwrap.IDWrap) {
				// Create source endpoint
				sourceEndpointID := idwrap.NewNow()
				sourceEndpoint := &mitemapi.ItemApi{
					ID:           sourceEndpointID,
					Name:         "GET /health",
					Url:          "https://api.myapp.com/health",
					Method:       "GET",
					CollectionID: sourceCollectionID,
					FolderID:     nil,
				}
				err := ias.CreateItemApi(ctx, sourceEndpoint)
				require.NoError(t, err)

				// Create target folder (targetKind unspecified, so validation should be skipped)
				targetFolderID := idwrap.NewNow()
				targetFolder := &mitemfolder.ItemFolder{
					ID:           targetFolderID,
					Name:         "Health Checks",
					CollectionID: targetCollectionID,
					ParentID:     nil,
				}
				err = ifs.CreateItemFolder(ctx, targetFolder)
				require.NoError(t, err)

				// Create collection items
				tx, err := base.DB.Begin()
				require.NoError(t, err)
				defer tx.Rollback()

				err = cis.CreateEndpointTX(ctx, tx, sourceEndpoint)
				require.NoError(t, err)
				err = cis.CreateFolderTX(ctx, tx, targetFolder)
				require.NoError(t, err)
				err = tx.Commit()
				require.NoError(t, err)

				return sourceEndpointID, targetFolderID
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Testing targetKind validation: %s", tt.description)

			// Setup test data
			sourceItemID, targetItemID := tt.setupFunc(t, ctx, sourceCollectionID, targetCollectionID, cis, ifs, ias)

			// Convert legacy IDs to collection item IDs if needed
			var targetCollectionItemID idwrap.IDWrap
			var err error

			if !tt.expectError || tt.expectedCode != connect.CodeInvalidArgument || tt.expectedMessagePart != "source kind must be specified" {
				// Only try to get collection item ID if we expect the request to get past initial validation
				targetCollectionItemID, err = cis.GetCollectionItemIDByLegacyID(ctx, targetItemID)
				if err != nil && !tt.expectError {
					require.NoError(t, err, "Failed to get target collection item ID")
				}
			}

			// Create request
			request := &itemv1.CollectionItemMoveRequest{
				Kind:               tt.sourceKind,
				ItemId:             sourceItemID.Bytes(),
				CollectionId:       sourceCollectionID.Bytes(),
				TargetCollectionId: targetCollectionID.Bytes(),
				TargetKind:         tt.targetKind.Enum(),
				Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
			}

			// Add target item ID if not testing source kind validation
			if tt.expectedMessagePart != "source kind must be specified" {
				request.TargetItemId = targetCollectionItemID.Bytes()
			}

			// Execute move
			resp, err := rpc.CollectionItemMove(authedCtx, connect.NewRequest(request))

			// Validate response
			if tt.expectError {
				assert.Error(t, err, "Expected error for: %s", tt.description)
				if connectErr := new(connect.Error); assert.ErrorAs(t, err, &connectErr) {
					assert.Equal(t, tt.expectedCode, connectErr.Code(),
						"Expected error code %v but got %v for: %s", tt.expectedCode, connectErr.Code(), tt.description)

					if tt.expectedMessagePart != "" {
						assert.Contains(t, connectErr.Message(), tt.expectedMessagePart,
							"Error message should contain '%s' but got: %s", tt.expectedMessagePart, connectErr.Message())
					}
				}
			} else {
				assert.NoError(t, err, "Unexpected error for: %s", tt.description)
				assert.NotNil(t, resp, "Response should not be nil for successful operations")
			}
		})
	}
}

// TestCrossCollectionTargetKindSemantics tests semantic meaning of targetKind combinations
func TestCrossCollectionTargetKindSemantics(t *testing.T) {
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

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	t.Run("Semantic Analysis: Folder into Folder Hierarchy", func(t *testing.T) {
		// This tests the semantic meaning of folder-to-folder moves
		// When targetKind=FOLDER, we're indicating this is folder reorganization

		// Create parent folder in target collection
		parentFolderID := idwrap.NewNow()
		parentFolder := &mitemfolder.ItemFolder{
			ID:           parentFolderID,
			Name:         "API Categories",
			CollectionID: targetCollectionID,
			ParentID:     nil,
		}
		err := ifs.CreateItemFolder(ctx, parentFolder)
		require.NoError(t, err)

		// Create child folder in source collection
		childFolderID := idwrap.NewNow()
		childFolder := &mitemfolder.ItemFolder{
			ID:           childFolderID,
			Name:         "Authentication APIs",
			CollectionID: sourceCollectionID,
			ParentID:     nil,
		}
		err = ifs.CreateItemFolder(ctx, childFolder)
		require.NoError(t, err)

		// Create collection items
		tx, err := base.DB.Begin()
		require.NoError(t, err)
		defer tx.Rollback()

		err = cis.CreateFolderTX(ctx, tx, parentFolder)
		require.NoError(t, err)
		err = cis.CreateFolderTX(ctx, tx, childFolder)
		require.NoError(t, err)
		err = tx.Commit()
		require.NoError(t, err)

		// Get collection item IDs
		parentCollectionItemID, err := cis.GetCollectionItemIDByLegacyID(ctx, parentFolderID)
		require.NoError(t, err)

		// Move child folder to be positioned relative to parent folder
		req := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:               itemv1.ItemKind_ITEM_KIND_FOLDER,
			ItemId:             childFolderID.Bytes(),
			CollectionId:       sourceCollectionID.Bytes(),
			TargetCollectionId: targetCollectionID.Bytes(),
			TargetItemId:       parentCollectionItemID.Bytes(),
			TargetKind:         itemv1.ItemKind_ITEM_KIND_FOLDER.Enum(),
			Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
		})

		resp, err := rpc.CollectionItemMove(authedCtx, req)
		assert.NoError(t, err, "Folder-to-folder semantic move should succeed")
		assert.NotNil(t, resp, "Response should not be nil")

		// Validate semantic result
		items, err := cis.ListCollectionItems(ctx, targetCollectionID, nil)
		require.NoError(t, err)
		require.Len(t, items, 2, "Target collection should have 2 folders")

		// Both items should be folders (semantic validation)
		for _, item := range items {
			assert.NotNil(t, item.FolderID, "All items should be folders in this semantic context")
			assert.Nil(t, item.EndpointID, "No endpoints should be present in folder reorganization")
		}
	})

	t.Run("Semantic Analysis: Endpoint into Folder Context", func(t *testing.T) {
		// This tests the semantic meaning of endpoint-to-folder moves
		// When targetKind=FOLDER, we're indicating the endpoint should be organized into folder structure

		// Create organization folder in target collection
		orgFolderID := idwrap.NewNow()
		orgFolder := &mitemfolder.ItemFolder{
			ID:           orgFolderID,
			Name:         "User Operations",
			CollectionID: targetCollectionID,
			ParentID:     nil,
		}
		err := ifs.CreateItemFolder(ctx, orgFolder)
		require.NoError(t, err)

		// Create endpoint in source collection
		endpointID := idwrap.NewNow()
		endpoint := &mitemapi.ItemApi{
			ID:           endpointID,
			Name:         "PUT /users/{id}/profile",
			Url:          "https://api.myapp.com/users/{id}/profile",
			Method:       "PUT",
			CollectionID: sourceCollectionID,
			FolderID:     nil,
		}
		err = ias.CreateItemApi(ctx, endpoint)
		require.NoError(t, err)

		// Create collection items
		tx, err := base.DB.Begin()
		require.NoError(t, err)
		defer tx.Rollback()

		err = cis.CreateFolderTX(ctx, tx, orgFolder)
		require.NoError(t, err)
		err = cis.CreateEndpointTX(ctx, tx, endpoint)
		require.NoError(t, err)
		err = tx.Commit()
		require.NoError(t, err)

		// Get collection item IDs
		orgFolderCollectionItemID, err := cis.GetCollectionItemIDByLegacyID(ctx, orgFolderID)
		require.NoError(t, err)

		// Move endpoint into folder context using targetParentFolderId and targetKind validation
		req := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:                 itemv1.ItemKind_ITEM_KIND_ENDPOINT,
			ItemId:               endpointID.Bytes(),
			CollectionId:         sourceCollectionID.Bytes(),
			TargetCollectionId:   targetCollectionID.Bytes(),
			TargetParentFolderId: orgFolderCollectionItemID.Bytes(),
			TargetKind:           itemv1.ItemKind_ITEM_KIND_FOLDER.Enum(),
			Position:             resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
		})

		resp, err := rpc.CollectionItemMove(authedCtx, req)
		assert.NoError(t, err, "Endpoint-to-folder semantic move should succeed")
		assert.NotNil(t, resp, "Response should not be nil")

		// Validate semantic result - endpoint should be nested in folder
		nestedItems, err := cis.ListCollectionItems(ctx, targetCollectionID, &orgFolderCollectionItemID)
		require.NoError(t, err)
		require.Len(t, nestedItems, 1, "Folder should contain 1 nested endpoint")

		nestedItem := nestedItems[0]
		assert.NotNil(t, nestedItem.EndpointID, "Nested item should be an endpoint")
		assert.Equal(t, targetCollectionID, nestedItem.CollectionID, "Endpoint should be in target collection")
		assert.Equal(t, orgFolderCollectionItemID, *nestedItem.ParentFolderID, "Endpoint should be in organization folder")
	})

	t.Run("Semantic Analysis: Endpoint Ordering Context", func(t *testing.T) {
		// This tests endpoint-to-endpoint semantic moves for ordering
		// When targetKind=ENDPOINT, we're indicating this is for ordering endpoints relative to each other

		// Create multiple endpoints in target collection for ordering
		endpoint1ID := idwrap.NewNow()
		endpoint1 := &mitemapi.ItemApi{
			ID:           endpoint1ID,
			Name:         "GET /orders",
			Url:          "https://api.myapp.com/orders",
			Method:       "GET",
			CollectionID: targetCollectionID,
			FolderID:     nil,
		}
		err := ias.CreateItemApi(ctx, endpoint1)
		require.NoError(t, err)

		endpoint2ID := idwrap.NewNow()
		endpoint2 := &mitemapi.ItemApi{
			ID:           endpoint2ID,
			Name:         "POST /orders",
			Url:          "https://api.myapp.com/orders",
			Method:       "POST",
			CollectionID: targetCollectionID,
			FolderID:     nil,
		}
		err = ias.CreateItemApi(ctx, endpoint2)
		require.NoError(t, err)

		// Create endpoint to move from source
		sourceEndpointID := idwrap.NewNow()
		sourceEndpoint := &mitemapi.ItemApi{
			ID:           sourceEndpointID,
			Name:         "PUT /orders/{id}",
			Url:          "https://api.myapp.com/orders/{id}",
			Method:       "PUT",
			CollectionID: sourceCollectionID,
			FolderID:     nil,
		}
		err = ias.CreateItemApi(ctx, sourceEndpoint)
		require.NoError(t, err)

		// Create collection items
		tx, err := base.DB.Begin()
		require.NoError(t, err)
		defer tx.Rollback()

		err = cis.CreateEndpointTX(ctx, tx, endpoint1)
		require.NoError(t, err)
		err = cis.CreateEndpointTX(ctx, tx, endpoint2)
		require.NoError(t, err)
		err = cis.CreateEndpointTX(ctx, tx, sourceEndpoint)
		require.NoError(t, err)
		err = tx.Commit()
		require.NoError(t, err)

		// Get collection item IDs
		endpoint1CollectionItemID, err := cis.GetCollectionItemIDByLegacyID(ctx, endpoint1ID)
		require.NoError(t, err)

		// Move source endpoint to be ordered between existing endpoints
		req := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:               itemv1.ItemKind_ITEM_KIND_ENDPOINT,
			ItemId:             sourceEndpointID.Bytes(),
			CollectionId:       sourceCollectionID.Bytes(),
			TargetCollectionId: targetCollectionID.Bytes(),
			TargetItemId:       endpoint1CollectionItemID.Bytes(),
			TargetKind:         itemv1.ItemKind_ITEM_KIND_ENDPOINT.Enum(),
			Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
		})

		resp, err := rpc.CollectionItemMove(authedCtx, req)
		assert.NoError(t, err, "Endpoint-to-endpoint semantic ordering should succeed")
		assert.NotNil(t, resp, "Response should not be nil")

		// Validate semantic result - endpoints should be properly ordered
		items, err := cis.ListCollectionItems(ctx, targetCollectionID, nil)
		require.NoError(t, err)
		require.Len(t, items, 3, "Target collection should have 3 endpoints")

		// All items should be endpoints in this ordering context
		endpointCount := 0
		for _, item := range items {
			if item.EndpointID != nil {
				endpointCount++
				assert.Nil(t, item.FolderID, "In endpoint ordering context, should not have folders mixed in")
			}
		}
		assert.Equal(t, 3, endpointCount, "All items should be endpoints in ordering context")
	})
}

// TestCrossCollectionTargetKindMismatchScenarios tests scenarios where targetKind doesn't match actual target item type
func TestCrossCollectionTargetKindMismatchScenarios(t *testing.T) {
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

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	t.Run("TargetKind Mismatch: Claiming folder is endpoint", func(t *testing.T) {
		// Create source endpoint
		sourceEndpointID := idwrap.NewNow()
		sourceEndpoint := &mitemapi.ItemApi{
			ID:           sourceEndpointID,
			Name:         "GET /mismatch-test",
			Url:          "https://api.myapp.com/mismatch-test",
			Method:       "GET",
			CollectionID: sourceCollectionID,
			FolderID:     nil,
		}
		err := ias.CreateItemApi(ctx, sourceEndpoint)
		require.NoError(t, err)

		// Create target folder (but we'll claim it's an endpoint)
		targetFolderID := idwrap.NewNow()
		targetFolder := &mitemfolder.ItemFolder{
			ID:           targetFolderID,
			Name:         "Actually a Folder",
			CollectionID: targetCollectionID,
			ParentID:     nil,
		}
		err = ifs.CreateItemFolder(ctx, targetFolder)
		require.NoError(t, err)

		// Create collection items
		tx, err := base.DB.Begin()
		require.NoError(t, err)
		defer tx.Rollback()

		err = cis.CreateEndpointTX(ctx, tx, sourceEndpoint)
		require.NoError(t, err)
		err = cis.CreateFolderTX(ctx, tx, targetFolder)
		require.NoError(t, err)
		err = tx.Commit()
		require.NoError(t, err)

		// Get collection item IDs
		targetFolderCollectionItemID, err := cis.GetCollectionItemIDByLegacyID(ctx, targetFolderID)
		require.NoError(t, err)

		// Try to move endpoint claiming target folder is an endpoint
		req := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:               itemv1.ItemKind_ITEM_KIND_ENDPOINT,
			ItemId:             sourceEndpointID.Bytes(),
			CollectionId:       sourceCollectionID.Bytes(),
			TargetCollectionId: targetCollectionID.Bytes(),
			TargetItemId:       targetFolderCollectionItemID.Bytes(),
			TargetKind:         itemv1.ItemKind_ITEM_KIND_ENDPOINT.Enum(), // Lie about what the target is
			Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
		})

		// This should still succeed because targetKind is for validation, not enforcement
		// The system should handle the actual target item type correctly regardless of targetKind claim
		resp, err := rpc.CollectionItemMove(authedCtx, req)
		assert.NoError(t, err, "Move should succeed despite targetKind mismatch - targetKind is for validation")
		assert.NotNil(t, resp, "Response should not be nil")

		// Verify move succeeded
		items, err := cis.ListCollectionItems(ctx, targetCollectionID, nil)
		require.NoError(t, err)

		found := false
		for _, item := range items {
			if item.EndpointID != nil && item.EndpointID.Compare(sourceEndpointID) == 0 {
				found = true
				assert.Equal(t, targetCollectionID, item.CollectionID, "Endpoint should be in target collection")
				break
			}
		}
		assert.True(t, found, "Endpoint should be moved successfully regardless of targetKind mismatch")
	})

	t.Run("TargetKind Advisory Nature", func(t *testing.T) {
		// This test confirms that targetKind is advisory/validation only
		// The actual operation should work based on real item types, not claimed types

		// Create source folder
		sourceFolderID := idwrap.NewNow()
		sourceFolder := &mitemfolder.ItemFolder{
			ID:           sourceFolderID,
			Name:         "Advisory Test Folder",
			CollectionID: sourceCollectionID,
			ParentID:     nil,
		}
		err := ifs.CreateItemFolder(ctx, sourceFolder)
		require.NoError(t, err)

		// Create target folder
		targetFolderID := idwrap.NewNow()
		targetFolder := &mitemfolder.ItemFolder{
			ID:           targetFolderID,
			Name:         "Target Advisory Folder",
			CollectionID: targetCollectionID,
			ParentID:     nil,
		}
		err = ifs.CreateItemFolder(ctx, targetFolder)
		require.NoError(t, err)

		// Create collection items
		tx, err := base.DB.Begin()
		require.NoError(t, err)
		defer tx.Rollback()

		err = cis.CreateFolderTX(ctx, tx, sourceFolder)
		require.NoError(t, err)
		err = cis.CreateFolderTX(ctx, tx, targetFolder)
		require.NoError(t, err)
		err = tx.Commit()
		require.NoError(t, err)

		// Get collection item IDs
		targetFolderCollectionItemID, err := cis.GetCollectionItemIDByLegacyID(ctx, targetFolderID)
		require.NoError(t, err)

		// Move with correct targetKind
		req := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:               itemv1.ItemKind_ITEM_KIND_FOLDER,
			ItemId:             sourceFolderID.Bytes(),
			CollectionId:       sourceCollectionID.Bytes(),
			TargetCollectionId: targetCollectionID.Bytes(),
			TargetItemId:       targetFolderCollectionItemID.Bytes(),
			TargetKind:         itemv1.ItemKind_ITEM_KIND_FOLDER.Enum(),
			Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
		})

		resp, err := rpc.CollectionItemMove(authedCtx, req)
		assert.NoError(t, err, "Move with correct targetKind should succeed")
		assert.NotNil(t, resp, "Response should not be nil")

		// Verify successful move
		items, err := cis.ListCollectionItems(ctx, targetCollectionID, nil)
		require.NoError(t, err)
		require.Len(t, items, 2, "Target collection should have 2 folders")

		foundSource := false
		foundTarget := false
		for _, item := range items {
			if item.FolderID != nil {
				if item.FolderID.Compare(sourceFolderID) == 0 {
					foundSource = true
				}
				if item.FolderID.Compare(targetFolderID) == 0 {
					foundTarget = true
				}
			}
		}
		assert.True(t, foundSource, "Source folder should be in target collection")
		assert.True(t, foundTarget, "Target folder should remain in target collection")
	})
}
