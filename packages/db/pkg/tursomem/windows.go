//go:build windows

package tursomem

import (
	"context"
	"database/sql"
	"fmt"
	"the-dev-tools/db/pkg/sqlc"

	_ "github.com/mattn/go-sqlite3"
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

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return result, fmt.Errorf("failed to open database: %w", err)
	}
	db.SetMaxOpenConns(1)
	fmt.Println("Creating tables")
	if err := sqlc.CreateLocalTables(ctx, db); err != nil {
		db.Close()
		return result, fmt.Errorf("failed to create tables: %w", err)
	}

	fmt.Println("Tables created")

	result = LocalDB{
		Write: db,
		Read:  db,
		Close: func() { _ = db.Close() },
	}

	return result, nil
}
