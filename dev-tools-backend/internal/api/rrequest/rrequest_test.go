package rrequest_test

import (
	"context"
	"dev-tools-backend/internal/api/middleware/mwauth"
	"dev-tools-backend/internal/api/rrequest"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mexampleheader"
	"dev-tools-backend/pkg/model/mitemapi"
	"dev-tools-backend/pkg/model/mitemapiexample"
	"dev-tools-backend/pkg/service/sassert"
	"dev-tools-backend/pkg/service/scollection"
	"dev-tools-backend/pkg/service/sexampleheader"
	"dev-tools-backend/pkg/service/sexamplequery"
	"dev-tools-backend/pkg/service/sitemapi"
	"dev-tools-backend/pkg/service/sitemapiexample"
	"dev-tools-backend/pkg/service/suser"
	"dev-tools-backend/pkg/testutil"
	requestv1 "dev-tools-spec/dist/buf/go/collection/item/request/v1"
	"fmt"
	"testing"

	"connectrpc.com/connect"
)

func TestRPCRequestHeaderCreate(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	cs := scollection.New(queries)
	us := suser.New(queries)
	ehs := sexampleheader.New(queries)
	eqs := sexamplequery.New(queries)

	as := sassert.New(queries)

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

	expectedItemApiID := item.ID
	exampleID := idwrap.NewNow()

	itemExample := &mitemapiexample.ItemApiExample{
		ID:           exampleID,
		ItemApiID:    expectedItemApiID,
		CollectionID: CollectionID,
		Name:         "item_example_name",
	}

	err = iaes.CreateApiExample(ctx, itemExample)
	if err != nil {
		t.Fatal(err)
	}

	expectedHeaderKey := "test_key"
	expectedEnable := true
	expectedValue := "test_value"
	expectedDescription := "test_description"

	req := connect.NewRequest(&requestv1.HeaderCreateRequest{
		ExampleId:   exampleID.Bytes(),
		Key:         expectedHeaderKey,
		Enabled:     expectedEnable,
		Value:       expectedValue,
		Description: expectedDescription,
	})

	rpcExample := rrequest.New(db, cs, us, iaes, ehs, eqs, as)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcExample.HeaderCreate(authedCtx, req)
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

	id, err := idwrap.NewFromBytes(msg.HeaderId)
	if err != nil {
		t.Error(err)
	}

	header, err := ehs.GetHeaderByID(ctx, id)
	if err != nil {
		t.Error(err)
	}

	testutil.Assert(t, expectedHeaderKey, header.HeaderKey)
	testutil.Assert(t, expectedEnable, header.Enable)
	testutil.Assert(t, expectedValue, header.Value)
	testutil.Assert(t, expectedDescription, header.Description)
}

func TestRPCRequestHeaderList(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	cs := scollection.New(queries)
	us := suser.New(queries)
	ehs := sexampleheader.New(queries)
	eqs := sexamplequery.New(queries)

	as := sassert.New(queries)

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

	expectedItemApiID := item.ID
	exampleID := idwrap.NewNow()

	itemExample := &mitemapiexample.ItemApiExample{
		ID:           exampleID,
		ItemApiID:    expectedItemApiID,
		CollectionID: CollectionID,
		Name:         "item_example_name",
	}

	err = iaes.CreateApiExample(ctx, itemExample)
	if err != nil {
		t.Fatal(err)
	}

	var exampleHeaderArray []mexampleheader.Header
	const exampleHeaderCount = 3
	for i := 0; i < exampleHeaderCount; i++ {
		exampleHeaderArray = append(exampleHeaderArray, mexampleheader.Header{
			ID:          idwrap.NewNow(),
			ExampleID:   exampleID,
			HeaderKey:   fmt.Sprintf("test_key_%d", i),
			Enable:      i%2 == 0,
			Value:       fmt.Sprintf("test_value_%d", i),
			Description: fmt.Sprintf("test_description_%d", i),
		})
	}

	for _, exampleHeader := range exampleHeaderArray {
		err = ehs.CreateHeader(ctx, exampleHeader)
		if err != nil {
			t.Fatal(err)
		}
	}

	req := connect.NewRequest(&requestv1.HeaderListRequest{
		ExampleId: exampleID.Bytes(),
	})

	rpcExample := rrequest.New(db, cs, us, iaes, ehs, eqs, as)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcExample.HeaderList(authedCtx, req)
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
	testutil.Assert(t, exampleHeaderCount, len(msg.Items))
	for i, header := range msg.Items {
		expectedHeader := exampleHeaderArray[i]

		testutil.Assert(t, expectedHeader.HeaderKey, header.Key)
		testutil.Assert(t, expectedHeader.Enable, header.Enabled)
		testutil.Assert(t, expectedHeader.Value, header.Value)
		testutil.Assert(t, expectedHeader.Description, header.Description)
	}
}

func TestRPCRequestHeaderUpdate(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	cs := scollection.New(queries)
	us := suser.New(queries)
	ehs := sexampleheader.New(queries)
	eqs := sexamplequery.New(queries)

	as := sassert.New(queries)

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

	expectedItemApiID := item.ID
	exampleID := idwrap.NewNow()

	itemExample := &mitemapiexample.ItemApiExample{
		ID:           exampleID,
		ItemApiID:    expectedItemApiID,
		CollectionID: CollectionID,
		Name:         "item_example_name",
	}

	err = iaes.CreateApiExample(ctx, itemExample)
	if err != nil {
		t.Fatal(err)
	}
	headerID := idwrap.NewNow()
	header := mexampleheader.Header{
		ID:          headerID,
		ExampleID:   exampleID,
		HeaderKey:   "test_key",
		Enable:      true,
		Value:       "test_value",
		Description: "test_description",
	}

	err = ehs.CreateHeader(ctx, header)
	if err != nil {
		t.Fatal(err)
	}

	updatedKey := "updated_key"
	updatedEnabled := false
	updatedValue := "updated_val"
	updatedDesc := "some_disc"

	req := connect.NewRequest(&requestv1.HeaderUpdateRequest{
		HeaderId:    headerID.Bytes(),
		Key:         updatedKey,
		Enabled:     updatedEnabled,
		Value:       updatedValue,
		Description: updatedDesc,
	})

	rpcExample := rrequest.New(db, cs, us, iaes, ehs, eqs, as)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcExample.HeaderUpdate(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}

	if resp == nil {
		t.Fatal("resp is nil")
	}

	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}

	header, err = ehs.GetHeaderByID(ctx, headerID)
	if err != nil {
		t.Error(err)
	}

	testutil.Assert(t, updatedKey, header.HeaderKey)
	testutil.Assert(t, updatedEnabled, header.Enable)
	testutil.Assert(t, updatedValue, header.Value)
	testutil.Assert(t, updatedDesc, header.Description)
}

func TestRPCRequestHeaderDelete(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	cs := scollection.New(queries)
	us := suser.New(queries)
	ehs := sexampleheader.New(queries)
	eqs := sexamplequery.New(queries)

	as := sassert.New(queries)

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

	expectedItemApiID := item.ID
	exampleID := idwrap.NewNow()

	itemExample := &mitemapiexample.ItemApiExample{
		ID:           exampleID,
		ItemApiID:    expectedItemApiID,
		CollectionID: CollectionID,
		Name:         "item_example_name",
	}

	err = iaes.CreateApiExample(ctx, itemExample)
	if err != nil {
		t.Fatal(err)
	}
	headerID := idwrap.NewNow()
	header := mexampleheader.Header{
		ID:          headerID,
		ExampleID:   exampleID,
		HeaderKey:   "test_key",
		Enable:      true,
		Value:       "test_value",
		Description: "test_description",
	}

	err = ehs.CreateHeader(ctx, header)
	if err != nil {
		t.Fatal(err)
	}

	req := connect.NewRequest(&requestv1.HeaderDeleteRequest{
		HeaderId: headerID.Bytes(),
	})

	rpcExample := rrequest.New(db, cs, us, iaes, ehs, eqs, as)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcExample.HeaderDelete(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}

	if resp == nil {
		t.Fatal("resp is nil")
	}

	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}

	header, err = ehs.GetHeaderByID(ctx, headerID)
	testutil.Assert(t, sexampleheader.ErrNoHeaderFound, err)
	testutil.Assert(t, mexampleheader.Header{}, header)
}
