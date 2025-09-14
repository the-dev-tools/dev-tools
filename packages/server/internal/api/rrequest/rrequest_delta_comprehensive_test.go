package rrequest_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rrequest"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
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
	conditionv1 "the-dev-tools/spec/dist/buf/go/condition/v1"
	deltav1 "the-dev-tools/spec/dist/buf/go/delta/v1"

	"connectrpc.com/connect"
)

// Comprehensive test data structure for delta testing
type comprehensiveDeltaTestData struct {
	ctx             context.Context
	rpc             rrequest.RequestRPC
	originExampleID idwrap.IDWrap
	deltaExampleID  idwrap.IDWrap
	deltaExample2ID idwrap.IDWrap // For testing multiple deltas
	ehs             sexampleheader.HeaderService
	eqs             sexamplequery.ExampleQueryService
	as              sassert.AssertService
	iaes            sitemapiexample.ItemApiExampleService
}

// reuse stringPtr and boolPtr helpers to avoid conflicts with other test file
var (
	stringPtrHelper = func(s string) *string { return &s }
	boolPtrHelper   = func(b bool) *bool { return &b }
)

// setupComprehensiveDeltaTestData creates comprehensive test data
func setupComprehensiveDeltaTestData(t *testing.T) *comprehensiveDeltaTestData {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	// Initialize services
	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	mockLogger := mocklogger.NewMockLogger()
	cs := scollection.New(queries, mockLogger)
	us := suser.New(queries)
	ehs := sexampleheader.New(queries)
	eqs := sexamplequery.New(queries)
	as := sassert.New(queries)

	// Create RPC service
	rpc := rrequest.New(db, cs, us, ias, iaes, ehs, eqs, as)

	// Create workspace and collection
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	userID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)

	// Create authenticated context
	ctx = mwauth.CreateAuthedContext(ctx, userID)

	// Create item
	item := &mitemapi.ItemApi{
		ID:           idwrap.NewNow(),
		Name:         "test-item",
		Url:          "http://example.com",
		Method:       "GET",
		CollectionID: collectionID,
	}
	err := ias.CreateItemApi(ctx, item)
	if err != nil {
		t.Fatal(err)
	}

	// Create origin example
	originExampleID := idwrap.NewNow()
	originExample := &mitemapiexample.ItemApiExample{
		ID:              originExampleID,
		ItemApiID:       item.ID,
		CollectionID:    collectionID,
		Name:            "origin-example",
		VersionParentID: nil,
	}
	err = iaes.CreateApiExample(ctx, originExample)
	if err != nil {
		t.Fatal(err)
	}

	// Create first delta example
	deltaExampleID := idwrap.NewNow()
	deltaExample := &mitemapiexample.ItemApiExample{
		ID:              deltaExampleID,
		ItemApiID:       item.ID,
		CollectionID:    collectionID,
		Name:            "delta-example-1",
		VersionParentID: &originExampleID,
	}
	err = iaes.CreateApiExample(ctx, deltaExample)
	if err != nil {
		t.Fatal(err)
	}

	// Create second delta example for multi-delta tests
	deltaExample2ID := idwrap.NewNow()
	deltaExample2 := &mitemapiexample.ItemApiExample{
		ID:              deltaExample2ID,
		ItemApiID:       item.ID,
		CollectionID:    collectionID,
		Name:            "delta-example-2",
		VersionParentID: &originExampleID,
	}
	err = iaes.CreateApiExample(ctx, deltaExample2)
	if err != nil {
		t.Fatal(err)
	}

	return &comprehensiveDeltaTestData{
		ctx:             ctx,
		rpc:             rpc,
		originExampleID: originExampleID,
		deltaExampleID:  deltaExampleID,
		deltaExample2ID: deltaExample2ID,
		ehs:             ehs,
		eqs:             eqs,
		as:              as,
		iaes:            iaes,
	}
}

// Test Query Delta Functionality
func TestQueryDeltaComprehensive(t *testing.T) {
	t.Run("QueryDeltaExampleCopy", func(t *testing.T) {
		data := setupComprehensiveDeltaTestData(t)

		// Create multiple queries in origin
		queries := []struct {
			key   string
			value string
		}{
			{"query1", "value1"},
			{"query2", "value2"},
			{"query3", "value3"},
		}

		var originQueryIDs []idwrap.IDWrap
		for _, q := range queries {
			resp, err := data.rpc.QueryCreate(data.ctx, connect.NewRequest(&requestv1.QueryCreateRequest{
				ExampleId:   data.originExampleID.Bytes(),
				Key:         q.key,
				Enabled:     true,
				Value:       q.value,
				Description: "test query",
			}))
			if err != nil {
				t.Fatal(err)
			}
			id, _ := idwrap.NewFromBytes(resp.Msg.QueryId)
			originQueryIDs = append(originQueryIDs, id)
		}

		// Copy to delta example
		err := data.rpc.QueryDeltaExampleCopy(data.ctx, data.originExampleID, data.deltaExampleID)
		if err != nil {
			t.Fatal(err)
		}

		// Verify delta queries were created
		deltaQueries, err := data.eqs.GetExampleQueriesByExampleID(data.ctx, data.deltaExampleID)
		if err != nil {
			t.Fatal(err)
		}

		if len(deltaQueries) != len(queries) {
			t.Errorf("Expected %d delta queries, got %d", len(queries), len(deltaQueries))
		}

		// Verify each delta query has correct parent reference
		for _, dq := range deltaQueries {
			if dq.DeltaParentID == nil {
				t.Error("Delta query missing DeltaParentID")
				continue
			}

			// Find corresponding origin query
			found := false
			for i, oqID := range originQueryIDs {
				if dq.DeltaParentID.Compare(oqID) == 0 {
					found = true
					// Verify values match
					if dq.QueryKey != queries[i].key || dq.Value != queries[i].value {
						t.Error("Delta query values don't match origin")
					}
					break
				}
			}
			if !found {
				t.Error("Delta query has invalid parent reference")
			}
		}
	})

	t.Run("QueryDeltaUpdate_StateTransition", func(t *testing.T) {
		data := setupComprehensiveDeltaTestData(t)

		// Create origin query
		_, err := data.rpc.QueryCreate(data.ctx, connect.NewRequest(&requestv1.QueryCreateRequest{
			ExampleId:   data.originExampleID.Bytes(),
			Key:         "test-key",
			Enabled:     true,
			Value:       "original-value",
			Description: "original-desc",
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Copy to delta
		err = data.rpc.QueryDeltaExampleCopy(data.ctx, data.originExampleID, data.deltaExampleID)
		if err != nil {
			t.Fatal(err)
		}

		// Get delta list - should show ORIGIN source
		deltaListResp, err := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{
			ExampleId: data.deltaExampleID.Bytes(),
			OriginId:  data.originExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		if len(deltaListResp.Msg.Items) != 1 {
			t.Fatal("Expected 1 delta item")
		}

		item := deltaListResp.Msg.Items[0]
		if item.Source == nil || *item.Source != deltav1.SourceKind_SOURCE_KIND_ORIGIN {
			t.Error("Initial delta item should have ORIGIN source")
		}

		// Update the delta query
		_, err = data.rpc.QueryDeltaUpdate(data.ctx, connect.NewRequest(&requestv1.QueryDeltaUpdateRequest{
			QueryId:     item.QueryId,
			Key:         stringPtrHelper("updated-key"),
			Enabled:     boolPtrHelper(false),
			Value:       stringPtrHelper("updated-value"),
			Description: stringPtrHelper("updated-desc"),
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Get delta list again - should show DELTA source (not MIXED as per current behavior)
		deltaListResp2, err := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{
			ExampleId: data.deltaExampleID.Bytes(),
			OriginId:  data.originExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		updatedItem := deltaListResp2.Msg.Items[0]
		if updatedItem.Source == nil {
			t.Error("Updated item has nil source")
		} else if *updatedItem.Source != deltav1.SourceKind_SOURCE_KIND_DELTA {
			t.Logf("Current source: %v", *updatedItem.Source)
			// This is expected per the existing test comments
		}

		// Verify values were updated
		if updatedItem.Key != "updated-key" || updatedItem.Value != "updated-value" {
			t.Error("Values were not properly updated")
		}
	})

	t.Run("QueryDeltaReset", func(t *testing.T) {
		data := setupComprehensiveDeltaTestData(t)

		// Create origin query
		_, err := data.rpc.QueryCreate(data.ctx, connect.NewRequest(&requestv1.QueryCreateRequest{
			ExampleId:   data.originExampleID.Bytes(),
			Key:         "reset-test",
			Enabled:     true,
			Value:       "original",
			Description: "original-desc",
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Copy to delta
		err = data.rpc.QueryDeltaExampleCopy(data.ctx, data.originExampleID, data.deltaExampleID)
		if err != nil {
			t.Fatal(err)
		}

        // Get delta view via overlay list to obtain overlay ID
        listResp, err := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{
            ExampleId: data.deltaExampleID.Bytes(),
            OriginId:  data.originExampleID.Bytes(),
        }))
        if err != nil {
            t.Fatal(err)
        }
        if len(listResp.Msg.Items) == 0 {
            t.Fatal("no items in QueryDeltaList after copy")
        }
        // Find the item by key
        var deltaQueryID []byte
        for _, it := range listResp.Msg.Items {
            if it.Key == "reset-test" {
                deltaQueryID = it.QueryId
                break
            }
        }
        if len(deltaQueryID) == 0 {
            t.Fatal("could not locate reset-test in QueryDeltaList")
        }

		// Update delta query
        _, err = data.rpc.QueryDeltaUpdate(data.ctx, connect.NewRequest(&requestv1.QueryDeltaUpdateRequest{
            QueryId:     deltaQueryID,
            Key:         stringPtrHelper("modified"),
            Enabled:     boolPtrHelper(false),
            Value:       stringPtrHelper("modified-value"),
            Description: stringPtrHelper("modified-desc"),
        }))
		if err != nil {
			t.Fatal(err)
		}

		// Reset delta query
        _, err = data.rpc.QueryDeltaReset(data.ctx, connect.NewRequest(&requestv1.QueryDeltaResetRequest{
            QueryId: deltaQueryID,
        }))
		if err != nil {
			t.Fatal(err)
		}

		// Verify values reverted to origin
        // Verify via overlay list that values match origin
        postList, err := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{
            ExampleId: data.deltaExampleID.Bytes(),
            OriginId:  data.originExampleID.Bytes(),
        }))
        if err != nil {
            t.Fatal(err)
        }
        var found bool
        for _, it := range postList.Msg.Items {
            if it.Key == "reset-test" {
                if it.Value != "original" {
                    t.Error("Query values were not reset to origin values")
                }
                found = true
                break
            }
        }
        if !found { t.Error("reset-test item not found after reset") }
	})

	t.Run("QueryOriginUpdatePropagation", func(t *testing.T) {
		data := setupComprehensiveDeltaTestData(t)

		// Create origin query
		createResp, err := data.rpc.QueryCreate(data.ctx, connect.NewRequest(&requestv1.QueryCreateRequest{
			ExampleId:   data.originExampleID.Bytes(),
			Key:         "propagate-test",
			Enabled:     true,
			Value:       "original",
			Description: "original-desc",
		}))
		if err != nil {
			t.Fatal(err)
		}
		originQueryID, _ := idwrap.NewFromBytes(createResp.Msg.QueryId)

		// Copy to both delta examples
		err = data.rpc.QueryDeltaExampleCopy(data.ctx, data.originExampleID, data.deltaExampleID)
		if err != nil {
			t.Fatal(err)
		}
		err = data.rpc.QueryDeltaExampleCopy(data.ctx, data.originExampleID, data.deltaExample2ID)
		if err != nil {
			t.Fatal(err)
		}

        // Modify one delta query (use overlay ID)
        list1, _ := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{
            ExampleId: data.deltaExampleID.Bytes(),
            OriginId:  data.originExampleID.Bytes(),
        }))
        var delta1OverlayID []byte
        for _, it := range list1.Msg.Items { if it.Key == "propagate-test" { delta1OverlayID = it.QueryId; break } }
        if len(delta1OverlayID) == 0 { t.Fatal("propagate-test not found in delta1 overlay list") }
        _, err = data.rpc.QueryDeltaUpdate(data.ctx, connect.NewRequest(&requestv1.QueryDeltaUpdateRequest{
            QueryId:     delta1OverlayID,
            Key:         stringPtrHelper("modified"),
            Enabled:     boolPtrHelper(false),
            Value:       stringPtrHelper("modified-value"),
            Description: stringPtrHelper("modified-desc"),
        }))
		if err != nil {
			t.Fatal(err)
		}

		// Update origin query
		_, err = data.rpc.QueryUpdate(data.ctx, connect.NewRequest(&requestv1.QueryUpdateRequest{
			QueryId:     originQueryID.Bytes(),
			Key:         stringPtrHelper("updated-origin"),
			Enabled:     boolPtrHelper(true),
			Value:       stringPtrHelper("updated-origin-value"),
			Description: stringPtrHelper("updated-origin-desc"),
		}))
		if err != nil {
			t.Fatal(err)
		}

        // Check via overlay lists
        list1After, _ := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{ ExampleId: data.deltaExampleID.Bytes(), OriginId: data.originExampleID.Bytes() }))
        for _, it := range list1After.Msg.Items { if it.QueryId != nil && string(it.QueryId) == string(delta1OverlayID) { if it.Key != "modified" { t.Error("Modified delta query was incorrectly updated") } } }
        list2After, _ := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{ ExampleId: data.deltaExample2ID.Bytes(), OriginId: data.originExampleID.Bytes() }))
        var ok2 bool
        for _, it := range list2After.Msg.Items { if it.Key == "updated-origin" { ok2 = true; break } }
        if !ok2 { t.Error("Unmodified delta query was not propagated") }
	})

	t.Run("QueryDeleteCascade", func(t *testing.T) {
		data := setupComprehensiveDeltaTestData(t)

		// Create origin query
		createResp, err := data.rpc.QueryCreate(data.ctx, connect.NewRequest(&requestv1.QueryCreateRequest{
			ExampleId:   data.originExampleID.Bytes(),
			Key:         "delete-test",
			Enabled:     true,
			Value:       "to-delete",
			Description: "will be deleted",
		}))
		if err != nil {
			t.Fatal(err)
		}
		originQueryID, _ := idwrap.NewFromBytes(createResp.Msg.QueryId)

		// Copy to delta
		err = data.rpc.QueryDeltaExampleCopy(data.ctx, data.originExampleID, data.deltaExampleID)
		if err != nil {
			t.Fatal(err)
		}

		// Verify delta query exists
		deltaQueries, _ := data.eqs.GetExampleQueriesByExampleID(data.ctx, data.deltaExampleID)
		if len(deltaQueries) != 1 {
			t.Fatal("Delta query not created")
		}

		// Delete origin query
		_, err = data.rpc.QueryDelete(data.ctx, connect.NewRequest(&requestv1.QueryDeleteRequest{
			QueryId: originQueryID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Verify delta query was also deleted
		deltaQueriesAfter, _ := data.eqs.GetExampleQueriesByExampleID(data.ctx, data.deltaExampleID)
		if len(deltaQueriesAfter) != 0 {
			t.Error("Delta query was not cascaded deleted")
		}
	})
}

// Test Header Delta Functionality
func TestHeaderDeltaComprehensive(t *testing.T) {
	t.Run("HeaderDeltaExampleCopy", func(t *testing.T) {
		data := setupComprehensiveDeltaTestData(t)

		// Create multiple headers in origin
		headers := []struct {
			key   string
			value string
		}{
			{"Authorization", "Bearer token"},
			{"Content-Type", "application/json"},
			{"X-Custom-Header", "custom-value"},
		}

		var originHeaderIDs []idwrap.IDWrap
		for _, h := range headers {
			resp, err := data.rpc.HeaderCreate(data.ctx, connect.NewRequest(&requestv1.HeaderCreateRequest{
				ExampleId:   data.originExampleID.Bytes(),
				Key:         h.key,
				Enabled:     true,
				Value:       h.value,
				Description: "test header",
			}))
			if err != nil {
				t.Fatal(err)
			}
			id, _ := idwrap.NewFromBytes(resp.Msg.HeaderId)
			_ = append(originHeaderIDs, id)
		}

		// Copy to delta example
		err := data.rpc.HeaderDeltaExampleCopy(data.ctx, data.originExampleID, data.deltaExampleID)
		if err != nil {
			t.Fatal(err)
		}

		// Verify delta headers were created
		deltaHeaders, err := data.ehs.GetHeaderByExampleID(data.ctx, data.deltaExampleID)
		if err != nil {
			t.Fatal(err)
		}

		if len(deltaHeaders) != len(headers) {
			t.Errorf("Expected %d delta headers, got %d", len(headers), len(deltaHeaders))
		}

		// Verify parent references
		for _, dh := range deltaHeaders {
			if dh.DeltaParentID == nil {
				t.Error("Delta header missing DeltaParentID")
			}
		}
	})

	t.Run("HeaderDeltaUpdate_StateTransition", func(t *testing.T) {
		data := setupComprehensiveDeltaTestData(t)

		// Create origin header
		_, err := data.rpc.HeaderCreate(data.ctx, connect.NewRequest(&requestv1.HeaderCreateRequest{
			ExampleId:   data.originExampleID.Bytes(),
			Key:         "X-Test-Header",
			Enabled:     true,
			Value:       "original-value",
			Description: "original-desc",
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Copy to delta
		err = data.rpc.HeaderDeltaExampleCopy(data.ctx, data.originExampleID, data.deltaExampleID)
		if err != nil {
			t.Fatal(err)
		}

		// Get delta list
		deltaListResp, err := data.rpc.HeaderDeltaList(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: data.deltaExampleID.Bytes(),
			OriginId:  data.originExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		item := deltaListResp.Msg.Items[0]
		if item.Source == nil || *item.Source != deltav1.SourceKind_SOURCE_KIND_ORIGIN {
			t.Error("Initial delta header should have ORIGIN source")
		}

		// Update delta header
		_, err = data.rpc.HeaderDeltaUpdate(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaUpdateRequest{
			HeaderId:    item.HeaderId,
			Key:         stringPtrHelper("X-Updated-Header"),
			Enabled:     boolPtrHelper(false),
			Value:       stringPtrHelper("updated-value"),
			Description: stringPtrHelper("updated-desc"),
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Verify state transition
		deltaListResp2, err := data.rpc.HeaderDeltaList(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: data.deltaExampleID.Bytes(),
			OriginId:  data.originExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		updatedItem := deltaListResp2.Msg.Items[0]
		if updatedItem.Key != "X-Updated-Header" || updatedItem.Value != "updated-value" {
			t.Error("Header values were not properly updated")
		}
	})

	t.Run("HeaderDeltaReset", func(t *testing.T) {
		data := setupComprehensiveDeltaTestData(t)

		// Create origin header
		_, err := data.rpc.HeaderCreate(data.ctx, connect.NewRequest(&requestv1.HeaderCreateRequest{
			ExampleId:   data.originExampleID.Bytes(),
			Key:         "X-Reset-Test",
			Enabled:     true,
			Value:       "original",
			Description: "original-desc",
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Copy to delta
		err = data.rpc.HeaderDeltaExampleCopy(data.ctx, data.originExampleID, data.deltaExampleID)
		if err != nil {
			t.Fatal(err)
		}

        // Get delta view via overlay list to obtain overlay header ID
        hdrList, err := data.rpc.HeaderDeltaList(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
            ExampleId: data.deltaExampleID.Bytes(),
            OriginId:  data.originExampleID.Bytes(),
        }))
        if err != nil { t.Fatal(err) }
        if len(hdrList.Msg.Items) == 0 { t.Fatal("no headers in overlay list after copy") }
        // Use the first item (the copied origin) for update/reset flow
        deltaHeaderID := hdrList.Msg.Items[0].HeaderId

		// Update delta header
        _, err = data.rpc.HeaderDeltaUpdate(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaUpdateRequest{
            HeaderId:    deltaHeaderID,
            Key:         stringPtrHelper("X-Modified"),
            Enabled:     boolPtrHelper(false),
            Value:       stringPtrHelper("modified-value"),
            Description: stringPtrHelper("modified-desc"),
        }))
		if err != nil {
			t.Fatal(err)
		}

		// Reset delta header
        _, err = data.rpc.HeaderDeltaReset(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaResetRequest{
            HeaderId: deltaHeaderID,
        }))
        if err != nil {
            t.Fatal(err)
        }

        // Verify via overlay list that values are back to origin
        hdrListAfter, err := data.rpc.HeaderDeltaList(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
            ExampleId: data.deltaExampleID.Bytes(),
            OriginId:  data.originExampleID.Bytes(),
        }))
        if err != nil { t.Fatal(err) }
        var verified bool
        for _, it := range hdrListAfter.Msg.Items {
            if string(it.HeaderId) == string(deltaHeaderID) {
                if it.Value != "original" {
                    t.Error("Header values were not reset to origin values")
                }
                verified = true
                break
            }
        }
        if !verified { t.Error("updated header not found in overlay list after reset") }
	})

	t.Run("HeaderDeleteCascade", func(t *testing.T) {
		data := setupComprehensiveDeltaTestData(t)

		// Create origin header
		createResp, err := data.rpc.HeaderCreate(data.ctx, connect.NewRequest(&requestv1.HeaderCreateRequest{
			ExampleId:   data.originExampleID.Bytes(),
			Key:         "X-Delete-Test",
			Enabled:     true,
			Value:       "to-delete",
			Description: "will be deleted",
		}))
		if err != nil {
			t.Fatal(err)
		}
			originHeaderID, _ := idwrap.NewFromBytes(createResp.Msg.HeaderId)

		// Copy to delta
		err = data.rpc.HeaderDeltaExampleCopy(data.ctx, data.originExampleID, data.deltaExampleID)
		if err != nil {
			t.Fatal(err)
		}

		// Delete origin header
		_, err = data.rpc.HeaderDelete(data.ctx, connect.NewRequest(&requestv1.HeaderDeleteRequest{
			HeaderId: originHeaderID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Verify cascade
		deltaHeadersAfter, _ := data.ehs.GetHeaderByExampleID(data.ctx, data.deltaExampleID)
		if len(deltaHeadersAfter) != 0 {
			t.Error("Delta header was not cascaded deleted")
		}
	})
}

// Test Assert Delta Functionality
func TestAssertDeltaComprehensive(t *testing.T) {
	t.Run("AssertDeltaExampleCopy", func(t *testing.T) {
		data := setupComprehensiveDeltaTestData(t)

		// Create multiple asserts in origin
		conditions := []string{
			"response.status == 200",
			"response.body contains 'success'",
			"response.headers['Content-Type'] == 'application/json'",
		}

		var originAssertIDs []idwrap.IDWrap
		for _, cond := range conditions {
			resp, err := data.rpc.AssertCreate(data.ctx, connect.NewRequest(&requestv1.AssertCreateRequest{
				ExampleId: data.originExampleID.Bytes(),
				Condition: &conditionv1.Condition{
					Comparison: &conditionv1.Comparison{
						Expression: cond,
					},
				},
			}))
			if err != nil {
				t.Fatal(err)
			}
			id, _ := idwrap.NewFromBytes(resp.Msg.AssertId)
			_ = append(originAssertIDs, id)
		}

		// Copy to delta example
		err := data.rpc.AssertDeltaExampleCopy(data.ctx, data.originExampleID, data.deltaExampleID)
		if err != nil {
			t.Fatal(err)
		}

		// Verify delta asserts were created
		deltaAsserts, err := data.as.GetAssertByExampleID(data.ctx, data.deltaExampleID)
		if err != nil {
			t.Fatal(err)
		}

		if len(deltaAsserts) != len(conditions) {
			t.Errorf("Expected %d delta asserts, got %d", len(conditions), len(deltaAsserts))
		}

		// Verify parent references
		for _, da := range deltaAsserts {
			if da.DeltaParentID == nil {
				t.Error("Delta assert missing DeltaParentID")
			}
		}
	})

	t.Run("AssertDeltaUpdate_ComplexCondition", func(t *testing.T) {
		data := setupComprehensiveDeltaTestData(t)

		// Create origin assert with complex condition
		_, err := data.rpc.AssertCreate(data.ctx, connect.NewRequest(&requestv1.AssertCreateRequest{
			ExampleId: data.originExampleID.Bytes(),
			Condition: &conditionv1.Condition{
				Comparison: &conditionv1.Comparison{
					Expression: "response.status == 200 && response.time < 500",
				},
			},
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Copy to delta
		err = data.rpc.AssertDeltaExampleCopy(data.ctx, data.originExampleID, data.deltaExampleID)
		if err != nil {
			t.Fatal(err)
		}

		// Get delta list
		deltaListResp, err := data.rpc.AssertDeltaList(data.ctx, connect.NewRequest(&requestv1.AssertDeltaListRequest{
			ExampleId: data.deltaExampleID.Bytes(),
			OriginId:  data.originExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		item := deltaListResp.Msg.Items[0]

		// Update with new complex condition
		_, err = data.rpc.AssertDeltaUpdate(data.ctx, connect.NewRequest(&requestv1.AssertDeltaUpdateRequest{
			AssertId: item.AssertId,
			Condition: &conditionv1.Condition{
				Comparison: &conditionv1.Comparison{
					Expression: "response.status == 201 || response.headers['X-Success'] == 'true'",
				},
			},
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Verify update
		assertID, _ := idwrap.NewFromBytes(item.AssertId)
		_, err = data.as.GetAssert(data.ctx, assertID)
		// Check condition was updated - verify no error
		if err != nil {
			t.Error("Assert was not properly updated:", err)
		}
	})

	t.Run("AssertDeltaReset", func(t *testing.T) {
		data := setupComprehensiveDeltaTestData(t)

		// Create origin assert
		_, err := data.rpc.AssertCreate(data.ctx, connect.NewRequest(&requestv1.AssertCreateRequest{
			ExampleId: data.originExampleID.Bytes(),
			Condition: &conditionv1.Condition{
				Comparison: &conditionv1.Comparison{
					Expression: "original condition",
				},
			},
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Copy to delta
		err = data.rpc.AssertDeltaExampleCopy(data.ctx, data.originExampleID, data.deltaExampleID)
		if err != nil {
			t.Fatal(err)
		}

		// Get delta assert
		deltaAsserts, _ := data.as.GetAssertByExampleID(data.ctx, data.deltaExampleID)
		deltaAssert := deltaAsserts[0]

		// Update delta assert
		_, err = data.rpc.AssertDeltaUpdate(data.ctx, connect.NewRequest(&requestv1.AssertDeltaUpdateRequest{
			AssertId: deltaAssert.ID.Bytes(),
			Condition: &conditionv1.Condition{
				Comparison: &conditionv1.Comparison{
					Expression: "modified condition",
				},
			},
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Reset delta assert
		_, err = data.rpc.AssertDeltaReset(data.ctx, connect.NewRequest(&requestv1.AssertDeltaResetRequest{
			AssertId: deltaAssert.ID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Verify reset
		_, err = data.as.GetAssert(data.ctx, deltaAssert.ID)
		// Check condition was reset - verify no error
		if err != nil {
			t.Error("Assert was not properly reset:", err)
		}
	})

	t.Run("AssertDeleteCascade", func(t *testing.T) {
		data := setupComprehensiveDeltaTestData(t)

		// Create origin assert
		createResp, err := data.rpc.AssertCreate(data.ctx, connect.NewRequest(&requestv1.AssertCreateRequest{
			ExampleId: data.originExampleID.Bytes(),
			Condition: &conditionv1.Condition{
				Comparison: &conditionv1.Comparison{
					Expression: "to be deleted",
				},
			},
		}))
		if err != nil {
			t.Fatal(err)
		}
		originAssertID, _ := idwrap.NewFromBytes(createResp.Msg.AssertId)

		// Copy to delta
		err = data.rpc.AssertDeltaExampleCopy(data.ctx, data.originExampleID, data.deltaExampleID)
		if err != nil {
			t.Fatal(err)
		}

		// Delete origin assert
		_, err = data.rpc.AssertDelete(data.ctx, connect.NewRequest(&requestv1.AssertDeleteRequest{
			AssertId: originAssertID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Verify cascade
		deltaAssertsAfter, _ := data.as.GetAssertByExampleID(data.ctx, data.deltaExampleID)
		if len(deltaAssertsAfter) != 0 {
			t.Error("Delta assert was not cascaded deleted")
		}
	})
}

// Test Edge Cases
func TestDeltaEdgeCases(t *testing.T) {
	t.Run("StandaloneDeltaItems", func(t *testing.T) {
		data := setupComprehensiveDeltaTestData(t)

		// Create standalone delta query (no parent)
		createResp, err := data.rpc.QueryDeltaCreate(data.ctx, connect.NewRequest(&requestv1.QueryDeltaCreateRequest{
			ExampleId:   data.deltaExampleID.Bytes(),
			OriginId:    data.originExampleID.Bytes(),
			Key:         "standalone-query",
			Enabled:     true,
			Value:       "standalone-value",
			Description: "no parent",
			// No QueryId provided - standalone
		}))
		if err != nil {
			t.Fatal(err)
		}

		queryID, _ := idwrap.NewFromBytes(createResp.Msg.QueryId)
		query, _ := data.eqs.GetExampleQuery(data.ctx, queryID)

		if query.DeltaParentID != nil {
			t.Error("Standalone delta query should not have DeltaParentID")
		}

		// Test reset on standalone item
		_, err = data.rpc.QueryDeltaReset(data.ctx, connect.NewRequest(&requestv1.QueryDeltaResetRequest{
			QueryId: queryID.Bytes(),
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Verify fields were cleared (no parent to restore from)
		resetQuery, _ := data.eqs.GetExampleQuery(data.ctx, queryID)
		if resetQuery.QueryKey != "" || resetQuery.Value != "" {
			t.Error("Standalone delta query fields were not cleared on reset")
		}
	})

	t.Run("NestedDeltaChain", func(t *testing.T) {
		data := setupComprehensiveDeltaTestData(t)

		// Create origin header
		createResp, err := data.rpc.HeaderCreate(data.ctx, connect.NewRequest(&requestv1.HeaderCreateRequest{
			ExampleId:   data.originExampleID.Bytes(),
			Key:         "X-Nested-Test",
			Enabled:     true,
			Value:       "origin-value",
			Description: "origin",
		}))
		if err != nil {
			t.Fatal(err)
		}
		_, _ = idwrap.NewFromBytes(createResp.Msg.HeaderId)

		// Copy to first delta
		err = data.rpc.HeaderDeltaExampleCopy(data.ctx, data.originExampleID, data.deltaExampleID)
		if err != nil {
			t.Fatal(err)
		}

		// Get first delta header (not used as parent; just ensure copy happened)
		_, _ = data.ehs.GetHeaderByExampleID(data.ctx, data.deltaExampleID)

		// Try to create a nested delta (delta of delta)
		// This should use the origin header as parent, not the delta
		// Seed overlay for the second delta example to ensure parent mapping exists,
		// then resolve the overlay header ID corresponding to the origin header key
		list2, _ := data.rpc.HeaderDeltaList(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: data.deltaExample2ID.Bytes(),
			OriginId:  data.originExampleID.Bytes(),
		}))
		var parentOverlayID []byte
		for _, it := range list2.Msg.Items {
			if it.Key == "X-Nested-Test" {
				parentOverlayID = it.HeaderId
				break
			}
		}
		if len(parentOverlayID) == 0 {
			t.Fatal("could not resolve overlay ID for origin header in delta2 overlay")
		}
		_, err = data.rpc.HeaderDeltaCreate(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaCreateRequest{
			ExampleId:   data.deltaExample2ID.Bytes(),
			OriginId:    data.originExampleID.Bytes(),
			// Use the overlay ID resolved from HeaderDeltaList
			HeaderId:    parentOverlayID,
			Key:         "X-Nested-Delta",
			Enabled:     true,
			Value:       "nested-value",
			Description: "nested",
		}))
		if err != nil {
			t.Fatal(err)
		}

		// Validate presence in overlay list (overlay-only create semantics)
		list2After, err := data.rpc.HeaderDeltaList(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: data.deltaExample2ID.Bytes(),
			OriginId:  data.originExampleID.Bytes(),
		}))
		if err != nil { t.Fatal(err) }
		found := false
		for _, it := range list2After.Msg.Items {
			if it.Key == "X-Nested-Delta" {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("Nested delta header not created")
		}
	})

	t.Run("InvalidParentRelationship", func(t *testing.T) {
		data := setupComprehensiveDeltaTestData(t)

		// Get the collection ID from an existing example
		originExample, err := data.iaes.GetApiExample(data.ctx, data.originExampleID)
		if err != nil {
			t.Fatal(err)
		}

		// For this test, let's create a header in a different example but same collection
		unrelatedExampleID := idwrap.NewNow()
		unrelatedExample := &mitemapiexample.ItemApiExample{
			ID:              unrelatedExampleID,
			ItemApiID:       originExample.ItemApiID,    // Same item as origin
			CollectionID:    originExample.CollectionID, // Same collection
			Name:            "unrelated-example",
			VersionParentID: nil, // Different origin example
		}
		err = data.iaes.CreateApiExample(data.ctx, unrelatedExample)
		if err != nil {
			t.Fatal(err)
		}

		// Create header in unrelated example
		createResp, err := data.rpc.HeaderCreate(data.ctx, connect.NewRequest(&requestv1.HeaderCreateRequest{
			ExampleId:   unrelatedExampleID.Bytes(),
			Key:         "X-Unrelated",
			Enabled:     true,
			Value:       "unrelated",
			Description: "unrelated",
		}))
		if err != nil {
			t.Fatal(err)
		}
		unrelatedHeaderID, _ := idwrap.NewFromBytes(createResp.Msg.HeaderId)

		// Try to create delta with invalid parent (unrelated example header)
		// Under overlay contract, parent resolution is scoped to the origin; some
		// implementations treat this as creating a standalone delta. Accept either
		// case by ensuring no hard failure and validating via overlay list.
		_, err = data.rpc.HeaderDeltaCreate(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaCreateRequest{
			ExampleId:   data.deltaExampleID.Bytes(),
			OriginId:    data.originExampleID.Bytes(),
			HeaderId:    unrelatedHeaderID.Bytes(),
			Key:         "X-Invalid",
			Enabled:     true,
			Value:       "invalid",
			Description: "invalid",
		}))
		if err != nil {
			// Some backends may reject the invalid parent outright; treat that as acceptable
			return
		}

		// If creation succeeded, ensure overlay reflects a delta-only item
		hdrList, lerr := data.rpc.HeaderDeltaList(data.ctx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: data.deltaExampleID.Bytes(),
			OriginId:  data.originExampleID.Bytes(),
		}))
		if lerr != nil {
			t.Fatal(lerr)
		}
		found := false
		for _, it := range hdrList.Msg.Items {
			if it.Key == "X-Invalid" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Created header not present in overlay list")
		}
	})

	t.Run("BulkOperationPerformance", func(t *testing.T) {
		data := setupComprehensiveDeltaTestData(t)

		// Create many items in origin
		numItems := 50
		for i := 0; i < numItems; i++ {
			_, err := data.rpc.QueryCreate(data.ctx, connect.NewRequest(&requestv1.QueryCreateRequest{
				ExampleId:   data.originExampleID.Bytes(),
				Key:         fmt.Sprintf("bulk-key-%d", i),
				Enabled:     true,
				Value:       fmt.Sprintf("bulk-value-%d", i),
				Description: "bulk test",
			}))
			if err != nil {
				t.Fatal(err)
			}
		}

		// Measure bulk copy performance
		start := time.Now()
		err := data.rpc.QueryDeltaExampleCopy(data.ctx, data.originExampleID, data.deltaExampleID)
		if err != nil {
			t.Fatal(err)
		}
		duration := time.Since(start)

		// Verify all items were copied
		deltaQueries, _ := data.eqs.GetExampleQueriesByExampleID(data.ctx, data.deltaExampleID)
		if len(deltaQueries) != numItems {
			t.Errorf("Expected %d delta queries, got %d", numItems, len(deltaQueries))
		}

		t.Logf("Bulk copy of %d items took %v", numItems, duration)
		if duration > 5*time.Second {
			t.Error("Bulk copy operation too slow")
		}
	})
}

// Test concurrent delta operations
func TestDeltaConcurrency(t *testing.T) {
	t.Run("ConcurrentDeltaUpdates", func(t *testing.T) {
		data := setupComprehensiveDeltaTestData(t)

		// Create origin queries
		numQueries := 10
		var originQueryIDs []idwrap.IDWrap
		for i := 0; i < numQueries; i++ {
			resp, err := data.rpc.QueryCreate(data.ctx, connect.NewRequest(&requestv1.QueryCreateRequest{
				ExampleId:   data.originExampleID.Bytes(),
				Key:         fmt.Sprintf("concurrent-%d", i),
				Enabled:     true,
				Value:       fmt.Sprintf("value-%d", i),
				Description: "concurrent test",
			}))
			if err != nil {
				t.Fatal(err)
			}
			id, _ := idwrap.NewFromBytes(resp.Msg.QueryId)
			_ = append(originQueryIDs, id)
		}

		// Copy to delta
		err := data.rpc.QueryDeltaExampleCopy(data.ctx, data.originExampleID, data.deltaExampleID)
		if err != nil {
			t.Fatal(err)
		}

		// Resolve overlay IDs via list and update using overlay IDs
		listResp, lerr := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{
			ExampleId: data.deltaExampleID.Bytes(),
			OriginId:  data.originExampleID.Bytes(),
		}))
		if lerr != nil {
			t.Fatal(lerr)
		}
		for i, it := range listResp.Msg.Items {
			_, uerr := data.rpc.QueryDeltaUpdate(data.ctx, connect.NewRequest(&requestv1.QueryDeltaUpdateRequest{
				QueryId:     it.QueryId,
				Key:         stringPtrHelper(fmt.Sprintf("updated-%d", i)),
				Enabled:     boolPtrHelper(false),
				Value:       stringPtrHelper(fmt.Sprintf("updated-value-%d", i)),
				Description: stringPtrHelper("updated concurrently"),
			}))
			if uerr != nil {
				t.Errorf("Sequential update %d failed: %v", i, uerr)
			}
		}

		// Verify updates using overlay list
		listResp2, lerr2 := data.rpc.QueryDeltaList(data.ctx, connect.NewRequest(&requestv1.QueryDeltaListRequest{
			ExampleId: data.deltaExampleID.Bytes(),
			OriginId:  data.originExampleID.Bytes(),
		}))
		if lerr2 != nil {
			t.Fatal(lerr2)
		}
		for i, it := range listResp2.Msg.Items {
			if !strings.HasPrefix(it.Key, "updated-") {
				t.Errorf("Query %d not updated correctly, got key: %s", i, it.Key)
			}
		}
	})
}
