package testutil

import (
	"context"
	"database/sql"
	"dev-tools-backend/internal/api/collection"
	"dev-tools-backend/internal/api/middleware/mwauth"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mcollection"
	"dev-tools-backend/pkg/model/muser"
	"dev-tools-backend/pkg/model/mworkspace"
	"dev-tools-backend/pkg/model/mworkspaceuser"
	"dev-tools-backend/pkg/service/scollection"
	"dev-tools-backend/pkg/service/suser"
	"dev-tools-backend/pkg/service/sworkspace"
	"dev-tools-backend/pkg/service/sworkspacesusers"
	"dev-tools-db/pkg/sqlc"
	"dev-tools-db/pkg/sqlc/gen"
	"testing"
	"time"
)

type BaseDBQueries struct {
	Queries *gen.Queries
	DB      *sql.DB
	t       *testing.T
	ctx     context.Context
}

type BaseTestServices struct {
	Ctx context.Context
	DB  *sql.DB
	Cs  scollection.CollectionService
	Us  suser.UserService
	Ws  sworkspace.WorkspaceService
	Wus sworkspacesusers.WorkspaceUserService
}

func CreateBaseDB(ctx context.Context, t *testing.T) *BaseDBQueries {
	db, err := sqlc.GetTestDB(ctx)
	if err != nil {
		t.Fatal(err)
	}
	queries, err := gen.Prepare(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	return &BaseDBQueries{Queries: queries, t: t, ctx: ctx}
}

func (c BaseDBQueries) GetBaseServices() BaseTestServices {
	queries := c.Queries
	cs := scollection.New(queries)
	ws := sworkspace.New(queries)
	wus := sworkspacesusers.New(queries)
	us := suser.New(queries)
	return BaseTestServices{
		Cs:  cs,
		Us:  us,
		Ws:  ws,
		Wus: wus,
	}
}

func (c BaseTestServices) CreateAuthedCtx(userID idwrap.IDWrap) context.Context {
	return mwauth.CreateAuthedContext(c.Ctx, userID)
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

func (c BaseTestServices) CreateCollectionRPC() collection.CollectionServiceRPC {
	return collection.New(c.DB, c.Cs, c.Ws, c.Us)
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
