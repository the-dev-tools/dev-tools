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
    deltav1 "the-dev-tools/spec/dist/buf/go/delta/v1"
    resourcesv1 "the-dev-tools/spec/dist/buf/go/resources/v1"

    "connectrpc.com/connect"
)

func TestUrlEncoded_DeltaOrderingAndMove_E2E(t *testing.T) {
    t.Parallel()
    ctx := context.Background()
    base := testutil.CreateBaseDB(ctx, t)
    db := base.DB
    q := base.Queries

    // Services
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

    // Endpoint and examples
    item := &mitemapi.ItemApi{ ID: idwrap.NewNow(), Name: "E2E", Url: "/e2e", Method: "POST", CollectionID: collectionID }
    if err := ias.CreateItemApi(ctx, item); err != nil { t.Fatal(err) }

    originEx := &mitemapiexample.ItemApiExample{ ID: idwrap.NewNow(), ItemApiID: item.ID, CollectionID: collectionID, Name: "origin", BodyType: mitemapiexample.BodyTypeUrlencoded }
    if err := iaes.CreateApiExample(ctx, originEx); err != nil { t.Fatal(err) }
    deltaEx := &mitemapiexample.ItemApiExample{ ID: idwrap.NewNow(), ItemApiID: item.ID, CollectionID: collectionID, Name: "delta", BodyType: mitemapiexample.BodyTypeUrlencoded, VersionParentID: &originEx.ID }
    if err := iaes.CreateApiExample(ctx, deltaEx); err != nil { t.Fatal(err) }

    // Origin items: 11, 22, 33
    for _, k := range []string{"11","22","33"} {
        req := connect.NewRequest(&bodyv1.BodyUrlEncodedCreateRequest{ ExampleId: originEx.ID.Bytes(), Key: k, Enabled: true })
        if _, err := rpc.BodyUrlEncodedCreate(authed, req); err != nil { t.Fatalf("create origin %s: %v", k, err) }
    }

    // Delta list should auto-proxy 3 items in order
    list1, err := rpc.BodyUrlEncodedDeltaList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    if err != nil { t.Fatal(err) }
    if len(list1.Msg.Items) != 3 { t.Fatalf("expected 3 delta items (proxies), got %d", len(list1.Msg.Items)) }

    // Create delta-only item 44 appended at tail
    createDeltaResp, err := rpc.BodyUrlEncodedDeltaCreate(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaCreateRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes(), Key: "44", Enabled: true }))
    if err != nil { t.Fatal(err) }

    list2, err := rpc.BodyUrlEncodedDeltaList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    if err != nil { t.Fatal(err) }
    if len(list2.Msg.Items) != 4 { t.Fatalf("expected 4 delta items, got %d", len(list2.Msg.Items)) }
    // 44 should be last
    if list2.Msg.Items[3].Key != "44" { t.Fatalf("expected tail to be 44, got %s", list2.Msg.Items[3].Key) }

    // Move 44 before first
    targetFirstID := list2.Msg.Items[0].BodyId
    _, err = rpc.BodyUrlEncodedDeltaMove(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaMoveRequest{
        ExampleId: deltaEx.ID.Bytes(),
        OriginId:  originEx.ID.Bytes(),
        BodyId:    createDeltaResp.Msg.BodyId,
        TargetBodyId: targetFirstID,
        Position:  resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
    }))
    if err != nil { t.Fatal(err) }

    list3, err := rpc.BodyUrlEncodedDeltaList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    if err != nil { t.Fatal(err) }
    if list3.Msg.Items[0].Key != "44" { t.Fatalf("expected 44 to be first after move, got %s", list3.Msg.Items[0].Key) }

    // Origin list remains 11,22,33 (3 items)
    listOrigin, err := rpc.BodyUrlEncodedList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedListRequest{ ExampleId: originEx.ID.Bytes() }))
    if err != nil { t.Fatal(err) }
    if len(listOrigin.Msg.Items) != 3 { t.Fatalf("expected origin count 3, got %d", len(listOrigin.Msg.Items)) }
}

// E2E: delta move AFTER (append to end) should reorder correctly
func TestUrlEncoded_DeltaMove_After_E2E(t *testing.T) {
    t.Parallel()
    ctx := context.Background()
    base := testutil.CreateBaseDB(ctx, t)
    db := base.DB
    q := base.Queries

    // Services
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

    // Endpoint and examples
    item := &mitemapi.ItemApi{ ID: idwrap.NewNow(), Name: "E2E", Url: "/e2e", Method: "POST", CollectionID: collectionID }
    if err := ias.CreateItemApi(ctx, item); err != nil { t.Fatal(err) }
    originEx := &mitemapiexample.ItemApiExample{ ID: idwrap.NewNow(), ItemApiID: item.ID, CollectionID: collectionID, Name: "origin", BodyType: mitemapiexample.BodyTypeUrlencoded }
    if err := iaes.CreateApiExample(ctx, originEx); err != nil { t.Fatal(err) }
    deltaEx := &mitemapiexample.ItemApiExample{ ID: idwrap.NewNow(), ItemApiID: item.ID, CollectionID: collectionID, Name: "delta", BodyType: mitemapiexample.BodyTypeUrlencoded, VersionParentID: &originEx.ID }
    if err := iaes.CreateApiExample(ctx, deltaEx); err != nil { t.Fatal(err) }

    // Origin items: a,b,c
    for _, k := range []string{"a","b","c"} {
        req := connect.NewRequest(&bodyv1.BodyUrlEncodedCreateRequest{ ExampleId: originEx.ID.Bytes(), Key: k, Enabled: true })
        if _, err := rpc.BodyUrlEncodedCreate(authed, req); err != nil { t.Fatalf("create origin %s: %v", k, err) }
    }

    // Ensure proxies
    list1, err := rpc.BodyUrlEncodedDeltaList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    if err != nil { t.Fatal(err) }
    if len(list1.Msg.Items) != 3 { t.Fatalf("expected 3 delta items, got %d", len(list1.Msg.Items)) }

    // Add delta-only item d (tail)
    createDeltaResp, err := rpc.BodyUrlEncodedDeltaCreate(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaCreateRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes(), Key: "d", Enabled: true }))
    if err != nil { t.Fatal(err) }

    // Move b AFTER d (append b to end)
    // Find ids: b is second item in proxies (list1)
    bID := list1.Msg.Items[1].BodyId
    _, err = rpc.BodyUrlEncodedDeltaMove(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaMoveRequest{
        ExampleId: deltaEx.ID.Bytes(),
        OriginId:  originEx.ID.Bytes(),
        BodyId:    bID,
        TargetBodyId: createDeltaResp.Msg.BodyId,
        Position:  resourcesv1.MovePosition_MOVE_POSITION_AFTER,
    }))
    if err != nil { t.Fatal(err) }

    list2, err := rpc.BodyUrlEncodedDeltaList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    if err != nil { t.Fatal(err) }
    last := list2.Msg.Items[len(list2.Msg.Items)-1]
    if last.BodyId == bID { return }
    t.Fatalf("expected b to be last after AFTER move; got last key=%s", last.Key)
}

// Ensure DeltaUpdate rejects origin ids and updates delta rows in place
func TestUrlEncoded_DeltaUpdate_Rules(t *testing.T) {
    t.Parallel()
    ctx := context.Background()
    base := testutil.CreateBaseDB(ctx, t)
    db := base.DB
    q := base.Queries

    // Services
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

    // Endpoint and examples
    item := &mitemapi.ItemApi{ ID: idwrap.NewNow(), Name: "delta-update", Url: "/e2e", Method: "POST", CollectionID: collectionID }
    if err := ias.CreateItemApi(ctx, item); err != nil { t.Fatal(err) }
    originEx := &mitemapiexample.ItemApiExample{ ID: idwrap.NewNow(), ItemApiID: item.ID, CollectionID: collectionID, Name: "origin", BodyType: mitemapiexample.BodyTypeUrlencoded }
    if err := iaes.CreateApiExample(ctx, originEx); err != nil { t.Fatal(err) }
    deltaEx := &mitemapiexample.ItemApiExample{ ID: idwrap.NewNow(), ItemApiID: item.ID, CollectionID: collectionID, Name: "delta", BodyType: mitemapiexample.BodyTypeUrlencoded, VersionParentID: &originEx.ID }
    if err := iaes.CreateApiExample(ctx, deltaEx); err != nil { t.Fatal(err) }

    // Origin: k
    cr, err := rpc.BodyUrlEncodedCreate(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedCreateRequest{ ExampleId: originEx.ID.Bytes(), Key: "k", Enabled: true }))
    if err != nil { t.Fatal(err) }

    // Delta list ensures proxy
    dl, err := rpc.BodyUrlEncodedDeltaList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    if err != nil { t.Fatal(err) }
    if len(dl.Msg.Items) != 1 { t.Fatalf("expected 1 proxy, got %d", len(dl.Msg.Items)) }
    proxyID := dl.Msg.Items[0].BodyId

    // Update proxy in place: make it mixed
    if _, err := rpc.BodyUrlEncodedDeltaUpdate(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaUpdateRequest{ BodyId: proxyID, Value: stringPtr("v") })); err != nil { t.Fatal(err) }

    dl2, err := rpc.BodyUrlEncodedDeltaList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    if err != nil { t.Fatal(err) }
    if *dl2.Msg.Items[0].Source != deltav1.SourceKind_SOURCE_KIND_MIXED { t.Fatalf("expected mixed after update") }

    // Try updating ORIGIN id via delta endpoint -> must be error
    _, err = rpc.BodyUrlEncodedDeltaUpdate(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaUpdateRequest{ BodyId: cr.Msg.BodyId, Value: stringPtr("v2") }))
    if err == nil { t.Fatalf("expected error when updating origin id via delta update") }
}

// Ensure newly created proxies are inserted between existing anchored proxies following origin order,
// without disturbing delta-only items or user-moved proxies.
func TestUrlEncoded_Delta_EnsureMissingProxiesInsertedBetweenAnchors(t *testing.T) {
    t.Parallel()
    ctx := context.Background()
    base := testutil.CreateBaseDB(ctx, t)
    db := base.DB
    q := base.Queries

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

    // Endpoint and examples
    item := &mitemapi.ItemApi{ ID: idwrap.NewNow(), Name: "delta-anchors", Url: "/e2e", Method: "POST", CollectionID: collectionID }
    if err := ias.CreateItemApi(ctx, item); err != nil { t.Fatal(err) }
    originEx := &mitemapiexample.ItemApiExample{ ID: idwrap.NewNow(), ItemApiID: item.ID, CollectionID: collectionID, Name: "origin", BodyType: mitemapiexample.BodyTypeUrlencoded }
    if err := iaes.CreateApiExample(ctx, originEx); err != nil { t.Fatal(err) }
    deltaEx := &mitemapiexample.ItemApiExample{ ID: idwrap.NewNow(), ItemApiID: item.ID, CollectionID: collectionID, Name: "delta", BodyType: mitemapiexample.BodyTypeUrlencoded, VersionParentID: &originEx.ID }
    if err := iaes.CreateApiExample(ctx, deltaEx); err != nil { t.Fatal(err) }

    // Origin items: a,b,c,d
    for _, k := range []string{"a","b","c","d"} {
        if _, err := rpc.BodyUrlEncodedCreate(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedCreateRequest{ ExampleId: originEx.ID.Bytes(), Key: k, Enabled: true })); err != nil { t.Fatalf("create origin %s: %v", k, err) }
    }

    // Add delta-only item x first
    dx, err := rpc.BodyUrlEncodedDeltaCreate(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaCreateRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes(), Enabled: true }))
    if err != nil { t.Fatal(err) }
    if _, err := rpc.BodyUrlEncodedDeltaUpdate(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaUpdateRequest{ BodyId: dx.Msg.BodyId, Key: stringPtr("x"), Enabled: boolPtr(true) })); err != nil { t.Fatal(err) }

    // First delta list ensures proxies a,b,c,d, order expected: x,a,b,c,d (proxies appended after existing)
    dl1, err := rpc.BodyUrlEncodedDeltaList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    if err != nil { t.Fatal(err) }
    keys1 := []string{}
    for _, it := range dl1.Msg.Items { keys1 = append(keys1, it.Key) }
    if len(keys1) != 5 || keys1[0] != "x" || keys1[1] != "a" || keys1[2] != "b" || keys1[3] != "c" || keys1[4] != "d" {
        t.Fatalf("initial delta order unexpected: %v", keys1)
    }

    // Delete b and c proxies
    var bID, cID []byte
    for _, it := range dl1.Msg.Items { if it.Key == "b" { bID = it.BodyId }; if it.Key == "c" { cID = it.BodyId } }
    if _, err := rpc.BodyUrlEncodedDeltaDelete(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaDeleteRequest{ BodyId: bID })); err != nil { t.Fatal(err) }
    if _, err := rpc.BodyUrlEncodedDeltaDelete(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaDeleteRequest{ BodyId: cID })); err != nil { t.Fatal(err) }

    // Next delta list should recreate b and c and insert them between a and d -> x,a,b,c,d
    dl2, err := rpc.BodyUrlEncodedDeltaList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    if err != nil { t.Fatal(err) }
    keys2 := []string{}
    for _, it := range dl2.Msg.Items { keys2 = append(keys2, it.Key) }
    if len(keys2) != 5 || keys2[0] != "x" || keys2[1] != "a" || keys2[2] != "b" || keys2[3] != "c" || keys2[4] != "d" {
        t.Fatalf("after ensure-missing, want [x a b c d], got %v", keys2)
    }
}

// If there are no anchored proxies yet, proxies should append after existing delta-only items in origin order.
func TestUrlEncoded_Delta_ProxiesAppendAfterDeltaOnlyWhenNoAnchors(t *testing.T) {
    t.Parallel()
    ctx := context.Background()
    base := testutil.CreateBaseDB(ctx, t)
    db := base.DB
    q := base.Queries

    mockLogger := mocklogger.NewMockLogger()
    us := suser.New(q)
    cs := scollection.New(q, mockLogger)
    ias := sitemapi.New(q)
    iaes := sitemapiexample.New(q)
    brs := sbodyraw.New(q)
    bfs := sbodyform.New(q)
    bues := sbodyurl.New(q)
    rpc := rbody.New(db, cs, iaes, us, bfs, bues, brs)

    // Bootstrap
    workspaceID := idwrap.NewNow()
    workspaceUserID := idwrap.NewNow()
    collectionID := idwrap.NewNow()
    userID := idwrap.NewNow()
    base.GetBaseServices().CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)
    authed := mwauth.CreateAuthedContext(ctx, userID)

    // Examples
    item := &mitemapi.ItemApi{ ID: idwrap.NewNow(), Name: "delta-append", Url: "/e2e", Method: "POST", CollectionID: collectionID }
    if err := ias.CreateItemApi(ctx, item); err != nil { t.Fatal(err) }
    originEx := &mitemapiexample.ItemApiExample{ ID: idwrap.NewNow(), ItemApiID: item.ID, CollectionID: collectionID, Name: "origin", BodyType: mitemapiexample.BodyTypeUrlencoded }
    if err := iaes.CreateApiExample(ctx, originEx); err != nil { t.Fatal(err) }
    deltaEx := &mitemapiexample.ItemApiExample{ ID: idwrap.NewNow(), ItemApiID: item.ID, CollectionID: collectionID, Name: "delta", BodyType: mitemapiexample.BodyTypeUrlencoded, VersionParentID: &originEx.ID }
    if err := iaes.CreateApiExample(ctx, deltaEx); err != nil { t.Fatal(err) }

    // Origin: a,b
    for _, k := range []string{"a","b"} {
        if _, err := rpc.BodyUrlEncodedCreate(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedCreateRequest{ ExampleId: originEx.ID.Bytes(), Key: k, Enabled: true })); err != nil { t.Fatalf("create origin %s: %v", k, err) }
    }
    // Pre-existing delta-only y
    dy, err := rpc.BodyUrlEncodedDeltaCreate(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaCreateRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes(), Enabled: true }))
    if err != nil { t.Fatal(err) }
    if _, err := rpc.BodyUrlEncodedDeltaUpdate(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaUpdateRequest{ BodyId: dy.Msg.BodyId, Key: stringPtr("y"), Enabled: boolPtr(true) })); err != nil { t.Fatal(err) }

    dl, err := rpc.BodyUrlEncodedDeltaList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    if err != nil { t.Fatal(err) }
    keys := []string{}
    for _, it := range dl.Msg.Items { keys = append(keys, it.Key) }
    if len(keys) != 3 || keys[0] != "y" || keys[1] != "a" || keys[2] != "b" {
        t.Fatalf("expected proxies appended after delta-only [y a b], got %v", keys)
    }
}

// If user reorders existing proxies, newly created proxies should insert relative to anchored neighbors,
// preserving the user's existing order for the rest.
func TestUrlEncoded_Delta_MissingProxyAfterUserReorder(t *testing.T) {
    t.Parallel()
    ctx := context.Background()
    base := testutil.CreateBaseDB(ctx, t)
    db := base.DB
    q := base.Queries

    mockLogger := mocklogger.NewMockLogger()
    us := suser.New(q)
    cs := scollection.New(q, mockLogger)
    ias := sitemapi.New(q)
    iaes := sitemapiexample.New(q)
    brs := sbodyraw.New(q)
    bfs := sbodyform.New(q)
    bues := sbodyurl.New(q)
    rpc := rbody.New(db, cs, iaes, us, bfs, bues, brs)

    // Bootstrap
    workspaceID := idwrap.NewNow()
    workspaceUserID := idwrap.NewNow()
    collectionID := idwrap.NewNow()
    userID := idwrap.NewNow()
    base.GetBaseServices().CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)
    authed := mwauth.CreateAuthedContext(ctx, userID)

    item := &mitemapi.ItemApi{ ID: idwrap.NewNow(), Name: "delta-reorder", Url: "/e2e", Method: "POST", CollectionID: collectionID }
    if err := ias.CreateItemApi(ctx, item); err != nil { t.Fatal(err) }
    originEx := &mitemapiexample.ItemApiExample{ ID: idwrap.NewNow(), ItemApiID: item.ID, CollectionID: collectionID, Name: "origin", BodyType: mitemapiexample.BodyTypeUrlencoded }
    if err := iaes.CreateApiExample(ctx, originEx); err != nil { t.Fatal(err) }
    deltaEx := &mitemapiexample.ItemApiExample{ ID: idwrap.NewNow(), ItemApiID: item.ID, CollectionID: collectionID, Name: "delta", BodyType: mitemapiexample.BodyTypeUrlencoded, VersionParentID: &originEx.ID }
    if err := iaes.CreateApiExample(ctx, deltaEx); err != nil { t.Fatal(err) }

    // Origin: a,b,c
    for _, k := range []string{"a","b","c"} {
        if _, err := rpc.BodyUrlEncodedCreate(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedCreateRequest{ ExampleId: originEx.ID.Bytes(), Key: k, Enabled: true })); err != nil { t.Fatalf("create origin %s: %v", k, err) }
    }
    // Ensure proxies
    dl1, err := rpc.BodyUrlEncodedDeltaList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    if err != nil { t.Fatal(err) }
    // Move c before a (user reorders proxies)
    var aID, cID []byte
    for _, it := range dl1.Msg.Items { if it.Key == "a" { aID = it.BodyId }; if it.Key == "c" { cID = it.BodyId } }
    if _, err := rpc.BodyUrlEncodedDeltaMove(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaMoveRequest{
        ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes(), BodyId: cID, TargetBodyId: aID, Position: resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
    })); err != nil { t.Fatal(err) }
    // Delete b
    var bID []byte
    dl2, _ := rpc.BodyUrlEncodedDeltaList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    for _, it := range dl2.Msg.Items { if it.Key == "b" { bID = it.BodyId } }
    if _, err := rpc.BodyUrlEncodedDeltaDelete(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaDeleteRequest{ BodyId: bID })); err != nil { t.Fatal(err) }
    // Next list should recreate b and insert after a (anchor), preserving c before a
    dl3, err := rpc.BodyUrlEncodedDeltaList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    if err != nil { t.Fatal(err) }
    got := []string{}
    for _, it := range dl3.Msg.Items { got = append(got, it.Key) }
    // Expected: c before a (user move preserved), then newly recreated b after a: [c, a, b]
    if len(got) != 3 || got[0] != "c" || got[1] != "a" || got[2] != "b" {
        t.Fatalf("after ensure-missing with user reorder, want [c a b], got %v", got)
    }
}
