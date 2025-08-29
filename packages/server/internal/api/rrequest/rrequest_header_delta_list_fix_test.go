package rrequest_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
)

// TestHeaderDeltaListWithProperDeltaExample tests that HeaderDeltaList works correctly
// when given a proper delta example with version_parent_id set
func TestHeaderDeltaListWithProperDeltaExample(t *testing.T) {
	t.Parallel()
	
	// Setup test database and context
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	
	// Initialize services
	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	mockLogger := mocklogger.NewMockLogger()
	cs := scollection.New(queries, mockLogger)
	us := suser.New(queries)
	ehs := sexampleheader.New(queries)
	eqs := sexamplequery.New(queries)
	as := sassert.New(queries)
	
	// Create RPC handler
	rpc := rrequest.New(base.DB, cs, us, ias, iaes, ehs, eqs, as)
	
	// Create workspace and collection
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	userID := idwrap.NewNow()
	
	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)
	
	// Create authenticated context
	authCtx := mwauth.CreateAuthedContext(ctx, userID)
	
	// Create origin endpoint
	originEndpointID := idwrap.NewNow()
	originEndpoint := &mitemapi.ItemApi{
		ID:           originEndpointID,
		CollectionID: collectionID,
		Name:         "test-origin",
		Method:       "GET",
		Url:          "/test",
		Hidden:       false,
	}
	err := ias.CreateItemApi(authCtx, originEndpoint)
	require.NoError(t, err)
	
	// Create origin example
	originExampleID := idwrap.NewNow()
	originExample := &mitemapiexample.ItemApiExample{
		ID:           originExampleID,
		ItemApiID:    originEndpointID,
		CollectionID: collectionID,
		Name:         "Origin Example",
	}
	err = iaes.CreateApiExample(authCtx, originExample)
	require.NoError(t, err)
	
	// Create delta endpoint (hidden)
	deltaEndpointID := idwrap.NewNow()
	deltaEndpoint := &mitemapi.ItemApi{
		ID:           deltaEndpointID,
		CollectionID: collectionID,
		Name:         "test-delta",
		Method:       "GET",
		Url:          "/test-delta",
		Hidden:       true,
	}
	err = ias.CreateItemApi(authCtx, deltaEndpoint)
	require.NoError(t, err)
	
	// Create delta example with VersionParentID pointing to origin
	deltaExampleID := idwrap.NewNow()
	deltaExample := &mitemapiexample.ItemApiExample{
		ID:              deltaExampleID,
		ItemApiID:       deltaEndpointID,
		CollectionID:    collectionID,
		Name:            "Delta Example",
		VersionParentID: &originExampleID, // This is the key - points to origin
	}
	err = iaes.CreateApiExample(authCtx, deltaExample)
	require.NoError(t, err)
	
	// Create headers in origin example using AppendHeader for proper linked list
	headers := []string{"Authorization", "Content-Type", "X-API-Key", "User-Agent"}
	for _, headerKey := range headers {
		header := mexampleheader.Header{
			ID:          idwrap.NewNow(),
			ExampleID:   originExampleID,
			HeaderKey:   headerKey,
			Value:       "value-" + headerKey,
			Enable:      true,
			Description: "Test header " + headerKey,
		}
		err := ehs.AppendHeader(authCtx, header)
		require.NoError(t, err)
	}
	
	// Create request for HeaderDeltaList
	req := connect.NewRequest(&requestv1.HeaderDeltaListRequest{
		ExampleId: deltaExampleID.Bytes(),
		OriginId:  originExampleID.Bytes(),
	})
	
	// Execute HeaderDeltaList
	resp, err := rpc.HeaderDeltaList(authCtx, req)
	require.NoError(t, err, "HeaderDeltaList should not fail with proper delta example")
	require.NotNil(t, resp)
	
	// Verify response
	assert.Len(t, resp.Msg.Items, len(headers), "Should auto-create delta headers for all origin headers")
	
	// Verify headers are in correct order
	for i, expectedKey := range headers {
		assert.Equal(t, expectedKey, resp.Msg.Items[i].Key, 
			"Header at position %d should be %s", i, expectedKey)
	}
	
	// Log header sources for debugging
	for _, header := range resp.Msg.Items {
		t.Logf("Header %s has source: %v", header.Key, header.Source)
	}
}

// TestHeaderDeltaListForeignKeyError reproduces the foreign key constraint error
func TestHeaderDeltaListForeignKeyError(t *testing.T) {
	t.Parallel()
	
	// Setup test database and context
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	
	// Initialize services
	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	mockLogger := mocklogger.NewMockLogger()
	cs := scollection.New(queries, mockLogger)
	us := suser.New(queries)
	ehs := sexampleheader.New(queries)
	eqs := sexamplequery.New(queries)
	as := sassert.New(queries)
	
	// Create RPC handler
	rpc := rrequest.New(base.DB, cs, us, ias, iaes, ehs, eqs, as)
	
	// Create workspace and collection
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	userID := idwrap.NewNow()
	
	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)
	
	// Create authenticated context
	authCtx := mwauth.CreateAuthedContext(ctx, userID)
	
	// Create origin endpoint
	originEndpointID := idwrap.NewNow()
	originEndpoint := &mitemapi.ItemApi{
		ID:           originEndpointID,
		CollectionID: collectionID,
		Name:         "test-origin",
		Method:       "GET",
		Url:          "/test",
		Hidden:       false,
	}
	err := ias.CreateItemApi(authCtx, originEndpoint)
	require.NoError(t, err)
	
	// Create origin example
	originExampleID := idwrap.NewNow()
	originExample := &mitemapiexample.ItemApiExample{
		ID:           originExampleID,
		ItemApiID:    originEndpointID,
		CollectionID: collectionID,
		Name:         "Origin Example",
	}
	err = iaes.CreateApiExample(authCtx, originExample)
	require.NoError(t, err)
	
	// Create a header in origin example
	header := mexampleheader.Header{
		ID:          idwrap.NewNow(),
		ExampleID:   originExampleID,
		HeaderKey:   "Test-Header",
		Value:       "test-value",
		Enable:      true,
		Description: "Test header",
	}
	err = ehs.AppendHeader(authCtx, header)
	require.NoError(t, err)
	
	t.Run("NonExistentDeltaExample", func(t *testing.T) {
		// Try to call HeaderDeltaList with non-existent delta example ID
		nonExistentID := idwrap.NewNow()
		
		req := connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: nonExistentID.Bytes(),
			OriginId:  originExampleID.Bytes(),
		})
		
		_, err := rpc.HeaderDeltaList(authCtx, req)
		assert.Error(t, err, "Should fail with non-existent delta example")
		// The error will be permission denied because the example doesn't exist
		assert.Contains(t, err.Error(), "permission denied", 
			"Should fail permission check for non-existent example")
	})
	
	t.Run("DeltaExampleWithoutVersionParent", func(t *testing.T) {
		// Create an example WITHOUT version_parent_id (not a proper delta)
		improperDeltaID := idwrap.NewNow()
		improperDelta := &mitemapiexample.ItemApiExample{
			ID:           improperDeltaID,
			ItemApiID:    originEndpointID, // Using same endpoint as origin
			CollectionID: collectionID,
			Name:         "Improper Delta",
			// Note: VersionParentID is nil - this is not a proper delta
		}
		err := iaes.CreateApiExample(authCtx, improperDelta)
		require.NoError(t, err)
		
		req := connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: improperDeltaID.Bytes(),
			OriginId:  originExampleID.Bytes(),
		})
		
		// This should work but won't auto-create headers properly
		resp, err := rpc.HeaderDeltaList(authCtx, req)
		require.NoError(t, err, "Should not fail even without version_parent_id")
		
		// The response will depend on implementation - it might return origin headers
		// or might return empty if it doesn't find delta headers
		t.Logf("Headers returned for improper delta: %d", len(resp.Msg.Items))
	})
}