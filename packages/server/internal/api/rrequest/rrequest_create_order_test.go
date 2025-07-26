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

func TestHeaderCreateMaintainsOrder(t *testing.T) {
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

	// Create three headers using the RPC endpoints to test ordering
	req1 := connect.NewRequest(&requestv1.HeaderCreateRequest{
		ExampleId:   exampleID.Bytes(),
		Key:         "Header1",
		Enabled:     true,
		Value:       "Value1",
		Description: "First header",
	})

	resp1, err := rpcRequest.HeaderCreate(authedCtx, req1)
	if err != nil {
		t.Fatal(err)
	}

	req2 := connect.NewRequest(&requestv1.HeaderCreateRequest{
		ExampleId:   exampleID.Bytes(),
		Key:         "Header2",
		Enabled:     true,
		Value:       "Value2",
		Description: "Second header",
	})

	resp2, err := rpcRequest.HeaderCreate(authedCtx, req2)
	if err != nil {
		t.Fatal(err)
	}

	req3 := connect.NewRequest(&requestv1.HeaderCreateRequest{
		ExampleId:   exampleID.Bytes(),
		Key:         "Header3",
		Enabled:     true,
		Value:       "Value3",
		Description: "Third header",
	})

	resp3, err := rpcRequest.HeaderCreate(authedCtx, req3)
	if err != nil {
		t.Fatal(err)
	}

	// Get the created header IDs
	header1ID, err := idwrap.NewFromBytes(resp1.Msg.HeaderId)
	if err != nil {
		t.Fatal(err)
	}

	header2ID, err := idwrap.NewFromBytes(resp2.Msg.HeaderId)
	if err != nil {
		t.Fatal(err)
	}

	header3ID, err := idwrap.NewFromBytes(resp3.Msg.HeaderId)
	if err != nil {
		t.Fatal(err)
	}

	// Verify the headers are in the correct order
	orderedHeaders, err := ehs.GetHeadersByExampleIDOrdered(ctx, exampleID)
	if err != nil {
		t.Fatal(err)
	}

	if len(orderedHeaders) != 3 {
		t.Fatalf("expected 3 headers, got %d", len(orderedHeaders))
	}

	// Headers should be in creation order: header1, header2, header3
	expectedOrder := []idwrap.IDWrap{header1ID, header2ID, header3ID}
	for i, header := range orderedHeaders {
		if header.ID != expectedOrder[i] {
			t.Errorf("at position %d, expected header %s, got %s", i, expectedOrder[i], header.ID)
		}
	}

	// Verify the linked list structure
	// First header should have no prev, next should point to second
	if orderedHeaders[0].Prev != nil {
		t.Error("first header should have no prev pointer")
	}
	if orderedHeaders[0].Next == nil || *orderedHeaders[0].Next != header2ID {
		t.Error("first header should point to second header")
	}

	// Second header should point back to first, forward to third
	if orderedHeaders[1].Prev == nil || *orderedHeaders[1].Prev != header1ID {
		t.Error("second header should point back to first header")
	}
	if orderedHeaders[1].Next == nil || *orderedHeaders[1].Next != header3ID {
		t.Error("second header should point to third header")
	}

	// Third header should point back to second, no next
	if orderedHeaders[2].Prev == nil || *orderedHeaders[2].Prev != header2ID {
		t.Error("third header should point back to second header")
	}
	if orderedHeaders[2].Next != nil {
		t.Error("third header should have no next pointer")
	}
}

func TestQueryCreateMaintainsOrder(t *testing.T) {
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

	// Create three queries using the RPC endpoints to test ordering
	req1 := connect.NewRequest(&requestv1.QueryCreateRequest{
		ExampleId:   exampleID.Bytes(),
		Key:         "query1",
		Enabled:     true,
		Value:       "value1",
		Description: "First query",
	})

	resp1, err := rpcRequest.QueryCreate(authedCtx, req1)
	if err != nil {
		t.Fatal(err)
	}

	req2 := connect.NewRequest(&requestv1.QueryCreateRequest{
		ExampleId:   exampleID.Bytes(),
		Key:         "query2",
		Enabled:     true,
		Value:       "value2",
		Description: "Second query",
	})

	resp2, err := rpcRequest.QueryCreate(authedCtx, req2)
	if err != nil {
		t.Fatal(err)
	}

	req3 := connect.NewRequest(&requestv1.QueryCreateRequest{
		ExampleId:   exampleID.Bytes(),
		Key:         "query3",
		Enabled:     true,
		Value:       "value3",
		Description: "Third query",
	})

	resp3, err := rpcRequest.QueryCreate(authedCtx, req3)
	if err != nil {
		t.Fatal(err)
	}

	// Get the created query IDs
	query1ID, err := idwrap.NewFromBytes(resp1.Msg.QueryId)
	if err != nil {
		t.Fatal(err)
	}

	query2ID, err := idwrap.NewFromBytes(resp2.Msg.QueryId)
	if err != nil {
		t.Fatal(err)
	}

	query3ID, err := idwrap.NewFromBytes(resp3.Msg.QueryId)
	if err != nil {
		t.Fatal(err)
	}

	// Verify the queries are in the correct order
	orderedQueries, err := eqs.GetQueriesByExampleIDOrdered(ctx, exampleID)
	if err != nil {
		t.Fatal(err)
	}

	if len(orderedQueries) != 3 {
		t.Fatalf("expected 3 queries, got %d", len(orderedQueries))
	}

	// Queries should be in creation order: query1, query2, query3
	expectedOrder := []idwrap.IDWrap{query1ID, query2ID, query3ID}
	for i, query := range orderedQueries {
		if query.ID != expectedOrder[i] {
			t.Errorf("at position %d, expected query %s, got %s", i, expectedOrder[i], query.ID)
		}
	}

	// Verify the linked list structure
	// First query should have no prev, next should point to second
	if orderedQueries[0].Prev != nil {
		t.Error("first query should have no prev pointer")
	}
	if orderedQueries[0].Next == nil || *orderedQueries[0].Next != query2ID {
		t.Error("first query should point to second query")
	}

	// Second query should point back to first, forward to third
	if orderedQueries[1].Prev == nil || *orderedQueries[1].Prev != query1ID {
		t.Error("second query should point back to first query")
	}
	if orderedQueries[1].Next == nil || *orderedQueries[1].Next != query3ID {
		t.Error("second query should point to third query")
	}

	// Third query should point back to second, no next
	if orderedQueries[2].Prev == nil || *orderedQueries[2].Prev != query2ID {
		t.Error("third query should point back to second query")
	}
	if orderedQueries[2].Next != nil {
		t.Error("third query should have no next pointer")
	}
}