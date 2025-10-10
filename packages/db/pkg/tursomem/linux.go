//go:build !windows

package tursomem

import (
	"context"
	"database/sql"
	"fmt"
	"the-dev-tools/db/pkg/sqlc"

	_ "github.com/tursodatabase/go-libsql"
)

type LocalDB struct {
	Write *sql.DB
	Read  *sql.DB
	Close func()
}

var (
	ErrDBNameNotFound = fmt.Errorf("db name not found")
	ErrDBPathNotFound = fmt.Errorf("db path not found")
)

func NewTursoLocal(ctx context.Context) (LocalDB, error) {
	var result LocalDB

	db, err := sql.Open("libsql", ":memory:")
	if err != nil {
		return result, fmt.Errorf("failed to open database: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := sqlc.CreateLocalTables(ctx, db); err != nil {
		db.Close()
		return result, fmt.Errorf("failed to create tables: %w", err)
	}

	result = LocalDB{
		Write: db,
		Read:  db,
		Close: func() { _ = db.Close() },
	}

	return result, nil
}
