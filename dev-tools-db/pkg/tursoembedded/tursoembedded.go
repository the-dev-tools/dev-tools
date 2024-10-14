package tursoembedded

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	libsqlEmbedded "github.com/tursodatabase/go-libsql"
)

var (
	ErrTokenNotFound    = fmt.Errorf("token not found")
	ErrUsernameNotFound = fmt.Errorf("username not found")
	ErrDBNameNotFound   = fmt.Errorf("db name not found")
)

func NewTursoEmbeded(dbName, username, token, volumePath, encryptionKey string) (*sql.DB, func(), error) {
	if dbName == "" {
		return nil, nil, ErrDBNameNotFound
	}
	if username == "" {
		return nil, nil, ErrDBNameNotFound
	}
	if token == "" {
		return nil, nil, ErrTokenNotFound
	}
	url := fmt.Sprintf("libsql://%s-%s.turso.io", dbName, username)

	_, err := os.Stat(volumePath)
	if os.IsNotExist(err) {
		err := os.MkdirAll(volumePath, os.ModeDir|os.ModePerm)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create directory: %w", err)
		}
	}
	dbFilePath := filepath.Join(volumePath, dbName)

	connector, err := libsqlEmbedded.NewEmbeddedReplicaConnector(dbFilePath, url,
		libsqlEmbedded.WithAuthToken(token),
		libsqlEmbedded.WithEncryption(encryptionKey),
		libsqlEmbedded.WithReadYourWrites(true),
		libsqlEmbedded.WithSyncInterval(time.Second*10),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create connector: %w", err)
	}

	db := sql.OpenDB(connector)
	err = db.Ping()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to ping database: %w", err)
	}
	a := func() {
		db.Close()
		connector.Close()
	}

	return db, a, nil
}
