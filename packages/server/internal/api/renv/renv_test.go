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
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/muser"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/model/mworkspaceuser"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/testutil"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/environment/v1"
)

type envTestFixture struct {
	ctx         context.Context
	base        *testutil.BaseDBQueries
	handler     EnvRPC
	envService  senv.EnvService
	varService  svar.VarService
	workspaceID idwrap.IDWrap
	userID      idwrap.IDWrap
}

func newEnvTestFixture(t *testing.T) *envTestFixture {
	t.Helper()

	base := testutil.CreateBaseDB(context.Background(), t)
	services := base.GetBaseServices()
	envService := senv.New(base.Queries, base.Logger())
	varService := svar.New(base.Queries, base.Logger())

	workspaceID := idwrap.NewNow()
	userID := idwrap.NewNow()

	now := time.Now()
	providerID := fmt.Sprintf("test-%s", userID.String())

	user := muser.User{
		ID:           userID,
		Email:        fmt.Sprintf("%s@example.com", userID.String()),
		Password:     []byte("pass"),
		ProviderID:   &providerID,
		ProviderType: muser.MagicLink,
		Status:       muser.Active,
	}
	if err := services.Us.CreateUser(context.Background(), &user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	workspace := mworkspace.Workspace{
		ID:      workspaceID,
		Name:    "Test Workspace",
		Updated: now,
	}
	if err := services.Ws.Create(context.Background(), &workspace); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	membership := mworkspaceuser.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        mworkspaceuser.RoleOwner,
	}
	if err := services.Wus.CreateWorkspaceUser(context.Background(), &membership); err != nil {
		t.Fatalf("create workspace user: %v", err)
	}

	authCtx := mwauth.CreateAuthedContext(context.Background(), userID)
	handler := New(base.DB, envService, varService, services.Us, services.Ws)

	t.Cleanup(base.Close)

	return &envTestFixture{
		ctx:         authCtx,
		base:        base,
		handler:     handler,
		envService:  envService,
		varService:  varService,
		workspaceID: workspaceID,
		userID:      userID,
	}
}

func floatEquals(a, b float64) bool {
	const tol = 1e-6
	return math.Abs(a-b) < tol
}

func TestEnvironmentCollectionReturnsOrderedEnvironments(t *testing.T) {
	t.Parallel()

	f := newEnvTestFixture(t)

	// Seed environments with explicit ordering.
	envB := menv.Env{
		ID:          idwrap.NewNow(),
		WorkspaceID: f.workspaceID,
		Name:        "Second",
		Description: "should appear second",
		Order:       2,
	}
	if err := f.envService.CreateEnvironment(f.ctx, &envB); err != nil {
		t.Fatalf("seed envB: %v", err)
	}

	envA := menv.Env{
		ID:          idwrap.NewNow(),
		WorkspaceID: f.workspaceID,
		Name:        "First",
		Description: "should appear first",
		Order:       1,
	}
	if err := f.envService.CreateEnvironment(f.ctx, &envA); err != nil {
		t.Fatalf("seed envA: %v", err)
	}

	resp, err := f.handler.EnvironmentCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	if err != nil {
		t.Fatalf("EnvironmentCollection err: %v", err)
	}

	if got := len(resp.Msg.Items); got != 2 {
		t.Fatalf("expected 2 environments, got %d", got)
	}

	if resp.Msg.Items[0].GetName() != envA.Name {
		t.Fatalf("expected first item %q, got %q", envA.Name, resp.Msg.Items[0].GetName())
	}
	if resp.Msg.Items[1].GetName() != envB.Name {
		t.Fatalf("expected second item %q, got %q", envB.Name, resp.Msg.Items[1].GetName())
	}
	if resp.Msg.Items[0].GetWorkspaceId() == nil {
		t.Fatal("workspace id should be populated")
	}
}

func TestEnvironmentCreateUpdateDelete(t *testing.T) {
	t.Parallel()

	f := newEnvTestFixture(t)

	envID := idwrap.NewNow()

	createReq := &apiv1.EnvironmentCreateRequest{
		Items: []*apiv1.EnvironmentCreate{
			{
				EnvironmentId: envID.Bytes(),
				WorkspaceId:   f.workspaceID.Bytes(),
				Name:          "Initial",
				Description:   "initial description",
				Order:         5,
			},
		},
	}

	if _, err := f.handler.EnvironmentCreate(f.ctx, connect.NewRequest(createReq)); err != nil {
		t.Fatalf("EnvironmentCreate err: %v", err)
	}

	envs, err := f.envService.ListEnvironments(f.ctx, f.workspaceID)
	if err != nil {
		t.Fatalf("list environments: %v", err)
	}
	if len(envs) != 1 {
		t.Fatalf("expected 1 environment, got %d", len(envs))
	}
	if envs[0].Name != "Initial" || envs[0].Description != "initial description" {
		t.Fatalf("unexpected env fields: %+v", envs[0])
	}
	if !floatEquals(envs[0].Order, 5) {
		t.Fatalf("expected order 5, got %f", envs[0].Order)
	}

	newName := "Updated"
	newDesc := "updated description"
	newOrder := float32(7)
	updateReq := &apiv1.EnvironmentUpdateRequest{
		Items: []*apiv1.EnvironmentUpdate{
			{
				EnvironmentId: envID.Bytes(),
				Name:          &newName,
				Description:   &newDesc,
				Order:         &newOrder,
			},
		},
	}

	if _, err := f.handler.EnvironmentUpdate(f.ctx, connect.NewRequest(updateReq)); err != nil {
		t.Fatalf("EnvironmentUpdate err: %v", err)
	}

	updated, err := f.envService.GetEnvironment(f.ctx, envID)
	if err != nil {
		t.Fatalf("get environment: %v", err)
	}
	if updated.Name != newName || updated.Description != newDesc {
		t.Fatalf("update failed, got %+v", updated)
	}
	if !floatEquals(updated.Order, float64(newOrder)) {
		t.Fatalf("expected order %.1f, got %f", newOrder, updated.Order)
	}

	deleteReq := &apiv1.EnvironmentDeleteRequest{
		Items: []*apiv1.EnvironmentDelete{
			{
				EnvironmentId: envID.Bytes(),
			},
		},
	}
	if _, err := f.handler.EnvironmentDelete(f.ctx, connect.NewRequest(deleteReq)); err != nil {
		t.Fatalf("EnvironmentDelete err: %v", err)
	}

	envs, err = f.envService.ListEnvironments(f.ctx, f.workspaceID)
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(envs) != 0 {
		t.Fatalf("expected no environments, got %d", len(envs))
	}
}

func TestEnvironmentVariableCollectionAndCRUD(t *testing.T) {
	t.Parallel()

	f := newEnvTestFixture(t)

	env := menv.Env{
		ID:          idwrap.NewNow(),
		WorkspaceID: f.workspaceID,
		Name:        "Vars Env",
		Description: "for variable tests",
		Order:       1,
	}
	if err := f.envService.CreateEnvironment(f.ctx, &env); err != nil {
		t.Fatalf("create env: %v", err)
	}

	varID := idwrap.NewNow()
	createReq := &apiv1.EnvironmentVariableCreateRequest{
		Items: []*apiv1.EnvironmentVariableCreate{
			{
				EnvironmentVariableId: varID.Bytes(),
				EnvironmentId:         env.ID.Bytes(),
				Key:                   "API_KEY",
				Enabled:               true,
				Value:                 "secret",
				Description:           "primary key",
				Order:                 3,
			},
		},
	}
	if _, err := f.handler.EnvironmentVariableCreate(f.ctx, connect.NewRequest(createReq)); err != nil {
		t.Fatalf("EnvironmentVariableCreate err: %v", err)
	}

	variable, err := f.varService.Get(f.ctx, varID)
	if err != nil {
		t.Fatalf("get variable: %v", err)
	}
	if variable.VarKey != "API_KEY" || variable.Value != "secret" || variable.Description != "primary key" || !variable.Enabled {
		t.Fatalf("unexpected variable fields: %+v", variable)
	}
	if !floatEquals(variable.Order, 3) {
		t.Fatalf("expected order 3, got %f", variable.Order)
	}

	resp, err := f.handler.EnvironmentVariableCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	if err != nil {
		t.Fatalf("EnvironmentVariableCollection err: %v", err)
	}
	if len(resp.Msg.Items) != 1 || resp.Msg.Items[0].GetKey() != "API_KEY" {
		t.Fatalf("unexpected collection response: %+v", resp.Msg.Items)
	}

	newKey := "AUTH_TOKEN"
	newValue := "updated"
	newDesc := "updated description"
	newEnabled := false
	newOrder := float32(9)
	updateReq := &apiv1.EnvironmentVariableUpdateRequest{
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
	}
	if _, err := f.handler.EnvironmentVariableUpdate(f.ctx, connect.NewRequest(updateReq)); err != nil {
		t.Fatalf("EnvironmentVariableUpdate err: %v", err)
	}

	variable, err = f.varService.Get(f.ctx, varID)
	if err != nil {
		t.Fatalf("get variable after update: %v", err)
	}
	if variable.VarKey != newKey || variable.Value != newValue || variable.Description != newDesc || variable.Enabled != newEnabled {
		t.Fatalf("update failed, variable %+v", variable)
	}
	if !floatEquals(variable.Order, float64(newOrder)) {
		t.Fatalf("expected order %.1f, got %f", newOrder, variable.Order)
	}

	deleteReq := &apiv1.EnvironmentVariableDeleteRequest{
		Items: []*apiv1.EnvironmentVariableDelete{
			{EnvironmentVariableId: varID.Bytes()},
		},
	}
	if _, err := f.handler.EnvironmentVariableDelete(f.ctx, connect.NewRequest(deleteReq)); err != nil {
		t.Fatalf("EnvironmentVariableDelete err: %v", err)
	}

	if _, err := f.varService.Get(f.ctx, varID); !errors.Is(err, svar.ErrNoVarFound) {
		t.Fatalf("expected ErrNoVarFound after delete, got %v", err)
	}
}
