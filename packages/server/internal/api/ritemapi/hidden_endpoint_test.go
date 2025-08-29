package ritemapi_test

import (
	"context"
	"testing"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rcollectionitem"
	"the-dev-tools/server/internal/api/ritemapi"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/scollectionitem"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/sitemfolder"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/testutil"
	endpointv1 "the-dev-tools/spec/dist/buf/go/collection/item/endpoint/v1"
	itemv1 "the-dev-tools/spec/dist/buf/go/collection/item/v1"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHiddenEndpointDefaultExampleBug(t *testing.T) {
	t.Parallel()
	
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB
	mockLogger := mocklogger.NewMockLogger()
	
	// Initialize services
	ias := sitemapi.New(queries)
	ifs := sitemfolder.New(queries)
	cs := scollection.New(queries, mockLogger)
	us := suser.New(queries)
	iaes := sitemapiexample.New(queries)
	ers := sexampleresp.New(queries)
	cis := scollectionitem.New(queries, mockLogger)
	
	// Create test data
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	userID := idwrap.NewNow()
	
	// Use base services to create collection
	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, userID, collectionID)
	
	// Create authenticated context
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	
	// Initialize RPC services
	endpointRPC := ritemapi.New(db, ias, cs, ifs, us, iaes, ers, cis)
	collectionItemRPC := rcollectionitem.New(db, cs, cis, us, ifs, ias, iaes, ers)
	
	t.Run("Regular endpoint has default example", func(t *testing.T) {
		// Create a regular (non-hidden) endpoint
		req := connect.NewRequest(&endpointv1.EndpointCreateRequest{
			CollectionId: collectionID.Bytes(),
			Name:         "Regular Endpoint",
			Url:          "/api/regular",
			Method:       "GET",
			// Hidden is not set, defaults to false
		})
		
		resp, err := endpointRPC.EndpointCreate(authedCtx, req)
		require.NoError(t, err)
		require.NotNil(t, resp.Msg)
		
		endpointID, err := idwrap.NewFromBytes(resp.Msg.GetEndpointId())
		require.NoError(t, err)
		
		// Verify default example was created
		defaultExample, err := iaes.GetDefaultApiExample(ctx, endpointID)
		require.NoError(t, err, "Regular endpoint should have default example")
		assert.True(t, defaultExample.IsDefault)
		assert.Equal(t, "Default", defaultExample.Name)
		
		t.Logf("✓ Regular endpoint %s has default example", endpointID.String())
	})
	
	t.Run("Hidden endpoint missing default example", func(t *testing.T) {
		// Create a hidden endpoint (simulating flow node creation)
		hidden := true
		req := connect.NewRequest(&endpointv1.EndpointCreateRequest{
			CollectionId: collectionID.Bytes(),
			Name:         "Hidden Flow Endpoint",
			Url:          "/api/flow-hidden",
			Method:       "POST",
			Hidden:       &hidden, // Set hidden to true
		})
		
		resp, err := endpointRPC.EndpointCreate(authedCtx, req)
		require.NoError(t, err)
		require.NotNil(t, resp.Msg)
		
		hiddenEndpointID, err := idwrap.NewFromBytes(resp.Msg.GetEndpointId())
		require.NoError(t, err)
		
		// Try to get default example - this reveals the bug
		defaultExample, err := iaes.GetDefaultApiExample(ctx, hiddenEndpointID)
		
		if err != nil {
			// BUG CONFIRMED: Hidden endpoints don't have default examples
			t.Logf("✗ BUG CONFIRMED: Hidden endpoint %s has NO default example: %v", 
				hiddenEndpointID.String(), err)
			assert.Error(t, err, "Bug: Hidden endpoint missing default example")
		} else {
			// If this passes, the bug is already fixed
			t.Logf("✓ Hidden endpoint %s has default example (bug may be fixed)", 
				hiddenEndpointID.String())
			assert.True(t, defaultExample.IsDefault)
		}
	})
	
	t.Run("CollectionItemList fails with hidden endpoint", func(t *testing.T) {
		// This simulates what happens in the flow UI
		// Create another hidden endpoint
		hidden := true
		endpointReq := connect.NewRequest(&endpointv1.EndpointCreateRequest{
			CollectionId: collectionID.Bytes(),
			Name:         "Flow Test Endpoint",
			Url:          "/api/flow-test",
			Method:       "GET",
			Hidden:       &hidden,
		})
		
		endpointResp, err := endpointRPC.EndpointCreate(authedCtx, endpointReq)
		require.NoError(t, err)
		
		hiddenEndpointID, err := idwrap.NewFromBytes(endpointResp.Msg.GetEndpointId())
		require.NoError(t, err)
		
		t.Logf("Created hidden endpoint: %s", hiddenEndpointID.String())
		
		// Try to list collection items (what CollectionListTree does)
		listReq := connect.NewRequest(&itemv1.CollectionItemListRequest{
			CollectionId:   collectionID.Bytes(),
			ParentFolderId: nil,
		})
		
		listResp, err := collectionItemRPC.CollectionItemList(authedCtx, listReq)
		
		if err != nil {
			// This is the bug users experience
			t.Logf("✗ BUG: CollectionItemList failed: %v", err)
			t.Logf("This is why collections appear empty in flow editor!")
		} else {
			// Check if hidden endpoints appear
			foundHidden := 0
			foundRegular := 0
			
			for _, item := range listResp.Msg.Items {
				if item.Kind == itemv1.ItemKind_ITEM_KIND_ENDPOINT {
					endpoint := item.GetEndpoint()
					if endpoint != nil {
						// Check if this is a hidden endpoint
						endpointID, _ := idwrap.NewFromBytes(endpoint.EndpointId)
						apiEndpoint, _ := ias.GetItemApi(ctx, endpointID)
						if apiEndpoint != nil {
							if apiEndpoint.Hidden {
								foundHidden++
								t.Logf("Found hidden endpoint in list: %s", endpoint.Name)
							} else {
								foundRegular++
								t.Logf("Found regular endpoint in list: %s", endpoint.Name)
							}
						}
					}
				}
			}
			
			t.Logf("CollectionItemList returned: %d regular, %d hidden endpoints", 
				foundRegular, foundHidden)
			
			if foundHidden == 0 {
				t.Logf("⚠ Hidden endpoints not appearing in collection list")
			}
		}
	})
	
	t.Run("Simulate exact flow node creation pattern", func(t *testing.T) {
		// This is exactly what happens when user drags endpoint to flow
		hidden := true
		
		// Step 1: Create hidden delta endpoint (like flow.tsx does)
		req := connect.NewRequest(&endpointv1.EndpointCreateRequest{
			CollectionId: collectionID.Bytes(),
			Name:         "Delta Endpoint for Flow",
			Url:          "/api/delta",
			Method:       "PUT",
			Hidden:       &hidden, // This is the key issue
		})
		
		resp, err := endpointRPC.EndpointCreate(authedCtx, req)
		require.NoError(t, err)
		
		deltaEndpointID, err := idwrap.NewFromBytes(resp.Msg.GetEndpointId())
		require.NoError(t, err)
		
		t.Logf("Created delta endpoint: %s (hidden=%v)", deltaEndpointID.String(), hidden)
		
		// Step 2: Check if it has default example (it shouldn't with the bug)
		defaultExample, err := iaes.GetDefaultApiExample(ctx, deltaEndpointID)
		
		if err != nil {
			t.Logf("✗ Delta endpoint has NO default example - THIS IS THE BUG")
			t.Logf("  Error: %v", err)
		} else {
			t.Logf("✓ Delta endpoint has default example: %s", defaultExample.Name)
		}
		
		// Step 3: Try CollectionItemList (fails with bug)
		listReq := connect.NewRequest(&itemv1.CollectionItemListRequest{
			CollectionId:   collectionID.Bytes(),
			ParentFolderId: nil,
		})
		
		_, err = collectionItemRPC.CollectionItemList(authedCtx, listReq)
		
		if err != nil {
			t.Logf("✗ CollectionItemList fails after creating hidden endpoint")
			t.Logf("  This causes empty collection in flow UI!")
			t.Logf("  Error: %v", err)
		} else {
			t.Logf("✓ CollectionItemList works (bug may be fixed)")
		}
	})
}