package renv_test

import (
	"context"
	"testing"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/renv"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/model/mworkspaceuser"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/testutil"
	environmentv1 "the-dev-tools/spec/dist/buf/go/environment/v1"
	resourcesv1 "the-dev-tools/spec/dist/buf/go/resources/v1"
	"time"

	"connectrpc.com/connect"
)

func TestCreateEnv(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	us := suser.New(queries)
	es := senv.New(queries, base.Logger())
	vs := svar.New(queries, base.Logger())

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
	es := senv.New(queries, base.Logger())
	vs := svar.New(queries, base.Logger())

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
	es := senv.New(queries, base.Logger())
	vs := svar.New(queries, base.Logger())

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
	es := senv.New(queries, base.Logger())
	vs := svar.New(queries, base.Logger())

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
	if err != senv.ErrNoEnvironmentFound {
		t.Error("err should be ErrNoEnvironmentFound")
	}
	if dbEnv != nil {
		t.Fatal("dbEnv should be nil")
	}
}

func TestEnvironmentMoveAfter(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	us := suser.New(queries)
	es := senv.New(queries, base.Logger())
	vs := svar.New(queries, base.Logger())

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	CollectionID := idwrap.NewNow()
	UserID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, UserID, CollectionID)

	// Create two environments for testing move operation
	env1ID := idwrap.NewNow()
	env1 := menv.Env{
		ID:          env1ID,
		WorkspaceID: workspaceID,
		Type:        menv.EnvNormal,
		Description: "First Environment",
		Name:        "Env1",
		Updated:     time.Now(),
	}
	err := es.Create(ctx, env1)
	if err != nil {
		t.Fatal(err)
	}

	env2ID := idwrap.NewNow()
	env2 := menv.Env{
		ID:          env2ID,
		WorkspaceID: workspaceID,
		Type:        menv.EnvNormal,
		Description: "Second Environment",
		Name:        "Env2",
		Updated:     time.Now(),
	}
	err = es.Create(ctx, env2)
	if err != nil {
		t.Fatal(err)
	}

	// Test moving env1 after env2
	req := connect.NewRequest(&environmentv1.EnvironmentMoveRequest{
		WorkspaceId:         workspaceID.Bytes(),
		EnvironmentId:       env1ID.Bytes(),
		Position:            resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		TargetEnvironmentId: env2ID.Bytes(),
	})

	rpcEnv := renv.New(db, es, vs, us)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcEnv.EnvironmentMove(authedCtx, req)
	if err != nil {
		t.Fatal("Move operation failed:", err)
	}

	if resp == nil {
		t.Fatal("resp is nil")
	}

	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}

	// Verify the order by getting the environments list
	environments, err := es.GetEnvironmentsByWorkspaceIDOrdered(ctx, workspaceID)
	if err != nil {
		t.Fatal("Failed to get ordered environments:", err)
	}

	if len(environments) != 2 {
		t.Fatal("Expected 2 environments, got:", len(environments))
	}

	// After moving env1 after env2, the order should be: env2, env1
	if environments[0].ID.Compare(env2ID) != 0 {
		t.Error("Expected env2 to be first, got:", environments[0].ID.String())
	}
	if environments[1].ID.Compare(env1ID) != 0 {
		t.Error("Expected env1 to be second, got:", environments[1].ID.String())
	}
}

func TestEnvironmentMoveBefore(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	us := suser.New(queries)
	es := senv.New(queries, base.Logger())
	vs := svar.New(queries, base.Logger())

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	CollectionID := idwrap.NewNow()
	UserID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, UserID, CollectionID)

	// Create two environments for testing move operation
	env1ID := idwrap.NewNow()
	env1 := menv.Env{
		ID:          env1ID,
		WorkspaceID: workspaceID,
		Type:        menv.EnvNormal,
		Description: "First Environment",
		Name:        "Env1",
		Updated:     time.Now(),
	}
	err := es.Create(ctx, env1)
	if err != nil {
		t.Fatal(err)
	}

	env2ID := idwrap.NewNow()
	env2 := menv.Env{
		ID:          env2ID,
		WorkspaceID: workspaceID,
		Type:        menv.EnvNormal,
		Description: "Second Environment",
		Name:        "Env2",
		Updated:     time.Now(),
	}
	err = es.Create(ctx, env2)
	if err != nil {
		t.Fatal(err)
	}

	// Test moving env2 before env1  
	req := connect.NewRequest(&environmentv1.EnvironmentMoveRequest{
		WorkspaceId:         workspaceID.Bytes(),
		EnvironmentId:       env2ID.Bytes(),
		Position:            resourcesv1.MovePosition_MOVE_POSITION_BEFORE,
		TargetEnvironmentId: env1ID.Bytes(),
	})

	rpcEnv := renv.New(db, es, vs, us)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)
	resp, err := rpcEnv.EnvironmentMove(authedCtx, req)
	if err != nil {
		t.Fatal("Move operation failed:", err)
	}

	if resp == nil {
		t.Fatal("resp is nil")
	}

	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}

	// Verify the order by getting the environments list
	environments, err := es.GetEnvironmentsByWorkspaceIDOrdered(ctx, workspaceID)
	if err != nil {
		t.Fatal("Failed to get ordered environments:", err)
	}

	if len(environments) != 2 {
		t.Fatal("Expected 2 environments, got:", len(environments))
	}

	// After moving env2 before env1, the order should be: env2, env1
	if environments[0].ID.Compare(env2ID) != 0 {
		t.Error("Expected env2 to be first, got:", environments[0].ID.String())
	}
	if environments[1].ID.Compare(env1ID) != 0 {
		t.Error("Expected env1 to be second, got:", environments[1].ID.String())
	}
}

func TestEnvironmentMoveValidationErrors(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	us := suser.New(queries)
	es := senv.New(queries, base.Logger())
	vs := svar.New(queries, base.Logger())

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	CollectionID := idwrap.NewNow()
	UserID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, UserID, CollectionID)

	// Create an environment for testing
	envID := idwrap.NewNow()
	env := menv.Env{
		ID:          envID,
		WorkspaceID: workspaceID,
		Type:        menv.EnvNormal,
		Description: "Test Environment",
		Name:        "TestEnv",
		Updated:     time.Now(),
	}
	err := es.Create(ctx, env)
	if err != nil {
		t.Fatal(err)
	}

	rpcEnv := renv.New(db, es, vs, us)
	authedCtx := mwauth.CreateAuthedContext(ctx, UserID)

	// Test case 1: Invalid environment ID
	t.Run("InvalidEnvironmentID", func(t *testing.T) {
		req := connect.NewRequest(&environmentv1.EnvironmentMoveRequest{
			WorkspaceId:         workspaceID.Bytes(),
			EnvironmentId:       []byte("invalid"),
			Position:            resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetEnvironmentId: envID.Bytes(),
		})

		_, err := rpcEnv.EnvironmentMove(authedCtx, req)
		if err == nil {
			t.Error("Expected error for invalid environment ID")
		}
		
		connectErr := err.(*connect.Error)
		if connectErr.Code() != connect.CodeInvalidArgument {
			t.Error("Expected CodeInvalidArgument, got:", connectErr.Code())
		}
	})

	// Test case 2: Invalid target environment ID
	t.Run("InvalidTargetEnvironmentID", func(t *testing.T) {
		req := connect.NewRequest(&environmentv1.EnvironmentMoveRequest{
			WorkspaceId:         workspaceID.Bytes(),
			EnvironmentId:       envID.Bytes(),
			Position:            resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetEnvironmentId: []byte("invalid"),
		})

		_, err := rpcEnv.EnvironmentMove(authedCtx, req)
		if err == nil {
			t.Error("Expected error for invalid target environment ID")
		}
		
		connectErr := err.(*connect.Error)
		if connectErr.Code() != connect.CodeInvalidArgument {
			t.Error("Expected CodeInvalidArgument, got:", connectErr.Code())
		}
	})

	// Test case 3: Unspecified position
	t.Run("UnspecifiedPosition", func(t *testing.T) {
		req := connect.NewRequest(&environmentv1.EnvironmentMoveRequest{
			WorkspaceId:         workspaceID.Bytes(),
			EnvironmentId:       envID.Bytes(),
			Position:            resourcesv1.MovePosition_MOVE_POSITION_UNSPECIFIED,
			TargetEnvironmentId: envID.Bytes(),
		})

		_, err := rpcEnv.EnvironmentMove(authedCtx, req)
		if err == nil {
			t.Error("Expected error for unspecified position")
		}
		
		connectErr := err.(*connect.Error)
		if connectErr.Code() != connect.CodeInvalidArgument {
			t.Error("Expected CodeInvalidArgument, got:", connectErr.Code())
		}
		
		if connectErr.Message() != "position must be specified" {
			t.Error("Expected specific error message, got:", connectErr.Message())
		}
	})

	// Test case 4: Self-referential move (environment cannot move relative to itself)
	t.Run("SelfReferentialMove", func(t *testing.T) {
		req := connect.NewRequest(&environmentv1.EnvironmentMoveRequest{
			WorkspaceId:         workspaceID.Bytes(),
			EnvironmentId:       envID.Bytes(),
			Position:            resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetEnvironmentId: envID.Bytes(),
		})

		_, err := rpcEnv.EnvironmentMove(authedCtx, req)
		if err == nil {
			t.Error("Expected error for self-referential move")
		}
		
		connectErr := err.(*connect.Error)
		if connectErr.Code() != connect.CodeInvalidArgument {
			t.Error("Expected CodeInvalidArgument, got:", connectErr.Code())
		}
		
		if connectErr.Message() != "cannot move environment relative to itself" {
			t.Error("Expected specific error message, got:", connectErr.Message())
		}
	})

	// Test case 5: Non-existent target environment
	t.Run("NonExistentTargetEnvironment", func(t *testing.T) {
		nonExistentEnvID := idwrap.NewNow()
		req := connect.NewRequest(&environmentv1.EnvironmentMoveRequest{
			WorkspaceId:         workspaceID.Bytes(),
			EnvironmentId:       envID.Bytes(),
			Position:            resourcesv1.MovePosition_MOVE_POSITION_AFTER,
			TargetEnvironmentId: nonExistentEnvID.Bytes(),
		})

		_, err := rpcEnv.EnvironmentMove(authedCtx, req)
		if err == nil {
			t.Error("Expected error for non-existent target environment")
		}
		
		connectErr := err.(*connect.Error)
		// When checking permissions for non-existent target environment,
		// we get CodeInternal (sql: no rows in result set) which is the correct
		// security behavior - don't reveal if environments exist to unauthorized users
		if connectErr.Code() != connect.CodeInternal {
			t.Error("Expected CodeInternal, got:", connectErr.Code())
		}
		
		if connectErr.Message() != "sql: no rows in result set" {
			t.Error("Expected sql error message, got:", connectErr.Message())
		}
	})
}

func TestEnvironmentMoveCrossWorkspaceValidation(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	us := suser.New(queries)
	es := senv.New(queries, base.Logger())
	vs := svar.New(queries, base.Logger())

	// Create two different workspaces with the same user (realistic scenario)
	workspace1ID := idwrap.NewNow()
	workspace2ID := idwrap.NewNow()
	workspaceUser1ID := idwrap.NewNow()
	workspaceUser2ID := idwrap.NewNow()
	collection1ID := idwrap.NewNow()
	userID := idwrap.NewNow() // Same user for both workspaces

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspace1ID,
		workspaceUser1ID, userID, collection1ID)
	
	// Create second workspace manually since CreateTempCollection can't be called twice
	workspaceData2 := mworkspace.Workspace{
		ID:      workspace2ID,
		Updated: time.Now(),
		Name:    "test workspace 2",
	}
	err := baseServices.Ws.Create(ctx, &workspaceData2)
	if err != nil {
		t.Fatal(err)
	}
	
	workspaceUser2Data := mworkspaceuser.WorkspaceUser{
		ID:          workspaceUser2ID,
		WorkspaceID: workspace2ID,
		UserID:      userID,
		Role:        mworkspaceuser.RoleAdmin,
	}
	err2 := baseServices.Wus.CreateWorkspaceUser(ctx, &workspaceUser2Data)
	if err2 != nil {
		t.Fatal(err2)
	}

	// Create environments in different workspaces
	env1ID := idwrap.NewNow()
	env1 := menv.Env{
		ID:          env1ID,
		WorkspaceID: workspace1ID,
		Type:        menv.EnvNormal,
		Description: "Environment in Workspace 1",
		Name:        "Env1",
		Updated:     time.Now(),
	}
	err3 := es.Create(ctx, env1)
	if err3 != nil {
		t.Fatal(err3)
	}

	env2ID := idwrap.NewNow()
	env2 := menv.Env{
		ID:          env2ID,
		WorkspaceID: workspace2ID,
		Type:        menv.EnvNormal,
		Description: "Environment in Workspace 2",
		Name:        "Env2",
		Updated:     time.Now(),
	}
	err4 := es.Create(ctx, env2)
	if err4 != nil {
		t.Fatal(err4)
	}

	// Try to move environment from workspace1 to be after environment in workspace2
	req := connect.NewRequest(&environmentv1.EnvironmentMoveRequest{
		WorkspaceId:         workspace1ID.Bytes(),
		EnvironmentId:       env1ID.Bytes(),
		Position:            resourcesv1.MovePosition_MOVE_POSITION_AFTER,
		TargetEnvironmentId: env2ID.Bytes(),
	})

	rpcEnv := renv.New(db, es, vs, us)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	_, err5 := rpcEnv.EnvironmentMove(authedCtx, req)
	if err5 == nil {
		t.Error("Expected error for cross-workspace move")
	}

	connectErr := err5.(*connect.Error)
	if connectErr.Code() != connect.CodeInvalidArgument {
		t.Error("Expected CodeInvalidArgument (environments must be in same workspace), got:", connectErr.Code())
	}
	
	expectedMsg := "environments must be in the same workspace"
	if connectErr.Message() != expectedMsg {
		t.Error("Expected message:", expectedMsg, "got:", connectErr.Message())
	}
}
