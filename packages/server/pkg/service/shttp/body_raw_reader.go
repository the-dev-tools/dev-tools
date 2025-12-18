package shttp

import (
	"context"
	"database/sql"
	"errors"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
)

type BodyRawReader struct {
	queries *gen.Queries
}

func NewBodyRawReader(db *sql.DB) *BodyRawReader {
	return &BodyRawReader{queries: gen.New(db)}
}

func NewBodyRawReaderFromQueries(queries *gen.Queries) *BodyRawReader {
	return &BodyRawReader{queries: queries}
}

func (r *BodyRawReader) Get(ctx context.Context, id idwrap.IDWrap) (*mhttp.HTTPBodyRaw, error) {
	bodyRaw, err := r.queries.GetHTTPBodyRawByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoHttpBodyRawFound
		}
		return nil, err
	}

	result := ConvertToModelHttpBodyRaw(bodyRaw)
	return &result, nil
}

func (r *BodyRawReader) GetByHttpID(ctx context.Context, httpID idwrap.IDWrap) (*mhttp.HTTPBodyRaw, error) {
	// Check permissions
	_, err := r.queries.GetHTTP(ctx, httpID)
	if err != nil {
		return nil, err
	}

	// Get the body raw for this HTTP
	bodyRaw, err := r.queries.GetHTTPBodyRaw(ctx, httpID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoHttpBodyRawFound
		}
		return nil, err
	}

	result := ConvertToModelHttpBodyRaw(bodyRaw)
	return &result, nil
}
