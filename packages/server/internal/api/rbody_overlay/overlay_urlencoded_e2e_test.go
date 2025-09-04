package rbody_overlay_test

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

// Overlay: seed, list classification, and create delta-only
func TestOverlay_URLEncoded_DeltaList_SeedAndClassify(t *testing.T) {
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

    item := &mitemapi.ItemApi{ ID: idwrap.NewNow(), Name: "overlay-list", Url: "/e2e", Method: "POST", CollectionID: collectionID }
    if err := ias.CreateItemApi(ctx, item); err != nil { t.Fatal(err) }
    originEx := &mitemapiexample.ItemApiExample{ ID: idwrap.NewNow(), ItemApiID: item.ID, CollectionID: collectionID, Name: "origin", BodyType: mitemapiexample.BodyTypeUrlencoded }
    if err := iaes.CreateApiExample(ctx, originEx); err != nil { t.Fatal(err) }
    deltaEx := &mitemapiexample.ItemApiExample{ ID: idwrap.NewNow(), ItemApiID: item.ID, CollectionID: collectionID, Name: "delta", BodyType: mitemapiexample.BodyTypeUrlencoded, VersionParentID: &originEx.ID }
    if err := iaes.CreateApiExample(ctx, deltaEx); err != nil { t.Fatal(err) }

    // Origin items
    for _, k := range []string{"a","b","c"} {
        if _, err := rpc.BodyUrlEncodedCreate(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedCreateRequest{ ExampleId: originEx.ID.Bytes(), Key: k, Enabled: true })); err != nil { t.Fatalf("create origin %s: %v", k, err) }
    }

    // Seed + list
    dl1, err := rpc.BodyUrlEncodedDeltaList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    if err != nil { t.Fatal(err) }
    if len(dl1.Msg.Items) != 3 { t.Fatalf("expected 3 items, got %d", len(dl1.Msg.Items)) }
    got := []string{ dl1.Msg.Items[0].Key, dl1.Msg.Items[1].Key, dl1.Msg.Items[2].Key }
    want := []string{"a","b","c"}
    if got[0] != want[0] || got[1] != want[1] || got[2] != want[2] { t.Fatalf("seed order mismatch: want %v, got %v", want, got) }

    // Create delta-only with key d appended at tail
    cr, err := rpc.BodyUrlEncodedDeltaCreate(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaCreateRequest{ ExampleId: deltaEx.ID.Bytes(), Key: "d", Enabled: true }))
    if err != nil { t.Fatal(err) }
    if len(cr.Msg.BodyId) == 0 { t.Fatalf("expected body id for delta create") }
    dl2, err := rpc.BodyUrlEncodedDeltaList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    if err != nil { t.Fatal(err) }
    if len(dl2.Msg.Items) != 4 || dl2.Msg.Items[3].Key != "d" { t.Fatalf("expected tail d, got len=%d tail=%s", len(dl2.Msg.Items), dl2.Msg.Items[len(dl2.Msg.Items)-1].Key) }
}

// Overlay: move before/after using origin and delta ids
func TestOverlay_URLEncoded_Move_BeforeAfter(t *testing.T) {
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

    workspaceID := idwrap.NewNow()
    workspaceUserID := idwrap.NewNow()
    collectionID := idwrap.NewNow()
    userID := idwrap.NewNow()
    base.GetBaseServices().CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)
    authed := mwauth.CreateAuthedContext(ctx, userID)

    item := &mitemapi.ItemApi{ ID: idwrap.NewNow(), Name: "overlay-move", Url: "/e2e", Method: "POST", CollectionID: collectionID }
    if err := ias.CreateItemApi(ctx, item); err != nil { t.Fatal(err) }
    originEx := &mitemapiexample.ItemApiExample{ ID: idwrap.NewNow(), ItemApiID: item.ID, CollectionID: collectionID, Name: "origin", BodyType: mitemapiexample.BodyTypeUrlencoded }
    if err := iaes.CreateApiExample(ctx, originEx); err != nil { t.Fatal(err) }
    deltaEx := &mitemapiexample.ItemApiExample{ ID: idwrap.NewNow(), ItemApiID: item.ID, CollectionID: collectionID, Name: "delta", BodyType: mitemapiexample.BodyTypeUrlencoded, VersionParentID: &originEx.ID }
    if err := iaes.CreateApiExample(ctx, deltaEx); err != nil { t.Fatal(err) }

    for _, k := range []string{"a","b","c"} { if _, err := rpc.BodyUrlEncodedCreate(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedCreateRequest{ ExampleId: originEx.ID.Bytes(), Key: k, Enabled: true })); err != nil { t.Fatal(err) } }
    // seed
    dl1, _ := rpc.BodyUrlEncodedDeltaList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    // create delta-only d
    dr, _ := rpc.BodyUrlEncodedDeltaCreate(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaCreateRequest{ ExampleId: deltaEx.ID.Bytes(), Key: "d", Enabled: true }))

    // Move b AFTER d
    var bID []byte
    for _, it := range dl1.Msg.Items { if it.Key == "b" { bID = it.BodyId } }
    if _, err := rpc.BodyUrlEncodedDeltaMove(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaMoveRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes(), BodyId: bID, TargetBodyId: dr.Msg.BodyId, Position: resourcesv1.MovePosition_MOVE_POSITION_AFTER })); err != nil { t.Fatal(err) }
    dl2, _ := rpc.BodyUrlEncodedDeltaList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    got := []string{ dl2.Msg.Items[0].Key, dl2.Msg.Items[1].Key, dl2.Msg.Items[2].Key, dl2.Msg.Items[3].Key }
    want := []string{"a","c","d","b"}
    for i := 0; i < 4; i++ { if got[i] != want[i] { t.Fatalf("after AFTER move: want %v got %v", want, got) } }

    // Move d BEFORE a
    var aID []byte
    for _, it := range dl2.Msg.Items { if it.Key == "a" { aID = it.BodyId } }
    if _, err := rpc.BodyUrlEncodedDeltaMove(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaMoveRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes(), BodyId: dr.Msg.BodyId, TargetBodyId: aID, Position: resourcesv1.MovePosition_MOVE_POSITION_BEFORE })); err != nil { t.Fatal(err) }
    dl3, _ := rpc.BodyUrlEncodedDeltaList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    got2 := []string{ dl3.Msg.Items[0].Key, dl3.Msg.Items[1].Key, dl3.Msg.Items[2].Key, dl3.Msg.Items[3].Key }
    want2 := []string{"d","a","c","b"}
    for i := 0; i < 4; i++ { if got2[i] != want2[i] { t.Fatalf("after BEFORE move: want %v got %v", want2, got2) } }
}

// Overlay: update/reset/delete semantics
func TestOverlay_URLEncoded_UpdateResetDelete(t *testing.T) {
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

    workspaceID := idwrap.NewNow()
    workspaceUserID := idwrap.NewNow()
    collectionID := idwrap.NewNow()
    userID := idwrap.NewNow()
    base.GetBaseServices().CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)
    authed := mwauth.CreateAuthedContext(ctx, userID)

    item := &mitemapi.ItemApi{ ID: idwrap.NewNow(), Name: "overlay-update", Url: "/e2e", Method: "POST", CollectionID: collectionID }
    if err := ias.CreateItemApi(ctx, item); err != nil { t.Fatal(err) }
    originEx := &mitemapiexample.ItemApiExample{ ID: idwrap.NewNow(), ItemApiID: item.ID, CollectionID: collectionID, Name: "origin", BodyType: mitemapiexample.BodyTypeUrlencoded }
    if err := iaes.CreateApiExample(ctx, originEx); err != nil { t.Fatal(err) }
    deltaEx := &mitemapiexample.ItemApiExample{ ID: idwrap.NewNow(), ItemApiID: item.ID, CollectionID: collectionID, Name: "delta", BodyType: mitemapiexample.BodyTypeUrlencoded, VersionParentID: &originEx.ID }
    if err := iaes.CreateApiExample(ctx, deltaEx); err != nil { t.Fatal(err) }

    // Origin: a,b
    for _, k := range []string{"a","b"} { if _, err := rpc.BodyUrlEncodedCreate(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedCreateRequest{ ExampleId: originEx.ID.Bytes(), Key: k, Enabled: true })); err != nil { t.Fatal(err) } }
    dl1, _ := rpc.BodyUrlEncodedDeltaList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    var aID []byte
    for _, it := range dl1.Msg.Items { if it.Key == "a" { aID = it.BodyId } }
    // Update origin-ref -> MIXED
    if _, err := rpc.BodyUrlEncodedDeltaUpdate(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaUpdateRequest{ BodyId: aID, Value: stringPtr("va"), Enabled: boolPtr(true) })); err != nil { t.Fatal(err) }
    dl2, _ := rpc.BodyUrlEncodedDeltaList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    var aItem *bodyv1.BodyUrlEncodedDeltaListItem
    for _, it := range dl2.Msg.Items { if it.Key == "a" { aItem = it; break } }
    if aItem == nil || aItem.Value != "va" || !aItem.Enabled { t.Fatalf("expected a to be MIXED with new value") }
    // Reset origin-ref -> ORIGIN
    if _, err := rpc.BodyUrlEncodedDeltaReset(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaResetRequest{ BodyId: aID })); err != nil { t.Fatal(err) }
    dl3, _ := rpc.BodyUrlEncodedDeltaList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    aItem = nil
    for _, it := range dl3.Msg.Items { if it.Key == "a" { aItem = it; break } }
    if aItem == nil || aItem.Value == "va" { t.Fatalf("expected a reset to origin values") }
    // Delete origin-ref -> removed
    if _, err := rpc.BodyUrlEncodedDeltaDelete(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaDeleteRequest{ BodyId: aID })); err != nil { t.Fatal(err) }
    dl4, _ := rpc.BodyUrlEncodedDeltaList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    if len(dl4.Msg.Items) != 1 || dl4.Msg.Items[0].Key != "b" { t.Fatalf("expected only b after delete, got %d", len(dl4.Msg.Items)) }

    // Delta-only reset
    dr, _ := rpc.BodyUrlEncodedDeltaCreate(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaCreateRequest{ ExampleId: deltaEx.ID.Bytes(), Key: "x", Enabled: true }))
    if _, err := rpc.BodyUrlEncodedDeltaUpdate(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaUpdateRequest{ BodyId: dr.Msg.BodyId, Value: stringPtr("vx") })); err != nil { t.Fatal(err) }
    if _, err := rpc.BodyUrlEncodedDeltaReset(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaResetRequest{ BodyId: dr.Msg.BodyId })); err != nil { t.Fatal(err) }
    dl5, _ := rpc.BodyUrlEncodedDeltaList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    var sawX bool
    for _, it := range dl5.Msg.Items { if it.BodyId != nil && string(it.BodyId) == string(dr.Msg.BodyId) { sawX = true; if it.Value != "" { t.Fatalf("expected cleared value for delta-only reset") } } }
    if !sawX { t.Fatalf("expected to find delta-only x in list") }
}

func stringPtr(s string) *string { return &s }
func boolPtr(b bool) *bool { return &b }
