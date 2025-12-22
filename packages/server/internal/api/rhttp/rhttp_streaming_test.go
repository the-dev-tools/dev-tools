package rhttp

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/eventstream/memory"
	"the-dev-tools/server/pkg/http/resolver"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/muser"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/testutil"
	httpv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
)

type httpStreamingFixture struct {
	ctx     context.Context
	base    *testutil.BaseDBQueries
	handler HttpServiceRPC

	hs  shttp.HTTPService
	us  suser.UserService
	ws  sworkspace.WorkspaceService
	wus sworkspace.UserService
	es  senv.EnvService

	userID idwrap.IDWrap
}

func newHttpStreamingFixture(t *testing.T) *httpStreamingFixture {
	t.Helper()

	base := testutil.CreateBaseDB(context.Background(), t)
	services := base.GetBaseServices()
	envService := senv.NewEnvironmentService(base.Queries, base.Logger())
	varService := senv.NewVariableService(base.Queries, base.Logger())

	userID := idwrap.NewNow()
	providerID := fmt.Sprintf("test-%s", userID.String())
	require.NoError(t, services.Us.CreateUser(context.Background(), &muser.User{
		ID:           userID,
		Email:        fmt.Sprintf("%s@example.com", userID.String()),
		Password:     []byte("password"),
		ProviderID:   &providerID,
		ProviderType: muser.MagicLink,
		Status:       muser.Active,
	}), "create user")

	// Create additional services needed for HTTP handler
	// Note: example services not needed for streaming tests

	// Child entity services from separate packages
	httpHeaderService := shttp.NewHttpHeaderService(base.Queries)
	httpSearchParamService := shttp.NewHttpSearchParamService(base.Queries)
	httpBodyFormService := shttp.NewHttpBodyFormService(base.Queries)
	httpBodyUrlEncodedService := shttp.NewHttpBodyUrlEncodedService(base.Queries)
	httpAssertService := shttp.NewHttpAssertService(base.Queries)

	// Create response and body raw services
	httpResponseService := shttp.NewHttpResponseService(base.Queries)
	httpBodyRawService := shttp.NewHttpBodyRawService(base.Queries)

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

	t.Cleanup(func() {
		httpStreamers.Http.Shutdown()
		httpStreamers.HttpHeader.Shutdown()
		httpStreamers.HttpSearchParam.Shutdown()
		httpStreamers.HttpBodyForm.Shutdown()
		httpStreamers.HttpBodyUrlEncoded.Shutdown()
		httpStreamers.HttpAssert.Shutdown()
		httpStreamers.HttpVersion.Shutdown()
		httpStreamers.HttpResponse.Shutdown()
		httpStreamers.HttpResponseHeader.Shutdown()
		httpStreamers.HttpResponseAssert.Shutdown()
		httpStreamers.HttpBodyRaw.Shutdown()
	})

	// Create resolver for delta resolution
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

	handler := New(base.DB, httpReader, services.Hs, services.Us, services.Ws, services.Wus, envService, varService, httpBodyRawService, httpHeaderService, httpSearchParamService, httpBodyFormService, httpBodyUrlEncodedService, httpAssertService, httpResponseService, requestResolver, httpStreamers)

	t.Cleanup(base.Close)

	return &httpStreamingFixture{
		ctx:     mwauth.CreateAuthedContext(context.Background(), userID),
		base:    base,
		handler: handler,
		hs:      services.Hs,
		us:      services.Us,
		ws:      services.Ws,
		wus:     services.Wus,
		es:      envService,
		userID:  userID,
	}
}

func (f *httpStreamingFixture) createWorkspace(t *testing.T, name string) idwrap.IDWrap {
	t.Helper()

	workspaceID := idwrap.NewNow()
	envID := idwrap.NewNow()

	ws := &mworkspace.Workspace{
		ID:        workspaceID,
		Name:      name,
		Updated:   dbtime.DBNow(),
		ActiveEnv: envID,
		GlobalEnv: envID,
	}
	require.NoError(t, f.ws.Create(f.ctx, ws), "create workspace")

	env := menv.Env{
		ID:          envID,
		WorkspaceID: workspaceID,
		Name:        "default",
		Type:        menv.EnvGlobal,
	}
	require.NoError(t, f.es.CreateEnvironment(f.ctx, &env), "create environment")

	member := &mworkspace.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		UserID:      f.userID,
		Role:        mworkspace.RoleOwner,
	}
	require.NoError(t, f.wus.CreateWorkspaceUser(f.ctx, member), "create workspace user")

	return workspaceID
}

func (f *httpStreamingFixture) createHttp(t *testing.T, workspaceID idwrap.IDWrap, name, url, method string) idwrap.IDWrap {
	t.Helper()

	httpID := idwrap.NewNow()
	httpModel := &mhttp.HTTP{
		ID:          httpID,
		WorkspaceID: workspaceID,
		Name:        name,
		Url:         url,
		Method:      method,
		Description: "Test HTTP entry",
	}

	require.NoError(t, f.hs.Create(f.ctx, httpModel), "create http")

	return httpID
}

func collectHttpSyncStreamingItems(t *testing.T, ch <-chan *httpv1.HttpSyncResponse, count int) []*httpv1.HttpSync {
	t.Helper()

	var items []*httpv1.HttpSync
	timeout := time.After(2 * time.Second)

	for len(items) < count {
		select {
		case resp, ok := <-ch:
			require.True(t, ok, "channel closed before collecting %d items", count)
			for _, item := range resp.GetItems() {
				if item != nil {
					items = append(items, item)
					if len(items) == count {
						break
					}
				}
			}
		case <-timeout:
			require.FailNow(t, fmt.Sprintf("timeout waiting for %d items, collected %d", count, len(items)))
		}
	}

	return items
}

func TestHttpSyncStreamsSnapshotAndUpdatesStreaming(t *testing.T) {
	t.Parallel()

	f := newHttpStreamingFixture(t)
	wsA := f.createWorkspace(t, "workspace-a")
	wsB := f.createWorkspace(t, "workspace-b")
	httpA := f.createHttp(t, wsA, "http-a", "https://example.com/a", "GET")
	httpB := f.createHttp(t, wsB, "http-b", "https://example.com/b", "POST")

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *httpv1.HttpSyncResponse, 10)
	errCh := make(chan error, 1)

	go func() {
		err := f.handler.streamHttpSync(ctx, f.userID, func(resp *httpv1.HttpSyncResponse) error {
			msgCh <- resp
			return nil
		})
		errCh <- err
		close(msgCh)
	}()

	// Snapshot removed
	select {
	case <-msgCh:
		require.FailNow(t, "Received unexpected snapshot item")
	case <-time.After(100 * time.Millisecond):
		// Good
	}

	newName := "renamed http"
	updateReq := connect.NewRequest(&httpv1.HttpUpdateRequest{
		Items: []*httpv1.HttpUpdate{
			{
				HttpId: httpA.Bytes(),
				Name:   &newName,
			},
		},
	})
	_, err := f.handler.HttpUpdate(f.ctx, updateReq)
	require.NoError(t, err, "HttpUpdate err")

	updateItems := collectHttpSyncStreamingItems(t, msgCh, 1)
	updateVal := updateItems[0].GetValue()
	require.NotNil(t, updateVal, "update response missing value union")
	require.Equal(t, httpv1.HttpSync_ValueUnion_KIND_UPDATE, updateVal.GetKind(), "expected update kind")
	require.Equal(t, newName, updateVal.GetUpdate().GetName(), "expected updated name")

	deleteReq := connect.NewRequest(&httpv1.HttpDeleteRequest{
		Items: []*httpv1.HttpDelete{
			{
				HttpId: httpB.Bytes(),
			},
		},
	})
	_, err = f.handler.HttpDelete(f.ctx, deleteReq)
	require.NoError(t, err, "HttpDelete err")

	deleteItems := collectHttpSyncStreamingItems(t, msgCh, 1)
	deleteVal := deleteItems[0].GetValue()
	require.NotNil(t, deleteVal, "delete response missing value union")
	require.Equal(t, httpv1.HttpSync_ValueUnion_KIND_DELETE, deleteVal.GetKind(), "expected delete kind")
	require.Equal(t, httpB.Bytes(), deleteVal.GetDelete().GetHttpId(), "expected deleted http %s", httpB.String())

	cancel()
	err = <-errCh
	if err != nil {
		require.True(t, errors.Is(err, context.Canceled), "stream returned error: %v", err)
	}
}

func TestHttpSyncFiltersUnauthorizedWorkspacesStreaming(t *testing.T) {
	t.Parallel()

	f := newHttpStreamingFixture(t)
	wsVisible := f.createWorkspace(t, "visible")
	f.createHttp(t, wsVisible, "visible-http", "https://visible.com", "GET")

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *httpv1.HttpSyncResponse, 5)
	errCh := make(chan error, 1)

	go func() {
		err := f.handler.streamHttpSync(ctx, f.userID, func(resp *httpv1.HttpSyncResponse) error {
			msgCh <- resp
			return nil
		})
		errCh <- err
		close(msgCh)
	}()

	// Snapshot removed
	select {
	case <-msgCh:
		require.FailNow(t, "Received unexpected snapshot item")
	case <-time.After(100 * time.Millisecond):
		// Good
	}

	otherUserID := idwrap.NewNow()
	providerID := fmt.Sprintf("other-%s", otherUserID.String())
	require.NoError(t, f.us.CreateUser(context.Background(), &muser.User{
		ID:           otherUserID,
		Email:        fmt.Sprintf("%s@example.com", otherUserID.String()),
		Password:     []byte("password"),
		ProviderID:   &providerID,
		ProviderType: muser.MagicLink,
		Status:       muser.Active,
	}), "create other user")

	otherWorkspaceID := idwrap.NewNow()
	otherEnvID := idwrap.NewNow()

	ws := &mworkspace.Workspace{
		ID:        otherWorkspaceID,
		Name:      "hidden",
		Updated:   dbtime.DBNow(),
		ActiveEnv: otherEnvID,
		GlobalEnv: otherEnvID,
	}
	require.NoError(t, f.ws.Create(context.Background(), ws), "create other workspace")

	env := menv.Env{
		ID:          otherEnvID,
		WorkspaceID: otherWorkspaceID,
		Name:        "default",
		Type:        menv.EnvGlobal,
	}
	require.NoError(t, f.es.CreateEnvironment(context.Background(), &env), "create other env")

	otherMember := &mworkspace.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: otherWorkspaceID,
		UserID:      otherUserID,
		Role:        mworkspace.RoleOwner,
	}
	require.NoError(t, f.wus.CreateWorkspaceUser(context.Background(), otherMember), "create other workspace user")

	// Create HTTP entry in hidden workspace
	hiddenHttpID := idwrap.NewNow()
	hiddenHttp := &mhttp.HTTP{
		ID:          hiddenHttpID,
		WorkspaceID: otherWorkspaceID,
		Name:        "hidden-http",
		Url:         "https://hidden.com",
		Method:      "GET",
	}
	require.NoError(t, f.hs.Create(context.Background(), hiddenHttp), "create hidden http")

	f.handler.streamers.Http.Publish(HttpTopic{WorkspaceID: otherWorkspaceID}, HttpEvent{
		Type: "insert",
		Http: &httpv1.Http{
			HttpId: hiddenHttpID.Bytes(),
			Name:   "hidden-http",
			Url:    "https://hidden.com",
			Method: httpv1.HttpMethod_HTTP_METHOD_GET,
		},
	})

	select {
	case resp := <-msgCh:
		require.FailNow(t, fmt.Sprintf("unexpected event for unauthorized workspace: %+v", resp))
	case <-time.After(150 * time.Millisecond):
	}

	cancel()
	err := <-errCh
	if err != nil {
		require.True(t, errors.Is(err, context.Canceled), "stream returned error: %v", err)
	}
}

func TestHttpCreatePublishesEventStreaming(t *testing.T) {
	t.Parallel()

	f := newHttpStreamingFixture(t)
	_ = f.createWorkspace(t, "test-workspace") // Ensure user has a workspace for HTTP creation

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *httpv1.HttpSyncResponse, 5)
	errCh := make(chan error, 1)

	go func() {
		err := f.handler.streamHttpSync(ctx, f.userID, func(resp *httpv1.HttpSyncResponse) error {
			msgCh <- resp
			return nil
		})
		errCh <- err
		close(msgCh)
	}()

	httpID := idwrap.NewNow()
	createReq := connect.NewRequest(&httpv1.HttpInsertRequest{
		Items: []*httpv1.HttpInsert{
			{
				HttpId: httpID.Bytes(),
				Name:   "api-created",
				Url:    "https://api-created.com",
				Method: httpv1.HttpMethod_HTTP_METHOD_POST,
			},
		},
	})
	_, err := f.handler.HttpInsert(f.ctx, createReq)
	require.NoError(t, err, "HttpInsert err")

	items := collectHttpSyncStreamingItems(t, msgCh, 1)
	val := items[0].GetValue()
	require.NotNil(t, val, "create response missing value union")
	require.Equal(t, httpv1.HttpSync_ValueUnion_KIND_INSERT, val.GetKind(), "expected insert kind")
	require.Equal(t, "api-created", val.GetInsert().GetName(), "expected created name api-created")

	cancel()
	err = <-errCh
	if err != nil {
		require.True(t, errors.Is(err, context.Canceled), "stream returned error: %v", err)
	}
}
