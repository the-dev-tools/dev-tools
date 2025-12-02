package rhttp

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/eventstream/memory"
	"the-dev-tools/server/pkg/http/resolver"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/muser"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/model/mworkspaceuser"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/shttpassert"
	"the-dev-tools/server/pkg/service/shttpbodyform"
	"the-dev-tools/server/pkg/service/shttpbodyurlencoded"

	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/service/sworkspacesusers"
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
	wus sworkspacesusers.WorkspaceUserService
	es  senv.EnvService

	userID idwrap.IDWrap
}

func newHttpStreamingFixture(t *testing.T) *httpStreamingFixture {
	t.Helper()

	base := testutil.CreateBaseDB(context.Background(), t)
	services := base.GetBaseServices()
	envService := senv.New(base.Queries, base.Logger())
	varService := svar.New(base.Queries, base.Logger())
	stream := memory.NewInMemorySyncStreamer[HttpTopic, HttpEvent]()
	t.Cleanup(stream.Shutdown)

	userID := idwrap.NewNow()
	providerID := fmt.Sprintf("test-%s", userID.String())
	if err := services.Us.CreateUser(context.Background(), &muser.User{
		ID:           userID,
		Email:        fmt.Sprintf("%s@example.com", userID.String()),
		Password:     []byte("password"),
		ProviderID:   &providerID,
		ProviderType: muser.MagicLink,
		Status:       muser.Active,
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Create additional services needed for HTTP handler
	// Note: example services not needed for streaming tests

	// Child entity services from separate packages
	httpHeaderService := shttp.NewHttpHeaderService(base.Queries)
	httpSearchParamService := shttp.NewHttpSearchParamService(base.Queries)
	httpBodyFormService := shttpbodyform.New(base.Queries)
	httpBodyUrlEncodedService := shttpbodyurlencoded.New(base.Queries)
	httpAssertService := shttpassert.New(base.Queries)

	// Create response and body raw services
	httpResponseService := shttp.NewHttpResponseService(base.Queries)
	httpBodyRawService := shttp.NewHttpBodyRawService(base.Queries)

	// Streamers
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
		&services.Hs,
		&httpHeaderService,
		httpSearchParamService,
		httpBodyRawService,
		&httpBodyFormService,
		&httpBodyUrlEncodedService,
		&httpAssertService,
	)

	handler := New(base.DB, services.Hs, services.Us, services.Ws, services.Wus, envService, varService, httpBodyRawService, httpHeaderService, httpSearchParamService, httpBodyFormService, httpBodyUrlEncodedService, httpAssertService, httpResponseService, requestResolver, stream, httpHeaderStream, httpSearchParamStream, httpBodyFormStream, httpBodyUrlEncodedStream, httpAssertStream, httpVersionStream, httpResponseStream, httpResponseHeaderStream, httpResponseAssertStream, httpBodyRawStream)

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
	if err := f.ws.Create(f.ctx, ws); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	env := menv.Env{
		ID:          envID,
		WorkspaceID: workspaceID,
		Name:        "default",
		Type:        menv.EnvGlobal,
	}
	if err := f.es.CreateEnvironment(f.ctx, &env); err != nil {
		t.Fatalf("create environment: %v", err)
	}

	member := &mworkspaceuser.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		UserID:      f.userID,
		Role:        mworkspaceuser.RoleOwner,
	}
	if err := f.wus.CreateWorkspaceUser(f.ctx, member); err != nil {
		t.Fatalf("create workspace user: %v", err)
	}

	if err := f.ws.AutoLinkWorkspaceToUserList(f.ctx, workspaceID, f.userID); err != nil {
		t.Fatalf("autolink workspace: %v", err)
	}

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

	if err := f.hs.Create(f.ctx, httpModel); err != nil {
		t.Fatalf("create http: %v", err)
	}

	return httpID
}

func collectHttpSyncStreamingItems(t *testing.T, ch <-chan *httpv1.HttpSyncResponse, count int) []*httpv1.HttpSync {
	t.Helper()

	var items []*httpv1.HttpSync
	timeout := time.After(2 * time.Second)

	for len(items) < count {
		select {
		case resp, ok := <-ch:
			if !ok {
				t.Fatalf("channel closed before collecting %d items", count)
			}
			for _, item := range resp.GetItems() {
				if item != nil {
					items = append(items, item)
					if len(items) == count {
						break
					}
				}
			}
		case <-timeout:
			t.Fatalf("timeout waiting for %d items, collected %d", count, len(items))
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
		t.Fatal("Received unexpected snapshot item")
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
	if _, err := f.handler.HttpUpdate(f.ctx, updateReq); err != nil {
		t.Fatalf("HttpUpdate err: %v", err)
	}

	updateItems := collectHttpSyncStreamingItems(t, msgCh, 1)
	updateVal := updateItems[0].GetValue()
	if updateVal == nil {
		t.Fatal("update response missing value union")
	}
	if updateVal.GetKind() != httpv1.HttpSync_ValueUnion_KIND_UPDATE {
		t.Fatalf("expected update kind, got %v", updateVal.GetKind())
	}
	if got := updateVal.GetUpdate().GetName(); got != newName {
		t.Fatalf("expected updated name %q, got %q", newName, got)
	}

	deleteReq := connect.NewRequest(&httpv1.HttpDeleteRequest{
		Items: []*httpv1.HttpDelete{
			{
				HttpId: httpB.Bytes(),
			},
		},
	})
	if _, err := f.handler.HttpDelete(f.ctx, deleteReq); err != nil {
		t.Fatalf("HttpDelete err: %v", err)
	}

	deleteItems := collectHttpSyncStreamingItems(t, msgCh, 1)
	deleteVal := deleteItems[0].GetValue()
	if deleteVal == nil {
		t.Fatal("delete response missing value union")
	}
	if deleteVal.GetKind() != httpv1.HttpSync_ValueUnion_KIND_DELETE {
		t.Fatalf("expected delete kind, got %v", deleteVal.GetKind())
	}
	if got := deleteVal.GetDelete().GetHttpId(); string(got) != string(httpB.Bytes()) {
		t.Fatalf("expected deleted http %s, got %x", httpB.String(), got)
	}

	cancel()
	if err := <-errCh; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("stream returned error: %v", err)
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
		t.Fatal("Received unexpected snapshot item")
	case <-time.After(100 * time.Millisecond):
		// Good
	}

	otherUserID := idwrap.NewNow()
	providerID := fmt.Sprintf("other-%s", otherUserID.String())
	if err := f.us.CreateUser(context.Background(), &muser.User{
		ID:           otherUserID,
		Email:        fmt.Sprintf("%s@example.com", otherUserID.String()),
		Password:     []byte("password"),
		ProviderID:   &providerID,
		ProviderType: muser.MagicLink,
		Status:       muser.Active,
	}); err != nil {
		t.Fatalf("create other user: %v", err)
	}

	otherWorkspaceID := idwrap.NewNow()
	otherEnvID := idwrap.NewNow()

	ws := &mworkspace.Workspace{
		ID:        otherWorkspaceID,
		Name:      "hidden",
		Updated:   dbtime.DBNow(),
		ActiveEnv: otherEnvID,
		GlobalEnv: otherEnvID,
	}
	if err := f.ws.Create(context.Background(), ws); err != nil {
		t.Fatalf("create other workspace: %v", err)
	}

	env := menv.Env{
		ID:          otherEnvID,
		WorkspaceID: otherWorkspaceID,
		Name:        "default",
		Type:        menv.EnvGlobal,
	}
	if err := f.es.CreateEnvironment(context.Background(), &env); err != nil {
		t.Fatalf("create other env: %v", err)
	}

	otherMember := &mworkspaceuser.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: otherWorkspaceID,
		UserID:      otherUserID,
		Role:        mworkspaceuser.RoleOwner,
	}
	if err := f.wus.CreateWorkspaceUser(context.Background(), otherMember); err != nil {
		t.Fatalf("create other workspace user: %v", err)
	}

	// Create HTTP entry in hidden workspace
	hiddenHttpID := idwrap.NewNow()
	hiddenHttp := &mhttp.HTTP{
		ID:          hiddenHttpID,
		WorkspaceID: otherWorkspaceID,
		Name:        "hidden-http",
		Url:         "https://hidden.com",
		Method:      "GET",
	}
	if err := f.hs.Create(context.Background(), hiddenHttp); err != nil {
		t.Fatalf("create hidden http: %v", err)
	}

	f.handler.stream.Publish(HttpTopic{WorkspaceID: otherWorkspaceID}, HttpEvent{
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
		t.Fatalf("unexpected event for unauthorized workspace: %+v", resp)
	case <-time.After(150 * time.Millisecond):
	}

	cancel()
	if err := <-errCh; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("stream returned error: %v", err)
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
	if _, err := f.handler.HttpInsert(f.ctx, createReq); err != nil {
		t.Fatalf("HttpInsert err: %v", err)
	}

	items := collectHttpSyncStreamingItems(t, msgCh, 1)
	val := items[0].GetValue()
	if val == nil {
		t.Fatal("create response missing value union")
	}
	if val.GetKind() != httpv1.HttpSync_ValueUnion_KIND_INSERT {
		t.Fatalf("expected insert kind, got %v", val.GetKind())
	}
	if got := val.GetInsert().GetName(); got != "api-created" {
		t.Fatalf("expected created name api-created, got %q", got)
	}

	cancel()
	if err := <-errCh; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("stream returned error: %v", err)
	}
}
