package rhttp

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/eventstream/memory"
	"the-dev-tools/server/pkg/http/resolver"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/testutil"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
	globalv1 "the-dev-tools/spec/dist/buf/go/global/v1"
)

// TestHttpBodyRawDelta_Unset verifies that UNSETting a delta body works correctly
// and propagates the UNSET event instead of an empty value.
func TestHttpBodyRawDelta_Unset(t *testing.T) {
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
		HttpBodyRaw: memory.NewInMemorySyncStreamer[HttpBodyRawTopic, HttpBodyRawEvent](),
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

	svc := New(
		db,
		httpReader,
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
		httpStreamers,
	)

	// 1. Create Workspace and User
	workspaceID := idwrap.NewNow()
	err := svc.ws.Create(ctx, &mworkspace.Workspace{
		ID:   workspaceID,
		Name: "Test Workspace",
	})
	require.NoError(t, err)

	err = svc.wus.CreateWorkspaceUser(ctx, &mworkspace.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		UserID:      mwauth.LocalDummyID,
		Role:        mworkspace.RoleOwner,
	})
	require.NoError(t, err)

	// 2. Create Base HTTP Request with Body
	baseHttpID := idwrap.NewNow()
	baseHttp := mhttp.HTTP{
		ID:          baseHttpID,
		WorkspaceID: workspaceID,
		Name:        "Base Request",
		Method:      "POST",
		Url:         "https://example.com",
		BodyKind:    mhttp.HttpBodyKindRaw,
		IsDelta:     false,
	}
	err = svc.hs.Create(ctx, &baseHttp)
	require.NoError(t, err)

	_, err = svc.bodyService.CreateFull(ctx, &mhttp.HTTPBodyRaw{
		ID:              idwrap.NewNow(),
		HttpID:          baseHttpID,
		RawData:         []byte("base-body"),
		CompressionType: 0,
	})
	require.NoError(t, err)

	// 3. Create Delta HTTP Request with Override Body
	deltaHttpID := idwrap.NewNow()
	deltaHttp := mhttp.HTTP{
		ID:           deltaHttpID,
		WorkspaceID:  workspaceID,
		Name:         "Base Request",
		Method:       "POST",
		Url:          "https://example.com",
		BodyKind:     mhttp.HttpBodyKindRaw,
		ParentHttpID: &baseHttpID,
		IsDelta:      true,
	}
	err = svc.hs.Create(ctx, &deltaHttp)
	require.NoError(t, err)

	// Create Delta Body Override
	_, err = svc.bodyService.CreateDelta(ctx, deltaHttpID, []byte("delta-body"))
	require.NoError(t, err)

	// 4. Start Delta Body Sync Stream
	deltaStream := make(chan *apiv1.HttpBodyRawDeltaSyncResponse, 10)
	deltaCtx, deltaCancel := context.WithCancel(ctx)
	defer deltaCancel()

	go func() {
		err := svc.streamHttpBodyRawDeltaSync(deltaCtx, mwauth.LocalDummyID, func(resp *apiv1.HttpBodyRawDeltaSyncResponse) error {
			deltaStream <- resp
			return nil
		})
		if err != nil && err != context.Canceled {
			require.FailNow(t, "Delta stream error: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// 5. UNSET Delta Body (Revert to Base)
	reqUpdate := &connect.Request[apiv1.HttpBodyRawDeltaUpdateRequest]{
		Msg: &apiv1.HttpBodyRawDeltaUpdateRequest{
			Items: []*apiv1.HttpBodyRawDeltaUpdate{
				{
					HttpId: deltaHttpID.Bytes(), // Usually ID is HttpId for body raw delta updates in API
					Data: &apiv1.HttpBodyRawDeltaUpdate_DataUnion{
						Kind:  apiv1.HttpBodyRawDeltaUpdate_DataUnion_KIND_UNSET,
						Unset: globalv1.Unset_UNSET.Enum(),
					},
				},
			},
		},
	}
	_, err = svc.HttpBodyRawDeltaUpdate(ctx, reqUpdate)
	require.NoError(t, err)

	// 6. Verify Delta Stream received UNSET
	select {
	case resp := <-deltaStream:
		items := resp.GetItems()
		require.NotEmpty(t, items)
		update := items[0].GetValue().GetUpdate()
		require.NotNil(t, update)
		require.Equal(t, deltaHttpID.Bytes(), update.DeltaHttpId)

		// Assert Data is UNSET
		// Current buggy implementation likely sends VALUE=""
		require.NotNil(t, update.Data)
		if update.Data.Kind == apiv1.HttpBodyRawDeltaSyncUpdate_DataUnion_KIND_VALUE {
			val := update.Data.GetValue()
			if val == "" {
				t.Fatalf("Bug confirmed: Received VALUE='' instead of UNSET")
			}
			t.Fatalf("Received unexpected VALUE: %s", val)
		}
		require.Equal(t, apiv1.HttpBodyRawDeltaSyncUpdate_DataUnion_KIND_UNSET, update.Data.Kind)

	case <-time.After(1 * time.Second):
		require.FailNow(t, "Timeout waiting for Delta update event")
	}

	// 7. Verify Persistence
	fetchedBody, err := svc.bodyService.GetByHttpID(ctx, deltaHttpID)
	require.NoError(t, err)
	// DeltaRawData should be nil or empty.
	// mhttp uses []byte. Nil or empty slice both mean empty.
	// But logically, if we Unset, it means inherit.
	// If mhttp.HTTPBodyRaw doesn't distinguish between "Empty Body Override" and "Inherit Body", that's a deeper model issue.
	// Assuming empty slice means inherit for DeltaRawData (based on resolveRawBody in delta.go).
	require.Empty(t, fetchedBody.DeltaRawData)
}
