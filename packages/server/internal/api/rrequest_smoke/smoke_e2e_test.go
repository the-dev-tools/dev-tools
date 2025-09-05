package rrequest_smoke_test

import (
    "context"
    "testing"
    "the-dev-tools/server/internal/api/middleware/mwauth"
    rrequest "the-dev-tools/server/internal/api/rrequest"
    "the-dev-tools/server/pkg/idwrap"
    "the-dev-tools/server/pkg/logger/mocklogger"
    "the-dev-tools/server/pkg/model/mitemapi"
    "the-dev-tools/server/pkg/model/mitemapiexample"
    "the-dev-tools/server/pkg/service/sassert"
    "the-dev-tools/server/pkg/service/scollection"
    "the-dev-tools/server/pkg/service/sexampleheader"
    "the-dev-tools/server/pkg/service/sexamplequery"
    "the-dev-tools/server/pkg/service/sitemapi"
    "the-dev-tools/server/pkg/service/sitemapiexample"
    "the-dev-tools/server/pkg/service/suser"
    "the-dev-tools/server/pkg/testutil"
    requestv1 "the-dev-tools/spec/dist/buf/go/collection/item/request/v1"
    resourcesv1 "the-dev-tools/spec/dist/buf/go/resources/v1"

    "connectrpc.com/connect"
)

// Header delta smoke test: list/create/update/reset/delete
func TestHeaderDelta_Smoke(t *testing.T) {
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
    ehs := sexampleheader.New(q)
    eqs := sexamplequery.New(q)
    as := sassert.New(q)
    rpc := rrequest.New(db, cs, us, ias, iaes, ehs, eqs, as)

    // Bootstrap workspace
    workspaceID := idwrap.NewNow()
    workspaceUserID := idwrap.NewNow()
    collectionID := idwrap.NewNow()
    userID := idwrap.NewNow()
    base.GetBaseServices().CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)
    authed := mwauth.CreateAuthedContext(ctx, userID)

    // Endpoint + examples
    item := &mitemapi.ItemApi{ ID: idwrap.NewNow(), Name: "hdr-smoke", Url: "/e2e", Method: "GET", CollectionID: collectionID }
    if err := ias.CreateItemApi(ctx, item); err != nil { t.Fatal(err) }
    originEx := &mitemapiexample.ItemApiExample{ ID: idwrap.NewNow(), ItemApiID: item.ID, CollectionID: collectionID, Name: "origin", BodyType: mitemapiexample.BodyTypeNone }
    if err := iaes.CreateApiExample(ctx, originEx); err != nil { t.Fatal(err) }
    deltaEx := &mitemapiexample.ItemApiExample{ ID: idwrap.NewNow(), ItemApiID: item.ID, CollectionID: collectionID, Name: "delta", BodyType: mitemapiexample.BodyTypeNone, VersionParentID: &originEx.ID }
    if err := iaes.CreateApiExample(ctx, deltaEx); err != nil { t.Fatal(err) }

    // Origin headers: A,B
    for _, k := range []string{"A","B"} {
        if _, err := rpc.HeaderCreate(authed, connect.NewRequest(&requestv1.HeaderCreateRequest{ ExampleId: originEx.ID.Bytes(), Key: k, Value: k, Enabled: true })); err != nil { t.Fatalf("create origin header %s: %v", k, err) }
    }

    // Delta list proxies
    l1, err := rpc.HeaderDeltaList(authed, connect.NewRequest(&requestv1.HeaderDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    if err != nil { t.Fatal(err) }
    if len(l1.Msg.Items) != 2 { t.Fatalf("expected 2 items, got %d", len(l1.Msg.Items)) }

    // Create delta-only C (tail)
    cr, err := rpc.HeaderDeltaCreate(authed, connect.NewRequest(&requestv1.HeaderDeltaCreateRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes(), Key: "C", Value: "C", Enabled: true }))
    if err != nil { t.Fatal(err) }
    if len(cr.Msg.HeaderId) == 0 { t.Fatalf("expected header id for delta create") }

    // Update origin-ref 'A' (mixed)
    var aID []byte
    for _, it := range l1.Msg.Items { if it.Origin != nil && it.Origin.Key == "A" { aID = it.HeaderId; break } }
    if len(aID) == 0 { t.Fatalf("failed to find origin A in delta list") }
    if _, err := rpc.HeaderDeltaUpdate(authed, connect.NewRequest(&requestv1.HeaderDeltaUpdateRequest{ HeaderId: aID, Value: stringPtr("VA"), Enabled: boolPtr(true) })); err != nil { t.Fatal(err) }

    // Validate mixed
    l2, _ := rpc.HeaderDeltaList(authed, connect.NewRequest(&requestv1.HeaderDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    var aMixed *requestv1.HeaderDeltaListItem
    for _, it := range l2.Msg.Items { if it.Origin != nil && it.Origin.Key == "A" { aMixed = it; break } }
    if aMixed == nil || aMixed.Value != "VA" || !aMixed.Enabled { t.Fatalf("expected A mixed updated") }

    // Reset mixed by its delta id
    if _, err := rpc.HeaderDeltaReset(authed, connect.NewRequest(&requestv1.HeaderDeltaResetRequest{ HeaderId: aMixed.HeaderId })); err != nil { t.Fatal(err) }
    l3, _ := rpc.HeaderDeltaList(authed, connect.NewRequest(&requestv1.HeaderDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    var aAfter *requestv1.HeaderDeltaListItem
    for _, it := range l3.Msg.Items { if it.Origin != nil && it.Origin.Key == "A" { aAfter = it; break } }
    if aAfter == nil || aAfter.Value == "VA" { t.Fatalf("expected A reset to origin value") }

    // Delete delta-only C
    if _, err := rpc.HeaderDeltaDelete(authed, connect.NewRequest(&requestv1.HeaderDeltaDeleteRequest{ HeaderId: cr.Msg.HeaderId })); err != nil { t.Fatal(err) }
    l4, _ := rpc.HeaderDeltaList(authed, connect.NewRequest(&requestv1.HeaderDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    if len(l4.Msg.Items) != 2 { t.Fatalf("expected 2 items after delete, got %d", len(l4.Msg.Items)) }
}

// Query delta smoke test: list/create/update/reset/delete
func TestQueryDelta_Smoke(t *testing.T) {
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
    ehs := sexampleheader.New(q)
    eqs := sexamplequery.New(q)
    as := sassert.New(q)
    rpc := rrequest.New(db, cs, us, ias, iaes, ehs, eqs, as)

    // Bootstrap workspace
    workspaceID := idwrap.NewNow()
    workspaceUserID := idwrap.NewNow()
    collectionID := idwrap.NewNow()
    userID := idwrap.NewNow()
    base.GetBaseServices().CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)
    authed := mwauth.CreateAuthedContext(ctx, userID)

    // Endpoint + examples
    item := &mitemapi.ItemApi{ ID: idwrap.NewNow(), Name: "qry-smoke", Url: "/e2e", Method: "GET", CollectionID: collectionID }
    if err := ias.CreateItemApi(ctx, item); err != nil { t.Fatal(err) }
    originEx := &mitemapiexample.ItemApiExample{ ID: idwrap.NewNow(), ItemApiID: item.ID, CollectionID: collectionID, Name: "origin", BodyType: mitemapiexample.BodyTypeNone }
    if err := iaes.CreateApiExample(ctx, originEx); err != nil { t.Fatal(err) }
    deltaEx := &mitemapiexample.ItemApiExample{ ID: idwrap.NewNow(), ItemApiID: item.ID, CollectionID: collectionID, Name: "delta", BodyType: mitemapiexample.BodyTypeNone, VersionParentID: &originEx.ID }
    if err := iaes.CreateApiExample(ctx, deltaEx); err != nil { t.Fatal(err) }

    // Origin queries: q1,q2
    for _, k := range []string{"q1","q2"} {
        if _, err := rpc.QueryCreate(authed, connect.NewRequest(&requestv1.QueryCreateRequest{ ExampleId: originEx.ID.Bytes(), Key: k, Value: k, Enabled: true })); err != nil { t.Fatalf("create origin query %s: %v", k, err) }
    }

    // Delta list proxies
    l1, err := rpc.QueryDeltaList(authed, connect.NewRequest(&requestv1.QueryDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    if err != nil { t.Fatal(err) }
    if len(l1.Msg.Items) != 2 { t.Fatalf("expected 2 items, got %d", len(l1.Msg.Items)) }

    // Create delta-only q3 (tail)
    cr, err := rpc.QueryDeltaCreate(authed, connect.NewRequest(&requestv1.QueryDeltaCreateRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes(), Key: "q3", Value: "q3", Enabled: true }))
    if err != nil { t.Fatal(err) }
    if len(cr.Msg.QueryId) == 0 { t.Fatalf("expected query id for delta create") }

    // Update origin-ref 'q1' (mixed)
    var q1ID []byte
    for _, it := range l1.Msg.Items { if it.Origin != nil && it.Origin.Key == "q1" { q1ID = it.QueryId; break } }
    if len(q1ID) == 0 { t.Fatalf("failed to find origin q1 in delta list") }
    if _, err := rpc.QueryDeltaUpdate(authed, connect.NewRequest(&requestv1.QueryDeltaUpdateRequest{ QueryId: q1ID, Value: stringPtr("v1"), Enabled: boolPtr(true) })); err != nil { t.Fatal(err) }

    // Validate mixed
    l2, _ := rpc.QueryDeltaList(authed, connect.NewRequest(&requestv1.QueryDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    var q1Mixed *requestv1.QueryDeltaListItem
    for _, it := range l2.Msg.Items { if it.Origin != nil && it.Origin.Key == "q1" { q1Mixed = it; break } }
    if q1Mixed == nil || q1Mixed.Value != "v1" || !q1Mixed.Enabled { t.Fatalf("expected q1 mixed updated") }

    // Reset mixed by its delta id
    if _, err := rpc.QueryDeltaReset(authed, connect.NewRequest(&requestv1.QueryDeltaResetRequest{ QueryId: q1Mixed.QueryId })); err != nil { t.Fatal(err) }
    l3, _ := rpc.QueryDeltaList(authed, connect.NewRequest(&requestv1.QueryDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    var q1After *requestv1.QueryDeltaListItem
    for _, it := range l3.Msg.Items { if it.Origin != nil && it.Origin.Key == "q1" { q1After = it; break } }
    if q1After == nil || q1After.Value == "v1" { t.Fatalf("expected q1 reset to origin value") }

    // Delete delta-only q3
    if _, err := rpc.QueryDeltaDelete(authed, connect.NewRequest(&requestv1.QueryDeltaDeleteRequest{ QueryId: cr.Msg.QueryId })); err != nil { t.Fatal(err) }
    l4, _ := rpc.QueryDeltaList(authed, connect.NewRequest(&requestv1.QueryDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    if len(l4.Msg.Items) != 2 { t.Fatalf("expected 2 items after delete, got %d", len(l4.Msg.Items)) }
}

func stringPtr(s string) *string { return &s }
func boolPtr(b bool) *bool { return &b }

// Header delta move: ensures rank-based overlay reordering works for origin and delta refs
func TestHeaderDelta_Move(t *testing.T) {
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
    ehs := sexampleheader.New(q)
    eqs := sexamplequery.New(q)
    as := sassert.New(q)
    rpc := rrequest.New(db, cs, us, ias, iaes, ehs, eqs, as)

    // Bootstrap workspace
    workspaceID := idwrap.NewNow()
    workspaceUserID := idwrap.NewNow()
    collectionID := idwrap.NewNow()
    userID := idwrap.NewNow()
    base.GetBaseServices().CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)
    authed := mwauth.CreateAuthedContext(ctx, userID)

    // Endpoint + examples
    item := &mitemapi.ItemApi{ ID: idwrap.NewNow(), Name: "hdr-move", Url: "/e2e-move", Method: "GET", CollectionID: collectionID }
    if err := ias.CreateItemApi(ctx, item); err != nil { t.Fatal(err) }
    originEx := &mitemapiexample.ItemApiExample{ ID: idwrap.NewNow(), ItemApiID: item.ID, CollectionID: collectionID, Name: "origin", BodyType: mitemapiexample.BodyTypeNone }
    if err := iaes.CreateApiExample(ctx, originEx); err != nil { t.Fatal(err) }
    deltaEx := &mitemapiexample.ItemApiExample{ ID: idwrap.NewNow(), ItemApiID: item.ID, CollectionID: collectionID, Name: "delta", BodyType: mitemapiexample.BodyTypeNone, VersionParentID: &originEx.ID }
    if err := iaes.CreateApiExample(ctx, deltaEx); err != nil { t.Fatal(err) }

    // Origin headers: A,B
    for _, k := range []string{"A","B"} {
        if _, err := rpc.HeaderCreate(authed, connect.NewRequest(&requestv1.HeaderCreateRequest{ ExampleId: originEx.ID.Bytes(), Key: k, Value: k, Enabled: true })); err != nil {
            t.Fatalf("create origin header %s: %v", k, err)
        }
    }

    listKeys := func(items []*requestv1.HeaderDeltaListItem) []string {
        out := make([]string, 0, len(items))
        for _, it := range items {
            if it.Origin != nil { out = append(out, it.Origin.Key) } else { out = append(out, it.Key) }
        }
        return out
    }

    // Initial list seeds order: [A, B]
    l1, err := rpc.HeaderDeltaList(authed, connect.NewRequest(&requestv1.HeaderDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    if err != nil { t.Fatal(err) }
    got := listKeys(l1.Msg.Items)
    want := []string{"A","B"}
    if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
        t.Fatalf("seed order mismatch: got %v want %v", got, want)
    }

    // Create delta-only C (tail)
    cr, err := rpc.HeaderDeltaCreate(authed, connect.NewRequest(&requestv1.HeaderDeltaCreateRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes(), Key: "C", Value: "C", Enabled: true }))
    if err != nil { t.Fatal(err) }
    if len(cr.Msg.HeaderId) == 0 { t.Fatalf("expected header id for delta create") }

    // Move A after B -> [B, A, C]
    var aID, bID []byte
    for _, it := range l1.Msg.Items { if it.Origin != nil && it.Origin.Key == "A" { aID = it.HeaderId }; if it.Origin != nil && it.Origin.Key == "B" { bID = it.HeaderId } }
    if len(aID) == 0 || len(bID) == 0 { t.Fatalf("failed to resolve A/B ids from initial list") }
    if _, err := rpc.HeaderDeltaMove(authed, connect.NewRequest(&requestv1.HeaderDeltaMoveRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes(), HeaderId: aID, TargetHeaderId: bID, Position: resourcesv1.MovePosition_MOVE_POSITION_AFTER })); err != nil {
        t.Fatal(err)
    }
    l2, _ := rpc.HeaderDeltaList(authed, connect.NewRequest(&requestv1.HeaderDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    got = listKeys(l2.Msg.Items)
    want = []string{"B","A","C"}
    if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
        t.Fatalf("order after A->after(B) mismatch: got %v want %v", got, want)
    }

    // Move C before B -> [C, B, A]
    if _, err := rpc.HeaderDeltaMove(authed, connect.NewRequest(&requestv1.HeaderDeltaMoveRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes(), HeaderId: cr.Msg.HeaderId, TargetHeaderId: l2.Msg.Items[0].HeaderId, Position: resourcesv1.MovePosition_MOVE_POSITION_BEFORE })); err != nil {
        t.Fatal(err)
    }
    l3, _ := rpc.HeaderDeltaList(authed, connect.NewRequest(&requestv1.HeaderDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    got = listKeys(l3.Msg.Items)
    want = []string{"C","B","A"}
    if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
        t.Fatalf("order after C->before(B) mismatch: got %v want %v", got, want)
    }
}

// Query delta move: overlay order re-ranking across origin and delta-only
func TestQueryDelta_Move(t *testing.T) {
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
    ehs := sexampleheader.New(q)
    eqs := sexamplequery.New(q)
    as := sassert.New(q)
    rpc := rrequest.New(db, cs, us, ias, iaes, ehs, eqs, as)

    // Bootstrap workspace
    workspaceID := idwrap.NewNow(); workspaceUserID := idwrap.NewNow(); collectionID := idwrap.NewNow(); userID := idwrap.NewNow()
    base.GetBaseServices().CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)
    authed := mwauth.CreateAuthedContext(ctx, userID)

    // Endpoint + examples
    item := &mitemapi.ItemApi{ ID: idwrap.NewNow(), Name: "qry-move", Url: "/e2e-qmove", Method: "GET", CollectionID: collectionID }
    if err := ias.CreateItemApi(ctx, item); err != nil { t.Fatal(err) }
    originEx := &mitemapiexample.ItemApiExample{ ID: idwrap.NewNow(), ItemApiID: item.ID, CollectionID: collectionID, Name: "origin", BodyType: mitemapiexample.BodyTypeNone }
    if err := iaes.CreateApiExample(ctx, originEx); err != nil { t.Fatal(err) }
    deltaEx := &mitemapiexample.ItemApiExample{ ID: idwrap.NewNow(), ItemApiID: item.ID, CollectionID: collectionID, Name: "delta", BodyType: mitemapiexample.BodyTypeNone, VersionParentID: &originEx.ID }
    if err := iaes.CreateApiExample(ctx, deltaEx); err != nil { t.Fatal(err) }

    // Origin queries q1, q2
    for _, k := range []string{"q1","q2"} {
        if _, err := rpc.QueryCreate(authed, connect.NewRequest(&requestv1.QueryCreateRequest{ ExampleId: originEx.ID.Bytes(), Key: k, Value: k, Enabled: true })); err != nil {
            t.Fatalf("create origin query %s: %v", k, err)
        }
    }

    // Seed
    l1, err := rpc.QueryDeltaList(authed, connect.NewRequest(&requestv1.QueryDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    if err != nil { t.Fatal(err) }
    // Create q3
    cr, err := rpc.QueryDeltaCreate(authed, connect.NewRequest(&requestv1.QueryDeltaCreateRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes(), Key: "q3", Value: "q3", Enabled: true }))
    if err != nil { t.Fatal(err) }
    if len(cr.Msg.QueryId) == 0 { t.Fatalf("expected query id") }

    // Move q1 after q2
    var q1ID, q2ID []byte
    for _, it := range l1.Msg.Items { if it.Origin != nil && it.Origin.Key == "q1" { q1ID = it.QueryId }; if it.Origin != nil && it.Origin.Key == "q2" { q2ID = it.QueryId } }
    if len(q1ID) == 0 || len(q2ID) == 0 { t.Fatalf("failed to resolve q1/q2 ids") }
    if _, err := rpc.QueryDeltaMove(authed, connect.NewRequest(&requestv1.QueryDeltaMoveRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes(), QueryId: q1ID, TargetQueryId: q2ID, Position: resourcesv1.MovePosition_MOVE_POSITION_AFTER })); err != nil {
        t.Fatal(err)
    }
    // Verify [q2, q1, q3]
    l2, _ := rpc.QueryDeltaList(authed, connect.NewRequest(&requestv1.QueryDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    keys := func(items []*requestv1.QueryDeltaListItem) []string { out := []string{}; for _, it := range items { if it.Origin != nil { out = append(out, it.Origin.Key) } else { out = append(out, it.Key) } }; return out }
    got := keys(l2.Msg.Items)
    want := []string{"q2","q1","q3"}
    if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
        t.Fatalf("order after q1->after(q2) mismatch: got %v want %v", got, want)
    }

    // Move q3 before q2 -> [q3, q2, q1]
    if _, err := rpc.QueryDeltaMove(authed, connect.NewRequest(&requestv1.QueryDeltaMoveRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes(), QueryId: cr.Msg.QueryId, TargetQueryId: l2.Msg.Items[0].QueryId, Position: resourcesv1.MovePosition_MOVE_POSITION_BEFORE })); err != nil {
        t.Fatal(err)
    }
    l3, _ := rpc.QueryDeltaList(authed, connect.NewRequest(&requestv1.QueryDeltaListRequest{ ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes() }))
    got = keys(l3.Msg.Items)
    want = []string{"q3","q2","q1"}
    if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
        t.Fatalf("order after q3->before(q2) mismatch: got %v want %v", got, want)
    }
}
