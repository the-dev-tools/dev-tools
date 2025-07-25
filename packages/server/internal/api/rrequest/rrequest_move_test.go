package rrequest_test

import (
	"context"
	"testing"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rrequest"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
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
	resourcesv1 "the-dev-tools/spec/dist/buf/go/resources/v1"

	"connectrpc.com/connect"
)

func TestHeaderMove(t *testing.T) {
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

	// Create three headers - they will be automatically ordered by the create methods
	header1ID := idwrap.NewNow()
	header2ID := idwrap.NewNow()
	header3ID := idwrap.NewNow()

	header1 := mexampleheader.Header{
		ID:          header1ID,
		ExampleID:   exampleID,
		HeaderKey:   "Header1",
		Value:       "Value1",
		Enable:      true,
		Description: "First header",
	}
	err = ehs.CreateHeader(ctx, header1)
	if err != nil {
		t.Fatal(err)
	}

	header2 := mexampleheader.Header{
		ID:          header2ID,
		ExampleID:   exampleID,
		HeaderKey:   "Header2",
		Value:       "Value2",
		Enable:      true,
		Description: "Second header",
	}
	err = ehs.CreateHeader(ctx, header2)
	if err != nil {
		t.Fatal(err)
	}

	header3 := mexampleheader.Header{
		ID:          header3ID,
		ExampleID:   exampleID,
		HeaderKey:   "Header3",
		Value:       "Value3",
		Enable:      true,
		Description: "Third header",
	}
	err = ehs.CreateHeader(ctx, header3)
	if err != nil {
		t.Fatal(err)
	}

	// Get initial order
	initialHeaders, err := ehs.GetHeadersByExampleIDOrdered(ctx, exampleID)
	if err != nil {
		t.Fatal(err)
	}

	if len(initialHeaders) != 3 {
		t.Fatalf("expected 3 headers initially, got %d", len(initialHeaders))
	}

	// Find which header is first and which is last for the test
	firstHeaderID := initialHeaders[0].ID
	lastHeaderID := initialHeaders[2].ID

	t.Logf("Initial order: first=%s, middle=%s, last=%s", initialHeaders[0].ID, initialHeaders[1].ID, initialHeaders[2].ID)
	t.Logf("Moving last header %s before first header %s", lastHeaderID, firstHeaderID)

	// Test moving the last header before the first header
	req := connect.NewRequest(&requestv1.HeaderMoveRequest{
		HeaderId:       lastHeaderID.Bytes(),
		TargetHeaderId: firstHeaderID.Bytes(),
		Position:       resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
	})

	rpcRequest := rrequest.New(db, cs, us, ias, iaes, ehs, eqs, as)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcRequest.HeaderMove(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}

	if resp == nil {
		t.Fatal("resp is nil")
	}

	// Verify the new order - the last header should now be first
	orderedHeaders, err := ehs.GetHeadersByExampleIDOrdered(ctx, exampleID)
	if err != nil {
		t.Fatal(err)
	}

	if len(orderedHeaders) != 3 {
		t.Fatalf("expected 3 headers after move, got %d", len(orderedHeaders))
	}

	t.Logf("Final order: first=%s, middle=%s, last=%s", orderedHeaders[0].ID, orderedHeaders[1].ID, orderedHeaders[2].ID)

	// The moved header should now be first
	if orderedHeaders[0].ID != lastHeaderID {
		t.Errorf("expected moved header %s to be first, but got %s", lastHeaderID, orderedHeaders[0].ID)
	}

	// The original first header should now be second
	if orderedHeaders[1].ID != firstHeaderID {
		t.Errorf("expected original first header %s to be second, but got %s", firstHeaderID, orderedHeaders[1].ID)
	}
}

func TestHeaderDeltaMove(t *testing.T) {
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

	// Create delta example that references the origin
	deltaExampleID := idwrap.NewNow()
	deltaExample := &mitemapiexample.ItemApiExample{
		ID:              deltaExampleID,
		ItemApiID:       item.ID,
		CollectionID:    CollectionID,
		Name:            "delta_example",
		IsDefault:       false,
		BodyType:        mitemapiexample.BodyTypeRaw,
		VersionParentID: &originExampleID,
	}

	err = iaes.CreateApiExample(ctx, deltaExample)
	if err != nil {
		t.Fatal(err)
	}

	// Create two delta headers
	deltaHeader1ID := idwrap.NewNow()
	deltaHeader2ID := idwrap.NewNow()

	deltaHeader1 := mexampleheader.Header{
		ID:          deltaHeader1ID,
		ExampleID:   deltaExampleID,
		HeaderKey:   "DeltaHeader1",
		Value:       "DeltaValue1",
		Enable:      true,
		Description: "First delta header",
	}
	err = ehs.CreateHeader(ctx, deltaHeader1)
	if err != nil {
		t.Fatal(err)
	}

	deltaHeader2 := mexampleheader.Header{
		ID:          deltaHeader2ID,
		ExampleID:   deltaExampleID,
		HeaderKey:   "DeltaHeader2",
		Value:       "DeltaValue2",
		Enable:      true,
		Description: "Second delta header",
	}
	err = ehs.CreateHeader(ctx, deltaHeader2)
	if err != nil {
		t.Fatal(err)
	}

	// Get initial order
	initialHeaders, err := ehs.GetHeadersByExampleIDOrdered(ctx, deltaExampleID)
	if err != nil {
		t.Fatal(err)
	}

	if len(initialHeaders) != 2 {
		t.Fatalf("expected 2 delta headers initially, got %d", len(initialHeaders))
	}

	firstHeaderID := initialHeaders[0].ID
	secondHeaderID := initialHeaders[1].ID

	// Test delta header move - move second header before first header
	req := connect.NewRequest(&requestv1.HeaderDeltaMoveRequest{
		HeaderId:       secondHeaderID.Bytes(),
		TargetHeaderId: firstHeaderID.Bytes(),
		Position:       resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
		OriginId:       originExampleID.Bytes(),
	})

	rpcRequest := rrequest.New(db, cs, us, ias, iaes, ehs, eqs, as)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcRequest.HeaderDeltaMove(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}

	if resp == nil {
		t.Fatal("resp is nil")
	}

	// Verify the new order
	orderedHeaders, err := ehs.GetHeadersByExampleIDOrdered(ctx, deltaExampleID)
	if err != nil {
		t.Fatal(err)
	}

	if len(orderedHeaders) != 2 {
		t.Fatalf("expected 2 headers after move, got %d", len(orderedHeaders))
	}

	// The moved header should now be first
	if orderedHeaders[0].ID != secondHeaderID {
		t.Errorf("expected moved header %s to be first, but got %s", secondHeaderID, orderedHeaders[0].ID)
	}
}

func TestQueryMove(t *testing.T) {
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

	// Create three queries - they will be automatically ordered by the create methods
	query1ID := idwrap.NewNow()
	query2ID := idwrap.NewNow()
	query3ID := idwrap.NewNow()

	query1 := mexamplequery.Query{
		ID:          query1ID,
		ExampleID:   exampleID,
		QueryKey:    "query1",
		Value:       "value1",
		Enable:      true,
		Description: "First query",
	}
	err = eqs.CreateExampleQuery(ctx, query1)
	if err != nil {
		t.Fatal(err)
	}

	query2 := mexamplequery.Query{
		ID:          query2ID,
		ExampleID:   exampleID,
		QueryKey:    "query2",
		Value:       "value2",
		Enable:      true,
		Description: "Second query",
	}
	err = eqs.CreateExampleQuery(ctx, query2)
	if err != nil {
		t.Fatal(err)
	}

	query3 := mexamplequery.Query{
		ID:          query3ID,
		ExampleID:   exampleID,
		QueryKey:    "query3",
		Value:       "value3",
		Enable:      true,
		Description: "Third query",
	}
	err = eqs.CreateExampleQuery(ctx, query3)
	if err != nil {
		t.Fatal(err)
	}

	// Get initial order
	initialQueries, err := eqs.GetQueriesByExampleIDOrdered(ctx, exampleID)
	if err != nil {
		t.Fatal(err)
	}

	if len(initialQueries) != 3 {
		t.Fatalf("expected 3 queries initially, got %d", len(initialQueries))
	}

	firstQueryID := initialQueries[0].ID
	lastQueryID := initialQueries[2].ID

	// Test moving the first query after the last query
	req := connect.NewRequest(&requestv1.QueryMoveRequest{
		QueryId:       firstQueryID.Bytes(),
		TargetQueryId: lastQueryID.Bytes(),
		Position:      resourcesv1.MovePosition_MOVE_POSITION_AFTER,
	})

	rpcRequest := rrequest.New(db, cs, us, ias, iaes, ehs, eqs, as)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcRequest.QueryMove(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}

	if resp == nil {
		t.Fatal("resp is nil")
	}

	// Verify the new order
	orderedQueries, err := eqs.GetQueriesByExampleIDOrdered(ctx, exampleID)
	if err != nil {
		t.Fatal(err)
	}

	if len(orderedQueries) != 3 {
		t.Fatalf("expected 3 queries after move, got %d", len(orderedQueries))
	}

	// The moved query should now be last
	if orderedQueries[2].ID != firstQueryID {
		t.Errorf("expected moved query %s to be last, but got %s", firstQueryID, orderedQueries[2].ID)
	}
}

func TestQueryDeltaMove(t *testing.T) {
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
		ID:              deltaExampleID,
		ItemApiID:       item.ID,
		CollectionID:    CollectionID,
		Name:            "delta_example",
		IsDefault:       false,
		BodyType:        mitemapiexample.BodyTypeRaw,
		VersionParentID: &originExampleID,
	}

	err = iaes.CreateApiExample(ctx, deltaExample)
	if err != nil {
		t.Fatal(err)
	}

	// Create two delta queries
	deltaQuery1ID := idwrap.NewNow()
	deltaQuery2ID := idwrap.NewNow()

	deltaQuery1 := mexamplequery.Query{
		ID:          deltaQuery1ID,
		ExampleID:   deltaExampleID,
		QueryKey:    "deltaQuery1",
		Value:       "deltaValue1",
		Enable:      true,
		Description: "First delta query",
	}
	err = eqs.CreateExampleQuery(ctx, deltaQuery1)
	if err != nil {
		t.Fatal(err)
	}

	deltaQuery2 := mexamplequery.Query{
		ID:          deltaQuery2ID,
		ExampleID:   deltaExampleID,
		QueryKey:    "deltaQuery2",
		Value:       "deltaValue2",
		Enable:      true,
		Description: "Second delta query",
	}
	err = eqs.CreateExampleQuery(ctx, deltaQuery2)
	if err != nil {
		t.Fatal(err)
	}

	// Get initial order
	initialQueries, err := eqs.GetQueriesByExampleIDOrdered(ctx, deltaExampleID)
	if err != nil {
		t.Fatal(err)
	}

	if len(initialQueries) != 2 {
		t.Fatalf("expected 2 delta queries initially, got %d", len(initialQueries))
	}

	firstQueryID := initialQueries[0].ID
	secondQueryID := initialQueries[1].ID

	// Test delta query move - move second query before first query
	req := connect.NewRequest(&requestv1.QueryDeltaMoveRequest{
		QueryId:       secondQueryID.Bytes(),
		TargetQueryId: firstQueryID.Bytes(),
		Position:      resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
		OriginId:      originExampleID.Bytes(),
	})

	rpcRequest := rrequest.New(db, cs, us, ias, iaes, ehs, eqs, as)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcRequest.QueryDeltaMove(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}

	if resp == nil {
		t.Fatal("resp is nil")
	}

	// Verify the new order
	orderedQueries, err := eqs.GetQueriesByExampleIDOrdered(ctx, deltaExampleID)
	if err != nil {
		t.Fatal(err)
	}

	if len(orderedQueries) != 2 {
		t.Fatalf("expected 2 queries after move, got %d", len(orderedQueries))
	}

	// The moved query should now be first
	if orderedQueries[0].ID != secondQueryID {
		t.Errorf("expected moved query %s to be first, but got %s", secondQueryID, orderedQueries[0].ID)
	}
}

// TestHeaderListAndMoveIntegration tests the complete workflow of creating headers,
// listing them, moving them, and verifying the new order through listing
func TestHeaderListAndMoveIntegration(t *testing.T) {
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

	// Create 3 headers in order: A -> B -> C
	headerA := mexampleheader.Header{
		ID:          idwrap.NewNow(),
		ExampleID:   exampleID,
		HeaderKey:   "Header-A",
		Value:       "value-a",
		Enable:      true,
		Description: "Header A",
	}
	err = ehs.CreateHeader(ctx, headerA)
	if err != nil {
		t.Fatal(err)
	}

	headerB := mexampleheader.Header{
		ID:          idwrap.NewNow(),
		ExampleID:   exampleID,
		HeaderKey:   "Header-B",
		Value:       "value-b",
		Enable:      true,
		Description: "Header B",
	}
	err = ehs.CreateHeader(ctx, headerB)
	if err != nil {
		t.Fatal(err)
	}

	headerC := mexampleheader.Header{
		ID:          idwrap.NewNow(),
		ExampleID:   exampleID,
		HeaderKey:   "Header-C",
		Value:       "value-c",
		Enable:      true,
		Description: "Header C",
	}
	err = ehs.CreateHeader(ctx, headerC)
	if err != nil {
		t.Fatal(err)
	}

	rpcRequest := rrequest.New(db, cs, us, ias, iaes, ehs, eqs, as)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)

	// Test 1: Initial listing should show A, B, C in that order
	t.Run("Initial_Order", func(t *testing.T) {
		listResp, err := rpcRequest.HeaderList(authedCtx, connect.NewRequest(&requestv1.HeaderListRequest{
			ExampleId: exampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		headers := listResp.Msg.GetItems()
		if len(headers) != 3 {
			t.Fatalf("Expected 3 headers, got %d", len(headers))
		}

		// Verify order: A, B, C
		testutil.Assert(t, headerA.ID, idwrap.NewFromBytesMust(headers[0].GetHeaderId()))
		testutil.Assert(t, headerB.ID, idwrap.NewFromBytesMust(headers[1].GetHeaderId()))
		testutil.Assert(t, headerC.ID, idwrap.NewFromBytesMust(headers[2].GetHeaderId()))

		// Verify content
		if headers[0].GetKey() != "Header-A" || headers[0].GetValue() != "value-a" {
			t.Errorf("Header A content mismatch: got key=%s, value=%s", headers[0].GetKey(), headers[0].GetValue())
		}
	})

	// Test 2: Move B before A (result should be: B -> A -> C)
	t.Run("Move_B_Before_A", func(t *testing.T) {
		_, err := rpcRequest.HeaderMove(authedCtx, connect.NewRequest(&requestv1.HeaderMoveRequest{
			HeaderId:       headerB.ID.Bytes(),
			TargetHeaderId: headerA.ID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Verify new order through listing
		listResp, err := rpcRequest.HeaderList(authedCtx, connect.NewRequest(&requestv1.HeaderListRequest{
			ExampleId: exampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		headers := listResp.Msg.GetItems()
		if len(headers) != 3 {
			t.Fatalf("Expected 3 headers, got %d", len(headers))
		}

		// Verify order: B, A, C
		testutil.Assert(t, headerB.ID, idwrap.NewFromBytesMust(headers[0].GetHeaderId()))
		testutil.Assert(t, headerA.ID, idwrap.NewFromBytesMust(headers[1].GetHeaderId()))
		testutil.Assert(t, headerC.ID, idwrap.NewFromBytesMust(headers[2].GetHeaderId()))
	})

	// Test 3: Move A after C (result should be: B -> C -> A)
	t.Run("Move_A_After_C", func(t *testing.T) {
		_, err := rpcRequest.HeaderMove(authedCtx, connect.NewRequest(&requestv1.HeaderMoveRequest{
			HeaderId:       headerA.ID.Bytes(),
			TargetHeaderId: headerC.ID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Verify new order through listing
		listResp, err := rpcRequest.HeaderList(authedCtx, connect.NewRequest(&requestv1.HeaderListRequest{
			ExampleId: exampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		headers := listResp.Msg.GetItems()
		if len(headers) != 3 {
			t.Fatalf("Expected 3 headers, got %d", len(headers))
		}

		// Verify order: B, C, A
		testutil.Assert(t, headerB.ID, idwrap.NewFromBytesMust(headers[0].GetHeaderId()))
		testutil.Assert(t, headerC.ID, idwrap.NewFromBytesMust(headers[1].GetHeaderId()))
		testutil.Assert(t, headerA.ID, idwrap.NewFromBytesMust(headers[2].GetHeaderId()))
	})
}

// TestQueryListAndMoveIntegration tests the complete workflow for queries
func TestQueryListAndMoveIntegration(t *testing.T) {
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

	// Create 3 queries in order: X -> Y -> Z
	queryX := mexamplequery.Query{
		ID:          idwrap.NewNow(),
		ExampleID:   exampleID,
		QueryKey:    "param_x",
		Value:       "value_x",
		Enable:      true,
		Description: "Query X",
	}
	err = eqs.CreateExampleQuery(ctx, queryX)
	if err != nil {
		t.Fatal(err)
	}

	queryY := mexamplequery.Query{
		ID:          idwrap.NewNow(),
		ExampleID:   exampleID,
		QueryKey:    "param_y",
		Value:       "value_y",
		Enable:      true,
		Description: "Query Y",
	}
	err = eqs.CreateExampleQuery(ctx, queryY)
	if err != nil {
		t.Fatal(err)
	}

	queryZ := mexamplequery.Query{
		ID:          idwrap.NewNow(),
		ExampleID:   exampleID,
		QueryKey:    "param_z",
		Value:       "value_z",
		Enable:      true,
		Description: "Query Z",
	}
	err = eqs.CreateExampleQuery(ctx, queryZ)
	if err != nil {
		t.Fatal(err)
	}

	rpcRequest := rrequest.New(db, cs, us, ias, iaes, ehs, eqs, as)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)

	// Test 1: Initial listing should show X, Y, Z in that order
	t.Run("Initial_Order", func(t *testing.T) {
		listResp, err := rpcRequest.QueryList(authedCtx, connect.NewRequest(&requestv1.QueryListRequest{
			ExampleId: exampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		queries := listResp.Msg.GetItems()
		if len(queries) != 3 {
			t.Fatalf("Expected 3 queries, got %d", len(queries))
		}

		// Verify order: X, Y, Z
		testutil.Assert(t, queryX.ID, idwrap.NewFromBytesMust(queries[0].GetQueryId()))
		testutil.Assert(t, queryY.ID, idwrap.NewFromBytesMust(queries[1].GetQueryId()))
		testutil.Assert(t, queryZ.ID, idwrap.NewFromBytesMust(queries[2].GetQueryId()))

		// Verify content
		if queries[0].GetKey() != "param_x" || queries[0].GetValue() != "value_x" {
			t.Errorf("Query X content mismatch: got key=%s, value=%s", queries[0].GetKey(), queries[0].GetValue())
		}
	})

	// Test 2: Move Z before X (result should be: Z -> X -> Y)
	t.Run("Move_Z_Before_X", func(t *testing.T) {
		_, err := rpcRequest.QueryMove(authedCtx, connect.NewRequest(&requestv1.QueryMoveRequest{
			QueryId:       queryZ.ID.Bytes(),
			TargetQueryId: queryX.ID.Bytes(),
			Position:      resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Verify new order through listing
		listResp, err := rpcRequest.QueryList(authedCtx, connect.NewRequest(&requestv1.QueryListRequest{
			ExampleId: exampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		queries := listResp.Msg.GetItems()
		if len(queries) != 3 {
			t.Fatalf("Expected 3 queries, got %d", len(queries))
		}

		// Verify order: Z, X, Y
		testutil.Assert(t, queryZ.ID, idwrap.NewFromBytesMust(queries[0].GetQueryId()))
		testutil.Assert(t, queryX.ID, idwrap.NewFromBytesMust(queries[1].GetQueryId()))
		testutil.Assert(t, queryY.ID, idwrap.NewFromBytesMust(queries[2].GetQueryId()))
	})
}

// TestHeaderDeltaListAndMoveIntegration tests delta examples with moves and lists
func TestHeaderDeltaListAndMoveIntegration(t *testing.T) {
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

	// Create origin headers
	originHeaderA := mexampleheader.Header{
		ID:          idwrap.NewNow(),
		ExampleID:   originExampleID,
		HeaderKey:   "Origin-A",
		Value:       "value-a",
		Enable:      true,
		Description: "Origin Header A",
	}
	err = ehs.CreateHeader(ctx, originHeaderA)
	if err != nil {
		t.Fatal(err)
	}

	originHeaderB := mexampleheader.Header{
		ID:          idwrap.NewNow(),
		ExampleID:   originExampleID,
		HeaderKey:   "Origin-B",
		Value:       "value-b",
		Enable:      true,
		Description: "Origin Header B",
	}
	err = ehs.CreateHeader(ctx, originHeaderB)
	if err != nil {
		t.Fatal(err)
	}

	// Create delta example
	deltaExampleID := idwrap.NewNow()
	deltaExample := &mitemapiexample.ItemApiExample{
		ID:              deltaExampleID,
		ItemApiID:       item.ID,
		CollectionID:    CollectionID,
		Name:            "delta_example",
		IsDefault:       false,
		BodyType:        mitemapiexample.BodyTypeRaw,
		VersionParentID: &originExampleID,
	}

	err = iaes.CreateApiExample(ctx, deltaExample)
	if err != nil {
		t.Fatal(err)
	}

	// Create a delta header
	deltaHeader := mexampleheader.Header{
		ID:          idwrap.NewNow(),
		ExampleID:   deltaExampleID,
		HeaderKey:   "Delta-C",
		Value:       "value-c",
		Enable:      true,
		Description: "Delta Header C",
	}
	err = ehs.CreateHeader(ctx, deltaHeader)
	if err != nil {
		t.Fatal(err)
	}

	rpcRequest := rrequest.New(db, cs, us, ias, iaes, ehs, eqs, as)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)

	// Test 1: Initial delta list should show all headers
	t.Run("Initial_Delta_List", func(t *testing.T) {
		listResp, err := rpcRequest.HeaderDeltaList(authedCtx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: deltaExampleID.Bytes(),
			OriginId:  originExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		headers := listResp.Msg.GetItems()
		if len(headers) != 3 {
			t.Fatalf("Expected 3 headers in delta list, got %d", len(headers))
		}

		// Should show headers in creation order: Origin-A, Origin-B, Delta-C
		if headers[0].GetKey() != "Origin-A" {
			t.Errorf("Expected first header to be Origin-A, got %s", headers[0].GetKey())
		}
		if headers[1].GetKey() != "Origin-B" {
			t.Errorf("Expected second header to be Origin-B, got %s", headers[1].GetKey())
		}
		if headers[2].GetKey() != "Delta-C" {
			t.Errorf("Expected third header to be Delta-C, got %s", headers[2].GetKey())
		}
	})

	// Test 2: Move origin header in delta and verify the order
	t.Run("Move_Origin_Header_In_Delta", func(t *testing.T) {
		// Move Origin-A after Origin-B
		_, err := rpcRequest.HeaderDeltaMove(authedCtx, connect.NewRequest(&requestv1.HeaderDeltaMoveRequest{
			HeaderId:       originHeaderA.ID.Bytes(),
			TargetHeaderId: originHeaderB.ID.Bytes(),
			Position:       resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			OriginId:       originExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Verify the new order in delta list
		listResp, err := rpcRequest.HeaderDeltaList(authedCtx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: deltaExampleID.Bytes(),
			OriginId:  originExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		headers := listResp.Msg.GetItems()
		if len(headers) != 3 {
			t.Fatalf("Expected 3 headers in delta list after move, got %d", len(headers))
		}

		// Order should now be: Origin-B, Origin-A, Delta-C
		if headers[0].GetKey() != "Origin-B" {
			t.Errorf("Expected first header to be Origin-B after move, got %s", headers[0].GetKey())
		}
		if headers[1].GetKey() != "Origin-A" {
			t.Errorf("Expected second header to be Origin-A after move, got %s", headers[1].GetKey())
		}
		if headers[2].GetKey() != "Delta-C" {
			t.Errorf("Expected third header to be Delta-C after move, got %s", headers[2].GetKey())
		}
	})
}