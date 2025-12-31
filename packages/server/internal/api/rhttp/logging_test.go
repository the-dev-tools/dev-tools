package rhttp

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rlog"
	"the-dev-tools/server/pkg/eventstream/memory"
	"the-dev-tools/server/pkg/http/resolver"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/muser"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/testutil"
	httpv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
)

func TestHttpRun_Logging(t *testing.T) {
	ctx := context.Background()
	userID := idwrap.NewNow()
	ctx = mwauth.CreateAuthedContext(ctx, userID)

	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()

	services := base.GetBaseServices()
	envService := senv.NewEnvironmentService(base.Queries, base.Logger())
	varService := senv.NewVariableService(base.Queries, base.Logger())

	// Setup Streamers
	httpStreamers := &HttpStreamers{
		Log:         memory.NewInMemorySyncStreamer[rlog.LogTopic, rlog.LogEvent](),
		Http:        memory.NewInMemorySyncStreamer[HttpTopic, HttpEvent](),
		HttpVersion: memory.NewInMemorySyncStreamer[HttpVersionTopic, HttpVersionEvent](),
	}
	defer func() {
		if httpStreamers.Log != nil {
			httpStreamers.Log.Shutdown()
		}
		if httpStreamers.Http != nil {
			httpStreamers.Http.Shutdown()
		}
		if httpStreamers.HttpVersion != nil {
			httpStreamers.HttpVersion.Shutdown()
		}
	}()

	// Other services
	httpHeaderService := shttp.NewHttpHeaderService(base.Queries)
	httpSearchParamService := shttp.NewHttpSearchParamService(base.Queries)
	httpBodyFormService := shttp.NewHttpBodyFormService(base.Queries)
	httpBodyUrlEncodedService := shttp.NewHttpBodyUrlEncodedService(base.Queries)
	httpAssertService := shttp.NewHttpAssertService(base.Queries)
	httpResponseService := shttp.NewHttpResponseService(base.Queries)
	httpBodyRawService := shttp.NewHttpBodyRawService(base.Queries)

	requestResolver := resolver.NewStandardResolver(
		&services.Hs,
		&httpHeaderService,
		httpSearchParamService,
		httpBodyRawService,
		httpBodyFormService,
		httpBodyUrlEncodedService,
		httpAssertService,
	)

	httpReader := shttp.NewReader(base.DB, base.Logger(), &services.Wus)

	handler := New(HttpServiceRPCDeps{
		DB: base.DB,
		Readers: HttpServiceRPCReaders{
			Http:      httpReader,
			User:      services.Wus.Reader(),
			Workspace: services.Ws.Reader(),
		},
		Services: HttpServiceRPCServices{
			Http:               services.Hs,
			User:               services.Us,
			Workspace:          services.Ws,
			WorkspaceUser:      services.Wus,
			Env:                envService,
			Variable:           varService,
			HttpBodyRaw:        httpBodyRawService,
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

	// Setup Data
	providerID := fmt.Sprintf("test-%s", userID.String())
	err := services.Us.CreateUser(context.Background(), &muser.User{
		ID:           userID,
		Email:        fmt.Sprintf("%s@example.com", userID.String()),
		Password:     []byte("password"),
		ProviderID:   &providerID,
		ProviderType: muser.MagicLink,
		Status:       muser.Active,
	})
	require.NoError(t, err)

	workspaceID := idwrap.NewNow()
	envID := idwrap.NewNow()

	ws := &mworkspace.Workspace{
		ID:        workspaceID,
		Name:      "Test Workspace",
		ActiveEnv: envID,
		GlobalEnv: envID,
	}
	err = services.Ws.Create(ctx, ws)
	require.NoError(t, err)

	env := menv.Env{
		ID:          envID,
		WorkspaceID: workspaceID,
		Name:        "default",
		Type:        menv.EnvGlobal,
	}
	err = envService.CreateEnvironment(ctx, &env)
	require.NoError(t, err)

	member := &mworkspace.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        mworkspace.RoleOwner,
	}
	err = services.Wus.CreateWorkspaceUser(ctx, member)
	require.NoError(t, err)

	// Create HTTP
	testServer := createStatusServer(t, http.StatusOK)
	defer testServer.Close()

	httpID := idwrap.NewNow()
	httpModel := &mhttp.HTTP{
		ID:          httpID,
		WorkspaceID: workspaceID,
		Name:        "Test HTTP",
		Url:         testServer.URL,
		Method:      "GET",
	}
	err = services.Hs.Create(ctx, httpModel)
	require.NoError(t, err)

	// Subscribe to logs
	logCh, err := httpStreamers.Log.Subscribe(ctx, func(topic rlog.LogTopic) bool {
		return topic.UserID == userID
	})
	require.NoError(t, err)

	// Run HTTP
	req := connect.NewRequest(&httpv1.HttpRunRequest{
		HttpId: httpID.Bytes(),
	})
	_, err = handler.HttpRun(ctx, req)
	require.NoError(t, err)

	// Wait for logs
	select {
	case evt := <-logCh:
		l := evt.Payload
		assert.Equal(t, rlog.EventTypeInsert, l.Type)
		assert.NotNil(t, l.Log)
		assert.Equal(t, "HTTP Test HTTP: Success", l.Log.Name)

		// Check structured value
		val := l.Log.Value.GetStructValue()
		require.NotNil(t, val)
		fields := val.Fields
		assert.Contains(t, fields, "http_id")
		assert.Contains(t, fields, "status")
		assert.Equal(t, httpID.String(), fields["http_id"].GetStringValue())
		assert.Equal(t, "Success", fields["status"].GetStringValue())

	case <-time.After(2 * time.Second):
		require.FailNow(t, "Timeout waiting for logs")
	}
}
