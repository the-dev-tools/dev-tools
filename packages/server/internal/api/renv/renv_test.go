package renv

import (
	"context"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/converter"
	"the-dev-tools/server/pkg/eventstream/memory"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/muser"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/testutil"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/environment/v1"
)

type envFixture struct {
	ctx         context.Context
	base        *testutil.BaseDBQueries
	handler     EnvRPC
	envService  senv.EnvService
	varService  senv.VariableService
	workspaceID idwrap.IDWrap
	userID      idwrap.IDWrap
}

func newEnvFixture(t *testing.T) *envFixture {
	t.Helper()

	base := testutil.CreateBaseDB(context.Background(), t)
	services := base.GetBaseServices()
	envService := senv.NewEnvironmentService(base.Queries, base.Logger())
	varService := senv.NewVariableService(base.Queries, base.Logger())
	envStream := memory.NewInMemorySyncStreamer[EnvironmentTopic, EnvironmentEvent]()
	varStream := memory.NewInMemorySyncStreamer[EnvironmentVariableTopic, EnvironmentVariableEvent]()
	t.Cleanup(envStream.Shutdown)
	t.Cleanup(varStream.Shutdown)

	workspaceID := idwrap.NewNow()
	userID := idwrap.NewNow()
	now := time.Now()

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

	err = services.WorkspaceService.Create(context.Background(), &mworkspace.Workspace{
		ID:      workspaceID,
		Name:    "Test Workspace",
		Updated: now,
	})
	require.NoError(t, err, "create workspace")

	err = services.WorkspaceUserService.CreateWorkspaceUser(context.Background(), &mworkspace.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        mworkspace.RoleOwner,
	})
	require.NoError(t, err, "create workspace user")

	authCtx := mwauth.CreateAuthedContext(context.Background(), userID)
	handler := New(EnvRPCDeps{
		DB: base.DB,
		Services: EnvRPCServices{
			Env:       envService,
			Variable:  varService,
			User:      services.UserService,
			Workspace: services.WorkspaceService,
		},
		Readers: EnvRPCReaders{
			Env:      envService.Reader(),
			Variable: varService.Reader(),
		},
		Streamers: EnvRPCStreamers{
			Env:      envStream,
			Variable: varStream,
		},
	})

	t.Cleanup(base.Close)

	return &envFixture{
		ctx:         authCtx,
		base:        base,
		handler:     handler,
		envService:  envService,
		varService:  varService,
		workspaceID: workspaceID,
		userID:      userID,
	}
}

func floatAlmostEqual(a, b float64) bool {
	const tol = 1e-6
	return math.Abs(a-b) < tol
}

func (f *envFixture) createEnv(t *testing.T, order float64) menv.Env {
	t.Helper()
	env := menv.Env{
		ID:          idwrap.NewNow(),
		WorkspaceID: f.workspaceID,
		Name:        fmt.Sprintf("env-%f", order),
		Description: "seeded env",
		Order:       order,
	}
	err := f.envService.CreateEnvironment(f.ctx, &env)
	require.NoError(t, err, "create env")
	return env
}

func (f *envFixture) createVar(t *testing.T, envID idwrap.IDWrap, order float64) idwrap.IDWrap {
	t.Helper()
	varID := idwrap.NewNow()
	err := f.varService.Create(f.ctx, menv.Variable{
		ID:          varID,
		EnvID:       envID,
		VarKey:      fmt.Sprintf("key-%f", order),
		Value:       "value",
		Enabled:     true,
		Description: "seeded var",
		Order:       order,
	})
	require.NoError(t, err, "create var")
	return varID
}

func TestEnvironmentCollectionOrdersResults(t *testing.T) {
	t.Parallel()

	f := newEnvFixture(t)
	envFirst := f.createEnv(t, 1)
	envSecond := f.createEnv(t, 2)

	resp, err := f.handler.EnvironmentCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err, "EnvironmentCollection")

	require.Len(t, resp.Msg.Items, 2)
	require.NotNil(t, resp.Msg.Items[0].GetEnvironmentId(), "environment ids should be populated")
	require.NotNil(t, resp.Msg.Items[1].GetEnvironmentId(), "environment ids should be populated")
	require.Equal(t, envFirst.Name, resp.Msg.Items[0].GetName())
	require.Equal(t, envSecond.Name, resp.Msg.Items[1].GetName())
}

func TestEnvironmentCreate(t *testing.T) {
	t.Parallel()

	f := newEnvFixture(t)
	envID := idwrap.NewNow()
	req := connect.NewRequest(&apiv1.EnvironmentInsertRequest{
		Items: []*apiv1.EnvironmentInsert{
			{
				EnvironmentId: envID.Bytes(),
				WorkspaceId:   f.workspaceID.Bytes(),
				Name:          "created env",
				Description:   "created via rpc",
				Order:         3,
			},
		},
	})

	_, err := f.handler.EnvironmentInsert(f.ctx, req)
	require.NoError(t, err, "EnvironmentInsert")

	stored, err := f.envService.GetEnvironment(f.ctx, envID)
	require.NoError(t, err, "GetEnvironment")
	require.Equal(t, "created env", stored.Name)
	require.Equal(t, "created via rpc", stored.Description)
	require.True(t, floatAlmostEqual(stored.Order, 3), "expected order 3, got %f", stored.Order)
}

func TestEnvironmentUpdate(t *testing.T) {
	t.Parallel()

	f := newEnvFixture(t)
	env := f.createEnv(t, 1)

	newName := "updated name"
	newDesc := "updated description"
	newOrder := float32(4)
	req := connect.NewRequest(&apiv1.EnvironmentUpdateRequest{
		Items: []*apiv1.EnvironmentUpdate{
			{
				EnvironmentId: env.ID.Bytes(),
				Name:          &newName,
				Description:   &newDesc,
				Order:         &newOrder,
			},
		},
	})

	_, err := f.handler.EnvironmentUpdate(f.ctx, req)
	require.NoError(t, err, "EnvironmentUpdate")

	stored, err := f.envService.GetEnvironment(f.ctx, env.ID)
	require.NoError(t, err, "GetEnvironment")
	require.Equal(t, newName, stored.Name)
	require.Equal(t, newDesc, stored.Description)
	require.True(t, floatAlmostEqual(stored.Order, float64(newOrder)), "expected order %.1f, got %f", newOrder, stored.Order)
}

func TestEnvironmentDelete(t *testing.T) {
	t.Parallel()

	f := newEnvFixture(t)
	env := f.createEnv(t, 1)

	req := connect.NewRequest(&apiv1.EnvironmentDeleteRequest{
		Items: []*apiv1.EnvironmentDelete{{EnvironmentId: env.ID.Bytes()}},
	})

	_, err := f.handler.EnvironmentDelete(f.ctx, req)
	require.NoError(t, err, "EnvironmentDelete")

	_, err = f.envService.GetEnvironment(f.ctx, env.ID)
	require.ErrorIs(t, err, senv.ErrNoEnvironmentFound)
}

func TestEnvironmentVariableCollection(t *testing.T) {
	t.Parallel()

	f := newEnvFixture(t)
	env := f.createEnv(t, 1)
	f.createVar(t, env.ID, 1)
	f.createVar(t, env.ID, 2)

	resp, err := f.handler.EnvironmentVariableCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err, "EnvironmentVariableCollection")
	require.Len(t, resp.Msg.Items, 2)
}

func TestEnvironmentVariableCreate(t *testing.T) {
	t.Parallel()

	f := newEnvFixture(t)
	env := f.createEnv(t, 1)
	varID := idwrap.NewNow()
	req := connect.NewRequest(&apiv1.EnvironmentVariableInsertRequest{
		Items: []*apiv1.EnvironmentVariableInsert{
			{
				EnvironmentVariableId: varID.Bytes(),
				EnvironmentId:         env.ID.Bytes(),
				Key:                   "API_KEY",
				Enabled:               true,
				Value:                 "secret",
				Description:           "primary key",
				Order:                 2,
			},
		},
	})

	_, err := f.handler.EnvironmentVariableInsert(f.ctx, req)
	require.NoError(t, err, "EnvironmentVariableInsert")

	stored, err := f.varService.Get(f.ctx, varID)
	require.NoError(t, err, "Get variable")
	require.Equal(t, "API_KEY", stored.VarKey)
	require.Equal(t, "secret", stored.Value)
	require.Equal(t, "primary key", stored.Description)
	require.True(t, floatAlmostEqual(stored.Order, 2), "expected order 2, got %f", stored.Order)
}

func TestEnvironmentVariableUpdate(t *testing.T) {
	t.Parallel()

	f := newEnvFixture(t)
	env := f.createEnv(t, 1)
	varID := f.createVar(t, env.ID, 1)

	newKey := "AUTH_TOKEN"
	newValue := "new"
	newDesc := "updated"
	newEnabled := false
	newOrder := float32(5)

	req := connect.NewRequest(&apiv1.EnvironmentVariableUpdateRequest{
		Items: []*apiv1.EnvironmentVariableUpdate{
			{
				EnvironmentVariableId: varID.Bytes(),
				Key:                   &newKey,
				Value:                 &newValue,
				Description:           &newDesc,
				Enabled:               &newEnabled,
				Order:                 &newOrder,
			},
		},
	})

	_, err := f.handler.EnvironmentVariableUpdate(f.ctx, req)
	require.NoError(t, err, "EnvironmentVariableUpdate")

	stored, err := f.varService.Get(f.ctx, varID)
	require.NoError(t, err, "Get variable")
	require.Equal(t, newKey, stored.VarKey)
	require.Equal(t, newValue, stored.Value)
	require.Equal(t, newDesc, stored.Description)
	require.Equal(t, newEnabled, stored.Enabled)
	require.True(t, floatAlmostEqual(stored.Order, float64(newOrder)), "expected order %.1f, got %f", newOrder, stored.Order)
}

func TestEnvironmentVariableDelete(t *testing.T) {
	t.Parallel()

	f := newEnvFixture(t)
	env := f.createEnv(t, 1)
	varID := f.createVar(t, env.ID, 1)

	req := connect.NewRequest(&apiv1.EnvironmentVariableDeleteRequest{
		Items: []*apiv1.EnvironmentVariableDelete{{EnvironmentVariableId: varID.Bytes()}},
	})

	_, err := f.handler.EnvironmentVariableDelete(f.ctx, req)
	require.NoError(t, err, "EnvironmentVariableDelete")

	_, err = f.varService.Get(f.ctx, varID)
	require.ErrorIs(t, err, senv.ErrNoVarFound)
}

func TestEnvironmentSyncStreamsUpdates(t *testing.T) {
	t.Parallel()

	f := newEnvFixture(t)
	envA := f.createEnv(t, 1)

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *apiv1.EnvironmentSyncResponse, 10)
	errCh := make(chan error, 1)

	go func() {
		err := f.handler.streamEnvironmentSync(ctx, f.userID, func(resp *apiv1.EnvironmentSyncResponse) error {
			msgCh <- resp
			return nil
		})
		errCh <- err
		close(msgCh)
	}()

	// Verify NO snapshot arrives (snapshots removed in favor of *Collection RPCs)
	select {
	case <-msgCh:
		require.FailNow(t, "received unexpected snapshot item")
	case <-time.After(100 * time.Millisecond):
		// Good - stream active, no snapshot sent
	}

	// Test live UPDATE event
	newName := "updated env"
	req := connect.NewRequest(&apiv1.EnvironmentUpdateRequest{
		Items: []*apiv1.EnvironmentUpdate{
			{
				EnvironmentId: envA.ID.Bytes(),
				Name:          &newName,
			},
		},
	})
	_, err := f.handler.EnvironmentUpdate(f.ctx, req)
	require.NoError(t, err, "EnvironmentUpdate")

	updateItems := collectEnvironmentSyncItems(t, msgCh, 1)
	updateVal := updateItems[0].GetValue()
	require.NotNil(t, updateVal, "update response missing value union")
	require.Equal(t, apiv1.EnvironmentSync_ValueUnion_KIND_UPDATE, updateVal.GetKind())
	require.Equal(t, newName, updateVal.GetUpdate().GetName())

	cancel()
	err = <-errCh
	if err != nil {
		require.ErrorIs(t, err, context.Canceled)
	}
}

func TestEnvironmentVariableSyncStreamsUpdates(t *testing.T) {
	t.Parallel()

	f := newEnvFixture(t)
	env := f.createEnv(t, 1)
	varA := f.createVar(t, env.ID, 1)

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *apiv1.EnvironmentVariableSyncResponse, 10)
	errCh := make(chan error, 1)

	go func() {
		err := f.handler.streamEnvironmentVariableSync(ctx, f.userID, func(resp *apiv1.EnvironmentVariableSyncResponse) error {
			msgCh <- resp
			return nil
		})
		errCh <- err
		close(msgCh)
	}()

	// Verify NO snapshot arrives (snapshots removed in favor of *Collection RPCs)
	select {
	case <-msgCh:
		require.FailNow(t, "received unexpected snapshot item")
	case <-time.After(100 * time.Millisecond):
		// Good - stream active, no snapshot sent
	}

	// Test live UPDATE event
	newValue := "changed"
	req := connect.NewRequest(&apiv1.EnvironmentVariableUpdateRequest{
		Items: []*apiv1.EnvironmentVariableUpdate{
			{
				EnvironmentVariableId: varA.Bytes(),
				Value:                 &newValue,
			},
		},
	})
	_, err := f.handler.EnvironmentVariableUpdate(f.ctx, req)
	require.NoError(t, err, "EnvironmentVariableUpdate")

	updateItems := collectEnvironmentVariableSyncItems(t, msgCh, 1)
	updateVal := updateItems[0].GetValue()
	require.NotNil(t, updateVal, "update response missing value union")
	require.Equal(t, apiv1.EnvironmentVariableSync_ValueUnion_KIND_UPDATE, updateVal.GetKind())
	require.Equal(t, newValue, updateVal.GetUpdate().GetValue())

	cancel()
	err = <-errCh
	if err != nil {
		require.ErrorIs(t, err, context.Canceled)
	}
}

func TestEnvironmentSyncFiltersUnauthorizedWorkspaces(t *testing.T) {
	t.Parallel()

	f := newEnvFixture(t)

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *apiv1.EnvironmentSyncResponse, 5)
	errCh := make(chan error, 1)

	go func() {
		err := f.handler.streamEnvironmentSync(ctx, f.userID, func(resp *apiv1.EnvironmentSyncResponse) error {
			msgCh <- resp
			return nil
		})
		errCh <- err
		close(msgCh)
	}()

	// Verify NO snapshot arrives (snapshots removed in favor of *Collection RPCs)
	select {
	case <-msgCh:
		require.FailNow(t, "received unexpected snapshot item")
	case <-time.After(100 * time.Millisecond):
		// Good - stream active, no snapshot sent
	}

	// Create an unauthorized workspace (user is not a member)
	otherWorkspaceID := idwrap.NewNow()
	services := f.base.GetBaseServices()
	err := services.WorkspaceService.Create(context.Background(), &mworkspace.Workspace{
		ID:      otherWorkspaceID,
		Name:    "other",
		Updated: time.Now(),
	})
	require.NoError(t, err, "create workspace")

	otherEnv := menv.Env{
		ID:          idwrap.NewNow(),
		WorkspaceID: otherWorkspaceID,
		Name:        "alien",
		Description: "hidden",
		Order:       42,
	}

	// Publish event for unauthorized workspace - should be filtered
	f.handler.envStream.Publish(EnvironmentTopic{WorkspaceID: otherWorkspaceID}, EnvironmentEvent{
		Type:        "insert",
		Environment: converter.ToAPIEnvironment(otherEnv),
	})

	select {
	case resp := <-msgCh:
		require.FailNow(t, "unexpected event for unauthorized workspace", "%+v", resp)
	case <-time.After(150 * time.Millisecond):
		// success: no events delivered for unauthorized workspace
	}

	cancel()
	err = <-errCh
	if err != nil {
		require.ErrorIs(t, err, context.Canceled)
	}
}

func collectEnvironmentSyncItems(t *testing.T, ch <-chan *apiv1.EnvironmentSyncResponse, count int) []*apiv1.EnvironmentSync {
	t.Helper()

	var items []*apiv1.EnvironmentSync
	timeout := time.After(2 * time.Second)

	for len(items) < count {
		select {
		case resp, ok := <-ch:
			require.True(t, ok, "channel closed before collecting %d items", count)
			for _, item := range resp.GetItems() {
				if item != nil {
					items = append(items, item)
				}
				if len(items) == count {
					break
				}
			}
		case <-timeout:
			require.FailNow(t, "timeout waiting for items", "timeout waiting for %d items, collected %d", count, len(items))
		}
	}

	return items
}

func collectEnvironmentVariableSyncItems(t *testing.T, ch <-chan *apiv1.EnvironmentVariableSyncResponse, count int) []*apiv1.EnvironmentVariableSync {
	t.Helper()

	var items []*apiv1.EnvironmentVariableSync
	timeout := time.After(2 * time.Second)

	for len(items) < count {
		select {
		case resp, ok := <-ch:
			require.True(t, ok, "channel closed before collecting %d items", count)
			for _, item := range resp.GetItems() {
				if item != nil {
					items = append(items, item)
				}
				if len(items) == count {
					break
				}
			}
		case <-timeout:
			require.FailNow(t, "timeout waiting for items", "timeout waiting for %d items, collected %d", count, len(items))
		}
	}

	return items
}

func TestEnvironmentVariableDeleteConcurrent(t *testing.T) {
	t.Parallel()

	f := newEnvFixture(t)
	env := f.createEnv(t, 1)

	var varIDs []idwrap.IDWrap
	for i := 0; i < 10; i++ {
		varIDs = append(varIDs, f.createVar(t, env.ID, float64(i)))
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(varIDs))

	for _, varID := range varIDs {
		wg.Add(1)
		go func(id idwrap.IDWrap) {
			defer wg.Done()
			req := connect.NewRequest(&apiv1.EnvironmentVariableDeleteRequest{
				Items: []*apiv1.EnvironmentVariableDelete{{EnvironmentVariableId: id.Bytes()}},
			})
			_, err := f.handler.EnvironmentVariableDelete(f.ctx, req)
			if err != nil {
				errCh <- err
			}
		}(varID)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
		close(errCh)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("deadlock: concurrent deletes timed out")
	}

	for err := range errCh {
		require.NoError(t, err, "concurrent delete failed")
	}
}

func TestEnvironmentVariableUpdateConcurrent(t *testing.T) {
	t.Parallel()

	f := newEnvFixture(t)
	env := f.createEnv(t, 1)

	var varIDs []idwrap.IDWrap
	for i := 0; i < 10; i++ {
		varIDs = append(varIDs, f.createVar(t, env.ID, float64(i)))
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(varIDs))

	for i, varID := range varIDs {
		wg.Add(1)
		go func(id idwrap.IDWrap, idx int) {
			defer wg.Done()
			newValue := fmt.Sprintf("updated-%d", idx)
			req := connect.NewRequest(&apiv1.EnvironmentVariableUpdateRequest{
				Items: []*apiv1.EnvironmentVariableUpdate{{
					EnvironmentVariableId: id.Bytes(),
					Value:                 &newValue,
				}},
			})
			_, err := f.handler.EnvironmentVariableUpdate(f.ctx, req)
			if err != nil {
				errCh <- err
			}
		}(varID, i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
		close(errCh)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("deadlock: concurrent updates timed out")
	}

	for err := range errCh {
		require.NoError(t, err, "concurrent update failed")
	}
}
