package rbody_form_overlay_test

import (
    "context"
    "testing"
    "the-dev-tools/server/internal/api/middleware/mwauth"
    rbody "the-dev-tools/server/internal/api/rbody"
    "the-dev-tools/server/pkg/idwrap"
    "the-dev-tools/server/pkg/logger/mocklogger"
    "the-dev-tools/server/pkg/model/mitemapi"
    "the-dev-tools/server/pkg/model/mitemapiexample"
    "the-dev-tools/server/pkg/service/sitemapi"
    "the-dev-tools/server/pkg/service/sbodyform"
    "the-dev-tools/server/pkg/service/sbodyraw"
    "the-dev-tools/server/pkg/service/sbodyurl"
    "the-dev-tools/server/pkg/service/scollection"
    "the-dev-tools/server/pkg/service/sitemapiexample"
    "the-dev-tools/server/pkg/service/suser"
    "the-dev-tools/server/pkg/testutil"
    bodyv1 "the-dev-tools/spec/dist/buf/go/collection/item/body/v1"
    resourcesv1 "the-dev-tools/spec/dist/buf/go/resources/v1"

    "connectrpc.com/connect"
)

// Body form delta overlay: list/create/update/reset/delete/move
func TestBodyFormDelta_Overlay_Flow(t *testing.T) {
    t.Parallel()
    ctx := context.Background()
    base := testutil.CreateBaseDB(ctx, t)
    db := base.DB
    q := base.Queries

    mockLogger := mocklogger.NewMockLogger()
    us := suser.New(q)
    cs := scollection.New(q, mockLogger)
    iaes := sitemapiexample.New(q)
    ias := sitemapi.New(q)
    bues := sbodyurl.New(q)
    bfs := sbodyform.New(q)
    brs := sbodyraw.New(q)
    rpc := rbody.New(db, cs, iaes, us, bfs, bues, brs)

    // Bootstrap workspace
    workspaceID := idwrap.NewNow(); workspaceUserID := idwrap.NewNow(); collectionID := idwrap.NewNow(); userID := idwrap.NewNow()
    base.GetBaseServices().CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)
    authed := mwauth.CreateAuthedContext(ctx, userID)

    // Endpoint + examples
    item := &mitemapi.ItemApi{ ID: idwrap.NewNow(), Name: "form-ov", Url: "/form-ov", Method: "POST", CollectionID: collectionID }
    if err := ias.CreateItemApi(ctx, item); err != nil { t.Fatal(err) }
    originEx := &mitemapiexample.ItemApiExample{ ID: idwrap.NewNow(), ItemApiID: item.ID, CollectionID: collectionID, Name: "origin", BodyType: mitemapiexample.BodyTypeForm }
    if err := iaes.CreateApiExample(ctx, originEx); err != nil { t.Fatal(err) }
    deltaEx := &mitemapiexample.ItemApiExample{ ID: idwrap.NewNow(), ItemApiID: item.ID, CollectionID: collectionID, Name: "delta", BodyType: mitemapiexample.BodyTypeForm, VersionParentID: &originEx.ID }
    if err := iaes.CreateApiExample(ctx, deltaEx); err != nil { t.Fatal(err) }

    // Origin forms A,B
    for _, k := range []string{"A","B"} {
        _, err := rpc.BodyFormCreate(authed, connect.NewRequest(&bodyv1.BodyFormCreateRequest{ ExampleId: originEx.ID.Bytes(), Key: k, Value: k, Enabled: true }))
        if err != nil { t.Fatalf("create origin form %s: %v", k, err) }
    }

    // List (seed)
    l1, err := rpc.BodyFormDeltaList(authed, connect.NewRequest(&bodyv1.BodyFormDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    if err != nil { t.Fatal(err) }
    if len(l1.Msg.Items) != 2 { t.Fatalf("expected 2 seed items, got %d", len(l1.Msg.Items)) }

    // Create delta-only C
    cr, err := rpc.BodyFormDeltaCreate(authed, connect.NewRequest(&bodyv1.BodyFormDeltaCreateRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes(), Key: "C", Value: "C", Enabled: true }))
    if err != nil { t.Fatal(err) }
    if len(cr.Msg.BodyId) == 0 { t.Fatalf("expected body id for delta create") }

    // Update origin-ref A -> mixed
    var aID []byte
    for _, it := range l1.Msg.Items { if it.Origin != nil && it.Origin.Key == "A" { aID = it.BodyId; break } }
    if len(aID) == 0 { t.Fatalf("failed to find origin A in list") }
    if _, err := rpc.BodyFormDeltaUpdate(authed, connect.NewRequest(&bodyv1.BodyFormDeltaUpdateRequest{ BodyId: aID, Value: stringPtr("VA"), Enabled: boolPtr(true) })); err != nil { t.Fatal(err) }
    l2, _ := rpc.BodyFormDeltaList(authed, connect.NewRequest(&bodyv1.BodyFormDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    var aMixed *bodyv1.BodyFormDeltaListItem
    for _, it := range l2.Msg.Items { if it.Origin != nil && it.Origin.Key == "A" { aMixed = it; break } }
    if aMixed == nil || aMixed.Value != "VA" || !aMixed.Enabled { t.Fatalf("expected A mixed updated") }

    // Reset mixed
    if _, err := rpc.BodyFormDeltaReset(authed, connect.NewRequest(&bodyv1.BodyFormDeltaResetRequest{ BodyId: aMixed.BodyId })); err != nil { t.Fatal(err) }
    l3, _ := rpc.BodyFormDeltaList(authed, connect.NewRequest(&bodyv1.BodyFormDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    var aAfter *bodyv1.BodyFormDeltaListItem
    for _, it := range l3.Msg.Items { if it.Origin != nil && it.Origin.Key == "A" { aAfter = it; break } }
    if aAfter == nil || aAfter.Value == "VA" { t.Fatalf("expected A reset to origin value") }

    // Delete delta-only C
    if _, err := rpc.BodyFormDeltaDelete(authed, connect.NewRequest(&bodyv1.BodyFormDeltaDeleteRequest{ BodyId: cr.Msg.BodyId })); err != nil { t.Fatal(err) }
    l4, _ := rpc.BodyFormDeltaList(authed, connect.NewRequest(&bodyv1.BodyFormDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    if len(l4.Msg.Items) != 2 { t.Fatalf("expected 2 items after delete, got %d", len(l4.Msg.Items)) }

    // Move order: A after B
    var bID []byte
    for _, it := range l4.Msg.Items { if it.Origin != nil && it.Origin.Key == "B" { bID = it.BodyId } }
    if _, err := rpc.BodyFormDeltaMove(authed, connect.NewRequest(&bodyv1.BodyFormDeltaMoveRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes(), BodyId: aAfter.BodyId, TargetBodyId: bID, Position: resourcesv1.MovePosition_MOVE_POSITION_AFTER })); err != nil { t.Fatal(err) }
    l5, _ := rpc.BodyFormDeltaList(authed, connect.NewRequest(&bodyv1.BodyFormDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    keys := func(items []*bodyv1.BodyFormDeltaListItem) []string { out := []string{}; for _, it := range items { if it.Origin != nil { out = append(out, it.Origin.Key) } else { out = append(out, it.Key) } }; return out }
    got := keys(l5.Msg.Items)
    want := []string{"B","A"}
    if len(got) != 2 || got[0] != want[0] || got[1] != want[1] { t.Fatalf("order after move mismatch: got %v want %v", got, want) }
}

func stringPtr(s string) *string { return &s }
func boolPtr(b bool) *bool { return &b }
