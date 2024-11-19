//go:build linux

package dbtest

import (
	"context"
	"database/sql"
	"dev-tools-db/pkg/sqlc"
	"dev-tools-db/pkg/sqlc/gen"

	_ "github.com/tursodatabase/go-libsql"
)

func GetTestDB(ctx context.Context) (*sql.DB, error) {
	db, err := sql.Open("libsql", ":memory:")
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
	db, err := sql.Open("libsql", ":memory:")
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
