package sqlitelocal

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"the-dev-tools/db/pkg/sqlc"

	_ "modernc.org/sqlite"
)

var (
	ErrUsernameNotFound = fmt.Errorf("username not found")
	ErrDBNameNotFound   = fmt.Errorf("db name not found")
	ErrDBPathNotFound   = fmt.Errorf("db path not found")
)

func NewSQLiteLocal(ctx context.Context, dbName, path, encryptionKey string) (*sql.DB, func(), error) {
	if dbName == "" {
		return nil, nil, ErrDBNameNotFound
	}
	if path == "" {
		return nil, nil, ErrDBNameNotFound
	}

	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		err := os.MkdirAll(path, os.ModeAppend)
		fmt.Println("Creating directory")
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create directory: %w", err)
		}
	}

	dbFilePath := filepath.Join(path, dbName+".db")
	_, err = os.Stat(dbFilePath)
	var firstTime bool
	if os.IsNotExist(err) {
		firstTime = true
	}

	connectionUrlParams := make(url.Values)
	connectionUrlParams.Add("_txlock", "immediate")
	connectionUrlParams.Add("_journal_mode", "WAL")
	connectionUrlParams.Add("_busy_timeout", "5000")
	connectionUrlParams.Add("_synchronous", "NORMAL")
	connectionUrlParams.Add("_cache_size", "1000000000")
	connectionUrlParams.Add("_foreign_keys", "true")

	connectionUrl := fmt.Sprintf("file:%s?%s", dbFilePath, connectionUrlParams.Encode())
	db, err := sql.Open("sqlite", connectionUrl)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open database: %w", err)
	}
	db.SetMaxOpenConns(1)
	err = db.Ping()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to ping database: %w", err)
	}
	a := func() {
		db.Close()
	}
	if firstTime {
		fmt.Println("Creating tables")
		err = sqlc.CreateLocalTables(ctx, db)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create tables: %w", err)
		}

		fmt.Println("Tables created")
	}

	return db, a, nil
}