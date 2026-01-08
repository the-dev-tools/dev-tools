//nolint:revive // exported
package testutil

import (
	"context"
	"database/sql"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/dbtest"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/logger/mocklogger"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/suser"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
	"log/slog"
	"testing"
)

type BaseDBQueries struct {
	Queries *gen.Queries
	DB      *sql.DB
	t       *testing.T
	ctx     context.Context
}

type BaseTestServices struct {
	Queries              *gen.Queries
	DB                   *sql.DB
	UserService          suser.UserService
	WorkspaceService     sworkspace.WorkspaceService
	WorkspaceUserService sworkspace.UserService
	HttpService          shttp.HTTPService
	FlowService          sflow.FlowService
	FlowVariableService  sflow.FlowVariableService
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

	mockLogger := mocklogger.NewMockLogger()
	ws := sworkspace.NewWorkspaceService(queries)
	wus := sworkspace.NewUserService(queries)
	us := suser.New(queries)
	hs := shttp.New(queries, mockLogger)
	fs := sflow.NewFlowService(queries)
	fvs := sflow.NewFlowVariableService(queries)
	return BaseTestServices{
		Queries:              queries,
		DB:                   c.DB,
		UserService:          us,
		WorkspaceService:     ws,
		WorkspaceUserService: wus,
		HttpService:          hs,
		FlowService:          fs,
		FlowVariableService:  fvs,
	}
}

func (b BaseTestServices) CreateTempCollection(ctx context.Context, userID idwrap.IDWrap, name string) (idwrap.IDWrap, error) {
	workspaceID := idwrap.NewNow()
	err := b.WorkspaceService.Create(ctx, &mworkspace.Workspace{
		ID:   workspaceID,
		Name: name,
	})
	if err != nil {
		return idwrap.IDWrap{}, err
	}

	err = b.WorkspaceUserService.CreateWorkspaceUser(ctx, &mworkspace.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        mworkspace.RoleOwner,
	})
	if err != nil {
		return idwrap.IDWrap{}, err
	}

	return workspaceID, nil
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

func (b BaseDBQueries) Logger() *slog.Logger {
	return mocklogger.NewMockLogger()
}
