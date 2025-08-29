package ritemapi_test

import (
	"context"
	"testing"
	"the-dev-tools/server/internal/api/middleware/mwauth"
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

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
)

func TestEndpointDuplicationAppearsInCollectionItemList(t *testing.T) {
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
	
	// Initialize RPC service
	endpointRPC := ritemapi.New(db, ias, cs, ifs, us, iaes, ers, cis)
	
	t.Run("Duplicated endpoint appears in collection item list", func(t *testing.T) {
		// Step 1: Create an endpoint
		createReq := connect.NewRequest(&endpointv1.EndpointCreateRequest{
			CollectionId: collectionID.Bytes(),
			Name:         "Original Endpoint",
			Url:          "/api/original",
			Method:       "GET",
		})
		
		createResp, err := endpointRPC.EndpointCreate(authedCtx, createReq)
		require.NoError(t, err)
		
		endpointID, err := idwrap.NewFromBytes(createResp.Msg.GetEndpointId())
		require.NoError(t, err)
		
		// Step 2: Verify the original endpoint appears in collection items
		originalItems, err := cis.ListCollectionItems(ctx, collectionID, nil)
		require.NoError(t, err)
		require.Equal(t, 1, len(originalItems), "Should have 1 endpoint in collection")
		require.NotNil(t, originalItems[0].EndpointID)
		require.Equal(t, endpointID, *originalItems[0].EndpointID)
		
		t.Logf("✓ Original endpoint appears in collection items")
		
		// Step 3: Duplicate the endpoint
		dupReq := connect.NewRequest(&endpointv1.EndpointDuplicateRequest{
			EndpointId: endpointID.Bytes(),
		})
		
		dupResp, err := endpointRPC.EndpointDuplicate(authedCtx, dupReq)
		require.NoError(t, err)
		require.NotNil(t, dupResp)
		require.NotNil(t, dupResp.Msg)
		require.NotEmpty(t, dupResp.Msg.EndpointId, "Should return new endpoint ID")
		
		newEndpointID, err := idwrap.NewFromBytes(dupResp.Msg.EndpointId)
		require.NoError(t, err)
		
		// Step 4: Verify BOTH endpoints appear in collection items
		afterDupItems, err := cis.ListCollectionItems(ctx, collectionID, nil)
		require.NoError(t, err)
		require.Equal(t, 2, len(afterDupItems), "Should have 2 endpoints after duplication")
		
		// Verify both endpoint IDs are present
		foundOriginal := false
		foundDuplicate := false
		
		for _, item := range afterDupItems {
			if item.EndpointID != nil {
				if item.EndpointID.Compare(endpointID) == 0 {
					foundOriginal = true
					t.Logf("✓ Found original endpoint: %s", endpointID.String())
				} else if item.EndpointID.Compare(newEndpointID) == 0 {
					foundDuplicate = true
					t.Logf("✓ Found duplicated endpoint: %s", newEndpointID.String())
				}
			}
		}
		
		require.True(t, foundOriginal, "Original endpoint should still be in collection items")
		require.True(t, foundDuplicate, "Duplicated endpoint should appear in collection items")
		
		// Step 5: Verify the duplicated endpoint has correct name
		duplicatedEndpoint, err := ias.GetItemApi(ctx, newEndpointID)
		require.NoError(t, err)
		require.Equal(t, "Original Endpoint Copy", duplicatedEndpoint.Name)
		
		t.Logf("✓ SUCCESS: Duplicated endpoint appears in CollectionItemList!")
		t.Logf("  Original ID: %s", endpointID.String())
		t.Logf("  Duplicate ID: %s", newEndpointID.String())
		t.Logf("  Total items in collection: %d", len(afterDupItems))
	})
}