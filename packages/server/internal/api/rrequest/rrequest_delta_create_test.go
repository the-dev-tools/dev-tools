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

func TestHeaderDeltaCreate(t *testing.T) {
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

	// Create origin example
	originExampleID := idwrap.NewNow()
	originExample := &mitemapiexample.ItemApiExample{
		ID:           originExampleID,
		ItemApiID:    item.ID,
		CollectionID: CollectionID,
		Name:         "origin_example",
		IsDefault:    true,
		BodyType:     mitemapiexample.BodyTypeRaw,
	}

	err = iaes.CreateApiExample(ctx, originExample)
	if err != nil {
		t.Fatal(err)
	}

	// Create delta example
	deltaExampleID := idwrap.NewNow()
	deltaExample := &mitemapiexample.ItemApiExample{
		ID:           deltaExampleID,
		ItemApiID:    item.ID,
		CollectionID: CollectionID,
		Name:         "delta_example",
		IsDefault:    false,
		BodyType:     mitemapiexample.BodyTypeRaw,
	}

	err = iaes.CreateApiExample(ctx, deltaExample)
	if err != nil {
		t.Fatal(err)
	}

	rpcRequest := rrequest.New(db, cs, us, ias, iaes, ehs, eqs, as)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)

	// First create a header in the origin example to create a delta from
	originHeaderReq := connect.NewRequest(&requestv1.HeaderCreateRequest{
		ExampleId:   originExampleID.Bytes(),
		Key:         "Origin-Header",
		Enabled:     true,
		Value:       "origin-value",
		Description: "Origin header description",
	})

	originHeaderResp, err := rpcRequest.HeaderCreate(authedCtx, originHeaderReq)
	if err != nil {
		t.Fatal(err)
	}

	originHeaderID, err := idwrap.NewFromBytes(originHeaderResp.Msg.HeaderId)
	if err != nil {
		t.Fatal(err)
	}

	// Test delta header creation
	req := connect.NewRequest(&requestv1.HeaderDeltaCreateRequest{
		ExampleId:   deltaExampleID.Bytes(),
		OriginId:    originExampleID.Bytes(),
		HeaderId:    originHeaderID.Bytes(),
		Key:         "Delta-Header",
		Enabled:     true,
		Value:       "delta-value",
		Description: "Delta header description",
	})

	resp, err := rpcRequest.HeaderDeltaCreate(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}

	if resp == nil {
		t.Fatal("response is nil")
	}

	if resp.Msg == nil {
		t.Fatal("response message is nil")
	}

	// Verify delta header was created
	headerID, err := idwrap.NewFromBytes(resp.Msg.HeaderId)
	if err != nil {
		t.Fatal(err)
	}

	// Fetch the created header to verify it exists and has correct values
	createdHeader, err := ehs.GetHeaderByID(ctx, headerID)
	if err != nil {
		t.Fatal(err)
	}

	if createdHeader.HeaderKey != "Delta-Header" {
		t.Errorf("expected header key 'Delta-Header', got '%s'", createdHeader.HeaderKey)
	}

	if createdHeader.Value != "delta-value" {
		t.Errorf("expected header value 'delta-value', got '%s'", createdHeader.Value)
	}

	if createdHeader.Description != "Delta header description" {
		t.Errorf("expected description 'Delta header description', got '%s'", createdHeader.Description)
	}

	if !createdHeader.Enable {
		t.Error("expected header to be enabled")
	}

	if createdHeader.ExampleID != deltaExampleID {
		t.Error("header example ID should be delta example ID")
	}

	// Should have a delta parent ID set pointing to the origin header
	if createdHeader.DeltaParentID == nil {
		t.Error("expected header to have a delta parent ID")
	} else if *createdHeader.DeltaParentID != originHeaderID {
		t.Errorf("expected delta parent ID to be %s, got %s", originHeaderID, *createdHeader.DeltaParentID)
	}
}

func TestQueryDeltaCreate(t *testing.T) {
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

	// Create origin example
	originExampleID := idwrap.NewNow()
	originExample := &mitemapiexample.ItemApiExample{
		ID:           originExampleID,
		ItemApiID:    item.ID,
		CollectionID: CollectionID,
		Name:         "origin_example",
		IsDefault:    true,
		BodyType:     mitemapiexample.BodyTypeRaw,
	}

	err = iaes.CreateApiExample(ctx, originExample)
	if err != nil {
		t.Fatal(err)
	}

	// Create delta example
	deltaExampleID := idwrap.NewNow()
	deltaExample := &mitemapiexample.ItemApiExample{
		ID:           deltaExampleID,
		ItemApiID:    item.ID,
		CollectionID: CollectionID,
		Name:         "delta_example",
		IsDefault:    false,
		BodyType:     mitemapiexample.BodyTypeRaw,
	}

	err = iaes.CreateApiExample(ctx, deltaExample)
	if err != nil {
		t.Fatal(err)
	}

	rpcRequest := rrequest.New(db, cs, us, ias, iaes, ehs, eqs, as)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)

	// First create a query in the origin example to create a delta from
	originQueryReq := connect.NewRequest(&requestv1.QueryCreateRequest{
		ExampleId:   originExampleID.Bytes(),
		Key:         "origin_param",
		Enabled:     true,
		Value:       "origin_value",
		Description: "Origin query parameter",
	})

	originQueryResp, err := rpcRequest.QueryCreate(authedCtx, originQueryReq)
	if err != nil {
		t.Fatal(err)
	}

	originQueryID, err := idwrap.NewFromBytes(originQueryResp.Msg.QueryId)
	if err != nil {
		t.Fatal(err)
	}

	// Test delta query creation
	req := connect.NewRequest(&requestv1.QueryDeltaCreateRequest{
		ExampleId:   deltaExampleID.Bytes(),
		OriginId:    originExampleID.Bytes(),
		QueryId:     originQueryID.Bytes(),
		Key:         "delta_param",
		Enabled:     true,
		Value:       "delta_value",
		Description: "Delta query parameter",
	})

	resp, err := rpcRequest.QueryDeltaCreate(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}

	if resp == nil {
		t.Fatal("response is nil")
	}

	if resp.Msg == nil {
		t.Fatal("response message is nil")
	}

	// Verify delta query was created
	queryID, err := idwrap.NewFromBytes(resp.Msg.QueryId)
	if err != nil {
		t.Fatal(err)
	}

	// Fetch the created query to verify it exists and has correct values
	createdQuery, err := eqs.GetExampleQuery(ctx, queryID)
	if err != nil {
		t.Fatal(err)
	}

	if createdQuery.QueryKey != "delta_param" {
		t.Errorf("expected query key 'delta_param', got '%s'", createdQuery.QueryKey)
	}

	if createdQuery.Value != "delta_value" {
		t.Errorf("expected query value 'delta_value', got '%s'", createdQuery.Value)
	}

	if createdQuery.Description != "Delta query parameter" {
		t.Errorf("expected description 'Delta query parameter', got '%s'", createdQuery.Description)
	}

	if !createdQuery.Enable {
		t.Error("expected query to be enabled")
	}

	if createdQuery.ExampleID != deltaExampleID {
		t.Error("query example ID should be delta example ID")
	}

	// Should have a delta parent ID set pointing to the origin query
	if createdQuery.DeltaParentID == nil {
		t.Error("expected query to have a delta parent ID")
	} else if *createdQuery.DeltaParentID != originQueryID {
		t.Errorf("expected delta parent ID to be %s, got %s", originQueryID, *createdQuery.DeltaParentID)
	}
}