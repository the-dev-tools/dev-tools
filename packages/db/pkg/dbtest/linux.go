//go:build linux

package dbtest

import (
    "context"
    "database/sql"
    "fmt"
    "regexp"
    "strings"
    "the-dev-tools/db/pkg/sqlc"
    "the-dev-tools/db/pkg/sqlc/gen"

    _ "github.com/mattn/go-sqlite3"
)

// ContextDBNameKey is the context key that, when present, provides a stable
// test-specific database name so multiple helpers within the same test share
// the same in-memory SQLite database.
type ContextDBNameKey string

// CtxDBNameKey is the exported key value to use with context.WithValue.
const CtxDBNameKey ContextDBNameKey = "dbtest:name"

var nonAlnum = regexp.MustCompile(`[^a-zA-Z0-9_]+`)

func dsnFromContext(ctx context.Context) string {
    // Default shared name if none provided
    name, _ := ctx.Value(CtxDBNameKey).(string)
    if name == "" {
        return "file:devtools_test?mode=memory&cache=shared&_busy_timeout=5000"
    }
    // Sanitize name for DSN
    norm := strings.ToLower(name)
    norm = nonAlnum.ReplaceAllString(norm, "_")
    return fmt.Sprintf("file:devtools_%s?mode=memory&cache=shared&_busy_timeout=5000", norm)
}

func GetTestDB(ctx context.Context) (*sql.DB, error) {
    // Use a shared in-memory database across helpers within the same test
    db, err := sql.Open("sqlite3", dsnFromContext(ctx))
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
    db, err := sql.Open("sqlite3", dsnFromContext(ctx))
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
