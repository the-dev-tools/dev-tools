package ritemapi_test

import (
	"context"
	"testing"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/ritemapi"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/model/mitemapi"
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

func TestEndpointDuplication(t *testing.T) {
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
	
	t.Run("Duplicate endpoint without default example", func(t *testing.T) {
		// Step 1: Create an endpoint manually without default example (simulating old endpoints)
		endpoint := &mitemapi.ItemApi{
			ID:           idwrap.NewNow(),
			Name:         "Old Endpoint",
			Url:          "/api/old",
			Method:       "GET",
			CollectionID: collectionID,
			Hidden:       false,
		}
		
		err := ias.CreateItemApi(ctx, endpoint)
		require.NoError(t, err)
		
		t.Logf("Created endpoint without default example: %s", endpoint.ID.String())
		
		// Step 2: Try to duplicate the endpoint
		dupReq := connect.NewRequest(&endpointv1.EndpointDuplicateRequest{
			EndpointId: endpoint.ID.Bytes(),
		})
		
		dupResp, err := endpointRPC.EndpointDuplicate(authedCtx, dupReq)
		require.NoError(t, err, "Should be able to duplicate endpoint without default example")
		require.NotNil(t, dupResp)
		require.NotNil(t, dupResp.Msg)
		require.NotEmpty(t, dupResp.Msg.EndpointId, "Should return new endpoint ID")
		
		newEndpointID, err := idwrap.NewFromBytes(dupResp.Msg.EndpointId)
		require.NoError(t, err)
		t.Logf("✓ Successfully duplicated endpoint without default example: new ID = %s", newEndpointID.String())
		
		if len(dupResp.Msg.ExampleId) > 0 {
			exampleID, err := idwrap.NewFromBytes(dupResp.Msg.ExampleId)
			require.NoError(t, err)
			t.Logf("✓ Duplicate has example ID: %s", exampleID.String())
		}
		
		t.Logf("✓ Duplication works for endpoints without default examples")
	})
	
	t.Run("Duplicate endpoint with examples", func(t *testing.T) {
		// Create endpoint with default example (using our fixed creation)
		req := connect.NewRequest(&endpointv1.EndpointCreateRequest{
			CollectionId: collectionID.Bytes(),
			Name:         "New Endpoint",
			Url:          "/api/new",
			Method:       "POST",
		})
		
		resp, err := endpointRPC.EndpointCreate(authedCtx, req)
		require.NoError(t, err)
		
		endpointID, err := idwrap.NewFromBytes(resp.Msg.GetEndpointId())
		require.NoError(t, err)
		
		// Verify it has default example
		defaultExample, err := iaes.GetDefaultApiExample(ctx, endpointID)
		require.NoError(t, err)
		require.NotNil(t, defaultExample)
		
		t.Logf("Created endpoint with default example: %s", endpointID.String())
		
		// Duplicate it
		dupReq := connect.NewRequest(&endpointv1.EndpointDuplicateRequest{
			EndpointId: endpointID.Bytes(),
		})
		
		dupResp, err := endpointRPC.EndpointDuplicate(authedCtx, dupReq)
		require.NoError(t, err)
		require.NotNil(t, dupResp)
		require.NotNil(t, dupResp.Msg)
		require.NotEmpty(t, dupResp.Msg.EndpointId, "Should return new endpoint ID")
		require.NotEmpty(t, dupResp.Msg.ExampleId, "Should return example ID")
		
		newEndpointID, err := idwrap.NewFromBytes(dupResp.Msg.EndpointId)
		require.NoError(t, err)
		exampleID, err := idwrap.NewFromBytes(dupResp.Msg.ExampleId)
		require.NoError(t, err)
		
		t.Logf("✓ Successfully duplicated endpoint with default example")
		t.Logf("  New endpoint ID: %s", newEndpointID.String())
		t.Logf("  Example ID: %s", exampleID.String())
	})
}