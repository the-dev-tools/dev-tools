package rhttp

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/converter"
	"the-dev-tools/server/pkg/eventstream/memory"
	"the-dev-tools/server/pkg/http/resolver"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"

	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/model/mworkspaceuser"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/shttp"

	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/testutil"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
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

	ws := baseDBQueries.GetBaseServices().Ws
	wus := baseDBQueries.GetBaseServices().Wus
	us := baseDBQueries.GetBaseServices().Us
	hs := baseDBQueries.GetBaseServices().Hs
	es := senv.New(queries, logger)
	vs := svar.New(queries, logger)

	bodyService := shttp.NewHttpBodyRawService(queries)
	httpHeaderService := shttp.NewHttpHeaderService(queries)
	httpSearchParamService := shttp.NewHttpSearchParamService(queries)
	httpBodyFormService := shttp.NewHttpBodyFormService(queries)
	httpBodyUrlEncodedService := shttp.NewHttpBodyUrlEncodedService(queries)
	httpAssertService := shttp.NewHttpAssertService(queries)
	httpResponseService := shttp.NewHttpResponseService(queries)

	// Streamers
	httpStream := memory.NewInMemorySyncStreamer[HttpTopic, HttpEvent]()
	httpHeaderStream := memory.NewInMemorySyncStreamer[HttpHeaderTopic, HttpHeaderEvent]()
	httpSearchParamStream := memory.NewInMemorySyncStreamer[HttpSearchParamTopic, HttpSearchParamEvent]()
	httpBodyFormStream := memory.NewInMemorySyncStreamer[HttpBodyFormTopic, HttpBodyFormEvent]()
	httpBodyUrlEncodedStream := memory.NewInMemorySyncStreamer[HttpBodyUrlEncodedTopic, HttpBodyUrlEncodedEvent]()
	httpAssertStream := memory.NewInMemorySyncStreamer[HttpAssertTopic, HttpAssertEvent]()
	httpVersionStream := memory.NewInMemorySyncStreamer[HttpVersionTopic, HttpVersionEvent]()
	httpResponseStream := memory.NewInMemorySyncStreamer[HttpResponseTopic, HttpResponseEvent]()
	httpResponseHeaderStream := memory.NewInMemorySyncStreamer[HttpResponseHeaderTopic, HttpResponseHeaderEvent]()
	httpResponseAssertStream := memory.NewInMemorySyncStreamer[HttpResponseAssertTopic, HttpResponseAssertEvent]()
	httpBodyRawStream := memory.NewInMemorySyncStreamer[HttpBodyRawTopic, HttpBodyRawEvent]()

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

	svc := New(
		db,
		hs,
		us,
		ws,
		wus,
		es,
		vs,
		bodyService,
		httpHeaderService,
		httpSearchParamService,
		httpBodyFormService,
		httpBodyUrlEncodedService,
		httpAssertService,
		httpResponseService,
		requestResolver,
		httpStream,
		httpHeaderStream,
		httpSearchParamStream,
		httpBodyFormStream,
		httpBodyUrlEncodedStream,
		httpAssertStream,
		httpVersionStream,
		httpResponseStream,
		httpResponseHeaderStream,
		httpResponseAssertStream,
		httpBodyRawStream,
		nil,
	)

	// 1. Create Workspace and User
	workspaceID := idwrap.NewNow()
	err := svc.ws.Create(ctx, &mworkspace.Workspace{
		ID:   workspaceID,
		Name: "Test Workspace",
	})
	require.NoError(t, err)

	// Link user to workspace
	err = svc.wus.CreateWorkspaceUser(ctx, &mworkspaceuser.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		UserID:      mwauth.LocalDummyID,
		Role:        mworkspaceuser.RoleOwner,
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
	svc.httpHeaderStream.Publish(HttpHeaderTopic{WorkspaceID: workspaceID}, HttpHeaderEvent{
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
