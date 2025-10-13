//go:build !windows

package tursolocal

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"the-dev-tools/db/pkg/sqlc"

	_ "github.com/tursodatabase/go-libsql"
)

var (
	ErrUsernameNotFound = fmt.Errorf("username not found")
	ErrDBNameNotFound   = fmt.Errorf("db name not found")
	ErrDBPathNotFound   = fmt.Errorf("db path not found")
)

func NewTursoLocal(ctx context.Context, dbName, path, encryptionKey string) (*LocalDB, error) {
	if dbName == "" {
		return nil, ErrDBNameNotFound
	}
	if path == "" {
		return nil, ErrDBNameNotFound
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Println("Creating directory")
		if err := os.MkdirAll(path, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}
	}

	dbFilePath := filepath.Join(path, dbName+".db")
	_, statErr := os.Stat(dbFilePath)
	firstTime := os.IsNotExist(statErr)

	connectionParams := url.Values{
		"_txlock":             []string{"immediate"},
		"_journal_mode":       []string{"WAL"},
		"_busy_timeout":       []string{"10000"},
		"_synchronous":        []string{"NORMAL"},
		"_cache_size":         []string{"-524288"},
		"_foreign_keys":       []string{"true"},
		"_wal_autocheckpoint": []string{"1000"},
		"_mmap_size":          []string{"268435456"},
		"_temp_store":         []string{"memory"},
	}
	connectionParams.Set("mode", "rwc")

	writerURL := fmt.Sprintf("file:%s?%s", dbFilePath, connectionParams.Encode())
	writeDB, err := sql.Open("libsql", writerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open write database: %w", err)
	}
	writeDB.SetMaxOpenConns(1)
	writeDB.SetMaxIdleConns(1)
	if err := writeDB.PingContext(ctx); err != nil {
		writeDB.Close()
		return nil, fmt.Errorf("failed to ping write database: %w", err)
	}

	if firstTime {
		fmt.Println("Creating tables")
		if err := sqlc.CreateLocalTables(ctx, writeDB); err != nil {
			writeDB.Close()
			return nil, fmt.Errorf("failed to create tables: %w", err)
		}
		fmt.Println("Tables created")
	}

	readParams := cloneValues(connectionParams)
	readParams.Set("mode", "ro")
	readParams.Del("_txlock")

	readerURL := fmt.Sprintf("file:%s?%s", dbFilePath, readParams.Encode())
	readDB, err := sql.Open("libsql", readerURL)
	if err != nil {
		writeDB.Close()
		return nil, fmt.Errorf("failed to open read database: %w", err)
	}
	readDB.SetMaxOpenConns(10)
	readDB.SetMaxIdleConns(10)
	if err := readDB.PingContext(ctx); err != nil {
		readDB.Close()
		writeDB.Close()
		return nil, fmt.Errorf("failed to ping read database: %w", err)
	}

	localDB := &LocalDB{
		WriteDB: writeDB,
		ReadDB:  readDB,
	}

	var closeOnce sync.Once
	var closeErr error
	closeAll := func() {
		if err := writeDB.Close(); err != nil && closeErr == nil {
			closeErr = err
		}
		if err := readDB.Close(); err != nil && closeErr == nil {
			closeErr = err
		}
	}

	localDB.CloseFunc = func(context.Context) error {
		closeOnce.Do(closeAll)
		return closeErr
	}
	localDB.CleanupFunc = func() {
		closeOnce.Do(closeAll)
	}

	return localDB, nil
}
