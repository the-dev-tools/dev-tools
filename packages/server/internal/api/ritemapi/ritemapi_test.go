package ritemapi_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"connectrpc.com/connect"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"the-dev-tools/db/pkg/sqlc"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rcollectionitem"
	"the-dev-tools/server/internal/api/ritemapi"
	"the-dev-tools/server/internal/api/ritemfolder"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/scollectionitem"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/sitemfolder"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/testutil"
	endpointv1 "the-dev-tools/spec/dist/buf/go/collection/item/endpoint/v1"
	folderv1 "the-dev-tools/spec/dist/buf/go/collection/item/folder/v1"
	itemv1 "the-dev-tools/spec/dist/buf/go/collection/item/v1"
)

// TestCrossCollectionFolderMoveRequestsVisible reproduces the bug where
// requests (endpoints) don’t show up after moving a folder to a different collection.
// It verifies that after moving a folder from collection A to collection B,
// the endpoints inside that folder are listed under collection B.
func TestCrossCollectionFolderMoveRequestsVisible(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()

	queries := base.Queries
	db := base.DB
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	_, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON")
	require.NoError(t, err)
	logger := mocklogger.NewMockLogger()

	// Seed a workspace, user, and two collections in the same workspace
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	collectionA := idwrap.NewNow()
	collectionB := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionA)
	// Reuse same workspace and user; Create second collection in same workspace
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionB)

	// Initialize services sharing same DB handle
	cs := scollection.New(queries, logger)
	us := suser.New(queries)
	cis := scollectionitem.New(queries, logger)
	ifs := sitemfolder.New(queries)
	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	ers := sexampleresp.New(queries)

	// RPCs
	collectionItemRPC := rcollectionitem.New(db, cs, cis, us, ifs, ias, iaes, ers)
	folderRPC := ritemfolder.New(db, ifs, us, cs, cis)
	apiRPC := ritemapi.New(db, ias, cs, ifs, us, iaes, ers, cis)

	authed := mwauth.CreateAuthedContext(ctx, userID)

	// Create folder in collection A
	folderReq := connect.NewRequest(&folderv1.FolderCreateRequest{
		CollectionId:   collectionA.Bytes(),
		Name:           "Moved Folder",
		ParentFolderId: nil, // root in A
	})
	folderResp, err := folderRPC.FolderCreate(authed, folderReq)
	require.NoError(t, err)
	folderID := idwrap.NewFromBytesMust(folderResp.Msg.FolderId)

	// Create endpoint inside that folder (in collection A)
	epReq := connect.NewRequest(&endpointv1.EndpointCreateRequest{
		CollectionId:   collectionA.Bytes(),
		Name:           "Inside Endpoint",
		Method:         "GET",
		Url:            "/inside",
		ParentFolderId: folderID.Bytes(),
	})
	_, err = apiRPC.EndpointCreate(authed, epReq)
	require.NoError(t, err)

	// Sanity: list items under folder in collection A
	listA := connect.NewRequest(&itemv1.CollectionItemListRequest{
		CollectionId:   collectionA.Bytes(),
		ParentFolderId: folderID.Bytes(),
	})
	listAResp, err := collectionItemRPC.CollectionItemList(authed, listA)
	require.NoError(t, err)
	require.Len(t, listAResp.Msg.Items, 1)
	assert.Equal(t, "Inside Endpoint", listAResp.Msg.Items[0].Endpoint.Name)

	// Cross-collection move: move the folder from A to B (to B root, end of list)
	moveReq := connect.NewRequest(&itemv1.CollectionItemMoveRequest{
		// The RPC expects legacy IDs; server maps to collection_items IDs internally
		Kind:               itemv1.ItemKind_ITEM_KIND_FOLDER,
		ItemId:             folderID.Bytes(),
		CollectionId:       collectionA.Bytes(),
		TargetCollectionId: collectionB.Bytes(),
		// No target item or parent folder -> add to end at B root
	})
	_, err = collectionItemRPC.CollectionItemMove(authed, moveReq)
	require.NoError(t, err)

	// Verify folder appears at collection B root
	listBRoot := connect.NewRequest(&itemv1.CollectionItemListRequest{
		CollectionId:   collectionB.Bytes(),
		ParentFolderId: nil,
	})
	listBRootResp, err := collectionItemRPC.CollectionItemList(authed, listBRoot)
	require.NoError(t, err)
	foundFolder := false
	for _, it := range listBRootResp.Msg.Items {
		if it.Kind == itemv1.ItemKind_ITEM_KIND_FOLDER && string(it.Folder.FolderId) == string(folderID.Bytes()) {
			foundFolder = true
			break
		}
	}
	assert.True(t, foundFolder, "moved folder should be visible at B root")

	// Critical assertion: endpoints inside moved folder should be visible in collection B
	listBFolder := connect.NewRequest(&itemv1.CollectionItemListRequest{
		CollectionId:   collectionB.Bytes(),
		ParentFolderId: folderID.Bytes(),
	})
	listBFolderResp, err := collectionItemRPC.CollectionItemList(authed, listBFolder)
	require.NoError(t, err)

	// This is the behavior we expect; currently this may fail if children
	// were not updated during cross-collection move.
	assert.Len(t, listBFolderResp.Msg.Items, 1, "endpoint should be listed under moved folder in target collection")
	if len(listBFolderResp.Msg.Items) > 0 {
		assert.Equal(t, "Inside Endpoint", listBFolderResp.Msg.Items[0].Endpoint.Name)
	}
}

func TestEndpointDeleteCascadesDeltaDependencies(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()

	origQueries := base.Queries
	origDB := base.DB

	uniqueName := ulid.Make().String()
	connStr := fmt.Sprintf("file:testdb_%s?mode=memory&cache=shared&_foreign_keys=1", uniqueName)
	newDB, err := sql.Open("sqlite3", connStr)
	require.NoError(t, err)
	newDB.SetMaxOpenConns(1)
	newDB.SetMaxIdleConns(1)
	_, err = newDB.ExecContext(ctx, "PRAGMA foreign_keys = ON")
	require.NoError(t, err)
	require.NoError(t, sqlc.CreateLocalTables(ctx, newDB))

	newQueries, err := gen.Prepare(ctx, newDB)
	require.NoError(t, err)

	base.DB = newDB
	base.Queries = newQueries

	if origQueries != nil {
		_ = origQueries.Close()
	}
	if origDB != nil {
		_ = origDB.Close()
	}

	queries := base.Queries
	db := base.DB
	logger := mocklogger.NewMockLogger()

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	collectionID := idwrap.NewNow()

	base.GetBaseServices().CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)

	cs := scollection.New(queries, logger)
	us := suser.New(queries)
	cis := scollectionitem.New(queries, logger)
	ifs := sitemfolder.New(queries)
	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	ers := sexampleresp.New(queries)

	apiRPC := ritemapi.New(db, ias, cs, ifs, us, iaes, ers, cis)
	authed := mwauth.CreateAuthedContext(ctx, userID)

	baseEndpointID := idwrap.NewNow()
	baseEndpoint := &mitemapi.ItemApi{
		ID:           baseEndpointID,
		CollectionID: collectionID,
		Name:         "base",
		Method:       "GET",
		Url:          "/base",
	}
	require.NoError(t, ias.CreateItemApi(authed, baseEndpoint))

	baseExampleID := idwrap.NewNow()
	baseExample := &mitemapiexample.ItemApiExample{
		ID:           baseExampleID,
		ItemApiID:    baseEndpointID,
		CollectionID: collectionID,
		Name:         "base",
	}
	require.NoError(t, iaes.CreateApiExample(authed, baseExample))

	deltaEndpointID := idwrap.NewNow()
	deltaParentID := baseEndpointID
	deltaEndpoint := &mitemapi.ItemApi{
		ID:            deltaEndpointID,
		CollectionID:  collectionID,
		Name:          "delta",
		Method:        "GET",
		Url:           "/delta",
		Hidden:        true,
		DeltaParentID: &deltaParentID,
	}
	require.NoError(t, ias.CreateItemApi(authed, deltaEndpoint))

	deltaExampleID := idwrap.NewNow()
	versionParentID := baseExampleID
	deltaExample := &mitemapiexample.ItemApiExample{
		ID:              deltaExampleID,
		ItemApiID:       deltaEndpointID,
		CollectionID:    collectionID,
		Name:            "delta",
		VersionParentID: &versionParentID,
	}
	require.NoError(t, iaes.CreateApiExample(authed, deltaExample))

	storedDelta, err := ias.GetItemApi(ctx, deltaEndpointID)
	require.NoError(t, err)
	require.NotNil(t, storedDelta.DeltaParentID)
	require.Equal(t, baseEndpointID, *storedDelta.DeltaParentID)

	deltaHeaderID := idwrap.NewNow()
	require.NoError(t, queries.DeltaHeaderDeltaInsert(ctx, gen.DeltaHeaderDeltaInsertParams{
		ExampleID:   deltaExampleID.Bytes(),
		ID:          deltaHeaderID.Bytes(),
		HeaderKey:   "X-Test",
		Value:       "delta",
		Description: "delta header",
		Enabled:     true,
	}))

	_, err = queries.DeltaHeaderDeltaExists(ctx, gen.DeltaHeaderDeltaExistsParams{
		ExampleID: deltaExampleID.Bytes(),
		ID:        deltaHeaderID.Bytes(),
	})
	require.NoError(t, err)

	flowID := idwrap.NewNow()
	require.NoError(t, queries.CreateFlow(ctx, gen.CreateFlowParams{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "flow",
	}))

	nodeID := idwrap.NewNow()
	require.NoError(t, queries.CreateFlowNode(ctx, gen.CreateFlowNodeParams{
		ID:        nodeID,
		FlowID:    flowID,
		Name:      "request",
		NodeKind:  int32(mnnode.NODE_KIND_REQUEST),
		PositionX: 0,
		PositionY: 0,
	}))

	endpointPtr := baseEndpointID
	examplePtr := baseExampleID
	deltaExamplePtr := deltaExampleID
	deltaEndpointPtr := deltaEndpointID
	require.NoError(t, queries.CreateFlowNodeRequest(ctx, gen.CreateFlowNodeRequestParams{
		FlowNodeID:      nodeID,
		EndpointID:      &endpointPtr,
		ExampleID:       &examplePtr,
		DeltaExampleID:  &deltaExamplePtr,
		DeltaEndpointID: &deltaEndpointPtr,
	}))

	nodeBeforeDelete, err := queries.GetFlowNodeRequest(ctx, nodeID)
	require.NoError(t, err)
	require.NotNil(t, nodeBeforeDelete.EndpointID)
	require.NotNil(t, nodeBeforeDelete.ExampleID)
	require.NotNil(t, nodeBeforeDelete.DeltaEndpointID)
	require.NotNil(t, nodeBeforeDelete.DeltaExampleID)

	_, err = apiRPC.EndpointDelete(authed, connect.NewRequest(&endpointv1.EndpointDeleteRequest{
		EndpointId: baseEndpointID.Bytes(),
	}))
	require.NoError(t, err)

	_, err = ias.GetItemApi(ctx, baseEndpointID)
	require.ErrorIs(t, err, sitemapi.ErrNoItemApiFound)

	deltaAfterDelete, err := ias.GetItemApi(ctx, deltaEndpointID)
	if err == nil {
		t.Logf("delta endpoint still present after delete: %+v", deltaAfterDelete)
	}
	require.ErrorIs(t, err, sitemapi.ErrNoItemApiFound)

	_, err = iaes.GetApiExample(ctx, baseExampleID)
	require.ErrorIs(t, err, sitemapiexample.ErrNoItemApiExampleFound)

	_, err = iaes.GetApiExample(ctx, deltaExampleID)
	require.ErrorIs(t, err, sitemapiexample.ErrNoItemApiExampleFound)

	exists, err := queries.DeltaHeaderDeltaExists(ctx, gen.DeltaHeaderDeltaExistsParams{
		ExampleID: deltaExampleID.Bytes(),
		ID:        deltaHeaderID.Bytes(),
	})
	t.Logf("delta overlay lookup after delete: exists=%d err=%v", exists, err)
	if err == nil {
		t.Fatalf("expected delta overlay rows to cascade delete, still found entry (exists=%d)", exists)
	}
	require.ErrorIs(t, err, sql.ErrNoRows)

	nodeAfterDelete, err := queries.GetFlowNodeRequest(ctx, nodeID)
	require.NoError(t, err)
	require.Nil(t, nodeAfterDelete.EndpointID)
	require.Nil(t, nodeAfterDelete.ExampleID)
	require.Nil(t, nodeAfterDelete.DeltaEndpointID)
	require.Nil(t, nodeAfterDelete.DeltaExampleID)
}
