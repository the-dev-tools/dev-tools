package ritemapiexample_test

import (
	"bytes"
	"context"
	"testing"
	"the-dev-tools/backend/internal/api/middleware/mwauth"
	"the-dev-tools/backend/internal/api/ritemapiexample"
	"the-dev-tools/backend/pkg/dbtime"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mitemapi"
	"the-dev-tools/backend/pkg/model/mitemapiexample"
	"the-dev-tools/backend/pkg/service/sassert"
	"the-dev-tools/backend/pkg/service/sassertres"
	"the-dev-tools/backend/pkg/service/sbodyform"
	"the-dev-tools/backend/pkg/service/sbodyraw"
	"the-dev-tools/backend/pkg/service/sbodyurl"
	"the-dev-tools/backend/pkg/service/scollection"
	"the-dev-tools/backend/pkg/service/senv"
	"the-dev-tools/backend/pkg/service/sexampleheader"
	"the-dev-tools/backend/pkg/service/sexamplequery"
	"the-dev-tools/backend/pkg/service/sexampleresp"
	"the-dev-tools/backend/pkg/service/sexamplerespheader"
	"the-dev-tools/backend/pkg/service/sitemapi"
	"the-dev-tools/backend/pkg/service/sitemapiexample"
	"the-dev-tools/backend/pkg/service/sresultapi"
	"the-dev-tools/backend/pkg/service/suser"
	"the-dev-tools/backend/pkg/service/svar"
	"the-dev-tools/backend/pkg/testutil"
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
	ras := sresultapi.New(queries)
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
		ParentID:     nil,
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

	rpcExample := ritemapiexample.New(db, iaes, ias, ras,
		cs, us, hs, qs, bfs, bues, brs, erhs, ers, es, vs, as, ars)
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
	ras := sresultapi.New(queries)
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
		ParentID:     nil,
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

	rpcExample := ritemapiexample.New(db, iaes, ias, ras,
		cs, us, hs, qs, bfs, bues, brs, erhs, ers, es, vs, as, ars)
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
	ras := sresultapi.New(queries)
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
		ParentID:     nil,
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
		ExampleId:  expectedID.Bytes(),
		EndpointId: item.ID.Bytes(),
		Name:       &updatedName,
		BodyKind:   &updatedBodyType,
	})

	rpcExample := ritemapiexample.New(db, iaes, ias, ras,
		cs, us, hs, qs, bfs, bues, brs, erhs, ers, es, vs, as, ars)
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
	ras := sresultapi.New(queries)
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
		ParentID:     nil,
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
		ExampleId:  expectedID.Bytes(),
		EndpointId: item.ID.Bytes(),
	})

	rpcExample := ritemapiexample.New(db, iaes, ias, ras,
		cs, us, hs, qs, bfs, bues, brs, erhs, ers, es, vs, as, ars)
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
