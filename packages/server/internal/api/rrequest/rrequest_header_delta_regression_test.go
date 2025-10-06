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

func TestHeaderDeltaUpdateMultiFlowRegression(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries

	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	mockLogger := mocklogger.NewMockLogger()
	cs := scollection.New(queries, mockLogger)
	us := suser.New(queries)
	ehs := sexampleheader.New(queries)
	eqs := sexamplequery.New(queries)
	as := sassert.New(queries)

	rpc := rrequest.New(base.DB, cs, us, ias, iaes, ehs, eqs, as)

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	userID := idwrap.NewNow()

	base.GetBaseServices().CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)

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
		Name:         "Origin Example",
		IsDefault:    true,
	}
	require.NoError(t, iaes.CreateApiExample(authCtx, originExample))

	originHeaderID := idwrap.NewNow()
	require.NoError(t, ehs.AppendHeader(authCtx, mexampleheader.Header{
		ID:          originHeaderID,
		ExampleID:   originExampleID,
		HeaderKey:   "Authorization",
		Value:       "Bearer origin",
		Enable:      true,
		Description: "seed header",
	}))

	makeDeltaExample := func(name string) idwrap.IDWrap {
		deltaID := idwrap.NewNow()
		delta := &mitemapiexample.ItemApiExample{
			ID:           deltaID,
			ItemApiID:    originEndpointID,
			CollectionID: collectionID,
			Name:         name,
			IsDefault:    false,
			VersionParentID: &originExampleID,
		}
		require.NoError(t, iaes.CreateApiExample(authCtx, delta))
		return deltaID
	}

	listValue := func(deltaID idwrap.IDWrap) string {
		resp, err := rpc.HeaderDeltaList(authCtx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: deltaID.Bytes(),
			OriginId:  originExampleID.Bytes(),
		}))
		require.NoError(t, err)
		require.Len(t, resp.Msg.Items, 1, "expected single header in list")
		return resp.Msg.Items[0].GetValue()
	}

	updateValue := func(deltaID idwrap.IDWrap, value string) {
		resp, err := rpc.HeaderDeltaList(authCtx, connect.NewRequest(&requestv1.HeaderDeltaListRequest{
			ExampleId: deltaID.Bytes(),
			OriginId:  originExampleID.Bytes(),
		}))
		require.NoError(t, err)
		require.Len(t, resp.Msg.Items, 1)
		item := resp.Msg.Items[0]
		require.Equal(t, originHeaderID.Bytes(), item.HeaderId)

		val := value
		_, err = rpc.HeaderDeltaUpdate(authCtx, connect.NewRequest(&requestv1.HeaderDeltaUpdateRequest{
			ExampleId: deltaID.Bytes(),
			HeaderId:  item.HeaderId,
			Value:     &val,
		}))
		require.NoError(t, err)
	}

	deltaOne := makeDeltaExample("delta-one")
	require.Equal(t, "Bearer origin", listValue(deltaOne))

	updateValue(deltaOne, "Bearer one")
	require.Equal(t, "Bearer one", listValue(deltaOne))

	deltaTwo := makeDeltaExample("delta-two")
	require.Equal(t, "Bearer origin", listValue(deltaTwo))

	updateValue(deltaTwo, "Bearer two")
	require.Equal(t, "Bearer two", listValue(deltaTwo))

	// Ensure the first delta retains its override
	require.Equal(t, "Bearer one", listValue(deltaOne))
}
