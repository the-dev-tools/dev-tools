//go:build windows

package dbtest

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/db/pkg/sqlc/gen"
)

func GetTestDB(ctx context.Context) (*sql.DB, error) {
	return nil, errors.New("not implemented")
}

func GetTestPreparedQueries(ctx context.Context) (*gen.Queries, error) {
	return nil, errors.New("not implemented")
}
