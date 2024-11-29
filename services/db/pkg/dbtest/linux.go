//go:build linux

package dbtest

import (
	"context"
	"database/sql"
	"the-dev-tools/db/pkg/sqlc"
	"the-dev-tools/db/pkg/sqlc/gen"

	_ "github.com/mattn/go-sqlite3"
)

func GetTestDB(ctx context.Context) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, err
	}

	// create tables
	err = sqlc.CreateLocalTables(ctx, db)
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
	err = sqlc.CreateLocalTables(ctx, db)
	if err != nil {
		return nil, err
	}

	prepared, err := gen.Prepare(ctx, db)
	if err != nil {
		return nil, err
	}

	return prepared, nil
}
