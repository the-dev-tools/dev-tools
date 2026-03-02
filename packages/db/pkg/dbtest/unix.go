//go:build !windows

package dbtest

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"

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

// EnableForeignKeys enables SQLite foreign key enforcement on db. It must be
// called after SetMaxOpenConns(1) so the PRAGMA applies to every connection.
// PRAGMA is a SQLite connection directive — there is no sqlc-generated equivalent.
func EnableForeignKeys(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON")
	return err
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
