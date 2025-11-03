package testutil

import (
	"context"
	"database/sql"
	"log/slog"
	"testing"
	"the-dev-tools/db/pkg/dbtest"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/service/sworkspacesusers"
)

type BaseDBQueries struct {
	Queries *gen.Queries
	DB      *sql.DB
	t       *testing.T
	ctx     context.Context
}

type BaseTestServices struct {
	DB  *sql.DB
	Us  suser.UserService
	Ws  sworkspace.WorkspaceService
	Wus sworkspacesusers.WorkspaceUserService
	Hs  shttp.HTTPService
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
	ws := sworkspace.New(queries)
	wus := sworkspacesusers.New(queries)
	us := suser.New(queries)
	hs := shttp.New(queries, mockLogger)
	return BaseTestServices{
		DB:  c.DB,
		Us:  us,
		Ws:  ws,
		Wus: wus,
		Hs:  hs,
	}
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
