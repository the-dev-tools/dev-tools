package scollectionitem

import (
	"context"
	"testing"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mitemfolder"
	"the-dev-tools/server/pkg/movable"

	"github.com/stretchr/testify/require"
)

func TestDebugMoveLogic(t *testing.T) {
	ctx := context.Background()
	db, queries, cleanup := createTestDB(t, ctx)
	defer cleanup()

	service := createTestService(queries)
	workspaceID := idwrap.NewNow()
	collectionID := createTestCollection(t, ctx, queries, workspaceID)

	t.Run("debug_move_before", func(t *testing.T) {
		// Create fresh items: A, B, C
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)

		folderA := &mitemfolder.ItemFolder{
			ID:           idwrap.NewNow(),
			CollectionID: collectionID,
			Name:         "A",
			ParentID:     nil,
		}
		folderB := &mitemfolder.ItemFolder{
			ID:           idwrap.NewNow(),
			CollectionID: collectionID,
			Name:         "B",
			ParentID:     nil,
		}
		folderC := &mitemfolder.ItemFolder{
			ID:           idwrap.NewNow(),
			CollectionID: collectionID,
			Name:         "C",
			ParentID:     nil,
		}

		err = service.CreateFolderTX(ctx, tx, folderA)
		require.NoError(t, err)
		err = service.CreateFolderTX(ctx, tx, folderB)
		require.NoError(t, err)
		err = service.CreateFolderTX(ctx, tx, folderC)
		require.NoError(t, err)

		err = tx.Commit()
		require.NoError(t, err)

		// Check initial order
		items, err := service.ListCollectionItems(ctx, collectionID, nil)
		require.NoError(t, err)
		require.Len(t, items, 3)
		
		t.Logf("Initial order: %s, %s, %s", items[0].Name, items[1].Name, items[2].Name)

		// Move B before C
		// Original: [A=0, B=1, C=2]
		// Move B(1) before C(2) -> should be [A, B, C] (no change) or [A, B, C] depending on interpretation
		// But typically "before C" means B should be inserted right before C's current position
		// So: [A, B, C] where B moves from position 1 to position 1 (before C at position 2) = no change
		
		// Actually, let's move A before C to see a real change
		// Move A(0) before C(2) -> should be [B, A, C]
		err = service.MoveCollectionItem(ctx, items[0].ID, &items[2].ID, movable.MovePositionBefore)
		require.NoError(t, err)

		newItems, err := service.ListCollectionItems(ctx, collectionID, nil)
		require.NoError(t, err)
		require.Len(t, newItems, 3)

		t.Logf("After moving A before C: %s, %s, %s", newItems[0].Name, newItems[1].Name, newItems[2].Name)
		
		// Expected: [B, A, C]
		require.Equal(t, "B", newItems[0].Name)
		require.Equal(t, "A", newItems[1].Name) 
		require.Equal(t, "C", newItems[2].Name)
	})
}