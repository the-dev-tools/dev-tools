package renv_test

import (
	"context"
	"testing"
	"the-dev-tools/backend/internal/api/middleware/mwauth"
	"the-dev-tools/backend/internal/api/renv"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/menv"
	"the-dev-tools/backend/pkg/service/senv"
	"the-dev-tools/backend/pkg/service/suser"
	"the-dev-tools/backend/pkg/service/svar"
	"the-dev-tools/backend/pkg/testutil"
	environmentv1 "the-dev-tools/spec/dist/buf/go/environment/v1"
	"time"

	"connectrpc.com/connect"
)

func TestCreateEnv(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	us := suser.New(queries)
	es := senv.New(queries)
	vs := svar.New(queries)

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	CollectionID := idwrap.NewNow()
	UserID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, UserID, CollectionID)

	envName := "test"
	EnvDesc := "test"

	req := connect.NewRequest(&environmentv1.EnvironmentCreateRequest{
		WorkspaceId: workspaceID.Bytes(),
		Name:        envName,
		Description: EnvDesc,
	})

	rpcEnv := renv.New(db, es, vs, us)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcEnv.EnvironmentCreate(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}

	if resp == nil {
		t.Fatal("resp is nil")
	}

	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}

	if resp.Msg.GetEnvironmentId() == nil {
		t.Fatal("resp.Msg.GetEnvironmentId() is nil")
	}

	envID, err := idwrap.NewFromBytes(resp.Msg.GetEnvironmentId())
	if err != nil {
		t.Error(err)
	}

	env, err := es.Get(ctx, envID)
	if err != nil {
		t.Fatal("cannot find created env", err)
	}

	if env.Name != envName {
		t.Error("created name is not same")
	}

	if env.Description != EnvDesc {
		t.Error("created description is not same")
	}

	if env.WorkspaceID != workspaceID {
		t.Error("created workspaceID is not same")
	}
}

func TestGetEnv(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	us := suser.New(queries)
	es := senv.New(queries)
	vs := svar.New(queries)

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	CollectionID := idwrap.NewNow()
	UserID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, UserID, CollectionID)

	envID := idwrap.NewNow()
	env := menv.Env{
		ID:          envID,
		WorkspaceID: workspaceID,
		Type:        menv.EnvGlobal,
		Description: "test",
		Name:        "Something",
		Updated:     time.Now(),
	}
	err := es.Create(ctx, env)
	if err != nil {
		t.Fatal(err)
	}

	req := connect.NewRequest(&environmentv1.EnvironmentGetRequest{
		EnvironmentId: envID.Bytes(),
	})

	rpcEnv := renv.New(db, es, vs, us)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcEnv.EnvironmentGet(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("resp is nil")
	}
	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}
	respEnvID, err := idwrap.NewFromBytes(resp.Msg.EnvironmentId)
	if err != nil {
		t.Fatal(err)
	}
	if envID.Compare(respEnvID) != 0 {
		t.Error("envID is not same")
	}

	if resp.Msg.Name != env.Name {
		t.Error("env name is not same")
	}

	if resp.Msg.Description != env.Description {
		t.Error("env description is not same")
	}
}

func TestUpdateEnv(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	us := suser.New(queries)
	es := senv.New(queries)
	vs := svar.New(queries)

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	CollectionID := idwrap.NewNow()
	UserID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, UserID, CollectionID)

	envID := idwrap.NewNow()
	env := menv.Env{
		ID:          envID,
		WorkspaceID: workspaceID,
		Type:        menv.EnvGlobal,
		Description: "test",
		Name:        "Something",
		Updated:     time.Now(),
	}
	err := es.Create(ctx, env)
	if err != nil {
		t.Fatal(err)
	}

	newName := "newName"
	newDesc := "newDesc"

	req := connect.NewRequest(&environmentv1.EnvironmentUpdateRequest{
		EnvironmentId: envID.Bytes(),
		Name:          &newName,
		Description:   &newDesc,
	})

	rpcEnv := renv.New(db, es, vs, us)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcEnv.EnvironmentUpdate(authedCtx, req)
	if err != nil {
		t.Error(err)
	}
	if resp == nil {
		t.Fatal("resp is nil")
	}
	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}

	dbEnv, err := es.Get(ctx, envID)
	if err != nil {
		t.Fatal(err)
	}
	if dbEnv == nil {
		t.Fatal("dbEnv is nil")
	}
	if dbEnv.Name != newName {
		t.Error("name is not updated")
	}
	if dbEnv.Description != newDesc {
		t.Error("description is not updated")
	}
}

func TestDeleteEnv(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	us := suser.New(queries)
	es := senv.New(queries)
	vs := svar.New(queries)

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	CollectionID := idwrap.NewNow()
	UserID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, UserID, CollectionID)

	envID := idwrap.NewNow()
	env := menv.Env{
		ID:          envID,
		WorkspaceID: workspaceID,
		Type:        menv.EnvGlobal,
		Description: "test",
		Name:        "Something",
		Updated:     time.Now(),
	}
	err := es.Create(ctx, env)
	if err != nil {
		t.Fatal(err)
	}

	req := connect.NewRequest(&environmentv1.EnvironmentDeleteRequest{
		EnvironmentId: envID.Bytes(),
	})

	rpcEnv := renv.New(db, es, vs, us)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcEnv.EnvironmentDelete(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("resp is nil")
	}
	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}

	dbEnv, err := es.Get(ctx, envID)
	if err == nil {
		t.Fatal("should be deleted")
	}
	if err != senv.ErrNoEnvFound {
		t.Error("err should be ErrNoEnvFound")
	}
	if dbEnv != nil {
		t.Fatal("dbEnv should be nil")
	}
}
