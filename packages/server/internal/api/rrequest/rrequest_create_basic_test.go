package rrequest_test

import (
	"context"
	"testing"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rrequest"
	"the-dev-tools/server/pkg/idwrap"
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

func TestBasicHeaderCreate(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	cs := scollection.New(queries, nil)
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

	// Create test API item
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

	// Create test example
	exampleID := idwrap.NewNow()
	itemExample := &mitemapiexample.ItemApiExample{
		ID:           exampleID,
		ItemApiID:    item.ID,
		CollectionID: CollectionID,
		Name:         "test_example",
		IsDefault:    true,
		BodyType:     mitemapiexample.BodyTypeRaw,
	}

	err = iaes.CreateApiExample(ctx, itemExample)
	if err != nil {
		t.Fatal(err)
	}

	rpcRequest := rrequest.New(db, cs, us, ias, iaes, ehs, eqs, as)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)

	// Test basic header creation
	req := connect.NewRequest(&requestv1.HeaderCreateRequest{
		ExampleId:   exampleID.Bytes(),
		Key:         "Test-Header",
		Enabled:     true,
		Value:       "test-value",
		Description: "Test header description",
	})

	resp, err := rpcRequest.HeaderCreate(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}

	if resp == nil {
		t.Fatal("response is nil")
	}

	if resp.Msg == nil {
		t.Fatal("response message is nil")
	}

	// Verify header was created
	headerID, err := idwrap.NewFromBytes(resp.Msg.HeaderId)
	if err != nil {
		t.Fatal(err)
	}

	// Fetch the created header to verify it exists and has correct values
	createdHeader, err := ehs.GetHeaderByID(ctx, headerID)
	if err != nil {
		t.Fatal(err)
	}

	if createdHeader.HeaderKey != "Test-Header" {
		t.Errorf("expected header key 'Test-Header', got '%s'", createdHeader.HeaderKey)
	}

	if createdHeader.Value != "test-value" {
		t.Errorf("expected header value 'test-value', got '%s'", createdHeader.Value)
	}

	if createdHeader.Description != "Test header description" {
		t.Errorf("expected description 'Test header description', got '%s'", createdHeader.Description)
	}

	if !createdHeader.Enable {
		t.Error("expected header to be enabled")
	}

	if createdHeader.ExampleID != exampleID {
		t.Error("header example ID doesn't match")
	}
}

func TestBasicQueryCreate(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	cs := scollection.New(queries, nil)
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

	// Create test API item
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

	// Create test example
	exampleID := idwrap.NewNow()
	itemExample := &mitemapiexample.ItemApiExample{
		ID:           exampleID,
		ItemApiID:    item.ID,
		CollectionID: CollectionID,
		Name:         "test_example",
		IsDefault:    true,
		BodyType:     mitemapiexample.BodyTypeRaw,
	}

	err = iaes.CreateApiExample(ctx, itemExample)
	if err != nil {
		t.Fatal(err)
	}

	rpcRequest := rrequest.New(db, cs, us, ias, iaes, ehs, eqs, as)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)

	// Test basic query creation
	req := connect.NewRequest(&requestv1.QueryCreateRequest{
		ExampleId:   exampleID.Bytes(),
		Key:         "test_param",
		Enabled:     true,
		Value:       "test_value",
		Description: "Test query parameter",
	})

	resp, err := rpcRequest.QueryCreate(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}

	if resp == nil {
		t.Fatal("response is nil")
	}

	if resp.Msg == nil {
		t.Fatal("response message is nil")
	}

	// Verify query was created
	queryID, err := idwrap.NewFromBytes(resp.Msg.QueryId)
	if err != nil {
		t.Fatal(err)
	}

	// Fetch the created query to verify it exists and has correct values
	createdQuery, err := eqs.GetExampleQuery(ctx, queryID)
	if err != nil {
		t.Fatal(err)
	}

	if createdQuery.QueryKey != "test_param" {
		t.Errorf("expected query key 'test_param', got '%s'", createdQuery.QueryKey)
	}

	if createdQuery.Value != "test_value" {
		t.Errorf("expected query value 'test_value', got '%s'", createdQuery.Value)
	}

	if createdQuery.Description != "Test query parameter" {
		t.Errorf("expected description 'Test query parameter', got '%s'", createdQuery.Description)
	}

	if !createdQuery.Enable {
		t.Error("expected query to be enabled")
	}

	if createdQuery.ExampleID != exampleID {
		t.Error("query example ID doesn't match")
	}
}

func TestMultipleHeaderCreation(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	cs := scollection.New(queries, nil)
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

	// Create test API item
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

	// Create test example
	exampleID := idwrap.NewNow()
	itemExample := &mitemapiexample.ItemApiExample{
		ID:           exampleID,
		ItemApiID:    item.ID,
		CollectionID: CollectionID,
		Name:         "test_example",
		IsDefault:    true,
		BodyType:     mitemapiexample.BodyTypeRaw,
	}

	err = iaes.CreateApiExample(ctx, itemExample)
	if err != nil {
		t.Fatal(err)
	}

	rpcRequest := rrequest.New(db, cs, us, ias, iaes, ehs, eqs, as)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)

	// Create multiple headers to test they can be created in sequence
	headers := []struct {
		key   string
		value string
		desc  string
	}{
		{"Authorization", "Bearer token123", "Auth header"},
		{"Content-Type", "application/json", "Content type header"},
		{"User-Agent", "test-client/1.0", "User agent header"},
	}

	var headerIDs []idwrap.IDWrap

	for _, header := range headers {
		req := connect.NewRequest(&requestv1.HeaderCreateRequest{
			ExampleId:   exampleID.Bytes(),
			Key:         header.key,
			Enabled:     true,
			Value:       header.value,
			Description: header.desc,
		})

		resp, err := rpcRequest.HeaderCreate(authedCtx, req)
		if err != nil {
			t.Fatalf("failed to create header %s: %v", header.key, err)
		}

		headerID, err := idwrap.NewFromBytes(resp.Msg.HeaderId)
		if err != nil {
			t.Fatal(err)
		}

		headerIDs = append(headerIDs, headerID)
	}

	// Verify all headers were created correctly
	if len(headerIDs) != 3 {
		t.Fatalf("expected 3 headers, got %d", len(headerIDs))
	}

	// Verify each header exists and has correct values
	for i, headerID := range headerIDs {
		createdHeader, err := ehs.GetHeaderByID(ctx, headerID)
		if err != nil {
			t.Fatalf("failed to get header %d: %v", i, err)
		}

		expectedHeader := headers[i]
		if createdHeader.HeaderKey != expectedHeader.key {
			t.Errorf("header %d: expected key '%s', got '%s'", i, expectedHeader.key, createdHeader.HeaderKey)
		}

		if createdHeader.Value != expectedHeader.value {
			t.Errorf("header %d: expected value '%s', got '%s'", i, expectedHeader.value, createdHeader.Value)
		}

		if createdHeader.Description != expectedHeader.desc {
			t.Errorf("header %d: expected description '%s', got '%s'", i, expectedHeader.desc, createdHeader.Description)
		}
	}
}