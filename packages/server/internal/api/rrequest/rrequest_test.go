package rrequest_test

import (
	"context"
	"testing"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rrequest"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/model/massert"
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

	"connectrpc.com/connect"
)

/*
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

func TestRPCRequestAssertCreate(t *testing.T) {
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
	expectedPath := "test_path"
	expectedValue := "test_name"

	req := connect.NewRequest(&requestv1.AssertCreateRequest{
		ExampleId: exampleID.Bytes(),
		Path: []*referencev1.ReferenceKey{{
			Key:  expectedPath,
			Kind: referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_UNSPECIFIED,
		}},
		Value: expectedValue,
	})

	rpcExample := rrequest.New(db, cs, us, iaes, ehs, eqs, as)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcExample.AssertCreate(authedCtx, req)
	testutil.AssertFatal(t, nil, err)
	testutil.AssertNotFatal(t, nil, resp)
	testutil.AssertNotFatal(t, nil, resp.Msg)
	msg := resp.Msg
	id, err := idwrap.NewFromBytes(msg.AssertId)
	testutil.AssertFatal(t, nil, err)
	assert, err := as.GetAssert(ctx, id)
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, expectedPath, assert.Path)
	testutil.Assert(t, expectedValue, assert.Value)
	testutil.Assert(t, exampleID, assert.ExampleID)
}

func TestRPCRequestAssertGet(t *testing.T) {
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

	// TODO: improve this test
	const expectedPath = "test_path"
	const expectedValue = "test_name"
	const expectedEnable = true
	const expectedType = massert.AssertTypeEqual

	assert := massert.Assert{
		ID:        idwrap.NewNow(),
		ExampleID: exampleID,
		Path:      expectedPath,
		Value:     expectedValue,
		Type:      expectedType,
		Enable:    expectedEnable,
	}

	err = as.CreateAssert(ctx, assert)
	testutil.AssertFatal(t, nil, err)

	req := connect.NewRequest(&requestv1.AssertListRequest{
		ExampleId: exampleID.Bytes(),
	})

	rpcExample := rrequest.New(db, cs, us, iaes, ehs, eqs, as)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcExample.AssertList(authedCtx, req)
	testutil.AssertFatal(t, nil, err)
	testutil.AssertNotFatal(t, nil, resp)
	testutil.AssertNotFatal(t, nil, resp.Msg)
	msg := resp.Msg
	for _, item := range msg.Items {
		testutil.Assert(t, expectedValue, item.Value)
	}
}

func TestRPCRequestAssertUpdate(t *testing.T) {
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

	assert := massert.Assert{
		ID:        idwrap.NewNow(),
		ExampleID: exampleID,
		Path:      "test_path",
		Value:     "test_name",
		Type:      massert.AssertTypeEqual,
		Enable:    true,
	}

	err = as.CreateAssert(ctx, assert)
	testutil.AssertFatal(t, nil, err)

	// TODO: add path to the request
	//
	// const updatedPath = "updated_path"
	const updatedValue = "updated_name"
	const updatedType = massert.AssertTypeEqual

	req := connect.NewRequest(&requestv1.AssertUpdateRequest{
		AssertId: assert.ID.Bytes(),
		Condition: &conditionv1.Condition{
			Comparison: &conditionv1.Comparison{
				Path:  []*referencev1.ReferenceKey{},
				Kind:  conditionv1.ComparisonKind_COMPARISON_KIND_EQUAL,
				Value: "aaaa",
			},
		},
	})

	rpcExample := rrequest.New(db, cs, us, iaes, ehs, eqs, as)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcExample.AssertUpdate(authedCtx, req)
	testutil.AssertFatal(t, nil, err)
	testutil.AssertNotFatal(t, nil, resp)
	testutil.AssertNotFatal(t, nil, resp.Msg)
	updatedAssert, err := as.GetAssert(ctx, assert.ID)
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, updatedValue, updatedAssert.Value)
	testutil.Assert(t, updatedType, updatedAssert.Type)
	testutil.Assert(t, exampleID, updatedAssert.ExampleID)
	testutil.Assert(t, assert.Enable, updatedAssert.Enable)
}
*/

func TestRPCRequestAssertDelete(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	mockLogger := mocklogger.NewMockLogger()

	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	cs := scollection.New(queries, mockLogger)
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

	assert := massert.Assert{
		ID:        idwrap.NewNow(),
		ExampleID: exampleID,
		Path:      "test_path",
		Value:     "test_name",
		Type:      massert.AssertTypeEqual,
		Enable:    true,
	}

	err = as.CreateAssert(ctx, assert)
	testutil.AssertFatal(t, nil, err)

	req := connect.NewRequest(&requestv1.AssertDeleteRequest{
		AssertId: assert.ID.Bytes(),
	})

	rpcExample := rrequest.New(db, cs, us, iaes, ehs, eqs, as)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcExample.AssertDelete(authedCtx, req)
	testutil.AssertFatal(t, nil, err)
	testutil.AssertNotFatal(t, nil, resp)
	testutil.AssertNotFatal(t, nil, resp.Msg)
	updatedAssert, err := as.GetAssert(ctx, assert.ID)
	testutil.Assert(t, sassert.ErrNoAssertFound, err)
	testutil.Assert(t, nil, updatedAssert)
}
