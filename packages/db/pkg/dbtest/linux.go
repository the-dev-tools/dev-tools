//go:build linux

package dbtest

import (
	"context"
	"database/sql"
	"fmt"
	"the-dev-tools/db/pkg/sqlc"
	"the-dev-tools/db/pkg/sqlc/gen"

	"github.com/oklog/ulid/v2"
	_ "modernc.org/sqlite"
)

func GetTestDB(ctx context.Context) (*sql.DB, error) {
	// Generate unique database name for this test to ensure isolation
	uniqueName := ulid.Make().String()
	connStr := fmt.Sprintf("file:testdb_%s?mode=memory&cache=shared", uniqueName)

	db, err := sql.Open("sqlite", connStr)
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
	// Generate unique database name for this test to ensure isolation
	uniqueName := ulid.Make().String()
	connStr := fmt.Sprintf("file:testdb_%s?mode=memory&cache=shared", uniqueName)

	db, err := sql.Open("sqlite", connStr)
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
