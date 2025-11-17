package renv

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/eventstream/memory"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/muser"
	"the-dev-tools/server/pkg/model/mvar"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/model/mworkspaceuser"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/testutil"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/environment/v1"
)

type envFixture struct {
	ctx         context.Context
	base        *testutil.BaseDBQueries
	handler     EnvRPC
	envService  senv.EnvService
	varService  svar.VarService
	workspaceID idwrap.IDWrap
	userID      idwrap.IDWrap
}

func newEnvFixture(t *testing.T) *envFixture {
	t.Helper()

	base := testutil.CreateBaseDB(context.Background(), t)
	services := base.GetBaseServices()
	envService := senv.New(base.Queries, base.Logger())
	varService := svar.New(base.Queries, base.Logger())
	envStream := memory.NewInMemorySyncStreamer[EnvironmentTopic, EnvironmentEvent]()
	varStream := memory.NewInMemorySyncStreamer[EnvironmentVariableTopic, EnvironmentVariableEvent]()
	t.Cleanup(envStream.Shutdown)
	t.Cleanup(varStream.Shutdown)

	workspaceID := idwrap.NewNow()
	userID := idwrap.NewNow()
	now := time.Now()

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

	if err := services.Ws.Create(context.Background(), &mworkspace.Workspace{
		ID:      workspaceID,
		Name:    "Test Workspace",
		Updated: now,
	}); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	if err := services.Wus.CreateWorkspaceUser(context.Background(), &mworkspaceuser.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        mworkspaceuser.RoleOwner,
	}); err != nil {
		t.Fatalf("create workspace user: %v", err)
	}

	authCtx := mwauth.CreateAuthedContext(context.Background(), userID)
	handler := New(base.DB, envService, varService, services.Us, services.Ws, envStream, varStream)

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
	if err := f.envService.CreateEnvironment(f.ctx, &env); err != nil {
		t.Fatalf("create env: %v", err)
	}
	return env
}

func (f *envFixture) createVar(t *testing.T, envID idwrap.IDWrap, order float64) idwrap.IDWrap {
	t.Helper()
	varID := idwrap.NewNow()
	if err := f.varService.Create(f.ctx, mvar.Var{
		ID:          varID,
		EnvID:       envID,
		VarKey:      fmt.Sprintf("key-%f", order),
		Value:       "value",
		Enabled:     true,
		Description: "seeded var",
		Order:       order,
	}); err != nil {
		t.Fatalf("create var: %v", err)
	}
	return varID
}

func TestEnvironmentCollectionOrdersResults(t *testing.T) {
	t.Parallel()

	f := newEnvFixture(t)
	envFirst := f.createEnv(t, 1)
	envSecond := f.createEnv(t, 2)

	resp, err := f.handler.EnvironmentCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	if err != nil {
		t.Fatalf("EnvironmentCollection err: %v", err)
	}

	if len(resp.Msg.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(resp.Msg.Items))
	}
	if resp.Msg.Items[0].GetEnvironmentId() == nil || resp.Msg.Items[1].GetEnvironmentId() == nil {
		t.Fatal("environment ids should be populated")
	}
	if resp.Msg.Items[0].GetName() != envFirst.Name {
		t.Fatalf("expected first environment %q, got %q", envFirst.Name, resp.Msg.Items[0].GetName())
	}
	if resp.Msg.Items[1].GetName() != envSecond.Name {
		t.Fatalf("expected second environment %q, got %q", envSecond.Name, resp.Msg.Items[1].GetName())
	}
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

	if _, err := f.handler.EnvironmentInsert(f.ctx, req); err != nil {
		t.Fatalf("EnvironmentInsert err: %v", err)
	}

	stored, err := f.envService.GetEnvironment(f.ctx, envID)
	if err != nil {
		t.Fatalf("GetEnvironment err: %v", err)
	}
	if stored.Name != "created env" || stored.Description != "created via rpc" {
		t.Fatalf("unexpected environment fields: %+v", stored)
	}
	if !floatAlmostEqual(stored.Order, 3) {
		t.Fatalf("expected order 3, got %f", stored.Order)
	}
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

	if _, err := f.handler.EnvironmentUpdate(f.ctx, req); err != nil {
		t.Fatalf("EnvironmentUpdate err: %v", err)
	}

	stored, err := f.envService.GetEnvironment(f.ctx, env.ID)
	if err != nil {
		t.Fatalf("GetEnvironment err: %v", err)
	}
	if stored.Name != newName || stored.Description != newDesc {
		t.Fatalf("unexpected fields: %+v", stored)
	}
	if !floatAlmostEqual(stored.Order, float64(newOrder)) {
		t.Fatalf("expected order %.1f, got %f", newOrder, stored.Order)
	}
}

func TestEnvironmentDelete(t *testing.T) {
	t.Parallel()

	f := newEnvFixture(t)
	env := f.createEnv(t, 1)

	req := connect.NewRequest(&apiv1.EnvironmentDeleteRequest{
		Items: []*apiv1.EnvironmentDelete{{EnvironmentId: env.ID.Bytes()}},
	})

	if _, err := f.handler.EnvironmentDelete(f.ctx, req); err != nil {
		t.Fatalf("EnvironmentDelete err: %v", err)
	}

	if _, err := f.envService.GetEnvironment(f.ctx, env.ID); !errors.Is(err, senv.ErrNoEnvironmentFound) {
		t.Fatalf("expected ErrNoEnvironmentFound, got %v", err)
	}
}

func TestEnvironmentVariableCollection(t *testing.T) {
	t.Parallel()

	f := newEnvFixture(t)
	env := f.createEnv(t, 1)
	f.createVar(t, env.ID, 1)
	f.createVar(t, env.ID, 2)

	resp, err := f.handler.EnvironmentVariableCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	if err != nil {
		t.Fatalf("EnvironmentVariableCollection err: %v", err)
	}
	if len(resp.Msg.Items) != 2 {
		t.Fatalf("expected 2 variables, got %d", len(resp.Msg.Items))
	}
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

	if _, err := f.handler.EnvironmentVariableInsert(f.ctx, req); err != nil {
		t.Fatalf("EnvironmentVariableInsert err: %v", err)
	}

	stored, err := f.varService.Get(f.ctx, varID)
	if err != nil {
		t.Fatalf("Get variable err: %v", err)
	}
	if stored.VarKey != "API_KEY" || stored.Value != "secret" || stored.Description != "primary key" {
		t.Fatalf("unexpected stored variable: %+v", stored)
	}
	if !floatAlmostEqual(stored.Order, 2) {
		t.Fatalf("expected order 2, got %f", stored.Order)
	}
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

	if _, err := f.handler.EnvironmentVariableUpdate(f.ctx, req); err != nil {
		t.Fatalf("EnvironmentVariableUpdate err: %v", err)
	}

	stored, err := f.varService.Get(f.ctx, varID)
	if err != nil {
		t.Fatalf("Get variable err: %v", err)
	}
	if stored.VarKey != newKey || stored.Value != newValue || stored.Description != newDesc || stored.Enabled != newEnabled {
		t.Fatalf("unexpected stored variable: %+v", stored)
	}
	if !floatAlmostEqual(stored.Order, float64(newOrder)) {
		t.Fatalf("expected order %.1f, got %f", newOrder, stored.Order)
	}
}

func TestEnvironmentVariableDelete(t *testing.T) {
	t.Parallel()

	f := newEnvFixture(t)
	env := f.createEnv(t, 1)
	varID := f.createVar(t, env.ID, 1)

	req := connect.NewRequest(&apiv1.EnvironmentVariableDeleteRequest{
		Items: []*apiv1.EnvironmentVariableDelete{{EnvironmentVariableId: varID.Bytes()}},
	})

	if _, err := f.handler.EnvironmentVariableDelete(f.ctx, req); err != nil {
		t.Fatalf("EnvironmentVariableDelete err: %v", err)
	}

	if _, err := f.varService.Get(f.ctx, varID); !errors.Is(err, svar.ErrNoVarFound) {
		t.Fatalf("expected ErrNoVarFound, got %v", err)
	}
}

func TestEnvironmentSyncStreamsSnapshotAndUpdates(t *testing.T) {
	t.Parallel()

	f := newEnvFixture(t)
	envA := f.createEnv(t, 1)
	envB := f.createEnv(t, 2)

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

	snapshot := collectEnvironmentSyncItems(t, msgCh, 2)
	seen := map[string]bool{}
	for _, item := range snapshot {
		val := item.GetValue()
		if val == nil {
			t.Fatal("snapshot item missing value union")
		}
		if val.GetKind() != apiv1.EnvironmentSync_ValueUnion_KIND_INSERT {
			t.Fatalf("expected insert kind for snapshot, got %v", val.GetKind())
		}
		envID := string(val.GetInsert().GetEnvironmentId())
		seen[envID] = true
	}
	if !seen[string(envA.ID.Bytes())] || !seen[string(envB.ID.Bytes())] {
		t.Fatalf("snapshot missing expected environments, seen=%v", seen)
	}

	newName := "updated env"
	req := connect.NewRequest(&apiv1.EnvironmentUpdateRequest{
		Items: []*apiv1.EnvironmentUpdate{
			{
				EnvironmentId: envA.ID.Bytes(),
				Name:          &newName,
			},
		},
	})
	if _, err := f.handler.EnvironmentUpdate(f.ctx, req); err != nil {
		t.Fatalf("EnvironmentUpdate err: %v", err)
	}

	updateItems := collectEnvironmentSyncItems(t, msgCh, 1)
	updateVal := updateItems[0].GetValue()
	if updateVal == nil {
		t.Fatal("update response missing value union")
	}
	if updateVal.GetKind() != apiv1.EnvironmentSync_ValueUnion_KIND_UPDATE {
		t.Fatalf("expected update kind, got %v", updateVal.GetKind())
	}
	if got := updateVal.GetUpdate().GetName(); got != newName {
		t.Fatalf("expected updated name %q, got %q", newName, got)
	}

	cancel()
	if err := <-errCh; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("stream returned error: %v", err)
	}
}

func TestEnvironmentVariableSyncStreamsSnapshotAndUpdates(t *testing.T) {
	t.Parallel()

	f := newEnvFixture(t)
	env := f.createEnv(t, 1)
	varA := f.createVar(t, env.ID, 1)
	f.createVar(t, env.ID, 2)

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

	snapshot := collectEnvironmentVariableSyncItems(t, msgCh, 2)
	varSeen := map[string]bool{}
	for _, item := range snapshot {
		val := item.GetValue()
		if val == nil {
			t.Fatal("snapshot variable missing value union")
		}
		if val.GetKind() != apiv1.EnvironmentVariableSync_ValueUnion_KIND_INSERT {
			t.Fatalf("expected insert kind for snapshot, got %v", val.GetKind())
		}
		varSeen[string(val.GetInsert().GetEnvironmentVariableId())] = true
	}
	if len(varSeen) != 2 {
		t.Fatalf("expected snapshot to contain 2 variables, got %d", len(varSeen))
	}

	newValue := "changed"
	req := connect.NewRequest(&apiv1.EnvironmentVariableUpdateRequest{
		Items: []*apiv1.EnvironmentVariableUpdate{
			{
				EnvironmentVariableId: varA.Bytes(),
				Value:                 &newValue,
			},
		},
	})
	if _, err := f.handler.EnvironmentVariableUpdate(f.ctx, req); err != nil {
		t.Fatalf("EnvironmentVariableUpdate err: %v", err)
	}

	updateItems := collectEnvironmentVariableSyncItems(t, msgCh, 1)
	updateVal := updateItems[0].GetValue()
	if updateVal == nil {
		t.Fatal("update response missing value union")
	}
	if updateVal.GetKind() != apiv1.EnvironmentVariableSync_ValueUnion_KIND_UPDATE {
		t.Fatalf("expected update kind, got %v", updateVal.GetKind())
	}
	if got := updateVal.GetUpdate().GetValue(); got != newValue {
		t.Fatalf("expected updated value %q, got %q", newValue, got)
	}

	cancel()
	if err := <-errCh; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("stream returned error: %v", err)
	}
}

func TestEnvironmentSyncFiltersUnauthorizedWorkspaces(t *testing.T) {
	t.Parallel()

	f := newEnvFixture(t)
	f.createEnv(t, 1)

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

	_ = collectEnvironmentSyncItems(t, msgCh, 1)

	otherWorkspaceID := idwrap.NewNow()
	services := f.base.GetBaseServices()
	if err := services.Ws.Create(context.Background(), &mworkspace.Workspace{
		ID:      otherWorkspaceID,
		Name:    "other",
		Updated: time.Now(),
	}); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	otherEnv := menv.Env{
		ID:          idwrap.NewNow(),
		WorkspaceID: otherWorkspaceID,
		Name:        "alien",
		Description: "hidden",
		Order:       42,
	}

	f.handler.envStream.Publish(EnvironmentTopic{WorkspaceID: otherWorkspaceID}, EnvironmentEvent{
		Type:        "insert",
		Environment: toAPIEnvironment(otherEnv),
	})

	select {
	case resp := <-msgCh:
		t.Fatalf("unexpected event for unauthorized workspace: %+v", resp)
	case <-time.After(150 * time.Millisecond):
		// success: no events delivered
	}

	cancel()
	if err := <-errCh; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("stream returned error: %v", err)
	}

}

func collectEnvironmentSyncItems(t *testing.T, ch <-chan *apiv1.EnvironmentSyncResponse, count int) []*apiv1.EnvironmentSync {
	t.Helper()

	var items []*apiv1.EnvironmentSync
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
				}
				if len(items) == count {
					break
				}
			}
		case <-timeout:
			t.Fatalf("timeout waiting for %d items, collected %d", count, len(items))
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
			if !ok {
				t.Fatalf("channel closed before collecting %d items", count)
			}
			for _, item := range resp.GetItems() {
				if item != nil {
					items = append(items, item)
				}
				if len(items) == count {
					break
				}
			}
		case <-timeout:
			t.Fatalf("timeout waiting for %d items, collected %d", count, len(items))
		}
	}

	return items
}
