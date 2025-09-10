//go:build windows && arm64

package tursolocal

import (
    "context"
    "database/sql"
    "fmt"
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

// NewTursoLocal provides a CGO-free Windows/ARM64 implementation using modernc.org/sqlite
func NewTursoLocal(ctx context.Context, dbName, path, encryptionKey string) (*sql.DB, func(), error) {
    if dbName == "" {
        return nil, nil, ErrDBNameNotFound
    }
    if path == "" {
        return nil, nil, ErrDBNameNotFound
    }

    if _, err := os.Stat(path); os.IsNotExist(err) {
        if err := os.MkdirAll(path, os.ModePerm); err != nil {
            return nil, nil, fmt.Errorf("failed to create directory: %w", err)
        }
    }

    dbFile := filepath.Join(path, dbName+".db")
    _, err := os.Stat(dbFile)
    firstTime := os.IsNotExist(err)

    // modernc driver name is "sqlite"
    // WAL + busy timeout for better local behavior
    dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&mode=rwc", dbFile)
    db, err := sql.Open("sqlite", dsn)
    if err != nil {
        return nil, nil, fmt.Errorf("failed to open database: %w", err)
    }
    db.SetMaxOpenConns(1)
    if err := db.Ping(); err != nil {
        return nil, nil, fmt.Errorf("failed to ping database: %w", err)
    }

    cleanup := func() { _ = db.Close() }

    if firstTime {
        if err := sqlc.CreateLocalTables(ctx, db); err != nil {
            return nil, nil, fmt.Errorf("failed to create tables: %w", err)
        }
    }

    return db, cleanup, nil
}

