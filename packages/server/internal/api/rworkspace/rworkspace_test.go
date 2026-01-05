package rworkspace

import (
	"context"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/renv"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/eventstream/memory"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/muser"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/testutil"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/workspace/v1"
)

type workspaceFixture struct {
	ctx     context.Context
	base    *testutil.BaseDBQueries
	handler WorkspaceServiceRPC

	ws  sworkspace.WorkspaceService
	wus sworkspace.UserService
	es  senv.EnvService
	us  suser.UserService

	userID idwrap.IDWrap
}

func newWorkspaceFixture(t *testing.T) *workspaceFixture {
	t.Helper()

	base := testutil.CreateBaseDB(context.Background(), t)
	services := base.GetBaseServices()
	envService := senv.NewEnvironmentService(base.Queries, base.Logger())
	stream := memory.NewInMemorySyncStreamer[WorkspaceTopic, WorkspaceEvent]()
	envStream := memory.NewInMemorySyncStreamer[renv.EnvironmentTopic, renv.EnvironmentEvent]()
	t.Cleanup(stream.Shutdown)
	t.Cleanup(envStream.Shutdown)

	userID := idwrap.NewNow()
	providerID := fmt.Sprintf("test-%s", userID.String())
	err := services.UserService.CreateUser(context.Background(), &muser.User{
		ID:           userID,
		Email:        fmt.Sprintf("%s@example.com", userID.String()),
		Password:     []byte("password"),
		ProviderID:   &providerID,
		ProviderType: muser.MagicLink,
		Status:       muser.Active,
	})
	require.NoError(t, err, "create user")

	handler := New(WorkspaceServiceRPCDeps{
		DB: base.DB,
		Services: WorkspaceServiceRPCServices{
			Workspace:     services.WorkspaceService,
			WorkspaceUser: services.WorkspaceUserService,
			User:          services.UserService,
			Env:           envService,
		},
		Readers: WorkspaceServiceRPCReaders{
			Workspace: services.WorkspaceService.Reader(),
			User:      services.WorkspaceUserService.Reader(),
		},
		Streamers: WorkspaceServiceRPCStreamers{
			Workspace:   stream,
			Environment: envStream,
		},
	})

	t.Cleanup(base.Close)

	return &workspaceFixture{
		ctx:     mwauth.CreateAuthedContext(context.Background(), userID),
		base:    base,
		handler: handler,
		ws:      services.WorkspaceService,
		wus:     services.WorkspaceUserService,
		es:      envService,
		us:      services.UserService,
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
	err := f.ws.Create(f.ctx, ws)
	require.NoError(t, err, "create workspace")

	env := menv.Env{
		ID:          envID,
		WorkspaceID: workspaceID,
		Name:        "default",
		Type:        menv.EnvGlobal,
	}
	err = f.es.CreateEnvironment(f.ctx, &env)
	require.NoError(t, err, "create environment")

	member := &mworkspace.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		UserID:      f.userID,
		Role:        mworkspace.RoleOwner,
	}
	err = f.wus.CreateWorkspaceUser(f.ctx, member)
	require.NoError(t, err, "create workspace user")

	return workspaceID
}

func collectWorkspaceSyncItems(t *testing.T, ch <-chan *apiv1.WorkspaceSyncResponse, count int) []*apiv1.WorkspaceSync {
	t.Helper()

	var items []*apiv1.WorkspaceSync
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
			require.FailNow(t, "timeout waiting for items", "timeout waiting for %d items, collected %d", count, len(items))
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
		require.NotNil(t, val, "snapshot item missing value union")
		require.Equal(t, apiv1.WorkspaceSync_ValueUnion_KIND_INSERT, val.GetKind())
		seen[string(val.GetInsert().GetWorkspaceId())] = true
	}
	require.True(t, seen[string(wsA.Bytes())], "snapshot missing wsA")
	require.True(t, seen[string(wsB.Bytes())], "snapshot missing wsB")

	newName := "renamed workspace"
	updateReq := connect.NewRequest(&apiv1.WorkspaceUpdateRequest{
		Items: []*apiv1.WorkspaceUpdate{
			{
				WorkspaceId: wsA.Bytes(),
				Name:        &newName,
			},
		},
	})
	_, err := f.handler.WorkspaceUpdate(f.ctx, updateReq)
	require.NoError(t, err, "WorkspaceUpdate")

	updateItems := collectWorkspaceSyncItems(t, msgCh, 1)
	updateVal := updateItems[0].GetValue()
	require.NotNil(t, updateVal, "update response missing value union")
	require.Equal(t, apiv1.WorkspaceSync_ValueUnion_KIND_UPDATE, updateVal.GetKind())
	require.Equal(t, newName, updateVal.GetUpdate().GetName())

	deleteReq := connect.NewRequest(&apiv1.WorkspaceDeleteRequest{
		Items: []*apiv1.WorkspaceDelete{
			{
				WorkspaceId: wsB.Bytes(),
			},
		},
	})
	_, err = f.handler.WorkspaceDelete(f.ctx, deleteReq)
	require.NoError(t, err, "WorkspaceDelete")

	deleteItems := collectWorkspaceSyncItems(t, msgCh, 1)
	deleteVal := deleteItems[0].GetValue()
	require.NotNil(t, deleteVal, "delete response missing value union")
	require.Equal(t, apiv1.WorkspaceSync_ValueUnion_KIND_DELETE, deleteVal.GetKind())
	require.Equal(t, string(wsB.Bytes()), string(deleteVal.GetDelete().GetWorkspaceId()))

	cancel()
	err = <-errCh
	if err != nil {
		require.ErrorIs(t, err, context.Canceled)
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
	err := f.us.CreateUser(context.Background(), &muser.User{
		ID:           otherUserID,
		Email:        fmt.Sprintf("%s@example.com", otherUserID.String()),
		Password:     []byte("password"),
		ProviderID:   &providerID,
		ProviderType: muser.MagicLink,
		Status:       muser.Active,
	})
	require.NoError(t, err, "create other user")

	otherWorkspaceID := idwrap.NewNow()
	otherEnvID := idwrap.NewNow()

	ws := &mworkspace.Workspace{
		ID:        otherWorkspaceID,
		Name:      "hidden",
		Updated:   dbtime.DBNow(),
		ActiveEnv: otherEnvID,
		GlobalEnv: otherEnvID,
	}
	err = f.ws.Create(context.Background(), ws)
	require.NoError(t, err, "create other workspace")

	env := menv.Env{
		ID:          otherEnvID,
		WorkspaceID: otherWorkspaceID,
		Name:        "default",
		Type:        menv.EnvGlobal,
	}
	err = f.es.CreateEnvironment(context.Background(), &env)
	require.NoError(t, err, "create other env")

	otherMember := &mworkspace.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: otherWorkspaceID,
		UserID:      otherUserID,
		Role:        mworkspace.RoleOwner,
	}
	err = f.wus.CreateWorkspaceUser(context.Background(), otherMember)
	require.NoError(t, err, "create other workspace user")

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
		require.FailNow(t, "unexpected event for unauthorized workspace", "%+v", resp)
	case <-time.After(150 * time.Millisecond):
	}

	cancel()
	err = <-errCh
	if err != nil {
		require.ErrorIs(t, err, context.Canceled)
	}
}


func TestWorkspaceInsertPublishesEnvironmentEvent(t *testing.T) {
	t.Parallel()

	f := newWorkspaceFixture(t)

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	// Subscribe to Environment events
	msgCh := make(chan *apiv1.WorkspaceSyncResponse, 5) // Use a dummy for workspace sync
	envCh := make(chan renv.EnvironmentEvent, 10)

	// Subscribe to the environment stream directly
	events, err := f.handler.envStream.Subscribe(ctx, func(topic renv.EnvironmentTopic) bool {
		return true // Accept all for test
	})
	require.NoError(t, err)

	go func() {
		for {
			select {
			case evt, ok := <-events:
				if !ok {
					return
				}
				envCh <- evt.Payload
			case <-ctx.Done():
				return
			}
		}
	}()

	createReq := connect.NewRequest(&apiv1.WorkspaceInsertRequest{
		Items: []*apiv1.WorkspaceInsert{
			{
				Name: "sync-test-workspace",
			},
		},
	})
	_, err = f.handler.WorkspaceInsert(f.ctx, createReq)
	require.NoError(t, err, "WorkspaceInsert")

	// Verify Environment Event
	select {
	case evt := <-envCh:
		require.Equal(t, "insert", evt.Type)
		require.NotNil(t, evt.Environment)
		require.Equal(t, "default", evt.Environment.Name)
		require.True(t, evt.Environment.IsGlobal)
	case <-time.After(2 * time.Second):
		require.FailNow(t, "timeout waiting for environment sync event")
	}

	_ = msgCh
	cancel()
}
