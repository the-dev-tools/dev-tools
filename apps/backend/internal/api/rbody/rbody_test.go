package rbody_test

import (
	"bytes"
	"context"
	"testing"
	"the-dev-tools/backend/internal/api/middleware/mwauth"
	"the-dev-tools/backend/internal/api/rbody"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mbodyform"
	"the-dev-tools/backend/pkg/model/mbodyraw"
	"the-dev-tools/backend/pkg/model/mbodyurl"
	"the-dev-tools/backend/pkg/model/mitemapi"
	"the-dev-tools/backend/pkg/model/mitemapiexample"
	"the-dev-tools/backend/pkg/service/sbodyform"
	"the-dev-tools/backend/pkg/service/sbodyraw"
	"the-dev-tools/backend/pkg/service/sbodyurl"
	"the-dev-tools/backend/pkg/service/scollection"
	"the-dev-tools/backend/pkg/service/sitemapi"
	"the-dev-tools/backend/pkg/service/sitemapiexample"
	"the-dev-tools/backend/pkg/service/suser"
	"the-dev-tools/backend/pkg/testutil"
	bodyv1 "the-dev-tools/spec/dist/buf/go/collection/item/body/v1"

	"connectrpc.com/connect"
)

func TestGetBodyRaw(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	us := suser.New(queries)
	cs := scollection.New(queries)
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
		ParentID:     nil,
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
		CompressType:  mbodyraw.CompressTypeNone,
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

	us := suser.New(queries)
	cs := scollection.New(queries)
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
		ParentID:     nil,
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

	req := connect.NewRequest(&bodyv1.BodyFormItemListRequest{
		ExampleId: itemExample.ID.Bytes(),
	})

	rpcBody := rbody.New(db, cs, iaes, us,
		bfs, bues, brs)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcBody.BodyFormItemList(authedCtx, req)
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
	for i := 0; i < formCount; i++ {
		if msg.Items[i].Description != formBodyArr[i].Description {
			t.Errorf("expected description %s, got %s", formBodyArr[i].Description, msg.Items[i].Description)
		}
		if msg.Items[i].Key != formBodyArr[i].BodyKey {
			t.Errorf("expected body key %s, got %s", formBodyArr[i].BodyKey, msg.Items[i].Key)
		}
		if msg.Items[i].Value != formBodyArr[i].Value {
			t.Errorf("expected value %s, got %s", formBodyArr[i].Value, msg.Items[i].Value)
		}
		if msg.Items[i].Enabled != formBodyArr[i].Enable {
			t.Errorf("expected enable %t, got %t", formBodyArr[i].Enable, msg.Items[i].Enabled)
		}
	}
}

func TestGetBodyUrlEncoded(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	us := suser.New(queries)
	cs := scollection.New(queries)
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
		ParentID:     nil,
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

	formBodyArr := make([]mbodyurl.BodyURLEncoded, formCount)

	for i := 0; i < formCount; i++ {
		formBodyArr[i] = mbodyurl.BodyURLEncoded{
			ID:          idwrap.NewNow(),
			Description: "test",
			BodyKey:     "test_key",
			Value:       "test_val",
			Enable:      true,
			ExampleID:   itemExample.ID,
		}
	}

	err = bues.CreateBulkBodyURLEncoded(ctx, formBodyArr)
	if err != nil {
		t.Fatal(err)
	}

	req := connect.NewRequest(&bodyv1.BodyUrlEncodedItemListRequest{
		ExampleId: itemExample.ID.Bytes(),
	})

	rpcBody := rbody.New(db, cs, iaes, us,
		bfs, bues, brs)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcBody.BodyUrlEncodedItemList(authedCtx, req)
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
	for i := 0; i < formCount; i++ {
		if msg.Items[i].Description != formBodyArr[i].Description {
			t.Errorf("expected description %s, got %s", formBodyArr[i].Description, msg.Items[i].Description)
		}
		if msg.Items[i].Key != formBodyArr[i].BodyKey {
			t.Errorf("expected body key %s, got %s", formBodyArr[i].BodyKey, msg.Items[i].Key)
		}
		if msg.Items[i].Value != formBodyArr[i].Value {
			t.Errorf("expected value %s, got %s", formBodyArr[i].Value, msg.Items[i].Value)
		}
		if msg.Items[i].Enabled != formBodyArr[i].Enable {
			t.Errorf("expected enable %t, got %t", formBodyArr[i].Enable, msg.Items[i].Enabled)
		}
	}
}
