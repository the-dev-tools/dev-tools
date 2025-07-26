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

func TestHeaderDeltaExampleCopy(t *testing.T) {
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

	// Create multiple headers in the origin example
	headers := []struct {
		key   string
		value string
		desc  string
	}{
		{"Authorization", "Bearer token123", "Auth header"},
		{"Content-Type", "application/json", "Content type header"},
		{"User-Agent", "test-client/1.0", "User agent header"},
	}

	var originHeaderIDs []idwrap.IDWrap

	for _, header := range headers {
		req := connect.NewRequest(&requestv1.HeaderCreateRequest{
			ExampleId:   originExampleID.Bytes(),
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

		originHeaderIDs = append(originHeaderIDs, headerID)
	}

	// Test bulk copy using HeaderDeltaExampleCopy
	err = rpcRequest.HeaderDeltaExampleCopy(authedCtx, originExampleID, deltaExampleID)
	if err != nil {
		t.Fatal(err)
	}

	// Verify headers were copied to delta example
	deltaHeaders, err := ehs.GetHeaderByExampleID(ctx, deltaExampleID)
	if err != nil {
		t.Fatal(err)
	}

	if len(deltaHeaders) != len(headers) {
		t.Fatalf("expected %d delta headers, got %d", len(headers), len(deltaHeaders))
	}

	// Verify each header has correct delta parent relationship
	for i, deltaHeader := range deltaHeaders {
		if deltaHeader.DeltaParentID == nil {
			t.Errorf("delta header %d should have a parent ID", i)
			continue
		}

		// Find the corresponding origin header
		found := false
		for _, originID := range originHeaderIDs {
			if *deltaHeader.DeltaParentID == originID {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("delta header %d has invalid parent ID %s", i, *deltaHeader.DeltaParentID)
		}

		if deltaHeader.ExampleID != deltaExampleID {
			t.Errorf("delta header %d has wrong example ID", i)
		}
	}
}

func TestQueryDeltaExampleCopy(t *testing.T) {
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

	// Create multiple queries in the origin example
	queries_data := []struct {
		key   string
		value string
		desc  string
	}{
		{"page", "1", "Page number"},
		{"limit", "10", "Items per page"},
		{"sort", "name", "Sort field"},
	}

	var originQueryIDs []idwrap.IDWrap

	for _, query := range queries_data {
		req := connect.NewRequest(&requestv1.QueryCreateRequest{
			ExampleId:   originExampleID.Bytes(),
			Key:         query.key,
			Enabled:     true,
			Value:       query.value,
			Description: query.desc,
		})

		resp, err := rpcRequest.QueryCreate(authedCtx, req)
		if err != nil {
			t.Fatalf("failed to create query %s: %v", query.key, err)
		}

		queryID, err := idwrap.NewFromBytes(resp.Msg.QueryId)
		if err != nil {
			t.Fatal(err)
		}

		originQueryIDs = append(originQueryIDs, queryID)
	}

	// Test bulk copy using QueryDeltaExampleCopy
	err = rpcRequest.QueryDeltaExampleCopy(authedCtx, originExampleID, deltaExampleID)
	if err != nil {
		t.Fatal(err)
	}

	// Verify queries were copied to delta example
	deltaQueries, err := eqs.GetExampleQueriesByExampleID(ctx, deltaExampleID)
	if err != nil {
		t.Fatal(err)
	}

	if len(deltaQueries) != len(queries_data) {
		t.Fatalf("expected %d delta queries, got %d", len(queries_data), len(deltaQueries))
	}

	// Verify each query has correct delta parent relationship
	for i, deltaQuery := range deltaQueries {
		if deltaQuery.DeltaParentID == nil {
			t.Errorf("delta query %d should have a parent ID", i)
			continue
		}

		// Find the corresponding origin query
		found := false
		for _, originID := range originQueryIDs {
			if *deltaQuery.DeltaParentID == originID {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("delta query %d has invalid parent ID %s", i, *deltaQuery.DeltaParentID)
		}

		if deltaQuery.ExampleID != deltaExampleID {
			t.Errorf("delta query %d has wrong example ID", i)
		}
	}
}