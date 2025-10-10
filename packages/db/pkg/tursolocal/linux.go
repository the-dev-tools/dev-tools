//go:build !windows

package tursolocal

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"the-dev-tools/db/pkg/sqlc"

	_ "github.com/tursodatabase/go-libsql"
)

var (
	ErrUsernameNotFound = fmt.Errorf("username not found")
	ErrDBNameNotFound   = fmt.Errorf("db name not found")
	ErrDBPathNotFound   = fmt.Errorf("db path not found")
)

type LocalDB struct {
	Write *sql.DB
	Read  *sql.DB
	Close func()
}

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

	connectionUrlParams := make(url.Values)
	connectionUrlParams.Add("_txlock", "immediate")
	connectionUrlParams.Add("_journal_mode", "WAL")
	connectionUrlParams.Add("_busy_timeout", "5000")
	connectionUrlParams.Add("_synchronous", "NORMAL")
	connectionUrlParams.Add("_cache_size", "1000000000")
	connectionUrlParams.Add("_foreign_keys", "true")

	connectionUrl := fmt.Sprintf("file:%s?%s", dbFilePath, connectionUrlParams.Encode())
	db, err := sql.Open("libsql", connectionUrl)
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

	readUrlParams := make(url.Values)
	readUrlParams.Add("_journal_mode", "WAL")
	readUrlParams.Add("_busy_timeout", "5000")
	readUrlParams.Add("_synchronous", "NORMAL")
	readUrlParams.Add("_cache_size", "1000000000")
	readUrlParams.Add("_foreign_keys", "true")
	readUrlParams.Add("_query_only", "1")

	readConnectionURL := fmt.Sprintf("file:%s?%s", dbFilePath, readUrlParams.Encode())
	readDB, err := sql.Open("libsql", readConnectionURL)
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
