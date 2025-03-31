package rworkspace_test

import (
	"context"
	"testing"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rworkspace"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/model/mworkspaceuser"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/service/sworkspacesusers"
	"the-dev-tools/server/pkg/testutil"
	workspacev1 "the-dev-tools/spec/dist/buf/go/workspace/v1"

	"connectrpc.com/connect"
)

func TestWorkspaceCreate(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	ws := sworkspace.New(queries)
	wus := sworkspacesusers.New(queries)
	us := suser.New(queries)
	es := senv.New(queries)

	wsIDBase := idwrap.NewNow()
	wsuserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	baseCollectionID := idwrap.NewNow()

	base.GetBaseServices().CreateTempCollection(t, ctx, wsIDBase,
		wsuserID, userID, baseCollectionID)

	testWsName := "test"

	req := connect.NewRequest(
		&workspacev1.WorkspaceCreateRequest{
			SelectedEnvironmentId: nil,
			Name:                  testWsName,
		},
	)

	serviceRPC := rworkspace.New(db, ws, wus, us, es)

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	resp, err := serviceRPC.WorkspaceCreate(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}
	msg := resp.Msg

	if msg.WorkspaceId == nil {
		t.Fatal("WorkspaceId is nil")
	}

	respWorkspaceID, err := idwrap.NewFromBytes(msg.WorkspaceId)
	if err != nil {
		t.Fatal(err)
	}

	dbWorkspace, err := ws.Get(ctx, respWorkspaceID)
	if err != nil {
		t.Fatal(err)
	}

	if testWsName != dbWorkspace.Name {
		t.Fatalf("Name mismatch, expected: %s, got: %s", testWsName, dbWorkspace.Name)
	}
}

func TestWorkspaceGet(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	ws := sworkspace.New(queries)
	wus := sworkspacesusers.New(queries)
	us := suser.New(queries)
	es := senv.New(queries)

	wsIDBase := idwrap.NewNow()
	wsuserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	baseCollectionID := idwrap.NewNow()

	base.GetBaseServices().CreateTempCollection(t, ctx, wsIDBase,
		wsuserID, userID, baseCollectionID)

	testWsID := idwrap.NewNow()
	testWsName := "test"

	testWorkspace := &mworkspace.Workspace{
		ID:      testWsID,
		Name:    testWsName,
		Updated: dbtime.DBNow(),
	}

	testWorkspaceUser := &mworkspaceuser.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: testWsID,
		UserID:      userID,
		Role:        mworkspaceuser.RoleOwner,
	}

	err := ws.Create(ctx, testWorkspace)
	if err != nil {
		t.Fatal(err)
	}

	err = wus.CreateWorkspaceUser(ctx, testWorkspaceUser)
	if err != nil {
		t.Fatal(err)
	}

	req := connect.NewRequest(
		&workspacev1.WorkspaceGetRequest{
			WorkspaceId: testWsID.Bytes(),
		},
	)

	serviceRPC := rworkspace.New(db, ws, wus, us, es)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp, err := serviceRPC.WorkspaceGet(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}
	msg := resp.Msg

	if msg.WorkspaceId == nil {
		t.Fatal("WorkspaceId is nil")
	}

	respWorkspaceID, err := idwrap.NewFromBytes(msg.WorkspaceId)
	if err != nil {
		t.Fatal(err)
	}

	if testWsID.Compare(respWorkspaceID) != 0 {
		t.Fatalf("WorkspaceGet failed: id mismatch")
	}

	if msg.Name != testWsName {
		t.Fatalf("WorkspaceGet failed: name mismatch")
	}

	if testWorkspace.Updated.Unix() != msg.Updated.Seconds {
		t.Fatalf("Updated mismatch, expected: %v, got: %v", testWorkspace.Updated.Unix(), msg.Updated.Seconds)
	}
}

func TestWorkspaceUpdate(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	ws := sworkspace.New(queries)
	wus := sworkspacesusers.New(queries)
	us := suser.New(queries)
	es := senv.New(queries)

	// TODO: change to correc service
	wsIDBase := idwrap.NewNow()
	wsuserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	baseCollectionID := idwrap.NewNow()

	base.GetBaseServices().CreateTempCollection(t, ctx, wsIDBase,
		wsuserID, userID, baseCollectionID)

	testWsID := idwrap.NewNow()
	testWsName := "test"

	testWorkspace := &mworkspace.Workspace{
		ID:      testWsID,
		Name:    testWsName,
		Updated: dbtime.DBNow(),
	}

	testWorkspaceUser := &mworkspaceuser.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: testWsID,
		UserID:      userID,
		Role:        mworkspaceuser.RoleOwner,
	}

	err := ws.Create(ctx, testWorkspace)
	if err != nil {
		t.Fatal(err)
	}

	err = wus.CreateWorkspaceUser(ctx, testWorkspaceUser)
	if err != nil {
		t.Fatal(err)
	}

	env := menv.Env{
		ID:          idwrap.NewNow(),
		WorkspaceID: testWsID,
		Type:        menv.EnvType(0),
		Name:        "test",
		Description: "desc",
		Updated:     dbtime.DBNow(),
	}

	err = es.Create(ctx, env)
	if err != nil {
		t.Fatal(err)
	}

	testNewWsName := "new test"

	req := connect.NewRequest(
		&workspacev1.WorkspaceUpdateRequest{
			WorkspaceId:           testWsID.Bytes(),
			Name:                  &testNewWsName,
			SelectedEnvironmentId: env.ID.Bytes(),
		},
	)

	serviceRPC := rworkspace.New(db, ws, wus, us, es)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp, err := serviceRPC.WorkspaceUpdate(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}

	dbWorkspace, err := ws.Get(ctx, testWsID)
	if err != nil {
		t.Fatal(err)
	}

	if testNewWsName != dbWorkspace.Name {
		t.Fatalf("Name mismatch, expected: %s, got: %s", testNewWsName, dbWorkspace.Name)
	}
}

func TestWorkspaceDelete(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	ws := sworkspace.New(queries)
	wus := sworkspacesusers.New(queries)
	us := suser.New(queries)
	es := senv.New(queries)

	// TODO: change to correc service
	wsIDBase := idwrap.NewNow()
	wsuserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	baseCollectionID := idwrap.NewNow()

	base.GetBaseServices().CreateTempCollection(t, ctx, wsIDBase,
		wsuserID, userID, baseCollectionID)

	testWsID := idwrap.NewNow()
	testWsName := "test"

	testWorkspace := &mworkspace.Workspace{
		ID:      testWsID,
		Name:    testWsName,
		Updated: dbtime.DBNow(),
	}

	testWorkspaceUser := &mworkspaceuser.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: testWsID,
		UserID:      userID,
		Role:        mworkspaceuser.RoleOwner,
	}

	err := ws.Create(ctx, testWorkspace)
	if err != nil {
		t.Fatal(err)
	}

	err = wus.CreateWorkspaceUser(ctx, testWorkspaceUser)
	if err != nil {
		t.Fatal(err)
	}

	req := connect.NewRequest(
		&workspacev1.WorkspaceDeleteRequest{
			WorkspaceId: testWsID.Bytes(),
		},
	)

	serviceRPC := rworkspace.New(db, ws, wus, us, es)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp, err := serviceRPC.WorkspaceDelete(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}

	dbWorkspace, err := ws.Get(ctx, testWsID)
	if err == nil {
		t.Fatal("Workspace not deleted")
	}
	if err != sworkspace.ErrNoWorkspaceFound {
		t.Fatalf("Expected ErrNoWorkspaceFound, got: %v", err)
	}
	if dbWorkspace != nil {
		t.Fatalf("Workspace not deleted")
	}
}
