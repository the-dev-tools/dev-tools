package rbody_test

import (
    "context"
    "testing"
    "the-dev-tools/server/internal/api/middleware/mwauth"
    "the-dev-tools/server/internal/api/rbody"
    "the-dev-tools/server/pkg/idwrap"
    "the-dev-tools/server/pkg/logger/mocklogger"
    "the-dev-tools/server/pkg/model/mbodyurl"
    "the-dev-tools/server/pkg/model/mitemapi"
    "the-dev-tools/server/pkg/model/mitemapiexample"
    "the-dev-tools/server/pkg/service/sbodyform"
    "the-dev-tools/server/pkg/service/sbodyraw"
    "the-dev-tools/server/pkg/service/sbodyurl"
    "the-dev-tools/server/pkg/service/scollection"
    "the-dev-tools/server/pkg/service/sitemapi"
    "the-dev-tools/server/pkg/service/sitemapiexample"
    "the-dev-tools/server/pkg/service/suser"
    "the-dev-tools/server/pkg/testutil"
    bodyv1 "the-dev-tools/spec/dist/buf/go/collection/item/body/v1"
    deltav1 "the-dev-tools/spec/dist/buf/go/delta/v1"

    "connectrpc.com/connect"
)

// Verifies that BodyUrlEncodedDeltaReset preserves DeltaParentID and restores fields from parent
func TestBodyUrlEncodedDeltaReset_PreservesParentAndRestoresValues(t *testing.T) {
    ctx := context.Background()
    base := testutil.CreateBaseDB(ctx, t)
    queries := base.Queries
    db := base.DB

    mockLogger := mocklogger.NewMockLogger()

    us := suser.New(queries)
    cs := scollection.New(queries, mockLogger)
    ias := sitemapi.New(queries)
    iaes := sitemapiexample.New(queries)

    brs := sbodyraw.New(queries)
    bfs := sbodyform.New(queries)
    bues := sbodyurl.New(queries)

    // Bootstrap workspace/collection
    workspaceID := idwrap.NewNow()
    workspaceUserID := idwrap.NewNow()
    collectionID := idwrap.NewNow()
    userID := idwrap.NewNow()
    base.GetBaseServices().CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)

    // Create endpoint
    item := &mitemapi.ItemApi{
        ID:           idwrap.NewNow(),
        Name:         "reset-test",
        Url:          "http://example.com",
        Method:       "POST",
        CollectionID: collectionID,
        FolderID:     nil,
    }
    if err := ias.CreateItemApi(ctx, item); err != nil {
        t.Fatal(err)
    }

    // Create origin and delta examples
    originExampleID := idwrap.NewNow()
    if err := iaes.CreateApiExample(ctx, &mitemapiexample.ItemApiExample{
        ID:              originExampleID,
        ItemApiID:       item.ID,
        CollectionID:    collectionID,
        Name:            "origin",
        BodyType:        mitemapiexample.BodyTypeUrlencoded,
        VersionParentID: nil,
    }); err != nil {
        t.Fatal(err)
    }

    deltaExampleID := idwrap.NewNow()
    if err := iaes.CreateApiExample(ctx, &mitemapiexample.ItemApiExample{
        ID:              deltaExampleID,
        ItemApiID:       item.ID,
        CollectionID:    collectionID,
        Name:            "delta",
        BodyType:        mitemapiexample.BodyTypeUrlencoded,
        VersionParentID: &originExampleID,
    }); err != nil {
        t.Fatal(err)
    }

    // Create an origin URL-encoded body
    originBody := &mbodyurl.BodyURLEncoded{
        ID:          idwrap.NewNow(),
        ExampleID:   originExampleID,
        BodyKey:     "k1",
        Value:       "v1",
        Description: "d1",
        Enable:      true,
    }
    if err := bues.CreateBodyURLEncoded(ctx, originBody); err != nil {
        t.Fatal(err)
    }

    rpc := rbody.New(db, cs, iaes, us, bfs, bues, brs)
    authed := mwauth.CreateAuthedContext(ctx, userID)

    // Sanity: list ensures a delta proxy exists (DELTA with parent)
    list1, err := rpc.BodyUrlEncodedDeltaList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaListRequest{
        ExampleId: deltaExampleID.Bytes(),
        OriginId:  originExampleID.Bytes(),
    }))
    if err != nil {
        t.Fatal(err)
    }
    if len(list1.Msg.Items) != 1 {
        t.Fatalf("expected 1 delta proxy, got %d", len(list1.Msg.Items))
    }
    if list1.Msg.Items[0].Source == nil || *list1.Msg.Items[0].Source != deltav1.SourceKind_SOURCE_KIND_DELTA {
        t.Fatalf("expected source DELTA initially, got %v", list1.Msg.Items[0].Source)
    }

    // Update via DeltaUpdate using the DELTA proxy id
    _, err = rpc.BodyUrlEncodedDeltaUpdate(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaUpdateRequest{
        BodyId:      list1.Msg.Items[0].BodyId,
        Key:         stringPtr("k1"),
        Enabled:     boolPtr(false),
        Value:       stringPtr("v1x"),
        Description: stringPtr("d1x"),
    }))
    if err != nil {
        t.Fatal(err)
    }

    // List again: delta proxy should reflect updated values
    list2, err := rpc.BodyUrlEncodedDeltaList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaListRequest{
        ExampleId: deltaExampleID.Bytes(),
        OriginId:  originExampleID.Bytes(),
    }))
    if err != nil {
        t.Fatal(err)
    }
    if len(list2.Msg.Items) != 1 { t.Fatalf("expected 1 delta item, got %d", len(list2.Msg.Items)) }
    updatedItem := list2.Msg.Items[0]
    if updatedItem.Value != "v1x" || updatedItem.Enabled != false || updatedItem.Description != "d1x" {
        t.Fatalf("values not updated as expected: got key=%s val=%s enabled=%v desc=%s", updatedItem.Key, updatedItem.Value, updatedItem.Enabled, updatedItem.Description)
    }

    // Reset and verify fields are restored to origin values while keeping parent link (still DELTA)
    _, err = rpc.BodyUrlEncodedDeltaReset(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaResetRequest{
        BodyId: updatedItem.BodyId,
    }))
    if err != nil {
        t.Fatal(err)
    }

    list3, err := rpc.BodyUrlEncodedDeltaList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaListRequest{
        ExampleId: deltaExampleID.Bytes(),
        OriginId:  originExampleID.Bytes(),
    }))
    if err != nil {
        t.Fatal(err)
    }
    if len(list3.Msg.Items) != 1 { t.Fatalf("expected 1 item after reset, got %d", len(list3.Msg.Items)) }
    after := list3.Msg.Items[0]
    if after.Value != "v1" || after.Enabled != true || after.Description != "d1" {
        t.Fatalf("expected values restored from origin (v1,true,d1); got val=%s enabled=%v desc=%s", after.Value, after.Enabled, after.Description)
    }
}

func stringPtr(s string) *string { return &s }
func boolPtr(b bool) *bool       { return &b }
