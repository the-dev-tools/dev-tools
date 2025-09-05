package ritemapiexample_test

import (
	"bytes"
	"context"
	"testing"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/ritemapiexample"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logconsole"
	"the-dev-tools/server/pkg/logger/mocklogger"
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
	"the-dev-tools/server/pkg/service/sitemfolder"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/testutil"
	bodyv1 "the-dev-tools/spec/dist/buf/go/collection/item/body/v1"
	examplev1 "the-dev-tools/spec/dist/buf/go/collection/item/example/v1"
	resourcesv1 "the-dev-tools/spec/dist/buf/go/resources/v1"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mexamplequery"
)

func TestGetExampleApi(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	mockLogger := mocklogger.NewMockLogger()

	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	ifs := sitemfolder.New(queries)
	ws := sworkspace.New(queries)
	cs := scollection.New(queries, mockLogger)
	us := suser.New(queries)
	hs := sexampleheader.New(queries)
	qs := sexamplequery.New(queries)
	bfs := sbodyform.New(queries)
	bues := sbodyurl.New(queries)
	brs := sbodyraw.New(queries)
	ers := sexampleresp.New(queries)
	erhs := sexamplerespheader.New(queries)
	es := senv.New(queries, mockLogger)
	vs := svar.New(queries, mockLogger)
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

	logChanMap := logconsole.NewLogChanMapWith(10000)

	rpcExample := ritemapiexample.New(db, iaes, ias, ifs,
		ws, cs, us, hs, qs, bfs, bues, brs, erhs, ers, es, vs, as, ars, logChanMap)
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

	mockLogger := mocklogger.NewMockLogger()

	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	ifs := sitemfolder.New(queries)
	ws := sworkspace.New(queries)
	cs := scollection.New(queries, mockLogger)
	us := suser.New(queries)
	hs := sexampleheader.New(queries)
	qs := sexamplequery.New(queries)
	bfs := sbodyform.New(queries)
	bues := sbodyurl.New(queries)
	brs := sbodyraw.New(queries)
	ers := sexampleresp.New(queries)
	erhs := sexamplerespheader.New(queries)
	es := senv.New(queries, mockLogger)
	vs := svar.New(queries, mockLogger)
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

	logChanMap := logconsole.NewLogChanMapWith(10000)

	rpcExample := ritemapiexample.New(db, iaes, ias, ifs,
		ws, cs, us, hs, qs, bfs, bues, brs, erhs, ers, es, vs, as, ars, logChanMap)
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

// New test: when a default example exists, ExampleCreate should copy headers/queries/body/assertions
func TestExampleCreate_CopiesFromDefault(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	mockLogger := mocklogger.NewMockLogger()

	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	ifs := sitemfolder.New(queries)
	ws := sworkspace.New(queries)
	cs := scollection.New(queries, mockLogger)
	us := suser.New(queries)
	hs := sexampleheader.New(queries)
	qs := sexamplequery.New(queries)
	bfs := sbodyform.New(queries)
	bues := sbodyurl.New(queries)
	brs := sbodyraw.New(queries)
	ers := sexampleresp.New(queries)
	erhs := sexamplerespheader.New(queries)
	es := senv.New(queries, mockLogger)
	vs := svar.New(queries, mockLogger)
	as := sassert.New(queries)
	ars := sassertres.New(queries)

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	userID := idwrap.NewNow()

	base.GetBaseServices().CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)

	// Create endpoint
	endpoint := &mitemapi.ItemApi{
		ID:           idwrap.NewNow(),
		Name:         "E",
		Url:          "http://example.com",
		Method:       "GET",
		CollectionID: collectionID,
	}
	require.NoError(t, ias.CreateItemApi(ctx, endpoint))

	// Create a default example with one header, one query, raw body, and an assertion
	defaultExample := &mitemapiexample.ItemApiExample{
		ID:           idwrap.NewNow(),
		ItemApiID:    endpoint.ID,
		CollectionID: collectionID,
		Name:         "Default",
		BodyType:     mitemapiexample.BodyTypeRaw,
		IsDefault:    true,
	}
	require.NoError(t, iaes.CreateApiExample(ctx, defaultExample))

	// Raw body
	require.NoError(t, brs.CreateBodyRaw(ctx, mbodyraw.ExampleBodyRaw{ID: idwrap.NewNow(), ExampleID: defaultExample.ID, Data: []byte("{\"ok\":true}")}))
	// Header
	require.NoError(t, hs.AppendHeader(ctx, mexampleheader.Header{ID: idwrap.NewNow(), ExampleID: defaultExample.ID, HeaderKey: "X-Default", Value: "yes"}))
	// Query
	require.NoError(t, qs.CreateExampleQuery(ctx, mexamplequery.Query{ID: idwrap.NewNow(), ExampleID: defaultExample.ID, QueryKey: "q", Value: "1"}))
	// Assertion
	require.NoError(t, as.CreateAssert(ctx, massert.Assert{ID: idwrap.NewNow(), ExampleID: defaultExample.ID, Enable: true}))

	// Now create a new example via RPC; it should copy from default
	logChanMap := logconsole.NewLogChanMapWith(10000)
	rpcExample := ritemapiexample.New(db, iaes, ias, ifs, ws, cs, us, hs, qs, bfs, bues, brs, erhs, ers, es, vs, as, ars, logChanMap)
	authed := mwauth.CreateAuthedContext(ctx, userID)
	name := "Copied"
	req := connect.NewRequest(&examplev1.ExampleCreateRequest{EndpointId: endpoint.ID.Bytes(), Name: name})
	resp, err := rpcExample.ExampleCreate(authed, req)
	require.NoError(t, err)

	newID, err := idwrap.NewFromBytes(resp.Msg.ExampleId)
	require.NoError(t, err)

	// Verify copied data exists
	ex, err := iaes.GetApiExample(ctx, newID)
	require.NoError(t, err)
	require.Equal(t, name, ex.Name)

	// Header
	hdrs, err := hs.GetHeaderByExampleID(ctx, newID)
	require.NoError(t, err)
	require.NotEmpty(t, hdrs)
	// Query
	qsNew, err := qs.GetExampleQueriesByExampleID(ctx, newID)
	require.NoError(t, err)
	require.NotEmpty(t, qsNew)
	// Body raw
	body, err := brs.GetBodyRawByExampleID(ctx, newID)
	require.NoError(t, err)
	require.NotNil(t, body)
	// Assertions
	asserts, err := as.GetAssertByExampleID(ctx, newID)
	require.NoError(t, err)
	require.NotEmpty(t, asserts)

	// Verify example appears in ordered list (i.e., not invisible) and has proper linking
	ordered, err := iaes.GetApiExamplesOrdered(ctx, endpoint.ID)
	require.NoError(t, err)
	// Find new example in the ordered list
	var found *mitemapiexample.ItemApiExample
	for i := range ordered {
		if ordered[i].ID.Compare(newID) == 0 {
			found = &ordered[i]
			break
		}
	}
	require.NotNil(t, found, "new example should be present in ordered list")
	// Basic sanity: non-default examples should be in the linked list with at least one of Prev/Next possibly set
	// (if it's the only non-default example it can have both nil, but it's still listed)
	// Here we just assert that the list contained it, which proves visibility.
}

func TestUpdateExampleApi(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	mockLogger := mocklogger.NewMockLogger()

	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	ifs := sitemfolder.New(queries)
	ws := sworkspace.New(queries)
	cs := scollection.New(queries, mockLogger)
	us := suser.New(queries)
	hs := sexampleheader.New(queries)
	qs := sexamplequery.New(queries)
	bfs := sbodyform.New(queries)
	bues := sbodyurl.New(queries)
	brs := sbodyraw.New(queries)
	ers := sexampleresp.New(queries)
	erhs := sexamplerespheader.New(queries)
	es := senv.New(queries, mockLogger)
	vs := svar.New(queries, mockLogger)
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

	logChanMap := logconsole.NewLogChanMapWith(10000)

	rpcExample := ritemapiexample.New(db, iaes, ias, ifs,
		ws, cs, us, hs, qs, bfs, bues, brs, erhs, ers, es, vs, as, ars, logChanMap)
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

	mockLogger := mocklogger.NewMockLogger()

	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	ifs := sitemfolder.New(queries)
	ws := sworkspace.New(queries)
	cs := scollection.New(queries, mockLogger)
	us := suser.New(queries)
	hs := sexampleheader.New(queries)
	qs := sexamplequery.New(queries)
	bfs := sbodyform.New(queries)
	bues := sbodyurl.New(queries)
	brs := sbodyraw.New(queries)
	ers := sexampleresp.New(queries)
	erhs := sexamplerespheader.New(queries)
	es := senv.New(queries, mockLogger)
	vs := svar.New(queries, mockLogger)
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

	logChanMap := logconsole.NewLogChanMapWith(10000)

	rpcExample := ritemapiexample.New(db, iaes, ias, ifs,
		ws, cs, us, hs, qs, bfs, bues, brs, erhs, ers, es, vs, as, ars, logChanMap)
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

// Regression: deleting the middle example must preserve the chain and order of remaining items
func TestExampleDelete_MiddleMaintainsOrder(t *testing.T) {
    ctx := context.Background()
    base := testutil.CreateBaseDB(ctx, t)
    queries := base.Queries
    db := base.DB

    mockLogger := mocklogger.NewMockLogger()

    ias := sitemapi.New(queries)
    iaes := sitemapiexample.New(queries)
    ifs := sitemfolder.New(queries)
    ws := sworkspace.New(queries)
    cs := scollection.New(queries, mockLogger)
    us := suser.New(queries)
    hs := sexampleheader.New(queries)
    qs := sexamplequery.New(queries)
    bfs := sbodyform.New(queries)
    bues := sbodyurl.New(queries)
    brs := sbodyraw.New(queries)
    ers := sexampleresp.New(queries)
    erhs := sexamplerespheader.New(queries)
    es := senv.New(queries, mockLogger)
    vs := svar.New(queries, mockLogger)
    as := sassert.New(queries)
    ars := sassertres.New(queries)

    workspaceID := idwrap.NewNow()
    workspaceUserID := idwrap.NewNow()
    collectionID := idwrap.NewNow()
    userID := idwrap.NewNow()

    baseServices := base.GetBaseServices()
    baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)

    endpoint := &mitemapi.ItemApi{
        ID:           idwrap.NewNow(),
        Name:         "endpoint",
        Url:          "/ep",
        Method:       "GET",
        CollectionID: collectionID,
        FolderID:     nil,
    }
    require.NoError(t, ias.CreateItemApi(ctx, endpoint))

    // Create three user examples A -> B -> C
    a := &mitemapiexample.ItemApiExample{ID: idwrap.NewNow(), ItemApiID: endpoint.ID, CollectionID: collectionID, Name: "A"}
    b := &mitemapiexample.ItemApiExample{ID: idwrap.NewNow(), ItemApiID: endpoint.ID, CollectionID: collectionID, Name: "B"}
    c := &mitemapiexample.ItemApiExample{ID: idwrap.NewNow(), ItemApiID: endpoint.ID, CollectionID: collectionID, Name: "C"}
    require.NoError(t, iaes.CreateApiExample(ctx, a))
    require.NoError(t, iaes.CreateApiExample(ctx, b))
    require.NoError(t, iaes.CreateApiExample(ctx, c))

    // Sanity: ordered list is A,B,C
    {
        logChanMap := logconsole.NewLogChanMapWith(10000)
        rpc := ritemapiexample.New(db, iaes, ias, ifs, ws, cs, us, hs, qs, bfs, bues, brs, erhs, ers, es, vs, as, ars, logChanMap)
        authed := mwauth.CreateAuthedContext(ctx, userID)
        listResp, err := rpc.ExampleList(authed, connect.NewRequest(&examplev1.ExampleListRequest{EndpointId: endpoint.ID.Bytes()}))
        require.NoError(t, err)
        require.Len(t, listResp.Msg.Items, 3)
        require.Equal(t, "A", listResp.Msg.Items[0].Name)
        require.Equal(t, "B", listResp.Msg.Items[1].Name)
        require.Equal(t, "C", listResp.Msg.Items[2].Name)
    }

    // Delete the middle (B)
    {
        logChanMap := logconsole.NewLogChanMapWith(10000)
        rpc := ritemapiexample.New(db, iaes, ias, ifs, ws, cs, us, hs, qs, bfs, bues, brs, erhs, ers, es, vs, as, ars, logChanMap)
        authed := mwauth.CreateAuthedContext(ctx, userID)
        _, err := rpc.ExampleDelete(authed, connect.NewRequest(&examplev1.ExampleDeleteRequest{ExampleId: b.ID.Bytes()}))
        require.NoError(t, err)
    }

    // Validate chain is intact: A -> C, and listed order is A,C
    {
        // Ordered listing
        logChanMap := logconsole.NewLogChanMapWith(10000)
        rpc := ritemapiexample.New(db, iaes, ias, ifs, ws, cs, us, hs, qs, bfs, bues, brs, erhs, ers, es, vs, as, ars, logChanMap)
        authed := mwauth.CreateAuthedContext(ctx, userID)
        listResp, err := rpc.ExampleList(authed, connect.NewRequest(&examplev1.ExampleListRequest{EndpointId: endpoint.ID.Bytes()}))
        require.NoError(t, err)
        require.Len(t, listResp.Msg.Items, 2)
        require.Equal(t, "A", listResp.Msg.Items[0].Name)
        require.Equal(t, "C", listResp.Msg.Items[1].Name)

        // Direct pointer checks
        aRow, err := iaes.GetApiExample(ctx, a.ID)
        require.NoError(t, err)
        cRow, err := iaes.GetApiExample(ctx, c.ID)
        require.NoError(t, err)
        require.NotNil(t, aRow.Next, "A.next should point to C after deletion")
        require.Equal(t, c.ID.String(), aRow.Next.String())
        require.NotNil(t, cRow.Prev, "C.prev should point to A after deletion")
        require.Equal(t, a.ID.String(), cRow.Prev.String())
    }
}

func TestExampleDelete_HeadMaintainsOrder(t *testing.T) {
    ctx := context.Background()
    base := testutil.CreateBaseDB(ctx, t)
    queries := base.Queries
    db := base.DB

    mockLogger := mocklogger.NewMockLogger()
    ias := sitemapi.New(queries)
    iaes := sitemapiexample.New(queries)
    ifs := sitemfolder.New(queries)
    ws := sworkspace.New(queries)
    cs := scollection.New(queries, mockLogger)
    us := suser.New(queries)
    hs := sexampleheader.New(queries)
    qs := sexamplequery.New(queries)
    bfs := sbodyform.New(queries)
    bues := sbodyurl.New(queries)
    brs := sbodyraw.New(queries)
    ers := sexampleresp.New(queries)
    erhs := sexamplerespheader.New(queries)
    es := senv.New(queries, mockLogger)
    vs := svar.New(queries, mockLogger)
    as := sassert.New(queries)
    ars := sassertres.New(queries)

    workspaceID := idwrap.NewNow()
    workspaceUserID := idwrap.NewNow()
    collectionID := idwrap.NewNow()
    userID := idwrap.NewNow()
    baseServices := base.GetBaseServices()
    baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)

    endpoint := &mitemapi.ItemApi{ID: idwrap.NewNow(), Name: "endpoint", Url: "/ep", Method: "GET", CollectionID: collectionID}
    require.NoError(t, ias.CreateItemApi(ctx, endpoint))
    a := &mitemapiexample.ItemApiExample{ID: idwrap.NewNow(), ItemApiID: endpoint.ID, CollectionID: collectionID, Name: "A"}
    b := &mitemapiexample.ItemApiExample{ID: idwrap.NewNow(), ItemApiID: endpoint.ID, CollectionID: collectionID, Name: "B"}
    c := &mitemapiexample.ItemApiExample{ID: idwrap.NewNow(), ItemApiID: endpoint.ID, CollectionID: collectionID, Name: "C"}
    require.NoError(t, iaes.CreateApiExample(ctx, a))
    require.NoError(t, iaes.CreateApiExample(ctx, b))
    require.NoError(t, iaes.CreateApiExample(ctx, c))

    logChanMap := logconsole.NewLogChanMapWith(10000)
    rpc := ritemapiexample.New(db, iaes, ias, ifs, ws, cs, us, hs, qs, bfs, bues, brs, erhs, ers, es, vs, as, ars, logChanMap)
    authed := mwauth.CreateAuthedContext(ctx, userID)
    _, err := rpc.ExampleDelete(authed, connect.NewRequest(&examplev1.ExampleDeleteRequest{ExampleId: a.ID.Bytes()}))
    require.NoError(t, err)

    listResp, err := rpc.ExampleList(authed, connect.NewRequest(&examplev1.ExampleListRequest{EndpointId: endpoint.ID.Bytes()}))
    require.NoError(t, err)
    require.Len(t, listResp.Msg.Items, 2)
    require.Equal(t, "B", listResp.Msg.Items[0].Name)
    require.Equal(t, "C", listResp.Msg.Items[1].Name)
    bRow, _ := iaes.GetApiExample(ctx, b.ID)
    cRow, _ := iaes.GetApiExample(ctx, c.ID)
    require.Nil(t, bRow.Prev, "B.prev should be nil after deleting A")
    require.NotNil(t, bRow.Next)
    require.Equal(t, c.ID.String(), bRow.Next.String())
    require.NotNil(t, cRow.Prev)
    require.Equal(t, b.ID.String(), cRow.Prev.String())
}

func TestExampleDelete_TailMaintainsOrder(t *testing.T) {
    ctx := context.Background()
    base := testutil.CreateBaseDB(ctx, t)
    queries := base.Queries
    db := base.DB

    mockLogger := mocklogger.NewMockLogger()
    ias := sitemapi.New(queries)
    iaes := sitemapiexample.New(queries)
    ifs := sitemfolder.New(queries)
    ws := sworkspace.New(queries)
    cs := scollection.New(queries, mockLogger)
    us := suser.New(queries)
    hs := sexampleheader.New(queries)
    qs := sexamplequery.New(queries)
    bfs := sbodyform.New(queries)
    bues := sbodyurl.New(queries)
    brs := sbodyraw.New(queries)
    ers := sexampleresp.New(queries)
    erhs := sexamplerespheader.New(queries)
    es := senv.New(queries, mockLogger)
    vs := svar.New(queries, mockLogger)
    as := sassert.New(queries)
    ars := sassertres.New(queries)

    workspaceID := idwrap.NewNow()
    workspaceUserID := idwrap.NewNow()
    collectionID := idwrap.NewNow()
    userID := idwrap.NewNow()
    baseServices := base.GetBaseServices()
    baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)

    endpoint := &mitemapi.ItemApi{ID: idwrap.NewNow(), Name: "endpoint", Url: "/ep", Method: "GET", CollectionID: collectionID}
    require.NoError(t, ias.CreateItemApi(ctx, endpoint))
    a := &mitemapiexample.ItemApiExample{ID: idwrap.NewNow(), ItemApiID: endpoint.ID, CollectionID: collectionID, Name: "A"}
    b := &mitemapiexample.ItemApiExample{ID: idwrap.NewNow(), ItemApiID: endpoint.ID, CollectionID: collectionID, Name: "B"}
    c := &mitemapiexample.ItemApiExample{ID: idwrap.NewNow(), ItemApiID: endpoint.ID, CollectionID: collectionID, Name: "C"}
    require.NoError(t, iaes.CreateApiExample(ctx, a))
    require.NoError(t, iaes.CreateApiExample(ctx, b))
    require.NoError(t, iaes.CreateApiExample(ctx, c))

    logChanMap := logconsole.NewLogChanMapWith(10000)
    rpc := ritemapiexample.New(db, iaes, ias, ifs, ws, cs, us, hs, qs, bfs, bues, brs, erhs, ers, es, vs, as, ars, logChanMap)
    authed := mwauth.CreateAuthedContext(ctx, userID)
    _, err := rpc.ExampleDelete(authed, connect.NewRequest(&examplev1.ExampleDeleteRequest{ExampleId: c.ID.Bytes()}))
    require.NoError(t, err)

    listResp, err := rpc.ExampleList(authed, connect.NewRequest(&examplev1.ExampleListRequest{EndpointId: endpoint.ID.Bytes()}))
    require.NoError(t, err)
    require.Len(t, listResp.Msg.Items, 2)
    require.Equal(t, "A", listResp.Msg.Items[0].Name)
    require.Equal(t, "B", listResp.Msg.Items[1].Name)
    aRow, _ := iaes.GetApiExample(ctx, a.ID)
    bRow, _ := iaes.GetApiExample(ctx, b.ID)
    require.NotNil(t, aRow.Next)
    require.Equal(t, b.ID.String(), aRow.Next.String())
    require.Nil(t, bRow.Next, "B.next should be nil after deleting C")
}

func TestExampleMoveParameterValidation(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	db := base.DB

	mockLogger := mocklogger.NewMockLogger()

	ias := sitemapi.New(base.Queries)
	iaes := sitemapiexample.New(base.Queries)
	ifs := sitemfolder.New(base.Queries)
	ws := sworkspace.New(base.Queries)
	cs := scollection.New(base.Queries, mockLogger)
	us := suser.New(base.Queries)
	hs := sexampleheader.New(base.Queries)
	qs := sexamplequery.New(base.Queries)
	bfs := sbodyform.New(base.Queries)
	bues := sbodyurl.New(base.Queries)
	brs := sbodyraw.New(base.Queries)
	erhs := sexamplerespheader.New(base.Queries)
	ers := sexampleresp.New(base.Queries)
	es := senv.New(base.Queries, mockLogger)
	vs := svar.New(base.Queries, mockLogger)
	as := sassert.New(base.Queries)
	ars := sassertres.New(base.Queries)
	logChanMap := logconsole.NewLogChanMapWith(10000)

	rpcExample := ritemapiexample.New(db, iaes, ias, ifs,
		ws, cs, us, hs, qs, bfs, bues, brs, erhs, ers, es, vs, as, ars, logChanMap)

	tests := []struct {
		name        string
		request     *examplev1.ExampleMoveRequest
		expectedErr string
	}{
		{
			name: "invalid endpoint ID",
			request: &examplev1.ExampleMoveRequest{
				EndpointId:      []byte("invalid"),
				ExampleId:       idwrap.NewNow().Bytes(),
				Position:        resourcesv1.MovePosition_MOVE_POSITION_AFTER,
				TargetExampleId: idwrap.NewNow().Bytes(),
			},
			expectedErr: "invalid endpoint ID",
		},
		{
			name: "invalid example ID",
			request: &examplev1.ExampleMoveRequest{
				EndpointId:      idwrap.NewNow().Bytes(),
				ExampleId:       []byte("invalid"),
				Position:        resourcesv1.MovePosition_MOVE_POSITION_AFTER,
				TargetExampleId: idwrap.NewNow().Bytes(),
			},
			expectedErr: "invalid example ID",
		},
		{
			name: "invalid target example ID",
			request: &examplev1.ExampleMoveRequest{
				EndpointId:      idwrap.NewNow().Bytes(),
				ExampleId:       idwrap.NewNow().Bytes(),
				Position:        resourcesv1.MovePosition_MOVE_POSITION_AFTER,
				TargetExampleId: []byte("invalid"),
			},
			expectedErr: "invalid target example ID",
		},
		{
			name: "invalid position",
			request: &examplev1.ExampleMoveRequest{
				EndpointId:      idwrap.NewNow().Bytes(),
				ExampleId:       idwrap.NewNow().Bytes(),
				Position:        resourcesv1.MovePosition_MOVE_POSITION_UNSPECIFIED,
				TargetExampleId: idwrap.NewNow().Bytes(),
			},
			expectedErr: "invalid position: must be AFTER or BEFORE",
		},
		{
			name: "move example relative to itself",
			request: func() *examplev1.ExampleMoveRequest {
				exampleID := idwrap.NewNow()
				return &examplev1.ExampleMoveRequest{
					EndpointId:      idwrap.NewNow().Bytes(),
					ExampleId:       exampleID.Bytes(),
					Position:        resourcesv1.MovePosition_MOVE_POSITION_AFTER,
					TargetExampleId: exampleID.Bytes(), // Same as ExampleId
				}
			}(),
			expectedErr: "cannot move example relative to itself",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := connect.NewRequest(tt.request)
			_, err := rpcExample.ExampleMove(ctx, req)

			if err == nil {
				t.Fatal("expected error but got none")
			}

			connectErr := err.(*connect.Error)
			if connectErr.Message() != tt.expectedErr {
				t.Errorf("expected error %q, got %q", tt.expectedErr, connectErr.Message())
			}

			if connectErr.Code() != connect.CodeInvalidArgument {
				t.Errorf("expected code %v, got %v", connect.CodeInvalidArgument, connectErr.Code())
			}
		})
	}
}
