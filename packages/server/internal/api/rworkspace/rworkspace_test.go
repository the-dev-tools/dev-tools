package rworkspace

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
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/muser"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/model/mworkspaceuser"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/service/sworkspacesusers"
	"the-dev-tools/server/pkg/testutil"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/workspace/v1"
)

type workspaceFixture struct {
	ctx     context.Context
	base    *testutil.BaseDBQueries
	handler WorkspaceServiceRPC

	ws  sworkspace.WorkspaceService
	wus sworkspacesusers.WorkspaceUserService
	es  senv.EnvService
	us  suser.UserService

	userID idwrap.IDWrap
}

func newWorkspaceFixture(t *testing.T) *workspaceFixture {
	t.Helper()

	base := testutil.CreateBaseDB(context.Background(), t)
	services := base.GetBaseServices()
	envService := senv.New(base.Queries, base.Logger())
	stream := memory.NewInMemorySyncStreamer[WorkspaceTopic, WorkspaceEvent]()
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

	handler := New(base.DB, services.Ws, services.Wus, services.Us, envService, stream)

	t.Cleanup(base.Close)

	return &workspaceFixture{
		ctx:     mwauth.CreateAuthedContext(context.Background(), userID),
		base:    base,
		handler: handler,
		ws:      services.Ws,
		wus:     services.Wus,
		es:      envService,
		us:      services.Us,
		userID:  userID,
	}
}

func (f *workspaceFixture) createWorkspace(t *testing.T, name string) idwrap.IDWrap {
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

	return workspaceID
}

func collectWorkspaceSyncItems(t *testing.T, ch <-chan *apiv1.WorkspaceSyncResponse, count int) []*apiv1.WorkspaceSync {
	t.Helper()

	var items []*apiv1.WorkspaceSync
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

func TestWorkspaceSyncStreamsSnapshotAndUpdates(t *testing.T) {
	t.Parallel()

	f := newWorkspaceFixture(t)
	wsA := f.createWorkspace(t, "workspace-a")
	wsB := f.createWorkspace(t, "workspace-b")

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *apiv1.WorkspaceSyncResponse, 10)
	errCh := make(chan error, 1)

	go func() {
		err := f.handler.streamWorkspaceSync(ctx, f.userID, func(resp *apiv1.WorkspaceSyncResponse) error {
			msgCh <- resp
			return nil
		})
		errCh <- err
		close(msgCh)
	}()

	snapshot := collectWorkspaceSyncItems(t, msgCh, 2)
	seen := make(map[string]bool)
	for _, item := range snapshot {
		val := item.GetValue()
		if val == nil {
			t.Fatal("snapshot item missing value union")
		}
		if val.GetKind() != apiv1.WorkspaceSync_ValueUnion_KIND_INSERT {
			t.Fatalf("expected insert kind for snapshot, got %v", val.GetKind())
		}
		seen[string(val.GetInsert().GetWorkspaceId())] = true
	}
	if !seen[string(wsA.Bytes())] || !seen[string(wsB.Bytes())] {
		t.Fatalf("snapshot missing expected workspaces, seen=%v", seen)
	}

	newName := "renamed workspace"
	updateReq := connect.NewRequest(&apiv1.WorkspaceUpdateRequest{
		Items: []*apiv1.WorkspaceUpdate{
			{
				WorkspaceId: wsA.Bytes(),
				Name:        &newName,
			},
		},
	})
	if _, err := f.handler.WorkspaceUpdate(f.ctx, updateReq); err != nil {
		t.Fatalf("WorkspaceUpdate err: %v", err)
	}

	updateItems := collectWorkspaceSyncItems(t, msgCh, 1)
	updateVal := updateItems[0].GetValue()
	if updateVal == nil {
		t.Fatal("update response missing value union")
	}
	if updateVal.GetKind() != apiv1.WorkspaceSync_ValueUnion_KIND_UPDATE {
		t.Fatalf("expected update kind, got %v", updateVal.GetKind())
	}
	if got := updateVal.GetUpdate().GetName(); got != newName {
		t.Fatalf("expected updated name %q, got %q", newName, got)
	}

	deleteReq := connect.NewRequest(&apiv1.WorkspaceDeleteRequest{
		Items: []*apiv1.WorkspaceDelete{
			{
				WorkspaceId: wsB.Bytes(),
			},
		},
	})
	if _, err := f.handler.WorkspaceDelete(f.ctx, deleteReq); err != nil {
		t.Fatalf("WorkspaceDelete err: %v", err)
	}

	deleteItems := collectWorkspaceSyncItems(t, msgCh, 1)
	deleteVal := deleteItems[0].GetValue()
	if deleteVal == nil {
		t.Fatal("delete response missing value union")
	}
	if deleteVal.GetKind() != apiv1.WorkspaceSync_ValueUnion_KIND_DELETE {
		t.Fatalf("expected delete kind, got %v", deleteVal.GetKind())
	}
	if got := deleteVal.GetDelete().GetWorkspaceId(); string(got) != string(wsB.Bytes()) {
		t.Fatalf("expected deleted workspace %s, got %x", wsB.String(), got)
	}

	cancel()
	if err := <-errCh; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("stream returned error: %v", err)
	}
}

func TestWorkspaceSyncFiltersUnauthorizedWorkspaces(t *testing.T) {
	t.Parallel()

	f := newWorkspaceFixture(t)
	f.createWorkspace(t, "visible")

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *apiv1.WorkspaceSyncResponse, 5)
	errCh := make(chan error, 1)

	go func() {
		err := f.handler.streamWorkspaceSync(ctx, f.userID, func(resp *apiv1.WorkspaceSyncResponse) error {
			msgCh <- resp
			return nil
		})
		errCh <- err
		close(msgCh)
	}()

	_ = collectWorkspaceSyncItems(t, msgCh, 1)

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

	f.handler.stream.Publish(WorkspaceTopic{WorkspaceID: otherWorkspaceID}, WorkspaceEvent{
		Type: eventTypeInsert,
		Workspace: &apiv1.Workspace{
			WorkspaceId:           otherWorkspaceID.Bytes(),
			SelectedEnvironmentId: otherEnvID.Bytes(),
			Name:                  "hidden",
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

func TestWorkspaceInsertPublishesEvent(t *testing.T) {
	t.Parallel()

	f := newWorkspaceFixture(t)

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *apiv1.WorkspaceSyncResponse, 5)
	errCh := make(chan error, 1)

	go func() {
		err := f.handler.streamWorkspaceSync(ctx, f.userID, func(resp *apiv1.WorkspaceSyncResponse) error {
			msgCh <- resp
			return nil
		})
		errCh <- err
		close(msgCh)
	}()

	createReq := connect.NewRequest(&apiv1.WorkspaceInsertRequest{
		Items: []*apiv1.WorkspaceInsert{
			{
				Name: "api-created",
			},
		},
	})
	if _, err := f.handler.WorkspaceInsert(f.ctx, createReq); err != nil {
		t.Fatalf("WorkspaceInsert err: %v", err)
	}

	items := collectWorkspaceSyncItems(t, msgCh, 1)
	val := items[0].GetValue()
	if val == nil {
		t.Fatal("create response missing value union")
	}
	if val.GetKind() != apiv1.WorkspaceSync_ValueUnion_KIND_INSERT {
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
