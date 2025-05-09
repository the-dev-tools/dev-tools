package sqlc

import (
	"context"
	"database/sql"
	_ "embed"
	"strings"
	"the-dev-tools/db/pkg/sqlc/gen"

	"github.com/pingcap/log"
)

//go:embed schema.sql
var ddl string

func CreateLocalTables(ctx context.Context, db *sql.DB) error {
	tables := strings.Split(ddl, ";")
	// INFO: this hack is needed because the ddl string has a trailing semicolon
	// but this should be remove when libsql fix this
	tables = tables[:len(tables)-1]
	for _, table := range tables {
		_, err := db.ExecContext(ctx, table)
		if err != nil {
			return err
		}
	}
	return nil
}

func GetService[I any](ctx context.Context, queries *gen.Queries, serviceFunc func(context.Context, *gen.Queries) I) I {
	return serviceFunc(ctx, queries)
}

// checks if error then logs error
func CloseQueriesAndLog(q *gen.Queries) {
	err := q.Close()
	if err != nil {
		log.Error(err.Error())
	}
}
