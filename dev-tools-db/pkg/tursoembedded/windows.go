//go:build windows

package tursoembedded

import (
	"database/sql"
	"fmt"
)

var (
	ErrTokenNotFound    = fmt.Errorf("token not found")
	ErrUsernameNotFound = fmt.Errorf("username not found")
	ErrDBNameNotFound   = fmt.Errorf("db name not found")
)

func NewTursoEmbeded(dbName, username, token, volumePath, encryptionKey string) (*sql.DB, func(), error) {
	return nil, nil, fmt.Errorf("not implemented")
}
