package ritemapiexample_test

import (
	"bytes"
	"context"
	"testing"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/ritemapiexample"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/service/sassert"
	"the-dev-tools/server/pkg/service/sassertres"
	"the-dev-tools/server/pkg/service/sbodyform"
	"the-dev-tools/server/pkg/service/sbodyraw"
	"the-dev-tools/server/pkg/service/sbodyurl"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/sexampleheader"
	"the-dev-tools/server/pkg/service/sexamplequery"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sexamplerespheader"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/testutil"
	bodyv1 "the-dev-tools/spec/dist/buf/go/collection/item/body/v1"
	examplev1 "the-dev-tools/spec/dist/buf/go/collection/item/example/v1"

	"connectrpc.com/connect"
)

func TestGetExampleApi(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	ws := sworkspace.New(queries)
	cs := scollection.New(queries)
	us := suser.New(queries)
	hs := sexampleheader.New(queries)
	qs := sexamplequery.New(queries)
	bfs := sbodyform.New(queries)
	bues := sbodyurl.New(queries)
	brs := sbodyraw.New(queries)
	ers := sexampleresp.New(queries)
	erhs := sexamplerespheader.New(queries)
	es := senv.New(queries)
	vs := svar.New(queries)
	as := sassert.New(queries)
	ars := sassertres.New(queries)

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

	expectedName := "test_name"
	expectedUpdatedAt := dbtime.DBNow()
	expectedIsDefault := true
	expectedBodyType := mitemapiexample.BodyTypeRaw
	expectedItemApiID := item.ID
	expectedCollectionID := CollectionID
	expectedID := idwrap.NewNow()

	itemExample := &mitemapiexample.ItemApiExample{
		ID:           expectedID,
		ItemApiID:    expectedItemApiID,
		CollectionID: expectedCollectionID,
		Name:         expectedName,
		Updated:      expectedUpdatedAt,
		IsDefault:    expectedIsDefault,
		BodyType:     expectedBodyType,
	}

	err = iaes.CreateApiExample(ctx, itemExample)
	if err != nil {
		t.Fatal(err)
	}

	req := connect.NewRequest(&examplev1.ExampleGetRequest{
		ExampleId: expectedID.Bytes(),
	})

	rpcExample := ritemapiexample.New(db, iaes, ias,
		ws, cs, us, hs, qs, bfs, bues, brs, erhs, ers, es, vs, as, ars)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcExample.ExampleGet(authedCtx, req)
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
	if !bytes.Equal(msg.ExampleId, expectedID.Bytes()) {
		t.Errorf("expected body %s, got %s", expectedID.Bytes(), msg.ExampleId)
	}

	if msg.Name != expectedName {
		t.Errorf("expected body %s, got %s", expectedName, msg.Name)
	}

	if msg.BodyKind != bodyv1.BodyKind(expectedBodyType) {
		t.Errorf("expected body %d, got %d", expectedBodyType, msg.BodyKind)
	}
}

func TestCreateExampleApi(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	ws := sworkspace.New(queries)
	cs := scollection.New(queries)
	us := suser.New(queries)
	hs := sexampleheader.New(queries)
	qs := sexamplequery.New(queries)
	bfs := sbodyform.New(queries)
	bues := sbodyurl.New(queries)
	brs := sbodyraw.New(queries)
	ers := sexampleresp.New(queries)
	erhs := sexamplerespheader.New(queries)
	es := senv.New(queries)
	vs := svar.New(queries)
	as := sassert.New(queries)
	ars := sassertres.New(queries)

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

	expectedName := "test_name"
	expectedBodyType := bodyv1.BodyKind_BODY_KIND_RAW

	req := connect.NewRequest(&examplev1.ExampleCreateRequest{
		EndpointId: item.ID.Bytes(),
		Name:       expectedName,
		BodyKind:   expectedBodyType,
	})

	rpcExample := ritemapiexample.New(db, iaes, ias,
		ws, cs, us, hs, qs, bfs, bues, brs, erhs, ers, es, vs, as, ars)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcExample.ExampleCreate(authedCtx, req)
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
	exampleID, err := idwrap.NewFromBytes(msg.ExampleId)
	if err != nil {
		t.Fatal(err)
	}

	example, err := iaes.GetApiExample(ctx, exampleID)
	if err != nil {
		t.Fatal(err)
	}

	if example.Name != expectedName {
		t.Errorf("expected body %s, got %s", expectedName, example.Name)
	}

	// TODO: add bodykind to rpc
	/*
		if bodyv1.BodyKind(example.BodyType) != expectedBodyType {
			fmt.Println(bodyv1.BodyKind(example.BodyType))
			fmt.Println(expectedBodyType)
			t.Error("body type is not same")
		}
	*/
}

func TestUpdateExampleApi(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	ws := sworkspace.New(queries)
	cs := scollection.New(queries)
	us := suser.New(queries)
	hs := sexampleheader.New(queries)
	qs := sexamplequery.New(queries)
	bfs := sbodyform.New(queries)
	bues := sbodyurl.New(queries)
	brs := sbodyraw.New(queries)
	ers := sexampleresp.New(queries)
	erhs := sexamplerespheader.New(queries)
	es := senv.New(queries)
	vs := svar.New(queries)
	as := sassert.New(queries)
	ars := sassertres.New(queries)

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

	expectedName := "test_name"
	expectedUpdatedAt := dbtime.DBNow()
	expectedIsDefault := true
	expectedBodyType := mitemapiexample.BodyTypeRaw
	expectedItemApiID := item.ID
	expectedCollectionID := CollectionID
	expectedID := idwrap.NewNow()

	itemExample := &mitemapiexample.ItemApiExample{
		ID:           expectedID,
		ItemApiID:    expectedItemApiID,
		CollectionID: expectedCollectionID,
		Name:         expectedName,
		Updated:      expectedUpdatedAt,
		IsDefault:    expectedIsDefault,
		BodyType:     expectedBodyType,
	}

	err = iaes.CreateApiExample(ctx, itemExample)
	if err != nil {
		t.Fatal(err)
	}

	updatedName := "updated_name"
	updatedBodyType := bodyv1.BodyKind_BODY_KIND_RAW

	req := connect.NewRequest(&examplev1.ExampleUpdateRequest{
		ExampleId: expectedID.Bytes(),
		Name:      &updatedName,
		BodyKind:  &updatedBodyType,
	})

	rpcExample := ritemapiexample.New(db, iaes, ias,
		ws, cs, us, hs, qs, bfs, bues, brs, erhs, ers, es, vs, as, ars)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcExample.ExampleUpdate(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}

	if resp == nil {
		t.Fatal("resp is nil")
	}

	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}
	example, err := iaes.GetApiExample(ctx, expectedID)
	if err != nil {
		t.Fatal(err)
	}

	if example.Name != updatedName {
		t.Errorf("expected body %s, got %s", expectedName, example.Name)
	}

	// TODO: add bodykind
}

func TestDeleteExampleApi(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	ws := sworkspace.New(queries)
	cs := scollection.New(queries)
	us := suser.New(queries)
	hs := sexampleheader.New(queries)
	qs := sexamplequery.New(queries)
	bfs := sbodyform.New(queries)
	bues := sbodyurl.New(queries)
	brs := sbodyraw.New(queries)
	ers := sexampleresp.New(queries)
	erhs := sexamplerespheader.New(queries)
	es := senv.New(queries)
	vs := svar.New(queries)
	as := sassert.New(queries)
	ars := sassertres.New(queries)

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

	expectedName := "test_name"
	expectedUpdatedAt := dbtime.DBNow()
	expectedIsDefault := true
	expectedBodyType := mitemapiexample.BodyTypeRaw
	expectedItemApiID := item.ID
	expectedCollectionID := CollectionID
	expectedID := idwrap.NewNow()

	itemExample := &mitemapiexample.ItemApiExample{
		ID:           expectedID,
		ItemApiID:    expectedItemApiID,
		CollectionID: expectedCollectionID,
		Name:         expectedName,
		Updated:      expectedUpdatedAt,
		IsDefault:    expectedIsDefault,
		BodyType:     expectedBodyType,
	}

	err = iaes.CreateApiExample(ctx, itemExample)
	if err != nil {
		t.Fatal(err)
	}

	req := connect.NewRequest(&examplev1.ExampleDeleteRequest{
		ExampleId: expectedID.Bytes(),
	})

	rpcExample := ritemapiexample.New(db, iaes, ias,
		ws, cs, us, hs, qs, bfs, bues, brs, erhs, ers, es, vs, as, ars)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcExample.ExampleDelete(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}

	if resp == nil {
		t.Fatal("resp is nil")
	}

	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}
	example, err := iaes.GetApiExample(ctx, expectedID)
	if err == nil {
		t.Error("expected error, got nil")
	}
	if err != sitemapiexample.ErrNoItemApiExampleFound {
		t.Errorf("expected error %s, got %s", sitemapiexample.ErrNoItemApiExampleFound, err)
	}
	if example != nil {
		t.Errorf("expected nil, got %v", example)
	}
}

func TestPrepareCopyExample(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries

	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	hs := sexampleheader.New(queries)
	qs := sexamplequery.New(queries)
	bfs := sbodyform.New(queries)
	bues := sbodyurl.New(queries)
	brs := sbodyraw.New(queries)
	ers := sexampleresp.New(queries)
	erhs := sexamplerespheader.New(queries)
	as := sassert.New(queries)
	ars := sassertres.New(queries)

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

	// Create original example
	originalExample := &mitemapiexample.ItemApiExample{
		ID:           idwrap.NewNow(),
		ItemApiID:    item.ID,
		CollectionID: CollectionID,
		Name:         "Original Example",
		Updated:      dbtime.DBNow(),
		IsDefault:    true,
		BodyType:     mitemapiexample.BodyTypeRaw,
	}

	err = iaes.CreateApiExample(ctx, originalExample)
	if err != nil {
		t.Fatal(err)
	}

	// Add a header to original example
	header := mexampleheader.Header{
		ID:        idwrap.NewNow(),
		ExampleID: originalExample.ID,
		HeaderKey: "TestHeader",
		Value:     "TestValue",
	}
	err = hs.CreateHeader(ctx, header)
	if err != nil {
		t.Fatal(err)
	}

	// Create new item for copy
	newItem := &mitemapi.ItemApi{
		ID:           idwrap.NewNow(),
		Name:         "new test",
		Url:          "test",
		Method:       "GET",
		CollectionID: CollectionID,
		FolderID:     nil,
	}

	err = ias.CreateItemApi(ctx, newItem)
	if err != nil {
		t.Fatal(err)
	}

	// Test PrepareCopyExample
	result, err := ritemapiexample.PrepareCopyExample(ctx, newItem.ID, *originalExample, hs, qs, brs, bfs, bues, ers, erhs, as, ars)
	if err != nil {
		t.Fatal(err)
	}

	// Verify copied example
	if result.Example.Name != originalExample.Name+" - Copy" {
		t.Errorf("expected name %s, got %s", originalExample.Name+" - Copy", result.Example.Name)
	}

	if result.Example.ItemApiID != newItem.ID {
		t.Error("ItemApiID not properly set")
	}

	if result.Example.CollectionID != CollectionID {
		t.Error("CollectionID not properly set")
	}

	if result.Example.BodyType != originalExample.BodyType {
		t.Error("BodyType not properly copied")
	}

	// Verify copied headers
	if len(result.Headers) != 1 {
		t.Fatalf("expected 1 header, got %d", len(result.Headers))
	}

	copiedHeader := result.Headers[0]
	if copiedHeader.HeaderKey != header.HeaderKey {
		t.Errorf("expected header key %s, got %s", header.HeaderKey, copiedHeader.HeaderKey)
	}

	if copiedHeader.Value != header.Value {
		t.Errorf("expected header value %s, got %s", header.Value, copiedHeader.Value)
	}

	if copiedHeader.ID == header.ID {
		t.Error("header ID should be different")
	}

	// Verify header has correct ExampleID
	if copiedHeader.ExampleID != result.Example.ID {
		t.Errorf("header ExampleID %s does not match new example ID %s",
			copiedHeader.ExampleID, result.Example.ID)
	}

	// Verify old header's ExampleID still points to original example
	if header.ExampleID != originalExample.ID {
		t.Error("original header's ExampleID was modified")
	}

	// Verify new header's ExampleID matches the new example's ID
	if copiedHeader.ExampleID != result.Example.ID {
		t.Errorf("new header's ExampleID %s does not match new example ID %s",
			copiedHeader.ExampleID, result.Example.ID)
	}
}
