package scollectionitem

import (
	"context"
	"testing"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemfolder"

	"github.com/stretchr/testify/require"
)

func TestDebugItemCreation(t *testing.T) {
	ctx := context.Background()
	db, queries, cleanup := createTestDB(t, ctx)
	defer cleanup()

	service := createTestService(queries)
	workspaceID := idwrap.NewNow()
	collectionID := createTestCollection(t, ctx, queries, workspaceID)

	t.Run("debug_single_folder", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer tx.Rollback()

		folder := &mitemfolder.ItemFolder{
			ID:           idwrap.NewNow(),
			CollectionID: collectionID,
			Name:         "Debug Folder",
			ParentID:     nil,
		}

		err = service.CreateFolderTX(ctx, tx, folder)
		require.NoError(t, err)

		err = tx.Commit()
		require.NoError(t, err)

		// Check what's in the database
		items, err := service.ListCollectionItems(ctx, collectionID, nil)
		require.NoError(t, err)

		t.Logf("Found %d items", len(items))
		for i, item := range items {
			t.Logf("Item %d: ID=%s, Name=%s, Type=%d, FolderID=%v, EndpointID=%v", 
				i, item.ID.String(), item.Name, item.ItemType, 
				getIDString(item.FolderID), getIDString(item.EndpointID))
		}
	})

	t.Run("debug_folder_and_endpoint", func(t *testing.T) {
		// Fresh DB for this test
		db2, queries2, cleanup2 := createTestDB(t, ctx)
		defer cleanup2()
		service2 := createTestService(queries2)
		collectionID2 := createTestCollection(t, ctx, queries2, workspaceID)

		tx, err := db2.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer tx.Rollback()

		// Create folder first
		folder := &mitemfolder.ItemFolder{
			ID:           idwrap.NewNow(),
			CollectionID: collectionID2,
			Name:         "Test Folder",
			ParentID:     nil,
		}
		err = service2.CreateFolderTX(ctx, tx, folder)
		require.NoError(t, err)

		// Then create endpoint
		endpoint := &mitemapi.ItemApi{
			ID:           idwrap.NewNow(),
			CollectionID: collectionID2,
			Name:         "Test Endpoint",
			Url:          "https://api.example.com/test",
			Method:       "GET",
			FolderID:     nil,
		}
		err = service2.CreateEndpointTX(ctx, tx, endpoint)
		require.NoError(t, err)

		err = tx.Commit()
		require.NoError(t, err)

		// Check what's in the database
		items, err := service2.ListCollectionItems(ctx, collectionID2, nil)
		require.NoError(t, err)

		t.Logf("Found %d items after creating folder + endpoint", len(items))
		for i, item := range items {
			t.Logf("Item %d: ID=%s, Name=%s, Type=%d, FolderID=%v, EndpointID=%v, PrevID=%v, NextID=%v", 
				i, item.ID.String(), item.Name, item.ItemType, 
				getIDString(item.FolderID), getIDString(item.EndpointID),
				getIDString(item.PrevID), getIDString(item.NextID))
		}

		// Validate linked list structure
		require.Equal(t, 2, len(items), "should have 2 items")
		
		// First item (folder) should be head
		require.Nil(t, items[0].PrevID, "first item should have nil PrevID")
		require.NotNil(t, items[0].NextID, "first item should point to next")
		require.Equal(t, items[1].ID, *items[0].NextID, "first item should point to second")
		
		// Second item (endpoint) should be tail  
		require.NotNil(t, items[1].PrevID, "second item should point to prev")
		require.Equal(t, items[0].ID, *items[1].PrevID, "second item should point back to first")
		require.Nil(t, items[1].NextID, "second item should have nil NextID (tail)")
		
		t.Logf("Expected: second item NextID should be nil, but got: %v", getIDString(items[1].NextID))
	})
}

