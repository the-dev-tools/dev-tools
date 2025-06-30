package rrequest_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rrequest"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/mcondition"
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
	conditionv1 "the-dev-tools/spec/dist/buf/go/condition/v1"
	deltav1 "the-dev-tools/spec/dist/buf/go/delta/v1"
	requestv1 "the-dev-tools/spec/dist/buf/go/collection/item/request/v1"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helpers

type deltaTestContext struct {
	ctx              context.Context
	db               *sql.DB
	rpc              rrequest.RequestRPC
	userID           idwrap.IDWrap
	collectionID     idwrap.IDWrap
	originExampleID  idwrap.IDWrap
	deltaExampleID   idwrap.IDWrap
	itemApiID        idwrap.IDWrap
	// Services
	qs               sexamplequery.ExampleQueryService
	hs               sexampleheader.HeaderService
	as               sassert.AssertService
	iaes             sitemapiexample.ItemApiExampleService
}

func setupDeltaTest(t *testing.T) *deltaTestContext {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	// Initialize services
	mockLogger := mocklogger.NewMockLogger()
	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	cs := scollection.New(queries, mockLogger)
	us := suser.New(queries)
	ehs := sexampleheader.New(queries)
	eqs := sexamplequery.New(queries)
	as := sassert.New(queries)

	// Create test data
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	userID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)

	// Create item API
	item := &mitemapi.ItemApi{
		ID:           idwrap.NewNow(),
		Name:         "test_delta_api",
		Url:          "https://api.example.com/test",
		Method:       "GET",
		CollectionID: collectionID,
		FolderID:     nil,
	}
	err := ias.CreateItemApi(ctx, item)
	require.NoError(t, err)

	// Create origin example
	originExampleID := idwrap.NewNow()
	originExample := &mitemapiexample.ItemApiExample{
		ID:              originExampleID,
		ItemApiID:       item.ID,
		CollectionID:    collectionID,
		Name:            "origin_example",
		VersionParentID: nil, // No version parent - this is an origin
	}
	err = iaes.CreateApiExample(ctx, originExample)
	require.NoError(t, err)

	// Create delta example with version parent
	deltaExampleID := idwrap.NewNow()
	deltaExample := &mitemapiexample.ItemApiExample{
		ID:              deltaExampleID,
		ItemApiID:       item.ID,
		CollectionID:    collectionID,
		Name:            "delta_example",
		VersionParentID: &originExampleID, // Has version parent - this is a delta
	}
	err = iaes.CreateApiExample(ctx, deltaExample)
	require.NoError(t, err)

	rpc := rrequest.New(db, cs, us, iaes, ehs, eqs, as)

	return &deltaTestContext{
		ctx:             mwauth.CreateAuthedContext(ctx, userID),
		db:              db,
		rpc:             rpc,
		userID:          userID,
		collectionID:    collectionID,
		originExampleID: originExampleID,
		deltaExampleID:  deltaExampleID,
		itemApiID:       item.ID,
		qs:              eqs,
		hs:              ehs,
		as:              as,
		iaes:            iaes,
	}
}

// Query Delta Tests

func TestQueryDeltaList_EmptyDelta(t *testing.T) {
	tc := setupDeltaTest(t)

	// Create some queries in origin example
	queries := []mexamplequery.Query{
		{
			ID:        idwrap.NewNow(),
			ExampleID: tc.originExampleID,
			QueryKey:  "api_key",
			Enable:    true,
			Value:     "secret123",
			Description: "API Key",
		},
		{
			ID:        idwrap.NewNow(),
			ExampleID: tc.originExampleID,
			QueryKey:  "page",
			Enable:    true,
			Value:     "1",
			Description: "Page number",
		},
	}

	for _, q := range queries {
		err := tc.qs.CreateExampleQuery(tc.ctx, q)
		require.NoError(t, err)
	}

	// List delta queries - should auto-create delta entries
	req := connect.NewRequest(&requestv1.QueryDeltaListRequest{
		ExampleId: tc.deltaExampleID.Bytes(),
		OriginId:  tc.originExampleID.Bytes(),
	})

	resp, err := tc.rpc.QueryDeltaList(tc.ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Should have 2 items, all with origin source
	assert.Len(t, resp.Msg.Items, 2)
	for _, item := range resp.Msg.Items {
		assert.NotNil(t, item.Source)
		assert.Equal(t, deltav1.SourceKind_SOURCE_KIND_ORIGIN, *item.Source)
		assert.NotNil(t, item.Origin)
		assert.Empty(t, item.Key) // Empty for auto-created entries
		assert.Empty(t, item.Value)
	}
}

func TestQueryDeltaList_MixedSources(t *testing.T) {
	tc := setupDeltaTest(t)

	// Create origin queries
	originQuery1 := mexamplequery.Query{
		ID:        idwrap.NewNow(),
		ExampleID: tc.originExampleID,
		QueryKey:  "param1",
		Enable:    true,
		Value:     "value1",
		Description: "First param",
	}
	originQuery2 := mexamplequery.Query{
		ID:        idwrap.NewNow(),
		ExampleID: tc.originExampleID,
		QueryKey:  "param2",
		Enable:    false,
		Value:     "value2",
		Description: "Second param",
	}

	err := tc.qs.CreateExampleQuery(tc.ctx, originQuery1)
	require.NoError(t, err)
	err = tc.qs.CreateExampleQuery(tc.ctx, originQuery2)
	require.NoError(t, err)

	// Create a delta query (will be shown as mixed)
	deltaQuery := mexamplequery.Query{
		ID:            idwrap.NewNow(),
		ExampleID:     tc.deltaExampleID,
		DeltaParentID: &originQuery1.ID,
		QueryKey:      "param1_modified",
		Enable:        false,
		Value:         "modified_value",
		Description:   "Modified first param",
	}
	err = tc.qs.CreateExampleQuery(tc.ctx, deltaQuery)
	require.NoError(t, err)

	// List delta queries
	req := connect.NewRequest(&requestv1.QueryDeltaListRequest{
		ExampleId: tc.deltaExampleID.Bytes(),
		OriginId:  tc.originExampleID.Bytes(),
	})

	resp, err := tc.rpc.QueryDeltaList(tc.ctx, req)
	require.NoError(t, err)

	// Should have 2 items
	assert.Len(t, resp.Msg.Items, 2)

	// Find the mixed source item
	var mixedItem *requestv1.QueryDeltaListItem
	var originItem *requestv1.QueryDeltaListItem
	for _, item := range resp.Msg.Items {
		if item.Source != nil && *item.Source == deltav1.SourceKind_SOURCE_KIND_MIXED {
			mixedItem = item
		} else if item.Source != nil && *item.Source == deltav1.SourceKind_SOURCE_KIND_ORIGIN {
			originItem = item
		}
	}

	assert.NotNil(t, mixedItem)
	assert.NotNil(t, originItem)

	// Verify mixed item has modified values
	assert.Equal(t, "param1_modified", mixedItem.Key)
	assert.Equal(t, "modified_value", mixedItem.Value)
	assert.Equal(t, false, mixedItem.Enabled)

	// Verify origin item is auto-created
	assert.Empty(t, originItem.Key)
	assert.Empty(t, originItem.Value)
}

func TestQueryDeltaUpdate_OriginToMixed(t *testing.T) {
	tc := setupDeltaTest(t)

	// Copy queries from origin to delta
	err := tc.rpc.QueryDeltaExampleCopy(tc.ctx, tc.originExampleID, tc.deltaExampleID)
	require.NoError(t, err)

	// Create an origin query in origin example
	originQuery := mexamplequery.Query{
		ID:        idwrap.NewNow(),
		ExampleID: tc.originExampleID,
		QueryKey:  "test_key",
		Enable:    true,
		Value:     "original",
		Description: "Original query",
	}
	err = tc.qs.CreateExampleQuery(tc.ctx, originQuery)
	require.NoError(t, err)

	// Copy to delta
	err = tc.rpc.QueryDeltaExampleCopy(tc.ctx, tc.originExampleID, tc.deltaExampleID)
	require.NoError(t, err)

	// Get the delta queries to find the copied one
	queries, err := tc.qs.GetExampleQueriesByExampleID(tc.ctx, tc.deltaExampleID)
	require.NoError(t, err)

	var deltaQueryID idwrap.IDWrap
	for _, q := range queries {
		if q.DeltaParentID != nil && q.DeltaParentID.Compare(originQuery.ID) == 0 {
			deltaQueryID = q.ID
			break
		}
	}
	require.NotEmpty(t, deltaQueryID)

	// Update the delta query (should create a mixed entry)
	modifiedKey := "modified_key"
	modifiedEnabled := false
	modifiedValue := "modified"
	modifiedDesc := "Modified query"
	updateReq := connect.NewRequest(&requestv1.QueryDeltaUpdateRequest{
		QueryId:     deltaQueryID.Bytes(),
		Key:         &modifiedKey,
		Enabled:     &modifiedEnabled,
		Value:       &modifiedValue,
		Description: &modifiedDesc,
	})

	_, err = tc.rpc.QueryDeltaUpdate(tc.ctx, updateReq)
	require.NoError(t, err)

	// Verify it's now mixed
	query, err := tc.qs.GetExampleQuery(tc.ctx, deltaQueryID)
	require.NoError(t, err)
	assert.Equal(t, "modified_key", query.QueryKey)
	assert.Equal(t, false, query.Enable)
}

func TestQueryDeltaReset_RestoresParentValues(t *testing.T) {
	tc := setupDeltaTest(t)

	// Create origin query
	originQuery := mexamplequery.Query{
		ID:        idwrap.NewNow(),
		ExampleID: tc.originExampleID,
		QueryKey:  "reset_test",
		Enable:    true,
		Value:     "original_value",
		Description: "Original description",
	}
	err := tc.qs.CreateExampleQuery(tc.ctx, originQuery)
	require.NoError(t, err)

	// Create modified delta query
	deltaQuery := mexamplequery.Query{
		ID:            idwrap.NewNow(),
		ExampleID:     tc.deltaExampleID,
		DeltaParentID: &originQuery.ID,
		QueryKey:      "modified_key",
		Enable:        false,
		Value:         "modified_value",
		Description:   "Modified description",
	}
	err = tc.qs.CreateExampleQuery(tc.ctx, deltaQuery)
	require.NoError(t, err)

	// Reset the delta query
	resetReq := connect.NewRequest(&requestv1.QueryDeltaResetRequest{
		QueryId: deltaQuery.ID.Bytes(),
	})

	_, err = tc.rpc.QueryDeltaReset(tc.ctx, resetReq)
	require.NoError(t, err)

	// Verify values are restored from parent
	query, err := tc.qs.GetExampleQuery(tc.ctx, deltaQuery.ID)
	require.NoError(t, err)
	assert.Equal(t, originQuery.QueryKey, query.QueryKey)
	assert.Equal(t, originQuery.Enable, query.Enable)
	assert.Equal(t, originQuery.Value, query.Value)
	assert.Equal(t, originQuery.Description, query.Description)
}

func TestQueryUpdate_PropagatestoOiriginSourceDeltas(t *testing.T) {
	tc := setupDeltaTest(t)

	// Create origin query
	originQuery := mexamplequery.Query{
		ID:        idwrap.NewNow(),
		ExampleID: tc.originExampleID,
		QueryKey:  "propagate_test",
		Enable:    true,
		Value:     "original",
		Description: "Original",
	}
	err := tc.qs.CreateExampleQuery(tc.ctx, originQuery)
	require.NoError(t, err)

	// Create delta query that references origin (will have origin source)
	deltaQuery := mexamplequery.Query{
		ID:            idwrap.NewNow(),
		ExampleID:     tc.deltaExampleID,
		DeltaParentID: &originQuery.ID,
		QueryKey:      originQuery.QueryKey,
		Enable:        originQuery.Enable,
		Value:         originQuery.Value,
		Description:   originQuery.Description,
	}
	err = tc.qs.CreateExampleQuery(tc.ctx, deltaQuery)
	require.NoError(t, err)

	// Update origin query
	updatedKey := "updated_key"
	updatedEnabled := false
	updatedValue := "updated_value"
	updatedDesc := "Updated description"
	updateReq := connect.NewRequest(&requestv1.QueryUpdateRequest{
		QueryId:     originQuery.ID.Bytes(),
		Key:         &updatedKey,
		Enabled:     &updatedEnabled,
		Value:       &updatedValue,
		Description: &updatedDesc,
	})

	_, err = tc.rpc.QueryUpdate(tc.ctx, updateReq)
	require.NoError(t, err)

	// Verify delta query was updated
	updated, err := tc.qs.GetExampleQuery(tc.ctx, deltaQuery.ID)
	require.NoError(t, err)
	assert.Equal(t, "updated_key", updated.QueryKey)
	assert.Equal(t, false, updated.Enable)
	assert.Equal(t, "updated_value", updated.Value)
}

func TestQueryDelete_CascadesToDeltaItems(t *testing.T) {
	tc := setupDeltaTest(t)

	// Create origin query
	originQuery := mexamplequery.Query{
		ID:        idwrap.NewNow(),
		ExampleID: tc.originExampleID,
		QueryKey:  "delete_test",
		Enable:    true,
		Value:     "will_be_deleted",
		Description: "To be deleted",
	}
	err := tc.qs.CreateExampleQuery(tc.ctx, originQuery)
	require.NoError(t, err)

	// Create delta queries
	deltaQuery1 := mexamplequery.Query{
		ID:            idwrap.NewNow(),
		ExampleID:     tc.deltaExampleID,
		DeltaParentID: &originQuery.ID,
		QueryKey:      originQuery.QueryKey,
		Enable:        originQuery.Enable,
		Value:         originQuery.Value,
		Description:   originQuery.Description,
	}
	deltaQuery2 := mexamplequery.Query{
		ID:            idwrap.NewNow(),
		ExampleID:     tc.deltaExampleID,
		DeltaParentID: &originQuery.ID,
		QueryKey:      "modified",
		Enable:        false,
		Value:         "modified",
		Description:   "Modified",
	}

	err = tc.qs.CreateExampleQuery(tc.ctx, deltaQuery1)
	require.NoError(t, err)
	err = tc.qs.CreateExampleQuery(tc.ctx, deltaQuery2)
	require.NoError(t, err)

	// Delete origin query
	deleteReq := connect.NewRequest(&requestv1.QueryDeleteRequest{
		QueryId: originQuery.ID.Bytes(),
	})

	_, err = tc.rpc.QueryDelete(tc.ctx, deleteReq)
	require.NoError(t, err)

	// Verify all queries are deleted
	_, err = tc.qs.GetExampleQuery(tc.ctx, originQuery.ID)
	assert.Error(t, err)
	_, err = tc.qs.GetExampleQuery(tc.ctx, deltaQuery1.ID)
	assert.Error(t, err)
	_, err = tc.qs.GetExampleQuery(tc.ctx, deltaQuery2.ID)
	assert.Error(t, err)
}

// Header Delta Tests

func TestHeaderDeltaList_AutoCreateEntries(t *testing.T) {
	tc := setupDeltaTest(t)

	// Create headers in origin example
	headers := []mexampleheader.Header{
		{
			ID:          idwrap.NewNow(),
			ExampleID:   tc.originExampleID,
			HeaderKey:   "Authorization",
			Enable:      true,
			Value:       "Bearer token123",
			Description: "Auth header",
		},
		{
			ID:          idwrap.NewNow(),
			ExampleID:   tc.originExampleID,
			HeaderKey:   "Content-Type",
			Enable:      true,
			Value:       "application/json",
			Description: "Content type",
		},
	}

	for _, h := range headers {
		err := tc.hs.CreateHeader(tc.ctx, h)
		require.NoError(t, err)
	}

	// List delta headers
	req := connect.NewRequest(&requestv1.HeaderDeltaListRequest{
		ExampleId: tc.deltaExampleID.Bytes(),
		OriginId:  tc.originExampleID.Bytes(),
	})

	resp, err := tc.rpc.HeaderDeltaList(tc.ctx, req)
	require.NoError(t, err)

	assert.Len(t, resp.Msg.Items, 2)
	for _, item := range resp.Msg.Items {
		assert.NotNil(t, item.Source)
		assert.Equal(t, deltav1.SourceKind_SOURCE_KIND_ORIGIN, *item.Source)
		assert.NotNil(t, item.Origin)
		assert.Empty(t, item.Key)
	}
}

func TestHeaderDeltaUpdate_CreatesNewMixedHeader(t *testing.T) {
	tc := setupDeltaTest(t)

	// Copy headers from origin to delta
	err := tc.rpc.HeaderDeltaExampleCopy(tc.ctx, tc.originExampleID, tc.deltaExampleID)
	require.NoError(t, err)

	// Create origin header
	originHeader := mexampleheader.Header{
		ID:          idwrap.NewNow(),
		ExampleID:   tc.originExampleID,
		HeaderKey:   "X-Custom",
		Enable:      true,
		Value:       "original",
		Description: "Custom header",
	}
	err = tc.hs.CreateHeader(tc.ctx, originHeader)
	require.NoError(t, err)

	// Copy to delta
	err = tc.rpc.HeaderDeltaExampleCopy(tc.ctx, tc.originExampleID, tc.deltaExampleID)
	require.NoError(t, err)

	// Get delta headers to find the copied one
	headers, err := tc.hs.GetHeaderByExampleID(tc.ctx, tc.deltaExampleID)
	require.NoError(t, err)

	var deltaHeaderID idwrap.IDWrap
	for _, h := range headers {
		if h.DeltaParentID != nil && h.DeltaParentID.Compare(originHeader.ID) == 0 {
			deltaHeaderID = h.ID
			break
		}
	}
	require.NotEmpty(t, deltaHeaderID)

	// Update should not create new header, just update existing
	key := "X-Modified"
	enabled := false
	value := "modified"
	description := "Modified header"
	updateReq := connect.NewRequest(&requestv1.HeaderDeltaUpdateRequest{
		HeaderId:    deltaHeaderID.Bytes(),
		Key:         &key,
		Enabled:     &enabled,
		Value:       &value,
		Description: &description,
	})

	_, err = tc.rpc.HeaderDeltaUpdate(tc.ctx, updateReq)
	require.NoError(t, err)

	// Verify update
	header, err := tc.hs.GetHeaderByID(tc.ctx, deltaHeaderID)
	require.NoError(t, err)
	assert.Equal(t, "X-Modified", header.HeaderKey)
	assert.Equal(t, false, header.Enable)
}

func TestHeaderDelete_CascadesToDeltaHeaders(t *testing.T) {
	tc := setupDeltaTest(t)

	// Create origin header
	originHeader := mexampleheader.Header{
		ID:          idwrap.NewNow(),
		ExampleID:   tc.originExampleID,
		HeaderKey:   "X-Delete-Test",
		Enable:      true,
		Value:       "will_delete",
		Description: "Will be deleted",
	}
	err := tc.hs.CreateHeader(tc.ctx, originHeader)
	require.NoError(t, err)

	// Create delta headers with different sources
	deltaHeader1 := mexampleheader.Header{
		ID:            idwrap.NewNow(),
		ExampleID:     tc.deltaExampleID,
		DeltaParentID: &originHeader.ID,
		HeaderKey:     originHeader.HeaderKey,
		Enable:        originHeader.Enable,
		Value:         originHeader.Value,
		Description:   originHeader.Description,
	}
	deltaHeader2 := mexampleheader.Header{
		ID:            idwrap.NewNow(),
		ExampleID:     tc.deltaExampleID,
		DeltaParentID: &originHeader.ID,
		HeaderKey:     "X-Modified",
		Enable:        false,
		Value:         "modified",
		Description:   "Modified",
	}

	err = tc.hs.CreateHeader(tc.ctx, deltaHeader1)
	require.NoError(t, err)
	err = tc.hs.CreateHeader(tc.ctx, deltaHeader2)
	require.NoError(t, err)

	// Delete origin header
	deleteReq := connect.NewRequest(&requestv1.HeaderDeleteRequest{
		HeaderId: originHeader.ID.Bytes(),
	})

	_, err = tc.rpc.HeaderDelete(tc.ctx, deleteReq)
	require.NoError(t, err)

	// Verify cascade deletion
	_, err = tc.hs.GetHeaderByID(tc.ctx, originHeader.ID)
	assert.Error(t, err)
	_, err = tc.hs.GetHeaderByID(tc.ctx, deltaHeader1.ID)
	assert.Error(t, err)
	_, err = tc.hs.GetHeaderByID(tc.ctx, deltaHeader2.ID)
	assert.Error(t, err)
}

// Assert Delta Tests

func TestAssertDeltaList_MixedSources(t *testing.T) {
	tc := setupDeltaTest(t)

	// Create origin assert
	originAssert := massert.Assert{
		ID:        idwrap.NewNow(),
		ExampleID: tc.originExampleID,
		Condition: mcondition.Condition{
			Comparisons: mcondition.Comparison{
				Expression: "status == \"200\"",
			},
		},
		Enable: true,
	}
	err := tc.as.CreateAssert(tc.ctx, originAssert)
	require.NoError(t, err)

	// Create delta assert
	deltaAssert := massert.Assert{
		ID:            idwrap.NewNow(),
		ExampleID:     tc.deltaExampleID,
		DeltaParentID: &originAssert.ID,
		Condition: mcondition.Condition{
			Comparisons: mcondition.Comparison{
				Expression: "status == \"201\"",
			},
		},
		Enable: true,
	}
	err = tc.as.CreateAssert(tc.ctx, deltaAssert)
	require.NoError(t, err)

	// List delta asserts
	req := connect.NewRequest(&requestv1.AssertDeltaListRequest{
		ExampleId: tc.deltaExampleID.Bytes(),
		OriginId:  tc.originExampleID.Bytes(),
	})

	resp, err := tc.rpc.AssertDeltaList(tc.ctx, req)
	require.NoError(t, err)

	assert.Len(t, resp.Msg.Items, 1)
	item := resp.Msg.Items[0]
	assert.NotNil(t, item.Source)
	assert.Equal(t, deltav1.SourceKind_SOURCE_KIND_MIXED, *item.Source)
	assert.NotNil(t, item.Origin)
}

func TestAssertUpdate_PropagatestoOriginSourceDeltas(t *testing.T) {
	tc := setupDeltaTest(t)

	// Create origin assert
	originAssert := massert.Assert{
		ID:        idwrap.NewNow(),
		ExampleID: tc.originExampleID,
		Condition: mcondition.Condition{
			Comparisons: mcondition.Comparison{
				Expression: "data.id == \"123\"",
			},
		},
		Enable: true,
	}
	err := tc.as.CreateAssert(tc.ctx, originAssert)
	require.NoError(t, err)

	// Create delta assert with origin source
	deltaAssert := massert.Assert{
		ID:            idwrap.NewNow(),
		ExampleID:     tc.deltaExampleID,
		DeltaParentID: &originAssert.ID,
		Condition:     originAssert.Condition,
		Enable:        originAssert.Enable,
	}
	err = tc.as.CreateAssert(tc.ctx, deltaAssert)
	require.NoError(t, err)

	// Update origin assert
	updateReq := connect.NewRequest(&requestv1.AssertUpdateRequest{
		AssertId: originAssert.ID.Bytes(),
		Condition: &conditionv1.Condition{
			Comparison: &conditionv1.Comparison{
				Expression: "data.id == \"456\"",
			},
		},
	})

	_, err = tc.rpc.AssertUpdate(tc.ctx, updateReq)
	require.NoError(t, err)

	// Verify propagation
	updated, err := tc.as.GetAssert(tc.ctx, deltaAssert.ID)
	require.NoError(t, err)
	assert.Contains(t, updated.Condition.Comparisons.Expression, "456")
}

func TestAssertDeltaReset_RestoresFromParent(t *testing.T) {
	tc := setupDeltaTest(t)

	// Create origin assert
	originAssert := massert.Assert{
		ID:        idwrap.NewNow(),
		ExampleID: tc.originExampleID,
		Condition: mcondition.Condition{
			Comparisons: mcondition.Comparison{
				Expression: "success == \"true\"",
			},
		},
		Enable: true,
	}
	err := tc.as.CreateAssert(tc.ctx, originAssert)
	require.NoError(t, err)

	// Create modified delta assert
	deltaAssert := massert.Assert{
		ID:            idwrap.NewNow(),
		ExampleID:     tc.deltaExampleID,
		DeltaParentID: &originAssert.ID,
		Condition: mcondition.Condition{
			Comparisons: mcondition.Comparison{
				Expression: "success == \"false\"",
			},
		},
		Enable: false,
	}
	err = tc.as.CreateAssert(tc.ctx, deltaAssert)
	require.NoError(t, err)

	// Reset delta assert
	resetReq := connect.NewRequest(&requestv1.AssertDeltaResetRequest{
		AssertId: deltaAssert.ID.Bytes(),
	})

	_, err = tc.rpc.AssertDeltaReset(tc.ctx, resetReq)
	require.NoError(t, err)

	// Verify reset
	reset, err := tc.as.GetAssert(tc.ctx, deltaAssert.ID)
	require.NoError(t, err)
	assert.Equal(t, originAssert.Condition.Comparisons.Expression, reset.Condition.Comparisons.Expression)
	assert.Equal(t, originAssert.Enable, reset.Enable)
}

// Edge Case Tests

func TestDeltaOperations_InvalidParentID(t *testing.T) {
	tc := setupDeltaTest(t)

	nonExistentID := idwrap.NewNow()

	// Test query with invalid parent
	queryReq := connect.NewRequest(&requestv1.QueryDeltaCreateRequest{
		ExampleId:   tc.deltaExampleID.Bytes(),
		QueryId:     nonExistentID.Bytes(), // Non-existent parent
		Key:         "test",
		Enabled:     true,
		Value:       "test",
		Description: "test",
	})

	_, err := tc.rpc.QueryDeltaCreate(tc.ctx, queryReq)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not_found")

	// Test header with invalid parent
	headerReq := connect.NewRequest(&requestv1.HeaderDeltaCreateRequest{
		ExampleId:   tc.deltaExampleID.Bytes(),
		HeaderId:    nonExistentID.Bytes(), // Non-existent parent
		Key:         "test",
		Enabled:     true,
		Value:       "test",
		Description: "test",
	})

	_, err = tc.rpc.HeaderDeltaCreate(tc.ctx, headerReq)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not_found")
}

func TestDeltaOperations_CrossExampleParent(t *testing.T) {
	tc := setupDeltaTest(t)

	// Create another example
	otherExampleID := idwrap.NewNow()
	otherExample := &mitemapiexample.ItemApiExample{
		ID:           otherExampleID,
		ItemApiID:    tc.itemApiID,
		CollectionID: tc.collectionID,
		Name:         "other_example",
	}
	err := tc.iaes.CreateApiExample(tc.ctx, otherExample)
	require.NoError(t, err)

	// Create query in other example
	otherQuery := mexamplequery.Query{
		ID:        idwrap.NewNow(),
		ExampleID: otherExampleID,
		QueryKey:  "other",
		Enable:    true,
		Value:     "other",
		Description: "Other query",
	}
	err = tc.qs.CreateExampleQuery(tc.ctx, otherQuery)
	require.NoError(t, err)

	// Try to create delta query with parent from different example
	req := connect.NewRequest(&requestv1.QueryDeltaCreateRequest{
		ExampleId:   tc.deltaExampleID.Bytes(),
		QueryId:     otherQuery.ID.Bytes(), // Parent from different example
		Key:         "test",
		Enabled:     true,
		Value:       "test",
		Description: "test",
	})

	_, err = tc.rpc.QueryDeltaCreate(tc.ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not belong to the specified example")
}

func TestDeltaBulkOperations_LargeDataset(t *testing.T) {
	tc := setupDeltaTest(t)

	// Create many origin queries (more than chunk size of 10)
	const numQueries = 25
	for i := 0; i < numQueries; i++ {
		query := mexamplequery.Query{
			ID:        idwrap.NewNow(),
			ExampleID: tc.originExampleID,
			QueryKey:  fmt.Sprintf("key_%d", i),
			Enable:    i%2 == 0,
			Value:     fmt.Sprintf("value_%d", i),
			Description: fmt.Sprintf("Query %d", i),
		}
		err := tc.qs.CreateExampleQuery(tc.ctx, query)
		require.NoError(t, err)
	}

	// Copy all to delta (tests bulk operations)
	err := tc.rpc.QueryDeltaExampleCopy(tc.ctx, tc.originExampleID, tc.deltaExampleID)
	require.NoError(t, err)

	// Verify all were copied
	deltaQueries, err := tc.qs.GetExampleQueriesByExampleID(tc.ctx, tc.deltaExampleID)
	require.NoError(t, err)
	assert.Len(t, deltaQueries, numQueries)
}

func TestDeltaOperations_PermissionChecks(t *testing.T) {
	tc := setupDeltaTest(t)

	// Create another user context without permissions
	unauthorizedUserID := idwrap.NewNow()
	unauthorizedCtx := mwauth.CreateAuthedContext(context.Background(), unauthorizedUserID)

	// Try to list delta queries without permission
	req := connect.NewRequest(&requestv1.QueryDeltaListRequest{
		ExampleId: tc.deltaExampleID.Bytes(),
		OriginId:  tc.originExampleID.Bytes(),
	})

	_, err := tc.rpc.QueryDeltaList(unauthorizedCtx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "permission_denied")
}

func TestDeltaReset_WithoutParent(t *testing.T) {
	tc := setupDeltaTest(t)

	// Create standalone delta query (no parent)
	standaloneQuery := mexamplequery.Query{
		ID:            idwrap.NewNow(),
		ExampleID:     tc.deltaExampleID,
		DeltaParentID: nil, // No parent
		QueryKey:      "standalone",
		Enable:        true,
		Value:         "test",
		Description:   "Standalone query",
	}
	err := tc.qs.CreateExampleQuery(tc.ctx, standaloneQuery)
	require.NoError(t, err)

	// Reset should clear fields
	resetReq := connect.NewRequest(&requestv1.QueryDeltaResetRequest{
		QueryId: standaloneQuery.ID.Bytes(),
	})

	_, err = tc.rpc.QueryDeltaReset(tc.ctx, resetReq)
	require.NoError(t, err)

	// Verify fields are cleared
	reset, err := tc.qs.GetExampleQuery(tc.ctx, standaloneQuery.ID)
	require.NoError(t, err)
	assert.Empty(t, reset.QueryKey)
	assert.False(t, reset.Enable)
	assert.Empty(t, reset.Value)
	assert.Empty(t, reset.Description)
}

func TestSourceTypeDetermination_ComplexScenarios(t *testing.T) {
	tc := setupDeltaTest(t)

	// Create origin query in origin example
	originQuery := mexamplequery.Query{
		ID:        idwrap.NewNow(),
		ExampleID: tc.originExampleID,
		QueryKey:  "source_test",
		Enable:    true,
		Value:     "origin",
		Description: "Origin query",
	}
	err := tc.qs.CreateExampleQuery(tc.ctx, originQuery)
	require.NoError(t, err)

	// Create mixed query in origin example (references another origin)
	mixedInOrigin := mexamplequery.Query{
		ID:            idwrap.NewNow(),
		ExampleID:     tc.originExampleID,
		DeltaParentID: &originQuery.ID,
		QueryKey:      "mixed_in_origin",
		Enable:        false,
		Value:         "mixed",
		Description:   "Mixed in origin example",
	}
	err = tc.qs.CreateExampleQuery(tc.ctx, mixedInOrigin)
	require.NoError(t, err)

	// Verify source types
	// In origin example (no version parent)
	assert.Equal(t, mexamplequery.QuerySourceOrigin, originQuery.DetermineDeltaType(false))
	assert.Equal(t, mexamplequery.QuerySourceMixed, mixedInOrigin.DetermineDeltaType(false))

	// Create delta query in delta example
	deltaQuery := mexamplequery.Query{
		ID:            idwrap.NewNow(),
		ExampleID:     tc.deltaExampleID,
		DeltaParentID: &originQuery.ID,
		QueryKey:      "delta_query",
		Enable:        true,
		Value:         "delta",
		Description:   "Delta query",
	}
	err = tc.qs.CreateExampleQuery(tc.ctx, deltaQuery)
	require.NoError(t, err)

	// In delta example (has version parent)
	assert.Equal(t, mexamplequery.QuerySourceDelta, deltaQuery.DetermineDeltaType(true))
}

// Table-driven tests for comprehensive coverage

func TestQueryDeltaOperations_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(tc *deltaTestContext) (idwrap.IDWrap, error)
		operation   string
		expectError bool
		errorMsg    string
		validate    func(t *testing.T, tc *deltaTestContext, id idwrap.IDWrap)
	}{
		{
			name: "create_delta_query_with_valid_parent",
			setupFunc: func(tc *deltaTestContext) (idwrap.IDWrap, error) {
				// Create origin query
				origin := mexamplequery.Query{
					ID:        idwrap.NewNow(),
					ExampleID: tc.originExampleID,
					QueryKey:  "parent",
					Enable:    true,
					Value:     "value",
				}
				err := tc.qs.CreateExampleQuery(tc.ctx, origin)
				if err != nil {
					return idwrap.IDWrap{}, err
				}

				// Create delta with parent
				req := connect.NewRequest(&requestv1.QueryDeltaCreateRequest{
					ExampleId: tc.deltaExampleID.Bytes(),
					QueryId:   origin.ID.Bytes(),
					Key:       "child",
					Enabled:   false,
					Value:     "child_value",
				})
				resp, err := tc.rpc.QueryDeltaCreate(tc.ctx, req)
				if err != nil {
					return idwrap.IDWrap{}, err
				}
				return idwrap.NewFromBytes(resp.Msg.QueryId)
			},
			expectError: false,
			validate: func(t *testing.T, tc *deltaTestContext, id idwrap.IDWrap) {
				query, err := tc.qs.GetExampleQuery(tc.ctx, id)
				require.NoError(t, err)
				assert.NotNil(t, query.DeltaParentID)
				assert.Equal(t, "child", query.QueryKey)
			},
		},
		{
			name: "delete_origin_cascades_to_all_deltas",
			setupFunc: func(tc *deltaTestContext) (idwrap.IDWrap, error) {
				// Create origin
				origin := mexamplequery.Query{
					ID:        idwrap.NewNow(),
					ExampleID: tc.originExampleID,
					QueryKey:  "cascade_test",
					Enable:    true,
					Value:     "origin",
				}
				err := tc.qs.CreateExampleQuery(tc.ctx, origin)
				if err != nil {
					return idwrap.IDWrap{}, err
				}

				// Create multiple deltas
				for i := 0; i < 3; i++ {
					delta := mexamplequery.Query{
						ID:            idwrap.NewNow(),
						ExampleID:     tc.deltaExampleID,
						DeltaParentID: &origin.ID,
						QueryKey:      fmt.Sprintf("delta_%d", i),
						Enable:        true,
						Value:         fmt.Sprintf("value_%d", i),
					}
					err := tc.qs.CreateExampleQuery(tc.ctx, delta)
					if err != nil {
						return idwrap.IDWrap{}, err
					}
				}

				return origin.ID, nil
			},
			operation:   "delete",
			expectError: false,
			validate: func(t *testing.T, tc *deltaTestContext, originID idwrap.IDWrap) {
				// Verify origin is deleted
				_, err := tc.qs.GetExampleQuery(tc.ctx, originID)
				assert.Error(t, err)

				// Verify all deltas are deleted
				queries, err := tc.qs.GetExampleQueriesByExampleID(tc.ctx, tc.deltaExampleID)
				require.NoError(t, err)
				for _, q := range queries {
					if q.DeltaParentID != nil && q.DeltaParentID.Compare(originID) == 0 {
						t.Errorf("Found delta query that should have been deleted: %v", q.ID)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := setupDeltaTest(t)
			id, err := tt.setupFunc(tc)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, tc, id)
				}
			}
		})
	}
}

// Concurrent operations test
func TestDeltaOperations_Concurrent(t *testing.T) {
	tc := setupDeltaTest(t)

	// Create origin queries
	const numQueries = 10
	originQueries := make([]idwrap.IDWrap, numQueries)
	for i := 0; i < numQueries; i++ {
		query := mexamplequery.Query{
			ID:        idwrap.NewNow(),
			ExampleID: tc.originExampleID,
			QueryKey:  fmt.Sprintf("concurrent_%d", i),
			Enable:    true,
			Value:     fmt.Sprintf("value_%d", i),
			Description: fmt.Sprintf("Concurrent test %d", i),
		}
		err := tc.qs.CreateExampleQuery(tc.ctx, query)
		require.NoError(t, err)
		originQueries[i] = query.ID
	}

	// Concurrently update all queries
	errCh := make(chan error, numQueries)
	for i := 0; i < numQueries; i++ {
		go func(idx int) {
			key := fmt.Sprintf("updated_%d", idx)
			enabled := false
			value := fmt.Sprintf("updated_value_%d", idx)
			description := fmt.Sprintf("Updated %d", idx)
			req := connect.NewRequest(&requestv1.QueryUpdateRequest{
				QueryId:     originQueries[idx].Bytes(),
				Key:         &key,
				Enabled:     &enabled,
				Value:       &value,
				Description: &description,
			})
			_, err := tc.rpc.QueryUpdate(tc.ctx, req)
			errCh <- err
		}(i)
	}

	// Check all operations succeeded
	for i := 0; i < numQueries; i++ {
		err := <-errCh
		assert.NoError(t, err)
	}

	// Verify all updates
	for i := 0; i < numQueries; i++ {
		query, err := tc.qs.GetExampleQuery(tc.ctx, originQueries[i])
		require.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("updated_%d", i), query.QueryKey)
		assert.Equal(t, false, query.Enable)
	}
}

// Test circular reference prevention
func TestDeltaOperations_CircularReferencePrevention(t *testing.T) {
	tc := setupDeltaTest(t)

	// Create a query that tries to reference itself
	selfRefQuery := idwrap.NewNow()
	req := connect.NewRequest(&requestv1.QueryDeltaCreateRequest{
		ExampleId:   tc.deltaExampleID.Bytes(),
		QueryId:     selfRefQuery.Bytes(), // Self reference
		Key:         "self_ref",
		Enabled:     true,
		Value:       "circular",
		Description: "Circular reference test",
	})

	_, err := tc.rpc.QueryDeltaCreate(tc.ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not_found")
}

// Test null and empty field handling
func TestDeltaOperations_NullEmptyFields(t *testing.T) {
	tc := setupDeltaTest(t)

	// Create query with empty fields
	emptyQuery := mexamplequery.Query{
		ID:          idwrap.NewNow(),
		ExampleID:   tc.originExampleID,
		QueryKey:    "", // Empty key
		Enable:      false,
		Value:       "", // Empty value
		Description: "", // Empty description
	}
	err := tc.qs.CreateExampleQuery(tc.ctx, emptyQuery)
	require.NoError(t, err)

	// Copy to delta - should handle empty fields
	err = tc.rpc.QueryDeltaExampleCopy(tc.ctx, tc.originExampleID, tc.deltaExampleID)
	require.NoError(t, err)

	// List delta queries
	req := connect.NewRequest(&requestv1.QueryDeltaListRequest{
		ExampleId: tc.deltaExampleID.Bytes(),
		OriginId:  tc.originExampleID.Bytes(),
	})

	resp, err := tc.rpc.QueryDeltaList(tc.ctx, req)
	require.NoError(t, err)
	
	// Verify empty fields are preserved
	found := false
	for _, item := range resp.Msg.Items {
		if item.Origin != nil && len(item.Origin.QueryId) > 0 {
			originID, _ := idwrap.NewFromBytes(item.Origin.QueryId)
			if originID.Compare(emptyQuery.ID) == 0 {
				found = true
				assert.Empty(t, item.Key)
				assert.Empty(t, item.Value)
				assert.Empty(t, item.Description)
				assert.False(t, item.Enabled)
			}
		}
	}
	assert.True(t, found, "Empty query not found in delta list")
}

// Test multiple delta examples referencing same origin
func TestDeltaOperations_MultipleDeltasFromSameOrigin(t *testing.T) {
	tc := setupDeltaTest(t)

	// Create another delta example
	deltaExample2ID := idwrap.NewNow()
	deltaExample2 := &mitemapiexample.ItemApiExample{
		ID:              deltaExample2ID,
		ItemApiID:       tc.itemApiID,
		CollectionID:    tc.collectionID,
		Name:            "delta_example_2",
		VersionParentID: &tc.originExampleID,
	}
	err := tc.iaes.CreateApiExample(tc.ctx, deltaExample2)
	require.NoError(t, err)

	// Create query in origin
	originQuery := mexamplequery.Query{
		ID:        idwrap.NewNow(),
		ExampleID: tc.originExampleID,
		QueryKey:  "shared_origin",
		Enable:    true,
		Value:     "original",
		Description: "Shared by multiple deltas",
	}
	err = tc.qs.CreateExampleQuery(tc.ctx, originQuery)
	require.NoError(t, err)

	// Copy to both delta examples
	err = tc.rpc.QueryDeltaExampleCopy(tc.ctx, tc.originExampleID, tc.deltaExampleID)
	require.NoError(t, err)
	err = tc.rpc.QueryDeltaExampleCopy(tc.ctx, tc.originExampleID, deltaExample2ID)
	require.NoError(t, err)

	// Update origin query
	sharedKey := "updated_shared"
	sharedEnabled := false
	sharedValue := "updated"
	sharedDescription := "Updated shared query"
	updateReq := connect.NewRequest(&requestv1.QueryUpdateRequest{
		QueryId:     originQuery.ID.Bytes(),
		Key:         &sharedKey,
		Enabled:     &sharedEnabled,
		Value:       &sharedValue,
		Description: &sharedDescription,
	})

	_, err = tc.rpc.QueryUpdate(tc.ctx, updateReq)
	require.NoError(t, err)

	// Verify both delta queries were updated
	queries1, err := tc.qs.GetExampleQueriesByExampleID(tc.ctx, tc.deltaExampleID)
	require.NoError(t, err)
	queries2, err := tc.qs.GetExampleQueriesByExampleID(tc.ctx, deltaExample2ID)
	require.NoError(t, err)

	// Check first delta
	found1 := false
	for _, q := range queries1 {
		if q.DeltaParentID != nil && q.DeltaParentID.Compare(originQuery.ID) == 0 {
			found1 = true
			assert.Equal(t, "updated_shared", q.QueryKey)
			assert.Equal(t, false, q.Enable)
		}
	}
	assert.True(t, found1)

	// Check second delta
	found2 := false
	for _, q := range queries2 {
		if q.DeltaParentID != nil && q.DeltaParentID.Compare(originQuery.ID) == 0 {
			found2 = true
			assert.Equal(t, "updated_shared", q.QueryKey)
			assert.Equal(t, false, q.Enable)
		}
	}
	assert.True(t, found2)
}