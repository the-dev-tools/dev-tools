package rrequest_test

import (
	"context"
	"strings"
	"testing"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rrequest"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/model/mexampleheader"
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
	deltav1 "the-dev-tools/spec/dist/buf/go/delta/v1"

	"connectrpc.com/connect"
)

// TestCopyPasteScenarioWithoutVersionParent simulates the exact copy-paste scenario
// where delta examples are created without VersionParentID (as happens via the API)
func TestCopyPasteScenarioWithoutVersionParent(t *testing.T) {
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

	// Create base endpoint
	baseEndpoint := &mitemapi.ItemApi{
		ID:           idwrap.NewNow(),
		Name:         "base-endpoint",
		Url:          "http://example.com/api",
		Method:       "GET",
		CollectionID: collectionID,
	}
	err := ias.CreateItemApi(ctx, baseEndpoint)
	if err != nil {
		t.Fatal(err)
	}

	// Create origin example
	originExampleID := idwrap.NewNow()
	originExample := &mitemapiexample.ItemApiExample{
		ID:              originExampleID,
		ItemApiID:       baseEndpoint.ID,
		CollectionID:    collectionID,
		Name:            "origin-example",
		VersionParentID: nil, // This is the origin
	}
	err = iaes.CreateApiExample(ctx, originExample)
	if err != nil {
		t.Fatal(err)
	}

	// Create headers in origin example
	headers := []mexampleheader.Header{
		{
			ID:        idwrap.NewNow(),
			ExampleID: originExampleID,
			HeaderKey: "Authorization",
			Value:     "Bearer token123",
			Enable:    true,
		},
		{
			ID:        idwrap.NewNow(),
			ExampleID: originExampleID,
			HeaderKey: "Content-Type",
			Value:     "application/json",
			Enable:    true,
		},
		{
			ID:        idwrap.NewNow(),
			ExampleID: originExampleID,
			HeaderKey: "Accept",
			Value:     "*/*",
			Enable:    true,
		},
	}
	for _, h := range headers {
		err = ehs.CreateHeader(ctx, h)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Simulate first node creation - create delta endpoint and example
	deltaEndpoint1 := &mitemapi.ItemApi{
		ID:           idwrap.NewNow(),
		Name:         "delta-endpoint-1",
		Url:          "http://example.com/api", // Same as base
		Method:       "GET",
		CollectionID: collectionID,
		Hidden:       true, // Hidden delta endpoint
	}
	err = ias.CreateItemApi(ctx, deltaEndpoint1)
	if err != nil {
		t.Fatal(err)
	}

	// Create delta example WITHOUT VersionParentID (simulating API behavior)
	deltaExample1ID := idwrap.NewNow()
	deltaExample1 := &mitemapiexample.ItemApiExample{
		ID:              deltaExample1ID,
		ItemApiID:       deltaEndpoint1.ID,
		CollectionID:    collectionID,
		Name:            "delta-example-1",
		VersionParentID: nil, // NOT SET - this is the issue
	}
	err = iaes.CreateApiExample(ctx, deltaExample1)
	if err != nil {
		t.Fatal(err)
	}

	// Call HeaderDeltaList to trigger auto-creation
	req1 := &requestv1.HeaderDeltaListRequest{
		ExampleId: deltaExample1ID.Bytes(),
		OriginId:  originExampleID.Bytes(),
	}
	resp1, err := rpc.HeaderDeltaList(ctx, connect.NewRequest(req1))
	if err != nil {
		t.Fatal(err)
	}

	// Verify headers were auto-created
	if len(resp1.Msg.Items) != 3 {
		t.Fatalf("Expected 3 headers after auto-creation, got %d", len(resp1.Msg.Items))
	}

	// Modify one header in delta1
	var authHeaderID idwrap.IDWrap
	for _, item := range resp1.Msg.Items {
		if item.Key == "Authorization" {
			authHeaderID, _ = idwrap.NewFromBytes(item.HeaderId)
			break
		}
	}

	updateReq := &requestv1.HeaderDeltaUpdateRequest{
		HeaderId: authHeaderID.Bytes(),
		Value:    stringPtrHelper("Bearer modified-token"),
	}
	_, err = rpc.HeaderDeltaUpdate(ctx, connect.NewRequest(updateReq))
	if err != nil {
		t.Fatal(err)
	}

	// Create a new header in delta1
	createReq := &requestv1.HeaderDeltaCreateRequest{
		ExampleId: deltaExample1ID.Bytes(),
		OriginId:  originExampleID.Bytes(),
		Key:       "X-Custom-Header",
		Value:     "custom-value",
		Enabled:   true,
	}
	_, err = rpc.HeaderDeltaCreate(ctx, connect.NewRequest(createReq))
	if err != nil {
		t.Fatal(err)
	}

	// Now simulate copy-paste: create another delta endpoint and example
	deltaEndpoint2 := &mitemapi.ItemApi{
		ID:           idwrap.NewNow(),
		Name:         "delta-endpoint-2",
		Url:          "http://example.com/api",
		Method:       "GET",
		CollectionID: collectionID,
		Hidden:       true,
	}
	err = ias.CreateItemApi(ctx, deltaEndpoint2)
	if err != nil {
		t.Fatal(err)
	}

	deltaExample2ID := idwrap.NewNow()
	deltaExample2 := &mitemapiexample.ItemApiExample{
		ID:              deltaExample2ID,
		ItemApiID:       deltaEndpoint2.ID,
		CollectionID:    collectionID,
		Name:            "delta-example-2",
		VersionParentID: nil, // NOT SET - simulating API behavior
	}
	err = iaes.CreateApiExample(ctx, deltaExample2)
	if err != nil {
		t.Fatal(err)
	}

	// Copy only modified items from delta1 to delta2 (simulating frontend logic)
	// First, get the list from delta1
	listReq := &requestv1.HeaderDeltaListRequest{
		ExampleId: deltaExample1ID.Bytes(),
		OriginId:  originExampleID.Bytes(),
	}
	listResp, err := rpc.HeaderDeltaList(ctx, connect.NewRequest(listReq))
	if err != nil {
		t.Fatal(err)
	}

	// Debug: log source headers
	t.Logf("Source headers from delta1: %d", len(listResp.Msg.Items))
	for _, item := range listResp.Msg.Items {
		sourceStr := "UNKNOWN"
		if item.Source != nil {
			sourceStr = item.Source.String()
		}
		t.Logf("  %s = %s (source: %s)", item.Key, item.Value, sourceStr)
	}

	// Copy only modified items
	copiedCount := 0
	for _, item := range listResp.Msg.Items {
		// Skip ORIGIN items
		if item.Source != nil && *item.Source == deltav1.SourceKind_SOURCE_KIND_ORIGIN {
			continue
		}

		// For MIXED/DELTA items, check if actually modified
		if item.Origin != nil {
			isModified := item.Key != item.Origin.Key ||
				item.Enabled != item.Origin.Enabled ||
				item.Value != item.Origin.Value ||
				item.Description != item.Origin.Description

			if !isModified {
				continue
			}
		}

		// Copy the item
		var originHeaderID []byte
		if item.Origin != nil {
			originHeaderID = item.Origin.HeaderId
		}

		copyReq := &requestv1.HeaderDeltaCreateRequest{
			ExampleId:   deltaExample2ID.Bytes(),
			OriginId:    originExampleID.Bytes(),
			Key:         item.Key,
			Value:       item.Value,
			Enabled:     item.Enabled,
			Description: item.Description,
		}
		t.Logf("Creating header with key='%s', value='%s'", item.Key, item.Value)
		if len(originHeaderID) > 0 {
			copyReq.HeaderId = originHeaderID
		}

		_, err = rpc.HeaderDeltaCreate(ctx, connect.NewRequest(copyReq))
		if err != nil {
			t.Fatal(err)
		}
		copiedCount++
		t.Logf("Copied header: %s = %s", item.Key, item.Value)
	}
	t.Logf("Total headers copied: %d", copiedCount)

	// Now call HeaderDeltaList on delta2 and verify results
	finalReq := &requestv1.HeaderDeltaListRequest{
		ExampleId: deltaExample2ID.Bytes(),
		OriginId:  originExampleID.Bytes(),
	}
	finalResp, err := rpc.HeaderDeltaList(ctx, connect.NewRequest(finalReq))
	if err != nil {
		t.Fatal(err)
	}

	// Debug output
	t.Logf("Final headers count: %d", len(finalResp.Msg.Items))
	for _, item := range finalResp.Msg.Items {
		sourceStr := "UNKNOWN"
		if item.Source != nil {
			sourceStr = item.Source.String()
		}
		t.Logf("Header: %s = %s (source: %s)", item.Key, item.Value, sourceStr)
	}

	// Verify we have all headers
	if len(finalResp.Msg.Items) == 0 {
		t.Fatal("No headers returned! This reproduces the reported issue.")
	}

	// We should have:
	// - 2 auto-created ORIGIN headers (Content-Type, Accept)
	// - 1 copied modified header (Authorization)
	// - 1 copied new header (X-Custom-Header)
	expectedCount := 4
	if len(finalResp.Msg.Items) != expectedCount {
		t.Fatalf("Expected %d headers, got %d", expectedCount, len(finalResp.Msg.Items))
	}

	// Verify each header and check for duplicates
	headerMap := make(map[string]*requestv1.HeaderDeltaListItem)
	headerCounts := make(map[string]int)
	for _, item := range finalResp.Msg.Items {
		headerMap[item.Key] = item
		headerCounts[strings.ToLower(item.Key)]++
	}
	
	// Check for duplicate headers
	for key, count := range headerCounts {
		if count > 1 {
			t.Errorf("Header %s appears %d times (should be unique)", key, count)
		}
	}

	// Check Authorization (should be modified)
	// First check if we have an entry with empty key (bug)
	var authHeader *requestv1.HeaderDeltaListItem
	for _, item := range finalResp.Msg.Items {
		if item.Value == "Bearer modified-token" || item.Key == "Authorization" {
			authHeader = item
			break
		}
	}
	
	if authHeader != nil {
		if authHeader.Key != "Authorization" {
			t.Errorf("Authorization header has wrong key: '%s'", authHeader.Key)
		}
		if authHeader.Value != "Bearer modified-token" {
			t.Errorf("Authorization header has wrong value: %s", authHeader.Value)
		}
		if authHeader.Source == nil || *authHeader.Source == deltav1.SourceKind_SOURCE_KIND_ORIGIN {
			t.Error("Authorization should not be ORIGIN")
		}
	} else {
		t.Error("Authorization header missing")
	}

	// Check Content-Type (should be ORIGIN)
	if ct, ok := headerMap["Content-Type"]; ok {
		if ct.Value != "application/json" {
			t.Errorf("Content-Type header has wrong value: %s", ct.Value)
		}
		if ct.Source == nil || *ct.Source != deltav1.SourceKind_SOURCE_KIND_ORIGIN {
			t.Error("Content-Type should be ORIGIN")
		}
	} else {
		t.Error("Content-Type header missing")
	}

	// Check custom header
	if custom, ok := headerMap["X-Custom-Header"]; ok {
		if custom.Value != "custom-value" {
			t.Errorf("X-Custom-Header has wrong value: %s", custom.Value)
		}
	} else {
		t.Error("X-Custom-Header missing")
	}
}