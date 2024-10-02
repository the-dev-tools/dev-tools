package tursolocal

import (
	"database/sql"
	"fmt"
	"path/filepath"
)

var (
	ErrUsernameNotFound = fmt.Errorf("username not found")
	ErrDBNameNotFound   = fmt.Errorf("db name not found")
	ErrDBPathNotFound   = fmt.Errorf("db path not found")
)

func NewTursoLocal(dbName, path, encryptionKey string) (*sql.DB, func(), error) {
	if dbName == "" {
		return nil, nil, ErrDBNameNotFound
	}
	if path == "" {
		return nil, nil, ErrDBNameNotFound
	}
	dbFilePath := filepath.Join(path, dbName)
	dbFilePath = fmt.Sprintf("file:%s.db", dbFilePath)

	db, err := sql.Open("libsql", dbFilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open database: %w", err)
	}
	err = db.Ping()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to ping database: %w", err)
	}
	a := func() {
		db.Close()
	}

	return db, a, nil
}
