package tursoclient

/*
import (
	"database/sql"
	"fmt"

	libsqlClient "github.com/tursodatabase/libsql-client-go/libsql"
)

var (
	ErrTokenNotFound    = fmt.Errorf("token not found")
	ErrUsernameNotFound = fmt.Errorf("username not found")
	ErrDBNameNotFound   = fmt.Errorf("db name not found")
)

func NewTurso(dbName, username, token string) (*sql.DB, error) {
	if dbName == "" {
		return nil, ErrDBNameNotFound
	}

	if username == "" {
		return nil, ErrDBNameNotFound
	}

	if token == "" {
		return nil, ErrTokenNotFound
	}

	url := fmt.Sprintf("libsql://%s-%s.turso.io", dbName, username)
	connector, err := libsqlClient.NewConnector(url, libsqlClient.WithAuthToken(token))
	if err != nil {
		return nil, fmt.Errorf("failed to create connector: %w", err)
	}

	db := sql.OpenDB(connector)
	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}
*/
