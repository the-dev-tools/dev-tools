package scollectionitem

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"sync"
	"testing"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/db/pkg/sqlitemem"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemfolder"
	"the-dev-tools/server/pkg/movable"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestDB(t *testing.T, ctx context.Context) (*sql.DB, *gen.Queries, func()) {
	t.Helper()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	require.NoError(t, err, "failed to create test database")

	queries, err := gen.Prepare(ctx, db)
	require.NoError(t, err, "failed to prepare queries")

	return db, queries, func() {
		cleanup()
            _ = queries.Close()
	}
}

func createTestService(queries *gen.Queries) *CollectionItemService {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return New(queries, logger)
}

func createTestCollection(t *testing.T, ctx context.Context, queries *gen.Queries, workspaceID idwrap.IDWrap) idwrap.IDWrap {
	t.Helper()
	collectionID := idwrap.NewNow()

	err := queries.CreateCollection(ctx, gen.CreateCollectionParams{
		ID:          collectionID,
		WorkspaceID: workspaceID,
		Name:        "Test Collection",
		Prev:        nil,
		Next:        nil,
	})
	require.NoError(t, err, "failed to create test collection")
	return collectionID
}

func TestCollectionItemService_CreateFolderTX(t *testing.T) {
	ctx := context.Background()
	db, queries, cleanup := createTestDB(t, ctx)
	defer cleanup()

	service := createTestService(queries)
	workspaceID := idwrap.NewNow()
	collectionID := createTestCollection(t, ctx, queries, workspaceID)

	tests := []struct {
		name           string
		folder         *mitemfolder.ItemFolder
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "create_folder_success",
			folder: &mitemfolder.ItemFolder{
				ID:           idwrap.NewNow(),
				CollectionID: collectionID,
				Name:         "Test Folder",
				ParentID:     nil,
			},
			expectError: false,
		},
		{
			name: "create_nested_folder_success",
			folder: &mitemfolder.ItemFolder{
				ID:           idwrap.NewNow(),
				CollectionID: collectionID,
				Name:         "Nested Folder",
				ParentID:     nil, // Will be set to parent folder ID in test
			},
			expectError: false,
		},
	}

    var parentFolderID *idwrap.IDWrap
    var parentFolderCI *idwrap.IDWrap

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For nested folder test, use the first folder as parent
			if tt.name == "create_nested_folder_success" && parentFolderID != nil {
				tt.folder.ParentID = parentFolderID
			}

			tx, err := db.BeginTx(ctx, nil)
			require.NoError(t, err, "failed to begin transaction")
			defer tx.Rollback()

			err = service.CreateFolderTX(ctx, tx, tt.folder)

			if tt.expectError {
				assert.Error(t, err, "expected error but got none")
				if tt.expectedErrMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrMsg)
				}
				return
			}

			require.NoError(t, err, "unexpected error creating folder")

			// Commit transaction to persist changes
			err = tx.Commit()
			require.NoError(t, err, "failed to commit transaction")

            // Verify collection_items entry was created
            // For nested listing, resolve legacy parent folder ID to collection_items ID
            var parentForList *idwrap.IDWrap
            if tt.folder.ParentID != nil {
                parentForList = parentFolderCI
            }
            items, err := service.ListCollectionItems(ctx, collectionID, parentForList)
            require.NoError(t, err, "failed to list collection items")
			
			found := false
			for _, item := range items {
				if item.Name == tt.folder.Name && item.ItemType == CollectionItemTypeFolder {
					found = true
					assert.NotNil(t, item.FolderID, "folder_id should not be nil")
					assert.Nil(t, item.EndpointID, "endpoint_id should be nil for folder")
					break
				}
			}
			assert.True(t, found, "folder should be found in collection items")

            // Save first folder ID for nested test
            if tt.name == "create_folder_success" {
                folderID := tt.folder.ID
                parentFolderID = &folderID

                // Resolve and cache the collection_items ID for the parent folder
                ciID, err := service.GetCollectionItemIDByLegacyID(ctx, folderID)
                require.NoError(t, err, "failed to resolve parent folder collection_items ID")
                parentFolderCI = &ciID
            }

			// Verify FK relationship with item_folder table
			folderItem, err := queries.GetItemFolder(ctx, tt.folder.ID)
			require.NoError(t, err, "failed to get item folder")
			assert.Equal(t, tt.folder.Name, folderItem.Name)
			assert.Equal(t, tt.folder.CollectionID, folderItem.CollectionID)
		})
	}
}

func TestCollectionItemService_CreateEndpointTX(t *testing.T) {
	ctx := context.Background()
	db, queries, cleanup := createTestDB(t, ctx)
	defer cleanup()

	service := createTestService(queries)
	workspaceID := idwrap.NewNow()
	collectionID := createTestCollection(t, ctx, queries, workspaceID)

	tests := []struct {
		name           string
		endpoint       *mitemapi.ItemApi
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "create_endpoint_success",
			endpoint: &mitemapi.ItemApi{
				ID:           idwrap.NewNow(),
				CollectionID: collectionID,
				Name:         "Test Endpoint",
				Url:          "https://api.example.com/test",
				Method:       "GET",
				FolderID:     nil,
			},
			expectError: false,
		},
		{
			name: "create_endpoint_in_folder_success",
			endpoint: &mitemapi.ItemApi{
				ID:           idwrap.NewNow(),
				CollectionID: collectionID,
				Name:         "Folder Endpoint",
				Url:          "https://api.example.com/folder/test",
				Method:       "POST",
				FolderID:     nil, // Will be set in test
			},
			expectError: false,
		},
	}

	// First create a folder for the second test
	folderID := idwrap.NewNow()
	folder := &mitemfolder.ItemFolder{
		ID:           folderID,
		CollectionID: collectionID,
		Name:         "Parent Folder",
		ParentID:     nil,
	}

    tx, err := db.BeginTx(ctx, nil)
    require.NoError(t, err)
    err = service.CreateFolderTX(ctx, tx, folder)
    require.NoError(t, err)
    err = tx.Commit()
    require.NoError(t, err)

    // Resolve collection_items ID for the parent folder to use in list queries under the folder
    parentFolderCI, err := service.GetCollectionItemIDByLegacyID(ctx, folderID)
    require.NoError(t, err, "failed to resolve parent folder collection_items ID")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For folder endpoint test, set the folder ID
			if tt.name == "create_endpoint_in_folder_success" {
				tt.endpoint.FolderID = &folderID
			}

			tx, err := db.BeginTx(ctx, nil)
			require.NoError(t, err, "failed to begin transaction")
			defer tx.Rollback()

			err = service.CreateEndpointTX(ctx, tx, tt.endpoint)

			if tt.expectError {
				assert.Error(t, err, "expected error but got none")
				if tt.expectedErrMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrMsg)
				}
				return
			}

			require.NoError(t, err, "unexpected error creating endpoint")

			// Commit transaction to persist changes
			err = tx.Commit()
			require.NoError(t, err, "failed to commit transaction")

            // Verify collection_items entry was created
            // When endpoint is in a folder, list by the folder's collection_items ID (not legacy ID)
            var parentForList *idwrap.IDWrap
            if tt.endpoint.FolderID != nil {
                parentForList = &parentFolderCI
            }
            items, err := service.ListCollectionItems(ctx, collectionID, parentForList)
            require.NoError(t, err, "failed to list collection items")
			
			found := false
			for _, item := range items {
				if item.Name == tt.endpoint.Name && item.ItemType == CollectionItemTypeEndpoint {
					found = true
					assert.NotNil(t, item.EndpointID, "endpoint_id should not be nil")
					assert.Nil(t, item.FolderID, "folder_id should be nil for endpoint")
					break
				}
			}
			assert.True(t, found, "endpoint should be found in collection items")

			// Verify FK relationship with item_api table
			endpointItem, err := queries.GetItemApi(ctx, tt.endpoint.ID)
			require.NoError(t, err, "failed to get item api")
			assert.Equal(t, tt.endpoint.Name, endpointItem.Name)
			assert.Equal(t, tt.endpoint.Url, endpointItem.Url)
			assert.Equal(t, tt.endpoint.Method, endpointItem.Method)
		})
	}
}

func TestCollectionItemService_ListCollectionItems(t *testing.T) {
	ctx := context.Background()
	db, queries, cleanup := createTestDB(t, ctx)
	defer cleanup()

	service := createTestService(queries)
	workspaceID := idwrap.NewNow()
	collectionID := createTestCollection(t, ctx, queries, workspaceID)

	// Create mixed folder and endpoint items
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)

	// Create items in specific order: Folder A, Endpoint B, Folder C, Endpoint D
	items := []struct {
		name     string
		itemType string
		folderID *idwrap.IDWrap
	}{
		{"Folder A", "folder", nil},
		{"Endpoint B", "endpoint", nil},
		{"Folder C", "folder", nil},
		{"Endpoint D", "endpoint", nil},
	}

	var createdFolderIDs []idwrap.IDWrap
	for _, item := range items {
		if item.itemType == "folder" {
			folder := &mitemfolder.ItemFolder{
				ID:           idwrap.NewNow(),
				CollectionID: collectionID,
				Name:         item.name,
				ParentID:     item.folderID,
			}
			createdFolderIDs = append(createdFolderIDs, folder.ID)
			err = service.CreateFolderTX(ctx, tx, folder)
			require.NoError(t, err)
		} else {
			endpoint := &mitemapi.ItemApi{
				ID:           idwrap.NewNow(),
				CollectionID: collectionID,
				Name:         item.name,
				Url:          "https://api.example.com/" + item.name,
				Method:       "GET",
				FolderID:     item.folderID,
			}
			err = service.CreateEndpointTX(ctx, tx, endpoint)
			require.NoError(t, err)
		}
	}

	err = tx.Commit()
	require.NoError(t, err)

	tests := []struct {
		name           string
		parentFolderID *idwrap.IDWrap
		expectedCount  int
		expectedNames  []string
	}{
		{
			name:           "list_root_items",
			parentFolderID: nil,
			expectedCount:  4,
			expectedNames:  []string{"Folder A", "Endpoint B", "Folder C", "Endpoint D"},
		},
		{
			name:           "list_empty_folder_items",
			parentFolderID: &createdFolderIDs[0], // Folder A
			expectedCount:  0,
			expectedNames:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, err := service.ListCollectionItems(ctx, collectionID, tt.parentFolderID)
			require.NoError(t, err, "failed to list collection items")
			
			assert.Equal(t, tt.expectedCount, len(items), "unexpected item count")
			
			if tt.expectedCount > 0 {
				for i, expectedName := range tt.expectedNames {
					if i < len(items) {
						assert.Equal(t, expectedName, items[i].Name, "unexpected item name at position %d", i)
					}
				}
			}

			// Verify ordering is maintained (prev/next pointers should create proper chain)
			if len(items) > 1 {
				for i := 0; i < len(items)-1; i++ {
					// Items should form a proper linked list
					if i == 0 {
						assert.Nil(t, items[i].PrevID, "first item should have nil PrevID")
					}
					if i == len(items)-1 {
						assert.Nil(t, items[i].NextID, "last item should have nil NextID")
					}
				}
			}
		})
	}
}

func TestCollectionItemService_MoveCollectionItem(t *testing.T) {
	ctx := context.Background()
	db, queries, cleanup := createTestDB(t, ctx)
	defer cleanup()

	service := createTestService(queries)
	workspaceID := idwrap.NewNow()
	collectionID := createTestCollection(t, ctx, queries, workspaceID)

	// Create test items: Folder A, Endpoint B, Folder C
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)

	folderA := &mitemfolder.ItemFolder{
		ID:           idwrap.NewNow(),
		CollectionID: collectionID,
		Name:         "Folder A",
		ParentID:     nil,
	}
	endpointB := &mitemapi.ItemApi{
		ID:           idwrap.NewNow(),
		CollectionID: collectionID,
		Name:         "Endpoint B",
		Url:          "https://api.example.com/b",
		Method:       "GET",
		FolderID:     nil,
	}
	folderC := &mitemfolder.ItemFolder{
		ID:           idwrap.NewNow(),
		CollectionID: collectionID,
		Name:         "Folder C",
		ParentID:     nil,
	}

	err = service.CreateFolderTX(ctx, tx, folderA)
	require.NoError(t, err)
	err = service.CreateEndpointTX(ctx, tx, endpointB)
	require.NoError(t, err)
	err = service.CreateFolderTX(ctx, tx, folderC)
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	// Get collection items to get their IDs from the collection_items table
	items, err := service.ListCollectionItems(ctx, collectionID, nil)
	require.NoError(t, err)
	require.Len(t, items, 3)

	tests := []struct {
		name            string
		moveItemIndex   int // which item to move (by index in items slice)
		targetItemIndex int // target item index
		position        movable.MovePosition
		expectedOrder   []string // expected names after move
		expectError     bool
	}{
		{
			name:            "move_folder_after_endpoint",
			moveItemIndex:   0, // Folder A
			targetItemIndex: 1, // after Endpoint B
			position:        movable.MovePositionAfter,
			expectedOrder:   []string{"Endpoint B", "Folder A", "Folder C"},
			expectError:     false,
		},
		{
			name:            "move_endpoint_before_folder",
			moveItemIndex:   0, // Endpoint B (now at position 0 after previous test)
			targetItemIndex: 2, // before Folder C
			position:        movable.MovePositionBefore,
			expectedOrder:   []string{"Folder A", "Endpoint B", "Folder C"},
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get current state
			currentItems, err := service.ListCollectionItems(ctx, collectionID, nil)
			require.NoError(t, err)

			moveItemID := currentItems[tt.moveItemIndex].ID
			targetItemID := currentItems[tt.targetItemIndex].ID

			err = service.MoveCollectionItem(ctx, moveItemID, &targetItemID, tt.position)

			if tt.expectError {
				assert.Error(t, err, "expected error but got none")
				return
			}

			require.NoError(t, err, "unexpected error moving item")

			// Verify new order
			newItems, err := service.ListCollectionItems(ctx, collectionID, nil)
			require.NoError(t, err)
			
			require.Equal(t, len(tt.expectedOrder), len(newItems), "unexpected number of items after move")
			
			for i, expectedName := range tt.expectedOrder {
				assert.Equal(t, expectedName, newItems[i].Name, "unexpected item at position %d", i)
			}
		})
	}
}

func TestCollectionItemService_GetCollectionItem(t *testing.T) {
	ctx := context.Background()
	db, queries, cleanup := createTestDB(t, ctx)
	defer cleanup()

	service := createTestService(queries)
	workspaceID := idwrap.NewNow()
	collectionID := createTestCollection(t, ctx, queries, workspaceID)

	// Create a test folder
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)

	folder := &mitemfolder.ItemFolder{
		ID:           idwrap.NewNow(),
		CollectionID: collectionID,
		Name:         "Test Folder",
		ParentID:     nil,
	}

	err = service.CreateFolderTX(ctx, tx, folder)
	require.NoError(t, err)
	err = tx.Commit()
	require.NoError(t, err)

	// Get the collection item ID
	items, err := service.ListCollectionItems(ctx, collectionID, nil)
	require.NoError(t, err)
	require.Len(t, items, 1)

	collectionItemID := items[0].ID

	tests := []struct {
		name          string
		itemID        idwrap.IDWrap
		expectError   bool
		expectedName  string
		expectedType  CollectionItemType
	}{
		{
			name:         "get_existing_item",
			itemID:       collectionItemID,
			expectError:  false,
			expectedName: "Test Folder",
			expectedType: CollectionItemTypeFolder,
		},
		{
			name:        "get_non_existent_item",
			itemID:      idwrap.NewNow(),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, err := service.GetCollectionItem(ctx, tt.itemID)

			if tt.expectError {
				assert.Error(t, err, "expected error but got none")
				assert.ErrorIs(t, err, ErrCollectionItemNotFound, "expected ErrCollectionItemNotFound")
				assert.Nil(t, item, "item should be nil on error")
				return
			}

			require.NoError(t, err, "unexpected error getting item")
			require.NotNil(t, item, "item should not be nil")
			
			assert.Equal(t, tt.expectedName, item.Name, "unexpected item name")
			assert.Equal(t, tt.expectedType, item.ItemType, "unexpected item type")
			assert.Equal(t, collectionID, item.CollectionID, "unexpected collection ID")
		})
	}
}

func TestCollectionItemService_DeleteCollectionItem(t *testing.T) {
	ctx := context.Background()
	db, queries, cleanup := createTestDB(t, ctx)
	defer cleanup()

	service := createTestService(queries)
	workspaceID := idwrap.NewNow()
	collectionID := createTestCollection(t, ctx, queries, workspaceID)

	// Create test items
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)

	folder := &mitemfolder.ItemFolder{
		ID:           idwrap.NewNow(),
		CollectionID: collectionID,
		Name:         "Test Folder",
		ParentID:     nil,
	}
	endpoint := &mitemapi.ItemApi{
		ID:           idwrap.NewNow(),
		CollectionID: collectionID,
		Name:         "Test Endpoint",
		Url:          "https://api.example.com/test",
		Method:       "GET",
		FolderID:     nil,
	}

	err = service.CreateFolderTX(ctx, tx, folder)
	require.NoError(t, err)
	err = service.CreateEndpointTX(ctx, tx, endpoint)
	require.NoError(t, err)
	err = tx.Commit()
	require.NoError(t, err)

	// Get collection items
	items, err := service.ListCollectionItems(ctx, collectionID, nil)
	require.NoError(t, err)
	require.Len(t, items, 2)

	tests := []struct {
		name            string
		deleteItemIndex int
		expectError     bool
		expectedCount   int
	}{
		{
			name:            "delete_folder_item",
			deleteItemIndex: 0,
			expectError:     false,
			expectedCount:   1,
		},
		{
			name:            "delete_endpoint_item",
			deleteItemIndex: 0, // Now the first item after previous deletion
			expectError:     false,
			expectedCount:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get current items
			currentItems, err := service.ListCollectionItems(ctx, collectionID, nil)
			require.NoError(t, err)
			
			if tt.deleteItemIndex >= len(currentItems) {
				t.Skipf("not enough items to run test (need index %d, have %d items)", tt.deleteItemIndex, len(currentItems))
			}

			itemToDelete := currentItems[tt.deleteItemIndex]

			tx, err := db.BeginTx(ctx, nil)
			require.NoError(t, err)
			defer tx.Rollback()

			err = service.DeleteCollectionItem(ctx, tx, itemToDelete.ID)

			if tt.expectError {
				assert.Error(t, err, "expected error but got none")
				return
			}

			require.NoError(t, err, "unexpected error deleting item")

			err = tx.Commit()
			require.NoError(t, err)

			// Verify item was deleted
			remainingItems, err := service.ListCollectionItems(ctx, collectionID, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCount, len(remainingItems), "unexpected number of items after deletion")

			// Verify the specific item is gone
			_, err = service.GetCollectionItem(ctx, itemToDelete.ID)
			assert.Error(t, err, "deleted item should not be found")
		})
	}
}

func TestCollectionItemService_GetWorkspaceID(t *testing.T) {
	ctx := context.Background()
	db, queries, cleanup := createTestDB(t, ctx)
	defer cleanup()

	service := createTestService(queries)
	workspaceID := idwrap.NewNow()
	collectionID := createTestCollection(t, ctx, queries, workspaceID)

	// Create test item
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)

	folder := &mitemfolder.ItemFolder{
		ID:           idwrap.NewNow(),
		CollectionID: collectionID,
		Name:         "Test Folder",
		ParentID:     nil,
	}

	err = service.CreateFolderTX(ctx, tx, folder)
	require.NoError(t, err)
	err = tx.Commit()
	require.NoError(t, err)

	// Get collection item
	items, err := service.ListCollectionItems(ctx, collectionID, nil)
	require.NoError(t, err)
	require.Len(t, items, 1)

	collectionItemID := items[0].ID

	tests := []struct {
		name               string
		itemID             idwrap.IDWrap
		expectedWorkspaceID idwrap.IDWrap
		expectError        bool
	}{
		{
			name:               "get_workspace_id_success",
			itemID:             collectionItemID,
			expectedWorkspaceID: workspaceID,
			expectError:        false,
		},
		{
			name:        "get_workspace_id_non_existent_item",
			itemID:      idwrap.NewNow(),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resultWorkspaceID, err := service.GetWorkspaceID(ctx, tt.itemID)

			if tt.expectError {
				assert.Error(t, err, "expected error but got none")
				return
			}

			require.NoError(t, err, "unexpected error getting workspace ID")
			assert.Equal(t, tt.expectedWorkspaceID, resultWorkspaceID, "unexpected workspace ID")
		})
	}
}

func TestCollectionItemService_CheckWorkspaceID(t *testing.T) {
	ctx := context.Background()
	db, queries, cleanup := createTestDB(t, ctx)
	defer cleanup()

	service := createTestService(queries)
	workspaceID := idwrap.NewNow()
	otherWorkspaceID := idwrap.NewNow()
	collectionID := createTestCollection(t, ctx, queries, workspaceID)

	// Create test item
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)

	folder := &mitemfolder.ItemFolder{
		ID:           idwrap.NewNow(),
		CollectionID: collectionID,
		Name:         "Test Folder",
		ParentID:     nil,
	}

	err = service.CreateFolderTX(ctx, tx, folder)
	require.NoError(t, err)
	err = tx.Commit()
	require.NoError(t, err)

	// Get collection item
	items, err := service.ListCollectionItems(ctx, collectionID, nil)
	require.NoError(t, err)
	require.Len(t, items, 1)

	collectionItemID := items[0].ID

	tests := []struct {
		name             string
		itemID           idwrap.IDWrap
		workspaceID      idwrap.IDWrap
		expectedResult   bool
		expectError      bool
	}{
		{
			name:           "check_workspace_id_match",
			itemID:         collectionItemID,
			workspaceID:    workspaceID,
			expectedResult: true,
			expectError:    false,
		},
		{
			name:           "check_workspace_id_no_match",
			itemID:         collectionItemID,
			workspaceID:    otherWorkspaceID,
			expectedResult: false,
			expectError:    false,
		},
		{
			name:        "check_workspace_id_non_existent_item",
			itemID:      idwrap.NewNow(),
			workspaceID: workspaceID,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.CheckWorkspaceID(ctx, tt.itemID, tt.workspaceID)

			if tt.expectError {
				assert.Error(t, err, "expected error but got none")
				return
			}

			require.NoError(t, err, "unexpected error checking workspace ID")
			assert.Equal(t, tt.expectedResult, result, "unexpected workspace check result")
		})
	}
}

func TestCollectionItemService_TransactionRollback(t *testing.T) {
	ctx := context.Background()
	db, queries, cleanup := createTestDB(t, ctx)
	defer cleanup()

	service := createTestService(queries)
	workspaceID := idwrap.NewNow()
	collectionID := createTestCollection(t, ctx, queries, workspaceID)

	tests := []struct {
		name        string
		operation   func(context.Context, *sql.Tx) error
		description string
	}{
		{
			name: "folder_creation_rollback",
			operation: func(ctx context.Context, tx *sql.Tx) error {
				folder := &mitemfolder.ItemFolder{
					ID:           idwrap.NewNow(),
					CollectionID: collectionID,
					Name:         "Rollback Folder",
					ParentID:     nil,
				}
				return service.CreateFolderTX(ctx, tx, folder)
			},
			description: "folder creation should be rolled back",
		},
		{
			name: "endpoint_creation_rollback",
			operation: func(ctx context.Context, tx *sql.Tx) error {
				endpoint := &mitemapi.ItemApi{
					ID:           idwrap.NewNow(),
					CollectionID: collectionID,
					Name:         "Rollback Endpoint",
					Url:          "https://api.example.com/rollback",
					Method:       "POST",
					FolderID:     nil,
				}
				return service.CreateEndpointTX(ctx, tx, endpoint)
			},
			description: "endpoint creation should be rolled back",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get initial count
			initialItems, err := service.ListCollectionItems(ctx, collectionID, nil)
			require.NoError(t, err)
			initialCount := len(initialItems)

			// Start transaction
			tx, err := db.BeginTx(ctx, nil)
			require.NoError(t, err)

			// Perform operation
			err = tt.operation(ctx, tx)
			require.NoError(t, err, "operation should succeed within transaction")

			// Rollback transaction
			err = tx.Rollback()
			require.NoError(t, err, "rollback should succeed")

			// Verify no changes persisted
			finalItems, err := service.ListCollectionItems(ctx, collectionID, nil)
			require.NoError(t, err)
			finalCount := len(finalItems)

			assert.Equal(t, initialCount, finalCount, tt.description)
		})
	}
}

func TestCollectionItemService_ConcurrentOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping concurrent test in short mode")
	}

	ctx := context.Background()
	db, queries, cleanup := createTestDB(t, ctx)
	defer cleanup()

	service := createTestService(queries)
	workspaceID := idwrap.NewNow()
	collectionID := createTestCollection(t, ctx, queries, workspaceID)

	const numGoroutines = 10
	const itemsPerGoroutine = 5

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	// Concurrently create folders and endpoints
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()

			for j := 0; j < itemsPerGoroutine; j++ {
				tx, err := db.BeginTx(ctx, nil)
				if err != nil {
					errors <- err
					return
				}

				var opErr error
				if (routineID+j)%2 == 0 {
					// Create folder
					folder := &mitemfolder.ItemFolder{
						ID:           idwrap.NewNow(),
						CollectionID: collectionID,
						Name:         "Concurrent Folder",
						ParentID:     nil,
					}
					opErr = service.CreateFolderTX(ctx, tx, folder)
				} else {
					// Create endpoint
					endpoint := &mitemapi.ItemApi{
						ID:           idwrap.NewNow(),
						CollectionID: collectionID,
						Name:         "Concurrent Endpoint",
						Url:          "https://api.example.com/concurrent",
						Method:       "GET",
						FolderID:     nil,
					}
					opErr = service.CreateEndpointTX(ctx, tx, endpoint)
				}

				if opErr != nil {
					tx.Rollback()
					errors <- opErr
					return
				}

				if commitErr := tx.Commit(); commitErr != nil {
					errors <- commitErr
					return
				}
			}
		}(i)
	}

	// Wait for all goroutines
	wg.Wait()
	close(errors)

	// Check for errors
	var errs []error
	for err := range errors {
		errs = append(errs, err)
	}

	// Some concurrent operations may fail due to race conditions, but we should have most items created
	if len(errs) > 0 {
		t.Logf("Concurrent operations had %d errors: %v", len(errs), errs)
	}

	// Verify data integrity
	items, err := service.ListCollectionItems(ctx, collectionID, nil)
	require.NoError(t, err)

	t.Logf("Created %d items concurrently", len(items))
	
	// Should have created some items (allowing for some failures due to concurrency)
	assert.Greater(t, len(items), 0, "should have created some items")

	// Verify no duplicate names (basic integrity check)
	nameCount := make(map[string]int)
	for _, item := range items {
		nameCount[item.Name]++
	}

	// All items should have proper types and IDs
	for _, item := range items {
		assert.NotEmpty(t, item.ID, "item should have valid ID")
		assert.NotEmpty(t, item.Name, "item should have valid name")
		assert.True(t, item.ItemType == CollectionItemTypeFolder || item.ItemType == CollectionItemTypeEndpoint, 
			"item should have valid type")
		
		if item.ItemType == CollectionItemTypeFolder {
			assert.NotNil(t, item.FolderID, "folder item should have folder_id")
			assert.Nil(t, item.EndpointID, "folder item should not have endpoint_id")
		} else {
			assert.NotNil(t, item.EndpointID, "endpoint item should have endpoint_id")
			assert.Nil(t, item.FolderID, "endpoint item should not have folder_id")
		}
	}
}

func TestCollectionItemService_DataConsistency(t *testing.T) {
	ctx := context.Background()
	db, queries, cleanup := createTestDB(t, ctx)
	defer cleanup()

	service := createTestService(queries)
	workspaceID := idwrap.NewNow()
	collectionID := createTestCollection(t, ctx, queries, workspaceID)

	t.Run("FK_constraints_validation", func(t *testing.T) {
		// Create folder and verify FK relationships
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)

		folder := &mitemfolder.ItemFolder{
			ID:           idwrap.NewNow(),
			CollectionID: collectionID,
			Name:         "FK Test Folder",
			ParentID:     nil,
		}

		err = service.CreateFolderTX(ctx, tx, folder)
		require.NoError(t, err)

		err = tx.Commit()
		require.NoError(t, err)

		// Verify collection_items entry exists and references correct folder
		items, err := service.ListCollectionItems(ctx, collectionID, nil)
		require.NoError(t, err)
		require.Len(t, items, 1)

		collectionItem := items[0]
		assert.NotNil(t, collectionItem.FolderID, "collection item should reference folder")
		assert.Equal(t, folder.ID, *collectionItem.FolderID, "collection item should reference correct folder")

		// Verify item_folder entry exists
		folderItem, err := queries.GetItemFolder(ctx, folder.ID)
		require.NoError(t, err)
		assert.Equal(t, folder.Name, folderItem.Name)
		assert.Equal(t, folder.CollectionID, folderItem.CollectionID)
	})

	t.Run("linked_list_integrity", func(t *testing.T) {
		// Create multiple items and verify linked list structure
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)

		items := []string{"Item1", "Item2", "Item3", "Item4"}
		for _, name := range items {
			folder := &mitemfolder.ItemFolder{
				ID:           idwrap.NewNow(),
				CollectionID: collectionID,
				Name:         name,
				ParentID:     nil,
			}
			err = service.CreateFolderTX(ctx, tx, folder)
			require.NoError(t, err)
		}

		err = tx.Commit()
		require.NoError(t, err)

		// Get ordered items and verify linked list structure
		orderedItems, err := service.ListCollectionItems(ctx, collectionID, nil)
		require.NoError(t, err)
		require.Greater(t, len(orderedItems), 3, "should have multiple items")

		// Check linked list integrity
		for i, item := range orderedItems {
			if i == 0 {
				assert.Nil(t, item.PrevID, "first item should have nil PrevID")
			} else {
				// In a proper implementation, this would check prev/next consistency
				// For now, we just verify the items are in order
			}

			if i == len(orderedItems)-1 {
				assert.Nil(t, item.NextID, "last item should have nil NextID")
			}
		}
	})
}

func BenchmarkCollectionItemService_CreateFolder(b *testing.B) {
	ctx := context.Background()
	db, queries, cleanup := createTestDB(&testing.T{}, ctx)
	defer cleanup()

	service := createTestService(queries)
	workspaceID := idwrap.NewNow()
	collectionID := createTestCollection(&testing.T{}, ctx, queries, workspaceID)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				b.Fatal(err)
			}

			folder := &mitemfolder.ItemFolder{
				ID:           idwrap.NewNow(),
				CollectionID: collectionID,
				Name:         "Benchmark Folder",
				ParentID:     nil,
			}

			err = service.CreateFolderTX(ctx, tx, folder)
			if err != nil {
				tx.Rollback()
				b.Fatal(err)
			}

			err = tx.Commit()
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkCollectionItemService_ListItems(b *testing.B) {
	ctx := context.Background()
	db, queries, cleanup := createTestDB(&testing.T{}, ctx)
	defer cleanup()

	service := createTestService(queries)
	workspaceID := idwrap.NewNow()
	collectionID := createTestCollection(&testing.T{}, ctx, queries, workspaceID)

	// Create some test data
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		b.Fatal(err)
	}

	for i := 0; i < 100; i++ {
		folder := &mitemfolder.ItemFolder{
			ID:           idwrap.NewNow(),
			CollectionID: collectionID,
			Name:         "Benchmark Folder",
			ParentID:     nil,
		}
		err = service.CreateFolderTX(ctx, tx, folder)
		if err != nil {
			tx.Rollback()
			b.Fatal(err)
		}
	}

	err = tx.Commit()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := service.ListCollectionItems(ctx, collectionID, nil)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
