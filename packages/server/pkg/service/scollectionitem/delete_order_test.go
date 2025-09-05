package scollectionitem

import (
    "context"
    "database/sql"
    "testing"

    "github.com/stretchr/testify/require"

    "the-dev-tools/db/pkg/sqlc/gen"
    "the-dev-tools/server/pkg/idwrap"
    "the-dev-tools/server/pkg/model/mitemapi"
    "the-dev-tools/server/pkg/model/mitemfolder"
)

// createThreeItems creates A (folder), B (endpoint), C (folder) at root and returns their collection_items IDs
func createThreeItems(t *testing.T, ctx context.Context, db *sql.DB, queries *gen.Queries, service *CollectionItemService, collectionID idwrap.IDWrap) (aItem, bItem, cItem idwrap.IDWrap) {
    tx, err := db.BeginTx(ctx, nil)
    require.NoError(t, err)
    s := service.TX(tx)
    // A folder
    folder := &mitemfolder.ItemFolder{ID: idwrap.NewNow(), CollectionID: collectionID, Name: "A", ParentID: nil}
    require.NoError(t, s.CreateFolderTX(ctx, tx, folder))
    // B endpoint
    endpoint := &mitemapi.ItemApi{ID: idwrap.NewNow(), CollectionID: collectionID, Name: "B", Url: "/b", Method: "GET", FolderID: nil}
    require.NoError(t, s.CreateEndpointTX(ctx, tx, endpoint))
    // C folder
    folderC := &mitemfolder.ItemFolder{ID: idwrap.NewNow(), CollectionID: collectionID, Name: "C", ParentID: nil}
    require.NoError(t, s.CreateFolderTX(ctx, tx, folderC))
    require.NoError(t, tx.Commit())

    // Map legacy IDs to collection_items IDs
    var err2 error
    aItem, err2 = service.GetCollectionItemIDByLegacyID(ctx, folder.ID)
    require.NoError(t, err2)
    bItem, err2 = service.GetCollectionItemIDByLegacyID(ctx, endpoint.ID)
    require.NoError(t, err2)
    cItem, err2 = service.GetCollectionItemIDByLegacyID(ctx, folderC.ID)
    require.NoError(t, err2)
    return
}

func TestDelete_MiddleMaintainsPointers(t *testing.T) {
    ctx := context.Background()
    db, queries, cleanup := createTestDB(t, ctx)
    defer cleanup()
    service := createTestService(queries)
    workspaceID := idwrap.NewNow()
    collectionID := createTestCollection(t, ctx, queries, workspaceID)
    aItem, bItem, cItem := createThreeItems(t, ctx, db, queries, service, collectionID)

    tx, err := db.BeginTx(ctx, nil)
    require.NoError(t, err)
    require.NoError(t, service.DeleteCollectionItem(ctx, tx, bItem))
    require.NoError(t, tx.Commit())

    rows, err := queries.GetCollectionItemsInOrder(ctx, gen.GetCollectionItemsInOrderParams{CollectionID: collectionID, ParentFolderID: nil, Column3: nil, CollectionID_2: collectionID})
    require.NoError(t, err)
    require.Len(t, rows, 2)
    require.Equal(t, aItem.String(), idwrap.NewFromBytesMust(rows[0].ID).String())
    require.Equal(t, cItem.String(), idwrap.NewFromBytesMust(rows[1].ID).String())
    aRow, _ := queries.GetCollectionItem(ctx, aItem)
    cRow, _ := queries.GetCollectionItem(ctx, cItem)
    require.Nil(t, aRow.PrevID)
    require.Equal(t, cItem.String(), aRow.NextID.String())
    require.Equal(t, aItem.String(), cRow.PrevID.String())
}
