package sqlc

import (
	"context"
	"database/sql"
	_ "embed"
	"regexp"
	"strings"
	"the-dev-tools/db/pkg/sqlc/gen"

	"github.com/pingcap/log"
)

//go:embed schema.sql
var ddl string

func CreateLocalTables(ctx context.Context, db *sql.DB) error {
	// Replace CREATE statements to handle existing tables gracefully
	modifiedDDL := strings.ReplaceAll(ddl, "CREATE TABLE ", "CREATE TABLE IF NOT EXISTS ")
	
	// Handle CREATE INDEX statements with case-insensitive regex patterns
	// This handles: CREATE INDEX, create index, CREATE UNIQUE INDEX, etc.
	createIndexRegex := regexp.MustCompile(`(?i)\bCREATE\s+(UNIQUE\s+)?INDEX\s+`)
	modifiedDDL = createIndexRegex.ReplaceAllStringFunc(modifiedDDL, func(match string) string {
		// Preserve the original case and spacing, but add IF NOT EXISTS
		if strings.Contains(strings.ToUpper(match), "UNIQUE") {
			return regexp.MustCompile(`(?i)\bCREATE\s+UNIQUE\s+INDEX\s+`).ReplaceAllString(match, "CREATE UNIQUE INDEX IF NOT EXISTS ")
		}
		return regexp.MustCompile(`(?i)\bCREATE\s+INDEX\s+`).ReplaceAllString(match, "CREATE INDEX IF NOT EXISTS ")
	})
	
	tables := strings.Split(modifiedDDL, ";")
	// INFO: this hack is needed because the ddl string has a trailing semicolon
	// but this should be removed when libsql fix this
	tables = tables[:len(tables)-1]
	for _, table := range tables {
		table = strings.TrimSpace(table)
		if table == "" {
			continue
		}
		_, err := db.ExecContext(ctx, table)
		if err != nil {
			// As a fallback, ignore "already exists" errors for robustness
			if strings.Contains(err.Error(), "already exists") {
				log.Warn("Table or index already exists, ignoring error: " + err.Error())
				continue
			}
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
