package rbody_test

import (
    "context"
    "testing"
    "the-dev-tools/server/internal/api/middleware/mwauth"
    "the-dev-tools/server/internal/api/rbody"
    "the-dev-tools/server/pkg/idwrap"
    "the-dev-tools/server/pkg/logger/mocklogger"
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
    resourcesv1 "the-dev-tools/spec/dist/buf/go/resources/v1"

    "connectrpc.com/connect"
)

// E2E: normal (non-delta) URL-encoded ordering and move BEFORE
func TestUrlEncoded_Move_Before_E2E(t *testing.T) {
    ctx := context.Background()
    base := testutil.CreateBaseDB(ctx, t)
    q := base.Queries
    db := base.DB

    mockLogger := mocklogger.NewMockLogger()
    us := suser.New(q)
    cs := scollection.New(q, mockLogger)
    ias := sitemapi.New(q)
    iaes := sitemapiexample.New(q)
    brs := sbodyraw.New(q)
    bfs := sbodyform.New(q)
    bues := sbodyurl.New(q)
    rpc := rbody.New(db, cs, iaes, us, bfs, bues, brs)

    // Workspace/collection
    workspaceID := idwrap.NewNow()
    workspaceUserID := idwrap.NewNow()
    collectionID := idwrap.NewNow()
    userID := idwrap.NewNow()
    base.GetBaseServices().CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)
    authed := mwauth.CreateAuthedContext(ctx, userID)

    // Endpoint + example
    item := &mitemapi.ItemApi{ ID: idwrap.NewNow(), Name: "E2E", Url: "/e2e", Method: "POST", CollectionID: collectionID }
    if err := ias.CreateItemApi(ctx, item); err != nil { t.Fatal(err) }
    ex := &mitemapiexample.ItemApiExample{ ID: idwrap.NewNow(), ItemApiID: item.ID, CollectionID: collectionID, Name: "origin", BodyType: mitemapiexample.BodyTypeUrlencoded }
    if err := iaes.CreateApiExample(ctx, ex); err != nil { t.Fatal(err) }

    // Create 1,2,3
    create := func(k string) []byte {
        resp, err := rpc.BodyUrlEncodedCreate(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedCreateRequest{ ExampleId: ex.ID.Bytes(), Key: k, Enabled: true }))
        if err != nil { t.Fatalf("create %s: %v", k, err) }
        return resp.Msg.BodyId
    }
    id1 := create("1")
    _ = create("2")
    id3 := create("3")

    // Check order 1,2,3
    list1, err := rpc.BodyUrlEncodedList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedListRequest{ ExampleId: ex.ID.Bytes() }))
    if err != nil { t.Fatal(err) }
    if len(list1.Msg.Items) != 3 { t.Fatalf("expected 3 items, got %d", len(list1.Msg.Items)) }
    if list1.Msg.Items[0].Key != "1" || list1.Msg.Items[1].Key != "2" || list1.Msg.Items[2].Key != "3" {
        t.Fatalf("expected 1,2,3; got %s,%s,%s", list1.Msg.Items[0].Key, list1.Msg.Items[1].Key, list1.Msg.Items[2].Key)
    }

    // Move 3 BEFORE 1
    _, err = rpc.BodyUrlEncodedMove(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedMoveRequest{
        ExampleId: ex.ID.Bytes(),
        BodyId:    id3,
        TargetBodyId: id1,
        Position: resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
    }))
    if err != nil { t.Fatal(err) }

    // Verify 3,1,2
    list2, err := rpc.BodyUrlEncodedList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedListRequest{ ExampleId: ex.ID.Bytes() }))
    if err != nil { t.Fatal(err) }
    if len(list2.Msg.Items) != 3 { t.Fatalf("expected 3 items after move, got %d", len(list2.Msg.Items)) }
    if list2.Msg.Items[0].Key != "3" || list2.Msg.Items[1].Key != "1" || list2.Msg.Items[2].Key != "2" {
        t.Fatalf("expected 3,1,2; got %s,%s,%s", list2.Msg.Items[0].Key, list2.Msg.Items[1].Key, list2.Msg.Items[2].Key)
    }
}

// E2E: move AFTER (including append-to-end) should reorder correctly
func TestUrlEncoded_Move_After_E2E(t *testing.T) {
    ctx := context.Background()
    base := testutil.CreateBaseDB(ctx, t)
    q := base.Queries
    db := base.DB

    mockLogger := mocklogger.NewMockLogger()
    us := suser.New(q)
    cs := scollection.New(q, mockLogger)
    ias := sitemapi.New(q)
    iaes := sitemapiexample.New(q)
    brs := sbodyraw.New(q)
    bfs := sbodyform.New(q)
    bues := sbodyurl.New(q)
    rpc := rbody.New(db, cs, iaes, us, bfs, bues, brs)

    // Workspace/collection
    workspaceID := idwrap.NewNow()
    workspaceUserID := idwrap.NewNow()
    collectionID := idwrap.NewNow()
    userID := idwrap.NewNow()
    base.GetBaseServices().CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)
    authed := mwauth.CreateAuthedContext(ctx, userID)

    // Endpoint + example
    item := &mitemapi.ItemApi{ ID: idwrap.NewNow(), Name: "E2E", Url: "/e2e", Method: "POST", CollectionID: collectionID }
    if err := ias.CreateItemApi(ctx, item); err != nil { t.Fatal(err) }
    ex := &mitemapiexample.ItemApiExample{ ID: idwrap.NewNow(), ItemApiID: item.ID, CollectionID: collectionID, Name: "origin", BodyType: mitemapiexample.BodyTypeUrlencoded }
    if err := iaes.CreateApiExample(ctx, ex); err != nil { t.Fatal(err) }

    // Create 1,2,3
    create := func(k string) []byte {
        resp, err := rpc.BodyUrlEncodedCreate(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedCreateRequest{ ExampleId: ex.ID.Bytes(), Key: k, Enabled: true }))
        if err != nil { t.Fatalf("create %s: %v", k, err) }
        return resp.Msg.BodyId
    }
    _ = create("1")
    id2 := create("2")
    id3 := create("3")

    // Sanity: 1,2,3
    list1, err := rpc.BodyUrlEncodedList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedListRequest{ ExampleId: ex.ID.Bytes() }))
    if err != nil { t.Fatal(err) }
    if list1.Msg.Items[0].Key != "1" || list1.Msg.Items[1].Key != "2" || list1.Msg.Items[2].Key != "3" {
        t.Fatalf("expected 1,2,3; got %s,%s,%s", list1.Msg.Items[0].Key, list1.Msg.Items[1].Key, list1.Msg.Items[2].Key)
    }

    // Move 2 AFTER 3 (append to end)
    _, err = rpc.BodyUrlEncodedMove(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedMoveRequest{
        ExampleId: ex.ID.Bytes(),
        BodyId:    id2,
        TargetBodyId: id3,
        Position: resourcesv1.MovePosition_MOVE_POSITION_AFTER,
    }))
    if err != nil { t.Fatal(err) }

    // Expect 1,3,2
    list2, err := rpc.BodyUrlEncodedList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedListRequest{ ExampleId: ex.ID.Bytes() }))
    if err != nil { t.Fatal(err) }
    if list2.Msg.Items[0].Key != "1" || list2.Msg.Items[1].Key != "3" || list2.Msg.Items[2].Key != "2" {
        t.Fatalf("expected 1,3,2 after AFTER move; got %s,%s,%s", list2.Msg.Items[0].Key, list2.Msg.Items[1].Key, list2.Msg.Items[2].Key)
    }
}
