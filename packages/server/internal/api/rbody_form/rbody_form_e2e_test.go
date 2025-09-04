package rbody_form_test

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

    "connectrpc.com/connect"
)

func TestBodyForm_Delta_List_Create_Update_Reset_Delete(t *testing.T) {
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

    item := &mitemapi.ItemApi{ ID: idwrap.NewNow(), Name: "form-e2e", Url: "/e2e", Method: "POST", CollectionID: collectionID }
    if err := ias.CreateItemApi(ctx, item); err != nil { t.Fatal(err) }
    originEx := &mitemapiexample.ItemApiExample{ ID: idwrap.NewNow(), ItemApiID: item.ID, CollectionID: collectionID, Name: "origin", BodyType: mitemapiexample.BodyTypeForm }
    if err := iaes.CreateApiExample(ctx, originEx); err != nil { t.Fatal(err) }
    deltaEx := &mitemapiexample.ItemApiExample{ ID: idwrap.NewNow(), ItemApiID: item.ID, CollectionID: collectionID, Name: "delta", BodyType: mitemapiexample.BodyTypeForm, VersionParentID: &originEx.ID }
    if err := iaes.CreateApiExample(ctx, deltaEx); err != nil { t.Fatal(err) }

    // Create origin forms: a,b
    for _, k := range []string{"a","b"} {
        req := connect.NewRequest(&bodyv1.BodyFormCreateRequest{ ExampleId: originEx.ID.Bytes(), Key: k, Enabled: true })
        if _, err := rpc.BodyFormCreate(authed, req); err != nil { t.Fatalf("create origin %s: %v", k, err) }
    }

    // Delta list should show origin items as ORIGIN
    l1, err := rpc.BodyFormDeltaList(authed, connect.NewRequest(&bodyv1.BodyFormDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    if err != nil { t.Fatal(err) }
    if len(l1.Msg.Items) != 2 { t.Fatalf("expected 2 items, got %d", len(l1.Msg.Items)) }

    // Create delta-only form 'c'
    cr, err := rpc.BodyFormDeltaCreate(authed, connect.NewRequest(&bodyv1.BodyFormDeltaCreateRequest{ ExampleId: deltaEx.ID.Bytes(), Key: "c", Enabled: true }))
    if err != nil { t.Fatal(err) }
    if len(cr.Msg.BodyId) == 0 { t.Fatalf("expected body id for delta create") }

    // Update origin-ref 'a' via delta update (should create mixed)
    var aID []byte
    for _, it := range l1.Msg.Items {
        if it.Origin != nil && it.Origin.Key == "a" {
            aID = it.BodyId
            break
        }
    }
    if len(aID) == 0 { t.Fatalf("did not find origin id for a in delta list") }
    if _, err := rpc.BodyFormDeltaUpdate(authed, connect.NewRequest(&bodyv1.BodyFormDeltaUpdateRequest{ BodyId: aID, Value: stringPtr("va"), Enabled: boolPtr(true) })); err != nil { t.Fatal(err) }

    // List: should still show two origins and one delta, with 'a' now mixed (value updated)
    l2, err := rpc.BodyFormDeltaList(authed, connect.NewRequest(&bodyv1.BodyFormDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    if err != nil { t.Fatal(err) }
    var aMixed *bodyv1.BodyFormDeltaListItem
    for _, it := range l2.Msg.Items {
        if it.Origin != nil && it.Origin.Key == "a" {
            aMixed = it
            break
        }
    }
    if aMixed == nil || aMixed.Value != "va" || !aMixed.Enabled { t.Fatalf("expected a mixed with value 'va'") }

    // Reset 'a' back to origin values
    // Reset the mixed row by its delta id
    if _, err := rpc.BodyFormDeltaReset(authed, connect.NewRequest(&bodyv1.BodyFormDeltaResetRequest{ BodyId: aMixed.BodyId })); err != nil { t.Fatal(err) }
    l3, _ := rpc.BodyFormDeltaList(authed, connect.NewRequest(&bodyv1.BodyFormDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    var aAfter *bodyv1.BodyFormDeltaListItem
    for _, it := range l3.Msg.Items { if it.Origin != nil && it.Origin.Key == "a" { aAfter = it; break } }
    if aAfter == nil || aAfter.Value == "va" { t.Fatalf("expected a reset to origin value") }

    // Delete delta-only 'c'
    if _, err := rpc.BodyFormDeltaDelete(authed, connect.NewRequest(&bodyv1.BodyFormDeltaDeleteRequest{ BodyId: cr.Msg.BodyId })); err != nil { t.Fatal(err) }
    l4, _ := rpc.BodyFormDeltaList(authed, connect.NewRequest(&bodyv1.BodyFormDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    // Only a,b remain in view
    if len(l4.Msg.Items) != 2 { t.Fatalf("expected 2 items after delta delete, got %d", len(l4.Msg.Items)) }
}

func stringPtr(s string) *string { return &s }
func boolPtr(b bool) *bool { return &b }
