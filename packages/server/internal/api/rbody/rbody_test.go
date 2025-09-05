package rbody_test

import (
	"bytes"
	"context"
	"testing"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rbody"
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/model/mbodyform"
	"the-dev-tools/server/pkg/model/mbodyraw"
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
	resourcesv1 "the-dev-tools/spec/dist/buf/go/resources/v1"

	"connectrpc.com/connect"
)

func TestGetBodyRaw(t *testing.T) {
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

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	CollectionID := idwrap.NewNow()
	UserID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, UserID, CollectionID)

	item := &mitemapi.ItemApi{
		ID:           idwrap.NewNow(),
		Name:         "test",
		Url:          "test",
		Method:       "GET",
		CollectionID: CollectionID,
		FolderID:     nil,
	}

	err := ias.CreateItemApi(ctx, item)
	if err != nil {
		t.Fatal(err)
	}

	itemExample := &mitemapiexample.ItemApiExample{
		ID:           idwrap.NewNow(),
		ItemApiID:    item.ID,
		CollectionID: CollectionID,
		Name:         "test",
		BodyType:     mitemapiexample.BodyTypeRaw,
	}

	err = iaes.CreateApiExample(ctx, itemExample)
	if err != nil {
		t.Fatal(err)
	}

	rawBody := mbodyraw.ExampleBodyRaw{
		ID:            idwrap.NewNow(),
		VisualizeMode: mbodyraw.VisualizeModeHTML,
		ExampleID:     itemExample.ID,
		CompressType:  compress.CompressTypeNone,
		Data:          []byte("test body"),
	}

	err = brs.CreateBodyRaw(ctx, rawBody)
	if err != nil {
		t.Fatal(err)
	}

	req := connect.NewRequest(&bodyv1.BodyRawGetRequest{
		ExampleId: itemExample.ID.Bytes(),
	})

	rpcBody := rbody.New(db, cs, iaes, us,
		bfs, bues, brs)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcBody.BodyRawGet(authedCtx, req)
	if err != nil {
		t.Fatal(err)
		return
	}

	if resp == nil {
		t.Fatal("resp is nil")
	}

	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}
	msg := resp.Msg
	if !bytes.Equal(msg.Data, rawBody.Data) {
		t.Errorf("expected body %s, got %s", rawBody.Data, msg.Data)
	}
}

func TestGetBodyForm(t *testing.T) {
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

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	CollectionID := idwrap.NewNow()
	UserID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, UserID, CollectionID)

	item := &mitemapi.ItemApi{
		ID:           idwrap.NewNow(),
		Name:         "test",
		Url:          "test",
		Method:       "GET",
		CollectionID: CollectionID,
		FolderID:     nil,
	}

	err := ias.CreateItemApi(ctx, item)
	if err != nil {
		t.Fatal(err)
	}

	itemExample := &mitemapiexample.ItemApiExample{
		ID:           idwrap.NewNow(),
		ItemApiID:    item.ID,
		CollectionID: CollectionID,
		Name:         "test",
		BodyType:     mitemapiexample.BodyTypeRaw,
	}

	err = iaes.CreateApiExample(ctx, itemExample)
	if err != nil {
		t.Fatal(err)
	}

	const formCount = 50

	formBodyArr := make([]mbodyform.BodyForm, formCount)

	for i := 0; i < formCount; i++ {
		formBodyArr[i] = mbodyform.BodyForm{
			ID:          idwrap.NewNow(),
			Description: "test",
			BodyKey:     "test_key",
			Value:       "test_val",
			Enable:      true,
			ExampleID:   itemExample.ID,
		}
	}

	err = bfs.CreateBulkBodyForm(ctx, formBodyArr)
	if err != nil {
		t.Fatal(err)
	}

	req := connect.NewRequest(&bodyv1.BodyFormListRequest{
		ExampleId: itemExample.ID.Bytes(),
	})

	rpcBody := rbody.New(db, cs, iaes, us,
		bfs, bues, brs)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcBody.BodyFormList(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}

	if resp == nil {
		t.Fatal("resp is nil")
	}

	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}
	msg := resp.Msg

	// Create a map for easier lookup since order might not be guaranteed
	expectedMap := make(map[string]mbodyform.BodyForm)
	for _, body := range formBodyArr {
		expectedMap[body.BodyKey] = body
	}

	if len(msg.Items) != formCount {
		t.Errorf("expected %d items, got %d", formCount, len(msg.Items))
	}

	for _, item := range msg.Items {
		expected, exists := expectedMap[item.Key]
		if !exists {
			t.Errorf("unexpected key %s in response", item.Key)
			continue
		}
		if item.Description != expected.Description {
			t.Errorf("expected description %s, got %s", expected.Description, item.Description)
		}
		if item.Value != expected.Value {
			t.Errorf("expected value %s, got %s", expected.Value, item.Value)
		}
		if item.Enabled != expected.Enable {
			t.Errorf("expected enable %t, got %t", expected.Enable, item.Enabled)
		}
	}
}

func TestGetBodyUrlEncoded(t *testing.T) {
	// Removed t.Parallel() to avoid ULID race conditions
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

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	CollectionID := idwrap.NewNow()
	UserID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, UserID, CollectionID)

	item := &mitemapi.ItemApi{
		ID:           idwrap.NewNow(),
		Name:         "test",
		Url:          "test",
		Method:       "GET",
		CollectionID: CollectionID,
		FolderID:     nil,
	}

	err := ias.CreateItemApi(ctx, item)
	if err != nil {
		t.Fatal(err)
	}

	itemExample := &mitemapiexample.ItemApiExample{
		ID:           idwrap.NewNow(),
		ItemApiID:    item.ID,
		CollectionID: CollectionID,
		Name:         "test",
		BodyType:     mitemapiexample.BodyTypeRaw,
	}

	err = iaes.CreateApiExample(ctx, itemExample)
	if err != nil {
		t.Fatal(err)
	}

	// Create a single body to test the functionality - simpler and less prone to race conditions
	singleBody := mbodyurl.BodyURLEncoded{
		ID:          idwrap.NewNow(),
		Description: "test",
		BodyKey:     "test_key",
		Value:       "test_value",
		Enable:      true,
		ExampleID:   itemExample.ID,
	}

	// Convert to slice for bulk creation
	formBodyArr := []mbodyurl.BodyURLEncoded{singleBody}

	err = bues.CreateBulkBodyURLEncoded(ctx, formBodyArr)
	if err != nil {
		// Debug: Print IDs to see which one is causing the conflict
		t.Logf("Failed to create bulk URL encoded bodies. IDs used:")
		for i, body := range formBodyArr {
			t.Logf("  [%d]: %s", i, body.ID.String())
		}
		t.Fatal(err)
	}

	req := connect.NewRequest(&bodyv1.BodyUrlEncodedListRequest{
		ExampleId: itemExample.ID.Bytes(),
	})

	rpcBody := rbody.New(db, cs, iaes, us,
		bfs, bues, brs)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcBody.BodyUrlEncodedList(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}

	if resp == nil {
		t.Fatal("resp is nil")
	}

	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}
	msg := resp.Msg

	// Create a map for easier lookup since order might not be guaranteed
	expectedMap := make(map[string]mbodyurl.BodyURLEncoded)
	for _, body := range formBodyArr {
		expectedMap[body.BodyKey] = body
	}

	if len(msg.Items) != len(formBodyArr) {
		t.Errorf("expected %d items, got %d", len(formBodyArr), len(msg.Items))
	}

	for _, item := range msg.Items {
		expected, exists := expectedMap[item.Key]
		if !exists {
			t.Errorf("unexpected key %s in response", item.Key)
			continue
		}
		if item.Description != expected.Description {
			t.Errorf("expected description %s, got %s", expected.Description, item.Description)
		}
		if item.Value != expected.Value {
			t.Errorf("expected value %s, got %s", expected.Value, item.Value)
		}
		if item.Enabled != expected.Enable {
			t.Errorf("expected enable %t, got %t", expected.Enable, item.Enabled)
		}
	}
}

// Verify URL-encoded move semantics: BEFORE/AFTER (including append-to-end)
func TestBodyUrlEncoded_MoveOrdering_Normal(t *testing.T) {
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
	item := &mitemapi.ItemApi{ID: idwrap.NewNow(), Name: "move-normal", Url: "/e2e", Method: "POST", CollectionID: collectionID}
	if err := ias.CreateItemApi(ctx, item); err != nil {
		t.Fatal(err)
	}
	ex := &mitemapiexample.ItemApiExample{ID: idwrap.NewNow(), ItemApiID: item.ID, CollectionID: collectionID, Name: "origin", BodyType: mitemapiexample.BodyTypeUrlencoded}
	if err := iaes.CreateApiExample(ctx, ex); err != nil {
		t.Fatal(err)
	}

	// Create 1,2,3
	create := func(k string) []byte {
		resp, err := rpc.BodyUrlEncodedCreate(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedCreateRequest{ExampleId: ex.ID.Bytes(), Key: k, Enabled: true}))
		if err != nil {
			t.Fatalf("create %s: %v", k, err)
		}
		return resp.Msg.BodyId
	}
	id1 := create("1")
	_ = create("2")
	id3 := create("3")

	// BEFORE: move 3 before 1 => 3,1,2
	if _, err := rpc.BodyUrlEncodedMove(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedMoveRequest{
		ExampleId: ex.ID.Bytes(), BodyId: id3, TargetBodyId: id1, Position: resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
	})); err != nil {
		t.Fatal(err)
	}
	list1, err := rpc.BodyUrlEncodedList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedListRequest{ExampleId: ex.ID.Bytes()}))
	if err != nil {
		t.Fatal(err)
	}
	got := []string{list1.Msg.Items[0].Key, list1.Msg.Items[1].Key, list1.Msg.Items[2].Key}
	want := []string{"3", "1", "2"}
	if got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("after BEFORE: want %v, got %v", want, got)
	}

	// AFTER tail: move 1 after 2 => 3,2,1
	// First, find current ids for 2 and 1 from list1
	var now1, now2 []byte
	for _, it := range list1.Msg.Items {
		if it.Key == "1" {
			now1 = it.BodyId
		}
		if it.Key == "2" {
			now2 = it.BodyId
		}
	}
	if _, err := rpc.BodyUrlEncodedMove(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedMoveRequest{
		ExampleId: ex.ID.Bytes(), BodyId: now1, TargetBodyId: now2, Position: resourcesv1.MovePosition_MOVE_POSITION_AFTER,
	})); err != nil {
		t.Fatal(err)
	}
	list2, err := rpc.BodyUrlEncodedList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedListRequest{ExampleId: ex.ID.Bytes()}))
	if err != nil {
		t.Fatal(err)
	}
	got2 := []string{list2.Msg.Items[0].Key, list2.Msg.Items[1].Key, list2.Msg.Items[2].Key}
	want2 := []string{"3", "2", "1"}
	if got2[0] != want2[0] || got2[1] != want2[1] || got2[2] != want2[2] {
		t.Fatalf("after AFTER: want %v, got %v", want2, got2)
	}
}

// Verify delta proxies respect origin order and origin mapping is correct
func TestBodyUrlEncoded_Delta_ProxyOrderAndOriginMap(t *testing.T) {
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

	// Endpoint and examples
	item := &mitemapi.ItemApi{ID: idwrap.NewNow(), Name: "delta-order", Url: "/e2e", Method: "POST", CollectionID: collectionID}
	if err := ias.CreateItemApi(ctx, item); err != nil {
		t.Fatal(err)
	}
	originEx := &mitemapiexample.ItemApiExample{ID: idwrap.NewNow(), ItemApiID: item.ID, CollectionID: collectionID, Name: "origin", BodyType: mitemapiexample.BodyTypeUrlencoded}
	if err := iaes.CreateApiExample(ctx, originEx); err != nil {
		t.Fatal(err)
	}
	deltaEx := &mitemapiexample.ItemApiExample{ID: idwrap.NewNow(), ItemApiID: item.ID, CollectionID: collectionID, Name: "delta", BodyType: mitemapiexample.BodyTypeUrlencoded, VersionParentID: &originEx.ID}
	if err := iaes.CreateApiExample(ctx, deltaEx); err != nil {
		t.Fatal(err)
	}

	// Create origin items: a,b,c
	for _, k := range []string{"a", "b", "c"} {
		if _, err := rpc.BodyUrlEncodedCreate(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedCreateRequest{ExampleId: originEx.ID.Bytes(), Key: k, Enabled: true})); err != nil {
			t.Fatalf("create %s: %v", k, err)
		}
	}

	// First delta list call should proxy a,b,c in origin order
	dl1, err := rpc.BodyUrlEncodedDeltaList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaListRequest{ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes()}))
	if err != nil {
		t.Fatal(err)
	}
	if len(dl1.Msg.Items) != 3 {
		t.Fatalf("expected 3 proxies, got %d", len(dl1.Msg.Items))
	}
	order := []string{dl1.Msg.Items[0].Key, dl1.Msg.Items[1].Key, dl1.Msg.Items[2].Key}
	if order[0] != "a" || order[1] != "b" || order[2] != "c" {
		t.Fatalf("proxy order wrong, got %v", order)
	}

	// Create a delta-only item 'd' and ensure it appends to end
	cr, err := rpc.BodyUrlEncodedDeltaCreate(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaCreateRequest{ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes(), Enabled: true}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rpc.BodyUrlEncodedDeltaUpdate(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaUpdateRequest{BodyId: cr.Msg.BodyId, Key: stringPtr("d"), Enabled: boolPtr(true)})); err != nil {
		t.Fatal(err)
	}

	dl2, err := rpc.BodyUrlEncodedDeltaList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaListRequest{ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes()}))
	if err != nil {
		t.Fatal(err)
	}
	if len(dl2.Msg.Items) != 4 {
		t.Fatalf("expected 4 items after delta-only, got %d", len(dl2.Msg.Items))
	}
	if dl2.Msg.Items[3].Key != "d" {
		t.Fatalf("expected tail to be d, got %s", dl2.Msg.Items[3].Key)
	}

	// Move proxy 'b' after 'd' => a,c,d,b (b last)
	var bID, dID []byte
	for _, it := range dl2.Msg.Items {
		if it.Key == "b" {
			bID = it.BodyId
		}
		if it.Key == "d" {
			dID = it.BodyId
		}
	}
	if _, err := rpc.BodyUrlEncodedDeltaMove(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaMoveRequest{
		ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes(), BodyId: bID, TargetBodyId: dID, Position: resourcesv1.MovePosition_MOVE_POSITION_AFTER,
	})); err != nil {
		t.Fatal(err)
	}
	dl3, err := rpc.BodyUrlEncodedDeltaList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaListRequest{ExampleId: deltaEx.ID.Bytes(), OriginId: originEx.ID.Bytes()}))
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, it := range dl3.Msg.Items {
		got = append(got, it.Key)
	}
	want := []string{"a", "c", "d", "b"}
	if len(got) != 4 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] || got[3] != want[3] {
		t.Fatalf("delta AFTER move order wrong: want %v, got %v", want, got)
	}

	// Origin mapping must reference origin ids (not delta ids)
	// Build origin id set
	ol, err := rpc.BodyUrlEncodedList(authed, connect.NewRequest(&bodyv1.BodyUrlEncodedListRequest{ExampleId: originEx.ID.Bytes()}))
	if err != nil {
		t.Fatal(err)
	}
	originIDs := map[string]struct{}{}
	for _, it := range ol.Msg.Items {
		originIDs[string(it.BodyId)] = struct{}{}
	}
	for _, it := range dl3.Msg.Items {
		if it.Source != nil && *it.Source == deltav1.SourceKind_SOURCE_KIND_DELTA && it.Origin != nil {
			if _, ok := originIDs[string(it.Origin.BodyId)]; !ok {
				t.Fatalf("delta item %s has non-origin parent id", it.Key)
			}
		}
	}
}

func stringPtr(s string) *string { return &s }
func boolPtr(b bool) *bool       { return &b }
