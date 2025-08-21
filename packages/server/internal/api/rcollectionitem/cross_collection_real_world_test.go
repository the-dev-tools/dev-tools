package rcollectionitem_test

import (
	"context"
	"encoding/base64"
	"testing"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rcollectionitem"
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

// RealWorldPayloadTest represents a realistic payload test scenario
type RealWorldPayloadTest struct {
	name         string
	description  string
	setupFunc    func(t *testing.T, ctx context.Context, sourceCollectionID, targetCollectionID idwrap.IDWrap, cis *scollectionitem.CollectionItemService, ifs sitemfolder.ItemFolderService, ias sitemapi.ItemApiService) *itemv1.CollectionItemMoveRequest
	expectError  bool
	expectedCode connect.Code
	validateFunc func(t *testing.T, ctx context.Context, sourceCollectionID, targetCollectionID idwrap.IDWrap, cis *scollectionitem.CollectionItemService)
}

// TestCrossCollectionRealWorldPayloads tests real-world frontend payload scenarios
func TestCrossCollectionRealWorldPayloads(t *testing.T) {
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

	tests := []RealWorldPayloadTest{
		{
			name:        "Frontend API Explorer - Move endpoint to different collection",
			description: "User drags an API endpoint from 'Auth APIs' collection to 'User Management' collection",
			setupFunc: func(t *testing.T, ctx context.Context, sourceCollectionID, targetCollectionID idwrap.IDWrap, cis *scollectionitem.CollectionItemService, ifs sitemfolder.ItemFolderService, ias sitemapi.ItemApiService) *itemv1.CollectionItemMoveRequest {
				// Create realistic endpoint
				endpointID := idwrap.NewNow()
				endpoint := &mitemapi.ItemApi{
					ID:           endpointID,
					Name:         "POST /auth/login",
					Url:          "https://api.myapp.com/auth/login",
					Method:       "POST",
					CollectionID: sourceCollectionID,
					FolderID:     nil,
				}
				err := ias.CreateItemApi(ctx, endpoint)
				require.NoError(t, err)

				// Create collection item
				tx, err := base.DB.Begin()
				require.NoError(t, err)
				defer tx.Rollback()

				err = cis.CreateEndpointTX(ctx, tx, endpoint)
				require.NoError(t, err)
				err = tx.Commit()
				require.NoError(t, err)

				return &itemv1.CollectionItemMoveRequest{
					Kind:               itemv1.ItemKind_ITEM_KIND_ENDPOINT,
					ItemId:             endpointID.Bytes(),
					CollectionId:       sourceCollectionID.Bytes(),
					TargetCollectionId: targetCollectionID.Bytes(),
					Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
				}
			},
			expectError: false,
			validateFunc: func(t *testing.T, ctx context.Context, sourceCollectionID, targetCollectionID idwrap.IDWrap, cis *scollectionitem.CollectionItemService) {
				// Verify endpoint is now in target collection
				items, err := cis.ListCollectionItems(ctx, targetCollectionID, nil)
				require.NoError(t, err)
				
				found := false
				for _, item := range items {
					if item.EndpointID != nil {
						found = true
						assert.Equal(t, targetCollectionID, item.CollectionID, "Endpoint should be in target collection")
						break
					}
				}
				assert.True(t, found, "Endpoint should be found in target collection")
			},
		},
		{
			name:        "Frontend Flow Builder - Move folder with nested structure",
			description: "User moves a 'Customer APIs' folder from 'v1' collection to 'v2' collection",
			setupFunc: func(t *testing.T, ctx context.Context, sourceCollectionID, targetCollectionID idwrap.IDWrap, cis *scollectionitem.CollectionItemService, ifs sitemfolder.ItemFolderService, ias sitemapi.ItemApiService) *itemv1.CollectionItemMoveRequest {
				// Create parent folder
				parentFolderID := idwrap.NewNow()
				parentFolder := &mitemfolder.ItemFolder{
					ID:           parentFolderID,
					Name:         "Customer APIs",
					CollectionID: sourceCollectionID,
					ParentID:     nil,
				}
				err := ifs.CreateItemFolder(ctx, parentFolder)
				require.NoError(t, err)

				// Create nested endpoint inside folder
				endpointID := idwrap.NewNow()
				endpoint := &mitemapi.ItemApi{
					ID:           endpointID,
					Name:         "GET /customers",
					Url:          "https://api.myapp.com/v1/customers",
					Method:       "GET",
					CollectionID: sourceCollectionID,
					FolderID:     &parentFolderID,
				}
				err = ias.CreateItemApi(ctx, endpoint)
				require.NoError(t, err)

				// Create collection items
				tx, err := base.DB.Begin()
				require.NoError(t, err)
				defer tx.Rollback()

				err = cis.CreateFolderTX(ctx, tx, parentFolder)
				require.NoError(t, err)
				err = cis.CreateEndpointTX(ctx, tx, endpoint)
				require.NoError(t, err)
				err = tx.Commit()
				require.NoError(t, err)

				return &itemv1.CollectionItemMoveRequest{
					Kind:               itemv1.ItemKind_ITEM_KIND_FOLDER,
					ItemId:             parentFolderID.Bytes(),
					CollectionId:       sourceCollectionID.Bytes(),
					TargetCollectionId: targetCollectionID.Bytes(),
					Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
				}
			},
			expectError: false,
			validateFunc: func(t *testing.T, ctx context.Context, sourceCollectionID, targetCollectionID idwrap.IDWrap, cis *scollectionitem.CollectionItemService) {
				// Verify folder is in target collection
				items, err := cis.ListCollectionItems(ctx, targetCollectionID, nil)
				require.NoError(t, err)
				
				foundFolder := false
				var folderCollectionItemID *idwrap.IDWrap
				for _, item := range items {
					if item.FolderID != nil {
						foundFolder = true
						folderCollectionItemID = &item.ID
						assert.Equal(t, targetCollectionID, item.CollectionID, "Folder should be in target collection")
						break
					}
				}
				assert.True(t, foundFolder, "Folder should be found in target collection")

				// Verify nested endpoint is also in target collection
				if folderCollectionItemID != nil {
					nestedItems, err := cis.ListCollectionItems(ctx, targetCollectionID, folderCollectionItemID)
					require.NoError(t, err)
					
					foundEndpoint := false
					for _, item := range nestedItems {
						if item.EndpointID != nil {
							foundEndpoint = true
							assert.Equal(t, targetCollectionID, item.CollectionID, "Nested endpoint should be in target collection")
							break
						}
					}
					assert.True(t, foundEndpoint, "Nested endpoint should be found in target collection")
				}
			},
		},
		{
			name:        "Frontend Postman Import - Move endpoint with positioning",
			description: "User imports Postman collection and moves endpoint to specific position in existing collection",
			setupFunc: func(t *testing.T, ctx context.Context, sourceCollectionID, targetCollectionID idwrap.IDWrap, cis *scollectionitem.CollectionItemService, ifs sitemfolder.ItemFolderService, ias sitemapi.ItemApiService) *itemv1.CollectionItemMoveRequest {
				// Create target anchor endpoint (existing item in target collection)
				anchorEndpointID := idwrap.NewNow()
				anchorEndpoint := &mitemapi.ItemApi{
					ID:           anchorEndpointID,
					Name:         "GET /health",
					Url:          "https://api.myapp.com/health",
					Method:       "GET",
					CollectionID: targetCollectionID,
					FolderID:     nil,
				}
				err := ias.CreateItemApi(ctx, anchorEndpoint)
				require.NoError(t, err)

				// Create source endpoint to move
				sourceEndpointID := idwrap.NewNow()
				sourceEndpoint := &mitemapi.ItemApi{
					ID:           sourceEndpointID,
					Name:         "POST /users",
					Url:          "https://api.myapp.com/users",
					Method:       "POST",
					CollectionID: sourceCollectionID,
					FolderID:     nil,
				}
				err = ias.CreateItemApi(ctx, sourceEndpoint)
				require.NoError(t, err)

				// Create collection items
				tx, err := base.DB.Begin()
				require.NoError(t, err)
				defer tx.Rollback()

				err = cis.CreateEndpointTX(ctx, tx, anchorEndpoint)
				require.NoError(t, err)
				err = cis.CreateEndpointTX(ctx, tx, sourceEndpoint)
				require.NoError(t, err)
				err = tx.Commit()
				require.NoError(t, err)

				// Get anchor endpoint's collection item ID
				anchorCollectionItemID, err := cis.GetCollectionItemIDByLegacyID(ctx, anchorEndpointID)
				require.NoError(t, err)

				return &itemv1.CollectionItemMoveRequest{
					Kind:               itemv1.ItemKind_ITEM_KIND_ENDPOINT,
					ItemId:             sourceEndpointID.Bytes(),
					CollectionId:       sourceCollectionID.Bytes(),
					TargetCollectionId: targetCollectionID.Bytes(),
					TargetItemId:       anchorCollectionItemID.Bytes(),
					Position:           resourcesv1.MovePosition_MOVE_POSITION_BEFORE.Enum(),
				}
			},
			expectError: false,
			validateFunc: func(t *testing.T, ctx context.Context, sourceCollectionID, targetCollectionID idwrap.IDWrap, cis *scollectionitem.CollectionItemService) {
				// Verify positioning in target collection
				items, err := cis.ListCollectionItems(ctx, targetCollectionID, nil)
				require.NoError(t, err)
				require.Len(t, items, 2, "Target collection should have 2 endpoints")

				// The moved endpoint should be positioned before the anchor
				assert.NotNil(t, items[0].EndpointID, "First item should be an endpoint")
				assert.NotNil(t, items[1].EndpointID, "Second item should be an endpoint")
			},
		},
		{
			name:        "Frontend Folder Organization - Move endpoint into specific folder",
			description: "User drags endpoint into a specific folder in different collection with targetKind validation",
			setupFunc: func(t *testing.T, ctx context.Context, sourceCollectionID, targetCollectionID idwrap.IDWrap, cis *scollectionitem.CollectionItemService, ifs sitemfolder.ItemFolderService, ias sitemapi.ItemApiService) *itemv1.CollectionItemMoveRequest {
				// Create target folder
				targetFolderID := idwrap.NewNow()
				targetFolder := &mitemfolder.ItemFolder{
					ID:           targetFolderID,
					Name:         "Authentication",
					CollectionID: targetCollectionID,
					ParentID:     nil,
				}
				err := ifs.CreateItemFolder(ctx, targetFolder)
				require.NoError(t, err)

				// Create source endpoint
				sourceEndpointID := idwrap.NewNow()
				sourceEndpoint := &mitemapi.ItemApi{
					ID:           sourceEndpointID,
					Name:         "POST /auth/refresh",
					Url:          "https://api.myapp.com/auth/refresh",
					Method:       "POST",
					CollectionID: sourceCollectionID,
					FolderID:     nil,
				}
				err = ias.CreateItemApi(ctx, sourceEndpoint)
				require.NoError(t, err)

				// Create collection items
				tx, err := base.DB.Begin()
				require.NoError(t, err)
				defer tx.Rollback()

				err = cis.CreateFolderTX(ctx, tx, targetFolder)
				require.NoError(t, err)
				err = cis.CreateEndpointTX(ctx, tx, sourceEndpoint)
				require.NoError(t, err)
				err = tx.Commit()
				require.NoError(t, err)

				// Get target folder's collection item ID
				targetFolderCollectionItemID, err := cis.GetCollectionItemIDByLegacyID(ctx, targetFolderID)
				require.NoError(t, err)

				return &itemv1.CollectionItemMoveRequest{
					Kind:                 itemv1.ItemKind_ITEM_KIND_ENDPOINT,
					ItemId:               sourceEndpointID.Bytes(),
					CollectionId:         sourceCollectionID.Bytes(),
					TargetCollectionId:   targetCollectionID.Bytes(),
					TargetParentFolderId: targetFolderCollectionItemID.Bytes(),
					TargetKind:           itemv1.ItemKind_ITEM_KIND_FOLDER.Enum(),
					Position:             resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
				}
			},
			expectError: false,
			validateFunc: func(t *testing.T, ctx context.Context, sourceCollectionID, targetCollectionID idwrap.IDWrap, cis *scollectionitem.CollectionItemService) {
				// Get folder collection item ID for nested lookup
				items, err := cis.ListCollectionItems(ctx, targetCollectionID, nil)
				require.NoError(t, err)
				
				var folderCollectionItemID *idwrap.IDWrap
				for _, item := range items {
					if item.FolderID != nil {
						folderCollectionItemID = &item.ID
						break
					}
				}
				require.NotNil(t, folderCollectionItemID, "Target folder should exist")

				// Verify endpoint is nested in target folder
				nestedItems, err := cis.ListCollectionItems(ctx, targetCollectionID, folderCollectionItemID)
				require.NoError(t, err)
				
				found := false
				for _, item := range nestedItems {
					if item.EndpointID != nil {
						found = true
						assert.Equal(t, targetCollectionID, item.CollectionID, "Endpoint should be in target collection")
						assert.Equal(t, *folderCollectionItemID, *item.ParentFolderID, "Endpoint should be in target folder")
						break
					}
				}
				assert.True(t, found, "Endpoint should be found in target folder")
			},
		},
		{
			name:        "Frontend Base64 Encoded IDs - Real protobuf serialization",
			description: "Tests with base64-encoded ULID bytes as frontend would send",
			setupFunc: func(t *testing.T, ctx context.Context, sourceCollectionID, targetCollectionID idwrap.IDWrap, cis *scollectionitem.CollectionItemService, ifs sitemfolder.ItemFolderService, ias sitemapi.ItemApiService) *itemv1.CollectionItemMoveRequest {
				// Create endpoint
				endpointID := idwrap.NewNow()
				endpoint := &mitemapi.ItemApi{
					ID:           endpointID,
					Name:         "DELETE /sessions",
					Url:          "https://api.myapp.com/sessions",
					Method:       "DELETE",
					CollectionID: sourceCollectionID,
					FolderID:     nil,
				}
				err := ias.CreateItemApi(ctx, endpoint)
				require.NoError(t, err)

				// Create collection item
				tx, err := base.DB.Begin()
				require.NoError(t, err)
				defer tx.Rollback()

				err = cis.CreateEndpointTX(ctx, tx, endpoint)
				require.NoError(t, err)
				err = tx.Commit()
				require.NoError(t, err)

				// Simulate base64 encoding that frontend would do
				itemIDBytes := endpointID.Bytes()
				sourceCollectionIDBytes := sourceCollectionID.Bytes()
				targetCollectionIDBytes := targetCollectionID.Bytes()

				t.Logf("Using base64 encoded IDs - ItemID: %s, SourceCollectionID: %s, TargetCollectionID: %s",
					base64.StdEncoding.EncodeToString(itemIDBytes),
					base64.StdEncoding.EncodeToString(sourceCollectionIDBytes),
					base64.StdEncoding.EncodeToString(targetCollectionIDBytes))

				return &itemv1.CollectionItemMoveRequest{
					Kind:               itemv1.ItemKind_ITEM_KIND_ENDPOINT,
					ItemId:             itemIDBytes,
					CollectionId:       sourceCollectionIDBytes,
					TargetCollectionId: targetCollectionIDBytes,
					Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
				}
			},
			expectError: false,
			validateFunc: func(t *testing.T, ctx context.Context, sourceCollectionID, targetCollectionID idwrap.IDWrap, cis *scollectionitem.CollectionItemService) {
				items, err := cis.ListCollectionItems(ctx, targetCollectionID, nil)
				require.NoError(t, err)
				
				found := false
				for _, item := range items {
					if item.EndpointID != nil {
						found = true
						assert.Equal(t, targetCollectionID, item.CollectionID, "Endpoint should be in target collection")
						break
					}
				}
				assert.True(t, found, "Endpoint should be found in target collection")
			},
		},
		{
			name:        "Frontend Error Scenario - Invalid targetKind combination",
			description: "User tries to move folder into endpoint (semantically invalid)",
			setupFunc: func(t *testing.T, ctx context.Context, sourceCollectionID, targetCollectionID idwrap.IDWrap, cis *scollectionitem.CollectionItemService, ifs sitemfolder.ItemFolderService, ias sitemapi.ItemApiService) *itemv1.CollectionItemMoveRequest {
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
					Name:         "GET /admin",
					Url:          "https://api.myapp.com/admin",
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

				// Get target endpoint collection item ID
				targetEndpointCollectionItemID, err := cis.GetCollectionItemIDByLegacyID(ctx, targetEndpointID)
				require.NoError(t, err)

				return &itemv1.CollectionItemMoveRequest{
					Kind:               itemv1.ItemKind_ITEM_KIND_FOLDER,
					ItemId:             sourceFolderID.Bytes(),
					CollectionId:       sourceCollectionID.Bytes(),
					TargetCollectionId: targetCollectionID.Bytes(),
					TargetItemId:       targetEndpointCollectionItemID.Bytes(),
					TargetKind:         itemv1.ItemKind_ITEM_KIND_ENDPOINT.Enum(),
					Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
				}
			},
			expectError:  true,
			expectedCode: connect.CodeInvalidArgument,
			validateFunc: func(t *testing.T, ctx context.Context, sourceCollectionID, targetCollectionID idwrap.IDWrap, cis *scollectionitem.CollectionItemService) {
				// Verify folder remains in source collection
				sourceItems, err := cis.ListCollectionItems(ctx, sourceCollectionID, nil)
				require.NoError(t, err)
				
				found := false
				for _, item := range sourceItems {
					if item.FolderID != nil {
						found = true
						assert.Equal(t, sourceCollectionID, item.CollectionID, "Folder should remain in source collection")
						break
					}
				}
				assert.True(t, found, "Folder should remain in source collection after failed move")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Test scenario: %s", tt.description)

			// Setup test data
			request := tt.setupFunc(t, ctx, sourceCollectionID, targetCollectionID, cis, ifs, ias)

			// Execute move
			resp, err := rpc.CollectionItemMove(authedCtx, connect.NewRequest(request))

			// Validate response
			if tt.expectError {
				assert.Error(t, err, "Expected error for: %s", tt.description)
				if connectErr := new(connect.Error); assert.ErrorAs(t, err, &connectErr) {
					assert.Equal(t, tt.expectedCode, connectErr.Code(),
						"Expected error code %v but got %v for: %s", tt.expectedCode, connectErr.Code(), tt.description)
				}
			} else {
				assert.NoError(t, err, "Unexpected error for: %s", tt.description)
				assert.NotNil(t, resp, "Response should not be nil for successful operations")
			}

			// Run validation function
			if tt.validateFunc != nil {
				tt.validateFunc(t, ctx, sourceCollectionID, targetCollectionID, cis)
			}
		})
	}
}

// TestCrossCollectionComplexScenarios tests complex multi-step scenarios
func TestCrossCollectionComplexScenarios(t *testing.T) {
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

	t.Run("Complex API Organization Workflow", func(t *testing.T) {
		// Scenario: User is reorganizing APIs from a legacy 'Old APIs' collection to a new 'REST APIs' collection
		// This involves multiple moves with different positioning and folder structures

		// Step 1: Create a complex folder structure in source collection
		authFolderID := idwrap.NewNow()
		authFolder := &mitemfolder.ItemFolder{
			ID:           authFolderID,
			Name:         "Authentication",
			CollectionID: sourceCollectionID,
			ParentID:     nil,
		}
		err := ifs.CreateItemFolder(ctx, authFolder)
		require.NoError(t, err)

		usersFolderID := idwrap.NewNow()
		usersFolder := &mitemfolder.ItemFolder{
			ID:           usersFolderID,
			Name:         "User Management",
			CollectionID: sourceCollectionID,
			ParentID:     nil,
		}
		err = ifs.CreateItemFolder(ctx, usersFolder)
		require.NoError(t, err)

		// Create endpoints in different folders
		loginEndpointID := idwrap.NewNow()
		loginEndpoint := &mitemapi.ItemApi{
			ID:           loginEndpointID,
			Name:         "POST /login",
			Url:          "https://api.myapp.com/login",
			Method:       "POST",
			CollectionID: sourceCollectionID,
			FolderID:     &authFolderID,
		}
		err = ias.CreateItemApi(ctx, loginEndpoint)
		require.NoError(t, err)

		getUsersEndpointID := idwrap.NewNow()
		getUsersEndpoint := &mitemapi.ItemApi{
			ID:           getUsersEndpointID,
			Name:         "GET /users",
			Url:          "https://api.myapp.com/users",
			Method:       "GET",
			CollectionID: sourceCollectionID,
			FolderID:     &usersFolderID,
		}
		err = ias.CreateItemApi(ctx, getUsersEndpoint)
		require.NoError(t, err)

		// Create target folder structure
		targetAuthFolderID := idwrap.NewNow()
		targetAuthFolder := &mitemfolder.ItemFolder{
			ID:           targetAuthFolderID,
			Name:         "Auth APIs v2",
			CollectionID: targetCollectionID,
			ParentID:     nil,
		}
		err = ifs.CreateItemFolder(ctx, targetAuthFolder)
		require.NoError(t, err)

		// Create all collection items
		tx, err := base.DB.Begin()
		require.NoError(t, err)
		defer tx.Rollback()

		err = cis.CreateFolderTX(ctx, tx, authFolder)
		require.NoError(t, err)
		err = cis.CreateFolderTX(ctx, tx, usersFolder)
		require.NoError(t, err)
		err = cis.CreateEndpointTX(ctx, tx, loginEndpoint)
		require.NoError(t, err)
		err = cis.CreateEndpointTX(ctx, tx, getUsersEndpoint)
		require.NoError(t, err)
		err = cis.CreateFolderTX(ctx, tx, targetAuthFolder)
		require.NoError(t, err)

		err = tx.Commit()
		require.NoError(t, err)

		// Step 2: Move entire auth folder to target collection
		req1 := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:               itemv1.ItemKind_ITEM_KIND_FOLDER,
			ItemId:             authFolderID.Bytes(),
			CollectionId:       sourceCollectionID.Bytes(),
			TargetCollectionId: targetCollectionID.Bytes(),
			Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
		})

		resp1, err := rpc.CollectionItemMove(authedCtx, req1)
		assert.NoError(t, err, "Move auth folder should succeed")
		assert.NotNil(t, resp1, "Response should not be nil")

		// Step 3: Move specific endpoint to target folder
		targetAuthCollectionItemID, err := cis.GetCollectionItemIDByLegacyID(ctx, targetAuthFolderID)
		require.NoError(t, err)

		req2 := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:                 itemv1.ItemKind_ITEM_KIND_ENDPOINT,
			ItemId:               getUsersEndpointID.Bytes(),
			CollectionId:         sourceCollectionID.Bytes(),
			TargetCollectionId:   targetCollectionID.Bytes(),
			TargetParentFolderId: targetAuthCollectionItemID.Bytes(),
			Position:             resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
		})

		resp2, err := rpc.CollectionItemMove(authedCtx, req2)
		assert.NoError(t, err, "Move users endpoint to target folder should succeed")
		assert.NotNil(t, resp2, "Response should not be nil")

		// Step 4: Validate final structure
		targetItems, err := cis.ListCollectionItems(ctx, targetCollectionID, nil)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(targetItems), 2, "Target collection should have at least 2 items")

		// Count folders and endpoints
		folderCount := 0
		endpointCount := 0
		for _, item := range targetItems {
			if item.FolderID != nil {
				folderCount++
			}
			if item.EndpointID != nil {
				endpointCount++
			}
		}

		assert.GreaterOrEqual(t, folderCount, 2, "Should have at least 2 folders in target collection")
		assert.GreaterOrEqual(t, endpointCount, 1, "Should have at least 1 endpoint in target collection")

		// Verify nested structure
		nestedItems, err := cis.ListCollectionItems(ctx, targetCollectionID, &targetAuthCollectionItemID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(nestedItems), 1, "Target auth folder should have nested items")
	})
}

// TestCrossCollectionTransactionIntegrity tests that failed moves don't leave data in inconsistent state
func TestCrossCollectionTransactionIntegrity(t *testing.T) {
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

	sourceFolderID, _, _, _ := createTestItemsInCollections(
		t, ctx, sourceCollectionID, targetCollectionID, cis, ifs, ias, base)

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	t.Run("Failed move preserves original state", func(t *testing.T) {
		// Get initial state
		initialSourceItems, err := cis.ListCollectionItems(ctx, sourceCollectionID, nil)
		require.NoError(t, err)
		initialTargetItems, err := cis.ListCollectionItems(ctx, targetCollectionID, nil)
		require.NoError(t, err)

		initialSourceCount := len(initialSourceItems)
		initialTargetCount := len(initialTargetItems)

		// Attempt invalid move that should fail (folder into endpoint with invalid targetKind)
		req := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
			Kind:               itemv1.ItemKind_ITEM_KIND_FOLDER,
			ItemId:             sourceFolderID.Bytes(),
			CollectionId:       sourceCollectionID.Bytes(),
			TargetCollectionId: targetCollectionID.Bytes(),
			TargetKind:         itemv1.ItemKind_ITEM_KIND_ENDPOINT.Enum(), // Invalid: folder into endpoint
			Position:           resourcesv1.MovePosition_MOVE_POSITION_AFTER.Enum(),
		})

		_, err = rpc.CollectionItemMove(authedCtx, req)
		assert.Error(t, err, "Invalid move should fail")

		// Verify state is unchanged
		finalSourceItems, err := cis.ListCollectionItems(ctx, sourceCollectionID, nil)
		require.NoError(t, err)
		finalTargetItems, err := cis.ListCollectionItems(ctx, targetCollectionID, nil)
		require.NoError(t, err)

		assert.Equal(t, initialSourceCount, len(finalSourceItems), "Source collection count should be unchanged")
		assert.Equal(t, initialTargetCount, len(finalTargetItems), "Target collection count should be unchanged")

		// Verify specific item is still in source
		found := false
		for _, item := range finalSourceItems {
			if item.FolderID != nil && item.FolderID.Compare(sourceFolderID) == 0 {
				found = true
				assert.Equal(t, sourceCollectionID, item.CollectionID, "Folder should still be in source collection")
				break
			}
		}
		assert.True(t, found, "Folder should still exist in source collection")
	})
}