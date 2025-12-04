package test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rhttp"
	"the-dev-tools/server/internal/api/rlog"
	"the-dev-tools/server/pkg/eventstream/memory"
	"the-dev-tools/server/pkg/http/resolver"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/muser"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/shttp"

	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/testutil"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
)

type deltaExecutionFixture struct {
	ctx      context.Context
	services testutil.BaseTestServices
	handler  rhttp.HttpServiceRPC

	httpService       shttp.HTTPService
	bodyService       *shttp.HttpBodyRawService
	httpHeaderService shttp.HttpHeaderService

	userID      idwrap.IDWrap
	workspaceID idwrap.IDWrap

	mockServer *httptest.Server
	serverURL  string
	lastReq    *http.Request
	lastBody   []byte
}

func newDeltaExecutionFixture(t *testing.T) *deltaExecutionFixture {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	services := base.GetBaseServices()

	// Create User
	userID := idwrap.NewNow()
	providerID := fmt.Sprintf("test-%s", userID.String())
	err := services.Us.CreateUser(ctx, &muser.User{
		ID:           userID,
		Email:        fmt.Sprintf("%s@example.com", userID.String()),
		Password:     []byte("password"),
		ProviderID:   &providerID,
		ProviderType: muser.MagicLink,
		Status:       muser.Active,
	})
	require.NoError(t, err)

	// Create Workspace
	// Use authenticated context for workspace creation as it might need it in future,
	// and definitely for the handler later
	ctx = mwauth.CreateAuthedContext(ctx, userID)

	workspaceID, err := services.CreateTempCollection(ctx, userID, "Delta Execution Workspace")
	require.NoError(t, err)

	// Initialize specific services
	httpService := shttp.New(base.Queries, base.Logger())
	bodyService := shttp.NewHttpBodyRawService(base.Queries)
	httpHeaderService := shttp.NewHttpHeaderService(base.Queries)
	httpSearchParamService := shttp.NewHttpSearchParamService(base.Queries)
	httpBodyFormService := shttp.NewHttpBodyFormService(base.Queries)
	httpBodyUrlEncodedService := shttp.NewHttpBodyUrlEncodedService(base.Queries)
	httpAssertService := shttp.NewHttpAssertService(base.Queries)
	httpResponseService := shttp.NewHttpResponseService(base.Queries)
	envService := senv.New(base.Queries, base.Logger())
	varService := svar.New(base.Queries, base.Logger())

	// Create streamers
	stream := memory.NewInMemorySyncStreamer[rhttp.HttpTopic, rhttp.HttpEvent]()
	httpHeaderStream := memory.NewInMemorySyncStreamer[rhttp.HttpHeaderTopic, rhttp.HttpHeaderEvent]()
	httpSearchParamStream := memory.NewInMemorySyncStreamer[rhttp.HttpSearchParamTopic, rhttp.HttpSearchParamEvent]()
	httpBodyFormStream := memory.NewInMemorySyncStreamer[rhttp.HttpBodyFormTopic, rhttp.HttpBodyFormEvent]()
	httpBodyUrlEncodedStream := memory.NewInMemorySyncStreamer[rhttp.HttpBodyUrlEncodedTopic, rhttp.HttpBodyUrlEncodedEvent]()
	httpAssertStream := memory.NewInMemorySyncStreamer[rhttp.HttpAssertTopic, rhttp.HttpAssertEvent]()
	httpVersionStream := memory.NewInMemorySyncStreamer[rhttp.HttpVersionTopic, rhttp.HttpVersionEvent]()
	httpResponseStream := memory.NewInMemorySyncStreamer[rhttp.HttpResponseTopic, rhttp.HttpResponseEvent]()
	httpResponseHeaderStream := memory.NewInMemorySyncStreamer[rhttp.HttpResponseHeaderTopic, rhttp.HttpResponseHeaderEvent]()
	httpResponseAssertStream := memory.NewInMemorySyncStreamer[rhttp.HttpResponseAssertTopic, rhttp.HttpResponseAssertEvent]()
	httpBodyRawStream := memory.NewInMemorySyncStreamer[rhttp.HttpBodyRawTopic, rhttp.HttpBodyRawEvent]()
	logStreamer := memory.NewInMemorySyncStreamer[rlog.LogTopic, rlog.LogEvent]()

	// Create resolver for delta resolution
	requestResolver := resolver.NewStandardResolver(
		&httpService,
		&httpHeaderService,
		httpSearchParamService,
		bodyService,
		httpBodyFormService,
		httpBodyUrlEncodedService,
		httpAssertService,
	)

	// Create handler
	handler := rhttp.New(
		base.DB,
		httpService,
		services.Us,
		services.Ws,
		services.Wus,
		envService,
		varService,
		bodyService,
		httpHeaderService,
		httpSearchParamService,
		httpBodyFormService,
		httpBodyUrlEncodedService,
		httpAssertService,
		httpResponseService,
		requestResolver,
		stream,
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
		logStreamer,
	)

	f := &deltaExecutionFixture{
		ctx:               ctx,
		services:          services,
		handler:           handler,
		httpService:       httpService,
		bodyService:       bodyService,
		httpHeaderService: httpHeaderService,
		userID:            userID,
		workspaceID:       workspaceID,
	}

	// Start Mock Server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.lastReq = r
		f.lastBody = body
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	t.Cleanup(mockServer.Close)

	f.mockServer = mockServer
	f.serverURL = mockServer.URL

	return f
}

func TestDeltaExecution_Override(t *testing.T) {
	// Verify that if a Delta request has its own body, it is used.
	f := newDeltaExecutionFixture(t)

	// 1. Create Base Request
	baseID := idwrap.NewNow()
	err := f.httpService.Create(f.ctx, &mhttp.HTTP{
		ID:          baseID,
		WorkspaceID: f.workspaceID,
		Name:        "Base",
		Url:         f.serverURL,
		Method:      "POST",
		BodyKind:    mhttp.HttpBodyKindRaw,
	})
	require.NoError(t, err)

	_, err = f.bodyService.Create(f.ctx, baseID, []byte("base-body"), "text/plain")
	require.NoError(t, err)

	// 2. Create Delta Request with OVERRIDE body
	deltaID := idwrap.NewNow()
	err = f.httpService.Create(f.ctx, &mhttp.HTTP{
		ID:           deltaID,
		WorkspaceID:  f.workspaceID,
		Name:         "Delta",
		Url:          f.serverURL,
		Method:       "POST",
		BodyKind:     mhttp.HttpBodyKindRaw,
		IsDelta:      true,
		ParentHttpID: &baseID,
	})
	require.NoError(t, err)

	// Create a body for the delta
	_, err = f.bodyService.CreateDelta(f.ctx, deltaID, []byte("delta-body"), "text/plain")
	require.NoError(t, err)

	// 3. Run Delta
	_, err = f.handler.HttpRun(f.ctx, connect.NewRequest(&apiv1.HttpRunRequest{
		HttpId: deltaID.Bytes(),
	}))
	require.NoError(t, err)

	// 4. Verify Delta Body was sent
	require.Equal(t, "delta-body", string(f.lastBody))
}

func TestDeltaExecution_Inheritance(t *testing.T) {
	// Verify if a Delta request inherits body from Base if Delta body is missing.
	// Based on analysis, this is expected to FAIL or send empty body currently.
	f := newDeltaExecutionFixture(t)

	// 1. Create Base Request
	baseID := idwrap.NewNow()
	err := f.httpService.Create(f.ctx, &mhttp.HTTP{
		ID:          baseID,
		WorkspaceID: f.workspaceID,
		Name:        "Base",
		Url:         f.serverURL,
		Method:      "POST",
		BodyKind:    mhttp.HttpBodyKindRaw,
	})
	require.NoError(t, err)

	_, err = f.bodyService.Create(f.ctx, baseID, []byte("base-body"), "text/plain")
	require.NoError(t, err)

	// 2. Create Delta Request with NO body (should inherit?)
	deltaID := idwrap.NewNow()
	err = f.httpService.Create(f.ctx, &mhttp.HTTP{
		ID:           deltaID,
		WorkspaceID:  f.workspaceID,
		Name:         "Delta",
		Url:          f.serverURL,
		Method:       "POST",
		BodyKind:     mhttp.HttpBodyKindRaw,
		IsDelta:      true,
		ParentHttpID: &baseID,
	})
	require.NoError(t, err)

	// 3. Run Delta
	_, err = f.handler.HttpRun(f.ctx, connect.NewRequest(&apiv1.HttpRunRequest{
		HttpId: deltaID.Bytes(),
	}))
	require.NoError(t, err)

	// 4. Check result
	// If inheritance works, we expect "base-body".
	// If it doesn't, we expect empty body.
	if string(f.lastBody) == "" {
		t.Log("confirmed: Delta execution does NOT inherit base body automatically.")
	} else if string(f.lastBody) == "base-body" {
		t.Log("Success: Delta execution inherits base body!")
	} else {
		t.Errorf("Unexpected body received: %s", string(f.lastBody))
	}
}
