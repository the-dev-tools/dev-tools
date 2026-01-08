//go:build windows

package tursolocal

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc"
	"net/url"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
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
		if err := os.MkdirAll(path, os.ModeAppend); err != nil {
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}
	}

	dbFile := filepath.Join(path, dbName+".db")
	_, statErr := os.Stat(dbFile)
	firstTime := os.IsNotExist(statErr)

	writerParams := url.Values{
		"mode":                []string{"rwc"},
		"_journal_mode":       []string{"WAL"},
		"_busy_timeout":       []string{"10000"},
		"_foreign_keys":       []string{"true"},
		"_synchronous":        []string{"NORMAL"},
		"_cache_size":         []string{"-524288"},
		"_temp_store":         []string{"memory"},
		"_wal_autocheckpoint": []string{"1000"},
	}

	writerDSN := fmt.Sprintf("file:%s?%s", dbFile, writerParams.Encode())
	writeDB, err := sql.Open("sqlite", writerDSN)
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

	readerParams := cloneValues(writerParams)
	readerParams.Set("mode", "ro")
	readerParams.Del("_wal_autocheckpoint")

	readerDSN := fmt.Sprintf("file:%s?%s", dbFile, readerParams.Encode())
	readDB, err := sql.Open("sqlite", readerDSN)
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
