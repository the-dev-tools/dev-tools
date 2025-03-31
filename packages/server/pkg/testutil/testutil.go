package testutil

import (
	"context"
	"database/sql"
	"testing"
	"the-dev-tools/db/pkg/dbtest"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/internal/api/rcollection"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcollection"
	"the-dev-tools/server/pkg/model/muser"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/model/mworkspaceuser"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/service/sworkspacesusers"
	"time"
)

type BaseDBQueries struct {
	Queries *gen.Queries
	DB      *sql.DB
	t       *testing.T
	ctx     context.Context
}

type BaseTestServices struct {
	DB  *sql.DB
	Cs  scollection.CollectionService
	Us  suser.UserService
	Ws  sworkspace.WorkspaceService
	Wus sworkspacesusers.WorkspaceUserService
}

func CreateBaseDB(ctx context.Context, t *testing.T) *BaseDBQueries {
	db, err := dbtest.GetTestDB(ctx)
	if err != nil {
		t.Fatal(err)
	}
	queries, err := gen.Prepare(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	return &BaseDBQueries{Queries: queries, t: t, ctx: ctx, DB: db}
}

func (c BaseDBQueries) GetBaseServices() BaseTestServices {
	queries := c.Queries
	cs := scollection.New(queries)
	ws := sworkspace.New(queries)
	wus := sworkspacesusers.New(queries)
	us := suser.New(queries)
	return BaseTestServices{
		DB:  c.DB,
		Cs:  cs,
		Us:  us,
		Ws:  ws,
		Wus: wus,
	}
}

func (c BaseTestServices) CreateTempCollection(t *testing.T, ctx context.Context, wsID, wuID, userID, collectionID idwrap.IDWrap) {
	cs := c.Cs
	ws := c.Ws
	wus := c.Wus
	us := c.Us

	workspaceData := mworkspace.Workspace{
		ID:      wsID,
		Updated: time.Now(),
		Name:    "test",
	}

	err := ws.Create(ctx, &workspaceData)
	if err != nil {
		t.Fatal(err)
	}

	providerID := "test"
	userData := muser.User{
		ID:           userID,
		Email:        "test@dev.tools",
		Password:     []byte("test"),
		ProviderID:   &providerID,
		ProviderType: muser.MagicLink,
		Status:       muser.Active,
	}

	err = us.CreateUser(ctx, &userData)
	if err != nil {
		t.Fatal(err)
	}

	workspaceUserData := mworkspaceuser.WorkspaceUser{
		ID:          wuID,
		WorkspaceID: wsID,
		UserID:      userID,
		Role:        mworkspaceuser.RoleAdmin,
	}

	err = wus.CreateWorkspaceUser(ctx, &workspaceUserData)
	if err != nil {
		t.Fatal(err)
	}

	collectionData := mcollection.Collection{
		ID:      collectionID,
		OwnerID: wsID,
		Name:    "test",
		Updated: time.Now(),
	}

	err = cs.CreateCollection(ctx, &collectionData)
	if err != nil {
		t.Fatal(err)
	}

	collectionGet, err := cs.GetCollection(ctx, collectionID)
	if err != nil {
		t.Fatal(err)
	}

	if collectionGet == nil {
		t.Fatal("Collection not found")
	}
}

func (c BaseTestServices) CreateCollectionRPC() rcollection.CollectionServiceRPC {
	return rcollection.New(c.DB, c.Cs, c.Ws, c.Us)
}

func (b BaseDBQueries) Close() {
	err := b.DB.Close()
	if err != nil {
		b.t.Error(err)
	}
	err = b.Queries.Close()
	if err != nil {
		b.t.Error(err)
	}
}

func AssertFatal[c comparable](t *testing.T, expected, got c) {
	t.Helper()
	if got != expected {
		t.Fatalf("got %v, expected %v", got, expected)
	}
}

func Assert[c comparable](t *testing.T, expected, got c) {
	t.Helper()
	if got != expected {
		t.Errorf("got %v, expected %v", got, expected)
	}
}

func AssertNot[c comparable](t *testing.T, not, got c) {
	t.Helper()
	if got == not {
		t.Errorf("got %v, expected not %v", got, not)
	}
}

func AssertNotFatal[c comparable](t *testing.T, not, got c) {
	t.Helper()
	if got == not {
		t.Fatalf("got %v, expected not %v", got, not)
	}
}
