package scollectionitem

import (
	"context"
	"log"
	"testing"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
)

func TestDebugInsertionLogic(t *testing.T) {
	ctx := context.Background()
	db, queries, cleanup := createTestDB(t, ctx)
	defer cleanup()

	service := createTestService(queries)
	workspaceID := idwrap.NewNow()
	collectionID := createTestCollection(t, ctx, queries, workspaceID)

	t.Run("step_by_step_insertion", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer tx.Rollback()

		// Step 1: Create folder
		folderID := idwrap.NewNow()

		log.Printf("=== STEP 1: Creating folder %s ===", folderID.String())

		// Manually debug the InsertAtPosition call
		repo := service.repository.TX(tx)
		
		// First insert the collection_items record
		err = queries.WithTx(tx).InsertCollectionItem(ctx, gen.InsertCollectionItemParams{
			ID:             idwrap.New(ulid.Make()),
			CollectionID:   collectionID,
			ParentFolderID: nil,
			ItemType:       0, // folder
			FolderID:       &folderID,
			EndpointID:     nil,
			Name:           "Test Folder",
		})
		require.NoError(t, err)

		// Get the collection item ID we just created 
		items, err := queries.WithTx(tx).GetCollectionItemsInOrder(ctx, gen.GetCollectionItemsInOrderParams{
			CollectionID:   collectionID,
			ParentFolderID: nil,
			CollectionID_2: collectionID,
		})
		require.NoError(t, err)
		
		var folderCollectionItemID idwrap.IDWrap
		if len(items) > 0 {
			folderCollectionItemID = idwrap.NewFromBytesMust(items[0].ID)
			log.Printf("Found collection item ID: %s for folder %s", folderCollectionItemID.String(), folderID.String())
		} else {
			log.Printf("No collection items found after folder insertion!")
		}
		
		// Now call InsertAtPosition for position 0 (should be first item)
		err = repo.InsertAtPosition(ctx, tx, folderCollectionItemID, collectionID, nil, 0)
		require.NoError(t, err)

		// Check state after folder
		items, err = queries.WithTx(tx).GetCollectionItemsInOrder(ctx, gen.GetCollectionItemsInOrderParams{
			CollectionID:   collectionID,
			ParentFolderID: nil,
			CollectionID_2: collectionID,
		})
		require.NoError(t, err)

		log.Printf("After folder insertion: found %d items", len(items))
		if len(items) > 0 {
			log.Printf("Folder: ID=%s, PrevID=%v, NextID=%v", 
				idwrap.NewFromBytesMust(items[0].ID).String(), 
				convertBytesToIDWrap(items[0].PrevID),
				convertBytesToIDWrap(items[0].NextID))
		}

		// Step 2: Create endpoint
		endpointID := idwrap.NewNow()

		log.Printf("=== STEP 2: Creating endpoint %s ===", endpointID.String())

		// Insert the collection_items record for endpoint
		endpointCollectionItemID := idwrap.New(ulid.Make())
		err = queries.WithTx(tx).InsertCollectionItem(ctx, gen.InsertCollectionItemParams{
			ID:             endpointCollectionItemID,
			CollectionID:   collectionID,
			ParentFolderID: nil,
			ItemType:       1, // endpoint
			FolderID:       nil,
			EndpointID:     &endpointID,
			Name:           "Test Endpoint",
		})
		require.NoError(t, err)
		
		log.Printf("Created endpoint collection item ID: %s for endpoint %s", endpointCollectionItemID.String(), endpointID.String())

		// Get current state before InsertAtPosition
		items, err = queries.WithTx(tx).GetCollectionItemsInOrder(ctx, gen.GetCollectionItemsInOrderParams{
			CollectionID:   collectionID,
			ParentFolderID: nil,
			CollectionID_2: collectionID,
		})
		require.NoError(t, err)
		
		log.Printf("Before endpoint InsertAtPosition: found %d items", len(items))
		for i, item := range items {
			log.Printf("Item %d: ID=%s, Name=%s, PrevID=%v, NextID=%v", i,
				idwrap.NewFromBytesMust(item.ID).String(), item.Name,
				convertBytesToIDWrap(item.PrevID), convertBytesToIDWrap(item.NextID))
		}

		// Now call InsertAtPosition for the endpoint (should be position 1, at tail)
		log.Printf("Calling InsertAtPosition for endpoint at position >= %d", len(items))
		err = repo.InsertAtPosition(ctx, tx, endpointCollectionItemID, collectionID, nil, len(items))
		require.NoError(t, err)

		// Check final state
		items, err = queries.WithTx(tx).GetCollectionItemsInOrder(ctx, gen.GetCollectionItemsInOrderParams{
			CollectionID:   collectionID,
			ParentFolderID: nil,
			CollectionID_2: collectionID,
		})
		require.NoError(t, err)

		log.Printf("=== FINAL STATE: found %d items ===", len(items))
		for i, item := range items {
			log.Printf("Item %d: ID=%s, Name=%s, PrevID=%v, NextID=%v", i,
				idwrap.NewFromBytesMust(item.ID).String(), item.Name,
				convertBytesToIDWrap(item.PrevID), convertBytesToIDWrap(item.NextID))
		}

		// Validate linked list
		if len(items) == 2 {
			// Check head
			if convertBytesToIDWrap(items[0].PrevID) != nil {
				t.Errorf("Head should have nil PrevID, got %v", convertBytesToIDWrap(items[0].PrevID))
			}
			if convertBytesToIDWrap(items[0].NextID) == nil {
				t.Errorf("Head should have NextID pointing to tail, got nil")
			} else if convertBytesToIDWrap(items[0].NextID).Compare(idwrap.NewFromBytesMust(items[1].ID)) != 0 {
				t.Errorf("Head NextID should point to tail, got %v", convertBytesToIDWrap(items[0].NextID))
			}

			// Check tail
			if convertBytesToIDWrap(items[1].PrevID) == nil {
				t.Errorf("Tail should have PrevID pointing to head, got nil")
			} else if convertBytesToIDWrap(items[1].PrevID).Compare(idwrap.NewFromBytesMust(items[0].ID)) != 0 {
				t.Errorf("Tail PrevID should point to head, got %v", convertBytesToIDWrap(items[1].PrevID))
			}
			if convertBytesToIDWrap(items[1].NextID) != nil {
				t.Errorf("Tail should have nil NextID, got %v", convertBytesToIDWrap(items[1].NextID))
			}
		}

		err = tx.Commit()
		require.NoError(t, err)
	})
}