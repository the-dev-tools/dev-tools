package rnode

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/service/sbodyform"
	"the-dev-tools/server/pkg/service/sbodyraw"
	"the-dev-tools/server/pkg/service/sbodyurl"
	"the-dev-tools/server/pkg/service/sexampleheader"
	"the-dev-tools/server/pkg/service/sexamplequery"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodeexecution"
	"the-dev-tools/server/pkg/service/snodefor"
	"the-dev-tools/server/pkg/service/snodeforeach"
	"the-dev-tools/server/pkg/service/snodeif"
	"the-dev-tools/server/pkg/service/snodejs"
	"the-dev-tools/server/pkg/service/snodenoop"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/testutil"
	nodev1 "the-dev-tools/spec/dist/buf/go/flow/node/v1"
)

func TestNodeRequestConfigLifecycle(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	t.Cleanup(base.Close)

	queries := base.Queries
	db := base.DB
	_, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	us := suser.New(queries)
	fs := sflow.New(queries)
	ns := snode.New(queries)
	nis := snodeif.New(queries)
	nrs := snoderequest.New(queries)
	nfls := snodefor.New(queries)
	nlfes := snodeforeach.New(queries)
	nss := snodenoop.New(queries)
	njss := snodejs.New(queries)
	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	eqs := sexamplequery.New(queries)
	ehs := sexampleheader.New(queries)
	brs := sbodyraw.New(queries)
	bfs := sbodyform.New(queries)
	bues := sbodyurl.New(queries)
	nes := snodeexecution.New(queries)

	svc := NewNodeServiceRPC(db, us, fs, *nis, nrs, nfls, nlfes, ns, nss, njss, ias, iaes, eqs, ehs, brs, bfs, bues, nes)

	baseServices := base.GetBaseServices()
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)

	flowID := idwrap.NewNow()
	require.NoError(t, fs.CreateFlow(ctx, mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Request Flow",
	}))

	nodeID := idwrap.NewNow()
	require.NoError(t, ns.CreateNode(ctx, mnnode.MNode{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     "Request",
		NodeKind: mnnode.NODE_KIND_REQUEST,
	}))

	require.NoError(t, nrs.CreateNodeRequest(ctx, mnrequest.MNRequest{
		FlowNodeID: nodeID,
	}))

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	assertGet := func(expectedRequestPresent bool, check func(req *nodev1.NodeRequest)) {
		t.Helper()
		resp, err := svc.NodeGet(authedCtx, connect.NewRequest(&nodev1.NodeGetRequest{NodeId: nodeID.Bytes()}))
		require.NoError(t, err)
		if expectedRequestPresent {
			require.NotNil(t, resp.Msg.Request)
			if check != nil {
				check(resp.Msg.Request)
			}
		} else {
			require.Nil(t, resp.Msg.Request)
		}
	}

	// Fresh node: request payload should be absent.
	assertGet(false, nil)

	// Flag node as configured without any surviving resources.
	require.NoError(t, nrs.UpdateNodeRequest(ctx, mnrequest.MNRequest{
		FlowNodeID:       nodeID,
		HasRequestConfig: true,
	}))

	assertGet(true, func(req *nodev1.NodeRequest) {
		require.Empty(t, req.EndpointId)
		require.Empty(t, req.ExampleId)
		require.Empty(t, req.DeltaEndpointId)
		require.Empty(t, req.DeltaExampleId)
	})

	// Configure endpoint via NodeUpdate to ensure flag stays true and IDs propagate.
	endpointID := idwrap.NewNow()
	require.NoError(t, queries.CreateItemApi(ctx, gen.CreateItemApiParams{
		ID:           endpointID,
		CollectionID: collectionID,
		Name:         "Endpoint",
		Url:          "https://example.dev",
		Method:       "GET",
	}))

	_, err = svc.NodeUpdate(authedCtx, connect.NewRequest(&nodev1.NodeUpdateRequest{
		NodeId: nodeID.Bytes(),
		Request: &nodev1.NodeRequest{
			EndpointId: endpointID.Bytes(),
		},
	}))
	require.NoError(t, err)
	storedConfigured, err := nrs.GetNodeRequest(ctx, nodeID)
	require.NoError(t, err)
	require.NotNil(t, storedConfigured.EndpointID)
	require.True(t, storedConfigured.HasRequestConfig)

	assertGet(true, func(req *nodev1.NodeRequest) {
		require.Equal(t, endpointID.Bytes(), req.EndpointId)
	})

	// Remove the underlying resources to emulate dangling configuration.
	require.NoError(t, queries.DeleteItemApi(ctx, endpointID))
	storedAfterDelete, err := nrs.GetNodeRequest(ctx, nodeID)
	require.NoError(t, err)
	require.Nil(t, storedAfterDelete.EndpointID)
	require.True(t, storedAfterDelete.HasRequestConfig)

	assertGet(true, func(req *nodev1.NodeRequest) {
		require.Empty(t, req.EndpointId)
		require.Empty(t, req.DeltaEndpointId)
		require.Empty(t, req.DeltaExampleId)
	})
}
