package rbody_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rbody"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/model/mbodyform"
	"the-dev-tools/server/pkg/model/mbodyurl"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/service/sbodyform"
	"the-dev-tools/server/pkg/service/sbodyraw"
	"the-dev-tools/server/pkg/service/sbodyurl"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/testutil"
	bodyv1 "the-dev-tools/spec/dist/buf/go/collection/item/body/v1"
	deltav1 "the-dev-tools/spec/dist/buf/go/delta/v1"
)

type bodyDeltaFixture struct {
	ctx             context.Context
	authCtx         context.Context
	rpc             rbody.BodyRPC
	bfs             sbodyform.BodyFormService
	bues            sbodyurl.BodyURLEncodedService
	originExampleID idwrap.IDWrap
	deltaExampleID  idwrap.IDWrap
}

func newBodyDeltaFixture(t *testing.T) bodyDeltaFixture {
	t.Helper()

	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries

	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	cs := scollection.New(queries, mocklogger.NewMockLogger())
	us := suser.New(queries)
	bfs := sbodyform.New(queries)
	bues := sbodyurl.New(queries)
	brs := sbodyraw.New(queries)

	rpc := rbody.New(base.DB, cs, iaes, us, bfs, bues, brs)

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
		Name:         "origin-body-endpoint",
		Method:       "POST",
		Url:          "/origin-body",
		Hidden:       false,
	}
	require.NoError(t, ias.CreateItemApi(authCtx, originEndpoint))

	originExampleID := idwrap.NewNow()
	originExample := &mitemapiexample.ItemApiExample{
		ID:           originExampleID,
		ItemApiID:    originEndpointID,
		CollectionID: collectionID,
		Name:         "origin-body-example",
	}
	require.NoError(t, iaes.CreateApiExample(authCtx, originExample))

	deltaEndpointID := idwrap.NewNow()
	deltaEndpoint := &mitemapi.ItemApi{
		ID:           deltaEndpointID,
		CollectionID: collectionID,
		Name:         "delta-body-endpoint",
		Method:       "POST",
		Url:          "/delta-body",
		Hidden:       true,
	}
	require.NoError(t, ias.CreateItemApi(authCtx, deltaEndpoint))

	deltaExampleID := idwrap.NewNow()
	deltaExample := &mitemapiexample.ItemApiExample{
		ID:              deltaExampleID,
		ItemApiID:       deltaEndpointID,
		CollectionID:    collectionID,
		Name:            "delta-body-example",
		VersionParentID: &originExampleID,
	}
	require.NoError(t, iaes.CreateApiExample(authCtx, deltaExample))

	return bodyDeltaFixture{
		ctx:             ctx,
		authCtx:         authCtx,
		rpc:             rpc,
		bfs:             bfs,
		bues:            bues,
		originExampleID: originExampleID,
		deltaExampleID:  deltaExampleID,
	}
}

func TestBodyFormDeltaResetRestoresDeltaExampleRow(t *testing.T) {
	t.Parallel()

	fx := newBodyDeltaFixture(t)

	originForm := &mbodyform.BodyForm{
		ID:          idwrap.NewNow(),
		ExampleID:   fx.originExampleID,
		BodyKey:     "token",
		Value:       "origin-value",
		Enable:      true,
		Description: "origin-desc",
	}
	require.NoError(t, fx.bfs.CreateBodyForm(fx.ctx, originForm))

	deltaForm := &mbodyform.BodyForm{
		ID:            idwrap.NewNow(),
		ExampleID:     fx.deltaExampleID,
		DeltaParentID: &originForm.ID,
		BodyKey:       originForm.BodyKey,
		Value:         originForm.Value,
		Enable:        originForm.Enable,
		Description:   originForm.Description,
	}
	require.NoError(t, fx.bfs.CreateBodyForm(fx.ctx, deltaForm))

	// seed overlay once so order/state tables exist
	_, err := fx.rpc.BodyFormDeltaList(fx.authCtx, connect.NewRequest(&bodyv1.BodyFormDeltaListRequest{
		ExampleId: fx.deltaExampleID.Bytes(),
		OriginId:  fx.originExampleID.Bytes(),
	}))
	require.NoError(t, err)

	deltaForm.Value = "custom-value"
	deltaForm.Description = "custom-desc"
	require.NoError(t, fx.bfs.UpdateBodyForm(fx.ctx, deltaForm))

	listResp, err := fx.rpc.BodyFormDeltaList(fx.authCtx, connect.NewRequest(&bodyv1.BodyFormDeltaListRequest{
		ExampleId: fx.deltaExampleID.Bytes(),
		OriginId:  fx.originExampleID.Bytes(),
	}))
	require.NoError(t, err)
	require.Len(t, listResp.Msg.Items, 1)
	formItem := listResp.Msg.Items[0]
	require.Equal(t, "custom-value", formItem.GetValue())
	require.Equal(t, deltav1.SourceKind_SOURCE_KIND_MIXED, formItem.GetSource())

	_, err = fx.rpc.BodyFormDeltaReset(fx.authCtx, connect.NewRequest(&bodyv1.BodyFormDeltaResetRequest{BodyId: formItem.GetBodyId()}))
	require.NoError(t, err)

	resetResp, err := fx.rpc.BodyFormDeltaList(fx.authCtx, connect.NewRequest(&bodyv1.BodyFormDeltaListRequest{
		ExampleId: fx.deltaExampleID.Bytes(),
		OriginId:  fx.originExampleID.Bytes(),
	}))
	require.NoError(t, err)
	require.Len(t, resetResp.Msg.Items, 1)
	resetItem := resetResp.Msg.Items[0]
	require.Equal(t, "origin-value", resetItem.GetValue())
	require.Equal(t, "origin-desc", resetItem.GetDescription())
	require.Equal(t, deltav1.SourceKind_SOURCE_KIND_ORIGIN, resetItem.GetSource())

	formsAfter, err := fx.bfs.GetBodyFormsByExampleID(fx.ctx, fx.deltaExampleID)
	require.NoError(t, err)
	require.Len(t, formsAfter, 1)
	require.Equal(t, "origin-value", formsAfter[0].Value)
	require.Equal(t, "origin-desc", formsAfter[0].Description)
}

func TestBodyUrlEncodedDeltaResetRestoresDeltaExampleRow(t *testing.T) {
	t.Parallel()

	fx := newBodyDeltaFixture(t)

	originUrl := &mbodyurl.BodyURLEncoded{
		ID:        idwrap.NewNow(),
		ExampleID: fx.originExampleID,
		BodyKey:   "token",
		Value:     "origin-url-value",
		Enable:    true,
	}
	require.NoError(t, fx.bues.CreateBodyURLEncoded(fx.ctx, originUrl))
	require.NoError(t, fx.bues.AppendAtEnd(fx.ctx, fx.originExampleID, originUrl.ID))

	deltaUrl := &mbodyurl.BodyURLEncoded{
		ID:            idwrap.NewNow(),
		ExampleID:     fx.deltaExampleID,
		DeltaParentID: &originUrl.ID,
		BodyKey:       originUrl.BodyKey,
		Value:         originUrl.Value,
		Enable:        originUrl.Enable,
	}
	require.NoError(t, fx.bues.CreateBodyURLEncoded(fx.ctx, deltaUrl))
	require.NoError(t, fx.bues.AppendAtEnd(fx.ctx, fx.deltaExampleID, deltaUrl.ID))

	// prime overlay order/state
	_, err := fx.rpc.BodyUrlEncodedDeltaList(fx.authCtx, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaListRequest{
		ExampleId: fx.deltaExampleID.Bytes(),
		OriginId:  fx.originExampleID.Bytes(),
	}))
	require.NoError(t, err)

	deltaUrl.Value = "custom-url-value"
	deltaUrl.Description = "custom"
	require.NoError(t, fx.bues.UpdateBodyURLEncoded(fx.ctx, deltaUrl))

	urlsBeforeReset, err := fx.bues.GetBodyURLEncodedByExampleID(fx.ctx, fx.deltaExampleID)
	require.NoError(t, err)
	require.Len(t, urlsBeforeReset, 1)
	require.Equal(t, "custom-url-value", urlsBeforeReset[0].Value)

	// Execute reset and ensure the backing row is restored
	_, err = fx.rpc.BodyUrlEncodedDeltaReset(fx.authCtx, connect.NewRequest(&bodyv1.BodyUrlEncodedDeltaResetRequest{BodyId: originUrl.ID.Bytes()}))
	require.NoError(t, err)

	urlsAfter, err := fx.bues.GetBodyURLEncodedByExampleID(fx.ctx, fx.deltaExampleID)
	require.NoError(t, err)
	require.Len(t, urlsAfter, 1)
	require.Equal(t, "origin-url-value", urlsAfter[0].Value)
	require.Equal(t, "", urlsAfter[0].Description)
}
