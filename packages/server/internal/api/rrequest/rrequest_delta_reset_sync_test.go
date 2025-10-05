package rrequest_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rrequest"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
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
	deltav1 "the-dev-tools/spec/dist/buf/go/delta/v1"
)

type requestDeltaFixture struct {
	ctx             context.Context
	authCtx         context.Context
	rpc             rrequest.RequestRPC
	ehs             sexampleheader.HeaderService
	eqs             sexamplequery.ExampleQueryService
	originExampleID idwrap.IDWrap
	deltaExampleID  idwrap.IDWrap
}

func newRequestDeltaFixture(t *testing.T) requestDeltaFixture {
	t.Helper()

	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries

	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	cs := scollection.New(queries, mocklogger.NewMockLogger())
	us := suser.New(queries)
	ehs := sexampleheader.New(queries)
	eqs := sexamplequery.New(queries)
	as := sassert.New(queries)

	rpc := rrequest.New(base.DB, cs, us, ias, iaes, ehs, eqs, as)

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	userID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)

	authCtx := mwauth.CreateAuthedContext(ctx, userID)

	originEndpointID := idwrap.NewNow()
	originEndpoint := &mitemapi.ItemApi{
		ID:           originEndpointID,
		CollectionID: collectionID,
		Name:         "origin-endpoint",
		Method:       "GET",
		Url:          "/origin",
		Hidden:       false,
	}
	require.NoError(t, ias.CreateItemApi(authCtx, originEndpoint))

	originExampleID := idwrap.NewNow()
	originExample := &mitemapiexample.ItemApiExample{
		ID:           originExampleID,
		ItemApiID:    originEndpointID,
		CollectionID: collectionID,
		Name:         "origin-example",
	}
	require.NoError(t, iaes.CreateApiExample(authCtx, originExample))

	deltaEndpointID := idwrap.NewNow()
	deltaEndpoint := &mitemapi.ItemApi{
		ID:           deltaEndpointID,
		CollectionID: collectionID,
		Name:         "delta-endpoint",
		Method:       "GET",
		Url:          "/delta",
		Hidden:       true,
	}
	require.NoError(t, ias.CreateItemApi(authCtx, deltaEndpoint))

	deltaExampleID := idwrap.NewNow()
	deltaExample := &mitemapiexample.ItemApiExample{
		ID:              deltaExampleID,
		ItemApiID:       deltaEndpointID,
		CollectionID:    collectionID,
		Name:            "delta-example",
		VersionParentID: &originExampleID,
	}
	require.NoError(t, iaes.CreateApiExample(authCtx, deltaExample))

	return requestDeltaFixture{
		ctx:             ctx,
		authCtx:         authCtx,
		rpc:             rpc,
		ehs:             ehs,
		eqs:             eqs,
		originExampleID: originExampleID,
		deltaExampleID:  deltaExampleID,
	}
}

func TestHeaderDeltaResetRestoresDeltaExampleRow(t *testing.T) {
	t.Parallel()

	fx := newRequestDeltaFixture(t)

	originHeader := mexampleheader.Header{
		ID:        idwrap.NewNow(),
		ExampleID: fx.originExampleID,
		HeaderKey: "authorization",
		Value:     "origin-token",
		Enable:    true,
	}
	require.NoError(t, fx.ehs.AppendHeader(fx.ctx, originHeader))

	require.NoError(t, fx.rpc.HeaderDeltaExampleCopy(fx.authCtx, fx.originExampleID, fx.deltaExampleID))

	deltaHeaders, err := fx.ehs.GetHeaderByExampleID(fx.ctx, fx.deltaExampleID)
	require.NoError(t, err)
	require.Len(t, deltaHeaders, 1)
	deltaHeader := deltaHeaders[0]
	require.NotNil(t, deltaHeader.DeltaParentID)
	require.Equal(t, 0, deltaHeader.DeltaParentID.Compare(originHeader.ID))

	deltaHeader.Value = "custom-token"
	deltaHeader.Description = "custom"
	require.NoError(t, fx.ehs.UpdateHeader(fx.ctx, deltaHeader))

	listResp, err := fx.rpc.HeaderDeltaList(fx.authCtx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
		ExampleId: fx.deltaExampleID.Bytes(),
		OriginId:  fx.originExampleID.Bytes(),
	}))
	require.NoError(t, err)
	require.Len(t, listResp.Msg.Items, 1)
	item := listResp.Msg.Items[0]
	require.Equal(t, "custom-token", item.GetValue())
	require.Equal(t, deltav1.SourceKind_SOURCE_KIND_MIXED, item.GetSource())

	_, err = fx.rpc.HeaderDeltaReset(fx.authCtx, connect.NewRequest(&requestv1.HeaderDeltaResetRequest{HeaderId: item.GetHeaderId()}))
	require.NoError(t, err)

	resetResp, err := fx.rpc.HeaderDeltaList(fx.authCtx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
		ExampleId: fx.deltaExampleID.Bytes(),
		OriginId:  fx.originExampleID.Bytes(),
	}))
	require.NoError(t, err)
	require.Len(t, resetResp.Msg.Items, 1)
	resetItem := resetResp.Msg.Items[0]
	require.Equal(t, "origin-token", resetItem.GetValue())
	require.Equal(t, deltav1.SourceKind_SOURCE_KIND_ORIGIN, resetItem.GetSource())

	afterHeaders, err := fx.ehs.GetHeaderByExampleID(fx.ctx, fx.deltaExampleID)
	require.NoError(t, err)
	require.Len(t, afterHeaders, 1)
	require.Equal(t, "origin-token", afterHeaders[0].Value)
	require.Equal(t, "", afterHeaders[0].Description)
}

func TestQueryDeltaResetRestoresDeltaExampleRow(t *testing.T) {
	t.Parallel()

	fx := newRequestDeltaFixture(t)

	originQuery := mexamplequery.Query{
		ID:          idwrap.NewNow(),
		ExampleID:   fx.originExampleID,
		QueryKey:    "token",
		Enable:      true,
		Description: "origin-description",
		Value:       "origin-query",
	}
	require.NoError(t, fx.eqs.CreateExampleQuery(fx.ctx, originQuery))

	require.NoError(t, fx.rpc.QueryDeltaExampleCopy(fx.authCtx, fx.originExampleID, fx.deltaExampleID))

	deltaQueries, err := fx.eqs.GetExampleQueriesByExampleID(fx.ctx, fx.deltaExampleID)
	require.NoError(t, err)
	require.Len(t, deltaQueries, 1)
	deltaQuery := deltaQueries[0]
	require.NotNil(t, deltaQuery.DeltaParentID)
	require.Equal(t, 0, deltaQuery.DeltaParentID.Compare(originQuery.ID))

	deltaQuery.Value = "custom-query"
	deltaQuery.Description = "custom description"
	require.NoError(t, fx.eqs.UpdateExampleQuery(fx.ctx, deltaQuery))

	listResp, err := fx.rpc.QueryDeltaList(fx.authCtx, connect.NewRequest(&requestv1.QueryDeltaListRequest{
		ExampleId: fx.deltaExampleID.Bytes(),
		OriginId:  fx.originExampleID.Bytes(),
	}))
	require.NoError(t, err)
	require.Len(t, listResp.Msg.Items, 1)
	item := listResp.Msg.Items[0]
	require.Equal(t, "custom-query", item.GetValue())
	require.Equal(t, deltav1.SourceKind_SOURCE_KIND_MIXED, item.GetSource())

	_, err = fx.rpc.QueryDeltaReset(fx.authCtx, connect.NewRequest(&requestv1.QueryDeltaResetRequest{QueryId: item.GetQueryId()}))
	require.NoError(t, err)

	resetResp, err := fx.rpc.QueryDeltaList(fx.authCtx, connect.NewRequest(&requestv1.QueryDeltaListRequest{
		ExampleId: fx.deltaExampleID.Bytes(),
		OriginId:  fx.originExampleID.Bytes(),
	}))
	require.NoError(t, err)
	require.Len(t, resetResp.Msg.Items, 1)
	resetItem := resetResp.Msg.Items[0]
	require.Equal(t, "origin-query", resetItem.GetValue())
	require.Equal(t, "origin-description", resetItem.GetDescription())
	require.Equal(t, deltav1.SourceKind_SOURCE_KIND_ORIGIN, resetItem.GetSource())

	afterQueries, err := fx.eqs.GetExampleQueriesByExampleID(fx.ctx, fx.deltaExampleID)
	require.NoError(t, err)
	require.Len(t, afterQueries, 1)
	require.Equal(t, "origin-query", afterQueries[0].Value)
	require.Equal(t, "origin-description", afterQueries[0].Description)
}
