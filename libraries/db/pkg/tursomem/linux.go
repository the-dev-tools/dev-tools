//go:build !windows

package tursomem

import (
	"context"
	"database/sql"
	"fmt"
	"the-dev-tools/db/pkg/sqlc"

	_ "github.com/tursodatabase/go-libsql"
)

var (
	ErrDBNameNotFound = fmt.Errorf("db name not found")
	ErrDBPathNotFound = fmt.Errorf("db path not found")
)

func NewTursoLocal(ctx context.Context) (*sql.DB, func(), error) {
	db, err := sql.Open("libsql", ":memory:")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open database: %w", err)
	}
	db.SetMaxOpenConns(1)
	a := func() {
		db.Close()
	}
	err = sqlc.CreateLocalTables(ctx, db)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return db, a, nil
}
