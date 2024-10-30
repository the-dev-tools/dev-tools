package sqlc

import (
	"context"
	"database/sql"
	"dev-tools-db/pkg/sqlc/gen"
	_ "embed"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema.sql
var ddl string

func GetTestDB(ctx context.Context) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, err
	}

	// create tables
	_, err = db.ExecContext(ctx, ddl)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func GetTestPreparedQueries(ctx context.Context) (*gen.Queries, error) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, err
	}

	// create tables
	if _, err := db.ExecContext(ctx, ddl); err != nil {
		return nil, err
	}
	prepared, err := gen.Prepare(ctx, db)
	if err != nil {
		return nil, err
	}

	return prepared, nil
}

func GetService[I any](ctx context.Context, queries *gen.Queries, serviceFunc func(context.Context, *gen.Queries) I) I {
	return serviceFunc(ctx, queries)
}
