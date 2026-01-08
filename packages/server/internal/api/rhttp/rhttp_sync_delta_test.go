package rhttp

import (
	"context"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/senv"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/converter"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream/memory"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/http/resolver"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/testutil"
	apiv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/http/v1"
)

// TestHttpSync_DeltaIsolation verifies that "Base" streams and "Delta" streams are correctly isolated.
// Delta entities should NOT appear in the Base stream (HttpHeaderSync).
// Base entities should NOT appear in the Delta stream (HttpHeaderDeltaSync) - strictly speaking, Delta stream handles delta records.
func TestHttpSync_DeltaIsolation(t *testing.T) {
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
		HttpVersion:        memory.NewInMemorySyncStreamer[HttpVersionTopic, HttpVersionEvent](),
		HttpResponse:       memory.NewInMemorySyncStreamer[HttpResponseTopic, HttpResponseEvent](),
		HttpResponseHeader: memory.NewInMemorySyncStreamer[HttpResponseHeaderTopic, HttpResponseHeaderEvent](),
		HttpResponseAssert: memory.NewInMemorySyncStreamer[HttpResponseAssertTopic, HttpResponseAssertEvent](),
		HttpBodyRaw:        memory.NewInMemorySyncStreamer[HttpBodyRawTopic, HttpBodyRawEvent](),
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
	err := svc.ws.Create(ctx, &mworkspace.Workspace{
		ID:   workspaceID,
		Name: "Test Workspace",
	})
	require.NoError(t, err)

	// Link user to workspace
	err = svc.wus.CreateWorkspaceUser(ctx, &mworkspace.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		UserID:      mwauth.LocalDummyID,
		Role:        mworkspace.RoleOwner,
	})
	require.NoError(t, err)

	// 2. Create Base HTTP Request
	baseHttpID := idwrap.NewNow()
	baseHttp := mhttp.HTTP{
		ID:          baseHttpID,
		WorkspaceID: workspaceID,
		Name:        "Base Request",
		Method:      "GET",
		Url:         "https://example.com",
		BodyKind:    mhttp.HttpBodyKindNone,
		IsDelta:     false,
	}
	err = svc.hs.Create(ctx, &baseHttp)
	require.NoError(t, err)

	// 3. Create Delta HTTP Request (linked to Base)
	deltaHttpID := idwrap.NewNow()
	deltaHttp := mhttp.HTTP{
		ID:           deltaHttpID,
		WorkspaceID:  workspaceID,
		Name:         "Base Request", // Inherited name
		Method:       "GET",
		Url:          "https://example.com",
		BodyKind:     mhttp.HttpBodyKindNone,
		ParentHttpID: &baseHttpID,
		IsDelta:      true,
		DeltaName:    convertStringPtr("Delta Override"),
	}
	err = svc.hs.Create(ctx, &deltaHttp)
	require.NoError(t, err)

	// 4. Start Base Header Stream
	baseStream := make(chan *apiv1.HttpHeaderSyncResponse, 10)
	baseCtx, baseCancel := context.WithCancel(ctx)
	defer baseCancel()

	go func() {
		err := svc.streamHttpHeaderSync(baseCtx, mwauth.LocalDummyID, func(resp *apiv1.HttpHeaderSyncResponse) error {
			baseStream <- resp
			return nil
		})
		if err != nil && err != context.Canceled {
			require.FailNow(t, "Base stream error: %v", err)
		}
	}()

	// 5. Start Delta Header Stream
	deltaStream := make(chan *apiv1.HttpHeaderDeltaSyncResponse, 10)
	deltaCtx, deltaCancel := context.WithCancel(ctx)
	defer deltaCancel()

	go func() {
		err := svc.streamHttpHeaderDeltaSync(deltaCtx, mwauth.LocalDummyID, func(resp *apiv1.HttpHeaderDeltaSyncResponse) error {
			deltaStream <- resp
			return nil
		})
		if err != nil && err != context.Canceled {
			require.FailNow(t, "Delta stream error: %v", err)
		}
	}()

	// Wait for streams to initialize
	time.Sleep(100 * time.Millisecond)

	// 6. Insert Base Header -> Should appear in Base Stream ONLY
	baseHeaderID := idwrap.NewNow()
	reqBase := &connect.Request[apiv1.HttpHeaderInsertRequest]{
		Msg: &apiv1.HttpHeaderInsertRequest{
			Items: []*apiv1.HttpHeaderInsert{
				{
					HttpHeaderId: baseHeaderID.Bytes(),
					HttpId:       baseHttpID.Bytes(),
					Key:          "Content-Type",
					Value:        "application/json",
					Enabled:      true,
				},
			},
		},
	}
	_, err = svc.HttpHeaderInsert(ctx, reqBase)
	require.NoError(t, err)

	// Verify Base Stream received it
	select {
	case resp := <-baseStream:
		// Access items carefully using generated accessors if possible, or direct field access
		items := resp.GetItems()
		require.NotEmpty(t, items)
		insert := items[0].GetValue().GetInsert()
		require.NotNil(t, insert)
		require.Equal(t, baseHeaderID.Bytes(), insert.HttpHeaderId)
		require.Equal(t, "Content-Type", insert.Key)
	case <-time.After(1 * time.Second):
		require.FailNow(t, "Timeout waiting for Base Header insert event")
	}

	// Verify Delta Stream did NOT receive it
	select {
	case <-deltaStream:
		require.FailNow(t, "Delta stream received Base Header insert event")
	case <-time.After(200 * time.Millisecond):
		// Pass
	}

	// 7. Insert Delta Header -> Should appear in Delta Stream ONLY
	deltaHeaderID := idwrap.NewNow()

	// Manually create the Delta Header using service because the RPC seems suspect
	deltaHeader := &mhttp.HTTPHeader{
		ID:                 deltaHeaderID,
		HttpID:             deltaHttpID,
		Key:                "Content-Type",
		Value:              "application/json", // Original value (ignored if overridden, or base)
		ParentHttpHeaderID: &baseHeaderID,
		IsDelta:            true,
		DeltaKey:           convertStringPtr("Content-Type"),
		DeltaValue:         convertStringPtr("text/xml"), // The override
		DeltaEnabled:       convertBoolPtr(true),
	}
	err = svc.httpHeaderService.Create(ctx, deltaHeader)
	require.NoError(t, err)
	// Manually publish event because we bypassed the RPC which usually publishes
	svc.streamers.HttpHeader.Publish(HttpHeaderTopic{WorkspaceID: workspaceID}, HttpHeaderEvent{
		Type:       eventTypeInsert,
		IsDelta:    true, // This is what we are testing!
		HttpHeader: converter.ToAPIHttpHeader(*deltaHeader),
	})

	// Verify Base Stream did NOT receive it (because IsDelta=true)
	select {
	case <-baseStream:
		require.FailNow(t, "Base stream received Delta Header insert event")
	case <-time.After(200 * time.Millisecond):
		// Pass
	}

	// Verify Delta Stream received it
	select {
	case resp := <-deltaStream:
		items := resp.GetItems()
		require.NotEmpty(t, items)
		insert := items[0].GetValue().GetInsert()
		require.NotNil(t, insert)
		require.Equal(t, deltaHeaderID.Bytes(), insert.DeltaHttpHeaderId)
		require.Equal(t, "text/xml", *insert.Value)
	case <-time.After(1 * time.Second):
		require.FailNow(t, "Timeout waiting for Delta Header insert event")
	}
}

func convertStringPtr(s string) *string {
	return &s
}

func convertBoolPtr(b bool) *bool {
	return &b
}

// TestHttpSync_SparsePatch verifies that updating one field of a Delta
// does not result in UNSET commands for other (unaffected) fields in the Sync event.
func TestHttpSync_SparsePatch(t *testing.T) {
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
		HttpVersion:        memory.NewInMemorySyncStreamer[HttpVersionTopic, HttpVersionEvent](),
		HttpResponse:       memory.NewInMemorySyncStreamer[HttpResponseTopic, HttpResponseEvent](),
		HttpResponseHeader: memory.NewInMemorySyncStreamer[HttpResponseHeaderTopic, HttpResponseHeaderEvent](),
		HttpResponseAssert: memory.NewInMemorySyncStreamer[HttpResponseAssertTopic, HttpResponseAssertEvent](),
		HttpBodyRaw:        memory.NewInMemorySyncStreamer[HttpBodyRawTopic, HttpBodyRawEvent](),
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
	err := svc.ws.Create(ctx, &mworkspace.Workspace{
		ID:   workspaceID,
		Name: "Test Workspace",
	})
	require.NoError(t, err)

	// Link user to workspace
	err = svc.wus.CreateWorkspaceUser(ctx, &mworkspace.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		UserID:      mwauth.LocalDummyID,
		Role:        mworkspace.RoleOwner,
	})
	require.NoError(t, err)

	// 2. Create Base HTTP Request
	baseHttpID := idwrap.NewNow()
	baseHttp := mhttp.HTTP{
		ID:          baseHttpID,
		WorkspaceID: workspaceID,
		Name:        "Base Request",
		Method:      "GET",
		Url:         "https://example.com",
		BodyKind:    mhttp.HttpBodyKindNone,
		IsDelta:     false,
	}
	err = svc.hs.Create(ctx, &baseHttp)
	require.NoError(t, err)

	// 3. Create Delta HTTP Request (linked to Base)
	deltaHttpID := idwrap.NewNow()
	deltaHttp := mhttp.HTTP{
		ID:           deltaHttpID,
		WorkspaceID:  workspaceID,
		Name:         "Base Request", // Inherited name
		Method:       "GET",
		Url:          "https://example.com",
		BodyKind:     mhttp.HttpBodyKindNone,
		ParentHttpID: &baseHttpID,
		IsDelta:      true,
		DeltaName:    convertStringPtr("Delta Override"),
		// DeltaMethod and DeltaUrl are intentionally nil (inherited)
	}
	err = svc.hs.Create(ctx, &deltaHttp)
	require.NoError(t, err)

	// 4. Start Delta Sync Stream
	deltaStream := make(chan *apiv1.HttpDeltaSyncResponse, 10)
	deltaCtx, deltaCancel := context.WithCancel(ctx)
	defer deltaCancel()

	go func() {
		err := svc.streamHttpDeltaSync(deltaCtx, mwauth.LocalDummyID, func(resp *apiv1.HttpDeltaSyncResponse) error {
			deltaStream <- resp
			return nil
		})
		if err != nil && err != context.Canceled {
			require.FailNow(t, "Delta stream error: %v", err)
		}
	}()

	// Wait for streams to initialize
	time.Sleep(100 * time.Millisecond)

	// 5. Update Delta Name ONLY
	newName := "Updated Delta Name"
	reqUpdate := &connect.Request[apiv1.HttpDeltaUpdateRequest]{
		Msg: &apiv1.HttpDeltaUpdateRequest{
			Items: []*apiv1.HttpDeltaUpdate{
				{
					DeltaHttpId: deltaHttpID.Bytes(),
					Name: &apiv1.HttpDeltaUpdate_NameUnion{
						Kind:  apiv1.HttpDeltaUpdate_NameUnion_KIND_VALUE,
						Value: &newName,
					},
					// Method and Url are omitted (undefined)
				},
			},
		},
	}
	_, err = svc.HttpDeltaUpdate(ctx, reqUpdate)
	require.NoError(t, err)

	// 6. Verify Delta Stream received a Patch
	select {
	case resp := <-deltaStream:
		items := resp.GetItems()
		require.NotEmpty(t, items)
		update := items[0].GetValue().GetUpdate()
		require.NotNil(t, update)
		require.Equal(t, deltaHttpID.Bytes(), update.DeltaHttpId)

		// Assert Name is UPDATED
		require.NotNil(t, update.Name)
		require.Equal(t, apiv1.HttpDeltaSyncUpdate_NameUnion_KIND_VALUE, update.Name.Kind)
		require.Equal(t, newName, update.Name.GetValue())

		// Assert Method is MISSING (Undefined), NOT Unset
		if update.Method != nil {
			// If present, it must NOT be UNSET. But ideally it should be nil (omitted).
			// The current buggy implementation sends UNSET because the DB value is nil.
			require.NotEqual(t, apiv1.HttpDeltaSyncUpdate_MethodUnion_KIND_UNSET, update.Method.Kind, "Method should not be explicitly UNSET when not modified")
			require.Nil(t, update.Method, "Method should be omitted from patch event")
		}

		// Assert Url is MISSING (Undefined), NOT Unset
		if update.Url != nil {
			require.NotEqual(t, apiv1.HttpDeltaSyncUpdate_UrlUnion_KIND_UNSET, update.Url.Kind, "Url should not be explicitly UNSET when not modified")
			require.Nil(t, update.Url, "Url should be omitted from patch event")
		}

	case <-time.After(1 * time.Second):
		require.FailNow(t, "Timeout waiting for Delta update event")
	}

	// 7. Verify Persistence of Name
	fetchedDelta, err := svc.httpReader.Get(ctx, deltaHttpID)
	require.NoError(t, err)
	require.NotNil(t, fetchedDelta.DeltaName)
	require.Equal(t, newName, *fetchedDelta.DeltaName)
}

func TestHttpSync_SparsePatch_URL(t *testing.T) {
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
		HttpVersion:        memory.NewInMemorySyncStreamer[HttpVersionTopic, HttpVersionEvent](),
		HttpResponse:       memory.NewInMemorySyncStreamer[HttpResponseTopic, HttpResponseEvent](),
		HttpResponseHeader: memory.NewInMemorySyncStreamer[HttpResponseHeaderTopic, HttpResponseHeaderEvent](),
		HttpResponseAssert: memory.NewInMemorySyncStreamer[HttpResponseAssertTopic, HttpResponseAssertEvent](),
		HttpBodyRaw:        memory.NewInMemorySyncStreamer[HttpBodyRawTopic, HttpBodyRawEvent](),
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
	err := svc.ws.Create(ctx, &mworkspace.Workspace{
		ID:   workspaceID,
		Name: "Test Workspace",
	})
	require.NoError(t, err)

	// Link user to workspace
	err = svc.wus.CreateWorkspaceUser(ctx, &mworkspace.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		UserID:      mwauth.LocalDummyID,
		Role:        mworkspace.RoleOwner,
	})
	require.NoError(t, err)

	// 2. Create Base HTTP Request
	baseHttpID := idwrap.NewNow()
	baseHttp := mhttp.HTTP{
		ID:          baseHttpID,
		WorkspaceID: workspaceID,
		Name:        "Base Request",
		Method:      "GET",
		Url:         "https://example.com",
		BodyKind:    mhttp.HttpBodyKindNone,
		IsDelta:     false,
	}
	err = svc.hs.Create(ctx, &baseHttp)
	require.NoError(t, err)

	// 3. Create Delta HTTP Request
	deltaHttpID := idwrap.NewNow()
	deltaHttp := mhttp.HTTP{
		ID:           deltaHttpID,
		WorkspaceID:  workspaceID,
		Name:         "Base Request",
		Method:       "GET",
		Url:          "https://example.com",
		BodyKind:     mhttp.HttpBodyKindNone,
		ParentHttpID: &baseHttpID,
		IsDelta:      true,
		DeltaName:    convertStringPtr("Delta Override"),
	}
	err = svc.hs.Create(ctx, &deltaHttp)
	require.NoError(t, err)

	// 4. Start Delta Sync Stream
	deltaStream := make(chan *apiv1.HttpDeltaSyncResponse, 10)
	deltaCtx, deltaCancel := context.WithCancel(ctx)
	defer deltaCancel()

	go func() {
		err := svc.streamHttpDeltaSync(deltaCtx, mwauth.LocalDummyID, func(resp *apiv1.HttpDeltaSyncResponse) error {
			deltaStream <- resp
			return nil
		})
		if err != nil && err != context.Canceled {
			require.FailNow(t, "Delta stream error: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// 5. Update Delta URL
	newUrl := "https://updated.com"
	reqUpdate := &connect.Request[apiv1.HttpDeltaUpdateRequest]{
		Msg: &apiv1.HttpDeltaUpdateRequest{
			Items: []*apiv1.HttpDeltaUpdate{
				{
					DeltaHttpId: deltaHttpID.Bytes(),
					Url: &apiv1.HttpDeltaUpdate_UrlUnion{
						Kind:  apiv1.HttpDeltaUpdate_UrlUnion_KIND_VALUE,
						Value: &newUrl,
					},
				},
			},
		},
	}
	_, err = svc.HttpDeltaUpdate(ctx, reqUpdate)
	require.NoError(t, err)

	// 6. Verify Delta Stream received a Patch
	select {
	case resp := <-deltaStream:
		items := resp.GetItems()
		require.NotEmpty(t, items)
		update := items[0].GetValue().GetUpdate()
		require.NotNil(t, update)
		require.Equal(t, deltaHttpID.Bytes(), update.DeltaHttpId)

		// Assert URL is UPDATED
		require.NotNil(t, update.Url)
		require.Equal(t, apiv1.HttpDeltaSyncUpdate_UrlUnion_KIND_VALUE, update.Url.Kind)
		require.Equal(t, newUrl, update.Url.GetValue())

		// Assert Name is OMITTED (Undefined)
		require.Nil(t, update.Name, "Name should be omitted from patch event")

	case <-time.After(1 * time.Second):
		require.FailNow(t, "Timeout waiting for Delta update event")
	}

	// 7. Verify Persistence
	fetchedDelta, err := svc.httpReader.Get(ctx, deltaHttpID)
	require.NoError(t, err)
	require.NotNil(t, fetchedDelta.DeltaUrl)
	require.Equal(t, newUrl, *fetchedDelta.DeltaUrl)
}
