//go:build windows

package tursolocal

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"the-dev-tools/db/pkg/sqlc"

	_ "github.com/mattn/go-sqlite3"
)

var (
	ErrUsernameNotFound = fmt.Errorf("username not found")
	ErrDBNameNotFound   = fmt.Errorf("db name not found")
	ErrDBPathNotFound   = fmt.Errorf("db path not found")
)

func NewTursoLocal(ctx context.Context, dbName, path, encryptionKey string) (LocalDB, error) {
	var result LocalDB

	if dbName == "" {
		return result, ErrDBNameNotFound
	}
	if path == "" {
		return result, ErrDBNameNotFound
	}

	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		err := os.MkdirAll(path, os.ModeAppend)
		fmt.Println("Creating directory")
		if err != nil {
			return result, fmt.Errorf("failed to create directory: %w", err)
		}
	}

	dbFilePath := filepath.Join(path, dbName+".db")
	_, err = os.Stat(dbFilePath)
	var firstTime bool
	if os.IsNotExist(err) {
		firstTime = true
	}
	dbFilePath = fmt.Sprintf("file:%s?mode=rwc&_journal_mode=WAL", dbFilePath)
	db, err := sql.Open("sqlite3", dbFilePath)
	if err != nil {
		return result, fmt.Errorf("failed to open database: %w", err)
	}
	db.SetMaxOpenConns(1)
	err = db.Ping()
	if err != nil {
		return result, fmt.Errorf("failed to ping database: %w", err)
	}
	if firstTime {
		fmt.Println("Creating tables")
		err = sqlc.CreateLocalTables(ctx, db)
		if err != nil {
			return result, fmt.Errorf("failed to create tables: %w", err)
		}

		fmt.Println("Tables created")
	}

	readConnStr := fmt.Sprintf("file:%s?mode=ro&_journal_mode=WAL&_query_only=1", filepath.Join(path, dbName+".db"))
	readDB, err := sql.Open("sqlite3", readConnStr)
	if err != nil {
		db.Close()
		return result, fmt.Errorf("failed to open read database: %w", err)
	}
	readDB.SetMaxOpenConns(16)
	readDB.SetMaxIdleConns(16)
	if err := readDB.Ping(); err != nil {
		readDB.Close()
		db.Close()
		return result, fmt.Errorf("failed to ping read database: %w", err)
	}

	result = LocalDB{
		Write: db,
		Read:  readDB,
		Close: func() {
			_ = db.Close()
			_ = readDB.Close()
		},
	}

	return result, nil
}
