package tursolocal

import (
	"context"
	"database/sql"
	"net/url"
)

// LocalDB wraps the read/write pools provided by the local Turso adapter.
type LocalDB struct {
	WriteDB     *sql.DB
	ReadDB      *sql.DB
	CleanupFunc func()
	CloseFunc   func(context.Context) error
}

// Default returns the primary writable connection pool.
func (l *LocalDB) Default() *sql.DB {
	if l == nil {
		return nil
	}
	return l.WriteDB
}

func cloneValues(src url.Values) url.Values {
	dest := make(url.Values, len(src))
	for k, v := range src {
		dest[k] = append([]string(nil), v...)
	}
	return dest
}
