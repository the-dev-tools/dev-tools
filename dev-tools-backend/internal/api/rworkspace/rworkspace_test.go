package rworkspace_test

import (
	"context"
	"dev-tools-backend/internal/api/middleware/mwauth"
	"dev-tools-backend/internal/api/rworkspace"
	"dev-tools-backend/pkg/dbtime"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/menv"
	"dev-tools-backend/pkg/model/mworkspace"
	"dev-tools-backend/pkg/model/mworkspaceuser"
	"dev-tools-backend/pkg/service/senv"
	"dev-tools-backend/pkg/service/suser"
	"dev-tools-backend/pkg/service/sworkspace"
	"dev-tools-backend/pkg/service/sworkspacesusers"
	"dev-tools-backend/pkg/testutil"
	"dev-tools-mail/pkg/emailclient"
	"dev-tools-mail/pkg/emailinvite"
	workspacev1 "dev-tools-spec/dist/buf/go/workspace/v1"
	"testing"

	"connectrpc.com/connect"
)

func TestWorkspaceCreate(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	defer queries.Close()
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

	testWsName := "test"

	req := connect.NewRequest(
		&workspacev1.WorkspaceCreateRequest{
			SelectedEnvironmentId: nil,
			Name:                  testWsName,
		},
	)

	serviceRPC := rworkspace.New(db, ws, wus, us, es, emailclient.EmailClient{}, &emailinvite.EmailTemplateManager{})

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
	defer queries.Close()
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
		&workspacev1.WorkspaceGetRequest{
			WorkspaceId: testWsID.Bytes(),
		},
	)

	serviceRPC := rworkspace.New(db, ws, wus, us, es, emailclient.EmailClient{}, &emailinvite.EmailTemplateManager{})
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
	defer queries.Close()
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
		Active:      true,
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
			Name:                  testNewWsName,
			SelectedEnvironmentId: env.ID.Bytes(),
		},
	)

	serviceRPC := rworkspace.New(db, ws, wus, us, es, emailclient.EmailClient{}, &emailinvite.EmailTemplateManager{})
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
	defer queries.Close()
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

	serviceRPC := rworkspace.New(db, ws, wus, us, es, emailclient.EmailClient{}, &emailinvite.EmailTemplateManager{})
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
