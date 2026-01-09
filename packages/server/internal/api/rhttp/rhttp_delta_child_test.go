package rhttp

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream/memory"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/http/resolver"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/senv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/testutil"
	apiv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/http/v1"
	globalv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/global/v1"
)

func TestHttpDelta_ChildEntities_SparsePatch(t *testing.T) {
	ctx := context.Background()
	ctx = mwauth.CreateAuthedContext(ctx, mwauth.LocalDummyID)

	baseDBQueries := testutil.CreateBaseDB(ctx, t)
	defer baseDBQueries.Close()

	// Setup services
	logger := baseDBQueries.Logger()
	queries := baseDBQueries.Queries
	db := baseDBQueries.DB

	ws := baseDBQueries.GetBaseServices().WorkspaceService
	wus := baseDBQueries.GetBaseServices().WorkspaceUserService
	us := baseDBQueries.GetBaseServices().UserService
	hs := baseDBQueries.GetBaseServices().HttpService
	es := senv.NewEnvironmentService(queries, logger)
	vs := senv.NewVariableService(queries, logger)

	bodyService := shttp.NewHttpBodyRawService(queries)
	httpHeaderService := shttp.NewHttpHeaderService(queries)
	httpSearchParamService := shttp.NewHttpSearchParamService(queries)
	httpBodyFormService := shttp.NewHttpBodyFormService(queries)
	httpBodyUrlEncodedService := shttp.NewHttpBodyUrlEncodedService(queries)
	httpAssertService := shttp.NewHttpAssertService(queries)
	httpResponseService := shttp.NewHttpResponseService(queries)

	// Streamers
	httpStreamers := &HttpStreamers{
		Http:               memory.NewInMemorySyncStreamer[HttpTopic, HttpEvent](),
		HttpHeader:         memory.NewInMemorySyncStreamer[HttpHeaderTopic, HttpHeaderEvent](),
		HttpSearchParam:    memory.NewInMemorySyncStreamer[HttpSearchParamTopic, HttpSearchParamEvent](),
		HttpBodyForm:       memory.NewInMemorySyncStreamer[HttpBodyFormTopic, HttpBodyFormEvent](),
		HttpBodyUrlEncoded: memory.NewInMemorySyncStreamer[HttpBodyUrlEncodedTopic, HttpBodyUrlEncodedEvent](),
		HttpAssert:         memory.NewInMemorySyncStreamer[HttpAssertTopic, HttpAssertEvent](),
	}

	// Create resolver for delta resolution
	requestResolver := resolver.NewStandardResolver(
		&hs,
		&httpHeaderService,
		httpSearchParamService,
		bodyService,
		httpBodyFormService,
		httpBodyUrlEncodedService,
		httpAssertService,
	)

	httpReader := shttp.NewReader(db, logger, &wus)

	svc := New(HttpServiceRPCDeps{
		DB: db,
		Readers: HttpServiceRPCReaders{
			Http:      httpReader,
			User:      wus.Reader(),
			Workspace: ws.Reader(),
		},
		Services: HttpServiceRPCServices{
			Http:               hs,
			User:               us,
			Workspace:          ws,
			WorkspaceUser:      wus,
			Env:                es,
			Variable:           vs,
			HttpBodyRaw:        bodyService,
			HttpHeader:         httpHeaderService,
			HttpSearchParam:    httpSearchParamService,
			HttpBodyForm:       httpBodyFormService,
			HttpBodyUrlEncoded: httpBodyUrlEncodedService,
			HttpAssert:         httpAssertService,
			HttpResponse:       httpResponseService,
		},
		Resolver:  requestResolver,
		Streamers: httpStreamers,
	})

	// 1. Create Workspace and User
	workspaceID := idwrap.NewNow()
	err := svc.ws.Create(ctx, &mworkspace.Workspace{ID: workspaceID, Name: "Test Workspace"})
	require.NoError(t, err)

	err = svc.wus.CreateWorkspaceUser(ctx, &mworkspace.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		UserID:      mwauth.LocalDummyID,
		Role:        mworkspace.RoleOwner,
	})
	require.NoError(t, err)

	// 2. Create Base HTTP Request
	baseHttpID := idwrap.NewNow()
	err = svc.hs.Create(ctx, &mhttp.HTTP{
		ID:          baseHttpID,
		WorkspaceID: workspaceID,
		Name:        "Base",
		Method:      "GET",
		Url:         "http://example.com",
	})
	require.NoError(t, err)

	// 3. Create Delta HTTP Request
	deltaHttpID := idwrap.NewNow()
	err = svc.hs.Create(ctx, &mhttp.HTTP{
		ID:           deltaHttpID,
		WorkspaceID:  workspaceID,
		Name:         "Base",
		ParentHttpID: &baseHttpID,
		IsDelta:      true,
	})
	require.NoError(t, err)

	t.Run("HttpHeaderDelta", func(t *testing.T) {
		// Create Base Header
		baseHeaderID := idwrap.NewNow()
		err := svc.httpHeaderService.Create(ctx, &mhttp.HTTPHeader{
			ID:      baseHeaderID,
			HttpID:  baseHttpID,
			Key:     "Base-Key",
			Value:   "Base-Value",
			Enabled: true,
		})
		require.NoError(t, err)

		// Create Delta Header (Override)
		deltaHeaderID := idwrap.NewNow()
		deltaVal := "Delta-Value"
		err = svc.httpHeaderService.Create(ctx, &mhttp.HTTPHeader{
			ID:                 deltaHeaderID,
			HttpID:             deltaHttpID,
			Key:                "Base-Key",
			Value:              "Base-Value",
			ParentHttpHeaderID: &baseHeaderID,
			IsDelta:            true,
			DeltaValue:         &deltaVal,
		})
		require.NoError(t, err)

		// Stream
		stream := make(chan *apiv1.HttpHeaderDeltaSyncResponse, 10)
		sCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		go svc.streamHttpHeaderDeltaSync(sCtx, mwauth.LocalDummyID, func(resp *apiv1.HttpHeaderDeltaSyncResponse) error {
			stream <- resp
			return nil
		})
		time.Sleep(50 * time.Millisecond)

		// Update ONLY Description (Sparse)
		newDesc := "Updated Desc"
		_, err = svc.HttpHeaderDeltaUpdate(ctx, connect.NewRequest(&apiv1.HttpHeaderDeltaUpdateRequest{
			Items: []*apiv1.HttpHeaderDeltaUpdate{{
				DeltaHttpHeaderId: deltaHeaderID.Bytes(),
				Description: &apiv1.HttpHeaderDeltaUpdate_DescriptionUnion{
					Kind:  apiv1.HttpHeaderDeltaUpdate_DescriptionUnion_KIND_VALUE,
					Value: &newDesc,
				},
			}},
		}))
		require.NoError(t, err)

		// Verify Event
		select {
		case resp := <-stream:
			update := resp.Items[0].GetValue().GetUpdate()
			require.Equal(t, deltaHeaderID.Bytes(), update.DeltaHttpHeaderId)
			require.NotNil(t, update.Description)
			require.Equal(t, newDesc, update.Description.GetValue())
			// Verify other fields are missing (Undefined)
			require.Nil(t, update.Value, "Value should be omitted")
			require.Nil(t, update.Key, "Key should be omitted")
		case <-time.After(time.Second):
			t.Fatal("Timeout")
		}

		// Verify Persistence
		h, err := svc.httpHeaderService.GetByID(ctx, deltaHeaderID)
		require.NoError(t, err)
		require.Equal(t, newDesc, *h.DeltaDescription)
		require.Equal(t, deltaVal, *h.DeltaValue, "Value should persist")
	})

	t.Run("HttpSearchParamDelta", func(t *testing.T) {
		// Create Base Param
		baseParamID := idwrap.NewNow()
		err := svc.httpSearchParamService.Create(ctx, &mhttp.HTTPSearchParam{
			ID:      baseParamID,
			HttpID:  baseHttpID,
			Key:     "q",
			Value:   "base",
			Enabled: true,
		})
		require.NoError(t, err)

		// Create Delta Param
		deltaParamID := idwrap.NewNow()
		deltaVal := "delta"
		err = svc.httpSearchParamService.Create(ctx, &mhttp.HTTPSearchParam{
			ID:                      deltaParamID,
			HttpID:                  deltaHttpID,
			Key:                     "q",
			Value:                   "base",
			ParentHttpSearchParamID: &baseParamID,
			IsDelta:                 true,
			DeltaValue:              &deltaVal,
		})
		require.NoError(t, err)

		// Stream
		stream := make(chan *apiv1.HttpSearchParamDeltaSyncResponse, 10)
		sCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		go svc.streamHttpSearchParamDeltaSync(sCtx, mwauth.LocalDummyID, func(resp *apiv1.HttpSearchParamDeltaSyncResponse) error {
			stream <- resp
			return nil
		})
		time.Sleep(50 * time.Millisecond)

		// Update ONLY Enabled (Sparse)
		enabled := false
		_, err = svc.HttpSearchParamDeltaUpdate(ctx, connect.NewRequest(&apiv1.HttpSearchParamDeltaUpdateRequest{
			Items: []*apiv1.HttpSearchParamDeltaUpdate{{
				DeltaHttpSearchParamId: deltaParamID.Bytes(),
				Enabled: &apiv1.HttpSearchParamDeltaUpdate_EnabledUnion{
					Kind:  apiv1.HttpSearchParamDeltaUpdate_EnabledUnion_KIND_VALUE,
					Value: &enabled,
				},
			}},
		}))
		require.NoError(t, err)

		// Verify Event
		select {
		case resp := <-stream:
			update := resp.Items[0].GetValue().GetUpdate()
			require.Equal(t, deltaParamID.Bytes(), update.DeltaHttpSearchParamId)
			require.NotNil(t, update.Enabled)
			require.Equal(t, enabled, update.Enabled.GetValue())
			// Verify other fields are missing
			require.Nil(t, update.Value, "Value should be omitted")
		case <-time.After(time.Second):
			t.Fatal("Timeout")
		}

		// Verify Persistence
		p, err := svc.httpSearchParamService.GetByID(ctx, deltaParamID)
		require.NoError(t, err)
		require.Equal(t, enabled, *p.DeltaEnabled)
		require.Equal(t, deltaVal, *p.DeltaValue)
	})

	t.Run("HttpBodyFormDelta", func(t *testing.T) {
		// Setup similar to above
		baseFormID := idwrap.NewNow()
		err := svc.httpBodyFormService.Create(ctx, &mhttp.HTTPBodyForm{
			ID:      baseFormID,
			HttpID:  baseHttpID,
			Key:     "f",
			Value:   "1",
			Enabled: true,
		})
		require.NoError(t, err)

		deltaFormID := idwrap.NewNow()
		deltaKey := "f_delta"
		err = svc.httpBodyFormService.Create(ctx, &mhttp.HTTPBodyForm{
			ID:                   deltaFormID,
			HttpID:               deltaHttpID,
			Key:                  "f",
			Value:                "1",
			ParentHttpBodyFormID: &baseFormID,
			IsDelta:              true,
			DeltaKey:             &deltaKey,
		})
		require.NoError(t, err)

		stream := make(chan *apiv1.HttpBodyFormDataDeltaSyncResponse, 10)
		sCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		go svc.streamHttpBodyFormDeltaSync(sCtx, mwauth.LocalDummyID, func(resp *apiv1.HttpBodyFormDataDeltaSyncResponse) error {
			stream <- resp
			return nil
		})
		time.Sleep(50 * time.Millisecond)

		// Update Value to UNSET (Sparse)
		_, err = svc.HttpBodyFormDataDeltaUpdate(ctx, connect.NewRequest(&apiv1.HttpBodyFormDataDeltaUpdateRequest{
			Items: []*apiv1.HttpBodyFormDataDeltaUpdate{{
				DeltaHttpBodyFormDataId: deltaFormID.Bytes(),
				Value: &apiv1.HttpBodyFormDataDeltaUpdate_ValueUnion{
					Kind:  apiv1.HttpBodyFormDataDeltaUpdate_ValueUnion_KIND_UNSET,
					Unset: globalv1.Unset_UNSET.Enum(),
				},
			}},
		}))
		require.NoError(t, err)

		select {
		case resp := <-stream:
			update := resp.Items[0].GetValue().GetUpdate()
			require.NotNil(t, update.Value)
			require.Equal(t, apiv1.HttpBodyFormDataDeltaSyncUpdate_ValueUnion_KIND_UNSET, update.Value.Kind)
			require.Nil(t, update.Key, "Key should be omitted")
		case <-time.After(time.Second):
			t.Fatal("Timeout")
		}

		f, err := svc.httpBodyFormService.GetByID(ctx, deltaFormID)
		require.NoError(t, err)
		require.Nil(t, f.DeltaValue)
		require.Equal(t, deltaKey, *f.DeltaKey)
	})

	t.Run("HttpAssertDelta", func(t *testing.T) {
		baseAssertID := idwrap.NewNow()
		err := svc.httpAssertService.Create(ctx, &mhttp.HTTPAssert{
			ID:      baseAssertID,
			HttpID:  baseHttpID,
			Value:   "res.status == 200",
			Enabled: true,
		})
		require.NoError(t, err)

		deltaAssertID := idwrap.NewNow()
		deltaVal := "res.status == 201"
		err = svc.httpAssertService.Create(ctx, &mhttp.HTTPAssert{
			ID:                 deltaAssertID,
			HttpID:             deltaHttpID,
			Value:              "res.status == 200",
			ParentHttpAssertID: &baseAssertID,
			IsDelta:            true,
			DeltaValue:         &deltaVal,
		})
		require.NoError(t, err)

		stream := make(chan *apiv1.HttpAssertDeltaSyncResponse, 10)
		sCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		go svc.streamHttpAssertDeltaSync(sCtx, mwauth.LocalDummyID, func(resp *apiv1.HttpAssertDeltaSyncResponse) error {
			stream <- resp
			return nil
		})
		time.Sleep(50 * time.Millisecond)

		// Update Enabled (Sparse)
		newEnabled := false
		_, err = svc.HttpAssertDeltaUpdate(ctx, connect.NewRequest(&apiv1.HttpAssertDeltaUpdateRequest{
			Items: []*apiv1.HttpAssertDeltaUpdate{{
				DeltaHttpAssertId: deltaAssertID.Bytes(),
				Enabled: &apiv1.HttpAssertDeltaUpdate_EnabledUnion{
					Kind:  apiv1.HttpAssertDeltaUpdate_EnabledUnion_KIND_VALUE,
					Value: &newEnabled,
				},
			}},
		}))
		require.NoError(t, err)

		select {
		case resp := <-stream:
			update := resp.Items[0].GetValue().GetUpdate()
			require.NotNil(t, update.Enabled)
			require.Equal(t, newEnabled, update.Enabled.GetValue())
			require.Nil(t, update.Value, "Value should be omitted")
		case <-time.After(time.Second):
			t.Fatal("Timeout")
		}

		a, err := svc.httpAssertService.GetByID(ctx, deltaAssertID)
		require.NoError(t, err)
		require.Equal(t, newEnabled, *a.DeltaEnabled)
		require.Equal(t, deltaVal, *a.DeltaValue)
	})
}
